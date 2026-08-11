# 定时测试优化：利用用户真实请求数据

## 1. 优化目标

**现状**：定时测试每次都主动发送测试请求到上游
**问题**：
- 浪费资源（额外的API调用成本）
- 增加渠道负担
- 如果渠道刚刚被用户正常使用过，再测一次意义不大

**优化**：如果渠道最近有用户正常请求，直接利用用户请求的数据作为测试结果

---

## 2. 优化逻辑

### 2.1 判断条件

在定时测试时，对每个渠道检查：
```
IF 该渠道在最近 20 秒内有用户正常请求 THEN
    跳过主动测试
    直接将用户请求数据写入 channel_test_histories
ELSE
    执行原有的主动测试逻辑
END IF
```

### 2.2 "正常请求"的定义

**正常请求**：
- ✅ 请求成功（无错误）
- ✅ 响应时间在阈值内
- ✅ 最近 20 秒内发生

---

## 3. 数据来源

### 3.1 用户请求数据在哪？

**现有系统**：
- 每次中转请求都会记录渠道使用情况
- `middleware/distributor.go` 选择渠道
- `controller/relay.go` 处理请求和响应
- 响应时会更新渠道状态（如果失败会调用 `processChannelError`）

**关键点**：用户请求的耗时、成功/失败状态，已经在现有逻辑中处理了。

### 3.2 需要新增什么

**方案 A：实时写入测试历史表**

在用户请求完成时，额外写入 `channel_test_histories` 表：

```go
// controller/relay.go 或相关中转完成的地方
func afterChannelUsed(channelId int, success bool, responseTime int64) {
    // 现有逻辑：更新 channels.response_time
    channel.UpdateResponseTime(responseTime)
    
    // 新增：同时写入测试历史
    model.RecordChannelTestHistory(channelId, success, int(responseTime))
}
```

**优点**：
- 定时测试时直接查表，判断最近是否有记录
- 数据统一存储

**缺点**：
- 用户请求频繁时，`channel_test_histories` 表会快速增长

---

**方案 B：使用缓存记录最近使用时间**

在内存中记录每个渠道最近一次成功使用的时间和响应时间：

```go
// service/channel_usage_cache.go
var channelLastUsage sync.Map  // map[int]*ChannelUsageRecord

type ChannelUsageRecord struct {
    ChannelId    int
    LastUsedAt   int64  // Unix timestamp
    ResponseTime int
    Success      bool
}

func RecordChannelUsage(channelId int, success bool, responseTime int) {
    record := &ChannelUsageRecord{
        ChannelId:    channelId,
        LastUsedAt:   time.Now().Unix(),
        ResponseTime: responseTime,
        Success:      success,
    }
    channelLastUsage.Store(channelId, record)
}

func GetRecentChannelUsage(channelId int, withinSeconds int64) *ChannelUsageRecord {
    if val, ok := channelLastUsage.Load(channelId); ok {
        record := val.(*ChannelUsageRecord)
        now := time.Now().Unix()
        if now - record.LastUsedAt <= withinSeconds && record.Success {
            return record
        }
    }
    return nil
}
```

**优点**：
- 不增加数据库写入压力
- 查询快速（内存）

**缺点**：
- 重启后丢失（但影响不大，最多多测一次）
- 需要额外的内存管理

---

### 3.3 推荐方案

**推荐方案 A**：统一写入 `channel_test_histories` 表

**理由**：
1. 数据统一，无论是用户请求还是主动测试，都在同一个表
2. 重启后数据不丢失
3. 可以查看完整的渠道使用历史（包括用户请求）
4. 心跳格可以反映真实的用户体验

**优化**：
- 用户请求写入时，不需要每次都写，可以**限流**（如 5 秒内最多写一次同一渠道）
- 或者设置一个开关：`RecordUserRequestAsTest`，默认开启

---

## 4. 实现细节

### 4.1 在用户请求完成后写入历史

**修改位置**：找到中转请求完成、更新渠道响应时间的地方

让我先查找一下：

```bash
grep -rn "UpdateResponseTime\|processChannelError" controller/relay.go
```

**预期位置**：
- 成功：响应返回后，记录响应时间
- 失败：`processChannelError` 被调用时

### 4.2 定时测试时检查最近使用

**修改文件**：`controller/channel-test.go` 的 `testAllChannels` 函数

```go
func testAllChannels(notify bool) error {
    // ... 现有代码 ...
    
    for _, channel := range channels {
        if channel.Status == common.ChannelStatusManuallyDisabled {
            continue
        }
        
        // 新增：检查最近 20 秒内是否有用户请求
        recentTest, err := model.GetMostRecentChannelTest(channel.Id, 20)
        if err == nil && recentTest != nil {
            // 有最近的用户请求数据，跳过主动测试
            common.SysLog(fmt.Sprintf("渠道 #%d 最近 20 秒内有用户请求，跳过主动测试", channel.Id))
            
            // 更新 channels 表的 test_time（response_time 已经在用户请求时更新了）
            channel.UpdateTestTime()
            
            time.Sleep(common.RequestInterval)
            continue
        }
        
        // 没有最近请求，执行原有的主动测试逻辑
        isChannelEnabled := channel.Status == common.ChannelStatusEnabled
        tik := time.Now()
        result := testChannel(channel, testUserID, "", "", shouldUseStreamForAutomaticChannelTest(channel))
        tok := time.Now()
        milliseconds := tok.Sub(tik).Milliseconds()
        
        // ... 原有测试逻辑 ...
    }
}
```

### 4.3 新增数据库查询函数

**文件**：`model/channel_test_history.go`

