# 渠道 0 t/s 监控设计方案（Redis 版 - 简化）

## 1. 设计目标

在现有渠道自动禁用机制基础上，增加"流式响应速度异常检测"：
- 复用日志中已有的 tokens 和耗时数据
- 最近10次中超过5次为 0 t/s 时，自动禁用渠道
- 无需重新计算，直接使用现有值

---

## 2. 数据来源（复用现有数据）

### 2.1 现有日志数据

在 `RecordConsumeLog` 记录日志时，已包含：

| 字段 | 来源 | 说明 |
|------|------|------|
| `CompletionTokens` | 响应结果 | 总 tokens 数 |
| `UseTimeSeconds` | 请求耗时 | 总耗时（秒） |
| `IsStream` | 请求类型 | 是否流式请求 |

**t/s 计算公式**（已有数据）：
```go
tps := float64(CompletionTokens) / float64(UseTimeSeconds)
```

**无需修改计算逻辑**，直接在记录日志后，额外写入 Redis。

---

## 3. Redis 存储设计

### 3.1 数据结构

**Key 设计**
```
格式：channel_tps:{channel_id}
类型：List
示例：channel_tps:123
```

**Value 结构**
```
数据类型：List（有序列表）
元素内容：float64 字符串形式
最大长度：10
顺序：最新的在左侧（表头）

示例值：
["12.5", "0", "0", "8.3", "0", "15.2", "0", "9.1", "0", "0"]
 ↑最新                                                 ↑最旧
```

**特殊值约定**
- `"0"`：真实的 0 t/s（慢速或卡死）
- `"-1"`：计算失败或不适用（判断时跳过）

---

### 3.2 Redis 操作

#### 写入操作
```redis
# 每次记录日志后执行
LPUSH channel_tps:{channel_id} {tps_value}  # 插入到列表头部
LTRIM channel_tps:{channel_id} 0 9          # 保持最多10个元素
EXPIRE channel_tps:{channel_id} 604800      # 设置过期时间7天
```

**Pipeline 优化**：
```go
pipe := rdb.Pipeline()
pipe.LPush(ctx, key, tps)
pipe.LTrim(ctx, key, 0, 9)
pipe.Expire(ctx, key, 7*24*time.Hour)
_, err := pipe.Exec(ctx)
```

#### 读取操作
```redis
# 判断禁用时执行
LRANGE channel_tps:{channel_id} 0 9  # 读取最近10次记录
```

---

### 3.3 过期策略

| 场景 | TTL 设置 | 说明 |
|------|---------|------|
| **正常使用的渠道** | 每次写入刷新为 7天 | 活跃渠道自动续期 |
| **不活跃渠道** | 7天后自动删除 | 节省内存 |
| **被禁用的渠道** | 保持不变 | 保留数据便于分析 |

---

## 4. 业务流程设计

### 4.1 记录 t/s 流程（简化）

```
┌─────────────────────┐
│  RecordConsumeLog    │  记录日志（现有逻辑）
└──────────┬──────────┘
           │
           ▼
    ┌──────────────┐
    │ IsStream?    │──No──> 不记录 t/s
    └──────┬───────┘
           │Yes
           ▼
┌─────────────────────┐
│  计算 t/s            │  CompletionTokens / UseTimeSeconds
└──────────┬──────────┘
           │
           ▼
    ┌─────────────┐
    │ tps > 0?    │──No──> 记录 "0"
    └──────┬──────┘
           │Yes
           ▼
    ┌─────────────────┐
    │ 记录实际 tps 值  │
    └──────┬──────────┘
           │
           ▼
┌─────────────────────────┐
│  异步写入 Redis          │  LPUSH + LTRIM + EXPIRE
└─────────────────────────┘
```

**关键点**：
- 在 `RecordConsumeLog` 函数末尾调用
- 只处理流式请求（`IsStream == true`）
- 异步执行，不阻塞日志记录

---

### 4.2 禁用判断流程

