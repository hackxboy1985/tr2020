# Seedance 转售渠道（globalaiclient.com）对接分析

> 上游：`https://globalaiclient.com`（文档：《Seedance 客户 API 接入文档》2026-11-10）
> 结论前提：老板要求「先写脚本验证、不改代码」；本文按这个前提给结论，最后单独列最小改动清单。
> 所有结论均来自代码实测，引用格式 `path:line`。

---

## 一、结论速览

| 问题 | 结论 |
|---|---|
| 是不是官方 Ark 格式？ | **视频「原生协议」是官方 Ark 格式**（`POST/GET/DELETE /api/v3/contents/generations/tasks`，§2.1/§3/§4 字段与火山官方一致）；**素材、OpenAI 风格视频、模型/参数配置接口是上游自建扩展**，不是官方格式。 |
| 上游是什么？ | 极可能在 **new-api（或本仓库分支）** 基础上加自建素材库。证据见 §2.4。 |
| 视频能直接接吗？ | **能，零改代码。** 渠道类型 54 + 路径推断正好落到原生 Ark 路径，请求体结构 100% 对齐文档 §4。 |
| 素材能直接接吗？ | **不能。** 现有 5 个适配器没有一个能打 `/v1/assets*`，必须新增 `GlobalAIAssetAdapter`（约 350~450 行 + 工厂注册），或让下游只传公网 URL、彻底不用素材库。 |
| 要不要改代码 | 视频链路 **0 行**；素材链路 **必须新增适配器**；另有 2 处小瑕疵（`expired` 终态、取消只看 HTTP 200）建议一并修。 |
| 预计改动量 | 素材适配器 0.5~1 人日（含单测）；终态/取消 0.2 人日；不接素材库则 0 行。 |

**一句话**：视频链路今天就能用配置接上并跑通（原生 Ark）；素材库是纯新增工作，且「本地上传」在当前 `AssetAdapter` 接口下**根本无法暴露**（接口只有 8 个方法，没有上传方法，见 §3.2）。

---

## 二、视频侧

### 2.1 渠道类型 54 与 adaptor 入口

- `constant/channel.go:54` `ChannelTypeDoubaoVideo = 54`；`relay/relay_adaptor.go:158-159` 把 54 映射到 `taskdoubao.TaskAdaptor`。
- `relay/channel/task/doubao/adaptor.go:131` `Init()` 是路径推断的唯一入口。

### 2.2 `asset_upstream_version` 如何决定路径（adaptor.go:144-213）

优先级：**显式路径 > 按版本推断**。

```
a.videoGeneratePath = settings.DoubaoVideoGeneratePath   // adaptor.go:151
a.videoFetchPath    = settings.DoubaoVideoFetchPath      // adaptor.go:152
upstreamVersion     = settings.AssetUpstreamVersion || "gateway"   // adaptor.go:154-157

switch upstreamVersion {                                 // adaptor.go:172
case "kwjm":           生成 /v1/videos/generations，查询同路径   // adaptor.go:176,180
case "sdos2":          生成 /v1/videos，查询 /v1/videos          // adaptor.go:190,194
case "gateway", default: 生成 /api/v3/contents/generations/tasks
                        查询 /api/v3/contents/generations/tasks // adaptor.go:202,206
}
```

关键点：

1. **`fetch_path` 是「集合路径」，不带尾部 `/{id}`**：`FetchTask` 自己拼 `%s%s/%s`（adaptor.go:546），`CancelTask` 也拼 `%s%s/%s` 后发 DELETE（adaptor.go:825）。所以填 `/api/v3/contents/generations/tasks` 而不是 `.../tasks/{id}`。
2. 请求 URL = 渠道 Base URL + path（adaptor.go:250-256），鉴权 `Bearer <渠道 Key>`（adaptor.go:262），与上游文档 §1 一致。
3. `naxis` / `naxis_rest` 会落到 default 分支 → 同样推断成原生 Ark 路径（adaptor.go:197-209），因此**资产版本配 `naxis_rest` 不会破坏视频路径**。
4. 成片地址策略：
   - `allowNonOfficialVideoURL = 渠道开关 || upstreamVersion == "sdos2"`（adaptor.go:161-162）
   - `enforceOfficialVideoURL = naxis / naxis_rest`（adaptor.go:165-166）→ **这个上游千万别配 naxis/naxis_rest**，否则非 `.volces.com` 的成片会被直接判失败（adaptor.go:741-751，`service/video_url_policy.go:17` 只认 `.volces.com`）。
