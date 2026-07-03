# 视频生成系统设计文档

> **版本**: 2.0  
> **最后更新**: 2026-07-02  
> **状态**: 设计确认，待实现

---

## 概述

本文档描述视频生成功能的完整技术架构。系统采用**本地持久化 + 多渠道管理**方案，设计与 new-api 现有渠道/分组机制保持一致。

---

## 设计目标

1. **多渠道并存**：同时支持多个 Coze 账号、多个三方平台账号，各自独立配置
2. **负载均衡**：同组渠道按权重随机选择，不做故障自动切换
3. **分组隔离**：与现有 new-api 用户分组机制一致，用户通过分组使用渠道，感知不到具体渠道
4. **本地状态管理**：项目状态持久化到本地，不完全依赖上游
5. **状态同步**：Webhook 回调（主动推）+ 查询时透传拉取（用户发起）两种方式互补

---

## 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                        Client (前端)                         │
└────────────────────────┬────────────────────────────────────┘
                         │ HTTP API
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                   new-api (视频生成网关)                      │
│                                                              │
│  ┌─────────────┐   ┌──────────────┐   ┌─────────────┐     │
│  │  Controller  │──▶│   Service    │──▶│   Adapter   │     │
│  └─────────────┘   └──────────────┘   └─────────────┘     │
│                      │                  │                    │
│                      ▼                  ├─ CozeAdapter       │
│               video_channels 表         └─ PlatformAdapter   │
│               (渠道配置 + 权重)                               │
│                                                              │
│  ┌─────────────────────────────────────────────────────┐   │
│  │              video_projects 表 (本地存储)             │   │
│  └─────────────────────────────────────────────────────┘   │
└───────────────┬────────────────────────────┬────────────────┘
                │ API调用                    │ Webhook回调
                ▼                            ▼
    ┌──────────────────┐          ┌──────────────────┐
    │ Coze A / Coze B  │          │ Platform A / B   │
    └──────────────────┘          └──────────────────┘
```

---

## 渠道与分组设计

与现有 new-api AI 渠道保持一致的设计理念：

```
渠道（VideoChannel）
  └─ groups = "default,vip"   # 该渠道对哪些用户组开放

用户（User）
  └─ group = "vip"            # 用户属于哪个组

创建项目时：
  user.group = "vip"
    ↓
  找 video_channels WHERE enabled=1
    AND (groups='' OR groups LIKE '%vip%')
    ↓
  按 weight 随机选一个渠道
    ↓
  调用对应适配器创建项目
