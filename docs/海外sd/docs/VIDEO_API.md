# 通用视频接口接入指南

接口版本：0.6.4 · 文档核对：2026-09-22

本版以 OpenAI Video 兼容协议作为视频创建、查询和下载的主入口。素材、素材组、分页列表和终态隐藏属于平台扩展。协议兼容不代表支持所有模型参数；下列说明为本平台实现范围。各模型的实际生成能力以独立验收结果为准。本版增加生成视频完成后 7 天保存与下载、参考素材读取签名 7 天有效期；素材每用户容量仍为 512 MiB。具体保存状态和历史文件边界见 [保存与下载周期](RETENTION.md)。

## 1. 地址、认证和模型

```bash
export API_BASE='http://106.54.45.168/seedance-gateway'
export API_KEY='<自己的 API Key>'
```

所有路径拼接到 API_BASE。所有请求均使用 `Authorization: Bearer <API Key>`。当前地址使用 HTTP，请在服务端保管密钥；HTTPS 接入地址以平台通知为准。不要将 Key 放入客户端、日志或 URL。

`GET /v1/models` 返回按当前 Key 模型限制过滤的目录；实际提交还需通过账户、权限和可用通道检查，列表不代表生成验收结果。模型名称和价格规则沿用既有配置：`seedance-2.0-mini`、`seedance-2.0`、`seedance-2.5`、`seedance-2.0-B`、`seedance-2.0-C` 均通过 `-720p` 或 `-1080p` 后缀选择分辨率。模型名为平台产品标识。素材接口需要 `media-service` 权限。

## 2. 创建视频

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

需要音视频参考或首尾帧时，可在 `/v1/videos` 使用平台扩展 `content` 请求结构；该结构使用 `duration、ratio`，不要与 `prompt、seconds、size` 混用。具体内容结构见 [扩展参数与旧协议说明](../CLIENT_API.md)。`seed、generate_audio、watermark、draft、metadata` 当前不接受，传入会报错。

创建响应示意（任务可能已经进入后续状态）：

```json
{"id":"task_example","object":"video","model":"seedance-2.0-1080p","status":"queued","progress":0,"created_at":1789952400,"completed_at":null,"seconds":"5","size":"1920x1080","expires_at":null,"error":null,"resolution":"1080p"}
```

## 3. 查询、下载和删除

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

`GET /v1/videos/{id}/content` 下载第一条视频；同时支持 HEAD，Range 请求按内容服务实际响应返回。尚未成功返回 409 `video_not_ready`，不存在、不属于当前用户或已隐藏返回 404；当前 Key 缺少该模型权限返回 403。多输出通过 `urls` 获取其余结果。0.6.4 会自动保存成片，保存期为完成后 7 天；已保存成片不依赖原下载链接继续有效。到期后返回 410 content_expired，并周期清理保存的成片；任务记录仍可查询。历史已失效、未能保存的文件不能凭升级恢复。查询中的 storage.status 可区分正在保存、已保存和暂不可用，详见 [保存与下载周期](RETENTION.md)。不要通过重新生成来重试下载。

平台扩展：`GET /v1/videos?page=1&page_size=20&status=completed`，返回 `object:list、data、total、page、page_size`，只含本用户且当前 Key 有权使用的模型任务；可按 `model`、`status`、逗号分隔的 `ids` 过滤。`page_size` 最大 100，默认 20；单次目录扫描最多 50,000 条，超限返回 422 `catalog_scan_limit`。

平台扩展：`DELETE /v1/videos/{id}` 只隐藏成功或失败终态任务，返回 `{"id":"task_example","deleted":true}`；后续查询及本 API 下载返回 404。运行中删除返回 409，不取消生成、不触发退款。

## 4. 素材与素材组的完整关联示例

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

## 5. 同一素材能否用于不同模型

素材归属同一个平台用户。同一用户多个 Key 共享素材，但各 Key 仍须具备 `media-service` 和目标模型权限。不同用户不能共享素材 ID。

同一已就绪素材可以在有兼容引用能力、且可使用该素材所属服务通道的模型间复用，不按 720p/1080p 创建两份。无需为每次视频任务重复导入。平台会绑定素材所在通道；若该通道不支持目标模型、素材未就绪、类型不匹配或超出模型限制，请求会被拒绝。

当前没有承诺所有模型、所有通道无条件通用，也没有自动跨通道复制素材。切换 `model` 不改变素材所有权；是否可复用取决于权限、状态、类型、时长和目标模型能力。正式批量使用前应先验证目标组合。

提交前可调用不计费的预检查：

```bash
curl "$API_BASE/seedance/media/preflight" -H "Authorization: Bearer $API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"asset_id":"task_asset_example","model":"seedance-2.0-1080p","media_type":"video","role":"reference_video"}'
```

返回 `compatibility` 和逐项 `checks`，使用 supported/unsupported/unverified。unverified 表示证据不足，不是已经支持。素材标签、标签筛选和错误分类详见 [接口补充说明](REPAIR_API.md)。

## 6. 错误与兼容边界

错误结构统一为 `{"error":{"code":"...","message":"..."}}`。401 表示认证无效；403 表示权限限制；404 表示资源不可见；409 表示状态或幂等冲突；410 表示内容已到期；413 表示请求过大；422 表示媒体校验失败；502/503 表示服务暂不可用。不要根据错误文本重新提交付费任务。

旧 `/seedance/api/v3/contents/generations/tasks` 路径继续保留，使用原来的 `running/succeeded` 状态和响应格式。新接入统一采用本指南的 `/v1/videos`。原始通用任务查询不作为客户入口；视频内容从本平台认证接口返回。

本版不提供真实生成 token、随机种子、运行中取消或所有参数透传。费用按平台既有价格配置和消费记录结算。公开响应只含平台 ID、模型名、目录信息与本平台下载地址。

## 7. 回调、Postman 与调用顺序

回调是平台扩展，需要预先配置接收域名。`GET /seedance/capabilities` 的 `tasks.callback` 只表示平台是否配置主机白名单；为 true 也需确认自己的 HTTPS 域名已获允许。回调 data 中的 storage 为通知生成时的保存状态，后续以任务查询为准。创建时成对传入 `callback_url` 与 `callback_secret`，具体签名、重试和通知结构见 [完成回调](../CLIENT_API.md#完成回调)。本次修复后新建的 `/v1/videos` 任务通知使用 `version: "video.v1"`、`video.completed/video.failed` 及完整 Video 对象。历史任务和旧创建入口仍使用旧通知。未完成联调时使用轮询。

导入同目录的 [Postman 集合](Seedance.postman_collection.json) 与 [环境模板](Seedance.postman_environment.json)，填写自己的 `api_key` 和可配置的 `base_url`。先查询模型，再手动提交一次生成，复制任务 ID 后查询与下载；素材流程先创建组、导入或上传、等到就绪，再引用素材。每次业务请求保存独立幂等键，同一次请求重试保持原键与原内容。集合中的生成会计费、删除会改变可见性，不要批量运行整个集合。

完整流程为：认证 → 查询模型 → 创建素材组 → 导入/上传并关联 → 查询素材就绪 → 创建视频 → 轮询 completed/failed → 认证下载 → 按需隐藏终态任务。

标签、素材适用预检查及稳定错误分类，见 [接口补充说明](REPAIR_API.md)。
