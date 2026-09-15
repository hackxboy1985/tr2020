# Doubao 官方 API 兼容接口文档

本接口完全兼容豆包（Doubao）视频生成官方 API 的请求和响应格式。调用方只需要替换 Base URL 和 API Token，无需修改代码。

## 接口地址

```text
Base URL: http://open.mints-id.com
```

| 操作 | 方法和路径 | 状态 |
| --- | --- | --- |
| 创建视频任务 | `POST /api/v3/contents/generations/tasks` | ✅ 支持 |
| 查询视频任务 | `GET /api/v3/contents/generations/tasks/{task_id}` | ✅ 支持 |
| 查询任务列表 | `GET /api/v3/contents/generations/tasks` | ✅ 支持 |
| 取消任务 | `DELETE /api/v3/contents/generations/tasks/{task_id}` | ✅ 支持 |

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
- 取消成功后，任务状态会变为 `cancelled`，并自动退还预扣费用

### 响应示例

**成功时（HTTP 200）：**

```json
{}
```

**任务不存在（HTTP 404）：**

```json
{
  "code": "task_not_exist",
  "message": "task not found or already deleted",
  "status_code": 404
}
```

**任务运行中，无法取消（HTTP 409）：**

```json
{
  "code": "task_not_cancellable",
  "message": "task is running, cannot cancel",
  "status_code": 409
}
```

**任务状态不支持取消（HTTP 400）：**

```json
{
  "code": "task_not_cancellable",
  "message": "task status is succeeded, cannot be cancelled",
  "status_code": 400
}
```

### 计费说明

- 取消成功后，任务状态会变为 `cancelled`
- 系统会自动退还预扣的全部配额
- 退款包括：
  - ✅ 用户余额/订阅额度
  - ✅ 完整的退款记录，便于对账
- 取消仅对 `queued` 状态有效，任务通常在提交后数秒内进入 `running`
- 如需取消，应在创建任务后立即发起

### 使用要点

- 根据官方 API 限制，只有 `queued` 状态的任务可以取消
- 任务进入 `running` 状态后无法取消（会返回 409 错误）
- 取消成功后，任务状态会更新为 `cancelled`
- 已取消的任务记录会保留在系统中，不会被删除

---

## 支持的模型
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

