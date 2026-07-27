# 多层中继部署设计方案

> 文档版本：2026-07-27
> 适用场景：用户 → 服务1 → 服务2 → 服务3 → Gateway/Ark 上游
> 前提：服务1、服务2、服务3 部署同一套 new-api 代码，配置不同

---

## 一、需求概述

将 new-api 部署为多层中继架构，支持任意深度的服务链。每一层服务对上层屏蔽下层实现细节，对下层表现为普通用户。

```
用户
 │  POST /v1/video/generations
 │  GET  /v1/videos/:task_id
 │  POST /api/seedance/assets
 │  GET  /api/seedance/assets/:id
 │  ...
 ▼
服务1（new-api）  ← 用户直接接入，负责计费、限流、用户管理
 │
 ▼
服务2（new-api）  ← 中间层，可叠加多层
 │
 ▼
服务3（new-api）  ← 最靠近上游
 │
 ▼
Seedance Gateway（port 9444）+ Ark 上游
```

---

## 二、接口覆盖范围

本方案覆盖 `seedance-asset-simple.md` 与 `seedance-asset-api.md` 中的全部接口：

| 接口 | 类型 | 中继方向 |
|------|------|---------|
| `POST /v1/video/generations` | 视频生成 | 服务1 → 服务2 → 服务3 → Ark |
| `GET /v1/videos/:task_id` | 视频查询 | 服务1 → 服务2 → 服务3 → Ark |
| `POST /api/seedance/asset-groups` | 素材组创建 | 服务1 → 服务2 → 服务3 → Gateway |
| `GET /api/seedance/asset-groups` | 素材组列表 | **本地 DB 查询，不中继** |
| `GET /api/seedance/asset-groups/:id` | 素材组详情 | 服务1 → 服务2 → 服务3 → Gateway |
| `PUT/PATCH /api/seedance/asset-groups/:id` | 素材组修改 | 服务1 → 服务2 → 服务3 → Gateway |
| `DELETE /api/seedance/asset-groups/:id` | 素材组删除 | 服务1 → 服务2 → 服务3 → Gateway |
| `POST /api/seedance/assets` | 素材创建 | 服务1 → 服务2 → 服务3 → Gateway |
| `GET /api/seedance/assets` | 素材列表 | **本地 DB 查询，不中继** |
| `GET /api/seedance/assets/:id` | 素材状态查询 | 服务1 → 服务2 → 服务3 → Gateway |
| `PUT/PATCH /api/seedance/assets/:id` | 素材修改 | 服务1 → 服务2 → 服务3 → Gateway |
| `DELETE /api/seedance/assets/:id` | 素材删除 | 服务1 → 服务2 → 服务3 → Gateway |
| `POST /api/seedance/face-verifications` | 人脸认证创建 | 服务1 → 服务2 → 服务3 → Gateway |
| `GET /api/seedance/face-verifications/:id` | 人脸认证查询 | 服务1 → 服务2 → 服务3 → Gateway |

**列表接口（`GET .../asset-groups`、`GET .../assets`）不需要中继**：每层服务查询自己的本地数据库，返回该层用户的数据。服务1的用户在服务1看到自己的素材列表，与下游无关。

---

## 三、接口分类与现状

### 2.1 视频生成接口

| 接口 | 说明 |
|------|------|
| `POST /v1/video/generations` | 提交视频生成任务 |
| `GET /v1/videos/:task_id` | 查询视频任务状态及结果 |

**现状**：服务内部的 doubao-video 渠道发往上游时，硬编码为 Ark 原生路径和格式：
- 生成：`POST {baseURL}/api/v3/contents/generations/tasks`
- 轮询：`GET  {baseURL}/api/v3/contents/generations/tasks/{task_id}`

**new-api 对外暴露的格式**（服务2作为被调方）：
- 生成：`POST /v1/video/generations`（支持标准格式与 Ark 原生 `content[]` 格式双入参）
- 查询：`GET  /v1/videos/:task_id`，返回 `OpenAIVideo` 结构

### 3.2 素材接口（需中继的部分）

| 接口 | 说明 |
|------|------|
| `POST /api/seedance/assets` | 创建素材 |
| `GET  /api/seedance/assets/:id` | 查询素材状态 |
| `PUT/PATCH/DELETE /api/seedance/assets/:id` | 修改/删除素材 |
| `POST /api/seedance/asset-groups` | 创建素材组 |
| `GET /api/seedance/asset-groups/:id` | 查询素材组 |
| `PUT/PATCH/DELETE /api/seedance/asset-groups/:id` | 修改/删除素材组 |
| `POST /api/seedance/face-verifications` | 创建人脸认证任务 |
| `GET  /api/seedance/face-verifications/:id` | 查询人脸认证状态 |

