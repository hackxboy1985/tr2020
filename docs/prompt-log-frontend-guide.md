# Prompt Log 前端实现指南

## 概述

后端接口已完成，前端需要在 4 个页面添加相关配置和展示功能。

---

## 1. 全局开关 → 管理后台日志设置

### 位置
管理后台 → 系统设置 → 运营设置 → 日志设置

### UI 组件
```tsx
<FormGroup>
  <Label>提示词保存设置</Label>
  <Switch
    label="启用提示词保存"
    description="全局主开关，关闭后所有用户和令牌都不会保存提示词"
    checked={settings.SavePromptEnabled}
    onChange={(checked) => updateSetting('SavePromptEnabled', checked)}
  />
  <Alert type="info">
    <p>• 默认关闭，需主动开启</p>
    <p>• 仅保存提示词，不保存响应内容</p>
    <p>• 异步批量写入，不影响性能</p>
    <p>• 需结合用户级或令牌级配置使用</p>
  </Alert>
</FormGroup>
```

### API 调用
```typescript
// 获取配置
GET /api/option

// 更新配置
PUT /api/option
{
  "SavePromptEnabled": true
}
```

### 配置说明
- 这是**最高优先级**的开关
- 关闭时，即使用户或令牌开启了保存，也不会保存
- 开启后，才会检查用户级和令牌级配置

---

## 2. 用户开关 → 个人隐私设置

### 位置
个人中心 → 隐私设置

### UI 组件
```tsx
<Card title="提示词保存设置">
  <Switch
    label="保存我的提示词"
    description="保存请求中的提示词内容用于审计和分析（仅管理员可查看）"
    checked={userSettings.save_prompt}
    onChange={(checked) => updateUserSetting('save_prompt', checked)}
  />
  
  <Alert type="warning" show={userSettings.save_prompt}>
    <p>⚠️ 启用后，您发送的所有提示词将被保存到数据库</p>
    <p>• 仅管理员可以查看保存的提示词</p>
    <p>• 令牌设置可以覆盖此配置</p>
  </Alert>
</Card>
```

### API 调用
```typescript
// 获取用户设置
GET /api/user/setting

// 更新用户设置
PUT /api/user/setting
{
  "save_prompt": true
}
```

### 用户体验
- 默认关闭
- 用户自主控制是否保存自己的提示词
- 可以随时开启或关闭
- 令牌级设置可以覆盖用户级设置

---

## 3. 令牌覆盖 → 令牌编辑页

### 位置
令牌管理 → 编辑令牌 → 访问限制

### UI 组件
```tsx
<FormGroup>
  <Label>提示词保存</Label>
  <Switch
    label="强制保存此令牌的提示词"
    description="开启后将强制保存此令牌的所有请求提示词，不受用户设置影响"
    checked={token.save_prompt}
    onChange={(checked) => setToken({ ...token, save_prompt: checked })}
  />
  
  <Alert type="info" show={token.save_prompt}>
    <p>✓ 此令牌的提示词将被强制保存</p>
    <p>• 优先级：令牌设置 > 用户设置</p>
    <p>• 适用场景：监控特定 API 密钥、审计外部集成</p>
  </Alert>
</FormGroup>
```

### API 调用
```typescript
// 创建令牌
POST /api/token
{
  "name": "API Key 1",
  "save_prompt": true,
  // ... 其他字段
}

// 更新令牌
PUT /api/token/:id
{
  "save_prompt": true
}
```

### 使用场景
- **监控特定 API 密钥**：对可疑或高风险的令牌启用
- **审计外部集成**：对第三方系统使用的令牌启用
- **测试和调试**：临时启用以查看实际请求内容
- **覆盖用户设置**：即使用户关闭了保存，此令牌仍然保存

---

## 4. 日志页 Prompt 展示（管理员）

### 位置
管理后台 → 日志 → 日志列表

### 表格列设计

```tsx
const columns = [
  // ... 现有列
  {
    title: 'Prompt',
    dataIndex: 'prompt_text',
    key: 'prompt_text',
    width: 120,
    render: (text, record) => {
      // 只有 CONSUME 类型的日志才有 prompt
      if (record.type !== 2 || !text) {
        return <span>-</span>;
      }
      
      return (
        <Space>
          <Tag color="blue">
            {text.length > 50 ? `${text.length} 字符` : '短文本'}
          </Tag>
          <Button 
            type="link" 
            size="small"
            onClick={() => showPromptModal(record)}
          >
            查看
          </Button>
        </Space>
      );
    }
  }
];
```

### Prompt 查看弹窗

