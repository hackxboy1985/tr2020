# Seedance 素材库 & 人脸认证接入设计文档

## 1. 背景

咪咕提供两个上游服务：

| 服务 | 地址 | 用途 |
|------|------|------|
| Seedance 视频生成 | `sd.dawnloadai.com:8443` | 视频生成、任务查询（**已有实现，无需开发**） |
| Seedance Gateway | `sd.dawnloadai.com:9444` | 素材库（AIGC/数字人）、真人人像库 |

本文档仅涉及 **Gateway（9444）** 的接入。

---

## 2. 整体调用链

```
客户端 (sk-xxx)
  → TokenAuth 验证（用户身份 + user_id + group）
  → 查找该 group 可用的 doubao-video 渠道
  → 取渠道的 SeedanceAssetBaseUrl + Key
  → 代理请求到上游 Gateway，原样透传请求体
  → 同步写/更新本地表
  → 原样透传上游响应给客户端
```

**鉴权说明**：
- 客户端用自己的 NewAPI Token（`sk-xxx`）访问我们的接口
- 我们用渠道配置的 Key（上游 Token）访问 Gateway
- 客户端不感知上游 Key

---

## 3. 渠道配置

在 `Channel` 表（`model/channel.go`）新增一列：

```go
SeedanceAssetBaseUrl string `json:"seedance_asset_base_url" gorm:"type:varchar(512);default:''"`
```

- 渠道类型为 `doubao-video`（ChannelType=54）时，管理员在渠道配置页填写 Gateway 地址，例如 `https://sd.dawnloadai.com:9444`
- `Key` 字段复用（即上游 Token），无需新增
- AutoMigrate 自动加列，存量记录默认空字符串，不影响任何现有渠道逻辑

**前端**：在渠道配置抽屉（`channel-mutate-drawer.tsx`）中，`currentType === 54` 时显示 `seedance_asset_base_url` 专属输入框。

---

## 4. 本地数据库表设计

### 4.1 素材组 `seedance_asset_groups`

| 列名 | 类型 | 说明 |
|------|------|------|
| id | bigint PK AUTO_INCREMENT | |
| user_id | int INDEX | NewAPI 用户 ID |
| channel_id | int INDEX | 所用渠道 ID |
| upstream_group_id | varchar(191) INDEX | 上游返回的 group_id（`Result.Id`） |
| name | varchar(255) | 素材组名称 |
| description | text | 描述 |
| group_type | varchar(50) | `AIGC` 或 `LivenessFace` |
| raw_data | json/text | 上游完整响应原文 |
| created_at | bigint | Unix 时间戳 |
| updated_at | bigint | Unix 时间戳 |
| deleted_at | bigint INDEX | 软删除，0 表示未删除 |

### 4.2 素材 `seedance_assets`

| 列名 | 类型 | 说明 |
|------|------|------|
| id | bigint PK AUTO_INCREMENT | |
| user_id | int INDEX | |
| channel_id | int INDEX | |
| upstream_asset_id | varchar(191) INDEX | 上游返回的 asset_id（`Result.Id`） |
| upstream_group_id | varchar(191) INDEX | 所属素材组 ID |
| name | varchar(255) | 素材名称 |
| asset_type | varchar(20) | `Image` / `Video` / `Audio` |
| source_url | text | 创建时提交的原始 URL |
| status | varchar(30) INDEX | `Processing` / `Active` / `Failed` |
| raw_data | json/text | 上游完整响应原文 |
| created_at | bigint | |
| updated_at | bigint | |
| deleted_at | bigint INDEX | 软删除 |

### 4.3 人脸认证任务 `seedance_face_verifications`

| 列名 | 类型 | 说明 |
|------|------|------|
| id | bigint PK AUTO_INCREMENT | |
| user_id | int INDEX | |
| channel_id | int INDEX | |
| verification_id | varchar(191) UNIQUE | 上游返回的 verification_id |
| status | varchar(30) INDEX | `waiting_user` / `callback_received` / `resolving` / `verified` / `failed` / `expired` |
| h5_url | text | 认证页面链接（展示给终端用户） |
| upstream_group_id | varchar(191) | 认证成功后上游返回的 group_id |
| expires_at | bigint | 过期时间（Unix） |
| raw_data | json/text | 上游完整响应原文 |
| created_at | bigint | |
| updated_at | bigint | |
| deleted_at | bigint INDEX | 软删除（注：人脸认证任务一般不允许用户主动删除，预留） |

> **软删除约定**：`deleted_at = 0` 表示正常，`deleted_at > 0` 表示已删除（值为删除时间戳）。不使用 GORM 的 `gorm.DeletedAt`，与项目现有 Task 等模型保持一致，手动管理。