**现状**：服务内部调上游时，硬编码为 Seedance Gateway 原生路径：
- 素材：`POST /api/seedance/proxy/assets`，`GET/PUT/PATCH/DELETE /api/seedance/proxy/assets/{id}`
- 素材组：`POST /api/seedance/proxy/assets/groups`，`GET/PUT/PATCH/DELETE /api/seedance/proxy/assets/groups/{id}`
- 人脸认证：`POST /api/seedance/face-verifications`，`GET /api/seedance/face-verifications/{verification_id}`

**new-api 对外暴露的路径**（服务2作为被调方）与 Gateway 路径**不一致**，是核心问题：

| Gateway 路径（当前调用） | new-api 用户侧路径（服务2暴露） |
|--------------------------|-------------------------------|
| `POST /api/seedance/proxy/assets` | `POST /api/seedance/assets` |
| `GET  /api/seedance/proxy/assets/{id}` | `GET  /api/seedance/assets/{id}` |
| `POST /api/seedance/proxy/assets/groups` | `POST /api/seedance/asset-groups` |
| `GET  /api/seedance/proxy/assets/groups/{id}` | `GET  /api/seedance/asset-groups/{id}` |
| `POST /api/seedance/face-verifications` | `POST /api/seedance/face-verifications`（相同 ✅）|
| `GET  /api/seedance/face-verifications/{verification_id}` | `GET  /api/seedance/face-verifications/:id`（`:id` 目前为数字 ❌）|

---

## 三、需要改动的代码

### 3.1 视频生成 — `relay/channel/task/doubao/adaptor.go`

#### 改动点1：`ParseTaskResult` 兼容 OpenAIVideo 状态值

当前 switch 只处理 Ark 原生状态：

```
succeeded → TaskStatusSuccess（取 content.video_url）
pending / queued → TaskStatusQueued
processing / running → TaskStatusInProgress
failed → TaskStatusFailure
```

新增对 new-api（服务2）返回的 OpenAIVideo 状态的处理：

```
completed → TaskStatusSuccess（取 metadata["url"]）
in_progress → TaskStatusInProgress
queued → TaskStatusQueued（已有，但原来归属于 Ark 的 queued）
failed → TaskStatusFailure（error 结构相同，已兼容）
```

**识别方式**：两套状态值完全不重叠（`succeeded` vs `completed`，`processing` vs `in_progress`），switch 天然区分，无歧义。

#### 改动点2：`responseTask` 结构体补充 `Metadata` 字段

服务2返回的 OpenAIVideo 中视频地址在 `metadata.url`，需在 `responseTask` 中增加字段捕获：

```go
Metadata map[string]interface{} `json:"metadata,omitempty"`
```

#### 改动点3：`ConvertToOpenAIVideo` 兼容双格式取视频 URL

当前从 `dResp.Content.VideoURL` 取视频地址。服务2返回的 data 中该字段为空，需补充从 `dResp.Metadata["url"]` 回退：

```
videoURL = dResp.Content.VideoURL
如果为空 → videoURL = dResp.Metadata["url"]
```

`ConvertToOpenAIVideo` 读的是本地 task 表存储的 `Data` 字段（轮询时存入），所以轮询的存储格式和转换逻辑必须匹配。

#### 无需改动的部分

- `BuildRequestBody`：发出的是 Ark 原生 `content[]` 格式，服务2入口已兼容此格式（`relay/common/relay_utils.go` 第216-268行），无需修改
- `DoResponse`：任务 ID 字段名两侧均为 `id`，兼容
- `BuildRequestURL` / `FetchTask`：路径改动通过渠道配置解决，代码无需修改

**视频接口改动量：1 个文件，约 15 行**

---

### 3.2 素材接口 — 新增配置开关 `seedance_relay_mode`

#### 改动点1：`dto/channel_settings.go`

在 `ChannelOtherSettings` 中新增字段：

```go
// SeedanceRelayMode 为 true 时，素材接口调用下游 new-api 路径（/api/seedance/assets 等）
// 为 false（默认）时，调用 Seedance Gateway 原生路径（/api/seedance/proxy/assets 等）
SeedanceRelayMode bool `json:"seedance_relay_mode,omitempty"`
```

