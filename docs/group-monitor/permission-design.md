# 分组监控权限与脱敏设计

## 1. 权限分级

### 1.1 用户角色定义

| 角色 | 说明 | 权限 |
|------|------|------|
| **普通用户** | 使用 API 的开发者 | 只能看到自己的使用日志、额度 |
| **管理员** | 系统管理员 | 可以看到所有渠道信息、配置、密钥 |

### 1.2 分组监控访问权限

**接口权限**：
```
GET /api/channel/group-monitor
权限要求：middleware.AdminAuth()  // 仅管理员可访问
```

**原因**：
- 分组监控涉及渠道状态、优先级、延迟等运维信息
- 普通用户不应该知道系统内部的渠道调度策略
- 渠道名称可能暴露上游供应商信息（安全考虑）

---

## 2. 渠道名称脱敏规则

### 2.1 为什么需要脱敏

**安全风险**：
- 渠道名称可能包含供应商信息（如 "OpenAI官方"、"Claude代理商A"）
- 暴露优先级和权重可能被恶意利用
- 普通用户不需要知道后端具体用哪个渠道

**场景举例**：
```
真实渠道名：
  - "OpenAI官方API - 美国节点"
  - "Claude Pro 企业账号 #3"
  - "国内中转 - 供应商A"

脱敏后（给普通用户看）：
  - "渠道1 (priority=100)"
  - "渠道2 (priority=50)"
  - "渠道3 (priority=50)"
```

### 2.2 脱敏策略

**方案 A：完全隐藏渠道维度**
- 普通用户只看到分组级别的监控
- 不展示具体是哪个渠道在服务
- 问题：无法解释为什么性能变化（切换了代表渠道）

**方案 B：显示优先级，隐藏渠道名（推荐）**
- 显示："代表渠道: Priority 100"
- 降级告警："警告: Priority 100 渠道已禁用，当前使用 Priority 50"
- 管理员额外显示："代表渠道: 渠道名 (priority=100, id=5)"

**方案 C：使用分组内编号**
- 显示："代表渠道: 渠道 #1"（按 priority 降序编号，最高优先级 = #1）
- 降级告警："警告: 渠道 #1 已禁用，当前使用渠道 #2"

---

## 3. 接口返回结构设计

### 3.1 管理员（isAdmin=true）

```json
{
  "group": "default",
  "status": "degraded",
  "representative_channel": {
    "id": 1,
    "name": "OpenAI官方API - 美国节点",  // ← 管理员可见
    "priority": 50,
    "response_time": 1147,
    "status": 1
  },
  "disabled_higher_priority_channels": [
    {
      "id": 2,
      "name": "Claude Pro 企业账号 #3",  // ← 管理员可见
      "priority": 100,
      "status": 3,
      "last_error": "连接超时",
      "disabled_at": 1234567890
    }
  ],
  "uptime_24h": 0.9231,
  "avg_latency": 1147,
  "heartbeats": [...]
}
```

### 3.2 普通用户（isAdmin=false）- 方案 B

```json
{
  "group": "default",
  "status": "degraded",
  "representative_channel": {
    "priority": 50,              // ← 只显示优先级
    "response_time": 1147,
    "status": 1
    // id 和 name 不返回
  },
  "disabled_higher_priority_channels": [
    {
      "priority": 100,           // ← 只显示优先级
      "status": 3
      // id、name、last_error 不返回
    }
  ],
  "uptime_24h": 0.9231,
  "avg_latency": 1147,
  "heartbeats": [...]
}
```

### 3.3 普通用户（isAdmin=false）- 方案 C（更友好）

```json
{
  "group": "default",
  "status": "degraded",
  "representative_channel": {
    "display_name": "渠道 #2",   // ← 按优先级排序的编号
    "priority": 50,
    "response_time": 1147,
    "status": 1
  },
  "disabled_higher_priority_channels": [
    {
      "display_name": "渠道 #1",
      "priority": 100,
      "status": 3
    }
  ],
  "uptime_24h": 0.9231,
  "avg_latency": 1147,
  "heartbeats": [...]
}
```

---

## 4. 前端展示对比

### 4.1 管理员视图

```
┌─────────────────────────────────────────────────────┐
│ default                                  🟡 运行降级  │
│ 可用率: 92.31%  |  延迟: 1147ms                      │
│ 代表渠道: OpenAI官方API - 美国节点 (priority=50, id=1) │
│ ⚠️ 警告: Claude Pro 企业账号 #3 (priority=100, id=2) 已禁用 │
│   原因: 连接超时                                     │
│   禁用时间: 3分钟前                                  │
│ 心跳: ████████████████████████████████             │
└─────────────────────────────────────────────────────┘
```

### 4.2 普通用户视图（方案 B）

```
┌─────────────────────────────────────────────┐
│ default                          🟡 运行降级  │
│ 可用率: 92.31%  |  延迟: 1147ms             │
│ 当前优先级: 50                              │
│ ⚠️ 系统提示: 优先级 100 的渠道暂时不可用      │
│ 心跳: ████████████████████████████████     │
└─────────────────────────────────────────────┘
```

