# 渠道状态变更邮件通知配置

## 1. 需求说明

当渠道测试后自动开启或关闭时，是否给管理员发送邮件通知。

**现状**：
- 渠道自动禁用时，`DisableChannel()` 会调用 `NotifyRootUser()` 发送邮件
- 渠道自动启用时，`EnableChannel()` 会调用 `NotifyRootUser()` 发送邮件
- **没有开关控制，默认总是发送**

**问题**：
- 某些环境下渠道频繁禁用/启用，邮件过多
- 管理员可能更关注分组监控页面，不需要邮件提醒

---

## 2. 配置项设计

### 2.1 新增全局变量

**文件**：`common/constants.go`

```go
// 新增：控制渠道状态变更时是否发送邮件通知
var NotifyOnChannelStatusChange = true  // 默认 true（保持现有行为）
```

### 2.2 配置项元数据

**文件**：`model/option.go`

在 `case` 分支中新增：

```go
case "NotifyOnChannelStatusChange":
    common.NotifyOnChannelStatusChange = boolValue
```

**配置键名**：`NotifyOnChannelStatusChange`
**类型**：布尔值
**默认值**：`true`（发送邮件，保持向后兼容）

---

## 3. 前端配置页面

### 3.1 页面位置

**路径**：系统设置 → 运维 → 监控与报警

### 3.2 UI 布局

```
┌─────────────────────────────────────────┐
│ 监控与报警                               │
├─────────────────────────────────────────┤
│                                         │
│ ☑ 启用自动禁用渠道                       │
│ ☑ 启用自动启用渠道                       │
│ 渠道禁用阈值: [10] 秒                    │
│ 自动测试渠道间隔: [30] 分钟              │
│                                         │
│ ☑ 渠道状态变更时发送邮件通知              │  ← 新增
│   当渠道自动开启或关闭时，给管理员发送邮件 │
│                                         │
├─────────────────────────────────────────┤  ← 分隔线
│                                         │
│ 分组监控渠道选择                          │
│ ...                                     │
└─────────────────────────────────────────┘
```

### 3.3 前端代码（React/TypeScript）

**文件**：`web/default/src/features/system-settings/operations/monitoring-settings-section.tsx`

```tsx
import { FormField } from '@/components/ui/form'
import { Switch } from '@/components/ui/switch'

<FormField
  label="渠道状态变更时发送邮件通知"
  description="当渠道自动开启或关闭时，给管理员发送邮件提醒"
>
  <Switch
    checked={settings.NotifyOnChannelStatusChange}
    onCheckedChange={(checked) =>
      handleSettingChange('NotifyOnChannelStatusChange', checked)
    }
  />
</FormField>
```

---

## 4. 后端实现修改

### 4.1 修改 `DisableChannel` 函数

**文件**：`service/channel.go`

```go
func DisableChannel(channelError types.ChannelError, reason string) {
	common.SysLog(fmt.Sprintf("通道「%s」（#%d）发生错误，准备禁用，原因：%s", channelError.ChannelName, channelError.ChannelId, common.LocalLogPreview(reason)))

	// 检查是否启用自动禁用功能
	if !channelError.AutoBan {
		common.SysLog(fmt.Sprintf("通道「%s」（#%d）未启用自动禁用功能，跳过禁用操作", channelError.ChannelName, channelError.ChannelId))
		return
	}

	success := model.UpdateChannelStatus(channelError.ChannelId, channelError.UsingKey, common.ChannelStatusAutoDisabled, reason)
	if success {
		// 新增：检查是否启用邮件通知
		if common.NotifyOnChannelStatusChange {
			subject := fmt.Sprintf("通道「%s」（#%d）已被禁用", channelError.ChannelName, channelError.ChannelId)
			content := fmt.Sprintf("通道「%s」（#%d）已被禁用，原因：%s", channelError.ChannelName, channelError.ChannelId, reason)
			NotifyRootUser(formatNotifyType(channelError.ChannelId, common.ChannelStatusAutoDisabled), subject, content)
		} else {
			common.SysLog(fmt.Sprintf("通道「%s」（#%d）已被禁用，但邮件通知已关闭", channelError.ChannelName, channelError.ChannelId))
		}
	}
}
```

### 4.2 修改 `EnableChannel` 函数

**文件**：`service/channel.go`

```go
func EnableChannel(channelId int, usingKey string, channelName string) {
	success := model.UpdateChannelStatus(channelId, usingKey, common.ChannelStatusEnabled, "")
	if success {
		// 新增：检查是否启用邮件通知
		if common.NotifyOnChannelStatusChange {
			subject := fmt.Sprintf("通道「%s」（#%d）已被启用", channelName, channelId)
			content := fmt.Sprintf("通道「%s」（#%d）已被启用", channelName, channelId)
			NotifyRootUser(formatNotifyType(channelId, common.ChannelStatusEnabled), subject, content)
		} else {
			common.SysLog(fmt.Sprintf("通道「%s」（#%d）已被启用，但邮件通知已关闭", channelName, channelId))
		}
	}
}
```