#### 改动点2：`service/seedance_proxy.go`

`SeedanceGatewayChannel` 结构体增加 `RelayMode bool` 字段，`GetSeedanceGatewayChannel` 读取并传递该配置。

`SeedanceProxyRequest` 根据 `RelayMode` 切换路径前缀和鉴权方式：

| | Gateway 模式（RelayMode=false） | Relay 模式（RelayMode=true） |
|-|-------------------------------|----------------------------|
| 鉴权 | `Bearer {Ark key}` | `Bearer {sk- token}` |
| 素材路径 | `/api/seedance/proxy/assets` | `/api/seedance/assets` |
| 素材组路径 | `/api/seedance/proxy/assets/groups` | `/api/seedance/asset-groups` |
| 人脸认证路径 | `/api/seedance/face-verifications` | `/api/seedance/face-verifications` |

人脸认证路径相同，但 GET 查询时服务2侧需补充业务 ID（`fv_xxxxxxxxx`）查询支持（见下）。

#### 改动点3：业务 ID 作为对外 LocalId

**目的**：让中继链路上各层可以直接用业务 ID（`asset-xxxxxxxx`、`group-xxxxxxxx`、`fv_xxxxxxxxx`）互相查询，无需维护跨层的数字 ID 映射。

涉及改动：

1. `controller/seedance.go` — 创建素材/素材组/人脸认证的响应中，`LocalId` 改为返回 `UpstreamAssetID` / `UpstreamGroupID` / `VerificationID`（业务 ID 字符串）
2. `resolveAsset`（素材）— 已支持 `asset-` 前缀字符串查询，无需改动 ✅
3. `resolveAssetGroup`（素材组）— 当前只支持数字 ID，需补充 `group-` 前缀字符串查询
4. `resolveVerification`（人脸认证）— 当前只支持数字 ID，需补充 `fv_` 前缀字符串查询

**向后兼容**：数字 ID 查询路径保留，老用户不受影响。

**素材接口改动量：3-4 个文件，约 100-120 行**

---

## 四、各服务配置差异

> 以三层为例：服务1 → 服务2 → 服务3 → Gateway/Ark

渠道类型均为 **doubao-video（类型54）**，在管理后台「渠道管理」中配置。每个渠道有三个关键字段：**Base URL**、**Key**、**其他（JSON）**。

---

### 服务3（现有生产，直连 Ark，无需改动）

**Base URL：**
```
https://ark.cn-beijing.volces.com
```

**Key：**
```
{Ark API Key}
```

**其他（JSON）：**
```json
{
  "seedance_asset_base_url": "https://gateway-host:9444"
}
```

> `doubao_video_generate_path` 和 `doubao_video_fetch_path` 不填，自动使用默认 Ark 路径，现有生产无需改动。

---

### 服务2（中间层，调服务3）

**Base URL：**
```
https://service3-host:8443
```

**Key：**
```
sk-xxx（在服务3管理后台为服务2创建的 Token）
```

**其他（JSON）：**
```json
{
  "doubao_video_generate_path": "/v1/video/generations",
  "doubao_video_fetch_path": "/v1/videos",
  "seedance_asset_base_url": "https://service3-host:8443",
  "seedance_relay_mode": true
}
```

---

### 服务1（用户接入层，调服务2）

**Base URL：**
```
https://service2-host:8443
```

**Key：**
```
sk-xxx（在服务2管理后台为服务1创建的 Token）
```

**其他（JSON）：**
```json
{
  "doubao_video_generate_path": "/v1/video/generations",
  "doubao_video_fetch_path": "/v1/videos",
  "seedance_asset_base_url": "https://service2-host:8443",
  "seedance_relay_mode": true
}
```

---

### 配置要点总结

| 配置项 | 服务3（直连 Ark） | 服务1/2（中继层） |
|--------|-----------------|-----------------|
| Base URL | Ark 官方地址 | 下一层 new-api 地址 |
| Key | Ark API Key | 下一层的 sk- Token |
| `doubao_video_generate_path` | **不填** | `/v1/video/generations` |
| `doubao_video_fetch_path` | **不填** | `/v1/videos` |
| `seedance_asset_base_url` | 真实 Gateway 地址 | 下一层 new-api 地址 |
| `seedance_relay_mode` | **不填（默认 false）** | `true` |

---

## 五、完整链路流转

### 5.1 视频生成