```
┌──────────────────────┐
│  现有禁用逻辑判断     │  错误码、关键词等
└──────────┬───────────┘
           │
        No │
           ▼
    ┌──────────────────┐
    │ 0 t/s 检测开关   │──关闭──> 不禁用
    │   已开启?        │
    └──────┬───────────┘
           │开启
           ▼
┌──────────────────────────┐
│  从 Redis 读取最近10次    │  LRANGE channel_tps:{id} 0 9
└──────────┬───────────────┘
           │
           ▼
    ┌──────────────────┐
    │ 样本数 >= 10?    │──No──> 不禁用（样本不足）
    └──────┬───────────┘
           │Yes
           ▼
┌──────────────────────────┐
│  统计值为 "0" 的次数      │  跳过 "-1"
└──────────┬───────────────┘
           │
           ▼
    ┌──────────────────┐
    │ 0次数 >= 5?      │──No──> 不禁用
    └──────┬───────────┘
           │Yes
           ▼
┌──────────────────────────┐
│  禁用渠道                 │
│  原因：连续多次0 t/s      │
└──────────────────────────┘
```

---

## 5. 代码集成点

### 5.1 记录 t/s 的位置

**主要文件**：
- `model/log.go` → `RecordConsumeLog` 函数末尾

**集成点**：
```go
func RecordConsumeLog(c *gin.Context, userId int, params RecordConsumeLogParams) {
    // ... 现有日志记录逻辑 ...

    // 新增：记录 t/s 到 Redis（只针对流式请求）
    if params.IsStream && params.CompletionTokens > 0 && params.UseTimeSeconds > 0 {
        tps := float64(params.CompletionTokens) / float64(params.UseTimeSeconds)
        go recordChannelTps(params.ChannelId, tps)
    }
}

func recordChannelTps(channelId int, tps float64) {
    if !common.DisableChannelByZeroTpsEnabled {
        return  // 功能未启用
    }

    ctx, cancel := context.WithTimeout(context.Background(), time.Second)
    defer cancel()

    key := fmt.Sprintf("channel_tps:%d", channelId)

    // 使用 Pipeline 优化
    pipe := common.RDB.Pipeline()
    pipe.LPush(ctx, key, fmt.Sprintf("%.2f", tps))
    pipe.LTrim(ctx, key, 0, 9)
    pipe.Expire(ctx, key, time.Duration(common.ZeroTpsRedisTTL)*time.Second)

    if _, err := pipe.Exec(ctx); err != nil {
        logger.SysError("写入渠道 t/s 到 Redis 失败: " + err.Error())
    }
}
```

---

### 5.2 禁用判断的位置

**主要文件**：
- `service/channel.go`

**调用链**：
```
controller/relay.go
  └─> relayHandler() 返回错误
      └─> service.ShouldDisableChannel(err, channelId)
          ├─> 现有逻辑：错误类型、状态码判断
          └─> 新增：shouldDisableByZeroTps(channelId)
              ├─> 从 Redis 读取：LRANGE channel_tps:{id} 0 9
              ├─> 统计 "0" 的次数
              └─> 返回 true/false
```

**伪代码**：
```go
func ShouldDisableChannel(err *types.NewAPIError, channelId int) bool {
    // 现有逻辑...
    if !common.AutomaticDisableChannelEnabled {
        return false
    }
    if err == nil {
        return false
    }
    if types.IsChannelError(err) {
        return true
    }
    // ... 其他现有判断 ...

    // 新增：检查 0 t/s（独立判断，不依赖 err）
    if shouldDisableByZeroTps(channelId) {
        return true
    }

    return false
}

func shouldDisableByZeroTps(channelId int) bool {
    if !common.DisableChannelByZeroTpsEnabled {
        return false
    }

    ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
    defer cancel()

    key := fmt.Sprintf("channel_tps:%d", channelId)
    vals, err := common.RDB.LRange(ctx, key, 0, 9).Result()

    if err != nil {
        logger.SysError(fmt.Sprintf("读取渠道 #%d t/s 数据失败: %s", channelId, err.Error()))
        return false  // Redis 不可用时不禁用
    }

    // 必须有10条数据才判断（样本不足时不禁用）
    if len(vals) < 10 {
        return false  // 样本不足
    }

    // 统计 0 的次数
    zeroCount := 0
    for _, v := range vals {
        if v == "0" || v == "0.00" {
            zeroCount++
        }
        // 跳过 "-1"（无效数据）
    }

    if zeroCount >= common.ZeroTpsThreshold {
        logger.SysLog(fmt.Sprintf("渠道 #%d 最近 %d 次请求中有 %d 次为 0 t/s，超过阈值 %d",
            channelId, len(vals), zeroCount, common.ZeroTpsThreshold))
        return true
    }

    return false
}
```