```

**用户永远感知不到背后是哪个渠道**，只感知自己所在的分组是否有视频生成权限。

倍率/配额控制挂在现有 `GroupRatio` 机制上，与 AI 模型计费一致。

---

## 数据模型

### video_channels 表（新建）

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | INT | 主键 |
| `name` | VARCHAR(100) | 渠道名称，管理员自定义（如"Coze主账号"、"平台A"） |
| `channel_type` | VARCHAR(20) | `coze` 或 `platform` |
| `base_url` | VARCHAR(512) | API 基础地址 |
| `api_key` | TEXT | API 密钥 |
| `api_secret` | TEXT | Webhook 签名密钥 |
| `workflow_id` | VARCHAR(255) | Coze 专用，工作流 ID |
| `create_path` | VARCHAR(512) | 创建项目的接口路径，如 `/v1/workflow/run`、`/api/video/create` |
| `status_query_path` | VARCHAR(512) | 状态查询路径模板，含 `{id}` 占位符，如 `/v1/workflow/run/{id}` |
| `groups` | VARCHAR(255) | 逗号分隔用户组，**不允许为空**（空字符串不匹配任何组，渠道将不可用） |
| `weight` | INT | 权重，同组内按权重随机选 |
| `enabled` | TINYINT | 1=启用，0=禁用 |
| `remark` | VARCHAR(255) | 管理员备注 |
| `created_at` | BIGINT | 创建时间 |
| `updated_at` | BIGINT | 更新时间 |

**调用方式**（适配器统一行为）：

```
创建项目：POST {base_url}{create_path}
查询状态：GET  {base_url}{status_query_path}  （{id} 替换为 remote_project_id）
```

只要上游平台鉴权方式为 Bearer Token，任意平台均可通过配置接入，无需改代码。

**各渠道类型的默认路径**（字段为空时回退到以下值）：

| channel_type | create_path 默认值 | status_query_path 默认值 |
|-------------|-------------------|------------------------|
| `coze` | `/v1/workflow/run` | `/v1/workflow/run/{id}` |
| `platform` | `/api/video/create` | `/api/video/projects/{id}` |

> `create_path` 和 `status_query_path` 对所有渠道类型（包括 Coze）均从配置字段读取，字段为空时才使用上表默认值。Coze 渠道填写 `status_query_path` 字段是有效的，可覆盖默认值。

**示例数据**：

```
id=1  name="Coze-A"     type=coze      workflow_id=111  create_path=/v1/workflow/run  status_query_path=/v1/workflow/run/{id}     groups="default,vip"  weight=3
id=2  name="Coze-B"     type=coze      workflow_id=222  create_path=/v1/workflow/run  status_query_path=/v1/workflow/run/{id}     groups="vip"          weight=1
id=3  name="Platform-A" type=platform  base_url=http://p1  create_path=/api/video/create  status_query_path=/api/video/projects/{id}  groups="default"   weight=2
id=4  name="Platform-B" type=platform  base_url=http://p2  create_path=/api/video/create  status_query_path=/api/video/projects/{id}  groups="default"   weight=2
```

> **注意**：`groups` 字段不允许空字符串。与现有 AI 渠道 `Group` 字段保持一致——空字符串不会匹配任何组，渠道将不可用。至少需要填写 `default`。

### video_projects 表（新增字段）

在现有字段基础上新增：

| 字段 | 类型 | 说明 |
|------|------|------|
| `channel_id` | INT | 实际使用的渠道 ID，关联 video_channels.id |

> `channel_type` 字段保留，**仅在创建时从 VideoChannel 快照写入，后续不更新**。即使管理员修改了 VideoChannel 的 channel_type，video_projects 中已有记录的 channel_type 不变，仅供历史查看，实际路由逻辑以 `channel_id` 为准。

---

## 状态查询路径模板

每个渠道可配置 `status_query_path`，支持 `{id}` 占位符在运行时替换为 `remote_project_id`：

| 渠道类型 | 默认路径 | 说明 |
|---------|---------|------|
| Coze | `/v1/workflow/run/{id}` | 固定，由适配器内置 |
| Platform | `/api/video/projects/{id}` | 可配置，管理员按实际平台填写 |

查询时适配器将 `{id}` 替换为实际的 `remote_project_id`，拼接 `base_url` 后发起请求。

---

## 状态同步方案

| 方式 | 触发时机 | 说明 |
|------|---------|------|
| **Webhook 回调** | 上游主动推送 | 快，但可能丢失 |
| **查询时透传** | 用户 `GET /projects/:id` | 兜底，每次查询都同步 |

**不实现定时轮询**。用户本来就会轮询查询接口，查询即同步，不需要额外的后台任务。

### 查询时透传逻辑

```
GET /api/video-generation/projects/:id
  ↓
查本地 video_projects 得到 channel_id + remote_project_id + status
  ↓
status 是终态（FAILED / ONE_CLICK_GENERATED）？
  ├─ 是 → 直接返回本地数据，不查上游
  └─ 否
       ↓
     remote_project_id 为空（CREATED 且上游调用尚未完成）？
       ├─ 是 → 直接返回本地数据（上游还没分配 ID，无法查询）
       └─ 否
            ↓
          根据 channel_id 加载 VideoChannel 配置
            ↓
          构造对应 Adapter 调用 GetProjectStatus(remote_project_id)
            ↓
          更新本地 DB（状态、进度、结果字段）
            ↓
          返回最新数据