```
用户 POST /v1/video/generations (Ark原生格式或标准格式)
 ↓
服务1 接收 → 内部转为 Ark 原生格式 → POST {service2}/v1/video/generations
 ↓
服务2 接收（relay_utils.go 自动解析 content[]）→ 内部转为 Ark 原生格式 → POST {service3}/v1/video/generations
 ↓
服务3 接收 → 内部转为 Ark 原生格式 → POST {Ark}/api/v3/contents/generations/tasks
 ↓
Ark 返回 task_id
 ↓
服务3 存本地任务 → 返回 OpenAIVideo{id: task3_xxx, status: queued}
 ↓
服务2 存本地任务（task2_xxx → task3_xxx）→ 返回 OpenAIVideo{id: task2_xxx, status: queued}
 ↓
服务1 存本地任务（task1_xxx → task2_xxx）→ 返回 OpenAIVideo{id: task1_xxx, status: queued}
 ↓
用户收到 task1_xxx
```

### 5.2 视频任务查询

```
用户 GET /v1/videos/task1_xxx
 ↓
服务1 查本地表 → 找到上游 task2_xxx → GET {service2}/v1/videos/task2_xxx
 ↓
服务2 查本地表 → 找到上游 task3_xxx → GET {service3}/v1/videos/task3_xxx
 ↓
服务3 查本地表 → 找到 Ark task_id → GET {Ark}/api/v3/contents/generations/tasks/{ark_task_id}
 ↓
Ark 返回 {status: "succeeded", content.video_url: "https://..."}
 ↓
服务3 ParseTaskResult 识别 succeeded → 取 content.video_url
     ConvertToOpenAIVideo → 返回 OpenAIVideo{status: completed, metadata.url: "https://..."}
 ↓
服务2 ParseTaskResult 识别 completed（新增）→ 取 metadata.url（新增）
     ConvertToOpenAIVideo 兼容双格式 → 返回 OpenAIVideo{status: completed, metadata.url: "https://..."}
 ↓
服务1 ParseTaskResult 识别 completed → 取 metadata.url
     ConvertToOpenAIVideo → 返回给用户 OpenAIVideo{status: completed, metadata.url: "https://..."}
 ↓
用户收到视频地址
```

### 5.3 素材创建

```
用户 POST /api/seedance/assets {URL, AssetType, Name}
 ↓
服务1 (RelayMode=true) → 调 POST {service2}/api/seedance/assets
      服务1 存本地记录（UpstreamAssetID = asset-xxx）
 ↓
服务2 (RelayMode=true) → 调 POST {service3}/api/seedance/assets
      服务2 存本地记录（UpstreamAssetID = asset-xxx）
 ↓
服务3 (RelayMode=false) → 调 POST {Gateway}/api/seedance/proxy/assets
      服务3 存本地记录（UpstreamAssetID = asset-xxx）
 ↓
Gateway → Ark 上游，返回 {Result: {Id: "asset-xxx", Status: "Processing"}}
 ↓
服务3 返回 {Result: {Id: "asset-xxx", LocalId: "asset-xxx", AssetRef: "asset://asset-xxx"}}
 ↓
服务2 返回 {Result: {Id: "asset-xxx", LocalId: "asset-xxx", AssetRef: "asset://asset-xxx"}}
 ↓
服务1 返回 {Result: {Id: "asset-xxx", LocalId: "asset-xxx", AssetRef: "asset://asset-xxx"}}
 ↓
用户收到，LocalId = "asset-xxx"（业务 ID）
```

### 5.4 素材查询

```
用户 GET /api/seedance/assets/asset-xxx
 ↓
服务1 resolveAsset("asset-xxx") → 查本地表 → 找到记录
      (RelayMode=true) → 调 GET {service2}/api/seedance/assets/asset-xxx
 ↓
服务2 resolveAsset("asset-xxx") → 查本地表 → 找到记录
      (RelayMode=false) → 调 GET {service3}/api/seedance/assets/asset-xxx
 ↓
服务3 resolveAsset("asset-xxx") → 查本地表 → 找到记录
      (RelayMode=false) → 调 GET {Gateway}/api/seedance/proxy/assets/asset-xxx
 ↓
返回 {Result: {Id: "asset-xxx", Status: "Active"}}，逐层透传
```

---

## 六、用户侧请求与返回差异分析

### 6.1 视频接口 — 无差异