---

## 6. 配置参数设计

### 6.1 全局配置

**代码配置（`common/constants.go`）**
```go
var (
    // 是否启用 0 t/s 检测
    DisableChannelByZeroTpsEnabled = true

    // 样本中出现几次 0 触发禁用（默认5，即10次中5次为0）
    ZeroTpsThreshold = 5

    // Redis key 过期时间（秒）
    ZeroTpsRedisTTL = 604800  // 7天
)
```

**管理面板配置路径**
```
监控与警报
├─ 渠道状态变更邮件通知：[开关]
├─ 10条 t/s 有几个为 0 则禁用：[5] （新增）
└─ ... 其他配置 ...
```

**配置说明**：
- 固定检查最近 **10条** t/s 记录
- 可配置其中有 **几条为 0** 时触发禁用（默认 5）
- 如果数据量 **不足 10条**，不触发禁用（样本不足）

---

### 6.2 渠道级配置（可选）

某些渠道可能需要豁免，在 `channels.other` 字段配置：

```json
{
  "zero_tps_detection_enabled": false,  // 该渠道禁用检测
  "zero_tps_threshold": 7               // 自定义阈值（更宽松）
}
```

**读取逻辑**：
```go
func shouldDisableByZeroTps(channelId int) bool {
    // 检查渠道级配置
    channel, _ := model.GetChannelById(channelId, true)
    if channel != nil && channel.Other != "" {
        var other map[string]interface{}
        if json.Unmarshal([]byte(channel.Other), &other) == nil {
            if enabled, ok := other["zero_tps_detection_enabled"].(bool); ok && !enabled {
                return false  // 该渠道禁用检测
            }
            if threshold, ok := other["zero_tps_threshold"].(float64); ok {
                // 使用渠道自定义阈值
                ZeroTpsThreshold = int(threshold)
            }
        }
    }

    // ... 后续逻辑 ...
}
```

---

## 7. 并发与容错设计

### 7.1 并发安全

**Redis 原子性保证**：
- `LPUSH`、`LTRIM`、`EXPIRE` 均为原子操作
- 无需额外加锁
- 多个请求并发写入同一渠道，自动排队处理

**异步写入**：
- 使用 `go recordChannelTps()` 异步执行
- 不阻塞日志记录主流程

---

### 7.2 容错处理

| 异常场景 | 处理方式 |
|---------|----------|
| **Redis 连接失败** | 记录错误日志，跳过写入，不影响主流程 |
| **写入超时** | 设置 1秒超时，超时则放弃本次记录 |
| **读取失败** | 禁用判断返回 false（不禁用） |
| **读取超时** | 设置 500ms 超时，超时返回 false |
| **数据格式错误** | 忽略异常值，只统计有效的 "0" |
| **UseTimeSeconds 为 0** | 记录 "-1"（避免除零错误） |

**容错代码**：
```go
func recordChannelTps(channelId int, tps float64) {
    defer func() {
        if r := recover(); r != nil {
            logger.SysError(fmt.Sprintf("记录渠道 t/s panic: %v", r))
        }
    }()

    // ... 写入逻辑 ...
}
```

---

## 8. 监控与日志

### 8.1 日志设计

**级别 DEBUG**（可选，默认关闭）
```
渠道 #123 本次请求 t/s: 12.5，已写入 Redis
```

**级别 INFO**
```
渠道 #123 最近10次 t/s: [0, 0, 15.2, 0, 8.3, 0, 12.5, 9.1, 0, 11.0]
其中 0 次数: 6，超过阈值 5，准备禁用
```

**级别 WARN**
```
写入 Redis 失败：渠道 #123，错误：connection timeout
```

