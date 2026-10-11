# 客户 API 快速接入（0.6.6）

本页只使用客户网关公开接口。视频以 `/v1/videos` 创建、查询和下载；素材使用 `/seedance/media`。接口返回的是本平台的任务 ID 和认证下载路径，不提供匿名播放链接。完整字段和扩展操作见 [视频接口](VIDEO_API.md)、[素材与回调](../CLIENT_API.md)、[保存期限](RETENTION.md)。

## 接入前准备

在平台账户中创建 API Key，并授权目标视频模型；需要导入素材时还须授权 `media-service`。服务端配置以下环境变量，基址必须包含平台分配的路径前缀。当前生产入口示例使用 HTTP，密钥只放服务端；HTTPS 入口以平台通知为准。

```bash
export SEEDANCE_BASE_URL='http://106.54.45.168/seedance-gateway'
export SEEDANCE_API_KEY='<自己的 API Key>'
```

先做不计费检查：

```bash
curl --fail-with-body "$SEEDANCE_BASE_URL/v1/models" \
  -H "Authorization: Bearer $SEEDANCE_API_KEY"
curl --fail-with-body "$SEEDANCE_BASE_URL/seedance/capabilities" \
  -H "Authorization: Bearer $SEEDANCE_API_KEY"
```

`GET /v1/models` 只显示当前 Key 可见模型，不保证每种参考素材组合已通过真实生成。模型 ID 大小写敏感，使用完整的 `-720p` 或 `-1080p` 后缀。

## 路径 A：纯文字或图片参考

以下请求会创建**计费任务**。每次业务请求生成一个固定的 `Idempotency-Key`（8–128 位字母、数字、点、下划线、冒号或连字符）；重试同一请求须复用原键和原请求体。

```bash
curl --fail-with-body "$SEEDANCE_BASE_URL/v1/videos" \
  -H "Authorization: Bearer $SEEDANCE_API_KEY" \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: example-text-20260929-001' \
  --data '{"model":"seedance-2.0-720p","prompt":"古代街道，镜头缓慢向前推进","seconds":5,"size":"1280x720"}'
```

返回 `id` 后只轮询该任务：

```bash
curl --fail-with-body "$SEEDANCE_BASE_URL/v1/videos/task_替换为返回ID" \
  -H "Authorization: Bearer $SEEDANCE_API_KEY"
```

视频状态是 `queued → in_progress → completed`，也可能到 `failed`。完成后取响应中的相对 `url` 或 `urls[]`，**直接接在整个基址后面**并继续带 Bearer Key。例如：

```bash
curl --fail-with-body \
  "$SEEDANCE_BASE_URL/v1/videos/task_替换为返回ID/content" \
  -H "Authorization: Bearer $SEEDANCE_API_KEY" \
  --output result.mp4
```

图片 URL 可加 `"input_reference":"https://example.com/reference.png"`；也可先导入素材后使用 `asset://task_…`。输出有多条时逐个下载 `urls[]`；`storage.status=ready` 表示全部输出已保存。视频保存期是完成后 7 天；超期下载返回 410，任务记录仍可查询。

## 路径 B：先导入视频参考，再生成

素材地址必须是可访问的真实 HTTP(S) 文件 URL，不能把 Markdown 链接格式（`[url](url)`）放进 JSON。也可使用 `POST /seedance/media/upload` 上传原始文件字节，不能用 multipart。导入音视频时平台测量真实时长；单文件上限 64 MiB，每用户参考素材累计上限 512 MiB。

```bash
curl --fail-with-body "$SEEDANCE_BASE_URL/seedance/media" \
  -H "Authorization: Bearer $SEEDANCE_API_KEY" \
  -H 'Content-Type: application/json' \
  --data '{"action":"import","url":"https://example.com/reference.mp4","media_type":"video","title":"参考视频"}'
```

导入响应中的 `id` 是**素材任务 ID**，`asset_uri` 是后续引用值。HTTP 200 和 `status=queued` 只表示受理。每隔约 2 秒用 `POST /seedance/media` 查询，直到 `succeeded`，失败则停止；不要把排队中的素材直接交给 `/v1/videos`。

```bash
curl --fail-with-body "$SEEDANCE_BASE_URL/seedance/media" \
  -H "Authorization: Bearer $SEEDANCE_API_KEY" \
  -H 'Content-Type: application/json' \
  --data '{"action":"get","id":"task_替换为素材ID"}'
```

可在提交前执行不计费预检查：

```bash
curl --fail-with-body "$SEEDANCE_BASE_URL/seedance/media/preflight" \
  -H "Authorization: Bearer $SEEDANCE_API_KEY" \
  -H 'Content-Type: application/json' \
  --data '{"asset_id":"task_替换为素材ID","model":"seedance-2.0-1080p","media_type":"video","role":"reference_video"}'
```