5. 成片 URL 校验是硬校验：必须是绝对 http(s)；相对路径且渠道不允许 → `UPSTREAM_VIDEO_UNAVAILABLE`（adaptor.go:711-737）。
6. 下游拉流：渠道允许非官链时，下游拿到的 `metadata.url` 会换成本服务 `/v1/videos/{task}/content`，服务端带上游 Key 拉流（adaptor.go:941-961、`service/video_url_policy.go:47-58`、`controller/video_proxy.go:120-133`）。基址取 `sdos2_relay_base_url`，回退 `ServerAddress`（`service/asset_adapter_factory.go:31-41`）。

### 2.3 两种上游协议怎么配

| 方案 | 配置 | 上游实际收到 | 结论 |
|---|---|---|---|
| **A. 原生 Ark（推荐）** | `asset_upstream_version` 缺省 / `"gateway"`，或显式两条 path 都填 `/api/v3/contents/generations/tasks` | `POST /api/v3/contents/generations/tasks`，body 为 Ark `content[]` | **完全对齐文档 §4**，终态 `succeeded` 已被解析（adaptor.go:779-787） |
| B. OpenAI 风格 | `asset_upstream_version: "sdos2"` 或显式 path 填 `/v1/videos` | `POST /v1/videos`，body 仍是 Ark `content[]`（**没有 `prompt`**） | 文档 §8.2/§8.3 只承诺 `prompt`+`metadata.content`，未承诺顶层 `content[]`；但上游若是 new-api 分支，其入口会走 `ValidateBasicTaskRequest` 的「prompt 为空 + content 非空」兼容分支（`relay/common/relay_utils.go:216-272`），**大概率也吃**。必须实测（见 §6） |
| C. 统一视频 `/v1/video/generations` | 无适配方式 | — | **不要用**，`fetch_path` 无法在 `/{id}` 前插 `/video` 前缀时保持两者不同… 实际可配 `generate_path=/v1/video/generations` + `fetch_path=/v1/video/generations`，但返回值/状态是兼容语义，收益不如 A |

**两种协议的状态/字段映射差异（已核对代码）**

| 上游协议 | 上游终态 | 本项目解析位置 | 是否已支持 |
|---|---|---|---|
| 原生 | `succeeded` | adaptor.go:779 | ✅ `TaskStatusSuccess` |
| 原生 | `expired` | adaptor.go:810-814 default 分支 | ❌ **被当成「处理中 30%」**，任务会一直轮询到 `TASK_TIMEOUT_MINUTES`（默认 1440 分钟，`common/init.go:157`）才失败退款 |
| 兼容 | `completed` | adaptor.go:789-798（会兜底读 `metadata.url`） | ✅ |
| 兼容 | `unknown` | adaptor.go:810-814 default | ❌ 同上，当处理中 |
| 兼容 | `queued/in_progress/failed` | adaptor.go:773,799,802 | ✅ |
| 原生 | `cancelled` | adaptor.go:806 | ✅ |
| 原生 | 取片字段 `content.video_url` | adaptor.go:73-76,782 | ✅ |
| 兼容 | 取片字段 `metadata.url`（相对路径） | adaptor.go:793-797 | ✅（需开 `allow_non_official_video_url`） |
| 两者 | `usage.completion_tokens/total_tokens` | adaptor.go:97-103,784-785 | ✅（但本上游是否真给 token 未验证） |

### 2.4 为什么判断上游是 new-api 分支