**级别 ERROR**
```
Redis 不可用，已降级关闭 0 t/s 检测
```

---

### 8.2 管理面板展示（可选）

**渠道列表增强**
```
渠道卡片增加字段：
┌──────────────────────────┐
│ 渠道名称：OpenAI-API-1   │
│ 状态：启用               │
│ 最近 t/s：[12, 0, 0, 8...] │  ← 新增
│ 0速率次数：3/10          │  ← 新增（红色警告）
└──────────────────────────┘
```

**API 端点**（可选）
```
GET /api/channel/{id}/tps-history

Response:
{
  "channel_id": 123,
  "recent_tps": ["12.5", "0", "0", "8.3", ...],
  "zero_count": 3,
  "sample_size": 10,
  "threshold": 5,
  "health_status": "warning"  // normal / warning / critical
}
```

---

## 9. 测试方案

### 9.1 单元测试

**测试 t/s 计算**
```go
func TestCalculateTps(t *testing.T) {
    tests := []struct{
        name             string
        completionTokens int
        useTimeSeconds   int
        expected         float64
    }{
        {"正常速度", 100, 10, 10.0},
        {"慢速", 10, 100, 0.1},
        {"极慢", 1, 100, 0.01},
        {"除零保护", 100, 0, 0.0},  // 应记录为 -1
        {"无 tokens", 0, 10, 0.0},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            var tps float64
            if tt.useTimeSeconds > 0 {
                tps = float64(tt.completionTokens) / float64(tt.useTimeSeconds)
            }
            assert.Equal(t, tt.expected, tps)
        })
    }
}
```

**测试禁用判断**
```go
func TestShouldDisableByZeroTps(t *testing.T) {
    tests := []struct{
        name     string
        tpsData  []string
        expected bool
    }{
        {"正常渠道", []string{"12", "8", "15", "10", "9", "11", "13", "14", "8", "12"}, false},
        {"偶尔0", []string{"12", "0", "15", "10", "0", "11", "13", "14", "8", "12"}, false},
        {"恰好达到阈值", []string{"0", "0", "15", "0", "0", "11", "0", "14", "8", "12"}, true},  // 5次0
        {"超过阈值", []string{"0", "0", "0", "0", "0", "0", "14", "8", "12", "11"}, true},  // 6次0
        {"全0", []string{"0", "0", "0", "0", "0", "0", "0", "0", "0", "0"}, true},
        {"样本不足5条", []string{"0", "0", "0", "0", "0"}, false},
        {"样本不足9条", []string{"0", "0", "0", "0", "0", "0", "0", "0", "0"}, false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // 写入测试数据到 Redis
            key := "channel_tps:999"
            rdb.Del(ctx, key)
            for _, v := range tt.tpsData {
                rdb.LPush(ctx, key, v)
            }

            result := shouldDisableByZeroTps(999)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

---

### 9.2 集成测试

**场景 1：模拟慢速渠道**
```
步骤：
1. 创建测试渠道 #999
2. 模拟10次流式请求，UseTimeSeconds 很大（模拟慢速）
3. 检查 Redis 数据
4. 触发禁用判断

预期：
- Redis 中有10条接近 0 的记录
- 渠道被自动禁用
- 禁用原因：最近10次请求中有X次为0 t/s
```

**场景 2：非流式请求不影响**
```
步骤：
1. 发送5次流式请求（正常）
2. 发送10次非流式请求（不记录）
3. 发送5次流式请求（慢速）

预期：
- Redis 中只有10次流式记录
- 非流式请求被忽略
```

**场景 3：Redis 不可用**
```
步骤：
1. 关闭 Redis 服务
2. 发送请求并记录日志

