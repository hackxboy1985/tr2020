# Seedance 接口文档（简化版）
- 包含素材创建及视频生成文档，可直接发起测试

## 概述

上传素材后，可在视频生成接口中通过 `asset://<asset_id>` 引用。

**无需手动创建素材组**：首次上传时系统自动为当前用户创建默认素材组。

## 鉴权

```http
Authorization: Bearer sk-<API_TOKEN>
Content-Type: application/json
```

---

## 1、创建素材

```http
POST /api/seedance/assets
```

**请求体：**

```json
{
  "URL": "https://example.com/image.jpg",
  "AssetType": "Image",
  "Name": "my-image"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `URL` | string | 是 | 公网可访问的 HTTPS 文件地址 |
| `AssetType` | string | 是 | `Image`、`Video` 或 `Audio` |
| `Name` | string | 否 | 素材名称 |
| `GroupId` | string | 否 | 指定素材组，不填则自动使用默认组 |
| `Force` | bool | 否 | `true` 时强制重新上传，忽略本地缓存；默认 `false` |

> **幂等性**：相同 URL 已有 `Active` 状态的素材时，直接返回已有素材，不重复上传。设置 `Force: true` 可强制重新上传。

**成功响应：**

```json
{
  "Result": {
    "Id": "asset-xxxxxxxx",
    "LocalId": 12,
    "AssetRef": "asset://asset-xxxxxxxx",
    "Status": "Processing"
  }
}
```

| 字段 | 说明 |
|------|------|
| `Result.Id` | 上游 asset_id |
| `Result.LocalId` | 本地记录 ID，用于查询接口的 `:id` 参数 |
| `Result.AssetRef` | 视频生成时直接填入 `images[]` 的值 |

---

## 2、查询素材状态

```http
GET /api/seedance/assets/:id
```

`:id` 支持两种格式：
- **本地 ID**（数字）：创建时响应里的 `Result.LocalId`
- **上游 asset_id**：创建时响应里的 `Result.Id`，例如 `asset-xxxxxxxx`

**响应示例：**

```json
{
  "Result": {
    "Id": "asset-xxxxxxxx",
    "Status": "Active"
  }
}
```

| 状态 | 说明 |
|------|------|
| `Processing` | 正在预处理，稍后重试 |
| `Active` | 可用，可用于视频生成 |
| `Failed` | 处理失败 |

轮询直到状态变为 `Active`，建议间隔 3 秒。


---

## 3、删除素材

```http
DELETE /api/seedance/assets/:id
```

`:id` 同样支持本地 ID（数字）或上游 `asset-xxxxxxxx` 格式。

---

## 3、生成视频及在视频中引用素材

素材状态为 `Active` 后，将 `Result.AssetRef`（即 `asset://asset-xxxxxxxx`）用于视频生成。

### 格式一：标准格式

```http
POST /v1/video/generations
```

```json
{
  "model": "doubao-seedance-2-0-260128",
  "prompt": "人物自然介绍产品",
  "seconds": "5",
  "images": ["asset://asset-xxxxxxxx"],
  "metadata": {
    "resolution": "720p",
    "ratio": "16:9",
    "generate_audio": false
  }
}
```

### 格式二：Ark 原生格式

```http
POST /v1/video/generations
```

```json
{
  "model": "doubao-seedance-2-0-260128",
  "content": [
    {"type": "text", "text": "人物自然介绍产品（使用音频中的声音）"},
    {"type": "image_url", "image_url": {"url": "asset://asset-xxxxxxxx"}},
    {"type": "audio_url", "audio_url": {"url": "https://example.com/voice.wav"}}
  ],
  "resolution": "720p",
  "ratio": "16:9",
  "duration": 5,
  "generate_audio": true
}
```

两种格式均支持，`content[]` 中媒体类型会自动补充 `role` 字段（`image_url` → `reference_image`，`audio_url` → `reference_audio`，`video_url` → `reference_video`）。

### 4、查询视频任务

```http
GET /v1/videos/:task_id
```

```bash
curl http://open.mints-id.com/v1/videos/task_xxxxxxxx \
  -H "Authorization: Bearer sk-xxx"
```

---

## 完整示例

```bash
# 1. 上传素材
ASSET=$(curl -s -X POST http://open.mints-id.com/api/seedance/assets \
  -H "Authorization: Bearer sk-xxx" \
  -H "Content-Type: application/json" \
  -d '{"URL":"https://example.com/photo.jpg","AssetType":"Image","Name":"photo"}')

LOCAL_ID=$(echo $ASSET | jq -r '.Result.LocalId')
ASSET_REF=$(echo $ASSET | jq -r '.Result.AssetRef')
echo "local_id=$LOCAL_ID, asset_ref=$ASSET_REF"

# 2. 轮询等待 Active
while true; do
  STATUS=$(curl -s http://open.mints-id.com/api/seedance/assets/$LOCAL_ID \
    -H "Authorization: Bearer sk-xxx" | jq -r '.Result.Status')
  echo "Status: $STATUS"
  [ "$STATUS" = "Active" ] && break
  [ "$STATUS" = "Failed" ] && { echo "Failed"; exit 1; }
  sleep 3
done

# 3. 视频生成
curl -X POST http://open.mints-id.com/v1/video/generations \
  -H "Authorization: Bearer sk-xxx" \
  -H "Content-Type: application/json" \
  -d "{
    \"model\": \"doubao-seedance-2-0-260128\",
    \"prompt\": \"人物自然介绍产品\",
    \"seconds\": \"5\",
    \"images\": [\"$ASSET_REF\"],
    \"metadata\": {\"resolution\": \"720p\", \"ratio\": \"16:9\"}
  }"
```