`unsupported` 表示已知不兼容；`unverified` 表示尚无充分证据，不能当成已验证支持。实际生成和效果仍需小额试单。素材成功后提交平台扩展 `content` 请求，视频参考的 `role` 为 `reference_video`；已导入素材的测量时长会由平台补入，无须猜测 `duration`：

```bash
curl --fail-with-body "$SEEDANCE_BASE_URL/v1/videos" \
  -H "Authorization: Bearer $SEEDANCE_API_KEY" \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: example-video-ref-20260929-001' \
  --data '{"model":"seedance-2.0-1080p","duration":5,"ratio":"16:9","content":[{"type":"text","text":"参考视频中的人物变成古代书生"},{"type":"video_url","video_url":{"url":"asset://task_替换为素材ID"},"role":"reference_video"}],"n":1}'
```

这里的顶层 `duration` 是**目标成片时长**，不是参考视频时长。`content` 格式不要混用 `prompt/seconds/size` 格式。图片和音频参考分别使用 `image_url`/`reference_image`、`audio_url`/`reference_audio`；具体模型是否支持须按组合验证。

## 可运行代码

- [Python 3 标准库示例](../examples/python/seedance_client.py)：查询模型，按需导入/轮询素材，创建/轮询/下载视频。
- [Node.js 20+ 示例](../examples/node/seedance_client.mjs)：同一流程，使用内置 `fetch`。
- [Postman 集合](Seedance.postman_collection.json) 与 [环境模板](Seedance.postman_environment.json)：手动逐项执行，生成会计费。

两个脚本默认只查询模型；明确传入 `--run` 才会导入素材并创建计费任务。例如：

```bash
python3 examples/python/seedance_client.py --run \
  --reference-url 'https://example.com/reference.mp4' \
  --reference-type video \
  --model seedance-2.0-1080p \
  --prompt '参考视频中的人物变成古代书生'

# Node.js 20+ 使用等号形式传参；省略参考 URL 即纯文字生成
node examples/node/seedance_client.mjs --run \
  --reference-url=https://example.com/reference.mp4 \
  --reference-type=video \
  --model=seedance-2.0-1080p \
  --prompt='参考视频中的人物变成古代书生'
```

使用 `--idempotency-key` 固定重试键；省略时脚本生成随机键并在提交前打印。脚本也会打印 `Asset URI`。重试已导入素材的同一请求时，使用 `--asset-uri asset://task_…`（Node.js 写作 `--asset-uri=asset://task_…`）替代 `--reference-url`，并保持模型、提示词、时长、素材类型及原幂等键完全一致，避免再次导入后产生不同的请求体。若请求超时且未得到任务 ID，先核对 `/v1/videos` 列表及消费记录；结果仍不明时联系平台，不要换键盲目重提。脚本下载到当前目录的 `seedance-output.mp4`，可用 `--output` 修改。

## 接口和错误速查

| 方法与路径 | 用途 | 是否计费/改变数据 |
|---|---|---|
| `GET /v1/models`、`GET /seedance/capabilities` | 查看当前 Key 的目录与平台声明能力 | 否 |
| `POST /seedance/media` `action=import/get/list/search/update/delete/batch_get` | 素材导入及管理 | 导入/修改/删除会改变数据 |
| `GET /seedance/media` | 本人素材分页列表 | 否 |
| `POST /seedance/media/upload` | 原始文件字节上传并导入 | 会改变数据 |
| `POST /seedance/media/preflight` | 已知适用条件检查 | 否 |
| `POST /seedance/media/groups` | 素材组增查改删 | 写操作改变数据 |
| `POST /v1/videos` | 创建视频 | 是，可能计费 |
| `GET /v1/videos/{id}`、`GET /v1/videos` | 查询单个/分页任务 | 否 |
| `GET /v1/videos/{id}/content` | 下载首个输出 | 否，需认证 |
| `GET /v1/tasks/{id}/artifacts/video_2/content` | 下载第二个输出；以返回的 `urls[]` 为准 | 否，需认证 |
| `DELETE /v1/videos/{id}` | 隐藏终态任务 | 改变可见性，不取消或退款 |

错误统一含 `error.code`。`401/403` 检查 Key 与权限；`409 asset_not_ready` 继续查询同一素材，成功后以原请求和原幂等键重试；`422 asset_failed` 或 `reference_duration_unverified` 要检查素材，等待不能修复；`409 idempotency_conflict` 表示同键请求内容改变；`409 submission_outcome_unknown`、超时或 5xx 表示提交结果可能不明，先核对任务和消费记录；`410 content_expired` 表示下载期已过。生成失败后查询任务和消费/退款记录，不以 HTTP 200 代替完成判断。

当前接入地址为 HTTP，公网回调需要另行配置允许的 HTTPS 接收域名。未完成回调联调时使用轮询。本文的示例响应和路径是平台封装协议，不证明某一上游供应商的官方直连身份。
