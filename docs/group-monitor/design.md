# 分组监控设计文档

## 1. 背景与目标

### 1.1 现有问题

渠道定时测试（`AutomaticallyTestChannels`）每次只覆写渠道最新的测试结果（`channels.response_time` + `channels.test_time`），**没有历史记录**，因此：

- 无法展示连续的心跳状态格（如最近 60 次检测的成功/失败历史）
- 无法计算可用率（需要历史数据）
- 分组是由多个渠道组成的，分组状态需要从渠道状态推导，没有直接的分组维度统计

### 1.2 目标

在不引入外部监控系统（如 Uptime Kuma）的前提下，基于现有内部渠道测试机制，实现：

1. **渠道测试历史入库**：每次测试结果写入历史表，保留最近 N 条
2. **分组状态推导**：根据分组内渠道的路由策略，推导出分组当前有效状态
3. **前端分组监控页面**：展示各分组的可用率、平均延迟、心跳历史格

---

## 2. 现有架构分析

### 2.1 渠道调度核心：ability 表

**路由调度不直接查 `channels` 表，而是查 `abilities` 表。**

```
abilities 表结构：
  group      string   // 分组名（primary key 之一）
  model      string   // 模型名（primary key 之一）
  channel_id int      // 渠道 ID（primary key 之一）
  enabled    bool     // 是否可用（由 channel.status 同步而来）
  priority   int64    // 优先级（从 channel.priority 同步）
  weight     uint     // 权重（从 channel.weight 同步）
  tag        string   // 标签
```

**关键理解**：
- `channels.status` 变化时，会同步更新 `abilities.enabled`
- 调度时查 `abilities` 而非 `channels`，`abilities.enabled=false` 的渠道不参与选择

### 2.2 渠道选择算法（service/channel_select.go + model/ability.go）

```
CacheGetRandomSatisfiedChannel(group, model, retry)
  └─ GetRandomSatisfiedChannel(group, model, retry)
       └─ getChannelQuery(group, model, retry)
            ├─ retry=0: WHERE group=? AND model=? AND enabled=true AND priority = MAX(priority)
            └─ retry=N: WHERE group=? AND model=? AND enabled=true AND priority = priorities[N]
       └─ 按 weight 加权随机选择（weight+10 保底权重）
```

**选择规则总结**：
1. 只选 `abilities.enabled = true` 的渠道
2. **优先级最高**（priority 值最大）的先用
3. 同优先级内按 **weight 加权随机**
4. 重试时降级到下一优先级

### 2.3 渠道状态常量（common/constants.go）

```go
ChannelStatusEnabled          = 1  // 启用
ChannelStatusManuallyDisabled = 2  // 手动禁用
ChannelStatusAutoDisabled     = 3  // 自动禁用（测试失败自动触发）
```

状态变化会触发 `UpdateAbilityStatus(channelId, enabled bool)` 同步到 abilities 表。

### 2.4 定时测试机制（controller/channel-test.go）

```
AutomaticallyTestChannels()  // goroutine，仅 Master 节点运行
  └─ testAllChannels(false)  // 按配置间隔（默认 N 分钟）触发
       └─ 遍历所有渠道（跳过手动禁用、跳过 SkipAutoTest 的）
            ├─ testChannel(channel, ...)  // 发一次真实请求
            ├─ 判断是否应禁用/启用渠道
            └─ channel.UpdateResponseTime(milliseconds)  // ← 只写最新结果，覆盖旧值
```

**关键发现**：每次测试只更新 `channels.response_time` 和 `channels.test_time`，**无历史记录**。

---

## 3. 分组状态推导设计

### 3.1 核心问题：渠道状态 → 分组状态

一个分组包含多个渠道，如何判断分组是否"可用"？

**参考路由策略**：调度时会从最高优先级渠道开始选，失败后降级。因此分组的可用性取决于**当前有没有任何可用渠道（enabled=true）**，而健康程度由**最高优先级渠道的测试结果**体现。

### 3.2 分组状态定义

```
分组状态 = f(该分组下所有渠道的状态)

规则（按优先级顺序判断）：
1. 找出该分组下优先级最高且 enabled=true 的渠道集合
2. 从中取最近测试时间最新的一条作为"代表渠道"
3. 若无任何 enabled=true 的渠道 → 分组状态 = 不可用（down）
4. 若代表渠道测试成功（response_time > 0）→ 正常（up）
5. 若代表渠道测试失败（response_time = 0）→ 降级（degraded），但仍有更低优先级可用
```

### 3.3 分组历史状态（连续心跳格）

由于分组状态是从渠道推导的，历史心跳格需要基于**渠道测试历史**来计算。

每次渠道测试时：
- 记录本次测试结果到 `channel_test_history` 表
- 查询分组状态时，读取该分组代表渠道最近 N 条历史，推导每个时间点的分组状态

---

## 4. 数据库设计

### 4.1 新增表：channel_test_history

