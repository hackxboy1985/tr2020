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
| `groups` | VARCHAR(255) | 逗号分隔，空=不限组（所有组可用） |
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

**各渠道类型的默认路径**：

| channel_type | create_path 默认值 | status_query_path 默认值 |
|-------------|-------------------|------------------------|
| `coze` | `/v1/workflow/run` | `/v1/workflow/run/{id}` |
| `platform` | `/api/video/create` | `/api/video/projects/{id}` |

**示例数据**：

```
id=1  name="Coze-A"     type=coze      workflow_id=111  create_path=/v1/workflow/run  status_query_path=/v1/workflow/run/{id}     groups="default,vip"  weight=3
id=2  name="Coze-B"     type=coze      workflow_id=222  create_path=/v1/workflow/run  status_query_path=/v1/workflow/run/{id}     groups="vip"          weight=1
id=3  name="Platform-A" type=platform  base_url=http://p1  create_path=/api/video/create  status_query_path=/api/video/projects/{id}  groups=""          weight=2
id=4  name="Platform-B" type=platform  base_url=http://p2  create_path=/api/video/create  status_query_path=/api/video/projects/{id}  groups="default"   weight=2
```

### video_projects 表（新增字段）

在现有字段基础上新增：

| 字段 | 类型 | 说明 |
|------|------|------|
| `channel_id` | INT | 实际使用的渠道 ID，关联 video_channels.id |

> `channel_type` 和 `remote_project_id` 字段保留，`channel_type` 通过 `channel_id` 关联获取。

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
查本地 video_projects 得到 channel_id + remote_project_id
  ↓
项目是终态（FAILED / ONE_CLICK_GENERATED）？
  ├─ 是 → 直接返回本地数据
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

**进行中状态**（每次查询都透传）：
- `CREATED`（刚创建还没调上游，跳过透传）
- `COZE_RUNNING`
- `VIDEO_PROCESSING`
- `VIDEO_CONCAT`
- `VIDEO_PREPARING`

---

## Webhook 路由设计

Webhook 路由改为按 `channel_id` 区分（而非 `channel_type`），这样可以精确找到回调来自哪个渠道实例：

```
POST /api/video-generation/webhook/{channel_id}
```

处理逻辑：
1. 通过 `channel_id` 加载 `VideoChannel` 配置
2. 构造对应 Adapter 验证签名 + 解析载荷
3. 通过 `remote_project_id` 找到本地 `video_projects` 记录
4. 更新状态字段

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
  "channel_id": 3,        // 可选，指定渠道（管理员可见；普通用户一般不传）
  "channel_type": "coze", // 可选，过滤渠道类型；与 channel_id 互斥
  // ... 其他现有字段不变
}
```

### 管理员接口

```
GET    /api/video-generation/projects            所有项目（加 is_admin 标识）
GET    /api/video-generation/projects/:id        项目详情
PUT    /api/video-generation/admin/projects/:id/status  手动更新状态
DELETE /api/video-generation/admin/projects/:id  删除项目
```

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
func selectChannel(userGroup string, reqChannelId int, reqChannelType string) (*VideoChannel, error) {
    // 1. 直接指定渠道 ID
    if reqChannelId > 0 {
        ch := GetVideoChannelById(reqChannelId)
        // 校验该渠道对用户组可用
        if ch.groups != "" && !ch.hasGroup(userGroup) {
            return nil, ErrNoPermission
        }
        return ch, nil
    }

    // 2. 按组 + 可选类型过滤，按权重随机选
    channels := GetEnabledVideoChannels(userGroup, reqChannelType)
    if len(channels) == 0 {
        return nil, ErrNoAvailableChannel
    }
    return weightedRandom(channels), nil
}
```

---

## 前端改动（管理后台）

### 移除

- 系统设置 → Content → Video Generation（单渠道配置 section）

### 新增

- 系统设置 → 渠道管理 → 视频生成渠道（列表 + 增删改查，类似现有 AI 渠道管理页）

表单字段：
- 名称、渠道类型（下拉）、Base URL、API Key、API Secret、Workflow ID（Coze时显示）
- 状态查询路径（Platform时显示，默认值 `/api/video/projects/{id}`）
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

1. **model/video_channel.go** — 更新表结构（加 groups、status_query_path）
2. **model/video_project.go** — 新增 channel_id 字段
3. **model/main.go** — AutoMigrate 已包含 VideoChannel ✅
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

## 编译状态

| 阶段 | 时间 | 结果 |
|------|------|------|
| 第一阶段（基础实现） | 2026-07-01 | ✅ 通过 |
| 第二阶段（动态配置） | 2026-07-02 | ✅ 通过 `go build ./...` |
| 第三阶段（多渠道管理） | 待实现 | — |