```

**终态定义**（不再查询上游）：
- `FAILED`
- `ONE_CLICK_GENERATED`

**需要透传的状态**（每次查询都调上游同步）：
- `RUNNING`
- `VIDEO_PROCESSING`
- `VIDEO_CONCAT`

**不透传的状态**：
- `CREATED`：上游调用在创建流程中同步发出，若调用成功则立即更新为 `RUNNING`，若失败则为 `FAILED`。查询时遇到 `CREATED` 说明 `remote_project_id` 还未写入，直接返回本地数据。
- `VIDEO_PREPARING`：本地拼接失败状态，由 new-api 内部流程产生，上游 API 此时已是 `succeeded`。再查上游只会得到成功响应，无法反映拼接失败原因。该状态需管理员手动介入重试，不透传上游。

---

## Webhook 路由设计

Webhook 路由按 `channel_id` 区分，精确找到回调来自哪个渠道实例：

```
POST /api/video-generation/webhook/{channel_id}
```

处理逻辑：
1. 通过 `channel_id` 加载 `VideoChannel` 配置
2. 找不到渠道 → 返回 **200** + 记录日志（防止上游无限重试）
3. 构造对应 Adapter 验证签名；签名验证失败 → 返回 **401**
4. 解析载荷，通过 `channel_id + remote_project_id` 查找本地 `video_projects` 记录
5. 找不到对应项目记录 → 返回 **200** + 记录日志（可能是已删除或数据不一致，同样不触发重试）
6. 更新状态字段

---

## API 接口规范

### 用户接口

```
POST   /api/video-generation/create              创建项目
GET    /api/video-generation/projects            我的项目列表
GET    /api/video-generation/projects/:id        项目详情（含状态透传）
DELETE /api/video-generation/projects/:id        删除项目
```

**创建请求新增字段**：

```json
{
  "channel_id": 3,        // 可选，仅管理员生效；普通用户传了直接忽略
  "channel_type": "coze", // 可选，过滤渠道类型；channel_id 存在时忽略此字段
  // ... 其他现有字段不变
}
```

> `channel_id` 对普通用户**静默忽略**（不报错），渠道选择始终走分组过滤 + 权重随机逻辑。管理员可通过 `channel_id` 强制指定渠道。

### 管理员接口

管理员与普通用户**共用同一套路由**，通过 middleware 注入的 `role` 字段分流，不另开路由前缀（与现有 new-api 日志、用户接口的设计一致）：

```
GET    /api/video-generation/projects            用户：我的项目列表
                                                 管理员：所有项目列表（role >= RoleAdminUser）
GET    /api/video-generation/projects/:id        用户：只能查自己的项目
                                                 管理员：可查任意项目
DELETE /api/video-generation/projects/:id        用户：只能删自己的项目
                                                 管理员：可删任意项目（统一路由，无需重复）
PUT    /api/video-generation/admin/projects/:id/status  手动更新状态（仅管理员路由）
```

Controller 内通过 `c.GetInt("role") >= common.RoleAdminUser` 判断是否管理员，与现有实现一致。

### 渠道管理接口（管理员）

```
GET    /api/video-generation/channels            渠道列表
POST   /api/video-generation/channels            创建渠道
PUT    /api/video-generation/channels/:id        更新渠道
DELETE /api/video-generation/channels/:id        删除渠道
PUT    /api/video-generation/channels/:id/status 启用/禁用
```

### Webhook 接口

```
POST   /api/video-generation/webhook/:channel_id  上游状态回调
```

---

## 分层架构

```
router/
  └─ video-router.go          # 路由注册

controller/
  ├─ video_generation.go      # 项目 CRUD API
  └─ video_channel.go         # 渠道管理 API（新建）

service/
  ├─ video_adapter.go              # 适配器接口（构造函数接收 *VideoChannel）
  ├─ video_adapter_coze.go         # Coze 实现
  ├─ video_adapter_platform.go     # Platform 实现（支持 status_query_path）
  └─ video_generation_service.go   # 业务逻辑（含渠道选择逻辑）

model/
  ├─ video_project.go         # 项目模型（新增 channel_id）
  └─ video_channel.go         # 渠道模型（新建）

dto/
  └─ video_project.go         # DTO（更新）
