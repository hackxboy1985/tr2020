# 可选保存用户提示词（Prompt Log）功能设计

## 1. 需求概述

针对用户或令牌进行可选配置，将用户的输入提示词（prompt）保存到数据库。**仅保存提示词，不保存响应内容。**

## 2. 设计原则

- **隐私优先**：默认关闭，需要用户或管理员主动开启
- **权限控制**：仅管理员可查看保存的提示词内容
- **不影响主流程**：异步批量写入，不阻塞请求处理
- **存储解耦**：prompt 数据与 logs 表分离，避免日志查询性能受影响

## 3. 配置层级

采用 **全局开关 + 用户级开关 + 令牌级覆盖** 三级配置：

| 层级 | 字段 | 默认值 | 说明 |
|---|---|---|---|
| 全局 | `SavePromptEnabled` | `false` | 主开关，关闭时所有 prompt 都不保存 |
| 用户 | `UserSetting.SavePrompt` | `false` | 用户级默认配置 |
| 令牌 | `Token.SavePrompt` | `false` | 覆盖用户级配置，true 时强制保存 |

优先级：`全局开关 → 令牌设置（true 覆盖） → 用户设置`

### 3.1 决策流程

```
┌──────────────────────────┐
│ SavePromptEnabled?       │──No──→ 不保存
└──────────┬───────────────┘
           │ Yes
           ▼
┌──────────────────────────┐
│ Token.SavePrompt?        │──Yes──→ 保存（令牌覆盖，优先级最高）
└──────────┬───────────────┘
           │ No (false)
           ▼
┌──────────────────────────┐
│ UserSetting.SavePrompt?  │──Yes──→ 保存
└──────────┬───────────────┘
           │ No (false)
           ▼
         不保存
```

> **注意**: Token.SavePrompt 只有 true/false 两个状态，没有"未设置"的第三态。
> false 时回退检查用户设置，true 时直接保存。

## 4. 存储方案

### 4.1 新建 `prompt_logs` 表（方案 B）

独立表，与 `logs` 表分离，通过 `log_id` 关联：

```go
type PromptLog struct {
    Id         int    `gorm:"primaryKey;index:idx_prompt_created_id,priority:2"`
    LogId      int    `gorm:"uniqueIndex"`                              // 关联 logs.id
    PromptText string `gorm:"type:text"`                                 // 提示词内容
    CreatedAt  int64  `gorm:"bigint;index:idx_prompt_created_id,priority:1"` // 复合索引 (created_at, id)
}
```

### 4.2 复合索引说明

- `idx_prompt_created_id` 复合索引 `(created_at, id)`：覆盖按时间范围查询 + ORDER BY id DESC 的场景
- `uniqueIndex` on `log_id`：保证每个日志只关联一个 prompt 记录

### 4.3 文本长度限制

MySQL TEXT 类型最大 65535 字节。程序在写入时自动截断到 64000 字节（安全边界），并确保截断位置不破坏 UTF-8 编码：

```go
func truncatePromptText(text string) string {
    if len(text) <= promptLogMaxTextBytes {
        return text
    }
    truncated := text[:promptLogMaxTextBytes]
    for len(truncated) > 0 && truncated[len(truncated)-1]&0xC0 == 0x80 {
        truncated = truncated[:len(truncated)-1]
    }
    return truncated
}
```

### 4.4 数据流

> **适用范围**: 仅 text/chat/completion 类型的请求会保存 prompt。
> image generation、audio、embedding、rerank 等非文本请求不保存。

```
用户请求 → Controller (relay.go)
    │
    ├─ 解析请求体 → helper.GetAndValidateRequest()
    │      ↓
    ├─ request.GetTokenCountMeta() → types.TokenCountMeta.CombineText
    │   (CombineText 在各 provider 的 DTO 中生成，汇总 messages 文本内容)
    │   (非 text 类型的请求没有 CombineText，不会触发保存)
    │
    ├─ 敏感词检测（如开启，使用相同的 CombineText）
    │
    ├─ 将 CombineText 存入 gin context
    │   c.Set(string(constant.ContextKeyPromptToSave), meta.CombineText)
    │
    ├─ 转发到上游 → 响应返回
    │
    └─ PostTextConsumeQuota() (service/text_quota.go)
        │
        └─ RecordConsumeLog() (model/log.go)
            │
            ├─ LOG_DB.Create(log) → log.Id 可用
            │
            └─ savePrompt(c, log.Id, userId)
                ├─ 检查 common.SavePromptEnabled（全局开关）
                ├─ 检查 ContextKeyTokenSavePrompt（令牌级覆盖）
                ├─ 检查 GetUserSetting().SavePrompt（用户级设置）
                └─ EnqueuePromptLog(logId, promptText)
                       ↓
                   channel 缓冲 → 批量写入 prompt_logs 表
```

### 4.5 批量写入机制

为避免高负载下的数据库写入瓶颈，采用 **channel-based 缓冲批量写入器**：

```go
const (
    promptLogBatchMaxSize    = 100    // 每批最大条数
    promptLogBatchInterval   = 5s     // 最大等待间隔
    promptLogChannelCapacity = 2000   // 缓冲队列大小
)
```

