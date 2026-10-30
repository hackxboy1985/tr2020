# Seedance 第 2 海外渠道（sdos2）接入配置

> 本文档说明如何将 `seedance-gateway`（第 2 个海外渠道，版本标识 **sdos2**）接入 new-api 作为上游视频通道。
> **配置项在文首，其余为背景与排障说明。**

---

## 一、配置项速查（重点）

### 1.1 渠道基础字段（不是 other）

| 字段 | 值 | 说明 |
|---|---|---|
| 渠道类型 | **54（DoubaoVideo）** | sdos2 复用 Doubao 视频适配器 |
| 渠道 Base URL | `http://106.54.45.168/seedance-gateway` | 上游网关地址，**含路径前缀** |
| 密钥 | 上游签发的 API Key | 形如 `sk-xxx` |
| 模型 | `doubao-seedance-2-0-260128` | **下游可见的模型名**（不是上游真实模型名） |
| 模型映射 `model_mapping` | `{"doubao-seedance-2-0-260128": "seedance-2.0-B-720p"}` | 下游名 → 上游名，用于匹配上游真实模型并复用现有计费表 |

### 1.2 渠道「其他配置」JSON

**最简配置（推荐）**

```json
{
  "asset_upstream_version": "sdos2",
  "seedance_asset_base_url": "http://106.54.45.168/seedance-gateway"
}
```

**完整配置**

```json
{
  "asset_upstream_version": "sdos2",
  "seedance_asset_base_url": "http://106.54.45.168/seedance-gateway",
  "sdos2_relay_base_url": "https://api.你的域名.com",
  "sdos2_video_content_public": false
}
```

### 1.3 配置项逐项说明

| 配置项 | 必填 | 默认值 | 说明 |
|---|---|---|---|
| `asset_upstream_version` | ✅ | `gateway` | **必须填 `sdos2`**。该值同时决定：① 选用 `Sdos2AssetAdapter` 处理素材接口；② 视频路径自动推断为 `/v1/videos` |
| `seedance_asset_base_url` | ✅ | 空 | 素材接口基址，填上游网关完整地址（含 `/seedance-gateway` 前缀）。留空时回退到渠道 Base URL |
| `sdos2_relay_base_url` | ❌ | 系统 `ServerAddress` | **本服务对外基址**，用于拼接返回给下游的成片下载地址。多渠道指向不同入口时必填 |
| `sdos2_video_content_public` | ❌ | `false` | **成片下载是否免鉴权**。`false`=需要 API Key；`true`=可匿名访问（可直接外链分享） |

### 1.4 配置完成后自动生效的行为

| 环节 | 下游调用 | 实际转发到上游 |
|---|---|---|
| 视频生成 | `POST /v1/videos` | `POST {base}/v1/videos` |
| 视频查询 | `GET /v1/videos/{id}` | `GET {base}/v1/videos/{id}` |
| 成片下载 | `GET /v1/videos/{id}/content` | `GET {base}/v1/videos/{id}/content`（服务端带 Key 拉流） |
| 创建素材组 | `POST /api/seedance/asset-groups` | `POST {base}/seedance/media/groups` `{"action":"create"}` |
| 上传素材 | `POST /api/seedance/assets` | `POST {base}/seedance/media` `{"action":"import"}` |
| 查询素材 | `GET /api/seedance/assets/{id}` | `POST {base}/seedance/media` `{"action":"get"}` |

> 下游接口**零改动**：OpenAI 风格（`/api/seedance/assets`）与官方 Action 风格（`/api/seedance/assets/v2/?Action=CreateAsset`）均保持原样。

### 1.5 计费配置

sdos2 **不返回** token / usage 数据（上游文档明确「本版不提供真实生成 token」），因此计费走**按模型+分辨率+时长查表**，而非按 token 结算。

配置方式：

1. 管理员在「模型倍率」中为 `doubao-seedance-2-0-260128` 配置基准费率（**720p 不含视频输入**的价格）
2. 系统按 `videoPriceMap` 自动折算其他分辨率档位

