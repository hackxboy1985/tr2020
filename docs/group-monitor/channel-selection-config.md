# 分组监控渠道选择配置设计

## 1. 需求说明

管理员需要自由选择每个分组要监控哪些渠道，而不是默认显示所有渠道。

**使用场景**：
- 某些低优先级渠道不想在监控页面显示
- 测试渠道不参与监控统计
- 只关注核心高优先级渠道

---

## 2. 配置页面设计

### 2.1 页面位置

**路径**：系统设置 → 运维 → 监控与报警

**布局**：
```
┌─────────────────────────────────────────┐
│ 监控与报警                               │
├─────────────────────────────────────────┤
│                                         │
│ [现有配置项]                             │
│ • 自动测试渠道间隔                        │
│ • 渠道禁用阈值                           │
│ • ...                                   │
│                                         │
├─────────────────────────────────────────┤  ← 分隔线
│                                         │
│ 分组监控渠道选择                          │
│                                         │
│ ┌─ default 分组 ──────────────────┐    │
│ │ ☑ 渠道1 (priority=100, OpenAI)  │    │
│ │ ☑ 渠道2 (priority=50, Claude)   │    │
│ │ ☐ 渠道3 (priority=10, 测试渠道)  │    │
│ └────────────────────────────────┘    │
│                                         │
│ ┌─ gpt-plus 分组 ─────────────────┐    │
│ │ ☑ 渠道4 (priority=200, GPT4)    │    │
│ │ ☑ 渠道5 (priority=100, Claude)  │    │
│ └────────────────────────────────┘    │
│                                         │
│        [保存配置]                        │
└─────────────────────────────────────────┘
```

### 2.2 前端组件结构

```tsx
// features/system-settings/operations/monitoring-settings-section.tsx

<Section title="监控与报警">
  {/* 现有配置项 */}
  <FormField label="自动测试渠道间隔" ... />
  <FormField label="渠道禁用阈值" ... />
  
  {/* 分隔线 */}
  <Separator className="my-6" />
  
  {/* 新增：分组监控渠道选择 */}
  <div className="space-y-4">
    <h3 className="text-lg font-medium">分组监控渠道选择</h3>
    <p className="text-sm text-muted-foreground">
      选择每个分组在监控页面中要显示的渠道（最多显示前3个优先级最高的已选渠道）
    </p>
    
    {groupChannelsData.map(group => (
      <Card key={group.name}>
        <CardHeader>
          <CardTitle>{group.name} 分组</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-2">
            {group.channels.map(channel => (
              <div key={channel.id} className="flex items-center space-x-2">
                <Checkbox
                  id={`channel-${channel.id}`}
                  checked={selectedChannels.includes(channel.id)}
                  onCheckedChange={(checked) => 
                    handleChannelToggle(group.name, channel.id, checked)
                  }
                />
                <Label htmlFor={`channel-${channel.id}`} className="flex-1 cursor-pointer">
                  {channel.name} (priority={channel.priority})
                </Label>
                <Badge variant={channel.status === 1 ? "success" : "secondary"}>
                  {channel.status === 1 ? "启用" : "禁用"}
                </Badge>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>
    ))}
  </div>
</Section>
```

---

## 3. 数据存储方案

### 3.1 方案 A：存储在 options 表（推荐）

**优点**：
- 无需新增表
- 利用现有配置系统
- 支持动态更新

**存储格式**：
```
key: GroupMonitorChannelSelection
value: JSON 字符串

{
  "default": [1, 2, 5],      // 选中的渠道 ID 列表
  "gpt-plus": [3, 4],
  "claude满血版价格稳定": [6, 7, 8]
}
```

**读取逻辑**：
```go
// model/option.go 新增
func GetGroupMonitorChannelSelection() map[string][]int {
    option, err := GetOption("GroupMonitorChannelSelection")
    if err != nil || option.Value == "" {
        return nil  // 未配置时返回 nil，表示显示所有渠道
    }
    
    var selection map[string][]int
    json.Unmarshal([]byte(option.Value), &selection)
    return selection
}

func SetGroupMonitorChannelSelection(selection map[string][]int) error {
    jsonBytes, _ := json.Marshal(selection)
    return UpdateOption("GroupMonitorChannelSelection", string(jsonBytes))
}
```

---

### 3.2 方案 B：存储在 channels 表新增字段