---

## 5. 接口路由设计

挂在 `new-api` 上，统一前缀 `/api/seedance`，使用 `TokenAuth()` 中间件。

### 素材组

```
POST   /api/seedance/asset-groups          创建素材组
GET    /api/seedance/asset-groups          查询素材组列表
GET    /api/seedance/asset-groups/:id      查询单个素材组
PUT    /api/seedance/asset-groups/:id      全量更新素材组
PATCH  /api/seedance/asset-groups/:id      部分更新素材组
DELETE /api/seedance/asset-groups/:id      删除素材组（软删除）
```

其中 `:id` 是本地表主键 ID，内部转换为上游 `upstream_group_id` 再请求 Gateway。

### 素材

```
POST   /api/seedance/assets                创建素材
GET    /api/seedance/assets                查询素材列表
GET    /api/seedance/assets/:id            查询单个素材
PUT    /api/seedance/assets/:id            全量更新素材
PATCH  /api/seedance/assets/:id            部分更新素材
DELETE /api/seedance/assets/:id            删除素材（软删除）
```

### 人脸认证

```
POST   /api/seedance/face-verifications          创建认证任务
GET    /api/seedance/face-verifications/:id      查询认证任务状态
```

---

## 6. 各接口行为说明

### 6.1 创建素材组（POST /api/seedance/asset-groups）

1. TokenAuth 获取 user_id、group
2. 查找该 group 下启用的 doubao-video 渠道，取 `SeedanceAssetBaseUrl` 和 `Key`
3. 透传请求体 → `POST {GatewayUrl}/api/seedance/proxy/assets/groups`
4. 上游成功（2xx）后：
   - 解析响应取 `Result.Id` 作为 `upstream_group_id`
   - 插入 `seedance_asset_groups` 本地记录
5. 返回上游原始响应给客户端

### 6.2 查询素材组列表（GET /api/seedance/asset-groups）

1. 透传查询参数 → 上游
2. 同时从本地表查询（按 user_id 过滤）
3. **返回上游响应**（本地表数据仅用于管理后台，不影响透传）

> 查询类接口全部透传上游响应，本地表仅供管理使用。

### 6.3 更新素材组（PUT/PATCH /api/seedance/asset-groups/:id）

1. 从本地表按 id + user_id 查 `upstream_group_id`，不存在则 404
2. 透传请求体 → `PUT/PATCH {GatewayUrl}/api/seedance/proxy/assets/groups/{upstream_group_id}`
3. 上游成功后：更新本地记录 `raw_data`、`name`/`description` 等字段、`updated_at`
4. 透传上游响应

### 6.4 删除素材组（DELETE /api/seedance/asset-groups/:id）

1. 从本地表按 id + user_id 查记录
2. 透传 → `DELETE {GatewayUrl}/api/seedance/proxy/assets/groups/{upstream_group_id}`
3. 上游成功后：本地记录设 `deleted_at = now()`
4. 透传上游响应

### 6.5 创建素材（POST /api/seedance/assets）

1. 透传请求体 → `POST {GatewayUrl}/api/seedance/proxy/assets`
2. 上游成功后：解析 `Result.Id` 插入本地 `seedance_assets`，初始 `status = Processing`
3. 透传响应

### 6.6 查询素材状态（GET /api/seedance/assets/:id）

1. 从本地表查 `upstream_asset_id`
2. 透传 → `GET {GatewayUrl}/api/seedance/proxy/assets/{upstream_asset_id}`
3. 解析上游响应中的 `Result.Status`，**更新本地表 status 字段**
4. 透传响应

### 6.7 删除素材（DELETE /api/seedance/assets/:id）

同素材组删除逻辑，软删除本地记录。

### 6.8 创建人脸认证任务（POST /api/seedance/face-verifications）

1. 透传请求体 → `POST {GatewayUrl}/api/seedance/face-verifications`
2. 上游成功后：解析 `verification_id`、`h5_url`、`expires_at`，插入本地记录（`status = waiting_user`）
3. 透传响应

### 6.9 查询人脸认证任务（GET /api/seedance/face-verifications/:id）

1. 从本地表按 id + user_id 查 `verification_id`
2. 透传 → `GET {GatewayUrl}/api/seedance/face-verifications/{verification_id}`
3. 解析上游响应 `status`，若变化则更新本地表（包括 `verified` 后写入 `upstream_group_id`）
4. 透传响应

---

## 7. 渠道选择逻辑