- 兼容响应字段 `id/task_id/object/model/status/progress/created_at/metadata.url`（文档 §8.4）与本项目 `dto/openai_video.go:17-33` **逐字段相同**；状态枚举 `queued/in_progress/completed/failed/unknown` 与 `dto/openai_video.go:8-15` 完全相同。
- 素材信封 `{success,message,data}` 与本项目 `common.ApiSuccess`（`common/gin.go:213-219`）完全一致。
- `GET /api/v3/contents/generations/tasks` 返回 `{items:[],total:0}`（lead 实测），与本项目 `controller/relay.go:666` `RelayTaskFetchList` 一致。
- `/v1/models` 返回非 Seedance 混合模型（lead 实测），说明是聚合平台，不是火山直连。
- 但它**多出** `/v1/assets*`、`/v1/assets/video-model-params`、`/v1/assets/video-model-audio-params`（本仓库 grep 无此路径），说明是**改造过的分支**。

→ 实践含义：**降级兼容路径（`/v1/videos` 吃 Ark `content[]`）大概率可用**；但素材库是新增私有契约，不能靠猜。

---

## 三、素材侧

### 3.1 下游入口与接口契约

- REST 入口：`router/seedance-router.go:16-29` → `POST/GET /api/seedance/asset-groups`、`POST/GET/PUT/PATCH/DELETE /api/seedance/assets`。
- 官方 Action 兼容入口：`router/video-router.go:58-63` `POST /api/seedance/assets/v2/?Action=...` → `controller/relay_asset.go:18-78` 转调同一批 controller。
- 接口定义：`service/asset_adapter.go:9-33`，**8 个方法**（CreateAssetGroup / GetAssetGroup / UpdateAssetGroup / DeleteAssetGroup / CreateAsset / GetAsset / UpdateAsset / DeleteAsset）。**没有上传方法**。
- 可选的真人核验接口：`service/asset_adapter.go:103-109` `FaceVerificationAdapter`（CreateFaceVerification / GetFaceVerification），靠类型断言启用（`controller/seedance.go:880-909`）。
- 本地表：`model/seedance_asset.go:10-47`，`Status` 注释即 `Processing / Active / Failed`，`AssetType` 为 `Image/Video/Audio`。

### 3.2 现有 5 个适配器 vs 上游 `/v1/assets*`

| 适配器 | 上游路径 | 请求字段 | 响应外壳 | 能复用？ |
|---|---|---|---|---|
| `GatewayAssetAdapter`（`service/asset_adapter_gateway.go:31,146`） | `/api/seedance/proxy/assets/groups`、`/api/seedance/proxy/assets` | PascalCase `Name/GroupId/URL/AssetType` | `{"Result":{...}}`（:45-53） | ❌ 路径、字段、外壳全不同 |
| `Sdos2AssetAdapter`（`service/asset_adapter_sdos2.go:128,162,253`） | `POST /seedance/media*{action}` | 蛇形 `action/url/group_id/media_type/title` | 裸对象 | ❌ 路径不同；状态语义 `succeeded`（:81-92）也与上游 `ready+sync_status` 不同 |
| `KwjmAssetAdapter`（`service/asset_adapter_kwjm.go:33`） | `POST /v3/open/{Action}`（带 `model`） | PascalCase | `{"Id":...}` | ❌ |
| `NaxisActionAssetAdapter`（`service/asset_adapter_naxis_action.go:413-423`） | `POST /?Action=…&Version=2024-01-01` | PascalCase | `{ResponseMetadata,Result}` | ❌ 上游是 `/api/material?Action=…`（文档 §9，行 740），前缀差一层；且 Action 集合只有 5 个，**没有素材组相关 Action** |
| `NaxisAssetAdapter`（`service/asset_adapter_naxis.go:158,181,212,226,256,280,308,322`） | `/v1/asset-groups`、`/v1/assets` | `{url,asset_type,name,group_id}`（:242-254） | **裸对象**（无信封） | ❌ 最接近，但：①组路径是 `/v1/asset-groups` ≠ 上游 `/v1/assets/groups`；②`CreateAsset` 字段名 `url/asset_type` ≠ 上游 `source_url/type`；③`group_id` 上游要求 **整数**；④上游返回 `{success,message,data}` 信封 → 现有代码解析出空 id 直接报错（naxis.go:268-270,293-296）；⑤`UpdateAsset` 用 PATCH，上游只列了 PUT |