- [豆包官方文档](https://docs.volcengine.com/docs/82379/1521675) - 豆包视频生成 API 官方文档

---

## Seedance Asset API（素材管理）

除了视频生成任务接口，本系统还支持 Seedance 素材管理 API，用于管理视频生成所需的素材（图片、视频、音频）和素材组。

### 接口地址

```text
Base URL: http://open.mints-id.com/api/seedance/assets/v2
```

### 统一请求方式

所有 Seedance Asset API 使用统一的 POST 请求，通过 `Action` 参数区分不同操作：

```bash
POST /api/seedance/assets/v2/?Action={ActionName}&Version=2024-01-01
```

### 支持的操作

| 功能 | Action | 方法 | 路径 |
| --- | --- | --- | --- |
| 创建素材组 | CreateAssetGroup | POST | `/?Action=CreateAssetGroup&Version=2024-01-01` |
| 查询素材组 | ListAssetGroups | POST | `/?Action=ListAssetGroups&Version=2024-01-01` |
| 获取素材组 | GetAssetGroup | POST | `/?Action=GetAssetGroup&Version=2024-01-01` |
| 更新素材组 | UpdateAssetGroup | POST | `/?Action=UpdateAssetGroup&Version=2024-01-01` |
| 删除素材组 | DeleteAssetGroup | POST | `/?Action=DeleteAssetGroup&Version=2024-01-01` |
| 创建素材 | CreateAsset | POST | `/?Action=CreateAsset&Version=2024-01-01` |
| 查询素材列表 | ListAssets | POST | `/?Action=ListAssets&Version=2024-01-01` |
| 获取素材详情 | GetAsset | POST | `/?Action=GetAsset&Version=2024-01-01` |
| 更新素材 | UpdateAsset | POST | `/?Action=UpdateAsset&Version=2024-01-01` |
| 删除素材 | DeleteAsset | POST | `/?Action=DeleteAsset&Version=2024-01-01` |

### 鉴权

```http
Authorization: Bearer sk-<NewAPI Token>
Content-Type: application/json
```

### 使用示例

#### 1. 创建素材组

```bash
curl -X POST \
  'https://your-domain.com/api/seedance/assets/v2/?Action=CreateAssetGroup&Version=2024-01-01' \
  -H 'Authorization: Bearer sk-<NewAPI Token>' \
  -H 'Content-Type: application/json' \
  -d '{
    "Name": "我的素材组",
    "Description": "用于存放项目素材"
  }'
```

**响应示例：**

```json
{
  "GroupId": "group_xxxxxxxx",
  "Name": "我的素材组",
  "Description": "用于存放项目素材",
  "CreatedAt": 1787912449,
  "UpdatedAt": 1787912449
}
```

#### 2. 创建素材

```bash
curl -X POST \
  'https://your-domain.com/api/seedance/assets/v2/?Action=CreateAsset&Version=2024-01-01' \
  -H 'Authorization: Bearer sk-<NewAPI Token>' \
  -H 'Content-Type: application/json' \
  -d '{
    "GroupId": "group_xxxxxxxx",
    "Type": "image",
    "Url": "https://example.com/image.jpg",
    "Name": "参考图片",
    "Tags": ["参考", "人物"]
  }'
```

**响应示例：**

```json
{
  "AssetId": "asset_xxxxxxxx",
  "GroupId": "group_xxxxxxxx",
  "Type": "image",
  "Url": "https://example.com/image.jpg",
  "Name": "参考图片",
  "Tags": ["参考", "人物"],
  "CreatedAt": 1787912449,
  "UpdatedAt": 1787912449
}
```

#### 3. 查询素材列表

```bash
curl -X POST \
  'https://your-domain.com/api/seedance/assets/v2/?Action=ListAssets&Version=2024-01-01' \
  -H 'Authorization: Bearer sk-<NewAPI Token>' \
  -H 'Content-Type: application/json' \
  -d '{
    "GroupId": "group_xxxxxxxx",
    "PageNumber": 1,
    "PageSize": 20
  }'
```

**响应示例：**

```json
{
  "Assets": [
    {
      "AssetId": "asset_xxxxxxxx",
      "GroupId": "group_xxxxxxxx",
      "Type": "image",
      "Url": "https://example.com/image.jpg",
      "Name": "参考图片",
      "Tags": ["参考", "人物"],
      "CreatedAt": 1787912449,
      "UpdatedAt": 1787912449
    }
  ],
  "Total": 1,
  "PageNumber": 1,
  "PageSize": 20
}
```

#### 4. 查询素材组列表

```bash
curl -X POST \
  'http://open.mints-id.com/api/seedance/assets/v2/?Action=ListAssetGroups&Version=2024-01-01' \
  -H 'Authorization: Bearer sk-<NewAPI Token>' \
  -H 'Content-Type: application/json' \
  -d '{
    "PageNumber": 1,
    "PageSize": 20
  }'
```

**响应示例：**

```json
{
  "Groups": [
    {
      "GroupId": "group_xxxxxxxx",
      "Name": "我的素材组",
      "Description": "用于存放项目素材",
      "AssetCount": 5,
      "CreatedAt": 1787912449,
      "UpdatedAt": 1787912449
    }
  ],
  "Total": 1,
  "PageNumber": 1,
  "PageSize": 20
}
```

#### 5. 获取素材组详情

```bash
curl -X POST \
  'http://open.mints-id.com/api/seedance/assets/v2/?Action=GetAssetGroup&Version=2024-01-01' \
  -H 'Authorization: Bearer sk-<NewAPI Token>' \
  -H 'Content-Type: application/json' \
  -d '{
    "GroupId": "group_xxxxxxxx"
  }'
```

**响应示例：**

```json
{
  "GroupId": "group_xxxxxxxx",
  "Name": "我的素材组",
  "Description": "用于存放项目素材",
  "AssetCount": 5,
  "CreatedAt": 1787912449,
  "UpdatedAt": 1787912449
}
```

#### 6. 更新素材组

```bash
curl -X POST \
  'http://open.mints-id.com/api/seedance/assets/v2/?Action=UpdateAssetGroup&Version=2024-01-01' \
  -H 'Authorization: Bearer sk-<NewAPI Token>' \
  -H 'Content-Type: application/json' \
  -d '{
    "GroupId": "group_xxxxxxxx",
    "Name": "更新后的素材组名称",
    "Description": "更新后的描述"
  }'
```

**响应示例：**

```json
{
  "GroupId": "group_xxxxxxxx",
  "Name": "更新后的素材组名称",
  "Description": "更新后的描述",
  "UpdatedAt": 1787912550
}
```

#### 7. 删除素材组

```bash
curl -X POST \
  'http://open.mints-id.com/api/seedance/assets/v2/?Action=DeleteAssetGroup&Version=2024-01-01' \
  -H 'Authorization: Bearer sk-<NewAPI Token>' \
  -H 'Content-Type: application/json' \
  -d '{
    "GroupId": "group_xxxxxxxx"
  }'
```

**响应示例：**

```json
{
  "Success": true
}
```

#### 8. 获取素材详情

```bash
curl -X POST \
  'http://open.mints-id.com/api/seedance/assets/v2/?Action=GetAsset&Version=2024-01-01' \
  -H 'Authorization: Bearer sk-<NewAPI Token>' \
  -H 'Content-Type: application/json' \
  -d '{
    "AssetId": "asset_xxxxxxxx"
  }'
```

**响应示例：**

```json
{
  "AssetId": "asset_xxxxxxxx",
  "GroupId": "group_xxxxxxxx",
  "Type": "image",
  "Url": "https://example.com/image.jpg",
  "Name": "参考图片",
  "Tags": ["参考", "人物"],
  "Size": 1024000,
  "CreatedAt": 1787912449,
  "UpdatedAt": 1787912449
}
```

#### 9. 更新素材

```bash
curl -X POST \
  'http://open.mints-id.com/api/seedance/assets/v2/?Action=UpdateAsset&Version=2024-01-01' \
  -H 'Authorization: Bearer sk-<NewAPI Token>' \
  -H 'Content-Type: application/json' \
  -d '{
    "AssetId": "asset_xxxxxxxx",
    "Name": "更新后的素材名称",
    "Tags": ["更新", "新标签"]
  }'
```

**响应示例：**

```json
{
  "AssetId": "asset_xxxxxxxx",
  "Name": "更新后的素材名称",
  "Tags": ["更新", "新标签"],
  "UpdatedAt": 1787912550
}
```

#### 10. 删除素材

```bash
curl -X POST \
  'http://open.mints-id.com/api/seedance/assets/v2/?Action=DeleteAsset&Version=2024-01-01' \
  -H 'Authorization: Bearer sk-<NewAPI Token>' \
  -H 'Content-Type: application/json' \
  -d '{
    "AssetId": "asset_xxxxxxxx"
  }'
```

**响应示例：**

```json
{
  "Success": true
}
```

### 素材类型

| 类型 | 说明 | 用途 |
| --- | --- | --- |
| image | 图片素材 | 作为参考图片用于 image2video 生成 |
| video | 视频素材 | 作为参考视频用于视频续写 |
| audio | 音频素材 | 作为参考音频用于音频生成 |

### 在视频生成中使用素材

创建素材后，可以在视频生成任务中引用素材 ID：

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
        "text": "基于参考图生成视频"
      },
      {
        "type": "image_url",
        "image_url": {
          "url": "asset://asset_xxxxxxxx"
        },
        "role": "reference_image"
      }
    ],
    "duration": 5
  }'
```

### 注意事项

1. **URL 格式**：素材 URL 支持 `https://` 和 `asset://` 两种格式
   - `https://` - 直接指定素材 URL
   - `asset://asset_id` - 引用已创建的素材

2. **素材管理**：素材会持久化存储在系统中，可以重复使用

3. **权限控制**：每个用户只能访问自己创建的素材组和素材

4. **配额计费**：素材存储和使用不额外计费，仅在视频生成时计费

### 更多信息

完整的 Seedance Asset API 设计文档和使用说明，请参考：
- [Seedance Asset API 设计文档](./seedance-asset-design.md)
- [Seedance Asset API 简明文档](./seedance-asset-simple.md)
