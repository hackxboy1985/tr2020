# Token 多分组支持 — 需求分析与设计方案

## 需求

1. Token 创建时支持**选择多个分组**（当前只能选 1 个）
2. 分组按**倍率从低到高**自动排序存储到数据库
3. 用户请求时，**从倍率低的组开始**自动选择并路由
4. 支持**渠道粘性（亲和）**
5. 当前分组失败后**自动进入下一个分组**
6. 一个 Token 可访问不同公司的模型，**无需切换令牌**

## 可行性

**80% 的代码架构已经存在。** 当前 `auto` 分组模式已经实现了：
- 跨分组顺序重试
- 分组切换时重置 retry 计数器
- 亲和性在分组间路由

核心差异：当前是全局配置分组列表（`setting.autoGroups`），需求是 Token 级别配置。

### 当前 vs 目标

| | 当前 auto 模式 | 目标 |
|---|---|---|
| 分组来源 | 全局 `setting.autoGroups` | Token 级别配置 |
| 分组顺序 | 管理员手动排 | 按倍率从低到高自动排 |
| 跨分组重试 | ✅ `CacheGetRandomSatisfiedChannel` | ✅ 复用 |
| 亲和性 | ✅ `GetPreferredChannelByAffinity` | ✅ 复用 |
| 前端选择器 | `<Form.Select>` 单选 | 多选组件 |

## 架构基础

### Token 模型 (`model/token.go:14-32`)

```go
type Token struct {
    Group           string `json:"group" gorm:"default:''"`        // 当前：单分组
    CrossGroupRetry bool   `json:"cross_group_retry"`               // 跨分组重试
}
```

### Auth 中间件分组解析 (`middleware/auth.go:382-399`)

```go
tokenGroup := token.Group
if tokenGroup != "" {
    userGroup = tokenGroup  // Token 分组覆盖用户分组
}
common.SetContextKey(c, constant.ContextKeyUsingGroup, userGroup)
```

### 跨分组重试 (`service/channel_select.go:83-162`)

```
TokenGroup == "auto" 时:
  1. GetUserAutoGroup() → 获取全局 auto-groups 列表
  2. for each group:
     - GetRandomSatisfiedChannel(group, model, priorityRetry) → 选渠道
     - nil → 下一分组
     - 找到 → break
  3. ContextKeyAutoGroupIndex 跟踪当前分组位置
```

### 分发器 (`middleware/distributor.go:104-166`)

```
GetPreferredChannelByAffinity → 亲和命中?
  ├─ 命中 → 检查渠道状态 → 可用 → 使用
  │                           └─ 不可用 → 清除亲和 → 随机选
  └─ 未命中 → CacheGetRandomSatisfiedChannel → 随机选

成功后(status < 400) → RecordChannelAffinity → 更新亲和
```

## 实现方案

### 一、数据模型 (`model/token.go`)

新增字段：

```go
type Token struct {
    // ... 现有字段保留 ...
    Group  string `json:"group" gorm:"default:''"`   // 兼容旧单分组
    Groups string `json:"groups"`                     // 新增：JSON数组, 按倍率排序
    // ["group_cheapest","group_mid","group_expensive"]
}
```

兼容策略：
- `Group` 非空 → 旧逻辑（单分组或 auto）
- `Groups` 非空 → 新逻辑（多分组）
- 优先使用 `Groups`

### 二、Auth 中间件 (`middleware/auth.go`)

新的分组解析逻辑：

```go
if token.Groups != "" {
    // 新逻辑：解析多分组
    var groups []string
    json.Unmarshal([]byte(token.Groups), &groups)
    // 存储到 context，供后续使用
    common.SetContextKey(c, constant.ContextKeyTokenOrderedGroups, groups)
    common.SetContextKey(c, constant.ContextKeyUsingGroup, groups[0])
} else if tokenGroup != "" {
    // 旧逻辑：单分组或 auto
    userGroup = tokenGroup
}
```

### 三、渠道选择 (`service/channel_select.go`)

修改分组来源：

```go
// 当前
if param.TokenGroup == "auto" {
    autoGroups = GetUserAutoGroup(userGroup)
}

// 改为
orderedGroups := getTokenOrderedGroups(param.Ctx)
if len(orderedGroups) > 0 {
    autoGroups = orderedGroups  // Token 专属分组，已按倍率排序
}
```

跨分组遍历逻辑**完全复用**，不需要改变。

### 四、Token API (`controller/token.go`)

```go
func AddToken(c *gin.Context) {
    // 1. 接收 groups 字段
    // 2. 查询各分组倍率
    // 3. 按倍率从低到高排序
    // 4. json.Marshal → 存入 Groups 字段
}
```

排序逻辑：

```go
type groupWithRatio struct {
    Name  string
    Ratio float64
}

groups := []groupWithRatio{
    {Name: "svip", Ratio: 1.0},
    {Name: "vip", Ratio: 1.2},
    {Name: "default", Ratio: 1.5},
}
sort.Slice(groups, func(i, j int) bool {
    return groups[i].Ratio < groups[j].Ratio
})
// → ["svip", "vip", "default"]
```

### 五、前端改动

**Token 编辑表单** (`EditTokenModal.jsx`)

```
当前: <Form.Select field='group' />   // 单选下拉

改为: <MultiGroupSelector />  // 多选 + 显示倍率 + 拖拽排序
```

多选组件功能：
- 多选分组（checkbox 或 tag）
- 实时显示各分组倍率
- 自动按倍率排序（或手动拖拽）
- 兼容旧字段（Group 为空时升级为多分组）

### 六、改动量预估

| 层 | 文件 | 改动类型 | 量 |
|---|---|---|---|
| 模型 | `model/token.go` | 加 `Groups` 字段 | 小 |
| 中间件 | `middleware/auth.go` | 解析多分组逻辑 | 中 |
| 渠道选择 | `service/channel_select.go` | 分组来源切换 | 小 |
| 上下文键 | `constant/context_key.go` | 新增 key | 1 行 |
| Token API | `controller/token.go` | 排序 + 验证 | 中 |
| 前端 | `Token编辑弹窗` + 多选组件 | 组件重写 | 中 |
| 数据库 | Migration | 加 `groups` 列 | 1 条 |

## 使用流程

```
用户创建 Token → 选择 3 个分组 → 自动按倍率排序 → 存入 groups 字段

请求进来:
  1. Auth: 解析 groups → ["便宜组","中等组","贵组"]
  2. Distributor: 亲和检查 → 渠道选择
  3. 便宜组有可用渠道 → 使用 → 成功 → 记录亲和
  4. 便宜组渠道失败(502) → 重试 → 进入中等组
  5. 中等组成功 → 记录亲和(中等组渠道) → 下次优先走中等组
```

## 兼容性

- 旧 Token（`Group` 非空，`Groups` 为空）：走旧逻辑，完全兼容
- 新 Token（`Groups` 非空）：走新逻辑，自动跨分组
- auto 分组机制不变：`Group="auto"` 依然按全局 autoGroups 路由