```

---

## 渠道选择逻辑（伪代码）

```go
// selectChannel 选择渠道
// - isAdmin=true 时 reqChannelId 生效，否则忽略
// - reqChannelType 可选，用于过滤渠道类型；channel_id 优先，同时传时 channel_type 被忽略
// - 渠道调用失败直接返回错误，不自动切换其他渠道
func selectChannel(userGroup string, isAdmin bool, reqChannelId int, reqChannelType string) (*VideoChannel, error) {
    // 1. 管理员指定 channel_id（普通用户传了直接忽略）
    if isAdmin && reqChannelId > 0 {
        ch, err := GetVideoChannelById(reqChannelId)
        if err != nil {
            return nil, ErrChannelNotFound
        }
        if !ch.Enabled {
            return nil, ErrChannelDisabled
        }
        return ch, nil
        // 注：管理员指定渠道时不校验 groups，允许跨组操作
    }

    // 2. 按用户组 + 可选渠道类型过滤，按权重随机选
    // 注意：groups 字段不使用 SQL LIKE 或 FIND_IN_SET（FIND_IN_SET 是 MySQL 专有，不兼容 PostgreSQL/SQLite）
    // 实现方式：先 SELECT 所有 enabled 渠道，在 Go 内存中用 strings.Split 精确匹配 userGroup
    // 数据量小（渠道通常 <100），内存过滤无性能问题
    channels, err := GetEnabledVideoChannelsForGroup(userGroup, reqChannelType)
    if err != nil {
        return nil, err
    }
    if len(channels) == 0 {
        return nil, ErrNoAvailableChannel
    }
    return weightedRandom(channels), nil
}
```

**渠道调用失败处理**：

选中渠道后调用上游 API 失败，**直接返回错误给用户，不自动切换其他渠道**。原因：
- 负载均衡是选渠道策略，不是容错策略
- 自动切换会隐藏上游故障，难以排查
- 如需容错，管理员手动禁用故障渠道即可

用户收到的错误示例：
```json
{"code": 500, "msg": "upstream channel error: connection timeout", "data": null}
```

---

## 前端改动（管理后台）

### 移除

- 系统设置 → Content → Video Generation（单渠道配置 section）

### 新增

- 系统设置 → 渠道管理 → 视频生成渠道（列表 + 增删改查，类似现有 AI 渠道管理页）

表单字段：
- 名称、渠道类型（下拉：coze / platform）、Base URL、API Key、API Secret
- Workflow ID（仅 Coze 渠道显示）
- 创建接口路径 `create_path`（两种渠道均显示，占位提示默认值）
- 状态查询路径 `status_query_path`（**两种渠道均显示**，占位提示默认值；字段为空则运行时回退默认值）
- 用户组、权重、备注

---

## 待清理内容（第二阶段引入，需回滚）

| 内容 | 位置 | 操作 |
|------|------|------|
| 9个 `VideoGeneration*` 全局变量 | `common/constants.go` | 删除 |
| 对应 option 注册 | `model/option.go` InitOptionMap | 删除 |
| 对应 option case | `model/option.go` updateOptionMap | 删除 |
| `GetEnvOrDefaultString` 调用 | service 适配器文件 | 已清理 |
| `VideoGenerationSettingsSection` | `web/.../content/` | 删除文件 |
| ContentSettings 中9个字段 | `web/.../types.ts` | 删除 |
| section-registry 中 video-generation 注册 | `section-registry.tsx` | 删除 |

---

## 实现步骤（按优先级）

1. **model/video_channel.go** — **新建**表（含 groups、create_path、status_query_path 字段）
2. **model/video_project.go** — 新增 channel_id 字段
3. **model/main.go** — 在 AutoMigrate 中添加 `&VideoChannel{}`
4. **service/video_adapter.go** — 构造函数改为接收 `*model.VideoChannel`
5. **service/video_adapter_coze.go** — 从 VideoChannel 读配置
6. **service/video_adapter_platform.go** — 从 VideoChannel 读配置 + 支持 status_query_path
7. **service/video_generation_service.go** — 渠道选择逻辑
8. **dto/video_project.go** — 新增 channel_id 请求字段
9. **controller/video_channel.go** — 渠道 CRUD（新建）
10. **controller/video_generation.go** — 创建时选渠道、查询时透传
11. **router/video-router.go** — 注册渠道管理路由，webhook 路由改 channel_id
12. **清理第二阶段引入的单渠道配置代码**
13. **前端：渠道管理页面**

---

## 待清理内容（第二阶段引入）

> ✅ 已于第三阶段实现时全部清理完毕。

---

## 编译状态

| 阶段 | 时间 | 结果 |
|------|------|------|
| 第一阶段（基础实现） | 2026-07-01 | ✅ 通过 |
| 第二阶段（动态配置） | 2026-07-02 | ✅ 通过 |
| 第三阶段（多渠道管理） | 2026-07-02 | ✅ 通过 `go build ./...` |
| 第四阶段（前端渠道管理页面） | 待实现 | — |
