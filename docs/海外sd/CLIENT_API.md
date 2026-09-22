# 视频接口扩展、素材管理与旧协议说明

接口版本：0.6.4 · 文档更新：2026-09-22

本文对应 0.6.4，保留素材、扩展参数及旧协议说明。视频主接口和保存周期见 [通用视频指南](docs/VIDEO_API.md) 与 [保存与下载周期](docs/RETENTION.md)。

新接入请优先阅读 [通用视频接口指南](docs/VIDEO_API.md)，其中包含本版完整创建/查询/下载响应、素材组关联示例和跨模型复用边界。本页保留旧协议参数说明，历史验收记录不代表本次新版本已经完成全部生产验收。

## 1. 接入地址与认证

```text
API_BASE = http://106.54.45.168/seedance-gateway
Authorization: Bearer <你自己账户创建的 API Key>
Content-Type: application/json
```

本文所有接口路径均拼接到 API_BASE。当前提供 HTTP 接入，连接未加密；HTTPS 地址就绪后由平台通知切换，请将基址作为配置项。API Key 仅保存在服务端环境变量中，不要嵌入网页、移动端包、代码仓库或日志。

在平台账户内创建自己的 API Key。需要生成权限及媒体库权限时，应允许目标视频模型和 `media-service`。不要使用其他用户的 Key。归属按用户隔离：同一用户的多个 Key 共享该用户素材和任务，不作为独立租户。

## 2. 历史实测范围与本版边界

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

## 3. 模型目录

`GET /v1/models` 返回当前 Key 可见模型。以下为完整命名目录，并不代表所有模型均完成生产验收。

| 名称 | 720p 模型 ID | 1080p 模型 ID |
|---|---|---|
| Seedance 2.0 Mini | seedance-2.0-mini-720p | seedance-2.0-mini-1080p |
| Seedance 2.0 | seedance-2.0-720p | seedance-2.0-1080p |
| Seedance 2.5 | seedance-2.5-720p | seedance-2.5-1080p |
| Seedance 2.0-C | seedance-2.0-C-720p | seedance-2.0-C-1080p |
| Seedance 2.0-B | seedance-2.0-B-720p | seedance-2.0-B-1080p |

模型 ID 区分大小写，B/C 使用大写。媒体操作使用 `media-service` 权限。分辨率由模型后缀决定，不能通过参数改变。费用按照客户在平台配置的价格、分组倍率及消费记录结算；本文不作为固定报价单。余额及消费记录请登录平台个人账户查看，本文不提供余额查询 API。

## 接口速查

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

## 4. 五分钟接入：创建、轮询、下载

以下为 macOS/Linux Bash 示例，依赖 curl 和 jq。创建任务会按账户配置扣费。

```bash
export API_BASE='http://106.54.45.168/seedance-gateway'
export API_KEY='<替换成自己的 Key>'

# 先确认模型权限
curl --fail-with-body "$API_BASE/v1/models" \
  -H "Authorization: Bearer $API_KEY"

# 为一次业务提交保存一个固定的幂等键；重试时不要换键或修改 JSON
export REQUEST_ID="video-$(date +%s)-$RANDOM"
curl --fail-with-body "$API_BASE/v1/videos" \
  -H "Authorization: Bearer $API_KEY" \
  -H 'Content-Type: application/json' \
  -H "Idempotency-Key: $REQUEST_ID" \
  --data '{"model":"seedance-2.0-1080p","prompt":"蓝色与金色的抽象光影缓慢流动","seconds":5,"size":"1920x1080"}' \
  -o created.json
export TASK_ID="$(jq -er '.id' created.json)"

# 每隔 10 秒查询一次，直到 completed 或 failed；不要重新 POST 生成请求
curl --fail-with-body "$API_BASE/v1/videos/$TASK_ID" \
  -H "Authorization: Bearer $API_KEY" -o task.json
cat task.json

# 仅 completed 后执行；输出路径需直接拼接 API_BASE
export VIDEO_PATH="$(jq -er '.url' task.json)"
curl --fail-with-body "${API_BASE}${VIDEO_PATH}" \
  -H "Authorization: Bearer $API_KEY" -o output.mp4
```

**下载拼接示例：** 返回 `/v1/videos/task_example/content` 时，完整地址为 `http://106.54.45.168/seedance-gateway/v1/videos/task_example/content`。不要用会丢弃基址路径前缀的 URL 解析方式。下载需要相同用户的有效 Key，不是可匿名分享的播放链接。成片自动保存完成后（storage.status=ready）可在 completed_at 后 7 天内下载；超过 expires_at 返回 410。历史已过期且未保存文件无法自动恢复；请及时下载自存。

## 5. 上传图片并用于生成

```bash
# 原始二进制上传，不使用 multipart/form-data
curl --fail-with-body "$API_BASE/seedance/media/upload?title=reference" \
  -H "Authorization: Bearer $API_KEY" -H 'Content-Type: image/png' \
  --data-binary @reference.png -o uploaded.json
export ASSET_ID="$(jq -er '.id' uploaded.json)"

# 重复查询，直到 status 为 succeeded；不要把上传响应当成注册完成
curl --fail-with-body "$API_BASE/seedance/media" \
  -H "Authorization: Bearer $API_KEY" -H 'Content-Type: application/json' \
  --data "$(jq -n --arg id "$ASSET_ID" '{action:"get",id:$id}')"

# 注册成功后引用。此请求会创建新的付费生成任务
jq -n --arg asset "asset://$ASSET_ID" \
  '{model:"seedance-2.0-1080p",duration:5,ratio:"16:9",content:[{type:"text",text:"让画面中的光影缓慢流动"},{type:"image_url",image_url:{url:$asset},role:"first_frame"}]}' > generate.json
curl --fail-with-body "$API_BASE/v1/videos" \
  -H "Authorization: Bearer $API_KEY" -H 'Content-Type: application/json' \
  -H "Idempotency-Key: image-$(date +%s)-$RANDOM" --data-binary @generate.json
```