- 请求处理 goroutine 通过 `channel` 非阻塞发送（channel 满时丢弃，不阻塞请求）
- 后台 worker goroutine 定期（每 5s）或积攒到 100 条时批量 `Create`
- 启动时通过 `model.InitPromptLogWriter()` 初始化

### 4.6 清理策略

prompt_logs 与 logs 使用相同的清理时序（时间条件一致，因为两者的 `created_at` 在同一个请求中同时设置）。

**清理方式**: 在 `DeleteOldLog` 中联动清理，无需独立的清理定时器。

```go
func DeleteOldLog(ctx context.Context, targetTimestamp int64, limit int) (int64, error) {
    total, _ := deleteOldLogs(ctx, targetTimestamp, limit)
    if total > 0 {
        // 使用相同的时间条件同步清理 prompt_logs
        promptCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        DeleteOldPromptLog(promptCtx, targetTimestamp, limit)
    }
    return total, nil
}
```

> **注意**: 与 logs 清理集成在同一个定时任务中，无需额外维护独立的清理配置。

### 4.7 数据库兼容性

| 数据库 | 文本限制 | 注意事项 |
|---|---|---|
| **MySQL** | TEXT 最大 64KB，超出截断 | 兼容 |
| **SQLite** | 无限制 | 兼容 |
| **PostgreSQL** | 无限制 | ⚠️ 不支持 `DELETE ... LIMIT` |

#### PostgreSQL 清理兼容性

当前 `DeleteOldPromptLog` 使用 `DELETE ... LIMIT` 分批删除，这在 PostgreSQL 中语法不支持。

**解决方案**: 检测 PostgreSQL 时改用子查询方式：

```go
func DeleteOldPromptLog(ctx context.Context, targetTimestamp int64, limit int) (int64, error) {
    for {
        if ctx.Err() != nil {
            return total, ctx.Err()
        }
        var ids []int
        tx := LOG_DB.Model(&PromptLog{}).
            Where("created_at < ?", targetTimestamp).
            Limit(limit).
            Select("id")
        if common.LogSqlType == common.DatabaseTypePostgreSQL || common.UsingPostgreSQL {
            // PostgreSQL 不支持 DELETE ... LIMIT，改用子查询
            tx.Find(&ids)
            if len(ids) == 0 {
                break
            }
            result := LOG_DB.Where("id IN ?", ids).Delete(&PromptLog{})
        } else {
            result := LOG_DB.Where("created_at < ?", targetTimestamp).Limit(limit).Delete(&PromptLog{})
        }
    }
}
```

## 5. 权限控制

- **查询接口**：仅管理员可调用 `GET /api/log/prompt/:logId`
- **用户日志列表**：普通用户查看日志时不返回 prompt 内容
- **管理员日志列表**：可通过附带 `SearchPromptLogsByLogIds` 批量查询

## 6. API 设计

### 6.1 管理端接口

| 方法 | 路径 | 说明 | 权限 |
|---|---|---|---|
| `GET` | `/api/log/prompt/:logId` | 获取指定日志的 prompt 内容 | 管理员 |
| `GET` | `/api/log/prompts` | 批量查询 prompt（按时间范围） | 管理员 |

### 6.2 日志列表关联

管理员查询日志列表时，在返回数据中附带 `prompt_text` 字段。

## 7. 前端改动（需在 web 前端仓库实现）

> **注意**: 当前仓库 `web/` 目录为空，以下改动需在单独的前端仓库中实现。

### 7.1 管理后台 → 运营设置 → 日志设置（全局开关）

**文件**: `SettingsLog.jsx`（操作设置 → 日志 tab）

在 `LogConsumeEnabled` 开关下方添加 `SavePromptEnabled` 开关：

```jsx
<Form.Item field="SavePromptEnabled" label="保存提示词内容">
  <Switch />
  <Typography.Text>开启后允许按用户/令牌配置保存请求提示词</Typography.Text>
</Form.Item>
```

### 7.2 个人设置 → 隐私设置（用户级开关）

**文件**: `NotificationSettings.jsx` 中的隐私设置 tab

在 `recordIpLog` 开关下方添加 `save_prompt` 开关：

```jsx
<Form.Item field="save_prompt" label="保存提示词">
  <Switch />
  <Typography.Text>开启后此账号的请求提示词将被保存（仅管理员可见）</Typography.Text>
</Form.Item>
```

后端对应 request body 字段：`SavePrompt bool \`json:"save_prompt"\``

### 7.3 令牌编辑 → 访问限制（令牌级覆盖）

**文件**: `EditTokenModal.jsx`

在"访问限制"区域（IP 白名单下方）添加开关：

```jsx
<Form.Item field="save_prompt" label="保存请求提示词">
  <Switch />
  <Typography.Text>覆盖个人设置，开启后此令牌的请求提示词将被保存</Typography.Text>
</Form.Item>
```

后端对应 Token 字段：`SavePrompt bool \`json:"save_prompt"\``

### 7.4 日志页面 → Prompt 列（管理员查看）

**文件**: `UsageLogsTable.jsx` / `UsageLogsColumnDefs.jsx`