**字段**：
```sql
ALTER TABLE channels ADD COLUMN monitor_visible BOOLEAN DEFAULT true;
```

**优点**：
- 查询更简单（WHERE monitor_visible = true）
- 不依赖分组维度

**缺点**：
- 修改表结构
- 无法区分"该渠道在某些分组显示，某些分组不显示"的场景

**不推荐**：因为一个渠道可能属于多个分组，单个布尔字段不够灵活。

---

### 3.3 推荐：方案 A（options 表）

更灵活，支持按分组单独配置。

---

## 4. 后端接口设计

### 4.1 获取分组渠道列表（用于配置页面）

```
GET /api/channel/group-channels
权限：AdminAuth()

响应：
[
  {
    "group": "default",
    "channels": [
      {
        "id": 1,
        "name": "OpenAI官方",
        "priority": 100,
        "status": 1,
        "selected": true  // 当前是否被选中
      },
      {
        "id": 2,
        "name": "Claude Pro",
        "priority": 50,
        "status": 1,
        "selected": true
      },
      {
        "id": 3,
        "name": "测试渠道",
        "priority": 10,
        "status": 2,
        "selected": false
      }
    ]
  },
  {
    "group": "gpt-plus",
    "channels": [...]
  }
]
```

**实现逻辑**：
```go
func GetGroupChannelsForConfig(c *gin.Context) {
    // 1. 从 abilities 表查出所有分组和渠道
    var abilities []model.Ability
    err := model.DB.
        Preload("Channel").  // 关联查询 channels 表
        Group("group, channel_id").
        Find(&abilities).Error
    
    // 2. 读取当前配置
    selection := model.GetGroupMonitorChannelSelection()
    
    // 3. 按分组分类
    groupMap := make(map[string][]ChannelConfigItem)
    for _, ability := range abilities {
        channel := ability.Channel  // 假设已 Preload
        
        selected := true  // 默认全选
        if selection != nil {
            if ids, ok := selection[ability.Group]; ok {
                selected = slices.Contains(ids, channel.Id)
            }
        }
        
        groupMap[ability.Group] = append(groupMap[ability.Group], ChannelConfigItem{
            ID:       channel.Id,
            Name:     channel.Name,
            Priority: *ability.Priority,
            Status:   channel.Status,
            Selected: selected,
        })
    }
    
    // 4. 转换为数组返回
    result := []GroupChannelsConfig{}
    for group, channels := range groupMap {
        // 按优先级降序排序
        sort.Slice(channels, func(i, j int) bool {
            return channels[i].Priority > channels[j].Priority
        })
        
        result = append(result, GroupChannelsConfig{
            Group:    group,
            Channels: channels,
        })
    }
    
    c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}
```

---

### 4.2 保存配置

```
POST /api/channel/group-monitor-config
权限：AdminAuth()

请求体：
{
  "default": [1, 2],
  "gpt-plus": [3, 4, 5]
}

响应：
{
  "success": true,
  "message": "配置已保存"
}
```

**实现**：
```go
func SaveGroupMonitorConfig(c *gin.Context) {
    var selection map[string][]int
    if err := c.ShouldBindJSON(&selection); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
        return
    }
    
    err := model.SetGroupMonitorChannelSelection(selection)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"success": true, "message": "配置已保存"})
}
```

---

## 5. 分组监控接口的过滤逻辑

### 5.1 修改 `GetGroupMonitorStatus` 接口

```go
func GetGroupMonitorStatus(c *gin.Context) {
    isAdmin := true  // 已通过 AdminAuth
    
    // 1. 读取渠道选择配置
    selection := model.GetGroupMonitorChannelSelection()
    
    // 2. 查询各分组状态
    groups := []GroupMonitorResult{}
    for groupName, selectedChannelIDs := range selection {
        // 如果该分组未配置，则跳过（或显示所有，取决于需求）
        if len(selectedChannelIDs) == 0 {
            continue
        }
        
        // 3. 查出该分组的 abilities（仅限选中的渠道）
        var abilities []model.Ability
        model.DB.
            Where("group = ? AND channel_id IN ? AND enabled = ?", groupName, selectedChannelIDs, true).
            Order("priority DESC").
            Find(&abilities)
        
        // 4. 取前3个
        top3 := abilities
        if len(abilities) > 3 {
            top3 = abilities[:3]
        }
        
        // 5. 计算心跳格、可用率等
        groupStatus := calculateGroupStatus(groupName, top3, isAdmin)
        groups = append(groups, groupStatus)
    }
    
    c.JSON(http.StatusOK, gin.H{"success": true, "data": groups})
}
```