---

## 5. 配置选项表（options 表）

### 5.1 初始化默认值

**数据库插入**（如果不存在）：

```sql
INSERT INTO options (key, value) 
VALUES ('NotifyOnChannelStatusChange', 'true')
ON CONFLICT (key) DO NOTHING;
```

### 5.2 前端读取

**API**：`GET /api/option`

返回示例：
```json
{
  "NotifyOnChannelStatusChange": true,
  "AutomaticDisableChannelEnabled": true,
  ...
}
```

### 5.3 前端保存

**API**：`PUT /api/option`

请求体：
```json
{
  "NotifyOnChannelStatusChange": false
}
```

---

## 6. 日志记录

### 6.1 关闭通知时的日志

```
通道「OpenAI官方」（#5）已被禁用，但邮件通知已关闭
通道「Claude Pro」（#3）已被启用，但邮件通知已关闭
```

**目的**：
- 管理员可以在日志中看到状态变更记录
- 即使不发邮件，也能事后追溯

---

## 7. 测试场景

### 7.1 开启通知（默认行为）

1. 配置：`NotifyOnChannelStatusChange = true`
2. 渠道测试失败 → 自动禁用
3. **预期**：管理员收到邮件"通道已被禁用"
4. 渠道测试成功 → 自动启用
5. **预期**：管理员收到邮件"通道已被启用"

### 7.2 关闭通知

1. 配置：`NotifyOnChannelStatusChange = false`
2. 渠道测试失败 → 自动禁用
3. **预期**：不发送邮件，但日志记录"已被禁用，但邮件通知已关闭"
4. 渠道测试成功 → 自动启用
5. **预期**：不发送邮件，但日志记录"已被启用，但邮件通知已关闭"

---

## 8. 向后兼容性

### 8.1 旧版本升级

- 默认值为 `true`，保持现有行为（发送邮件）
- 无需数据迁移
- 管理员可手动关闭

### 8.2 新部署

- 默认开启邮件通知
- 管理员根据需要关闭

---

## 9. 与其他通知的关系

### 9.1 不影响其他通知

**仍然发送邮件的场景**：
- 用户注册
- 密码重置
- 额度不足
- 系统升级提醒
- 手动测试渠道结果（非自动测试）

**只控制自动测试导致的状态变更通知**：
- 自动禁用渠道
- 自动启用渠道

### 9.2 与站内通知的关系

如果系统有站内通知功能，建议：
- 关闭邮件时，仍然保留站内通知
- 或新增独立配置项控制站内通知

---

## 10. 实现检查清单

| 步骤 | 文件 | 修改内容 |
|------|------|---------|
| 1 | `common/constants.go` | 新增 `NotifyOnChannelStatusChange` 变量 |
| 2 | `model/option.go` | 新增 case 分支读取配置 |
| 3 | `service/channel.go` | `DisableChannel()` 添加检查 |
| 4 | `service/channel.go` | `EnableChannel()` 添加检查 |
| 5 | 前端配置页面 | 新增 Switch 开关 |
| 6 | 数据库 | 插入默认配置（可选） |
| 7 | 测试 | 验证开启/关闭两种情况 |

---

## 11. 配置项命名说明

**为什么叫 `NotifyOnChannelStatusChange` 而不是 `SendEmailOnChannelStatusChange`？**

- 更通用，未来可能支持其他通知方式（钉钉、Slack、企业微信）
- 与现有 `NotifyRootUser` 函数命名一致

---

## 12. 可选增强（二期考虑）

### 12.1 细分控制

```
☑ 渠道禁用时发送通知
☑ 渠道启用时发送通知
```

分别控制禁用和启用的通知。

### 12.2 通知方式选择

```
渠道状态变更通知方式：
☑ 邮件
☑ 站内消息
☐ 钉钉机器人
☐ 企业微信
```

### 12.3 通知频率限制

```
渠道状态变更通知频率限制：
同一渠道 [10] 分钟内最多发送 [1] 次通知
```

避免渠道频繁抖动导致邮件轰炸。

---

## 13. 总结

| 配置项 | 说明 | 默认值 | 影响范围 |
|--------|------|--------|---------|
| `NotifyOnChannelStatusChange` | 渠道状态变更时是否发送邮件 | `true` | 自动禁用/启用渠道 |

**最小改动原则**：
- 只在 `DisableChannel()` 和 `EnableChannel()` 中添加判断
- 默认值为 `true`，向后兼容
- 日志始终记录，方便事后追溯