1. 在日志表格中，管理员可见的日志详情区域添加 `prompt_text` 显示
2. 使用可展开的 `<Collapsible>` 组件，默认收起
3. 普通用户调用日志接口时 `prompt_text` 字段为空
4. 管理员日志详情中显示：

```
┌─────────────────────────────────────────────┐
│ ▶ Prompt (点击展开)                           │
│ ───────────────────────────────────────────── │
│ 用户输入的完整提示词内容...                      │
└─────────────────────────────────────────────┘
```

## 8. 影响面

### 8.1 存储
- prompt 占用空间取决于用户请求大小（截断到 64KB）
- 联动 `logs` 表的过期清理策略，同时清理 `prompt_logs`

### 8.2 性能
- 批量异步写入，对请求延迟无影响
- 缓冲 channel 满时丢弃条目，不阻塞主流程
- 复合索引 `(created_at, id)` 保障查询性能

### 8.3 隐私与安全
- **默认关闭**，需主动开启
- **仅管理员可查看**
- 字段在数据库中明文存储（与现有 logs 表策略一致）

## 9. 实现步骤

1. ✅ `dto/user_settings.go` — 添加 `SavePrompt` 字段
2. ✅ `model/token.go` — 添加 `SavePrompt` 字段，更新 `Update()` 方法
3. ✅ `model/prompt_log.go` — PromptLog 模型 + channel 批量写入器 + 查询方法 + 清理方法
4. ✅ `model/main.go` — 迁移注册（主 DB + LOG_DB）
5. ✅ `common/constants.go` — 添加 `SavePromptEnabled` 全局开关
6. ✅ `model/option.go` — 注册全局选项
7. ✅ `middleware/auth.go` — 将 `token.SavePrompt` 存入 gin context
8. ✅ `constant/context_key.go` — 添加 `ContextKeyTokenSavePrompt` 和 `ContextKeyPromptToSave`
9. ✅ `controller/user.go` — 用户设置接口添加 `save_prompt`
10. ✅ `controller/token.go` — 令牌创建/更新接口添加 `SavePrompt`
11. ✅ `controller/relay.go` — 将 `CombineText` 存入 gin context
12. ✅ `model/log.go` — `RecordConsumeLog` 中调用 `savePrompt()` + `savePrompt` 函数
13. ✅ `main.go` — 启动时调用 `InitPromptLogWriter()`
14. ⬜ 前端 UI 改动（用户设置页 + 令牌编辑页 + 日志展示页）
15. ⬜ 管理端 prompt 查询 API 接口

## 9. 优雅退出

服务关闭时，需确保缓冲区的数据被 flush 到数据库，避免数据丢失。

```go
// 关闭 channel → batchLoop 收到 !ok → flush 剩余数据 → 退出
func FlushPromptLogs() {
    promptLogFlushOnce.Do(func() {
        close(promptLogChan)
    })
}
```

在 main.go 中通过 defer 注册：

```go
model.InitPromptLogWriter()
defer model.FlushPromptLogs()
```

## 10. 监控指标

当前实现中，以下行为会产生 SysLog：

| 事件 | 日志级别 | 触发条件 |
|---|---|---|
| 批量写入失败 | common.SysLog | LOG_DB.Create 报错 |
| channel 满丢弃 | common.SysLog | channel buffer 满（2000 条） |

可扩展的 Prometheus 指标设计（待实现）：

```
prompt_log_enqueued_total   — 入队总数（counter）
prompt_log_saved_total      — 成功写入总数（counter）
prompt_log_dropped_total    — 因 channel 满丢弃总数（counter）
prompt_log_errors_total     — 写入失败总数（counter）
prompt_log_buffer_size      — 当前队列长度（gauge）
```

## 11. 已实现 vs 待实现清单

| 分类 | 项目 | 状态 |
|---|---|---|
| 后端模型 | PromptLog 模型 | ✅ |
| 后端模型 | 批量写入器（channel + batch） | ✅ |
| 后端模型 | PostgreSQL DELETE 兼容 | ✅ |
| 后端模型 | 文本截断（64KB） | ✅ |
| 后端模型 | 清理级联（联动 DeleteOldLog） | ✅ |
| 后端模型 | 优雅退出（FlushPromptLogs） | ✅ |
| 后端 API | GET /api/log/prompt/:id | ✅ |
| 后端 API | GET /api/log/ 附带 prompt_text（管理员） | ✅ |
| 后端配置 | 全局开关 SavePromptEnabled | ✅ |
| 后端配置 | 用户级 setting.save_prompt | ✅ |
| 后端配置 | 令牌级 token.save_prompt | ✅ |
| 后端配置 | 令牌 SavePrompt 存入 gin context | ✅ |
| 后端配置 | RecordConsumeLog 联动保存 | ✅ |
| 前端 | 全局开关 → 管理后台日志设置 | ⬜ |
| 前端 | 用户开关 → 个人隐私设置 | ⬜ |
| 前端 | 令牌覆盖 → 令牌编辑页 | ⬜ |
| 前端 | 日志页 prompt 展示（管理员） | ⬜ |