以下为详细接口契约。示例中的任务 ID、素材 ID 和分组 ID 均须替换为自己账户实际返回的值。

---

## 旧协议创建与 content 扩展

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

## 旧协议查询与下载

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

## 媒资服务

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

## 通用视频主入口

`POST /v1/videos`、`GET /v1/videos/{id}` 和 `GET/HEAD /v1/videos/{id}/content` 是主接入方式；另有列表和终态隐藏扩展，详见 [通用视频指南](docs/VIDEO_API.md)。模型使用同样的十个 Seedance 名称，后缀决定分辨率。`resolution` 和 `size` 若填写必须与模型匹配，例如 1080p 模型可用 `1920x1080` 或 `1080x1920`；不能通过参数切换到 720p。可用 JSON `prompt`、`seconds`、`size`、`input_reference`，或对应 multipart 文本字段；不支持文件上传。JSON 也可直接使用上面的 `content` 结构，两种结构不要混用。视频状态值遵循主协议：`queued`、`in_progress`、`completed`、`failed`。


## 媒体库与任务管理

所有新增接口仍使用同一 Bearer API Key。归属按平台用户判断，同用户不同 Key 共享素材目录，但分别执行 Key 的模型权限。API Key、过期时间、停用状态、用户状态和 IP 限制实时校验；不能通过传 user_id 指定归属。

- `GET /seedance/capabilities`：返回当前 Key 的模型目录、接口能力和限制。`models[].production_verified` 当前固定为 false，不代表模型不可用，也不是逐模型验收结果；真实验收范围见本文开头。`tasks.callback` 表示平台是否配置了回调主机白名单：false 表示暂未启用，true 仍需确认客户接收域名已被允许且已完成联调。
- `GET /seedance/media?page=1&page_size=20&keyword=角色` 或 `POST /seedance/media {"action":"list"}`：无需提供素材 ID；支持 `group_id`、`keyword`、`media_type`、`status`、`page`、`page_size`、`tag`。每页最大 100。历史导入记录也可列出，历史记录缺少类型时 `media_type` 为 null；列表状态为已保存的任务状态，需实时刷新素材状态时调用 get。
- `POST /seedance/media/groups`：`action` 为 create/get/list/update/delete；create 使用 name/description，get/delete 使用 id，update 使用 id/name/description，list 可按 keyword/page/page_size 筛选。返回 `group_...` 格式的分组 ID。非空组删除返回 409，先移动或删除素材。
- 导入可附加 `group_id`；修改素材可附加 `group_id`，传 null 移出分组。跨用户分组、素材和混合批次均拒绝。

### 本地文件上传

`POST /seedance/media/upload?title=参考图&group_id=group_...`

请求体是文件原始字节，Content-Type 为 `image/png`、`image/jpeg`、`image/webp`、`video/mp4`、`audio/mpeg`、`audio/wav` 或 `audio/mp4`，不是 multipart。单文件上限 64 MiB，单用户本地存储上限 512 MiB；服务最多同时处理两个二进制上传。服务会校验实际媒体类型并测量音视频时长，不接受任意文件改扩展名上传。

返回 `id`、`status`、`media_type` 和音视频的 `duration`。随后按原 get 接口等待注册成功，再用 `asset://task_...` 引用。上传产生的读取签名链接有效期 7 天（604800 秒），仅用于服务端抓取；持有该链接可在有效期内读取对应文件，调用方不应分享链接。签名过期不会自动删除已上传素材，也不等同于素材 ID 失效；素材能否继续用于生成应查询其状态。视频生成下载仍要求 Bearer。

对本地上传的音视频，引用时自动采用实测时长；若客户填写的 duration 与实测差异大于或等于 0.1 秒，拒绝请求。远程 URL 导入、直接 URL 参考仍依赖声明时长，尚未完成可信远程媒体计量。

### 旧协议任务列表与删除

`GET /seedance/api/v3/contents/generations/tasks?page=1&page_size=20&status=succeeded&model=seedance-2.0-C-720p`

返回 data/total/page/page_size。可按 ids（逗号分隔）筛选。也提供 `/api/v3/contents/generations/tasks` 路径别名，支持 filter.status、filter.model、filter.task_ids。

`DELETE /seedance/api/v3/contents/generations/tasks/{id}`：仅终态任务可删除，成功响应为 `{"id":"task_example","deleted":true}`，其中 id 为实际任务 ID；从客户目录、查询和下载入口隐藏，平台消费记录保留。重复删除幂等；非终态返回 409 `task_cannot_be_cancelled`。这不是服务端取消接口，不触发退款或停止服务端任务。

### 提交防重

创建视频时可带 `Idempotency-Key` 请求头（8–128 位字母、数字、点、下划线、冒号、短横线）。同用户、相同 Key 和相同 JSON 内容返回原结果；内容不同返回 409。创建请求超时或响应无法确认时，同 Key 返回 `submission_outcome_unknown`，不得换 Key 盲目重提，应先查任务列表并联系管理员对账。记录跨进程重启保留。

### 完成回调

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

### 当前边界

- 不支持运行中或已提交排队任务的服务端取消。
- 不提供真人认证 H5、视频后处理或官方 Seedance 全量参数兼容。
- 媒体库列表单次扫描最多 50,000 条当前用户导入/生成记录，超过会明确报 catalog_scan_limit，不静默截断。
- 终态删除为本地可见性控制；服务端素材删除仍使用原媒体 delete 操作。


## 错误处理与上线自检

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