```tsx
function PromptModal({ log, open, onClose }) {
  const [promptData, setPromptData] = useState(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (open && log?.id) {
      loadPrompt(log.id);
    }
  }, [open, log?.id]);

  async function loadPrompt(logId) {
    setLoading(true);
    try {
      // 如果列表已经返回了 prompt_text，直接使用
      if (log.prompt_text) {
        setPromptData({ prompt_text: log.prompt_text });
      } else {
        // 否则单独查询
        const res = await fetch(`/api/log/prompt/${logId}`);
        const data = await res.json();
        setPromptData(data.data);
      }
    } catch (error) {
      message.error('获取提示词失败');
    } finally {
      setLoading(false);
    }
  }

  return (
    <Modal
      title="提示词内容"
      open={open}
      onCancel={onClose}
      width={800}
      footer={[
        <Button key="copy" onClick={() => copyToClipboard(promptData?.prompt_text)}>
          复制
        </Button>,
        <Button key="close" onClick={onClose}>
          关闭
        </Button>
      ]}
    >
      {loading ? (
        <Spin />
      ) : (
        <>
          <Space direction="vertical" style={{ width: '100%' }}>
            <Alert
              type="info"
              message={
                <Space>
                  <span>用户ID: {log.user_id}</span>
                  <Divider type="vertical" />
                  <span>Token: {log.token_name}</span>
                  <Divider type="vertical" />
                  <span>时间: {formatTime(log.created_at)}</span>
                </Space>
              }
            />
            
            <Card>
              <pre style={{ 
                maxHeight: '500px', 
                overflow: 'auto',
                whiteSpace: 'pre-wrap',
                wordBreak: 'break-word'
              }}>
                {promptData?.prompt_text || '无提示词内容'}
              </pre>
            </Card>

            {promptData?.prompt_text?.length >= 64000 && (
              <Alert
                type="warning"
                message="提示词已截断"
                description="此提示词超过 64KB 限制，已自动截断保存"
              />
            )}
          </Space>
        </>
      )}
    </Modal>
  );
}
```

### 列表数据获取

```typescript
// 日志列表 API
GET /api/log?page=1&page_size=20

// 后端自动返回（管理员）
{
  "code": 200,
  "data": {
    "items": [
      {
        "id": 123,
        "user_id": 1,
        "token_name": "API Key 1",
        "type": 2,  // CONSUME
        "model_name": "gpt-4",
        "prompt_tokens": 100,
        "completion_tokens": 200,
        "prompt_text": "用户的完整提示词...",  // ✅ 自动附加（仅管理员）
        "created_at": 1719820800
      }
    ]
  }
}
```

### 权限控制

```typescript
// 前端：仅管理员显示 Prompt 列
{isAdmin && (
  <Table.Column
    title="Prompt"
    dataIndex="prompt_text"
    render={renderPromptCell}
  />
)}

// 后端：已实现权限控制
// controller/log.go:GetAllLogs() 中
// 只有当 common.SavePromptEnabled=true 时才会附加 prompt_text
// 非管理员调用时不会返回 prompt_text
```

---

## 前端技术栈建议

### React + Ant Design 示例

```tsx
// 1. 全局开关
import { Switch, Alert } from 'antd';

<Form.Item label="启用提示词保存" name="SavePromptEnabled">
  <Switch />
</Form.Item>

// 2. 用户设置
import { Card, Switch, Alert } from 'antd';

<Card title="隐私设置">
  <Form.Item name="save_prompt" valuePropName="checked">
    <Switch />
  </Form.Item>
</Card>

// 3. 令牌编辑
<Form.Item 
  label="强制保存提示词" 
  name="save_prompt" 
  valuePropName="checked"
  tooltip="覆盖用户级设置，强制保存此令牌的提示词"
>
  <Switch />
</Form.Item>

// 4. 日志列表
import { Table, Modal, Button } from 'antd';

<Table 
  columns={columns}
  dataSource={logs}
/>
```

### Vue 3 + Element Plus 示例

```vue
<!-- 1. 全局开关 -->
<el-form-item label="启用提示词保存">
  <el-switch v-model="settings.SavePromptEnabled" />
</el-form-item>

<!-- 2. 用户设置 -->
<el-card title="隐私设置">
  <el-switch 
    v-model="userSettings.save_prompt"
    active-text="保存我的提示词"
  />
</el-card>

<!-- 3. 令牌编辑 -->
<el-form-item label="强制保存提示词">
  <el-switch v-model="token.save_prompt" />
</el-form-item>

<!-- 4. 日志列表 -->
<el-table :data="logs">
  <el-table-column prop="prompt_text" label="Prompt">
    <template #default="{ row }">
      <el-button @click="showPrompt(row)">查看</el-button>
    </template>
  </el-table-column>
</el-table>
```

