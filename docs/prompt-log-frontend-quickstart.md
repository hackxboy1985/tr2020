# Prompt Log 前端开发快速启动指南

## 开发前准备

### 1. 确认后端运行
```bash
# 启动后端服务
go run main.go

# 或使用 Docker
docker-compose up -d
```

### 2. 验证 API 可用性
```bash
# 测试系统状态接口
curl http://localhost:3000/api/status | jq '.save_prompt_enabled, .save_prompt_user_visible'

# 应返回:
# false
# false
```

---

## 前端开发步骤

### 第 1 步：管理后台全局设置

#### 文件位置
查找系统设置页面，通常在：
- `web/default/src/routes/_authenticated/system-settings/`
- `web/default/src/features/system-settings/`

#### 需要添加的组件
```tsx
// 在日志设置或运营设置页面添加

<FormGroup>
  <Label>提示词保存设置</Label>
  
  <Switch
    label="启用提示词保存"
    checked={settings.SavePromptEnabled}
    onChange={(checked) => updateSetting('SavePromptEnabled', checked)}
  />

  {settings.SavePromptEnabled && (
    <Switch
      label="用户提示词开关显示"
      description="控制用户是否能看到和设置 prompt 保存选项"
      checked={settings.SavePromptUserVisible}
      onChange={(checked) => updateSetting('SavePromptUserVisible', checked)}
    />
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
</FormGroup>
```

---

### 第 2 步：个人隐私设置

#### 文件位置
查找个人设置页面：
- `web/default/src/routes/_authenticated/profile/` 或 `/settings/`
- `web/default/src/features/profile/`

#### 需要添加的代码
```tsx
// 1. 获取系统配置
const [systemConfig, setSystemConfig] = useState<any>({});

useEffect(() => {
  fetch('/api/status')
    .then(res => res.json())
    .then(data => setSystemConfig(data));
}, []);

// 2. 条件显示
const showPromptSetting = 
  systemConfig.save_prompt_enabled && 
  systemConfig.save_prompt_user_visible;

// 3. 渲染组件
{showPromptSetting && (
  <>
    <Switch
      label="保存我的提示词"
      description="保存请求中的提示词内容用于审计和分析（仅管理员可查看）"
      checked={userSettings.save_prompt}
      onChange={(checked) => updateUserSetting('save_prompt', checked)}
    />
    
    <Alert type="warning">
      <p>⚠️ 启用后，您发送的所有提示词将被保存到数据库</p>
      <p>• 仅管理员可以查看保存的提示词</p>
      <p>• 令牌设置可以覆盖此配置</p>
    </Alert>
  </>
)}
```

---

### 第 3 步：令牌编辑页面

#### 文件位置
查找令牌管理页面：
- `web/default/src/routes/_authenticated/tokens/`
- `web/default/src/features/tokens/`

#### 需要添加的代码
```tsx
// 1. 判断显示条件
const showPromptSetting = 
  isAdmin || 
  (systemConfig.save_prompt_enabled && systemConfig.save_prompt_user_visible);

// 2. 在表单中添加
{showPromptSetting && (
  <FormField
    control={form.control}
    name="save_prompt"
    render={({ field }) => (
      <FormItem>
        <FormLabel>强制保存提示词</FormLabel>
        <FormControl>
          <Switch
            checked={field.value}
            onCheckedChange={field.onChange}
          />
        </FormControl>
        <FormDescription>
          {isAdmin 
            ? "管理员权限：强制保存此令牌的所有请求提示词" 
            : "开启后将强制保存此令牌的所有请求提示词，不受用户设置影响"}
        </FormDescription>
      </FormItem>
    )}
  />
)}
```

---

### 第 4 步：日志列表展示

#### 文件位置
查找日志管理页面：
- `web/default/src/routes/_authenticated/logs/`
- `web/default/src/features/logs/`

#### 4.1 添加 Prompt 列
```tsx
// 在 columns 定义中添加
{
  accessorKey: 'prompt_text',
  header: 'Prompt',
  cell: ({ row }) => {
    const promptText = row.original.prompt_text;
    const logType = row.original.type;
    
    // 只显示 CONSUME 类型（type=2）的日志
    if (logType !== 2 || !promptText) {
      return <span>-</span>;
    }
    
    return (
      <div className="flex items-center gap-2">
        <Badge variant="secondary">
          {promptText.length} 字符
        </Badge>
        <Button
          variant="link"
          size="sm"
          onClick={() => setSelectedLog(row.original)}
        >
          查看
        </Button>
      </div>
    );
  }
}
```