**判定：5 个适配器没有一个能直接复用。** 工厂对未知版本一律 fallback 到 gateway（`service/asset_adapter_factory.go:241-254`、`:397-409`），所以**即使手填一个不存在的新版本值也不会报错，只会静默打到咪咕路径然后 404**——排障时要盯这条日志。

### 3.3 新增 `GlobalAIAssetAdapter` 需要实现的方法清单

先在 `dto/channel_settings.go:70` 附近加常量（或在 `service/asset_adapter_factory.go` 加 `UpstreamVersionGlobalAI = "globaia"`），然后在工厂两处 switch 注册（`asset_adapter_factory.go:163-254` 与 `:316-410`）。

| 方法 | 上游调用 | 字段映射 | 备注 |
|---|---|---|---|
| `CreateAssetGroup` | `POST /v1/assets/groups` body `{"name":...}` | 响应 `data.id`(**数字**) → `ID`（`strconv`），`data.name`→`Name` | 文档 §6.1，HTTP 201 |
| `GetAssetGroup` | **文档未给 GET /{id}**，只有 `GET /v1/assets/groups`（列表） | 列表里按 id 过滤，或直接读本地表 | ⚠️ 接口缺口，需实测是否真的没有单查 |
| `UpdateAssetGroup` | **文档无此接口** | — | ⚠️ 只能返回「不支持」或本地 no-op，会改变现有下游行为（当前 gateway/naxis 都支持） |
| `DeleteAssetGroup` | `DELETE /v1/assets/groups/{id}?force=true` | 204 | 文档 §6.4（行 443） |
| `CreateAsset` | `POST /v1/assets` body `{"group_id":<int>,"type":"image|video|audio","source_url":"..."}` | 请求：`GroupID`→`strconv.Atoi` 后 `group_id`；`URL`→`source_url`；`AssetType`→小写 `type`；`Name` **上游无此字段**（丢弃，或用 PUT 重命名补齐）<br>响应：`data.id`→`ID`；`data.asset_id`/`data.asset_uri`→`URL`（生成引用用）；`data.group_id`→`GroupID` | 文档 §6.3（行 374-388）。⚠️ 只有 URL 导入，**没有 multipart 上传** |
| `GetAsset` | `GET /v1/assets/{id}` | `data.id/name/group_id/type/status/sync_status/upstream_status/asset_uri/sync_hint_code/sync_hint` → 统一响应 | 状态映射见下表 |
| `UpdateAsset` | `PUT /v1/assets/{id}` body `{"name":...}` | 200 | 文档 §6.4 用 **PUT**（行 427），不是 PATCH |
| `DeleteAsset` | `DELETE /v1/assets/{id}` | 204 | 文档行 437 |
| （可选）`CreateFaceVerification` | `POST /api/material?Action=CreateVisualValidateSession&Version=2024-01-01` body `{Name,Description,RedirectURL}` | `Result.BytedToken/H5Link/GroupId` → `FaceVerificationSession` | 字段与 `naxisActionSessionResult`（naxis_action.go:75-79）几乎一致，**是最容易复用的一块** |
| （可选）`GetFaceVerification` | `POST /api/material?Action=GetVisualValidateResult` body `{BytedToken}` | `Result.GroupId` 为空=等待；`Result.Error.Code` 判失败 | 文档 §9.2、§9.5（行 874-894） |
| （可选）真人组/真人素材 | `/v1/assets/person-groups*`、`/person-assets*` | 远超现有 `FaceVerificationAdapter` 能力，需要新接口 | ⚠️ 现存接口表达不了（无登记人物素材的方法） |

**状态映射（下游只认这三态，硬比较 `Status == "Active"`：`controller/seedance.go:432`、`service/seedance_asset_sync.go:67,76`）**