| 项目 | 现有（直连 Ark） | 改动后（经过中继） |
|------|----------------|-----------------|
| 提交请求格式 | 不变 | 不变 |
| 提交响应格式 | 不变（OpenAIVideo） | 不变 |
| 查询请求格式 | 不变 | 不变 |
| 查询响应格式 | 不变（OpenAIVideo） | 不变 |
| task_id 格式 | `task_xxx` | `task_xxx`（服务1生成） |
| 视频地址字段 | `metadata.url` | `metadata.url`（不变） |

**用户侧：零感知，完全兼容。**

### 6.2 素材接口 — LocalId 格式变化

`seedance-asset-api.md` 当前说明 `:id` 为"本地表主键 ID"（数字），改动后变为业务 ID 字符串。

| 项目 | 现有 | 改动后 |
|------|------|-------|
| 创建素材响应 `Result.LocalId` | `42`（数字） | `"asset-xxxxxxxx"`（业务 ID） |
| 创建素材响应 `Result.AssetRef` | `"asset://asset-xxxxxxxx"` | 不变 |
| 创建素材组响应 `Result.Id` | `"group-xxxxxxxx"` | 不变 |
| 创建素材组 `LocalId`（若有） | 数字 | `"group-xxxxxxxx"` |
| 创建人脸认证响应 `verification_id` | `"fv_xxxxxxxxx"` | 不变（本来就是业务 ID） |
| 查询接口 `:id` 参数 | 仅支持数字 | 支持数字（向后兼容）+ 支持业务 ID |
| 素材列表、素材组列表 | 本地 DB 返回 | 本地 DB 返回（不变） |

**现有用户**：已存储的数字 LocalId（如 `42`）继续有效，查询接口保留数字 ID 支持，不受影响。

**新用户**：使用业务 ID 作为标识符，与 `AssetRef`（`asset://asset-xxx`）、`group_id`、`verification_id` 格式统一，无需额外转换。

---

## 七、风险评估

### 7.1 视频接口

| 风险点 | 级别 | 说明 |
|--------|------|------|
| ParseTaskResult 新增 case | 低 | 纯增量，Ark 状态值与 new-api 状态值无重叠，不影响现有直连逻辑 |
| ConvertToOpenAIVideo 双格式 | 低 | 加回退逻辑，原有逻辑保持不变 |
| 生产服务2（现有）发版影响 | 零 | 生产服务2代码改动向后兼容，直连 Ark 路径不变 |
| 服务1（新部署）上线 | 低 | 全新实例，可充分测试后再接流量 |

### 7.2 素材接口

| 风险点 | 级别 | 说明 |
|--------|------|------|
| LocalId 格式变化 | 低 | 查询接口保留数字兼容，老用户无感 |
| SeedanceRelayMode 开关 | 低 | 默认 false，现有配置不填即保持原有行为 |
| 素材组/人脸认证字符串 ID 支持 | 低 | 增量逻辑，不修改现有数字 ID 路径 |

## 七、各层是否需要发版

| 服务 | 是否需要发版 | 原因 |
|------|------------|------|
| 服务3（现有生产，直连 Ark） | **不需要** | 现有代码已能正确作为被调方：接收 `content[]` 格式、返回 `OpenAIVideo` 结构、支持业务 ID 查询 |
| 服务2（新部署，调服务3） | **需要** | `ParseTaskResult` 和 `ConvertToOpenAIVideo` 需兼容 `OpenAIVideo` 响应格式；新增 `seedance_relay_mode` 配置 |
| 服务1（新部署，调服务2） | **需要** | 同服务2 |

**服务3零改动、零风险，现有用户完全不受影响。**

---

## 八、改动量汇总

| 文件 | 改动内容 | 行数 |
|------|----------|------|
| `relay/channel/task/doubao/adaptor.go` | ParseTaskResult 兼容 completed/in_progress；responseTask 加 Metadata 字段；ConvertToOpenAIVideo 双格式取 URL | ~15 行 |
| `dto/channel_settings.go` | 新增 `SeedanceRelayMode bool` 字段 | ~3 行 |
| `service/seedance_proxy.go` | SeedanceGatewayChannel 加 RelayMode；SeedanceProxyRequest 路径切换逻辑 | ~30 行 |
| `controller/seedance.go` | 创建响应改用业务 ID 作为 LocalId | ~10 行 |
| `model/seedance_asset.go` 相关 | 素材组、人脸认证 resolve 函数补充字符串 ID 查询 | ~20 行 |

**总计：5 个文件，约 80 行，无新文件，无数据库 schema 变更。**