预期：
- 日志记录正常完成
- t/s 写入失败但不影响主流程
- 错误日志记录 Redis 连接失败
```

---

## 10. 边界条件处理

| 场景 | 处理方式 |
|------|----------|
| **新渠道首次使用** | 样本 < 10 时不判断 |
| **非流式请求** | 不记录到 Redis |
| **UseTimeSeconds 为 0** | 记录 "-1"，避免除零 |
| **CompletionTokens 为 0** | 记录 "0" |
| **计算结果为负数** | 理论上不可能，但可加保护：`max(tps, 0)` |
| **渠道被删除** | Redis 数据自然过期（7天） |
| **渠道重新启用** | 可选：`DEL channel_tps:{id}` 清空历史 |
| **Redis 不可用** | 跳过写入/读取，不影响主流程 |

---

## 11. 部署与上线

### 11.1 环境依赖

**Redis 版本要求**
- 最低版本：Redis 2.8+（支持 LTRIM）
- 推荐版本：Redis 6.0+

**配置要求**
```
redis.conf:
  maxmemory-policy: allkeys-lru  # 内存不足时自动淘汰
  save ""                        # 关闭持久化（数据可丢失）
```

**连接池配置**
```go
RedisClient = redis.NewClient(&redis.Options{
    Addr:         "localhost:6379",
    PoolSize:     50,
    MinIdleConns: 10,
    MaxRetries:   3,
    DialTimeout:  5 * time.Second,
    ReadTimeout:  3 * time.Second,
    WriteTimeout: 3 * time.Second,
})
```

---

### 11.2 灰度发布

**Phase 1：只记录（1周）**
```go
DisableChannelByZeroTpsEnabled = false  // 关闭禁用
// 只写入 Redis，观察数据
```

**观察指标**：
- Redis 内存占用
- 写入成功率
- t/s 数据分布

**Phase 2：告警（1周）**
```go
DisableChannelByZeroTpsEnabled = true
// 满足条件时只记录日志，不真正禁用
if shouldDisableByZeroTps(channelId) {
    logger.Warn(fmt.Sprintf("渠道 #%d 触发 0 t/s 检测（灰度阶段未禁用）", channelId))
    return false  // 暂不禁用
}
```

**Phase 3：正式启用**
```go
DisableChannelByZeroTpsEnabled = true
// 完整功能上线
```

---

### 11.3 回滚方案

**紧急回滚**
```go
// 关闭全局开关
DisableChannelByZeroTpsEnabled = false
```

**数据清理**（可选）
```bash
# 批量删除所有 tps 记录
redis-cli KEYS "channel_tps:*" | xargs redis-cli DEL
```

---

## 12. 性能评估

### 12.1 内存占用

**单个渠道**
```
Key:   "channel_tps:123"       → 18 字节
Value: ["12.5", "0", ...] × 10 → 约 60 字节
元数据                          → 约 20 字节
--------------------------------------------
总计：约 100 字节/渠道
```

**100个渠道**
```
100 × 100 字节 = 10KB
```

**1000个渠道**
```
1000 × 100 字节 = 100KB
```

**结论**：内存占用极小，可忽略。

---

### 12.2 性能影响

**写入性能**
```
操作：LPUSH + LTRIM + EXPIRE（Pipeline）
耗时：< 1ms（本地 Redis）
方式：异步执行
影响：对日志记录无影响
```

**读取性能**
```
操作：LRANGE 0 9
耗时：< 0.5ms
频率：仅在请求失败时触发（低频）
影响：可忽略
```

**结论**：性能影响可忽略，不会增加请求延迟。

---

## 13. 总结

### 方案优势
✅ **极简实现**：复用现有数据，无需重新计算
✅ **低侵入性**：只在 `RecordConsumeLog` 末尾增加一行调用
✅ **高性能**：异步写入，Pipeline 优化
✅ **自动过期**：不活跃渠道自动清理
✅ **容错友好**：Redis 不可用时不影响主流程
✅ **灵活配置**：全局开关 + 渠道级配置

### 推荐配置
```
样本大小：10 次
触发阈值：5 次 (50%)
Redis TTL：7 天
灰度周期：2-3 周
```

### 关键改进点
1. **直接复用** `CompletionTokens / UseTimeSeconds`，无需重新计算
2. **在日志记录后**异步写入 Redis，完全无侵入
3. **Pipeline 优化**，减少 Redis 往返次数
4. **容错降级**，Redis 不可用时自动跳过

---

**文档完成，可直接用于开发实施。**
