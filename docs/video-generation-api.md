# 视频生成 API 接口文档

**鉴权**：所有接口需在 Header 中携带 API Key：
```
Authorization: Bearer <your_api_key>
```




---

## 1. 创建视频项目

**POST** `/api/video-generation/create`

### 请求体（JSON）

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `brand` | string | ✓ | 品牌名 |
| `product_name` | string | ✓ | 产品名 |
| `prompt` | string | ✓ | 创意描述 |
| `vtype` | string | ✓ | 视频类型 |
| `duration` | int | ✓ | 时长（秒），可选值：`15` `30` `45` `60` |
| `resolution` | string | ✓ | 分辨率：`720p` `1080p` `2k` `4k` |
| `whstr` | string | ✓ | 宽高比，如 `16:9` `9:16` |
| `video_model` | string | | 模型：`alpha-pro`（高质量）/ `alpha-flash`（快速） |
| `mediaList` | array | | 媒体列表，见下方说明 |
| `product_img_url` | string | | 产品图 URL（兼容旧格式，优先使用 mediaList） |
| `tagline` | string | | 广告语 |
| `selling_points` | string | | 卖点描述 |
| `language` | string | | 广告语言，如 `zh` `en` |
| `platform` | string | | 投放平台 |
| `region` | string | | 投放地区 |
| `vtype_add` | string | | 剧情子类型 |

### mediaList 元素结构

| 字段 | 类型 | 说明 |
|---|---|---|
| `mediaType` | string | `PRODUCT`（产品图）/ `ROLE`（角色）/ `OTHER` |
| `mediaUrl` | string | 媒体 URL |
| `roleName` | string | 角色名（mediaType=ROLE 时有效） |
| `sortOrder` | int | 排序 |

### 计费规则

- 基础费用 = `duration（秒）× 单秒单价（分辨率）`
- 创建时**预扣**，任务完成后按上游实际消耗**结算**（多退少补）

### 请求示例

```json
{
  "brand": "示例品牌",
  "product_name": "示例产品",
  "prompt": "展示产品在现代厨房中的使用场景",
  "vtype": "narrative",
  "duration": 30,
  "resolution": "1080p",
  "whstr": "16:9",
  "video_model": "alpha-pro",
  "mediaList": [
    {
      "mediaType": "PRODUCT",
      "mediaUrl": "https://example.com/product.jpg",
      "sortOrder": 1
    }
  ]
}
```

### 响应字段

| 字段 | 类型 | 说明 |
|---|---|---|
| `project_id` | int | 项目 ID |
| `project_name` | string | 项目名称 |
| `status` | string | 状态，初始为 `CREATED` |
| `created_at` | int | 创建时间（Unix 时间戳） |

### 响应示例

```json
{
  "code": 200,
  "msg": "video project created successfully",
  "data": {
    "project_id": 15,
    "project_name": "user_20260705_1751716800",
    "status": "CREATED",
    "created_at": 1751716800
  }
}
```

---

## 2. 查询视频项目

**GET** `/api/video-generation/projects/:id`

### 状态枚举

| status | 说明 |
|---|---|
| `CREATED` | 已创建，等待处理 |
| `RUNNING` | 生成中 |
| `VIDEO_PROCESSING` | 视频处理中 |
| `SUCCESS` | 完成 ✓ |
| `FAILED` | 失败 |

**轮询建议**：进行中状态每 10~30 秒查询一次，终态（`SUCCESS` / `FAILED`）停止轮询。

### 响应字段

| 字段 | 类型 | 说明 |
|---|---|---|
| `project_id` | int | 项目 ID |
| `project_name` | string | 项目名称 |
| `status` | string | 状态，见上方枚举 |
| `progress` | string | 进度（生成中时有值，如 `50%`） |
| `error_msg` | string | 失败原因（失败时有值） |
| `product_img_url` | string | 产品图 URL |
| `brand` | string | 品牌名 |
| `product_name` | string | 产品名 |
| `main_image_url` | string | 主图 URL（生成后有值） |
| `generated_result` | string | 生成结果（生成后有值） |
| `first_video_url` | string | 视频地址（完成后有值） |
| `created_at` | int | 创建时间（Unix 时间戳） |
| `updated_at` | int | 更新时间（Unix 时间戳） |

### 响应示例

```json
{
  "code": 200,
  "msg": "success",
  "data": {
    "project_id": 15,
    "project_name": "user_20260705_1751716800",
    "status": "SUCCESS",
    "progress": "100%",
    "product_img_url": "https://example.com/product.jpg",
    "brand": "示例品牌",
    "product_name": "示例产品",
    "main_image_url": "https://...",
    "first_video_url": "https://...",
    "created_at": 1751716800,
    "updated_at": 1751720000
  }
}
```

---

## 错误响应格式

```json
{ "code": 400, "msg": "错误说明", "data": null }
```

| code | 说明 |
|---|---|
| 400 | 参数错误 |
| 401 | 鉴权失败 |
| 404 | 项目不存在 |
| 429 | 超出请求频率限制 |
| 500 | 服务器内部错误 |