#### 4.2 创建 Prompt 查看弹窗
```tsx
function PromptDialog({ log, open, onClose }: PromptDialogProps) {
  const [promptData, setPromptData] = useState<any>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (open && log?.id) {
      setLoading(true);
      
      // 如果列表已返回 prompt_text，直接使用
      if (log.prompt_text) {
        setPromptData({ prompt_text: log.prompt_text });
        setLoading(false);
      } else {
        // 否则单独查询
        fetch(`/api/log/prompt/${log.id}`)
          .then(res => res.json())
          .then(data => {
            setPromptData(data.data);
            setLoading(false);
          })
          .catch(() => {
            setLoading(false);
          });
      }
    }
  }, [open, log]);

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent className="max-w-4xl max-h-[80vh]">
        <DialogHeader>
          <DialogTitle>提示词内容</DialogTitle>
        </DialogHeader>
        
        {loading ? (
          <div className="flex justify-center p-8">
            <Spinner />
          </div>
        ) : (
          <div className="space-y-4">
            <Alert>
              <div className="flex gap-4">
                <span>用户ID: {log?.user_id}</span>
                <span>Token: {log?.token_name}</span>
                <span>时间: {formatTime(log?.created_at)}</span>
              </div>
            </Alert>
            
            <div className="bg-muted rounded-lg p-4 max-h-[500px] overflow-auto">
              <pre className="whitespace-pre-wrap break-words text-sm">
                {promptData?.prompt_text || '无提示词内容'}
              </pre>
            </div>

            {promptData?.prompt_text?.length >= 64000 && (
              <Alert variant="destructive">
                <AlertTitle>提示词已截断</AlertTitle>
                <AlertDescription>
                  此提示词超过 64KB 限制，已自动截断保存
                </AlertDescription>
              </Alert>
            )}
          </div>
        )}
        
        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => navigator.clipboard.writeText(promptData?.prompt_text || '')}
          >
            复制
          </Button>
          <Button onClick={onClose}>
            关闭
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
```

---

## 国际化（i18n）

### 添加翻译键
在 `web/default/src/i18n/locales/` 下的语言文件中添加：

```json
{
  "Save Prompt": "保存提示词",
  "Enable prompt saving": "启用提示词保存",
  "User prompt switch display": "用户提示词开关显示",
  "Force save prompts": "强制保存提示词",
  "View prompt": "查看提示词",
  "Prompt content": "提示词内容",
  "Prompt truncated": "提示词已截断",
  "This prompt exceeds the 64KB limit and has been automatically truncated": "此提示词超过 64KB 限制，已自动截断保存"
}
```

### 使用翻译
```tsx
import { useTranslation } from 'react-i18next';

function Component() {
  const { t } = useTranslation();
  
  return <label>{t('Save Prompt')}</label>;
}
```

---

## API 调用示例

### 1. 获取系统配置
```typescript
const getSystemConfig = async () => {
  const response = await fetch('/api/status');
  const data = await response.json();
  return {
    savePromptEnabled: data.save_prompt_enabled,
    savePromptUserVisible: data.save_prompt_user_visible
  };
};
```

### 2. 更新系统设置
```typescript
const updateSystemSetting = async (key: string, value: boolean) => {
  const response = await fetch('/api/option', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ [key]: value })
  });
  return response.json();
};
```

### 3. 更新用户设置
```typescript
const updateUserSetting = async (savePrompt: boolean) => {
  const response = await fetch('/api/user/setting', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ save_prompt: savePrompt })
  });
  return response.json();
};
```

### 4. 创建/更新令牌
```typescript
const createToken = async (tokenData: TokenFormData) => {
  const response = await fetch('/api/token', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      name: tokenData.name,
      save_prompt: tokenData.savePrompt,
      // ... 其他字段
    })
  });
  return response.json();
};
```

### 5. 查询 Prompt
```typescript
const getPromptLog = async (logId: number) => {
  const response = await fetch(`/api/log/prompt/${logId}`);
  const data = await response.json();
  return data.data;
};
```

---

## 开发建议

### 1. 优先级排序
1. ✅ 先实现日志列表展示（用户最直观）
2. ✅ 再实现全局设置（管理员必需）
3. ✅ 最后实现用户和令牌设置

### 2. 测试场景
```bash
# 1. 测试全局开关
- 关闭 SavePromptEnabled → 不保存任何 prompt
- 开启 SavePromptEnabled + 关闭 SavePromptUserVisible → 强制保存所有用户
- 开启两个开关 → 用户可控

# 2. 测试权限控制
- 管理员登录 → 应该看到日志列表的 Prompt 列
- 普通用户登录 → 不应该看到 Prompt 列

# 3. 测试条件显示
- SavePromptUserVisible=false → 用户看不到个人设置和令牌设置
- SavePromptUserVisible=true → 用户可以看到并修改
```

### 3. 调试技巧
```typescript
// 在组件中添加调试输出
useEffect(() => {
  console.log('System Config:', systemConfig);
  console.log('Show Prompt Setting:', showPromptSetting);
}, [systemConfig]);
```

---

## 常见问题

### Q1: 用户设置不显示？
**A**: 检查 `/api/status` 返回的 `save_prompt_user_visible` 是否为 true

### Q2: 日志列表没有 prompt_text？
**A**: 确认：
1. 用户是管理员
2. SavePromptEnabled 已开启
3. 该日志的 type=2（CONSUME 类型）

### Q3: 令牌设置对普通用户不显示？
**A**: 检查 `save_prompt_user_visible` 配置，false 时仅管理员可见

---

## 完成标准

- [ ] 全局设置页面显示两个开关
- [ ] 根据配置显示不同说明文字
- [ ] 个人设置条件显示 save_prompt 开关
- [ ] 令牌编辑条件显示 save_prompt 开关
- [ ] 日志列表添加 Prompt 列（仅管理员）
- [ ] Prompt 查看弹窗功能完整
- [ ] 所有文本支持国际化（zh/en）
- [ ] 响应式设计，移动端友好
- [ ] 所有 API 调用添加错误处理

---

## 参考资源

- 📋 **完整设计**: `docs/prompt-log-enhanced-design.md`
- 📖 **前端指南**: `docs/prompt-log-frontend-guide.md`
- 📊 **功能总结**: `docs/prompt-log-summary.md`
- 🔧 **实现检查**: `docs/prompt-log-implementation-check.md`

---

开始开发吧！如有问题随时查阅文档或询问。