```go
// relay/channel/task/doubao/constants.go
"doubao-seedance-2-0-260128": {
    Resolution480P:  {46.0, 28.0},   // 不含视频, 含视频
    Resolution720P:  {46.0, 28.0},
    Resolution1080P: {51.0, 31.0},
    Resolution4K:    {26.0, 16.0},
},
```

**关键点**：计费查询用的是 `OriginModelName`（映射**前**的下游模型名 `doubao-seedance-2-0-260128`），不是上游的 `seedance-2.0-B-720p`。所以现有价格表可直接复用，无需新增档位。

⚠️ **下游必须传 `size`**（如 `1280x720`），否则分辨率解析失败，倍率折算不生效，会退回基础费率。

---

## 二、典型配置示例

```
渠道类型：54 DoubaoVideo
渠道名称：Seedance sdos2
Base URL：http://106.54.45.168/seedance-gateway
密钥：sk-xxxxx
模型：doubao-seedance-2-0-260128
模型映射：{"doubao-seedance-2-0-260128": "seedance-2.0-B-720p"}

其他配置：
{
  "asset_upstream_version": "sdos2",
  "seedance_asset_base_url": "http://106.54.45.168/seedance-gateway"
}
```

---

## 三、上游接口契约（背景说明）

上游 `seedance-gateway` 提供两套接口：

### 3.1 视频接口（OpenAI Video 兼容）

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/v1/videos` | 创建视频任务 |
| GET | `/v1/videos/{id}` | 查询任务 |
| GET | `/v1/videos/{id}/content` | 下载成片（需 Bearer） |
| GET | `/v1/videos` | 任务列表 |

状态值：`queued` → `in_progress` → `completed` / `failed`

### 3.2 素材接口（Action 风格）

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/seedance/media/groups` | 素材组增删改查（`action`: create/get/update/delete/list） |
| POST | `/seedance/media` | 素材导入与查询（`action`: import/get/batch_get/update/delete/search/list） |
| GET | `/seedance/media` | 素材分页列表 |
| POST | `/seedance/media/upload` | 二进制上传（本项目未使用，下游只传 URL） |

素材状态值：`queued` / `running` / **`succeeded`** / `failed`（**与视频状态不同**）

---

## 四、素材字段映射

`Sdos2AssetAdapter` 负责下游字段与上游字段的双向转换：

### 4.1 请求映射

| 下游字段 | 上游字段 | 说明 |
|---|---|---|
| `GroupId` | `group_id` | 素材组 ID |
| `URL` | `url` | 素材源地址，**上游自己去抓取** |
| `AssetType` | `media_type` | `Image`→`image`、`Video`→`video`、`Audio`→`audio` |
| `Name` | `title` | 素材标题 |
| — | `action` | 自动补 `"import"` / `"get"` 等 |

### 4.2 响应映射

| 上游字段 | 下游字段 | 说明 |
|---|---|---|
| `id` | `Id` | 素材 ID（`task_` 开头，见下方说明） |
| `asset_uri` | `URL` | `asset://task_xxx`，用于视频生成引用 |
| `media_type` | `AssetType` | |
| `title` | `Name` | |
| `group_id` | `GroupId` | |
| `status` | `Status` | 需状态归一，见 4.3 |

### 4.3 状态值归一（重要）

| 上游状态 | 下游状态 | 说明 |
|---|---|---|
| `succeeded` | **`Active`** | 只有此状态素材可被引用 |
| `queued` / `running` | `Processing` | 处理中 |
| `failed` | `Failed` | 失败 |

> 上游素材 ID 以 `task_` 开头（与视频任务同命名空间），通过 `model: "media-service"` 字段区分。这是上游约定，不是 bug。

---

## 五、成片下载地址改造

### 5.1 问题

上游返回的成片地址是**受鉴权保护的相对路径**：

```json
{"content": {"video_url": "/v1/videos/task_xxx/content"}}
```

