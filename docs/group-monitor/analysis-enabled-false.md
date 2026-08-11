# 分组监控中 enabled=false 渠道的处理分析

## 问题描述

**场景**：
- 分组 A 有两个渠道：
  - 渠道 1：priority=100，enabled=true，最近测试成功
  - 渠道 2：priority=50，enabled=false（刚被自动禁用）

**问题**：
如果取 `priority 最高 && enabled=true` 的渠道作为分组代表，会取到渠道 1。但渠道 2 刚刚测试失败被禁用，它的失败状态应该体现在分组监控中吗？还是只看当前可用的渠道 1？

---

## 核心矛盾

### 观点 A：只看 enabled=true 的渠道（与路由对齐）
**理由**：
- 路由调度时只会选 `enabled=true` 的渠道
- 用户实际请求走的是渠道 1，那么监控也应该反映渠道 1 的状态
- `enabled=false` 的渠道已经退出服务，不应该影响分组状态

**结论**：分组状态 = "运行正常"（基于渠道 1）

---

### 观点 B：应该包含 enabled=false 的渠道（全局视角）
**理由**：
- 监控的目的是发现问题，优先级高的渠道挂了是**更严重**的问题
- 虽然降级到了渠道 1，但系统已经"降级运行"，管理员应该知道
- 如果只看 enabled=true，渠道 2 挂了管理员可能不知道（因为还有渠道 1 兜底）

**结论**：分组状态 = "降级"（高优先级渠道挂了）

---

## 现有系统的行为分析

### 1. 自动禁用触发条件（service/channel.go:45）

```go
func ShouldDisableChannel(err *types.NewAPIError) bool {
  if !common.AutomaticDisableChannelEnabled {
    return false
  }
  if err == nil {
    return false
  }
  // 渠道错误类型、跳过重试错误、状态码匹配、错误关键词匹配
  return ...
}
```

**关键发现**：不是一次失败就禁用，而是满足特定条件（如特定错误类型、关键词匹配）。

### 2. 自动启用触发条件（service/channel.go:67）

```go
func ShouldEnableChannel(newAPIError *types.NewAPIError, status int) bool {
  if !common.AutomaticEnableChannelEnabled {
    return false
  }
  if newAPIError != nil {
    return false
  }
  if status != common.ChannelStatusAutoDisabled {  // 只恢复自动禁用的，不恢复手动禁用的
    return false
  }
  return true
}
```

**关键发现**：测试成功后会**自动恢复** `enabled=true`（仅限 `status=3` 自动禁用的）。

### 3. 测试流程（controller/channel-test.go:955-962）

```
每个渠道测试后：
  if (当前启用 && 应该禁用 && 允许自动禁用) {
    processChannelError()  → status=3, enabled=false
  }
  if (当前禁用 && 应该启用 && status=3) {
    EnableChannel()  → status=1, enabled=true
  }
  UpdateResponseTime(milliseconds)  // ← 无论成功失败都写入
```

**重要**：`UpdateResponseTime` 在 **禁用/启用之后** 调用，也就是说：
- 渠道 2 测试失败 → 被禁用（enabled=false, status=3）
- 但 `response_time` 和 `test_time` 依然会更新（记录失败时的 0 或超时值）

---

## 实际场景模拟

### 场景 1：渠道 2（高优先级）刚被禁用

**时间线**：
```
T0: 渠道 1 (priority=50, enabled=true, response_time=1147ms, tested_at=T0-10min)
    渠道 2 (priority=100, enabled=true, response_time=980ms, tested_at=T0-10min)
    → 分组状态："正常"，代表渠道=2

T1: 渠道 2 测试失败 → enabled=false, status=3, response_time=0, tested_at=T1
    渠道 1 (priority=50, enabled=true, response_time=1147ms, tested_at=T0-10min)
    → 分组状态：？
```

**方案 A**：只看 enabled=true → 代表渠道=1 → "正常"（延迟 1147ms）
**方案 B**：包含 enabled=false → 代表渠道=2 → "降级"（高优先级挂了）

---

### 场景 2：渠道 2 连续多次失败，历史全红

