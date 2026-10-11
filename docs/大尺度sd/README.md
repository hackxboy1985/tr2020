# 视频生成 API 客户接入包

版本：0.6.6 · 整理日期：2026-09-29

此文件夹可单独交付。视频创建、查询、素材管理与下载均通过客户网关；返回的 `task_…`、`asset://task_…` 和 `/v1/videos/…/content` 是本平台的任务、素材与认证下载地址。下载时将相对路径直接拼接到完整 API 基址，并携带自己的 Bearer Key。

## 从哪里开始

1. 阅读 [快速接入指南](docs/QUICKSTART.md)，按“查询模型 → 准备素材 → 等待就绪 → 创建视频 → 轮询 → 下载”接入。
2. 查阅 [视频接口详细说明](docs/VIDEO_API.md)；素材、分组、回调与旧接口见 [扩展说明](CLIENT_API.md)。
3. 使用 [Postman 集合](docs/Seedance.postman_collection.json) 和 [环境模板](docs/Seedance.postman_environment.json) 手动联调，或运行下方 Python/Node.js 示例。

首次联调建议先调用不计费的 `GET /v1/models`。创建视频的 `POST /v1/videos` 可能计费；导入素材返回 `queued` 时，须等到素材 `succeeded` 再引用。

## 本文件夹内容

| 文件 | 用途 |
|---|---|
| [docs/QUICKSTART.md](docs/QUICKSTART.md) | 最短接入路径、curl 示例、常见错误 |
| [docs/VIDEO_API.md](docs/VIDEO_API.md) | 视频创建、状态、下载和素材关联的接口说明 |
| [CLIENT_API.md](CLIENT_API.md) | 素材管理、参数扩展、回调与旧接口兼容 |
| [docs/RETENTION.md](docs/RETENTION.md) | 成片保存和下载期限 |
| [docs/REPAIR_API.md](docs/REPAIR_API.md) | 素材标签、预检查与错误码补充 |
| [docs/Seedance.postman_collection.json](docs/Seedance.postman_collection.json) | 可导入 Postman 的请求集合 |
| [docs/Seedance.postman_environment.json](docs/Seedance.postman_environment.json) | Postman 环境变量模板 |
| [examples/python/seedance_client.py](examples/python/seedance_client.py) | Python 3 标准库接入示例 |
| [examples/node/seedance_client.mjs](examples/node/seedance_client.mjs) | Node.js 20+ 接入示例 |

## 运行示例

在**本文件夹根目录**运行。先在服务端环境中配置：

```bash
export SEEDANCE_BASE_URL='http://106.54.45.168/seedance-gateway'
export SEEDANCE_API_KEY='<自己的 API Key>'
```

不加 `--run` 时，两个脚本只查询当前 Key 可见模型：

```bash
python3 examples/python/seedance_client.py
node examples/node/seedance_client.mjs
```

以下命令会导入参考视频并创建**可能计费**的视频任务：

```bash
python3 examples/python/seedance_client.py --run \
  --reference-url 'https://example.com/reference.mp4' \
  --reference-type video \
  --model seedance-2.0-1080p \
  --prompt '参考视频中的人物变成古代书生'

node examples/node/seedance_client.mjs --run \
  --reference-url=https://example.com/reference.mp4 \
  --reference-type=video \
  --model=seedance-2.0-1080p \
  --prompt='参考视频中的人物变成古代书生'
```

将示例 URL 换成真实、可访问的文件地址。脚本会打印素材 URI、视频任务 ID 和幂等键；生成完成后将首个输出保存为 `seedance-output.mp4`，可用 `--output` 修改。若重试同一次生成，使用打印的 `asset://task_…` 作为 `--asset-uri`，并复用原幂等键及完全相同的模型、提示词、时长和素材类型。提交超时而未取得任务 ID 时，先核对任务列表与消费记录，勿更换幂等键盲目重提。

## 接入边界

- API Key 仅保存在服务端，不放进网页、移动端包、仓库或 URL。当前示例基址为 HTTP；可用的 HTTPS 入口以平台通知为准。
- `GET /v1/models` 反映 Key 可见目录；`POST /seedance/media/preflight` 的 `unverified` 不表示目标模型组合已完成真实生成验收。正式使用前请用自己的素材做小额效果与计费核对。
- 视频状态是 `queued/in_progress/completed/failed`；素材状态是 `queued/running/succeeded/failed`。两套状态不要混用。
- 成片完成后保存 7 天；下载需认证。详情见 [保存与下载期限](docs/RETENTION.md)。
- Postman 集合包含计费生成和删除请求，请逐项手动执行，不要整套批量运行。