| 上游字段 | 下游 `Status` | 说明 |
|---|---|---|
| `status=ready` 且 `sync_status=active` 且 `upstream_status=Active` | **`Active`** | 唯一可被视频引用（配合 `asset_uri`） |
| `status=ready` 但 `sync_status` 缺失 / `pending` / `processing` | `Processing` | 「文件就绪但上游未同步完」，文档 §6.3（行 413）明确要求等两个字段 |
| `status=uploading|processing|queued` | `Processing` | |
| `sync_status=failed` / `upstream_status=failed` / `sync_hint_code != ""` | `Failed` | 失败原因写 `RawData` |
| 素材被删（404） | 保留 `Failed`，不抛 | 与 `isUpstreamNotFound`（seedance_asset_sync.go:289-300）口径一致 |
| `type`: `image/video/audio` | `Image/Video/Audio`（首字母大写，`model/seedance_asset.go:36` 注释） | |

**另一个硬风险（与适配器无关）**：上游普通素材 ID 是**数字**（lead 实测 `group_id=138`），而 `controller/seedance.go:585-591` `resolveAsset` 用「数字 → 本地自增 ID、`asset-`/`ast_` 前缀 → 上游 ID」分流，`controller/seedance.go:672-679` 对 group 只认 `group-` 前缀。数字上游 ID 落库后，下游用数字查会被当成本地表 ID → **会互相串号**。新适配器建议把上游数字 ID 包一层前缀（如 `ga-138`）写进 `UpstreamAssetID/UpstreamGroupID`，并同步扩展这两处分流判断。

---

## 四、请求体互操作

### 4.1 提交上游时**永远发 Ark `content[]`**，不发 `prompt`/`size`

`convertToRequestPayload`（adaptor.go:584-643）：

- `r.Model = 下游模型名`（:586），随后被 `info.UpstreamModelName` 覆盖（:446-450）
- `taskcommon.UnmarshalMetadata` 把 `metadata` 整体反序列化进 `requestPayload`（:591）
- 图片：`req.Images` → `{type:"image_url",image_url:{url}}`（:606-615）
- role 缺省补全：`reference_audio/image/video`（:618-628）
- 时长：`seconds`（字符串）优先，其次 `duration`（:630-634）
- 提示词：**删掉所有 `type=="text"` 的旧条目，追加一条 `{type:"text",text:prompt}` 到末尾**（:636-640）

`requestPayload` 字段（adaptor.go:44-63）与文档 §4（行 174-186）逐项对照：`model/content/duration/resolution/ratio/generate_audio/watermark/seed/camera_fixed/return_last_frame/callback_url` **全部覆盖**；额外字段 `service_tier/execution_expires_after/draft/tools` 都是 `omitempty`，客户端不传就不会发出去。

→ **结论：原生协议 A 的请求体与文档 §4 完全兼容，无需转换。**

### 4.2 `resolution` 怎么解析，`'720p'` 能不能过校验

- 校验入口：adaptor.go:229-235，解析不到直接 400 `invalid_resolution`（"resolution is required and must be one of: 480p, 720p, 1080p, 4k"）。
- `ResolveVideoResolution`（`relay/channel/task/doubao/resolution.go:35-52`）只认两处：
  1. `metadata.resolution`（含嵌套 `video_generation/parameters/params/input`，:55-71）
  2. `content[].resolution`（:74-84）
- `NormalizeResolution`（`relay/channel/task/doubao/constants.go:103-130`）接受 `720p/720P/720/HD` 等，**`'720p'` 完全合法**；`2k`、`1920x1080` 明确不接受（constants.go:82-96、:139-144 测试用例）。
- `size` **完全不参与**：`resolution.go:9-11` 注释写明「Doubao/Seedance 不存在 size 字段，不做 WxH 尺寸解析」，单测 `relay/channel/task/doubao/billing_resolution_test.go:166-170` 断言 `Size:"1920x1080"` 必须解析为 `""`。

> ⚠️ **纠正现有文档**：`docs/seedance-sdos2渠道配置.md:85`「下游必须传 size 否则倍率不生效」和 `:238`「计费倍率不生效：下游未传 size」**与当前代码不符**（那是旧实现的遗留描述）。正确说法是：**必须传 `resolution`**（Ark 原生放根级，OpenAI 风格放 `metadata.resolution`），传 `size` 没有任何作用。

### 4.3 一个会踩的入站坑：`prompt` + `content[]` 同时出现