直接给下游会导致：
1. 相对路径无法访问
2. 即使拼上上游域名，下游也没有上游 API Key

### 5.2 方案

服务端代理拉流：

```
下游 → GET {sdos2_relay_base_url}/v1/videos/{public_task_id}/content
        ↓ VideoProxyPublic（可选鉴权）
        ↓ 查询任务 → 找到渠道 → 拼上游真实地址
上游 → GET {base}/v1/videos/{upstream_task_id}/content
        Header: Authorization: Bearer {上游Key}
        ↓ 流式转发
下游 ← 视频字节流
```

**下游拿到的始终是本服务的绝对地址，上游 Key 不外泄。**

### 5.3 鉴权开关行为

| `sdos2_video_content_public` | 行为 |
|---|---|
| `false`（默认） | 需要 API Key；只能下载自己的任务（用户隔离） |
| `true` | 可匿名访问；按 task_id 直接定位任务，**不校验归属** |

> ⚠️ 开启 `true` 后，任何拿到 task_id 的人都能下载该视频。task_id 是随机生成的（`task_` + 随机串），但并非不可猜测，请评估风险后开启。

---

## 六、实现代码位置

| 功能 | 文件 |
|---|---|
| 素材适配器 | `service/asset_adapter_sdos2.go` |
| 适配器注册 | `service/asset_adapter_factory.go`（`case "sdos2"`） |
| 视频路径推断 | `relay/channel/task/doubao/adaptor.go`（`Init` 的 `case "sdos2"`） |
| 相对路径归一化 | `relay/channel/task/doubao/adaptor.go`（`resolveResultURL`） |
| 下游地址替换 | `relay/channel/task/doubao/adaptor.go`（`ConvertToOpenAIVideo`） |
| 成片代理（可选鉴权） | `controller/video_proxy_public.go` |
| 上游拉流分支 | `controller/video_proxy.go`（DoubaoVideo / VolcEngine case） |
| 可选鉴权中间件 | `middleware/auth.go`（`OptionalTokenOrUserAuth`） |
| 匿名任务查询 | `model/task.go`（`GetByTaskIdPublic`） |
| 配置字段定义 | `dto/channel_settings.go` |

---

## 七、排障

| 现象 | 原因 | 处理 |
|---|---|---|
| 视频生成 404 | `asset_upstream_version` 没填 `sdos2` | 会落到 default 分支推断 `/api/v3/contents/generations/tasks`，检查配置 |
| 素材接口 404/400 | 误配 `asset_upstream_version: "gateway"` | 那是咪咕契约 `/api/seedance/proxy/assets`，改为 `sdos2` |
| 上游收到错误模型名 | `model_mapping` 方向写反 | 应为 `{"下游名": "上游名"}` |
| 计费倍率不生效 | 下游未传 `size` | 请求体补 `"size": "1280x720"` |
| 素材一直 `Processing` | 上游还在处理 | 每 2 秒查询 `GET /api/seedance/assets/{id}` 直到 `Active` |
| 下载 401 | 渠道未开免鉴权 | 下游带 API Key，或配 `sdos2_video_content_public: true` |
| 成片地址是 localhost | `ServerAddress` 未配 | 配 `sdos2_relay_base_url` 或系统 `ServerAddress` |

---

## 八、验证记录

实测环境：`http://106.54.45.168/seedance-gateway`（2026-10-29）

| 验证项 | 结果 |
|---|---|
| 模型 `seedance-2.0-B-720p` 可用 | ✅ |
| 素材 URL 导入 → 轮询 → `succeeded` | ✅ |
| 单图参考生成 5 秒 | ✅ h264 1280×720 / AAC |
| 同一素材复用生成 3 次 | ✅ |
| 双图参考生成 10 秒 | ✅ 300 帧 |
| 双图参考生成 15 秒 | ✅ 450 帧 |
| 成片生命周期 | 完成后 7 天，过期 410 |

测试产物位于 `docs/大尺度sd/tools/`，测试脚本 `seedance-2.0-B.sh`。
