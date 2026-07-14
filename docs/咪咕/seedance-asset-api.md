# 给用户 上游 Seedance 资产库接口文档

## 概述

资产库用于管理 Seedance 视频生成所需的素材（图片、视频、音频）和真人人脸认证。素材上传并处理完成后，可在视频生成接口中通过 `asset://<asset_id>` 引用。

## 鉴权

所有接口均需携带 NewAPI 令牌：

```http
Authorization: Bearer sk-<API_TOKEN>
Content-Type: application/json
```

## 服务地址

```text
https://<your-newapi-host>
```

---

## 一、素材组

素材组用于归类管理素材。有两种类型：
- `AIGC`：数字人/虚拟形象素材，无需人脸认证
- `LivenessFace`：真人人像素材，需先完成人脸认证

### 1.1 创建素材组

```http
POST /api/seedance/asset-groups
```

**请求体：**

```json
{
  "Name": "my-assets",
  "Description": "素材组描述",
  "GroupType": "AIGC"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `Name` | string | 是 | 素材组名称 |
| `Description` | string | 否 | 描述 |
| `GroupType` | string | 是 | `AIGC` 或 `LivenessFace` |

**成功响应（透传上游）：**

```json
{
  "Result": {
    "Id": "group-xxxxxxxx",
    "Name": "my-assets",
    "GroupType": "AIGC"
  }
}
```

保存 `Result.Id` 用于后续创建素材。

---

### 1.2 查询素材组列表

```http
GET /api/seedance/asset-groups?PageNumber=1&PageSize=20
```

查询真人素材组时加参数：`GroupType=LivenessFace`

---

### 1.3 查询单个素材组

```http
GET /api/seedance/asset-groups/:id
```

`:id` 为本地表主键 ID（创建时由 NewAPI 分配）。

---

### 1.4 更新素材组

```http
PUT   /api/seedance/asset-groups/:id
PATCH /api/seedance/asset-groups/:id
```

---

### 1.5 删除素材组

```http
DELETE /api/seedance/asset-groups/:id
```

软删除，数据保留在本地表中。

---

## 二、素材

### 2.1 创建素材

```http
POST /api/seedance/assets
```

**请求体：**

```json
{
  "GroupId": "group-xxxxxxxx",
  "URL": "https://example.com/image.jpg",
  "AssetType": "Image",
  "Name": "my-image"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `GroupId` | string | 否 | 素材组 ID，不填则自动使用默认组 |
| `URL` | string | 是 | 公网可访问的 HTTPS 文件地址 |
| `AssetType` | string | 是 | `Image`、`Video` 或 `Audio` |
| `Name` | string | 否 | 素材名称 |
| `Force` | bool | 否 | `true` 时强制重新上传，忽略本地缓存；默认 `false` |

> **幂等性**：相同 URL 已有 `Active` 状态的素材时，直接返回已有素材，不重复上传。设置 `Force: true` 可强制重新上传。

**成功响应：**

```json
{
  "Result": {
    "Id": "asset-xxxxxxxx",
    "Status": "Processing"
  }
}
```

保存 `Result.Id`，状态为 `Processing` 表示上游正在处理，需继续轮询。

---

### 2.2 查询素材列表

```http
GET /api/seedance/assets?PageNumber=1&PageSize=20
```

---

### 2.3 查询素材状态

```http
GET /api/seedance/assets/:id
```

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
| `Processing` | 上游正在预处理 |
| `Active` | 可用，可通过 `asset://<asset_id>` 引用 |
| `Failed` | 处理失败 |

状态变为 `Active` 后，才可在视频生成中使用。

---

### 2.4 更新素材

```http
PUT   /api/seedance/assets/:id
PATCH /api/seedance/assets/:id
```

---

### 2.5 删除素材

```http
DELETE /api/seedance/assets/:id
```

---

## 三、人脸认证

真人人像素材需先完成 H5 活体认证，获得 `group_id` 后再上传素材。

### 3.1 创建认证任务

```http
POST /api/seedance/face-verifications
```

**请求体：**

```json
{
  "return_url": "https://your-app.com/face-result"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `return_url` | string | 否 | 认证完成后跳转的页面地址 |

**成功响应：**

```json
{
  "verification_id": "fv_xxxxxxxxx",
  "status": "waiting_user",
  "h5_url": "https://...",
  "expires_at": 1783740000
}
```

将 `h5_url` 展示给终端用户，引导用户完成活体认证。

---

### 3.2 查询认证任务状态

```http
GET /api/seedance/face-verifications/:id
```

**响应示例（认证成功）：**

```json
{
  "verification_id": "fv_xxxxxxxxx",
  "status": "verified",
  "group_id": "group-xxxxxxxx",
  "expires_at": 1783740000
}
```

| 状态 | 说明 |
|------|------|
| `waiting_user` | 等待用户完成 H5 认证 |
| `callback_received` | 已收到认证回调 |
| `resolving` | 正在获取结果 |
| `verified` | 认证成功，可读取 `group_id` |
| `failed` | 认证失败 |
| `expired` | 认证任务已过期，需重新创建 |

认证成功后，使用返回的 `group_id` 向该组上传本人素材（`AssetType` 可为 `Image`、`Video` 或 `Audio`）。

---

## 四、完整调用流程

### 数字人/AIGC 素材流程

```
1. POST /api/seedance/asset-groups        创建素材组（GroupType=AIGC）
2. POST /api/seedance/assets              上传素材
3. GET  /api/seedance/assets/:id          轮询，等待 Status=Active
4. POST /v1/video/generations             使用 asset://<asset_id> 生成视频
5. GET  /v1/videos/:task_id               轮询视频任务
```

### 真人人像素材流程

```
1. POST /api/seedance/face-verifications  创建认证任务
2. 引导用户打开 h5_url 完成活体认证
3. GET  /api/seedance/face-verifications/:id  轮询，等待 status=verified，获取 group_id
4. POST /api/seedance/assets              向该 group_id 上传本人素材
5. GET  /api/seedance/assets/:id          轮询，等待 Status=Active
6. POST /v1/video/generations             使用 asset://<asset_id> 生成视频
```

---

## 五、在视频生成中引用素材

素材状态变为 `Active` 后，在视频生成接口的 `images` 字段中使用：

```json
{
  "model": "doubao-seedance-2-0-260128",
  "prompt": "人物自然介绍产品",
  "seconds": "5",
  "images": ["asset://asset-xxxxxxxx"],
  "metadata": {
    "resolution": "720p",
    "ratio": "16:9"
  }
}
```

---

## 六、错误码

| HTTP 状态码 | 说明 |
|------------|------|
| `401` | Token 缺失或无效 |
| `404` | 资源不存在（本地表中未找到） |
| `503` | 未找到可用的 Seedance Gateway 渠道 |
| `502` | 上游请求失败 |