### 4.3 普通用户视图（方案 C - 推荐）

```
┌─────────────────────────────────────────────┐
│ default                          🟡 运行降级  │
│ 可用率: 92.31%  |  延迟: 1147ms             │
│ 代表渠道: 渠道 #2 (priority=50)             │
│ ⚠️ 系统提示: 渠道 #1 (priority=100) 暂时不可用 │
│ 心跳: ████████████████████████████████     │
└─────────────────────────────────────────────┘
```

---

## 5. 后端实现要点

### 5.1 权限检查

```go
func GetGroupMonitorStatus(c *gin.Context) {
    // 通过 middleware.AdminAuth() 已确保是管理员

    isAdmin := true  // 当前请求来自管理员
    groups := getGroupMonitorData(isAdmin)
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "data": groups,
    })
}
```

### 5.2 数据脱敏函数

```go
type GroupMonitorResult struct {
    Group                         string                     `json:"group"`
    Status                        string                     `json:"status"`
    RepresentativeChannel         ChannelInfo                `json:"representative_channel"`
    DisabledHigherPriorityChannels []DisabledChannelInfo     `json:"disabled_higher_priority_channels"`
    Uptime24h                     float64                    `json:"uptime_24h"`
    AvgLatency                    int                        `json:"avg_latency"`
    Heartbeats                    []HeartbeatRecord          `json:"heartbeats"`
}

type ChannelInfo struct {
    ID           *int    `json:"id,omitempty"`            // 管理员可见
    Name         *string `json:"name,omitempty"`          // 管理员可见
    DisplayName  string  `json:"display_name,omitempty"`  // 普通用户可见
    Priority     int64   `json:"priority"`
    ResponseTime int     `json:"response_time"`
    Status       int     `json:"status"`
}

func buildChannelInfo(channel *model.Channel, priority int64, isAdmin bool, displayName string) ChannelInfo {
    info := ChannelInfo{
        Priority:     priority,
        ResponseTime: channel.ResponseTime,
        Status:       channel.Status,
    }
    
    if isAdmin {
        info.ID = &channel.Id
        info.Name = &channel.Name
    } else {
        info.DisplayName = displayName  // "渠道 #1"
    }
    
    return info
}
```

### 5.3 编号生成逻辑（方案 C）

```go
// 对分组内渠道按优先级降序排序，生成编号
func generateChannelDisplayNames(abilities []model.Ability) map[int]string {
    // 按 priority 降序排序
    sort.Slice(abilities, func(i, j int) bool {
        if abilities[i].Priority == abilities[j].Priority {
            return abilities[i].ChannelId < abilities[j].ChannelId  // 同优先级按ID排序
        }
        return *abilities[i].Priority > *abilities[j].Priority
    })
    
    displayNames := make(map[int]string)
    for idx, ability := range abilities {
        displayNames[ability.ChannelId] = fmt.Sprintf("渠道 #%d", idx+1)
    }
    return displayNames
}
```

---

## 6. 推荐方案总结

**推荐：方案 C（分组内编号 + 管理员完整信息）**

| 字段 | 管理员 | 普通用户 |
|------|--------|---------|
| 渠道 ID | ✅ 显示 | ❌ 隐藏 |
| 渠道名称 | ✅ 显示 | ❌ 隐藏 |
| 渠道编号 | ✅ 显示 | ✅ 显示（"渠道 #1"） |
| 优先级 | ✅ 显示 | ✅ 显示 |
| 响应时间 | ✅ 显示 | ✅ 显示 |
| 心跳历史 | ✅ 显示 | ✅ 显示 |
| 错误详情 | ✅ 显示 | ❌ 隐藏 |

**优点**：
1. 管理员可以看到完整信息，便于运维调试
2. 普通用户可以知道系统状态（分组可用率、延迟），但不知道具体供应商
3. "渠道 #1" 比 "priority=100" 更友好，普通用户也能理解优先级关系
4. 安全性：不泄露渠道名称、供应商信息

---

## 7. 路由与中间件配置

```go
// router/api-router.go

// 管理员接口
channelRoute.GET("/group-monitor", middleware.AdminAuth(), controller.GetGroupMonitorStatus)

// 如果未来需要普通用户也能看（带脱敏）
channelRoute.GET("/group-monitor/public", middleware.UserAuth(), controller.GetGroupMonitorStatusPublic)
```

---

## 8. 是否允许普通用户访问分组监控？

### 选项 A：仅管理员（推荐）
- 分组监控是运维工具，普通用户不需要
- 避免泄露系统内部信息
- 简化实现

### 选项 B：开放给普通用户（带脱敏）
- 用户可以看到自己使用的分组健康状况
- 透明度更高，用户体验更好
- 需要实现脱敏逻辑

**建议**：
- 一期：仅管理员
- 二期：根据需求考虑是否开放给普通用户（带脱敏）
