# Doubao 官方 API 兼容接口文档

本接口完全兼容豆包（Doubao）视频生成官方 API 的请求和响应格式。调用方只需要替换 Base URL 和 API Token，无需修改代码。

## 接口地址

```text
Base URL: https://your-domain.com
```

| 操作 | 方法和路径 |
| --- | --- |
| 创建视频任务 | `POST /api/v3/contents/generations/tasks` |
| 查询视频任务 | `GET /api/v3/contents/generations/tasks/{task_id}` |
| 查询任务列表 | `GET /api/v3/contents/generations/tasks` |
| 取消任务 | `DELETE /api/v3/contents/generations/tasks/{task_id}` |

## 鉴权

```http
Authorization: Bearer sk-<NewAPI Token>
Content-Type: application/json
```

## 特性说明

- ✅ **完全兼容豆包官方 API 格式**：字段名称、JSON 结构与官方完全一致
- ✅ **入参支持官方格式**：`content[]` 数组、`resolution: "720p"`、`ratio: "16:9"` 等
- ✅ **出参格式一致**：响应结构与官方 API 完全相同
- ✅ **任务 ID 管理**：返回本系统生成的 `task_xxx` 格式 ID，用于后续查询和管理
- ✅ **透传所有字段**：保留官方所有参数，支持未来新增字段

## 1. 创建视频任务

### 请求示例

```bash
curl -X POST \
  'https://your-domain.com/api/v3/contents/generations/tasks' \
  -H 'Authorization: Bearer sk-<NewAPI Token>' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "doubao-seedance-2-0-260128",
    "content": [
      {
        "type": "text",
        "text": "一只猫在弹钢琴"
      },
      {
        "type": "image_url",
        "image_url": {
          "url": "https://example.com/image.jpg"
        },
        "role": "reference_image"
      }
    ],
    "resolution": "720p",
    "ratio": "16:9",
    "duration": 5,
    "generate_audio": false,
    "watermark": false,
    "seed": 12345
  }'
```

### 支持的参数

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| model | string | 是 | 模型名称，如 `doubao-seedance-2-0-260128` |
| content | array | 是 | 内容数组，包含 text、image_url、video_url、audio_url 等 |
| resolution | string | 否 | 分辨率，如 `720p`、`1080p` |
| ratio | string | 否 | 宽高比，如 `16:9`、`9:16` |
| duration | int | 否 | 视频时长（秒） |
| generate_audio | bool | 否 | 是否生成音频 |
| watermark | bool | 否 | 是否添加水印 |
| seed | int | 否 | 随机种子 |
| service_tier | string | 否 | 服务等级 |
| camera_fixed | bool | 否 | 是否固定镜头 |
| draft | bool | 否 | 是否为草稿模式 |

### 响应示例

```json
{
  "id": "task_rYVBCZdmBujBIc01dGJEZAZ9t4KyVFkn"
}
```

保存响应中的 `id` 用于后续查询。

## 2. 查询视频任务

### 请求示例

```bash
curl 'https://your-domain.com/api/v3/contents/generations/tasks/task_rYVBCZdmBujBIc01dGJEZAZ9t4KyVFkn' \
  -H 'Authorization: Bearer sk-<NewAPI Token>'
```

### 响应示例

```json
{
  "id": "task_rYVBCZdmBujBIc01dGJEZAZ9t4KyVFkn",
  "model": "doubao-seedance-2-0-260128",
  "status": "succeeded",
  "content": {
    "video_url": "https://ark-acg-cn-beijing.tos-cn-beijing.volces.com/..."
  },
  "usage": {
    "completion_tokens": 389700,
    "total_tokens": 389700
  },
  "created_at": 1787648453,
  "updated_at": 1787648683,
  "seed": 10136,
  "resolution": "720p",
  "ratio": "9:16",
  "duration": 18,
  "framespersecond": 24,
  "service_tier": "default",
  "execution_expires_after": 172800,
  "generate_audio": true,
  "draft": false,
  "priority": 0,
  "output_format": "mp4"
}
```

### 任务状态

| 状态 | 说明 |
| --- | --- |
| pending | 等待中 |
| queued | 排队中 |
| processing / running | 处理中 |
| succeeded | 成功 |
| failed | 失败 |