```sql
CREATE TABLE channel_test_histories (
    id            BIGINT PRIMARY KEY AUTO_INCREMENT,
    channel_id    INT    NOT NULL,
    success       BOOL   NOT NULL,          -- 本次测试是否成功
    response_time INT    NOT NULL,          -- 响应时间（毫秒），失败时为 0
    tested_at     BIGINT NOT NULL,          -- 测试时间（Unix 秒）

    INDEX idx_channel_tested (channel_id, tested_at DESC)
);
```

**保留策略**：每个渠道最多保留最近 100 条，定时清理旧数据（或在写入时删除超出部分）。

### 4.2 不新增分组表

分组状态完全由 `channel_test_histories` + `abilities` + `channels` 实时计算，不单独存储分组状态，避免数据冗余。

---

## 5. 后端接口设计

### 5.1 新增接口：获取分组监控状态

```
GET /api/channel/group-monitor
权限：管理员

响应：
[
  {
    "group": "default",
    "status": "up",          // up / degraded / down
    "uptime_24h": 0.9231,    // 最近 24h 可用率（基于历史）
    "avg_latency": 1147,     // 最近 N 次成功测试的平均延迟（ms）
    "last_tested_at": 1234567890,
    "representative_channel": {
      "id": 5,
      "name": "xxx渠道",
      "priority": 100,
      "response_time": 1147,
      "status": 1
    },
    "heartbeats": [          // 最近 60 次（每格对应一次定时测试）
      {"success": true,  "response_time": 1147, "tested_at": 1234567890},
      {"success": true,  "response_time": 980,  "tested_at": 1234567830},
      {"success": false, "response_time": 0,    "tested_at": 1234567770},
      ...
    ]
  },
  ...
]
```

### 5.2 实现逻辑

```
1. 从 abilities 表查出所有 enabled=true 的 (group, channel_id, priority)
2. 按 group 分组
3. 对每个 group：
   a. 找出 priority 最高的 channel_id（代表渠道）
   b. 从 channel_test_histories 取该 channel_id 最近 60 条记录
   c. 计算可用率、平均延迟、状态
   d. 如果代表渠道 enabled=false，降级到下一优先级的渠道
4. 返回所有分组的监控数据
```

### 5.3 写入历史（修改测试逻辑）

在 `controller/channel-test.go` 的 `testAllChannels` 函数中，`channel.UpdateResponseTime(milliseconds)` 调用之后，额外写入历史：

```go
// 现有代码
channel.UpdateResponseTime(milliseconds)

// 新增：写入历史记录
success := result.newAPIError == nil && milliseconds <= disableThreshold
model.RecordChannelTestHistory(channel.Id, success, int(milliseconds))
```

单次手动测试（`TestChannel` 接口，第 875 行）同理。

---

## 6. 前端设计

### 6.1 页面位置

在管理员端侧边栏新增"分组监控"页面，路由：`/dashboard/group-monitor`（或集成到现有 dashboard）。

### 6.2 展示内容（参考截图设计）

每个分组卡片展示：
- **分组名** + **状态标签**（运行正常 / 降级 / 异常）
- **可用率**（最近 24h）
- **平均延迟**（ms）
- **上次检测时间**（N 秒/分钟前）
- **最近 60 次心跳格**（绿=成功，红=失败，橙=超时）
- **代表渠道信息**（名称、优先级、当前响应时间）

### 6.3 数据刷新

- 页面加载时请求一次
- 提供"刷新"按钮
- 可选：每 30 秒自动轮询

---

## 7. 实现步骤

| 步骤 | 内容 | 文件 |
|------|------|------|
| 1 | 新增 `ChannelTestHistory` model 和 `RecordChannelTestHistory` 函数 | `model/channel_test_history.go` |
| 2 | 在定时测试和手动测试后写入历史 | `controller/channel-test.go` |
| 3 | 新增后端接口 `GetGroupMonitorStatus` | `controller/channel_group_monitor.go` |
| 4 | 注册路由 | `router/api-router.go` |
| 5 | 前端新增分组监控页面 | `web/default/src/features/group-monitor/` |

---

## 8. 关键设计决策

### Q1：分组心跳应该用哪个渠道的历史？

**答**：优先级最高且当前 enabled=true 的渠道。与路由策略一致——调度时最先用的就是它。若最高优先级渠道禁用，则降级到次优先级，与重试机制对齐。

### Q2：历史保留多少条？

**答**：每个渠道保留最近 100 条（对应约 100 个测试周期）。前端展示 60 格，100 条留有余量。定时清理超出部分。

### Q3：可用率怎么算？

**答**：`成功次数 / 总测试次数`，基于最近 100 条历史（非固定时间窗口，因为测试间隔可配置）。如需精确时间窗口（如 24h），则按 `tested_at >= now-86400` 过滤。

### Q4：分组没有配置任何渠道怎么处理？

**答**：不展示该分组，或展示状态为"无数据"。

### Q5：多模型场景下分组状态怎么处理？

**答**：一个分组下可能有多个模型对应不同渠道。监控时取**所有模型中代表渠道最新测试时间最近的一条**作为分组代表，或者按模型分别展示（二期考虑）。一期简化为：对每个分组，不区分模型，取该分组 priority 最高的渠道。
