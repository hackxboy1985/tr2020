# Prompt Log 功能完整实现总结

## 提交历史

### 1. feat: add prompt log save feature (complete backend implementation)
**Commit**: `0194b9fad`
**时间**: 2024-07-01

**实现内容**:
- ✅ PromptLog 模型和批量写入机制
- ✅ 数据库迁移（自动创建 prompt_logs 表）
- ✅ 三级配置：全局开关 + 用户级 + 令牌级
- ✅ 查询接口和日志列表附加
- ✅ 决策逻辑和清理级联
- ✅ 后端 100% 完成（21/21）

### 2. feat: add SavePromptUserVisible control for prompt log feature
**Commit**: `6c012f93a`
**时间**: 2024-07-01

**增强内容**:
- ✅ 新增 SavePromptUserVisible 全局开关
- ✅ 四级配置系统
- ✅ 强制保存模式（用户不可见）
- ✅ 用户可控模式（用户可见）
- ✅ 前端显示逻辑控制

---

## 最终配置架构

### 四级配置系统

```
Level 1: SavePromptEnabled (全局主开关)
         ↓ false → 不保存任何 prompt
         ↓ true
         
Level 2: SavePromptUserVisible (可见性控制)
         ↓ false → 强制保存所有用户（用户不可见不可控）
         ↓ true → 用户可控模式
         
Level 3: Token.SavePrompt (令牌级覆盖)
         ↓ true → 强制保存此令牌
         ↓ false
         
Level 4: UserSetting.SavePrompt (用户级配置)
         ↓ true → 保存
         ↓ false → 不保存
```

### 配置组合矩阵

| SavePromptEnabled | SavePromptUserVisible | Token.SavePrompt | User.SavePrompt | 结果 |
|-------------------|----------------------|------------------|-----------------|------|
| false | * | * | * | ❌ 不保存 |
| true | false | * | * | ✅ **强制保存** |
| true | true | true | * | ✅ 保存（令牌覆盖） |
| true | true | false | true | ✅ 保存（用户选择） |
| true | true | false | false | ❌ 不保存（用户选择） |

---

## 数据库结构

### prompt_logs 表

```sql
CREATE TABLE `prompt_logs` (
  `id` INT AUTO_INCREMENT PRIMARY KEY,
  `log_id` INT UNIQUE NOT NULL,
  `prompt_text` TEXT,
  `created_at` BIGINT NOT NULL,
  INDEX `idx_prompt_created_id` (`created_at`, `id`)
);
```

### tokens 表新增字段

```sql
ALTER TABLE `tokens` ADD COLUMN `save_prompt` TINYINT(1) DEFAULT 0;
```

### options 表新增配置

```sql
INSERT INTO `options` (`key`, `value`) VALUES 
  ('SavePromptEnabled', 'false'),
  ('SavePromptUserVisible', 'false');
```

---

## API 接口

### 1. 系统状态接口（新增字段）

```http
GET /api/status

响应:
{
  "version": "...",
  "save_prompt_enabled": false,           # 新增
  "save_prompt_user_visible": false,      # 新增
  ...
}
```

### 2. 查询单个 Prompt

```http
GET /api/log/prompt/:id
Authorization: Bearer <admin_token>

响应:
{
  "code": 200,
  "data": {
    "id": 123,
    "log_id": 456,
    "prompt_text": "...",
    "created_at": 1719820800
  }
}
```

### 3. 日志列表（自动附加 prompt_text）

```http
GET /api/log?page=1&page_size=20
Authorization: Bearer <admin_token>

响应:
{
  "code": 200,
  "data": {
    "items": [
      {
        "id": 456,
        "prompt_text": "..."  # 管理员自动附加
      }
    ]
  }
}
```

### 4. 更新用户设置

```http
PUT /api/user/setting
{
  "save_prompt": true
}
```

### 5. 创建/更新令牌

```http
POST /api/token
{
  "name": "API Key",
  "save_prompt": true,
  ...
}

PUT /api/token/:id
{
  "save_prompt": true
}
```

---

## 前端实现指南

### 1. 管理后台 - 全局设置

**位置**: 系统设置 → 运营设置 → 日志设置