**时间线**：
```
T0~T5: 渠道 2 连续 6 次测试失败 → enabled=false
       历史：[失败, 失败, 失败, 失败, 失败, 失败]
```

**方案 A**：代表渠道=1，渠道 1 的历史全绿 → 心跳格全绿
**方案 B**：代表渠道=2，渠道 2 的历史全红 → 心跳格全红，但实际用户请求走渠道 1（正常）

**矛盾**：方案 B 会让管理员看到"分组全红"，但实际用户体验是正常的（因为走的是渠道 1）。

---

## 推荐方案：分层展示

### 方案 C：动态代表渠道 + 告警标记

```
分组状态计算规则：
1. 代表渠道 = priority 最高 && enabled=true 的渠道（与路由对齐）
2. 心跳格 = 代表渠道的测试历史
3. 可用率 = 代表渠道的成功率
4. 降级标记 = 检查是否有更高优先级的渠道 enabled=false
```

**前端展示**：
```
┌─────────────────────────────────────────────┐
│ default                          🟡 运行降级  │  ← 状态：运行降级
│ 可用率: 92.31%  |  延迟: 1147ms             │
│ 代表渠道: 渠道1 (priority=50)               │
│ ⚠️ 警告: 渠道2 (priority=100) 已禁用         │  ← 降级原因
│ 心跳: ■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■     │  ← 渠道1的历史（全绿）
└─────────────────────────────────────────────┘
```

**状态定义**：
- `up（运行正常）`：代表渠道可用，且无更高优先级渠道被禁用
- `degraded（运行降级）`：代表渠道可用，但有更高优先级渠道被禁用
- `down（不可用）`：无任何 enabled=true 的渠道

---

## 实现建议

### 后端接口返回结构

```json
{
  "group": "default",
  "status": "degraded",
  "representative_channel": {
    "id": 1,
    "name": "渠道1",
    "priority": 50,
    "response_time": 1147,
    "status": 1
  },
  "disabled_higher_priority_channels": [  // 新增字段
    {
      "id": 2,
      "name": "渠道2",
      "priority": 100,
      "status": 3,
      "last_error": "连接超时",
      "disabled_at": 1234567890
    }
  ],
  "uptime_24h": 0.9231,
  "avg_latency": 1147,
  "heartbeats": [...]  // 代表渠道（渠道1）的历史
}
```

### 查询逻辑

```sql
-- 1. 找出该分组所有渠道按优先级排序
SELECT channel_id, priority, enabled, status 
FROM abilities 
WHERE group = ? AND model = ? 
ORDER BY priority DESC;

-- 2. 取第一个 enabled=true 的作为代表渠道
-- 3. 检查是否有更高优先级的 enabled=false → 设置 status=degraded
-- 4. 查询代表渠道的测试历史
SELECT * FROM channel_test_histories 
WHERE channel_id = ? 
ORDER BY tested_at DESC 
LIMIT 60;
```

---

## 结论

**推荐：方案 C（动态代表渠道 + 降级标记）**

**优点**：
1. ✅ 心跳格反映当前实际服务的渠道状态（与用户体验一致）
2. ✅ 降级标记让管理员知道高优先级渠道挂了（运维告警）
3. ✅ 符合路由逻辑（代表渠道 = 当前调度会选的渠道）
4. ✅ 避免"全红但实际正常"的误导

**缺点**：
- 实现稍复杂（需要额外查询更高优先级被禁用的渠道）

---

## 特殊场景处理

### 场景：所有渠道都 enabled=false

```
代表渠道 = null
状态 = down（不可用）
心跳格 = 空或显示最后一个被禁用的渠道历史
```

### 场景：渠道 2 自动恢复

```
T2: 渠道 2 测试成功 → enabled=true, status=1
    → 代表渠道从 1 切换回 2
    → 状态从 "降级" 恢复为 "正常"
```

---

## 待讨论问题

1. **降级标记的时效性**：如果渠道 2 被禁用超过 24 小时，还需要显示降级标记吗？
2. **多模型场景**：一个分组下不同模型有不同渠道，如何聚合状态？
3. **手动禁用 vs 自动禁用**：手动禁用的渠道是否应该触发降级标记？
