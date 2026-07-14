# 上游视频Seedance 2.0 火山方舟原生兼容接口文档

本接口兼容火山方舟视频生成 API 的请求字段。调用方只需要替换 Base URL 和 API Token，不需要把官方字段改写到 `metadata`。

## 接口地址

```text
Base URL: https://sd.dawnloadai.com:8443
```

| 操作 | 方法和路径 |
| --- | --- |
| 创建视频任务 | `POST /api/v3/contents/generations/tasks` |
| 查询视频任务 | `GET /api/v3/contents/generations/tasks/{task_id}` |

鉴权：

```http
Authorization: Bearer sk-<NewAPI Token>
Content-Type: application/json
```

## 透传规则

- 官方请求字段保持在 JSON 根级，字段名称和数据结构不变。
- NewAPI 保留官方字段以及后续新增的未知字段。
- `model` 对外填写 `doubao-seedance-2-0-260128`，网关会在服务端映射到该用户的专属接入点。
- ProjectName、Ark API Key 和接入点 ID 不向调用方暴露。
- 对外任务 ID 由 NewAPI 生成并按用户隔离，创建响应中的 `id` 用于后续查询。
- 鉴权、额度、分组折扣和分辨率计费仍由 NewAPI 执行。

## 创建任务

```bash
curl -X POST \
  'https://sd.dawnloadai.com:8443/api/v3/contents/generations/tasks' \
  -H 'Authorization: Bearer sk-<NewAPI Token>' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "doubao-seedance-2-0-260128",
    "content": [
      {
        "type": "text",
        "text": "人物面对镜头自然介绍产品"
      },
      {
        "type": "image_url",
        "image_url": {
          "url": "asset://asset-xxxxxxxx"
        }
      }
    ],
    "resolution": "720p",
    "ratio": "16:9",
    "duration": 5,
    "generate_audio": false,
    "watermark": false
  }'
```

创建成功后保存响应中的 `id`。

## 输入素材

官方 `content` 数组可以直接使用以下类型：

```json
{"type":"image_url","image_url":{"url":"https://example.com/image.jpg"}}
```

```json
{"type":"video_url","video_url":{"url":"https://example.com/reference.mp4"}}
```

```json
{"type":"audio_url","audio_url":{"url":"https://example.com/reference.mp3"}}
```

素材库资源使用 `asset://<asset_id>`。输出分辨率和是否包含 `video_url` 会参与 Seedance 2.0 差异化计费。

## 查询任务

```bash
curl \
  'https://sd.dawnloadai.com:8443/api/v3/contents/generations/tasks/task_xxxxxxxxx' \
  -H 'Authorization: Bearer sk-<NewAPI Token>'
```

任务成功时，视频地址位于官方响应结构的 `content.video_url`。

## 旧兼容接口

原有 `POST /v1/video/generations` 和 `GET /v1/video/generations/{task_id}` 保留，已有调用无需修改。新接入建议使用本文件中的火山方舟原生接口。
