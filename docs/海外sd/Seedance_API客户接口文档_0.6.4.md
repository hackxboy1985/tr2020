# Seedance API 客户接口文档

版本：**0.6.4**  
更新日期：**2026-09-22**

本文包含视频生成、任务查询与下载、参考素材管理、分组与标签、模型适用性预检查、回调、错误处理和旧协议兼容说明。

当前规则：生成视频从完成起保存 **7 天**；参考素材读取签名从签发起有效 **7 天**；每用户素材容量 **512 MiB**，单文件上传 **64 MiB**。生成成功后应查询 `storage.status`，只有 `ready` 表示全部成片已保存。

## 阅读顺序

1. [视频生成与下载](#video-api)：新接入先完成创建、查询和下载。
2. [素材管理与扩展接口](#extended-api)：上传或导入参考素材，再用于生成。
3. [预检查、标签与错误分类](#supplement-api)：提交前检查素材适用条件。
4. [保存期限与下载状态](#retention-api)：确认七天期限、保存状态与到期处理。

配套文件：[Postman 请求集合](docs/Seedance.postman_collection.json)、[Postman 环境模板](docs/Seedance.postman_environment.json)。导入后填写自己的 `api_key`，按需手动执行；生成请求会计费，不要批量运行整个集合。


<a id="video-api"></a>

## 一、视频生成与下载

接口版本：0.6.4 · 文档核对：2026-09-22

本版以 OpenAI Video 兼容协议作为视频创建、查询和下载的主入口。素材、素材组、分页列表和终态隐藏属于平台扩展。协议兼容不代表支持所有模型参数；下列说明为本平台实现范围。各模型的实际生成能力以独立验收结果为准。本版增加生成视频完成后 7 天保存与下载、参考素材读取签名 7 天有效期；素材每用户容量仍为 512 MiB。具体保存状态和历史文件边界见 [保存与下载周期](docs/RETENTION.md)。

### 1. 地址、认证和模型

```bash
export API_BASE='http://106.54.45.168/seedance-gateway'
export API_KEY='<自己的 API Key>'
```

所有路径拼接到 API_BASE。所有请求均使用 `Authorization: Bearer <API Key>`。当前地址使用 HTTP，请在服务端保管密钥；HTTPS 接入地址以平台通知为准。不要将 Key 放入客户端、日志或 URL。

`GET /v1/models` 返回按当前 Key 模型限制过滤的目录；实际提交还需通过账户、权限和可用通道检查，列表不代表生成验收结果。模型名称和价格规则沿用既有配置：`seedance-2.0-mini`、`seedance-2.0`、`seedance-2.5`、`seedance-2.0-B`、`seedance-2.0-C` 均通过 `-720p` 或 `-1080p` 后缀选择分辨率。模型名为平台产品标识。素材接口需要 `media-service` 权限。

### 2. 创建视频

```bash
curl "$API_BASE/v1/videos" \
  -H "Authorization: Bearer $API_KEY" \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: video-project-001' \
  -d '{"model":"seedance-2.0-1080p","prompt":"蓝色与金色的抽象光影缓慢流动","seconds":5,"size":"1920x1080"}'
```

创建会计费；返回任务 ID 后轮询同一任务，不重复提交。相同用户、幂等键和请求返回首次提交响应；修改请求后复用该键会返回 409。提交超时且结果未知时，不自动重试付费创建，应先核对任务列表和消费记录。

| 字段 | 说明 |
|---|---|
| model | 必填，使用有权限的完整模型 ID |
| prompt | 必填且非空的文字描述；即使提供图片，仍须提供文字（也接受 input 别名） |
| seconds | 时长，默认 5；协议接受 1–30 的整数，模型自身限制可能更严格 |
| size | 如 `1920x1080`、`1080x1920`、`1280x720`，须符合模型分辨率后缀和支持的比例 |
| input_reference | 单张图片 HTTP(S) URL 或本人已就绪素材的 `asset://task_…` |
| images | 图片 URL 或素材 URI 数组，与 input_reference/image 合并去重后最多 9 张；多图能力受模型约束 |
| n | 平台扩展，1–4，默认 1；多输出能力需要对应模型支持 |
| resolution、aspect_ratio | 平台扩展；分辨率不能与模型后缀冲突；比例支持 `16:9`、`9:16`、`4:3`、`3:4`、`1:1` |

也接受 `duration`、`image`、`input` 别名。建议统一使用 `seconds`、`input_reference`、`prompt`。multipart 支持文字字段及 JSON 编码的 `images` 数组；二进制图片先通过素材上传接口导入。标准图片字段合并后，一张按图生视频、两张按首尾帧顺序、三张及以上按参考模式处理；需要明确指定角色时使用下述 `content` 扩展。

需要音视频参考或首尾帧时，可在 `/v1/videos` 使用平台扩展 `content` 请求结构；该结构使用 `duration、ratio`，不要与 `prompt、seconds、size` 混用。具体内容结构见 [扩展参数与旧协议说明](CLIENT_API.md)。`seed、generate_audio、watermark、draft、metadata` 当前不接受，传入会报错。

创建响应示意（任务可能已经进入后续状态）：

```json
{"id":"task_example","object":"video","model":"seedance-2.0-1080p","status":"queued","progress":0,"created_at":1789952400,"completed_at":null,"seconds":"5","size":"1920x1080","expires_at":null,"error":null,"resolution":"1080p"}
```

### 3. 查询、下载和删除

```bash
curl "$API_BASE/v1/videos/task_example" -H "Authorization: Bearer $API_KEY"
curl "$API_BASE/v1/videos/task_example/content" -H "Authorization: Bearer $API_KEY" --output video.mp4
```

成功响应示意：

```json
{"id":"task_example","object":"video","model":"seedance-2.0-1080p","status":"completed","progress":100,"created_at":1789952400,"completed_at":1789952460,"seconds":"5","size":"1920x1080","expires_at":1790557260,"storage":{"status":"ready","retention_seconds":604800,"expires_at":1790557260},"error":null,"resolution":"1080p","url":"/v1/videos/task_example/content","urls":["/v1/videos/task_example/content"],"outputs":[{"video_url":"/v1/videos/task_example/content"}],"content":{"video_url":"/v1/videos/task_example/content"}}
```

| 字段 | 语义 |
|---|---|
| status | `queued`、`in_progress`、`completed`、`failed`；轮询遇终态停止 |
| progress | 0–100 的任务阶段进度，不代表逐帧完成率，也不用于估算剩余时间 |
| model | 公共模型名；可访问的当前视频使用已登记的模型，不返回内部标识 |
| resolution、aspect_ratio | 平台扩展；分辨率按模型，比例有可信记录时返回 |
| created_at | Unix 秒时间戳；历史缺失或无效值为 null |
| completed_at | 已记录的终态时间；运行中或历史记录无终态时间时为 null，不使用更新时间代替 |
| seconds | 字符串形式的已接受生成时长；历史任务无法恢复时为 null，不是媒体文件测量时长 |
| size | 提交时指定的目标尺寸；未指定或历史记录缺失为 null；实际文件尺寸以媒体文件为准 |
| expires_at | 全部输出自动保存完成后，返回 completed_at + 604800（Unix 秒）。自动保存未完成时仍返回已知内容链接到期时间或 null，请结合 storage.status 判断 |
| storage | 平台扩展：status 为 not_ready/pending/ready/unavailable/expired；retention_seconds 为 604800，expires_at 为目标保存到期时间。ready 才表示全部输出已保存 |
| error | 非失败状态为 null；失败为平台错误码和说明，不返回内部诊断信息 |
| url、urls | 平台扩展，成功后的相对下载地址；均拼接 API_BASE，并携带认证 |

`GET /v1/videos/{id}/content` 下载第一条视频；同时支持 HEAD，Range 请求按内容服务实际响应返回。尚未成功返回 409 `video_not_ready`，不存在、不属于当前用户或已隐藏返回 404；当前 Key 缺少该模型权限返回 403。多输出通过 `urls` 获取其余结果。0.6.4 会自动保存成片，保存期为完成后 7 天；已保存成片不依赖原下载链接继续有效。到期后返回 410 content_expired，并周期清理保存的成片；任务记录仍可查询。历史已失效、未能保存的文件不能凭升级恢复。查询中的 storage.status 可区分正在保存、已保存和暂不可用，详见 [保存与下载周期](docs/RETENTION.md)。不要通过重新生成来重试下载。

平台扩展：`GET /v1/videos?page=1&page_size=20&status=completed`，返回 `object:list、data、total、page、page_size`，只含本用户且当前 Key 有权使用的模型任务；可按 `model`、`status`、逗号分隔的 `ids` 过滤。`page_size` 最大 100，默认 20；单次目录扫描最多 50,000 条，超限返回 422 `catalog_scan_limit`。

平台扩展：`DELETE /v1/videos/{id}` 只隐藏成功或失败终态任务，返回 `{"id":"task_example","deleted":true}`；后续查询及本 API 下载返回 404。运行中删除返回 409，不取消生成、不触发退款。

### 4. 素材与素材组的完整关联示例

素材组是同一用户下的管理目录；素材通过 `group_id` 关联目录，引用视频时使用素材的 `asset_uri`，不使用组 ID。

创建组：

```bash
curl "$API_BASE/seedance/media/groups" -H "Authorization: Bearer $API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"action":"create","name":"我的素材组","description":"项目人物参考"}'
```

```json
{"id":"group_example","name":"我的素材组","description":"项目人物参考","created_at":1789952400,"updated_at":1789952400}
```

导入素材并关联刚创建的组（替换示例 URL 为实际可访问地址）：

```bash
curl "$API_BASE/seedance/media" -H "Authorization: Bearer $API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"action":"import","group_id":"group_example","media_type":"image","url":"https://example.com/image.jpg","title":"参考图片"}'
```

```json
{"id":"task_asset_example","model":"media-service","status":"queued","asset_uri":"asset://task_asset_example","title":"参考图片","group_id":"group_example","tags":[],"media_type":"image","duration":null,"created_at":1789952400,"updated_at":1789952400}
```

查询直到 `status` 为 `succeeded`。素材状态使用平台素材协议：`queued、running、succeeded、failed`；历史未知状态可能为 `unknown`，与视频状态不同。只有 `succeeded` 才可引用。

```bash
curl "$API_BASE/seedance/media" -H "Authorization: Bearer $API_KEY" \
  -H 'Content-Type: application/json' -d '{"action":"get","id":"task_asset_example"}'
curl "$API_BASE/seedance/media?group_id=group_example&page=1&page_size=20" \
  -H "Authorization: Bearer $API_KEY"
```

单条查询与列表中的素材均返回 `id、asset_uri、group_id、title、tags、media_type、duration、status、created_at、updated_at`。`duration` 无测量记录时为 null。单条和批量读取会合并最新注册状态与目录信息；列表读取已保存的状态，可能稍有延迟。

在视频中使用该素材：

```bash
curl "$API_BASE/v1/videos" -H "Authorization: Bearer $API_KEY" \
  -H 'Content-Type: application/json' -H 'Idempotency-Key: video-project-asset-001' \
  -d '{"model":"seedance-2.0-1080p","prompt":"参考图片中的画面缓慢运动","seconds":5,"size":"1920x1080","input_reference":"asset://task_asset_example"}'
```

其他目录操作：向 `/seedance/media/groups` POST `action:list/get/update/delete`；读写单个组携带 `id`。向 `/seedance/media` POST `action:update,id,group_id` 可移动素材，`group_id:null` 可解除关联。非空组不能删除，返回 409 `group_not_empty`；先移动或删除组内素材。

本地文件上传：向 `/seedance/media/upload?group_id=group_example&title=参考图片` POST 文件原始字节，Content-Type 使用实际 MIME，例如 `image/png`；不要发送 multipart。上传后同样轮询素材就绪。单文件 64 MiB，每用户参考素材累计 512 MiB；服务读取签名链接有效 7 天，链接过期不自动删除原素材。

### 5. 同一素材能否用于不同模型

素材归属同一个平台用户。同一用户多个 Key 共享素材，但各 Key 仍须具备 `media-service` 和目标模型权限。不同用户不能共享素材 ID。

同一已就绪素材可以在有兼容引用能力、且可使用该素材所属服务通道的模型间复用，不按 720p/1080p 创建两份。无需为每次视频任务重复导入。平台会绑定素材所在通道；若该通道不支持目标模型、素材未就绪、类型不匹配或超出模型限制，请求会被拒绝。

当前没有承诺所有模型、所有通道无条件通用，也没有自动跨通道复制素材。切换 `model` 不改变素材所有权；是否可复用取决于权限、状态、类型、时长和目标模型能力。正式批量使用前应先验证目标组合。

提交前可调用不计费的预检查：

```bash
curl "$API_BASE/seedance/media/preflight" -H "Authorization: Bearer $API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"asset_id":"task_asset_example","model":"seedance-2.0-1080p","media_type":"video","role":"reference_video"}'
```

返回 `compatibility` 和逐项 `checks`，使用 supported/unsupported/unverified。unverified 表示证据不足，不是已经支持。素材标签、标签筛选和错误分类详见 [接口补充说明](docs/REPAIR_API.md)。

### 6. 错误与兼容边界

错误结构统一为 `{"error":{"code":"...","message":"..."}}`。401 表示认证无效；403 表示权限限制；404 表示资源不可见；409 表示状态或幂等冲突；410 表示内容已到期；413 表示请求过大；422 表示媒体校验失败；502/503 表示服务暂不可用。不要根据错误文本重新提交付费任务。

旧 `/seedance/api/v3/contents/generations/tasks` 路径继续保留，使用原来的 `running/succeeded` 状态和响应格式。新接入统一采用本指南的 `/v1/videos`。原始通用任务查询不作为客户入口；视频内容从本平台认证接口返回。

本版不提供真实生成 token、随机种子、运行中取消或所有参数透传。费用按平台既有价格配置和消费记录结算。公开响应只含平台 ID、模型名、目录信息与本平台下载地址。

### 7. 回调、Postman 与调用顺序

回调是平台扩展，需要预先配置接收域名。`GET /seedance/capabilities` 的 `tasks.callback` 只表示平台是否配置主机白名单；为 true 也需确认自己的 HTTPS 域名已获允许。回调 data 中的 storage 为通知生成时的保存状态，后续以任务查询为准。创建时成对传入 `callback_url` 与 `callback_secret`，具体签名、重试和通知结构见 [完成回调](docs/../CLIENT_API.md#完成回调)。本次修复后新建的 `/v1/videos` 任务通知使用 `version: "video.v1"`、`video.completed/video.failed` 及完整 Video 对象。历史任务和旧创建入口仍使用旧通知。未完成联调时使用轮询。

导入同目录的 [Postman 集合](docs/Seedance.postman_collection.json) 与 [环境模板](docs/Seedance.postman_environment.json)，填写自己的 `api_key` 和可配置的 `base_url`。先查询模型，再手动提交一次生成，复制任务 ID 后查询与下载；素材流程先创建组、导入或上传、等到就绪，再引用素材。每次业务请求保存独立幂等键，同一次请求重试保持原键与原内容。集合中的生成会计费、删除会改变可见性，不要批量运行整个集合。

完整流程为：认证 → 查询模型 → 创建素材组 → 导入/上传并关联 → 查询素材就绪 → 创建视频 → 轮询 completed/failed → 认证下载 → 按需隐藏终态任务。

标签、素材适用预检查及稳定错误分类，见 [接口补充说明](docs/REPAIR_API.md)。


<a id="extended-api"></a>

## 二、素材管理与扩展接口

接口版本：0.6.4 · 文档更新：2026-09-22

本文对应 0.6.4，保留素材、扩展参数及旧协议说明。视频主接口和保存周期见 [通用视频指南](docs/VIDEO_API.md) 与 [保存与下载周期](docs/RETENTION.md)。

新接入请优先阅读 [通用视频接口指南](docs/VIDEO_API.md)，其中包含本版完整创建/查询/下载响应、素材组关联示例和跨模型复用边界。本页保留旧协议参数说明，历史验收记录不代表本次新版本已经完成全部生产验收。

### 1. 接入地址与认证

```text
API_BASE = http://106.54.45.168/seedance-gateway
Authorization: Bearer <你自己账户创建的 API Key>
Content-Type: application/json
```

本文所有接口路径均拼接到 API_BASE。当前提供 HTTP 接入，连接未加密；HTTPS 地址就绪后由平台通知切换，请将基址作为配置项。API Key 仅保存在服务端环境变量中，不要嵌入网页、移动端包、代码仓库或日志。

在平台账户内创建自己的 API Key。需要生成权限及媒体库权限时，应允许目标视频模型和 `media-service`。不要使用其他用户的 Key。归属按用户隔离：同一用户的多个 Key 共享该用户素材和任务，不作为独立租户。

### 2. 历史实测范围与本版边界

2026-09-22 已完成两个独立用户的 3 秒视频参考 → Seedance 2.0 1080p / 10 秒生成 → 认证下载 → 完整播放 → 计费核对，并完成素材、任务和下载隔离检查。0.6.4 增加七天保存与签名周期；到期边界采用时钟推进测试，不能表述为已在生产等待满七天。

| 能力 | 本版状态 |
|---|---|
| Seedance 2.0 1080p，5 秒，16:9，单输出 | 2026-09-19 历史实测成功，覆盖文字生成及本人图片首帧引用 |
| PNG 上传、注册、查询、列表、搜索、改名、删除、分组 | 2026-09-19 历史实测通过 |
| 不同用户素材、分组、任务、下载权限隔离 | 2026-09-19 历史实测通过 |
| 任务查询、下载、终态隐藏删除；独立扣费及失败退款 | 2026-09-19 历史实测通过 |
| Seedance 2.0 1080p，10 秒，3 秒视频参考 | 2026-09-22 两个独立用户真实生成、下载、播放和扣费核对通过 |
| 其他模型、音频参考、多图、首尾帧组合、长视频、多输出、独立 URL 导入 | 已有接口实现，未完成完整生产组合验收 |
| 完成回调 | 已实现签名通知、失败重试和投递状态查询；按 2026-09-19 验收记录，接收域名尚未配置，公网投递未验证。启用前请使用轮询 |
| 运行中任务取消 | 不支持 |

视频生成可能因内容检测或处理错误失败；收到任务 ID 只代表提交成功。请以终态及账户实际消费记录为准。不同内容的耗时存在差异，当前没有承诺固定完成时间。

### 3. 模型目录

`GET /v1/models` 返回当前 Key 可见模型。以下为完整命名目录，并不代表所有模型均完成生产验收。

| 名称 | 720p 模型 ID | 1080p 模型 ID |
|---|---|---|
| Seedance 2.0 Mini | seedance-2.0-mini-720p | seedance-2.0-mini-1080p |
| Seedance 2.0 | seedance-2.0-720p | seedance-2.0-1080p |
| Seedance 2.5 | seedance-2.5-720p | seedance-2.5-1080p |
| Seedance 2.0-C | seedance-2.0-C-720p | seedance-2.0-C-1080p |
| Seedance 2.0-B | seedance-2.0-B-720p | seedance-2.0-B-1080p |

模型 ID 区分大小写，B/C 使用大写。媒体操作使用 `media-service` 权限。分辨率由模型后缀决定，不能通过参数改变。费用按照客户在平台配置的价格、分组倍率及消费记录结算；本文不作为固定报价单。余额及消费记录请登录平台个人账户查看，本文不提供余额查询 API。

### 接口速查

除表中说明外，路径均拼接到 `API_BASE`，使用 `Authorization: Bearer <API Key>`。

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/v1/models` | 查询当前 Key 可见模型 |
| GET | `/seedance/capabilities` | 查询模型、接口能力及限制 |
| POST | `/v1/videos` | 主协议：创建视频 |
| GET | `/v1/videos/{id}` | 主协议：查询视频 |
| GET / HEAD | `/v1/videos/{id}/content` | 主协议：认证下载第一个输出 |
| GET | `/v1/videos` | 平台扩展：分页查询本人视频 |
| DELETE | `/v1/videos/{id}` | 平台扩展：隐藏终态视频 |
| POST | `/seedance/media` | 导入、读取、批量读取、搜索、修改及删除素材 |
| GET | `/seedance/media` | 分页查询本人素材 |
| POST | `/seedance/media/upload` | 上传原始文件字节 |
| POST | `/seedance/media/preflight` | 检查素材与目标模型的已知适用条件，不提交生成 |
| POST | `/seedance/media/groups` | 创建、查询、列表、修改及删除分组 |
| GET | `/seedance/callbacks/{task_id}` | 查询已登记的回调投递状态 |
| POST / GET | `/seedance/api/v3/contents/generations/tasks` | 兼容旧协议：创建 / 分页查询 |
| GET / DELETE | `/seedance/api/v3/contents/generations/tasks/{id}` | 兼容旧协议：查询 / 隐藏 |
| GET / HEAD | `/v1/tasks/{id}/artifacts/{artifact}/content` | 旧下载及多输出；artifact 为 video 或 video_2 至 video_4 |

视频任务另支持 `/api/v3/contents/generations/tasks` 路径别名。旧接入保持原有格式；新接入统一使用 `/v1/videos`，详见新版通用视频指南。

### 4. 五分钟接入：创建、轮询、下载

以下为 macOS/Linux Bash 示例，依赖 curl 和 jq。创建任务会按账户配置扣费。

```bash
export API_BASE='http://106.54.45.168/seedance-gateway'
export API_KEY='<替换成自己的 Key>'

## 先确认模型权限
curl --fail-with-body "$API_BASE/v1/models" \
  -H "Authorization: Bearer $API_KEY"

## 为一次业务提交保存一个固定的幂等键；重试时不要换键或修改 JSON
export REQUEST_ID="video-$(date +%s)-$RANDOM"
curl --fail-with-body "$API_BASE/v1/videos" \
  -H "Authorization: Bearer $API_KEY" \
  -H 'Content-Type: application/json' \
  -H "Idempotency-Key: $REQUEST_ID" \
  --data '{"model":"seedance-2.0-1080p","prompt":"蓝色与金色的抽象光影缓慢流动","seconds":5,"size":"1920x1080"}' \
  -o created.json
export TASK_ID="$(jq -er '.id' created.json)"

## 每隔 10 秒查询一次，直到 completed 或 failed；不要重新 POST 生成请求
curl --fail-with-body "$API_BASE/v1/videos/$TASK_ID" \
  -H "Authorization: Bearer $API_KEY" -o task.json
cat task.json

## 仅 completed 后执行；输出路径需直接拼接 API_BASE
export VIDEO_PATH="$(jq -er '.url' task.json)"
curl --fail-with-body "${API_BASE}${VIDEO_PATH}" \
  -H "Authorization: Bearer $API_KEY" -o output.mp4
```

**下载拼接示例：** 返回 `/v1/videos/task_example/content` 时，完整地址为 `http://106.54.45.168/seedance-gateway/v1/videos/task_example/content`。不要用会丢弃基址路径前缀的 URL 解析方式。下载需要相同用户的有效 Key，不是可匿名分享的播放链接。成片自动保存完成后（storage.status=ready）可在 completed_at 后 7 天内下载；超过 expires_at 返回 410。历史已过期且未保存文件无法自动恢复；请及时下载自存。

### 5. 上传图片并用于生成

```bash
## 原始二进制上传，不使用 multipart/form-data
curl --fail-with-body "$API_BASE/seedance/media/upload?title=reference" \
  -H "Authorization: Bearer $API_KEY" -H 'Content-Type: image/png' \
  --data-binary @reference.png -o uploaded.json
export ASSET_ID="$(jq -er '.id' uploaded.json)"

## 重复查询，直到 status 为 succeeded；不要把上传响应当成注册完成
curl --fail-with-body "$API_BASE/seedance/media" \
  -H "Authorization: Bearer $API_KEY" -H 'Content-Type: application/json' \
  --data "$(jq -n --arg id "$ASSET_ID" '{action:"get",id:$id}')"

## 注册成功后引用。此请求会创建新的付费生成任务
jq -n --arg asset "asset://$ASSET_ID" \
  '{model:"seedance-2.0-1080p",duration:5,ratio:"16:9",content:[{type:"text",text:"让画面中的光影缓慢流动"},{type:"image_url",image_url:{url:$asset},role:"first_frame"}]}' > generate.json
curl --fail-with-body "$API_BASE/v1/videos" \
  -H "Authorization: Bearer $API_KEY" -H 'Content-Type: application/json' \
  -H "Idempotency-Key: image-$(date +%s)-$RANDOM" --data-binary @generate.json
```

以下为详细接口契约。示例中的任务 ID、素材 ID 和分组 ID 均须替换为自己账户实际返回的值。

---

### 旧协议创建与 content 扩展

`POST /seedance/api/v3/contents/generations/tasks`

```json
{
  "model": "seedance-2.0-1080p",
  "content": [{"type": "text", "text": "一只柯基在草地上奔跑"}],
  "duration": 5,
  "ratio": "16:9"
}
```

提交后返回 `id`、公共 `model` 和 `status`。保存任务 ID，用同一用户令牌查询。

| 字段 | 说明 |
|---|---|
| `model` | 十个视频模型之一；`media-service` 不能生成视频 |
| `content` | 非空数组，包含文字和可选图片/视频/音频参考；文字提示不能为空 |
| `duration` | 1–30 秒整数，默认 5；具体模型的真实支持范围仍受服务能力约束 |
| `resolution` | 可省略，自动按模型后缀确定；如填写必须与模型后缀一致，否则拒绝 |
| `ratio` | `16:9`、`9:16`、`4:3`、`3:4`、`1:1` |
| `n` | 可选网关扩展，生成数量 1–4，默认 1 |

内容支持以下形式：

```json
[
  {"type":"text","text":"让参考人物走向镜头"},
  {"type":"image_url","image_url":{"url":"https://example.com/image.png"},"role":"reference_image"},
  {"type":"video_url","video_url":{"url":"https://example.com/video.mp4"},"role":"reference_video","duration":4}
]
```

音频使用 `audio_url: {"url":"..."}` 和 `role: "reference_audio"`。直接 URL 音视频参考，以及引用经 URL 导入的音视频素材时，须在生成请求的对应 content 项填写真实 `duration`（秒），用于计费估算；导入请求本身不接受 duration 字段。通过本地文件上传接口注册的音视频引用时可省略，由服务采用实测时长。最多接受 9 个参考素材。生成请求中的图片可传不超过 2 MiB 的 Base64 Data URL（image/png、image/jpeg、image/webp），由接口校验并转换。二进制文件使用下文的独立上传接口。

首帧图片使用 `role: "first_frame"`；可另加一张 `last_frame`，无需按首尾顺序排列。首尾帧不能与其他参考模式混用。省略 role 的图片按 `reference_image` 处理。

已完成注册的自有素材可使用 `image_url.url: "asset://task_..."`，视频/音频同理。素材 ID 必须属于同一用户并已完成注册。

本节为保留的旧协议和 `content` 扩展字段。新接入的视频主协议以通用视频指南为准。`seed`、`generate_audio`、`watermark`、`draft` 仍未实现，传入会明确拒绝。接口提供任务列表、终态任务删除和完成回调（需另行开通）；不提供取消服务端执行。媒资接口、`n`、参考时长和 `callback_secret` 是扩展字段。

### 旧协议查询与下载

`GET /seedance/api/v3/contents/generations/tasks/{id}`

状态为 `queued`、`running`、`succeeded` 或 `failed`。成功示例：

```json
{
  "id": "task_example",
  "model": "seedance-2.0-1080p",
  "status": "succeeded",
  "content": {"video_url":"/v1/tasks/task_example/artifacts/video/content"},
  "outputs": [{"video_url":"/v1/tasks/task_example/artifacts/video/content"}]
}
```

`content.video_url` 是第一个输出；多个输出见 `outputs`。地址为相对于 API_BASE 的路径（必须保留 /seedance-gateway 前缀），下载必须携带相同的 Bearer 令牌。`model` 在任务模型或分辨率元数据缺失时可能省略。

本节下载规则适用于 API 任务响应。平台网页的任务预览可能返回完整的本站地址及临时访问凭据，与 API 的 Bearer 下载方式不同；不要把网页预览链接当作长期可分享地址，也不要将完整地址再次拼接到 `API_BASE`。

失败示例：

```json
{"id":"task_example","status":"failed","error":{"code":"service_error","message":"Generation failed."}}
```

### 媒资服务

`POST /seedance/media`，按 `action` 选择操作。所有字段均为小写。

| 操作 | 请求示例 |
|---|---|
| 导入 | `{"action":"import","url":"https://example.com/a.png","media_type":"image","title":"参考图"}` |
| 读取/等待注册 | `{"action":"get","id":"task_asset"}` |
| 批量读取 | `{"action":"batch_get","ids":["task_a","task_b"]}` |
| 改标题 | `{"action":"update","id":"task_asset","title":"新标题"}` |
| 删除 | `{"action":"delete","ids":["task_asset"]}` |
| 范围搜索 | `{"action":"search","ids":["task_a","task_b"],"keyword":"参考","page":1,"page_size":20}` |

导入返回网关 `id` 和 `model: "media-service"`；持续 `get`，只有 `status: "succeeded"` 才可引用。批量响应使用 `data`、`total` 和可选 `missing_ids`。提供 IDs 的搜索仍限定在这些自有素材中；本版允许省略 IDs，搜索当前用户的素材目录。素材分组按用户隔离。客户只能查看平台为本人配置的余额及本人消费记录。具体扣费以客户账户费率与消费记录为准。

### 通用视频主入口

`POST /v1/videos`、`GET /v1/videos/{id}` 和 `GET/HEAD /v1/videos/{id}/content` 是主接入方式；另有列表和终态隐藏扩展，详见 [通用视频指南](docs/VIDEO_API.md)。模型使用同样的十个 Seedance 名称，后缀决定分辨率。`resolution` 和 `size` 若填写必须与模型匹配，例如 1080p 模型可用 `1920x1080` 或 `1080x1920`；不能通过参数切换到 720p。可用 JSON `prompt`、`seconds`、`size`、`input_reference`，或对应 multipart 文本字段；不支持文件上传。JSON 也可直接使用上面的 `content` 结构，两种结构不要混用。视频状态值遵循主协议：`queued`、`in_progress`、`completed`、`failed`。


### 媒体库与任务管理

所有新增接口仍使用同一 Bearer API Key。归属按平台用户判断，同用户不同 Key 共享素材目录，但分别执行 Key 的模型权限。API Key、过期时间、停用状态、用户状态和 IP 限制实时校验；不能通过传 user_id 指定归属。

- `GET /seedance/capabilities`：返回当前 Key 的模型目录、接口能力和限制。`models[].production_verified` 当前固定为 false，不代表模型不可用，也不是逐模型验收结果；真实验收范围见本文开头。`tasks.callback` 表示平台是否配置了回调主机白名单：false 表示暂未启用，true 仍需确认客户接收域名已被允许且已完成联调。
- `GET /seedance/media?page=1&page_size=20&keyword=角色` 或 `POST /seedance/media {"action":"list"}`：无需提供素材 ID；支持 `group_id`、`keyword`、`media_type`、`status`、`page`、`page_size`、`tag`。每页最大 100。历史导入记录也可列出，历史记录缺少类型时 `media_type` 为 null；列表状态为已保存的任务状态，需实时刷新素材状态时调用 get。
- `POST /seedance/media/groups`：`action` 为 create/get/list/update/delete；create 使用 name/description，get/delete 使用 id，update 使用 id/name/description，list 可按 keyword/page/page_size 筛选。返回 `group_...` 格式的分组 ID。非空组删除返回 409，先移动或删除素材。
- 导入可附加 `group_id`；修改素材可附加 `group_id`，传 null 移出分组。跨用户分组、素材和混合批次均拒绝。

#### 本地文件上传

`POST /seedance/media/upload?title=参考图&group_id=group_...`

请求体是文件原始字节，Content-Type 为 `image/png`、`image/jpeg`、`image/webp`、`video/mp4`、`audio/mpeg`、`audio/wav` 或 `audio/mp4`，不是 multipart。单文件上限 64 MiB，单用户本地存储上限 512 MiB；服务最多同时处理两个二进制上传。服务会校验实际媒体类型并测量音视频时长，不接受任意文件改扩展名上传。

返回 `id`、`status`、`media_type` 和音视频的 `duration`。随后按原 get 接口等待注册成功，再用 `asset://task_...` 引用。上传产生的读取签名链接有效期 7 天（604800 秒），仅用于服务端抓取；持有该链接可在有效期内读取对应文件，调用方不应分享链接。签名过期不会自动删除已上传素材，也不等同于素材 ID 失效；素材能否继续用于生成应查询其状态。视频生成下载仍要求 Bearer。

对本地上传的音视频，引用时自动采用实测时长；若客户填写的 duration 与实测差异大于或等于 0.1 秒，拒绝请求。远程 URL 导入、直接 URL 参考仍依赖声明时长，尚未完成可信远程媒体计量。

#### 旧协议任务列表与删除

`GET /seedance/api/v3/contents/generations/tasks?page=1&page_size=20&status=succeeded&model=seedance-2.0-C-720p`

返回 data/total/page/page_size。可按 ids（逗号分隔）筛选。也提供 `/api/v3/contents/generations/tasks` 路径别名，支持 filter.status、filter.model、filter.task_ids。

`DELETE /seedance/api/v3/contents/generations/tasks/{id}`：仅终态任务可删除，成功响应为 `{"id":"task_example","deleted":true}`，其中 id 为实际任务 ID；从客户目录、查询和下载入口隐藏，平台消费记录保留。重复删除幂等；非终态返回 409 `task_cannot_be_cancelled`。这不是服务端取消接口，不触发退款或停止服务端任务。

#### 提交防重

创建视频时可带 `Idempotency-Key` 请求头（8–128 位字母、数字、点、下划线、冒号、短横线）。同用户、相同 Key 和相同 JSON 内容返回原结果；内容不同返回 409。创建请求超时或响应无法确认时，同 Key 返回 `submission_outcome_unknown`，不得换 Key 盲目重提，应先查任务列表并联系管理员对账。记录跨进程重启保留。

#### 完成回调

回调功能已实现，使用前需要平台配置接收域名并完成联调。2026-09-19 历史记录中尚未开放公网回调；当前是否启用需查询能力目录并与平台确认；这不表示接口缺少回调能力，也不表示只提交回调字段即可立即使用。

启用流程：

1. 客户准备可接收 POST 请求的公网 HTTPS 地址，并提供接收域名供平台配置白名单。
2. 平台完成配置后，客户创建新任务时同时传入 `callback_url` 和 `callback_secret`。
3. 客户按下述规则验签、检查时间戳、持久化去重并返回 2xx。
4. 双方验证成功/失败通知及重试行为后启用；保留任务查询作为补偿方式。

创建视频可附加 `callback_url` 和 32–256 字符的 `callback_secret`，两者必须同时提供。URL 必须为管理员预先允许的 HTTPS 主机，不允许内网地址、重定向、用户信息或非 443 端口。回调域名尚未配置时提交回调会被拒绝，不会先发生成任务。

向 `POST /v1/videos` 提交的回调请求示例（需先完成域名配置；以下域名和密钥均为示例，不可直接用于生产）：

```json
{
  "model": "seedance-2.0-1080p",
  "content": [{"type": "text", "text": "蓝色与金色的抽象光影缓慢流动"}],
  "duration": 5,
  "ratio": "16:9",
  "callback_url": "https://callbacks.example.com/seedance/events",
  "callback_secret": "REPLACE_WITH_YOUR_32_TO_256_CHARACTER_SECRET"
}
```

终态通知包含稳定事件 id、type 和 data。本次本地修复后，新建 `/v1/videos` 任务使用 `version: "video.v1"`、`video.completed/video.failed`，data 与查询 Video 对象一致。旧入口及历史未记录协议版本的任务保留 `video.succeeded/video.failed` 和旧 data，已生成通知的重试不改变报文。生产是否生效以发布记录为准。请求头：

- `X-Seedance-Event`：事件 ID，用于消费者去重。
- `X-Seedance-Timestamp`：秒级时间戳；接收方应校验新鲜度。
- `X-Seedance-Signature`：`sha256=` 加 HMAC-SHA256(callback_secret, timestamp + "." + 原始请求体) 的十六进制摘要。

验签必须使用接收到的原始请求体字节，不要先解析 JSON 再序列化。签名应使用恒定时间比较；事件 ID 用于去重，时间戳用于拒绝过期通知。回调通知成功或失败状态，实际扣费及退款到账仍应核对账户记录。

已持久化登记的通知采用至少一次投递方式，接收方应持久化去重后再返回 2xx。任务提交与回调登记之间存在跨服务故障窗口，不保证端到端零丢失；接收方应保留任务查询用于对账补偿。非 2xx 或网络错误最多尝试 8 次，指数退避；回调密钥加密保存，任务成功不会因为回调失败而改变。`GET /seedance/callbacks/{task_id}` 查询 attempts/delivered/next_at/last_status，不返回 URL 和密钥。

#### 当前边界

- 不支持运行中或已提交排队任务的服务端取消。
- 不提供真人认证 H5、视频后处理或官方 Seedance 全量参数兼容。
- 媒体库列表单次扫描最多 50,000 条当前用户导入/生成记录，超过会明确报 catalog_scan_limit，不静默截断。
- 终态删除为本地可见性控制；服务端素材删除仍使用原媒体 delete 操作。


### 错误处理与上线自检

| HTTP / 情况 | 客户处理方式 |
|---|---|
| 400 | 检查字段、JSON、模型与分辨率是否一致，以及是否传入不支持的参数 |
| 401 / 403 | 检查 Key、账户状态、模型授权、有效期和 IP 限制 |
| 404 | 资源不存在、已隐藏或不属于当前用户；不要尝试枚举其他用户 ID |
| 409 | 检查非空分组、非终态删除或幂等请求冲突；按 error.code 处理 |
| 413 / 415 / 422 | 检查文件大小、Content-Type 和真实媒体内容 |
| 429 | 降低请求频率并退避；不要通过重新登录反复查询余额 |
| 5xx / 超时 | 查询类请求可退避重试；生成提交结果不明时先查任务，不要更换幂等键盲目重提 |
| HTTP 成功但任务 failed | 异步生成失败，保存任务 ID 并查询消费/退款记录，必要时联系平台支持 |

建议每 10 秒轮询，遇到限流逐步延长至 30 秒；这是客户端建议，不是服务端速率承诺。轮询超时不代表任务已取消，请保留任务 ID 后续查询。请求结果不明错误 `submission_outcome_unknown` 需核对任务列表或联系平台。

上线前确认：Key 存放在服务端；基址可配置；先校验上传注册成功再引用；状态值按所用协议处理；下载携带认证且保留基址前缀；实现失败和限流处理；同业务请求保存幂等键；完成自己业务素材的效果、费用和并发验证。按 2026-09-19 验收记录，平台接入地址尚未配置 HTTPS，回调接收域名也尚未配置，两者应分别处理。回调接收地址必须使用 HTTPS；平台接入地址使用 HTTP 并非发送 HTTPS 回调的直接阻碍。回调配置和公网联调完成前，请使用轮询获取结果。

`docs/` 目录另有 `Seedance.postman_collection.json` 和 `Seedance.postman_environment.json` 可用于辅助接入；本 Markdown 文件可独立阅读。导入集合后填写 api_key；task_id、asset_id、group_id 按响应填写。生成请求会计费，请勿直接批量运行整个集合。

本版已包含：素材 `tags`、列表 `tag` 筛选、`POST /seedance/media/preflight`、公共错误分类。详见 [接口补充说明](docs/REPAIR_API.md)。


<a id="supplement-api"></a>

## 三、预检查、标签与错误分类

此文描述 0.6.4 接口契约。原 NewAPI 视频协议、模型名称、价格及用户所有权规则保持不变。上线状态以发布验收记录为准。

### 素材标签

`POST /seedance/media` 的 import、update 接受 `tags`。最多 20 项，每项 1–64 字符，去首尾空格、去重。空数组清除标签；省略保留原值。标签仅存于本平台，不转发给生成服务。

```json
{"action":"import","url":"https://example.com/reference.jpg","media_type":"image","title":"产品图","group_id":"group_example","tags":["新品","秋季"]}
```

```json
{"action":"update","id":"task_example","tags":["新品"]}
```

导入、上传、单条、批量、列表均返回 `tags` 数组。列表支持 `GET /seedance/media?tag=新品` 或 POST `{"action":"list","tag":"新品"}`，匹配完整标签，可与组、类型、状态筛选组合。上传后可用 update 添加标签。历史无标签记录为 `[]`。

### 素材与目标模型预检查

使用正常 Bearer Key 调用 `POST /seedance/media/preflight`，不会创建任务、扣费或迁移素材。

```json
{"asset_id":"task_example","model":"seedance-2.0-720p","media_type":"image","role":"reference_image"}
```

`media_type`、`role` 可省略。外部用户、隐藏/删除素材返回 404；Key 无目标模型或素材权限返回 403。

返回 `asset_id/model/compatibility/checks/production_verified`。各检查项使用 `supported/unsupported/unverified` 三态；明确不满足任何已知条件时整体为 unsupported。所有本地条件通过仍不等于模型实际接受，真实参考能力未验证时整体为 unverified。不会公开内部通道、模型或凭据。

检查项包括权限、素材就绪、类型、参考角色、当前通道的目标模型启用情况及真实模型能力。单素材检查不证明多素材组合、视频/音频时长或全部角色组合可用。检查与实际提交之间配置可能变化；正式提交会重新校验权限、就绪、已知类型和通道条件，多通道引用被拒绝。真实生成仍以宿主及底层能力校验为准。

能力目录的 `models[].reference_support` 标为 unverified，提供 `preflight` 路径；`media.tags/preflight` 标识接口已实现，不是模型能力验收证明。

### 视频字段与回调

已接受的时长、尺寸、比例和创建协议写入持久存储。明确尺寸可恢复比例；明确分辨率及比例可计算请求尺寸。历史任务从已存参数恢复，未知值保持 null。这些字段不是视频文件实测结果。

0.6.4 在全部输出自动保存完成后返回七天保存期限：`expires_at = completed_at + 604800`，`storage.status=ready`。正在保存或保存暂不可用时，顶层 expires_at 仍为已知内容链接到期时间或 null；详见 [保存与下载周期](docs/RETENTION.md)。

本修复之后新建 `/v1/videos` 的回调示意：

```json
{"id":"evt_task_example","type":"video.completed","version":"video.v1","data":{"id":"task_example","object":"video","status":"completed"}}
```

示意省略了其他 Video 字段；实际 data 使用与 GET /v1/videos/{id} 相同投影。旧路径及历史任务保留 `video.succeeded/video.failed` 和旧 data。已持久化事件重试保持相同正文，原签名及去重机制不变。

### 稳定错误分类

HTTP 错误为 `error.code/message/type/param`，固定公开提示，不透传底层错误、地址或标识。

| code | 含义与处理 |
|---|---|
| asset_not_ready | 等待素材处理成功 |
| media_type_mismatch | 更正引用类型 |
| model_incompatible | 当前素材通道不满足目标模型条件 |
| asset_channel_conflict | 当前素材不能混在同一次请求使用 |
| invalid_tags | 修正标签数量及长度 |
| content_expired | 保存期已结束，或存在可靠内容到期证据；使用已下载文件或联系平台，不自动重提付费生成 |
| content_unavailable | 文件不可用；不推断一定过期 |
| invalid_range | 修正字节范围，保留 HTTP 416 |
| download_service_unavailable | 下载服务异常；无法凭该错误确认文件已过期 |
| service_unavailable / rate_limited | 稍后重试；生成提交结果不明时勿盲目重复提交 |

错误分类不会恢复已失效文件。超过七天的长期保存、跨通道自动注册、客户 Action/PascalCase 格式兼容、真实全模型验证不在本批已实现能力中。

### 0.6.3 下载补丁

真实视频参考验收发现：部分签名媒体允许 GET，却拒绝 HEAD。内容接口现以已授权 GET 获取真实响应头，对 HEAD 客户端立即停止接收文件体，保留实际 Content-Length/Range 等安全字段。无需重新生成或额外扣费，账号隔离与鉴权规则不变。


<a id="retention-api"></a>

## 四、保存期限与下载状态

### 生成视频：完成后保存 7 天

生成完成后，平台自动保存成片。保存期从 `completed_at` 起计算 **604800 秒（7 天）**，不会因查询、下载或重启重新计时。多输出任务会逐个保存。

查询 `GET /v1/videos/{id}`，结合以下字段判断：

| 字段 / 状态 | 含义与处理 |
|---|---|
| `status=completed` | 视频生成完成；保存可能还在进行 |
| `storage.status=not_ready` | 尚未生成成功 |
| `storage.status=pending` | 正在等待或进行自动保存；可稍后查询 |
| `storage.status=ready` | 全部输出已保存；可在七天期限内通过认证接口下载 |
| `storage.status=unavailable` | 自动保存暂不可用，平台会重试；原内容尚有效时仍可下载。若持续出现，请保留任务 ID 联系平台，不重复付费生成 |
| `storage.status=expired` | 七天保存期已结束 |
| `storage.retention_seconds` | 固定为 `604800` |
| `storage.expires_at` | 以完成时间计算的目标保存到期时间；pending/unavailable 时不表示文件已成功保存 |
| `expires_at` | ready 时为七天到期时间；尚未保存完成时为已知原内容到期时间或 null；到期后仍保留七天截止时间 |

示例：

```json
{
  "id": "task_example",
  "status": "completed",
  "completed_at": 1789952460,
  "expires_at": 1790557260,
  "storage": {
    "status": "ready",
    "retention_seconds": 604800,
    "expires_at": 1790557260
  },
  "url": "/v1/videos/task_example/content"
}
```

下载地址拼接 `API_BASE`，使用所属用户的有效 Bearer Key。支持 GET、HEAD 和单段 Range；不能把该地址当作免认证的分享链接。查询和下载仍执行用户、模型权限及任务隐藏检查。

```bash
curl "$API_BASE/v1/videos/$TASK_ID" -H "Authorization: Bearer $API_KEY"
curl -I "$API_BASE/v1/videos/$TASK_ID/content" -H "Authorization: Bearer $API_KEY"
curl "$API_BASE/v1/videos/$TASK_ID/content" \
  -H "Authorization: Bearer $API_KEY" --output video.mp4
```

七天到期后下载返回 **HTTP 410，`content_expired`**，平台周期清理保存的成片。任务记录仍可查询，`status` 仍表示原生成结果，不改为 failed。需要长期保存的客户应在到期前下载至自己的存储。

已保存的成片不再依赖原内容链接有效期。升级前已经失效、且没有成功保存的文件不能自动恢复；历史任务是否可用以实际 `storage` 状态和下载结果为准，不因升级统一延长七天。删除任务仍是终态隐藏，不取消生成、不触发退款，也不重新计算保存时间。

回调通知中的 `storage` 是通知生成时的状态；如为 pending，请继续查询任务。回调重试正文保持不变。

### 参考素材：容量不变，读取签名有效 7 天

- 单文件上传上限：**64 MiB**。
- 每用户参考素材累计上限：**512 MiB（536870912 字节）**；同一用户多个 Key 共用该容量。
- 上传文件供服务端读取的签名链接：从签发起有效 **7 天（604800 秒）**。旧的 24 小时链接保持原到期时间，不会自动续期。
- 签名链接到期不会自动删除已上传的素材文件，也不等于 `asset://task_…` 素材 ID 到期。是否可继续用于生成，请查询素材状态并做目标模型预检查。
- 持有签名链接者在有效期内可读取对应文件；不要分享或记录到公开日志。客户通常使用素材 ID，无需管理内部读取链接。
- 生成视频的七天保存空间与上述参考素材容量分别计算。

当前限制可以通过 `GET /seedance/capabilities` 查看：

```json
{
  "tasks": {"video_retention_seconds":604800},
  "limits": {
    "max_upload_bytes":67108864,
    "user_storage_bytes":536870912,
    "upload_link_ttl_seconds":604800,
    "base64_image_bytes":2097152
  }
}
```

示例仅列出相关字段；实际响应还包括模型及其他能力。