---

## 开发优先级建议

### P0 - 核心功能（必须实现）
1. ✅ 日志页 Prompt 展示 - 最常用，管理员需要查看
2. ✅ 全局开关 - 控制整个功能的启用

### P1 - 配置功能（重要）
3. ✅ 令牌覆盖设置 - 灵活控制特定令牌
4. ✅ 用户级设置 - 让用户自主控制

### 实现顺序
```
第一阶段：日志页展示 + 全局开关（让管理员能看到和控制）
第二阶段：令牌级配置（让管理员能针对特定令牌监控）
第三阶段：用户级配置（让用户自主控制隐私）
```

---

## 测试检查清单

### 功能测试
- [ ] 全局开关关闭时，用户/令牌开启也不保存
- [ ] 全局开启 + 令牌开启 → 保存
- [ ] 全局开启 + 令牌关闭 + 用户开启 → 保存
- [ ] 全局开启 + 令牌关闭 + 用户关闭 → 不保存
- [ ] 日志列表显示 prompt 标签（管理员）
- [ ] 点击查看显示完整内容
- [ ] 非管理员看不到 prompt 列

### UI 测试
- [ ] 开关状态正确显示
- [ ] 说明文字清晰易懂
- [ ] 警告提示在适当时机显示
- [ ] Prompt 弹窗支持长文本滚动
- [ ] 复制功能正常工作

### 性能测试
- [ ] 日志列表加载速度（含 prompt）
- [ ] 大量数据时的渲染性能
- [ ] Prompt 弹窗加载速度

---

## API 接口汇总

| 功能 | 接口 | 方法 | 说明 |
|------|------|------|------|
| 获取全局配置 | `/api/option` | GET | 返回 SavePromptEnabled |
| 更新全局配置 | `/api/option` | PUT | 更新 SavePromptEnabled |
| 获取用户设置 | `/api/user/setting` | GET | 返回 save_prompt |
| 更新用户设置 | `/api/user/setting` | PUT | 更新 save_prompt |
| 创建令牌 | `/api/token` | POST | 支持 save_prompt 字段 |
| 更新令牌 | `/api/token/:id` | PUT | 支持 save_prompt 字段 |
| 日志列表 | `/api/log` | GET | 自动附加 prompt_text（管理员） |
| 查询单个 Prompt | `/api/log/prompt/:id` | GET | 获取完整 prompt_text |

---

## 注意事项

1. **权限控制**：Prompt 内容仅管理员可见
2. **性能优化**：列表中只显示摘要，点击查看完整内容
3. **文本截断**：超过 64KB 的 prompt 会被截断，需提示用户
4. **隐私提示**：开启保存时显示警告信息
5. **数据清理**：告知用户 prompt 会随日志一起清理

---

## 示例截图参考

### 全局开关
```
┌─────────────────────────────────────────┐
│ 日志设置                                 │
├─────────────────────────────────────────┤
│ 启用提示词保存           [ ON ]         │
│ 全局主开关，关闭后所有用户都不会保存     │
│                                          │
│ ℹ️ 提示：                                │
│ • 默认关闭，需主动开启                   │
│ • 仅保存提示词，不保存响应               │
│ • 异步批量写入，不影响性能               │
└─────────────────────────────────────────┘
```

### 日志列表
```
┌────────────────────────────────────────────────────────────┐
│ ID  │ 用户  │ 模型     │ Token  │ Prompt        │ 时间     │
├────────────────────────────────────────────────────────────┤
│ 123 │ user1 │ gpt-4    │ 100/200│ [256字符] 查看│ 12:30   │
│ 124 │ user2 │ claude-3 │ 150/300│ [512字符] 查看│ 12:31   │
└────────────────────────────────────────────────────────────┘
```

### Prompt 弹窗
```
┌─────────────────────────────────────────────────┐
│ 提示词内容                          [ × ]       │
├─────────────────────────────────────────────────┤
│ ℹ️ 用户ID: 1 | Token: API Key 1 | 12:30       │
├─────────────────────────────────────────────────┤
│ ┌─────────────────────────────────────────────┐ │
│ │ You are a helpful assistant...              │ │
│ │                                             │ │
│ │ User: How to implement...                   │ │
│ │                                             │ │
│ │ (滚动查看更多)                               │ │
│ └─────────────────────────────────────────────┘ │
├─────────────────────────────────────────────────┤
│                            [ 复制 ]   [ 关闭 ]  │
└─────────────────────────────────────────────────┘
```