---

## 6. 默认行为

### 6.1 未配置时的行为

**选项 A**：显示所有渠道（默认全选）
- 新部署系统时，监控页面立即可用
- 管理员后续可以取消勾选不需要的渠道

**选项 B**：不显示任何渠道（默认全不选）
- 强制管理员主动配置
- 避免暴露不想监控的渠道

**推荐**：选项 A（默认全选），用户体验更好。

---

### 6.2 实现逻辑

```go
selection := model.GetGroupMonitorChannelSelection()

if selection == nil {
    // 未配置时，查询所有分组和渠道（默认全选）
    var abilities []model.Ability
    model.DB.Where("enabled = ?", true).Find(&abilities)
    
    // 按分组分类，每个分组取前3个
    ...
} else {
    // 已配置时，只查询选中的渠道
    ...
}
```

---

## 7. 前端状态管理

```tsx
// features/system-settings/operations/monitoring-settings-section.tsx

const [groupChannels, setGroupChannels] = useState<GroupChannelsConfig[]>([])
const [selectedChannels, setSelectedChannels] = useState<Record<string, number[]>>({})

// 加载配置
useEffect(() => {
  fetchGroupChannelsConfig().then(data => {
    setGroupChannels(data)
    
    // 初始化选中状态
    const initial: Record<string, number[]> = {}
    data.forEach(group => {
      initial[group.group] = group.channels
        .filter(ch => ch.selected)
        .map(ch => ch.id)
    })
    setSelectedChannels(initial)
  })
}, [])

// 切换渠道选中状态
const handleChannelToggle = (group: string, channelId: number, checked: boolean) => {
  setSelectedChannels(prev => {
    const groupIds = prev[group] || []
    if (checked) {
      return { ...prev, [group]: [...groupIds, channelId] }
    } else {
      return { ...prev, [group]: groupIds.filter(id => id !== channelId) }
    }
  })
}

// 保存配置
const handleSave = async () => {
  await saveGroupMonitorConfig(selectedChannels)
  toast.success("配置已保存")
}
```

---

## 8. 优化：只显示前3个选中的渠道

由于心跳格颜色规则只看前 3 个优先级最高的渠道，配置页面可以给出提示：

```tsx
<p className="text-sm text-yellow-600">
  ⚠️ 注意：监控页面最多显示前 3 个优先级最高的已选渠道
</p>
```

**交互优化**：
- 当某个分组选中超过 3 个渠道时，显示 badge 标记哪些会被实际使用
- 例如：前 3 个渠道旁边显示 `✓ 将显示`，其他显示 `备用`

---

## 9. 路由注册

```go
// router/api-router.go

channelRoute := apiRouter.Group("/channel")
{
    // 获取分组渠道配置（用于配置页面）
    channelRoute.GET("/group-channels", middleware.AdminAuth(), controller.GetGroupChannelsForConfig)
    
    // 保存分组监控配置
    channelRoute.POST("/group-monitor-config", middleware.AdminAuth(), controller.SaveGroupMonitorConfig)
    
    // 获取分组监控状态（用于监控页面）
    channelRoute.GET("/group-monitor", middleware.AdminAuth(), controller.GetGroupMonitorStatus)
}
```

---

## 10. 迁移与兼容性

### 10.1 首次部署

- 数据库无需迁移（使用 options 表）
- 首次访问时，`GroupMonitorChannelSelection` 不存在 → 默认显示所有渠道

### 10.2 旧版本升级

- 无需数据迁移
- 自动兼容（未配置 = 显示所有）

---

## 11. 总结

| 功能点 | 实现方式 |
|--------|---------|
| **配置页面** | 系统设置 → 监控与报警 → 最底部 |
| **数据存储** | options 表，key = `GroupMonitorChannelSelection` |
| **默认行为** | 未配置时显示所有渠道（全选） |
| **过滤逻辑** | 查询时只取选中的渠道 ID |
| **前端组件** | 按分组展示 Checkbox 列表 |
| **权限** | 仅管理员可配置和查看 |