`relay/common/relay_utils.go:216` 的 Ark 兼容分支条件是 **`prompt 为空 && content 非空`**。根级 `resolution/ratio/generate_audio/...` 的搬迁（:257-271）也在这个分支里。

→ 下游若同时传 `prompt` 和 `content[]`，根级 `resolution` **不会被搬进 metadata** → `ResolveVideoResolution` 返回空 → 直接 400。**对接文档要写清楚：Ark 原生写法不要带 `prompt`；或者把 `resolution` 放进 `metadata`。**

### 4.4 计费需要什么字段

1. **模型名必须命中 `videoPriceMap`**：`EstimateBilling` 用 `info.OriginModelName`（adaptor.go:309，映射**前**的下游名）查表（adaptor.go:321、:330），`GetVideoResolutionPrice` 严格查表、无回退（constants.go:149-155）。当前表里只有 4 个键：`doubao-seedance-2-5-260628` / `doubao-seedance-2-0-260128` / `doubao-seedance-2-0-fast-260128` / `doubao-seedance-2-0-mini-260615`（constants.go:49,59,67,73）。**下游模型名必须用这 4 个之一**，靠 `model_mapping` 映射到上游 `doubao-seedance-2.0`。
2. **基础费率必须配**：`ModelPriceHelperPerCall`（`relay/helper/price.go:167-230`）先取 `ModelPrice`，没有则取 `ModelRatio`；两者都没有时——自用模式下 `GetModelRatio` 兜底返回 `37.5`（`setting/ratio_setting/model_ratio.go:420`），非自用模式直接 400 `model_price_error`（price.go:179-195）。
3. **视频必须走「按次」而不是「按 token」**：上游是否返 `usage` 未验证；`EstimateBilling` 的倍率只乘在 `PriceData.Quota` 上（`relay/relay_task.go:191-204`），`Quota = ModelPrice × QuotaPerUnit(500000) × groupRatio`（price.go:202、`common/constants.go:62`）。所以为这个渠道配 `ModelPrice`（$/次，按「720p 不含视频」基准价），并**不要**把模型加进 `TASK_PRICE_PATCH` 环境变量（`common/init.go:159-170`），否则 `relay_task.go:198` 会跳过倍率折算。
4. 计费倍率公式：`倍率 = 该档目标价 / 720p 不含视频价`；带视频输入自动打折（adaptor.go:266-356 注释里给了 2.5 的完整算例）。`hasVideoInput` 靠扫 `metadata.content[].video_url` 或 `metadata.video_url`（adaptor.go:377-408）。

---

## 五、建议渠道配置 JSON

### 5.1 视频链路（今天就可用，零改代码）

渠道基础字段：

```
类型：54（DoubaoVideo）
名称：Seedance globalaiclient
Base URL：https://globalaiclient.com         ← 不要带 /v1，路径由 other 里的 path 补
密钥：<平台 Key>
模型（下游可见）：
  doubao-seedance-2-0-260128
  doubao-seedance-2-0-fast-260128
  doubao-seedance-2-0-mini-260615
  doubao-seedance-2-5-260628
```

`model_mapping`（下游名 → 上游文档名，文档 §1.1 行 31-36）：

```json
{
  "doubao-seedance-2-0-260128": "doubao-seedance-2.0",
  "doubao-seedance-2-0-fast-260128": "doubao-seedance-2.0-fast",
  "doubao-seedance-2-0-mini-260615": "doubao-seedance-2.0-mini",
  "doubao-seedance-2-5-260628": "doubao-seedance-2.5"
}
```

`other` 设置（**原生方案 A**）：

```json
{
  "asset_upstream_version": "gateway",
  "doubao_video_generate_path": "/api/v3/contents/generations/tasks",
  "doubao_video_fetch_path": "/api/v3/contents/generations/tasks",
  "allow_non_official_video_url": true,
  "sdos2_relay_base_url": "https://你的网关对外域名",
  "doubao_video_always_fetch_upstream": true,
  "openai_video_always_fetch_upstream": true
}
```

逐项理由：

