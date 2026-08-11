# 分组监控配置页面设计（修订版）

## 1. 需求澄清

### 1.1 配置目标

**不是**：选择分组内的哪些渠道参与监控 ❌
**而是**：选择监控页面要展示哪些分组 ✅

**原因**：
- 所有渠道的定时测试逻辑保持不变（自动禁用/启用）
- 监控页面只是一个"查看"功能，不影响实际测试逻辑
- 管理员可能只想关注某几个重要分组，隐藏其他分组

---

## 2. 配置页面布局

### 2.1 页面位置

**路径**：系统设置 → 运维 → 监控与报警

### 2.2 完整布局

```
┌─────────────────────────────────────────────────────┐
│ 监控与报警                                           │
├─────────────────────────────────────────────────────┤
│                                                     │
│ ☑ 启用自动禁用渠道                                   │
│ ☑ 启用自动启用渠道                                   │
│                                                     │
│ 渠道禁用阈值                                         │
│ [10] 秒                                             │
│                                                     │
│ 自动测试渠道间隔                                      │
│ [30] 分钟                                           │
│                                                     │
│ 渠道状态变更邮件通知                                  │
│ ☑ 当渠道自动开启或关闭时发送邮件给管理员              │
│                                                     │
├─────────────────────────────────────────────────────┤  ← 分隔线
│                                                     │
│ 分组监控配置                                         │
│                                                     │
│ 选择要在监控页面展示的分组：                          │
│                                                     │
│ ☑ default                                          │
│ ☑ gpt-plus                                         │
│ ☑ claude满血版价格稳定                               │
│ ☐ codex-pro                                        │
│ ☐ test-group                                       │
│                                                     │
│ 提示：所有分组的渠道仍会正常进行定时测试，            │
│       此配置仅影响监控页面的显示内容。                │
│                                                     │
│              [保存配置]                              │
└─────────────────────────────────────────────────────┘
```

---

## 3. 配置说明

### 3.1 选择逻辑

- **选中的分组**：在监控页面展示
- **未选中的分组**：在监控页面隐藏
- **默认行为**：未配置时，展示所有分组

### 3.2 不影响测试逻辑

**重要**：
- ✅ 所有渠道的定时测试正常进行（无论分组是否被选中）
- ✅ 自动禁用/启用逻辑不受影响
- ✅ 测试历史正常记录
- ❌ 不会跳过未选中分组的渠道测试

**这个配置只是一个"显示过滤器"**

---

## 4. 数据存储

### 4.1 存储格式

**表**：`options` 表  
**key**：`GroupMonitorVisibleGroups`  
**value**：JSON 数组

```json
["default", "gpt-plus", "claude满血版价格稳定"]
```

### 4.2 读取逻辑

```go
// model/option.go
func GetGroupMonitorVisibleGroups() []string {
    option, err := GetOption("GroupMonitorVisibleGroups")
    if err != nil || option.Value == "" {
        return nil  // nil 表示显示所有分组
    }
    
    var groups []string
    json.Unmarshal([]byte(option.Value), &groups)
    return groups
}

func SetGroupMonitorVisibleGroups(groups []string) error {
    jsonBytes, _ := json.Marshal(groups)
    return UpdateOption("GroupMonitorVisibleGroups", string(jsonBytes))
}
```

---

## 5. 前端实现

### 5.1 获取所有分组列表

**接口**：`GET /api/channel/groups`

```go
func GetAllGroups(c *gin.Context) {
    var groups []string
    err := model.DB.Model(&model.Ability{}).
        Distinct("group").
        Where("enabled = ?", true).
        Pluck("group", &groups).Error
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
        return
    }
    
    // 读取当前配置
    selectedGroups := model.GetGroupMonitorVisibleGroups()
    
    // 构建返回数据
    result := []gin.H{}
    for _, group := range groups {
        selected := true  // 默认全选
        if selectedGroups != nil {
            selected = slices.Contains(selectedGroups, group)
        }
        
        result = append(result, gin.H{
            "name":     group,
            "selected": selected,
        })
    }
    
    c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}
```

**响应示例**：
```json
{
  "success": true,
  "data": [
    {"name": "default", "selected": true},
    {"name": "gpt-plus", "selected": true},
    {"name": "codex-pro", "selected": false},
    {"name": "test-group", "selected": false}
  ]
}
```

---

### 5.2 保存配置

**接口**：`POST /api/channel/group-monitor-config`

**请求体**：
```json
{
  "visible_groups": ["default", "gpt-plus", "claude满血版价格稳定"]
}
```

**实现**：
```go
func SaveGroupMonitorConfig(c *gin.Context) {
    var req struct {
        VisibleGroups []string `json:"visible_groups"`
    }
    
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
        return
    }
    
    err := model.SetGroupMonitorVisibleGroups(req.VisibleGroups)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"success": true, "message": "配置已保存"})
}
```

---

### 5.3 前端组件