```go
// GetMostRecentChannelTest 获取渠道最近 N 秒内的测试记录
func GetMostRecentChannelTest(channelId int, withinSeconds int64) (*ChannelTestHistory, error) {
    var history ChannelTestHistory
    cutoff := common.GetTimestamp() - withinSeconds
    
    err := DB.Where("channel_id = ? AND tested_at >= ?", channelId, cutoff).
        Order("tested_at DESC").
        First(&history).Error
    
    if err == gorm.ErrRecordNotFound {
        return nil, nil
    }
    
    return &history, err
}
```

### 4.4 新增 Channel 方法

**文件**：`model/channel.go`

```go
// UpdateTestTime 只更新测试时间，不更新响应时间（用于利用用户请求数据时）
func (channel *Channel) UpdateTestTime() {
    err := DB.Model(channel).Select("test_time").Updates(Channel{
        TestTime: common.GetTimestamp(),
    }).Error
    if err != nil {
        common.SysLog(fmt.Sprintf("failed to update test time: channel_id=%d, error=%v", channel.Id, err))
    }
}
```

---

## 5. 用户请求写入历史的实现

### 5.1 找到写入位置

**关键函数**：
- `controller/relay.go` 中处理响应的地方
- 或者 `middleware/distributor.go` 中记录渠道使用的地方

**查找方式**：
```bash
grep -rn "c.JSON\|c.Writer\|relay.*response" controller/relay.go
```

### 5.2 添加写入逻辑

**示例**（具体位置需要根据代码确定）：

```go
// 在响应成功返回后
func relayResponse(c *gin.Context, ...) {
    // ... 现有响应逻辑 ...
    
    // 记录渠道使用到测试历史
    if channelId, exists := common.GetContextKey(c, constant.ContextKeyChannelId); exists {
        if id, ok := channelId.(int); ok {
            responseTime := int(time.Since(startTime).Milliseconds())
            success := statusCode < 400  // 或其他成功判断逻辑
            
            go model.RecordChannelTestHistory(id, success, responseTime)
        }
    }
}
```

**注意**：
- 使用 `go` 异步写入，不阻塞用户请求
- 只在成功响应时写入（或根据业务需求调整）

---

## 6. 开关控制

### 6.1 新增配置项

**文件**：`common/constants.go`

```go
var UseUserRequestForChannelTest = true  // 利用用户请求数据作为测试结果
```

**文件**：`model/option.go`

```go
case "UseUserRequestForChannelTest":
    common.UseUserRequestForChannelTest = boolValue
```

### 6.2 前端配置

**位置**：监控与报警页面

```
☑ 利用用户请求数据优化测试
  当渠道最近有用户正常请求时，跳过主动测试，直接使用用户请求数据
```

---

## 7. 效果评估

### 7.1 预期效果

**高频渠道**（用户请求频繁）：
- 几乎不需要主动测试
- 测试数据完全来自真实用户请求
- 节省大量测试成本

**低频渠道**（用户请求稀少）：
- 仍然进行主动测试
- 保证及时发现问题

### 7.2 数据示例

假设定时测试间隔 = 30 分钟：

| 渠道 | 用户请求频率 | 优化前 | 优化后 |
|------|------------|-------|-------|
| 高频渠道 | 每分钟 10+ 次 | 主动测试 48 次/天 | 主动测试 0 次/天 |
| 中频渠道 | 每 5 分钟 1 次 | 主动测试 48 次/天 | 主动测试 ~10 次/天 |
| 低频渠道 | 每小时 < 1 次 | 主动测试 48 次/天 | 主动测试 48 次/天 |

---

## 8. 边界情况处理

### 8.1 用户请求失败了怎么办？

**场景**：用户请求失败 → 写入 `channel_test_histories` (success=false)

**定时测试行为**：
- 查到最近 20 秒内有记录
- 但 `success=false`
- **应该跳过还是主动测试？**

**推荐**：只有 `success=true` 的记录才算"有效"，失败记录不算

**修改查询**：
```go
err := DB.Where("channel_id = ? AND tested_at >= ? AND success = ?", channelId, cutoff, true).
    Order("tested_at DESC").
    First(&history).Error
```

### 8.2 20 秒合理吗？

**可配置化**：

```go
var ChannelTestSkipWindow = 20  // 秒
```

**推荐值**：
- 20 秒：比较保守，确保数据新鲜
- 60 秒：更激进，但可能错过问题
- 建议：可配置，默认 20 秒

---

## 9. 实现检查清单

| 步骤 | 文件 | 内容 |
|------|------|------|
| 1 | `model/channel_test_history.go` | 新增 `GetMostRecentChannelTest` 函数 |
| 2 | `model/channel.go` | 新增 `UpdateTestTime` 方法 |
| 3 | `controller/relay.go` | 用户请求完成后写入测试历史 |
| 4 | `controller/channel-test.go` | 定时测试时检查最近使用 |
| 5 | `common/constants.go` | 新增 `UseUserRequestForChannelTest` 开关 |
| 6 | `model/option.go` | 新增配置读取 |
| 7 | 前端配置页面 | 新增开关 |

---

## 10. 总结

### 核心思想

**充分利用真实用户请求数据，减少不必要的主动测试**

### 优点

1. ✅ 节省测试成本（高频渠道几乎不需要主动测试）
2. ✅ 真实反映用户体验（测试数据来自真实请求）
3. ✅ 减少上游负担
4. ✅ 保持现有逻辑兼容（可通过开关控制）

### 注意事项

1. ⚠️ 只有成功的用户请求才算"有效测试"
2. ⚠️ 低频渠道仍需主动测试
3. ⚠️ 写入测试历史要做限流（避免高频写入）
