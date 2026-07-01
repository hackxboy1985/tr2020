# Prompt Log 功能增强设计（增加用户可见性控制）

## 需求变更

在原有的三级配置基础上，增加"用户提示词开关显示"控制，实现：
1. 控制用户是否能看到和设置 save_prompt 选项
2. 关闭时强制所有用户 save_prompt = true
3. 控制令牌页面的 save_prompt 选项可见性

---

## 新增配置层级

采用 **4 层配置**：

| 层级 | 字段 | 默认值 | 说明 |
|---|---|---|---|
| 全局主开关 | `SavePromptEnabled` | `false` | 主开关，关闭时所有 prompt 都不保存 |
| **全局可见性** | **`SavePromptUserVisible`** | **`false`** | **控制用户是否能看到和设置 prompt 保存选项** |
| 用户级 | `UserSetting.SavePrompt` | `false` | 用户级默认配置（仅当 SavePromptUserVisible=true 时生效） |
| 令牌级 | `Token.SavePrompt` | `false` | 令牌级配置 |

---

## 决策流程（新版）

```
┌────────────────────────────┐
│ SavePromptEnabled?         │──No──→ 不保存
└──────────┬─────────────────┘
           │ Yes
           ▼
┌────────────────────────────┐
│ SavePromptUserVisible?     │──No──→ 强制保存所有用户 ✅
└──────────┬─────────────────┘        （用户无法控制）
           │ Yes (用户可控制)
           ▼
┌────────────────────────────┐
│ Token.SavePrompt?          │──Yes──→ 保存 ✅
└──────────┬─────────────────┘
           │ No (false)
           ▼
┌────────────────────────────┐
│ UserSetting.SavePrompt?    │──Yes──→ 保存 ✅
└──────────┬─────────────────┘
           │ No (false)
           ▼
         不保存 ❌
```

---

## 配置组合说明

| SavePromptEnabled | SavePromptUserVisible | 行为 |
|-------------------|----------------------|------|
| `false` | 任意 | ❌ 不保存任何 prompt |
| `true` | `false` | ✅ **强制保存所有用户**，用户不可见不可控 |
| `true` | `true` | ✅ 用户可见可控，根据用户/令牌设置决定 |

---

## 前端 UI 变更

### 1. 管理后台 - 全局设置（新增开关）