```tsx
// features/system-settings/operations/monitoring-settings-section.tsx

import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Info } from 'lucide-react'

const [groups, setGroups] = useState<{name: string, selected: boolean}[]>([])

useEffect(() => {
  // 加载分组列表
  fetchGroups().then(data => setGroups(data))
}, [])

const handleGroupToggle = (groupName: string, checked: boolean) => {
  setGroups(prev => 
    prev.map(g => g.name === groupName ? {...g, selected: checked} : g)
  )
}

const handleSave = async () => {
  const visibleGroups = groups.filter(g => g.selected).map(g => g.name)
  await saveGroupMonitorConfig({ visible_groups: visibleGroups })
  toast.success("配置已保存")
}

return (
  <Section title="监控与报警">
    {/* 现有配置项 */}
    <FormField label="启用自动禁用渠道" ... />
    <FormField label="启用自动启用渠道" ... />
    <FormField label="渠道禁用阈值" ... />
    <FormField label="自动测试渠道间隔" ... />
    <FormField label="渠道状态变更邮件通知" ... />
    
    {/* 分隔线 */}
    <Separator className="my-6" />
    
    {/* 分组监控配置 */}
    <div className="space-y-4">
      <div>
        <h3 className="text-lg font-medium">分组监控配置</h3>
        <p className="text-sm text-muted-foreground mt-1">
          选择要在监控页面展示的分组
        </p>
      </div>
      
      <Alert>
        <Info className="h-4 w-4" />
        <AlertDescription>
          所有分组的渠道仍会正常进行定时测试，此配置仅影响监控页面的显示内容。
        </AlertDescription>
      </Alert>
      
      <div className="space-y-2">
        {groups.map(group => (
          <div key={group.name} className="flex items-center space-x-2">
            <Checkbox
              id={`group-${group.name}`}
              checked={group.selected}
              onCheckedChange={(checked) => 
                handleGroupToggle(group.name, checked as boolean)
              }
            />
            <Label 
              htmlFor={`group-${group.name}`} 
              className="cursor-pointer font-normal"
            >
              {group.name}
            </Label>
          </div>
        ))}
      </div>
      
      <Button onClick={handleSave}>保存配置</Button>
    </div>
  </Section>
)
```

---

## 6. 监控页面过滤逻辑

### 6.1 获取分组监控状态接口

**接口**：`GET /api/channel/group-monitor`

**修改**：只返回选中的分组

```go
func GetGroupMonitorStatus(c *gin.Context) {
    isAdmin := true  // 已通过 AdminAuth
    
    // 读取要显示的分组配置
    visibleGroups := model.GetGroupMonitorVisibleGroups()
    
    // 如果未配置，则显示所有分组
    var allGroups []string
    if visibleGroups == nil {
        // 查询所有分组
        model.DB.Model(&model.Ability{}).
            Distinct("group").
            Where("enabled = ?", true).
            Pluck("group", &allGroups)
        visibleGroups = allGroups
    }
    
    // 对每个可见分组，计算监控数据
    results := []GroupMonitorResult{}
    for _, groupName := range visibleGroups {
        groupStatus := calculateGroupMonitorStatus(groupName, isAdmin)
        results = append(results, groupStatus)
    }
    
    c.JSON(http.StatusOK, gin.H{"success": true, "data": results})
}
```

---

## 7. 与旧设计的差异

| 项目 | 旧设计（错误） | 新设计（正确） |
|------|---------------|---------------|
| **配置粒度** | 分组 → 渠道（细化到具体渠道） | 只到分组级别 |
| **配置含义** | 选择哪些渠道参与监控 | 选择哪些分组在页面显示 |
| **是否影响测试** | 会影响（未选中的不测试） | 不影响（所有渠道都测试） |
| **数据结构** | `{"default": [1,2,5], "gpt-plus": [3,4]}` | `["default", "gpt-plus"]` |
| **接口名称** | `/group-channels` | `/groups` |

---

## 8. 配置示例场景

### 场景 1：只关注生产分组

**配置**：
```
☑ default
☑ gpt-plus
☐ test-group
☐ dev-group
```

**结果**：
- 监控页面只显示 `default` 和 `gpt-plus` 两个分组
- `test-group` 和 `dev-group` 的渠道仍然正常测试、自动禁用/启用
- 只是监控页面不展示这两个分组的状态

---

### 场景 2：全部显示（默认）

**配置**：未配置或全选

**结果**：
- 监控页面显示所有分组

---

## 9. 总结

### 关键点

1. ✅ **配置仅影响展示**，不影响测试逻辑
2. ✅ **配置粒度是分组**，不细化到渠道
3. ✅ **所有渠道继续测试**，保持现有逻辑不变
4. ✅ **简化数据结构**，只存储分组名数组

### 为什么这样设计

- **简单**：只需勾选分组名，不需要复杂的渠道选择
- **安全**：不影响现有测试和自动禁用逻辑
- **灵活**：管理员可以只关注重要分组，减少页面干扰