```go
// 伪代码
func getSeedanceGatewayChannel(userGroup string) (*Channel, error) {
    channels := GetEnabledChannelsByType(ChannelTypeDoubaoVideo, userGroup)
    // 过滤出配置了 SeedanceAssetBaseUrl 的渠道
    // 按 Weight 随机选一个
}
```

若找不到可用渠道，返回 `503 Service Unavailable`。

---

## 8. 计费配置说明

Seedance 2.0 视频生成已有完整的预扣 + 多退少补逻辑，只需在管理后台正确配置模型倍率。

### 配置项

只需配置 **ModelRatio**，无需配置 ModelPrice：

| 模型 | ModelRatio | 说明 |
|------|-----------|------|
| `doubao-seedance-2-0-260128` | `23` | 显示为 ¥46/M tokens |
| `doubao-seedance-2-0-fast-260128` | `18.5` | 显示为 ¥37/M tokens |

### 计费流程

**提交时预扣**（`ModelPriceHelperPerCall`）：
```
没有 ModelPrice 时：预扣 = ModelRatio/2 × 500,000 × groupRatio
= 23/2 × 500,000 × 0.8 = 4,600,000 积分（¥9.2，vip80 分组）
```

**完成时多退少补**（`RecalculateTaskQuotaByTokens`）：
```
实际扣费 = totalTokens × ModelRatio × groupRatio × otherMultiplier
```

**视频输入折扣**（含视频帧输入时自动触发）：
- `doubao-seedance-2-0-260128`：折扣系数 ≈ 0.609（28/46）
- `doubao-seedance-2-0-fast-260128`：折扣系数 ≈ 0.595（22/37）

---

## 9. 文件结构规划

```
controller/
  seedance.go              # 所有后端 handler（素材组、素材、人脸认证）

model/
  seedance_asset.go        # SeedanceAssetGroup、SeedanceAsset 模型 + CRUD
  seedance_face.go         # SeedanceFaceVerification 模型 + CRUD
  channel.go               # 新增 SeedanceAssetBaseUrl 字段

router/
  seedance-router.go       # 路由注册
  main.go                  # 引入 SetSeedanceRouter

service/
  seedance_proxy.go        # Gateway 代理请求公共方法（透传、错误处理）

web/default/src/
  pages/SeedanceAssets/    # 管理后台前端页面
    index.tsx              # 素材组 + 素材列表
    FaceVerifications.tsx  # 人脸认证任务列表
```

---

## 10. 管理后台前端页面

路由挂在管理后台，仅管理员可见。

### 10.1 素材库管理页（/console/seedance/assets）

**素材组列表**：
- 展示字段：ID、上游 group_id、名称、类型（AIGC/LivenessFace）、用户、创建时间、状态
- 操作：查看详情、软删除

**素材列表**（可按素材组过滤）：
- 展示字段：ID、上游 asset_id、所属组、名称、类型（Image/Video/Audio）、状态（Processing/Active/Failed）、用户、创建时间
- 操作：查看详情、软删除

### 10.2 人脸认证管理页（/console/seedance/face-verifications）

**认证任务列表**：
- 展示字段：ID、verification_id、用户、状态、H5 链接、认证成功后的 group_id、过期时间、创建时间
- 操作：查看详情（只读）

---

## 11. 不在本期范围内

- 素材状态自动轮询（由客户端自行轮询查询接口，服务端查询时顺带更新）
- 人脸认证 Webhook 回调（上游推送）

---

## 12. 开发检查清单

**后端**
- [ ] `model/channel.go` 新增 `SeedanceAssetBaseUrl` 字段
- [ ] `model/seedance_asset.go` 创建素材组、素材模型及 CRUD
- [ ] `model/seedance_face.go` 创建认证任务模型及 CRUD
- [ ] `model/main.go` AutoMigrate 加入三张新表
- [ ] `service/seedance_proxy.go` 实现代理请求公共方法
- [ ] `controller/seedance.go` 实现所有 handler
- [ ] `router/seedance-router.go` 注册路由
- [ ] `router/main.go` 引入新路由

**前端**
- [ ] 渠道配置抽屉 `channel-mutate-drawer.tsx`：type 54 时显示 `seedance_asset_base_url` 输入框
- [ ] 素材库管理页（素材组列表 + 素材列表）
- [ ] 人脸认证管理页

**配置（上线后管理员操作）**
- [ ] 管理后台配置 `doubao-seedance-2-0-260128` ModelRatio = `23`
- [ ] 管理后台配置 `doubao-seedance-2-0-fast-260128` ModelRatio = `18.5`
- [ ] doubao-video 渠道填写 `SeedanceAssetBaseUrl` = `https://sd.dawnloadai.com:9444`
