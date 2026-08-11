# 分组监控实现说明（与设计文档的差异）

## 1. 接口路由变更

### 设计文档
```
GET /api/channel/group-monitor  （AdminAuth）
```

### 实际实现
```
GET /api/user/group-monitor  （UserAuth，在 selfRoute 下）
```

**原因**：普通用户也需要查看自己有权访问的分组监控状态，接口内部根据角色返回不同数据：
- 管理员：返回所有配置要展示的分组，渠道名/ID 完整返回
- 普通用户：用 `GetUserUsableGroups(userGroup)` 过滤，只返回该用户有权访问的分组，渠道名/ID 脱敏

管理员专属接口（仍在 AdminAuth 下）：
```
GET  /api/channel/groups              # 获取所有分组列表（配置页面用）
POST /api/channel/group-monitor-config  # 保存分组可见性配置
```

---

## 2. 分组监控配置含义修正

### 设计文档（旧版 channel-selection-config.md）
~~选择分组内的哪些渠道参与监控~~

### 实际实现（channel-selection-config-revised.md）
**选择哪些分组在监控页面展示**，不影响渠道测试逻辑。

配置存储：
- **key**：`GroupMonitorVisibleGroups`
- **value**：逗号分隔的分组名字符串（如 `default,gpt-plus`）
- **未配置时**：展示所有分组

---

## 3. 用户请求写入历史（新增功能）

设计文档 `test-optimization.md` 中规划的功能已实现：

### 写入触发点
`controller/relay.go` 第 229 行（请求成功返回前）：
```go
if newAPIError == nil {
    // 30秒内每渠道最多写一次（sync.Map 限流）
    if shouldWriteChannelTestHistory(channel.Id) {
        go func(chId, rt int) {
            logger.LogInfo(c, fmt.Sprintf("渠道 #%d 用户请求成功（%dms），写入测试历史", chId, rt))
            model.RecordChannelTestHistory(chId, true, rt)
        }(channel.Id, responseTime)
    }
    return
}
```

### 限流方案
- **方案**：`sync.Map`（进程内）
- **窗口**：30 秒
- **多机影响**：2 台机器最多写 2 次，可接受
- **异步写入**：`go func(...)` 不阻塞用户请求

### 定时测试跳过窗口
- **设计文档**：20 秒
- **实际实现**：**30 秒**（与限流窗口保持一致，避免边界情况）

---

## 4. 侧边栏配置

"分组监控"是独立菜单项，位于侧边栏"数据看板"下方，**不在数据看板子菜单内**。

### 涉及文件
| 文件 | 改动 |
|------|------|
| `use-sidebar-data.ts` | 新增"分组监控"菜单项（`/dashboard/group-monitor`，Heart 图标） |
| `use-sidebar-config.ts` | 注册 `group_monitor` 配置 key 和路由映射 |
| `config.ts` | `SIDEBAR_MODULES_DEFAULT` 加 `group_monitor: { enabled: true, adminOnly: true }` |
| `sidebar-modules-section.tsx` | 配置页面加 `group_monitor` 的中文描述 |

### 侧边栏模块配置页面
在"数据看板（detail）"配置项后面新增"分组监控（group_monitor）"开关，默认开启，adminOnly。

---

## 5. 心跳格颜色与渠道数量

### 颜色层级（与 heartbeat-color-rules.md 一致）

| 渠道数量 | 可能颜色 |
|---------|---------|
| 1 个 | 🟢 绿、🔴 红 |
| 2 个 | 🟢 绿、🟡 黄、🔴 红 |
| 3 个及以上 | 🟢 绿、🟡 黄、🟠 橙、🔴 红 |

**判断依据**：前端图例根据 `top_channels.length` 动态显示，不是固定 4 色。

---

## 6. 渠道编号生成

设计文档中提到"渠道 #1、#2..."，实际按分组内所有渠道（包括 disabled）按 `priority DESC, channel_id ASC` 排序编号，编号固定，不随启用状态变化。

---

## 7. 邮件通知开关

### 配置项
- **key**：`NotifyOnChannelStatusChange`
- **类型**：bool
- **默认值**：`true`（向后兼容，保持原有发邮件行为）

### 影响范围
仅控制 `DisableChannel()` 和 `EnableChannel()` 中的 `NotifyRootUser()` 调用，不影响其他通知。

---

## 8. 完整文件改动清单

### 后端
| 文件 | 说明 |
|------|------|
| `common/constants.go` | 新增 `NotifyOnChannelStatusChange = true` |
| `model/option.go` | 新增 `NotifyOnChannelStatusChange` case、`GetGroupMonitorVisibleGroups`、`SetGroupMonitorVisibleGroups` |
| `model/channel_test_history.go` | 新增表模型、写入/查询函数 |
| `model/main.go` | AutoMigrate 注册 `ChannelTestHistory` |
| `service/channel.go` | `DisableChannel`/`EnableChannel` 加邮件通知开关判断 |
| `controller/relay.go` | 用户请求成功时异步写入测试历史（30秒限流） |
| `controller/channel-test.go` | 定时测试前检查 30 秒内是否有成功请求，有则跳过；测试后写入历史 |
| `controller/group_monitor.go` | 新增：`GetAllGroups`、`SaveGroupMonitorConfig`、`GetGroupMonitorStatus` |
| `router/api-router.go` | 注册路由 |

### 前端
| 文件 | 说明 |
|------|------|
| `system-settings/types.ts` | 新增 `NotifyOnChannelStatusChange` 字段 |
| `system-settings/operations/section-registry.tsx` | 传递新字段 |
| `system-settings/integrations/monitoring-settings-section.tsx` | 新增邮件通知开关 + 分组选择区域 |
| `system-settings/maintenance/config.ts` | 新增 `group_monitor` 默认配置 |
| `system-settings/maintenance/sidebar-modules-section.tsx` | 新增 `group_monitor` meta 描述 |
| `dashboard/section-registry.tsx` | 新增 `group-monitor` section |
| `dashboard/index.tsx` | 渲染 `group-monitor` 面板 |
| `dashboard/components/group-monitor/group-monitor-panel.tsx` | 新建：分组监控展示页面 |
| `hooks/use-sidebar-data.ts` | 新增"分组监控"菜单项 |
| `hooks/use-sidebar-config.ts` | 注册 `group_monitor` 配置和路由映射 |