成功时，视频地址位于 `content.video_url` 字段。

## 3. 查询任务列表

### 请求示例

```bash
# 基础查询
curl 'https://your-domain.com/api/v3/contents/generations/tasks?page_num=1&page_size=20' \
  -H 'Authorization: Bearer sk-<NewAPI Token>'

# 按状态筛选
curl 'https://your-domain.com/api/v3/contents/generations/tasks?filter.status=succeeded&page_num=1&page_size=20' \
  -H 'Authorization: Bearer sk-<NewAPI Token>'

# 查询指定任务 ID
curl 'https://your-domain.com/api/v3/contents/generations/tasks?filter.task_ids=task_xxx&filter.task_ids=task_yyy' \
  -H 'Authorization: Bearer sk-<NewAPI Token>'

# 按模型和服务等级筛选
curl 'https://your-domain.com/api/v3/contents/generations/tasks?filter.model=doubao-seedance-2-0-260128&filter.service_tier=default' \
  -H 'Authorization: Bearer sk-<NewAPI Token>'
```

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| page_num | int | 否 | 页码，默认 1，取值范围 [1, 500] |
| page_size | int | 否 | 每页数量，默认 20，取值范围 [1, 500] |
| filter.status | string | 否 | 任务状态筛选（queued/running/succeeded/failed/cancelled） |
| filter.model | string | 否 | 模型名称筛选（精确匹配） |
| filter.service_tier | string | 否 | 服务等级筛选（default/flex） |
| filter.task_ids | string[] | 否 | 任务 ID 筛选（支持多个，重复参数名传递） |

### 响应示例

```json
{
  "items": [
    {
      "id": "task_rYVBCZdmBujBIc01dGJEZAZ9t4KyVFkn",
      "model": "doubao-seedance-2-0-260128",
      "status": "succeeded",
      "content": {
        "video_url": "https://...",
        "last_frame_url": "https://..."
      },
      "usage": {
        "completion_tokens": 389700,
        "total_tokens": 389700,
        "tool_usage": {
          "web_search": 0
        }
      },
      "created_at": 1787648453,
      "updated_at": 1787648683,
      "seed": 10136,
      "resolution": "720p",
      "ratio": "9:16",
      "duration": 18,
      "framespersecond": 24,
      "service_tier": "default",
      "execution_expires_after": 172800,
      "generate_audio": true,
      "draft": false,
      "output_format": "mp4",
      "tools": [
        {
          "type": "web_search"
        }
      ]
    }
  ],
  "total": 1
}
```

### 说明

- 仅支持查询最近 7 天的任务记录
- 视频 URL 有效期为 24 小时，请及时下载或转存
- Seedance 2.5 模型生成的视频 URL 下载次数上限为 100 次

## 4. 取消任务

### 请求示例

```bash
curl -X DELETE \
  'https://your-domain.com/api/v3/contents/generations/tasks/task_rYVBCZdmBujBIc01dGJEZAZ9t4KyVFkn' \
  -H 'Authorization: Bearer sk-<NewAPI Token>'
```

### 说明

- 仅可取消 `queued`、`processing`、`in_progress` 状态的任务
- 已完成（`succeeded`）或已失败（`failed`）的任务无法取消

### 响应示例

```json
{
  "id": "task_rYVBCZdmBujBIc01dGJEZAZ9t4KyVFkn",
  "model": "doubao-seedance-2-0-260128",
  "status": "failed",
  "error": {
    "code": "task_cancelled",
    "message": "cancelled by user"
  },
  "created_at": 1787648453,
  "updated_at": 1787648690
}
```

## 支持的模型

- `doubao-seedance-1-0-pro-250528`
- `doubao-seedance-1-0-lite-t2v`
- `doubao-seedance-1-0-lite-i2v`
- `doubao-seedance-1-5-pro-251215`
- `doubao-seedance-2-0-260128`
- `doubao-seedance-2-0-fast-260128`

## 与其他接口的区别

### Doubao 官方路径 vs OpenAI 兼容路径