| 字段 | 为什么 |
|---|---|
| `asset_upstream_version: "gateway"` | 让视频路径推断成原生 Ark（adaptor.go:197-209）；同时**素材工厂会选 GatewayAssetAdapter（注定失败）**，见下节说明 |
| `doubao_video_generate_path/fetch_path` | 显式写死，防止以后改 `asset_upstream_version` 时把视频路径带偏（adaptor.go:151-152 优先级最高）。**fetch 不带 `/{id}`**（adaptor.go:546） |
| `allow_non_official_video_url: true` | 上游成片域名大概率不是 `.volces.com`；不开会被判失败（adaptor.go:711-751）。开了以后下游拿到本服务代理地址（adaptor.go:949-957） |
| `sdos2_relay_base_url` | 代理地址的对外基址，不填回退 `ServerAddress`（asset_adapter_factory.go:31-41），填错下游拿到 localhost |
| `*_always_fetch_upstream: true` | 默认 OpenaI 格式**永不查上游**（relay_task.go:675-678）、Ark 格式仅 Data 不完整才查（relay_task.go:506-521）。联调期必须开 true，否则状态不刷新 |

**`ModelRatio` / `ModelPrice` 注意事项**

- 必须给上面 4 个下游模型名配 `ModelPrice`（按次，单位美元），值取「720p 不含视频」的对外定价。没配且非自用模式 → 400。
- 不要配 `ModelRatio`（视频没有 token 语义）；若要用表达式计费，模型须走 `tiered_expr`（price.go:171-173）。
- 不要把模型名加进 `TASK_PRICE_PATCH`。
- `doubao-seedance-2-0-260128` 的 4k 档在价格表里是 26（比 720p 的 46 还低，constants.go:59-64），若上游 2.0 也支持 4k，这个档位换算出的倍率 `26/46≈0.565`，定价前请复核。

### 5.2 素材链路（当前配置必然失败，属于预期）

- 只要 `asset_upstream_version` 是 `gateway/kwjm/sdos2/naxis/naxis_rest` 之一，`GetAssetAdapter` 就会选到对应旧适配器（asset_adapter_factory.go:163-254），打到 `/api/seedance/proxy/assets*`、`/seedance/media*`、`/v3/open/*`、`/v1/asset-groups*` 等上游不存在的路径 → 404 / SPA HTML 200。
- **好消息**：视频链路只在请求里出现 `asset://` 引用时才会碰素材适配器（`relay/channel/task/doubao/asset_refs.go:35-37`），纯文生视频、公网图片/视频 URL 参考都**不依赖**素材库。所以「先跑通视频」是完全可行的。
- **若一定要用素材库但暂时不改代码**：让下游跳过 `/api/seedance/*`，直接把公网 URL 塞进 Ark `content[].image_url.url`；`asset://` 引用一律不用。

---

## 六、待实测项（拿到 key 后按序验证）