```tsx
<Card title="提示词保存设置">
  <Form.Item label="启用提示词保存" name="SavePromptEnabled">
    <Switch />
  </Form.Item>

  {settings.SavePromptEnabled && (
    <Form.Item 
      label="用户提示词开关显示" 
      name="SavePromptUserVisible"
      tooltip="控制用户是否能看到和设置 prompt 保存选项"
    >
      <Switch />
    </Form.Item>
  )}

  <Alert type="info">
    {!settings.SavePromptEnabled ? (
      <p>• 提示词保存功能已关闭</p>
    ) : settings.SavePromptUserVisible ? (
      <>
        <p>• ✅ 用户可见模式：用户可以在个人设置中控制是否保存</p>
        <p>• 令牌页面的保存选项对所有用户可见</p>
      </>
    ) : (
      <>
        <p>• 🔒 强制保存模式：自动保存所有用户的提示词</p>
        <p>• 用户无法看到和修改保存设置</p>
        <p>• 令牌页面的保存选项仅管理员可见</p>
      </>
    )}
  </Alert>
</Card>
```

---

### 2. 个人隐私设置（条件显示）

**位置**: 个人中心 → 隐私设置

```tsx
function PrivacySettings() {
  const [systemSettings, setSystemSettings] = useState({});
  
  useEffect(() => {
    // 从 /api/status 获取系统配置
    fetch('/api/status')
      .then(res => res.json())
      .then(data => setSystemSettings(data));
  }, []);

  // 仅当 SavePromptUserVisible = true 时显示
  const showPromptSetting = 
    systemSettings.save_prompt_enabled && 
    systemSettings.save_prompt_user_visible;

  return (
    <Card title="隐私设置">
      {showPromptSetting && (
        <>
          <Form.Item label="保存我的提示词" name="save_prompt">
            <Switch />
          </Form.Item>
          
          <Alert type="warning">
            <p>⚠️ 启用后，您发送的所有提示词将被保存到数据库</p>
            <p>• 仅管理员可以查看保存的提示词</p>
            <p>• 令牌设置可以覆盖此配置</p>
          </Alert>
        </>
      )}
    </Card>
  );
}
```

---

### 3. 令牌编辑页（条件显示）

**位置**: 令牌管理 → 编辑令牌

```tsx
function TokenEditForm({ isAdmin }) {
  const [systemSettings, setSystemSettings] = useState({});
  
  useEffect(() => {
    fetch('/api/status')
      .then(res => res.json())
      .then(data => setSystemSettings(data));
  }, []);

  // 显示条件：
  // 1. 管理员：总是显示
  // 2. 普通用户：仅当 SavePromptUserVisible = true 时显示
  const showPromptSetting = 
    isAdmin || 
    (systemSettings.save_prompt_enabled && 
     systemSettings.save_prompt_user_visible);

  return (
    <Form>
      {showPromptSetting && (
        <Form.Item label="强制保存提示词" name="save_prompt">
          <Switch />
        </Form.Item>
      )}
    </Form>
  );
}
```

---

### 4. 日志列表（管理员）

**位置**: 管理后台 → 日志 → 日志列表

```tsx
const columns = [
  // ... 其他列
  {
    title: 'Prompt',
    dataIndex: 'prompt_text',
    render: (text, record) => {
      if (record.type !== 2 || !text) return <span>-</span>;
      
      return (
        <Space>
          <Tag color="blue">
            {text.length} 字符
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

---

## 使用场景

### 场景 1: 企业级强制审计

**配置**:
```json
{
  "SavePromptEnabled": true,
  "SavePromptUserVisible": false
}
```

**效果**:
- ✅ 自动保存所有用户的提示词
- ❌ 用户无法看到和修改保存设置
- ❌ 令牌页面的保存选项仅管理员可见
- ✅ 适用于：企业内部审计、合规要求

---

### 场景 2: SaaS 用户自主控制

**配置**:
```json
{
  "SavePromptEnabled": true,
  "SavePromptUserVisible": true
}
```

**效果**:
- ✅ 用户可以在个人设置中控制是否保存
- ✅ 令牌页面的保存选项对所有用户可见
- ✅ 管理员可以通过令牌设置强制保存特定 API Key
- ✅ 适用于：多租户 SaaS、用户隐私控制

---

### 场景 3: 特定令牌监控

**配置**:
```json
{
  "SavePromptEnabled": true,
  "SavePromptUserVisible": true,
  "Token.SavePrompt": true  // 特定令牌
}
```

**效果**:
- ✅ 此令牌的所有请求强制保存
- ✅ 不受用户设置影响
- ✅ 适用于：监控外部集成、调试特定 API Key

---

## 数据量估算

**假设**:
- 每天 10 万次请求
- 50% 开启 prompt 保存
- 每条平均 2 KB

**数据量**:
- 每天：~100 MB
- 30 天：~3 GB
- 1 年：~36 GB

**建议**:
- 定期清理旧数据（已实现自动清理）
- 监控 prompt_logs 表大小
- 考虑使用独立的 LOG_DB

---

## 验证测试

### 测试 1: 强制保存模式

```bash
# 配置
curl -X PUT /api/option \
  -H "Authorization: Bearer <admin_token>" \
  -d '{"SavePromptEnabled": "true", "SavePromptUserVisible": "false"}'