| 功能 | Doubao 官方路径 | OpenAI 兼容路径 |
| --- | --- | --- |
| 创建任务 | `POST /api/v3/contents/generations/tasks` | `POST /v1/videos` 或 `/v1/video/generations` |
| 查询任务 | `GET /api/v3/contents/generations/tasks/{id}` | `GET /v1/videos/{id}` 或 `/v1/video/generations/{id}` |
| 查询列表 | `GET /api/v3/contents/generations/tasks` | 不支持 |
| 取消任务 | `DELETE /api/v3/contents/generations/tasks/{id}` | 不支持 |
| 入参格式 | Doubao 官方 `content[]` 数组格式 | OpenAI 简化格式（`prompt` + `metadata`） |
| 出参格式 | Doubao 官方 responseTask 格式 | OpenAI Video 格式 |

## 注意事项

1. **完全兼容**：本接口与豆包官方 API 完全兼容，可无缝替换
2. **任务 ID**：返回的 `id` 是本系统生成的 `task_xxx` 格式，用于隔离和管理
3. **计费**：按本系统配置的模型价格和分组折扣计费
4. **额度管理**：遵循本系统的用户额度和速率限制
5. **向后兼容**：原有 `/v1/videos/` 和 `/v1/video/generations` 路径仍然可用

## 迁移指南

如果您当前使用豆包官方 API，迁移步骤：

1. 将 Base URL 从 `https://ark.cn-beijing.volces.com` 改为您的 NewAPI 地址
2. 将 API Key 从豆包官方 Key 改为 NewAPI Token（`sk-xxx` 格式）
3. 代码其他部分无需修改

## 错误处理

所有接口遵循统一的错误响应格式：

```json
{
  "code": "error_code",
  "message": "错误描述",
  "status_code": 400
}
```

常见错误码：

| 错误码 | 说明 |
| --- | --- |
| invalid_request | 请求参数错误 |
| task_not_found | 任务不存在 |
| task_not_cancellable | 任务状态不支持取消 |
| unauthorized | 认证失败 |
| insufficient_quota | 额度不足 |

## 示例代码

### Python

```python
import requests

BASE_URL = "https://your-domain.com"
API_KEY = "sk-your-token"

headers = {
    "Authorization": f"Bearer {API_KEY}",
    "Content-Type": "application/json"
}

# 创建任务
response = requests.post(
    f"{BASE_URL}/api/v3/contents/generations/tasks",
    headers=headers,
    json={
        "model": "doubao-seedance-2-0-260128",
        "content": [
            {"type": "text", "text": "一只猫在弹钢琴"}
        ],
        "resolution": "720p",
        "ratio": "16:9",
        "duration": 5
    }
)
task_id = response.json()["id"]

# 查询任务
response = requests.get(
    f"{BASE_URL}/api/v3/contents/generations/tasks/{task_id}",
    headers=headers
)
task = response.json()
print(f"状态: {task['status']}")
if task['status'] == 'succeeded':
    print(f"视频URL: {task['content']['video_url']}")
```

### Node.js

```javascript
const axios = require('axios');

const BASE_URL = 'https://your-domain.com';
const API_KEY = 'sk-your-token';

const headers = {
    'Authorization': `Bearer ${API_KEY}`,
    'Content-Type': 'application/json'
};

// 创建任务
const createTask = async () => {
    const response = await axios.post(
        `${BASE_URL}/api/v3/contents/generations/tasks`,
        {
            model: 'doubao-seedance-2-0-260128',
            content: [
                { type: 'text', text: '一只猫在弹钢琴' }
            ],
            resolution: '720p',
            ratio: '16:9',
            duration: 5
        },
        { headers }
    );
    return response.data.id;
};

// 查询任务
const getTask = async (taskId) => {
    const response = await axios.get(
        `${BASE_URL}/api/v3/contents/generations/tasks/${taskId}`,
        { headers }
    );
    return response.data;
};

(async () => {
    const taskId = await createTask();
    console.log(`任务ID: ${taskId}`);
    
    const task = await getTask(taskId);
    console.log(`状态: ${task.status}`);
    if (task.status === 'succeeded') {
        console.log(`视频URL: ${task.content.video_url}`);
    }
})();
```

## 相关文档

- [ARK Video API 文档](./ARK-VIDEO-API.zh-CN.md) - 火山方舟原生兼容接口（旧版）
- [豆包官方文档](https://docs.volcengine.com/docs/82379/1521675) - 豆包视频生成 API 官方文档