| # | 验证项 | 方法 | 判定标准 |
|---|---|---|---|
| 1 | 原生创建任务 | `POST /api/v3/contents/generations/tasks`，body 用文档 §3.1 | 200 + `{id}`；记下 id 格式（`cgt-` 开头=真 Ark 代理，`task_` 开头=new-api 分支） |
| 2 | 原生查询终态 | 5s 轮询 `GET .../tasks/{id}` | 看 `status` 实际取值是否只有 `succeeded`；**专门测失败/超时任务是否出现 `expired`**（出现则必须改代码，见 §2.3） |
| 3 | 成片 URL 形态 | 同上响应的 `content.video_url` | 绝对 `.volces.com` → 可不代理；绝对非 volces / 相对路径 → 必须 `allow_non_official_video_url:true` |
| 4 | 兼容入口是否吃 Ark body | `POST /v1/videos`，body 用我们 adaptor 的真实产物（`content[]`、无 `prompt`） | 200 + `{id}` → 方案 B 也可用；400 `prompt is required` → 只能走方案 A |
| 5 | 兼容终态 | `GET /v1/videos/{id}` | `completed` 时 `metadata.url` 是绝对还是相对（文档说是相对，行 728） |
| 6 | `expired` /`unknown` 是否真的会出现 | 造超时/违规提示词 | 出现即确认 §2.3 的缺陷 |
| 7 | 取消语义 | `DELETE .../tasks/{id}` | 上游文档行 304 说可能 200 + `success=false`；本项目只看 HTTP 200（adaptor.go:853-857）→ 会「假取消成功」 |
| 8 | 素材组 | `POST /v1/assets/groups` → `GET /v1/assets/groups` → `GET /v1/assets/groups/{id}` | 重点确认**是否存在单查接口**（文档只列列表），决定 `GetAssetGroup` 能不能实现 |
| 9 | 素材组改名 | `PUT /v1/assets/groups/{id}` | 文档没写；不存在则 `UpdateAssetGroup` 只能降级 |
| 10 | URL 导入素材 | `POST /v1/assets` body `{group_id:138,type:"image",source_url:"..."}` | 201 + `data.id`(数字) + `status=ready`；观察 `sync_status` 多久变 `active` |
| 11 | 素材轮询 | `GET /v1/assets/{id}` | 确认 `status/sync_status/upstream_status/sync_hint_code` 的实际取值域，对齐 §3.3 映射表 |
| 12 | 素材改名/删除 | `PUT /v1/assets/{id}` / `DELETE /v1/assets/{id}` | 200 / 204 |
| 13 | 素材在生成里的引用 | 原生 body 里 `image_url.url = "123"`（数字字符串）与 `asset://xxx` 两种形态 | 文档 §6.5 说两种都行，实测哪种真的行 |
| 14 | 本地上传链路 | `POST /v1/assets/upload`（multipart，`group_id` 必须在 `file` 前）/ `upload-request`+`upload-commit` | 决定要不要在接口层新增上传能力（现在 `AssetAdapter` 没这方法） |
| 15 | 真人认证 | `POST /api/material?Action=CreateVisualValidateSession` | 返回 `Result.BytedToken/H5Link` → 新适配器实现 `FaceVerificationAdapter` 即可复用现有下游接口 |
| 16 | 上游是否真返 `usage` | 成功任务响应 | 有 token → 可选 `AdjustBillingOnComplete` 精细化；没有 → 保持按次 |
| 17 | `/v1/assets/video-model-params` | GET | 若给出与文档 §1.1 不同的档位（比如 2.0 不支持 4k），用于校正下游模型能力 |
| 18 | 幂等键 | 带 `Idempotency-Key` 重复提交 | 本项目 adaptor 目前**不发**该头（`BuildRequestHeader` adaptor.go:259-263）→ 重试会产生重复任务+重复计费，需评估 |

---

## 七、最小改动清单（若决定接素材库）

1. **新增** `service/asset_adapter_globaia.go`（约 350~450 行）：实现 §3.3 的 8 个方法 + 可选 `FaceVerificationAdapter`；`{success,message,data}` 信封解析；数字 id 加前缀（`ga-`/`gag-`）避免与本地自增 id 串号。
2. **改** `service/asset_adapter_factory.go`：加 `UpstreamVersionGlobalAI = "globaia"` 常量 + 两处 switch 分支（`:163-254` 与 `:316-410`），baseURL 复用 `seedance_asset_base_url`。
3. **改** `dto/channel_settings.go:61-70`：补一行注释/常量说明（可选）。
4. **改** `controller/seedance.go:585-591,672-679`：让 `resolveAsset`/`resolveAssetGroup` 识别新前缀，避免数字上游 id 与本地 id 冲突。
5. **改** `relay/channel/task/doubao/adaptor.go:760-817`：`ParseTaskResult` 增加 `"expired"`（及 `"unknown"` 超时判定）→ `TaskStatusFailure`，否则任务会挂到 `TASK_TIMEOUT_MINUTES`。
6. **改** `relay/channel/task/doubao/adaptor.go:853-857`：`CancelTask` 除 HTTP 200 外解析 `{success,data.status}`，避免「假取消成功」。
7. **（可选）** `relay/channel/task/doubao/adaptor.go:259-263`：`BuildRequestHeader` 透传/生成 `Idempotency-Key`，降低重复计费风险。

**注意**：第 1、2 项是新增渠道能力，不做任何提交/推送动作；本文只做分析。