# 发起请求（任意用户）
curl -X POST /api/v1/chat/completions \
  -H "Authorization: Bearer <user_token>" \
  -d '{"model": "gpt-4", "messages": [{"role": "user", "content": "test"}]}'

# 检查数据库
SELECT * FROM prompt_logs ORDER BY id DESC LIMIT 1;
# 应该看到保存的 prompt
```

---

### 测试 2: 用户可控模式

```bash
# 配置
curl -X PUT /api/option \
  -H "Authorization: Bearer <admin_token>" \
  -d '{"SavePromptEnabled": "true", "SavePromptUserVisible": "true"}'

# 用户关闭保存
curl -X PUT /api/user/setting \
  -H "Authorization: Bearer <user_token>" \
  -d '{"save_prompt": false}'

# 发起请求
curl -X POST /api/v1/chat/completions \
  -H "Authorization: Bearer <user_token>" \
  -d '{"model": "gpt-4", "messages": [{"role": "user", "content": "test"}]}'

# 检查数据库
SELECT * FROM prompt_logs WHERE log_id = <log_id>;
# 应该没有记录
```

---

### 测试 3: 令牌覆盖

```bash
# 令牌设置强制保存
curl -X PUT /api/token/<token_id> \
  -H "Authorization: Bearer <admin_token>" \
  -d '{"save_prompt": true}'

# 发起请求（即使用户关闭了保存）
curl -X POST /api/v1/chat/completions \
  -H "Authorization: Bearer <token_with_save_prompt>" \
  -d '{"model": "gpt-4", "messages": [{"role": "user", "content": "test"}]}'

# 检查数据库
SELECT * FROM prompt_logs WHERE log_id = <log_id>;
# 应该看到保存的 prompt（令牌覆盖生效）
```

---

## 文档列表

1. **docs/prompt-log-design.md** - 原始设计文档（三级配置）
2. **docs/prompt-log-implementation-check.md** - 实现检查报告
3. **docs/prompt-log-enhanced-design.md** - 增强设计文档（四级配置）
4. **docs/prompt-log-frontend-guide.md** - 前端实现指南

---

## 总结

### 后端实现状态

| 项目 | 状态 | 说明 |
|------|------|------|
| PromptLog 模型 | ✅ 100% | 完整实现 |
| 批量写入机制 | ✅ 100% | channel + batch |
| 数据库迁移 | ✅ 100% | 自动创建表 |
| 四级配置系统 | ✅ 100% | 全局 + 可见性 + 令牌 + 用户 |
| 决策逻辑 | ✅ 100% | 完整决策流程 |
| 查询接口 | ✅ 100% | 单个查询 + 列表附加 |
| 清理级联 | ✅ 100% | 联动清理 |
| 优雅退出 | ✅ 100% | FlushPromptLogs |
| API 接口 | ✅ 100% | 所有接口完成 |

### 前端实现状态

| 项目 | 状态 | 说明 |
|------|------|------|
| 全局设置 UI | ⬜ 待开发 | 2个开关 + 说明 |
| 个人设置 UI | ⬜ 待开发 | 条件显示 |
| 令牌设置 UI | ⬜ 待开发 | 条件显示 |
| 日志列表 UI | ⬜ 待开发 | Prompt 列 + 弹窗 |

### 总体进度

- **后端**: ✅ 100% 完成
- **前端**: ⬜ 0% 完成
- **文档**: ✅ 100% 完成

---

## 下一步

1. **前端开发**：根据 `docs/prompt-log-frontend-guide.md` 实现 4 个 UI 功能
2. **测试验证**：完整测试三种场景（强制保存、用户可控、令牌覆盖）
3. **性能监控**：监控 prompt_logs 表大小和批量写入性能
4. **文档更新**：补充用户使用手册和管理员操作指南