```tsx
<Card title="提示词保存设置">
  {/* 主开关 */}
  <Form.Item 
    label="启用提示词保存" 
    name="SavePromptEnabled"
    tooltip="全局主开关，关闭后所有用户都不会保存提示词"
  >
    <Switch />
  </Form.Item>

  {/* 新增：可见性控制 */}
  {settings.SavePromptEnabled && (
    <Form.Item 
      label="用户提示词开关显示" 
      name="SavePromptUserVisible"
      tooltip="控制用户是否能看到和设置 prompt 保存选项"
    >
      <Switch />
    </Form.Item>
  )}

  {/* 说明 */}
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

```tsx
function PrivacySettings() {
  const [systemSettings, setSystemSettings] = useState({});
  
  useEffect(() => {
    // 获取系统配置
    fetchSystemSettings().then(setSystemSettings);
  }, []);

  // 仅当 SavePromptUserVisible = true 时显示
  const showPromptSetting = 
    systemSettings.SavePromptEnabled && 
    systemSettings.SavePromptUserVisible;

  return (
    <Card title="隐私设置">
      {/* 其他设置... */}
      
      {showPromptSetting && (
        <Form.Item 
          label="保存我的提示词" 
          name="save_prompt"
          tooltip="保存请求中的提示词内容用于审计和分析（仅管理员可查看）"
        >
          <Switch />
        </Form.Item>
      )}

      {showPromptSetting && (
        <Alert type="warning">
          <p>⚠️ 启用后，您发送的所有提示词将被保存到数据库</p>
          <p>• 仅管理员可以查看保存的提示词</p>
          <p>• 令牌设置可以覆盖此配置</p>
        </Alert>
      )}
    </Card>
  );
}
```

---

### 3. 令牌编辑页（条件显示）

```tsx
function TokenEditForm({ isAdmin }) {
  const [systemSettings, setSystemSettings] = useState({});
  
  useEffect(() => {
    fetchSystemSettings().then(setSystemSettings);
  }, []);

  // 显示条件：
  // 1. 管理员：总是显示
  // 2. 普通用户：仅当 SavePromptUserVisible = true 时显示
  const showPromptSetting = 
    isAdmin || 
    (systemSettings.SavePromptEnabled && systemSettings.SavePromptUserVisible);

  return (
    <Form>
      {/* 其他字段... */}
      
      {showPromptSetting && (
        <Form.Item 
          label="强制保存提示词" 
          name="save_prompt"
          tooltip={
            isAdmin 
              ? "管理员权限：强制保存此令牌的所有请求提示词" 
              : "开启后将强制保存此令牌的所有请求提示词，不受用户设置影响"
          }
        >
          <Switch disabled={!isAdmin && !systemSettings.SavePromptUserVisible} />
        </Form.Item>
      )}

      {showPromptSetting && (
        <Alert type="info">
          <p>• 优先级：令牌设置 > 用户设置</p>
          {isAdmin && <p>• 管理员可以为任何令牌设置此选项</p>}
        </Alert>
      )}
    </Form>
  );
}
```

---

## 后端实现变更

### 1. 新增配置项

**common/constants.go**
```go
var SavePromptEnabled = false        // 现有
var SavePromptUserVisible = false    // 新增
```

**model/option.go**
```go
func InitOptionMap() {
    // ... 现有代码
    common.OptionMap["SavePromptEnabled"] = strconv.FormatBool(common.SavePromptEnabled)
    common.OptionMap["SavePromptUserVisible"] = strconv.FormatBool(common.SavePromptUserVisible)  // 新增
}
```

---

### 2. 修改决策逻辑

**model/log.go - savePrompt() 函数**

```go
func savePrompt(c *gin.Context, logId int, userId int) {
    // 1. 检查全局主开关
    if !common.SavePromptEnabled {
        return
    }

    // 2. 如果用户不可见模式，强制保存
    if !common.SavePromptUserVisible {
        // 强制保存所有用户
        promptText := c.GetString(string(constant.ContextKeyPromptToSave))
        if promptText != "" {
            EnqueuePromptLog(logId, promptText)
        }
        return
    }

    // 3. 用户可见模式：检查令牌和用户设置
    // 检查令牌级覆盖（最高优先级）
    if common.GetContextKeyBool(c, constant.ContextKeyTokenSavePrompt) {
        promptText := c.GetString(string(constant.ContextKeyPromptToSave))
        if promptText != "" {
            EnqueuePromptLog(logId, promptText)
        }
        return
    }

    // 检查用户设置
    settingMap, err := GetUserSetting(userId, false)
    if err != nil || !settingMap.SavePrompt {
        return
    }

    promptText := c.GetString(string(constant.ContextKeyPromptToSave))
    if promptText != "" {
        EnqueuePromptLog(logId, promptText)
    }
}
```

---

### 3. API 接口调整

**controller/user.go - 获取用户设置**

```go
func GetUserSetting(c *gin.Context) {
    userId := c.GetInt("id")
    settings, err := model.GetUserSetting(userId, false)
    if err != nil {
        common.ApiError(c, err)
        return
    }

    // 添加系统配置信息
    response := gin.H{
        "settings": settings,
        "system_config": gin.H{
            "save_prompt_enabled": common.SavePromptEnabled,
            "save_prompt_user_visible": common.SavePromptUserVisible,  // 新增
        },
    }

    common.ApiSuccess(c, response)
}
```

**controller/token.go - 令牌列表/详情**

```go
func GetTokens(c *gin.Context) {
    // ... 现有代码获取 tokens

    // 添加系统配置
    isAdmin := model.IsAdmin(c.GetInt("id"))
    
    response := gin.H{
        "tokens": tokens,
        "system_config": gin.H{
            "save_prompt_enabled": common.SavePromptEnabled,
            "save_prompt_user_visible": common.SavePromptUserVisible,
            "is_admin": isAdmin,
        },
    }

    common.ApiSuccess(c, response)
}
```

---

## 配置场景示例

### 场景 1: 完全关闭
```
SavePromptEnabled = false
SavePromptUserVisible = 任意
```
**结果**: ❌ 不保存任何 prompt

---

### 场景 2: 强制保存模式（推荐生产环境）
```
SavePromptEnabled = true
SavePromptUserVisible = false
```
**结果**: 
- ✅ 强制保存所有用户的 prompt
- 🔒 用户看不到"保存提示词"选项
- 🔒 令牌页面的 save_prompt 仅管理员可见可改
- 适用于：合规要求、审计需要

---

### 场景 3: 用户自主控制模式
```
SavePromptEnabled = true
SavePromptUserVisible = true
```
**结果**:
- ✅ 用户可以在个人设置中选择是否保存
- ✅ 用户可以在令牌页面设置强制保存
- ✅ 尊重用户隐私选择
- 适用于：开发测试、用户自主性高的场景

---

## 数据库变更

**无需新增字段**，配置存储在 `options` 表：

```sql
-- 系统启动时自动写入
INSERT INTO options (`key`, `value`) VALUES 
  ('SavePromptEnabled', 'false'),
  ('SavePromptUserVisible', 'false');
```

---

## 前端 API 调用

### 获取系统配置

```typescript
// 个人设置页面
GET /api/user/setting
Response:
{
  "code": 200,
  "data": {
    "settings": {
      "save_prompt": false
    },
    "system_config": {
      "save_prompt_enabled": true,
      "save_prompt_user_visible": false  // ← 根据此字段决定是否显示
    }
  }
}

// 令牌管理页面
GET /api/token
Response:
{
  "code": 200,
  "data": {
    "tokens": [...],
    "system_config": {
      "save_prompt_enabled": true,
      "save_prompt_user_visible": false,
      "is_admin": true  // ← 根据此字段决定是否显示
    }
  }
}
```

---

## 权限矩阵

| 用户类型 | SavePromptUserVisible | 个人设置可见 | 令牌设置可见 | 说明 |
|---------|----------------------|------------|------------|------|
| 普通用户 | `false` | ❌ | ❌ | 强制保存，用户无感知 |
| 普通用户 | `true` | ✅ | ✅ | 用户可自主控制 |
| 管理员 | `false` | ✅ (查看) | ✅ (修改) | 管理员总是可见可控 |
| 管理员 | `true` | ✅ | ✅ | 管理员总是可见可控 |

---

## 实现优先级

### Phase 1: 后端（高优先级）
1. ✅ 添加 `SavePromptUserVisible` 配置项
2. ✅ 修改 `savePrompt()` 决策逻辑
3. ✅ 修改 API 接口返回系统配置

### Phase 2: 前端（高优先级）
1. ✅ 管理后台添加"用户提示词开关显示"
2. ✅ 个人设置页面条件显示
3. ✅ 令牌编辑页面条件显示

### Phase 3: 测试
1. 测试强制保存模式
2. 测试用户可见模式
3. 测试管理员权限
4. 测试配置组合

---

## 总结

通过增加 `SavePromptUserVisible` 配置，实现：

1. **灵活性**: 管理员可以选择强制保存或让用户自主控制
2. **合规性**: 强制保存模式满足审计要求
3. **隐私性**: 用户可见模式尊重用户隐私选择
4. **权限控制**: 管理员始终拥有完全控制权

**推荐配置**:
- 生产环境: `SavePromptEnabled=true, SavePromptUserVisible=false` (强制保存)
- 开发环境: `SavePromptEnabled=true, SavePromptUserVisible=true` (用户可控)
