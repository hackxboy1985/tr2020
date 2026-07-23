# 海报图像 API 渠道接入设计文档
* 内部使用，不对外公开

## 一、概述

将上游海报图像 API（`/openapi/v1/poster/*` 和 `/openapi/v1/ai/*`）接入 new-api relay 渠道体系。

- **鉴权**：`Authorization: Bearer <api_key>`，与广告视频上游相同
- **计费**：按次，每个接口对应一个模型名，后台配置模型单价
- **参数透传**：**所有参数（包括 query/prompt）均通过 `metadata` 字段传递**，与 kling/hailuo 渠道一致

---

## 二、渠道注册

| 常量 | 值 | 说明 |
|------|----|------|
| `ChannelTypePoster` | `58` | 新增渠道类型 |
| `APITypePoster` | `iota` | 新增 API 类型 |
| Base URL 默认值 | `http://your-host:9096` | 可在渠道配置中覆盖 |

---

## 三、接口分类与模型名

### 3.1 异步接口（TaskAdaptor）

走 `relay/channel/task/poster/`，实现 `TaskAdaptor` 接口。

| 模型名 | 上游接口 | 说明 |
|--------|---------|------|
| `poster-generate` | `POST /openapi/v1/poster/generateAsync` | 异步海报生成（主力接口） |
| `poster-free-creation` | `POST /openapi/v1/poster/allAroundCreation` | 自由创作异步 |

轮询接口：`GET /openapi/v1/poster/queryTaskResult?taskId={taskId}`（推荐，1.5）

> **旧轮询接口 1.4**（`GET /openapi/v1/poster/queryResult`）响应结构不同，暂不支持，如需接入再补充。

**客户端路由（新增）：**
- 提交：`POST /v1/images/tasks`
- 查询：`GET /v1/images/tasks/:task_id`

需新增 relay mode 常量 `RelayModeImageTaskSubmit` / `RelayModeImageTaskFetchByID`，并在 `relay-router.go` 注册路由，在 `relay/constant/relay_mode.go` 补充 `Path2RelayMode` 映射。

### 3.2 同步接口（Adaptor）

走 `relay/channel/poster/`，实现 `Adaptor` 接口，通过 `ImageHelper` 处理。

| 模型名 | 上游接口 | 需要文本 | 说明 |
|--------|---------|---------|------|
| `poster-generate-sync` | `POST /openapi/v1/poster/generate` | 是 | 同步海报生成（直接返回图片 URL） |
| `poster-extension` | `POST /openapi/v1/ai/extension` | 否 | 智能延展 |
| `poster-translate` | `POST /openapi/v1/ai/translate` | 否 | 图片翻译 |
| `poster-enlarge` | `POST /openapi/v1/ai/enlarge` | 否 | 无损放大 |
| `poster-matting` | `POST /openapi/v1/ai/matting` | 否 | AI 抠图 |
| `poster-enhance` | `POST /openapi/v1/ai/enhance` | 否 | AI 超清 |
| `poster-partial-redraw` | `POST /openapi/v1/ai/partialRedrawing` | 是 | 局部重绘 |
| `poster-scene-replace` | `POST /openapi/v1/ai/sceneReplace` | 是 | 场景替换 |
| `poster-product-replace` | `POST /openapi/v1/ai/productReplace` | 是 | 商品替换 |
| `poster-color-change` | `POST /openapi/v1/ai/colorChange` | 是 | 商品换色 |
| `poster-assisted` | `POST /openapi/v1/ai/assisted` | 是 | AI 帮写 |

---

## 四、参数透传详解

### 4.1 透传机制

**所有参数全部放在 `metadata` 中**，adaptor 通过 `req.UnmarshalMetadata(&upstreamReq)` 一次性反序列化：

```go
// adaptor 内部
var posterReq PosterGenerateRequest
if err := req.UnmarshalMetadata(&posterReq); err != nil {
    return nil, err
}
```

**兼容两种写法**（优先取 `metadata.query`，fallback 到外层 `prompt`）：

```go
// query 优先从 metadata 读，兼容外层 prompt
query := posterReq.Query
if query == "" {
    query = req.Prompt
}
```

| 写法 | 说明 |
|------|------|
| 全部放 `metadata`（推荐） | query/prompt 也放在 metadata 里，接口统一 |
| 外层 `prompt` + `metadata` 其余参数 | 也支持，兼容现有习惯 |

---

### 4.2 异步海报生成（poster-generate）

**用户请求（推荐写法）：**
```json
POST /v1/images/tasks
{
  "model": "poster-generate",
  "metadata": {
    "query": "一款高端护肤品，突出保湿效果，背景简洁白色",
    "generateType": 100,
    "posterType": 6,
    "platformType": "天猫",
    "languageType": "中文",
    "detailPictureNumber": 4,
    "modelEdition": 3,
    "needText": true,
    "aspectRatio": "1:1",
    "fileUrlList": ["https://example.com/product.jpg"]
  }
}
```

**透传到上游：**
```json
POST /openapi/v1/poster/generateAsync
Authorization: Bearer <api_key>
{
  "query": "一款高端护肤品，突出保湿效果，背景简洁白色",
  "generateType": 100,
  "posterType": 6,
  "platformType": "天猫",
  "languageType": "中文",
  "detailPictureNumber": 4,
  "modelEdition": 3,
  "needText": true,
  "aspectRatio": "1:1",
  "fileUrlList": ["https://example.com/product.jpg"]
}
```

**字段映射说明：**

| 客户端字段 | 上游字段 | 必填 | 说明 |
|-----------|---------|------|------|
| `metadata.query` | `query` | 是 | 需求描述 |
| `metadata.generateType` | `generateType` | 是 | 100=产品单图，200=产品详情图 |
| `metadata.posterType` | `posterType` | 是 | 5=跨境，6=中文电商 |
| `metadata.platformType` | `platformType` | 是 | 如 `Amazon`、`天猫` |
| `metadata.languageType` | `languageType` | 是 | 如 `英语`、`中文` |
| `metadata.detailPictureNumber` | `detailPictureNumber` | 是 | 1-6 或 1-15 |
| `metadata.modelEdition` | `modelEdition` | 是 | 2/3/9 |
| `metadata.needText` | `needText` | 是 | 是否带文案 |
| `metadata.aspectRatio` | `aspectRatio` | 否 | 如 `1:1`、`16:9` |
| `metadata.fileUrlList` | `fileUrlList` | 否 | 参考图片列表 |

**上游响应：**
```json
{
  "code": 200,
  "msg": "操作成功",
  "data": { "agentGenerateTaskId": "task_abc123456" }
}
```
→ 存入系统 task，返回客户端 task_id

**提交成功后返回给用户：**
```json
{
  "id": "local_task_abc123",
  "object": "image.task",
  "status": "processing",
  "model": "poster-generate",
  "created": 1720000000
}
```

**轮询：**
```
GET /v1/images/tasks/{task_id}
```
→ 系统轮询上游 `GET /openapi/v1/poster/queryTaskResult?taskId=task_abc123456`

上游轮询响应：
```json
{
  "code": 200,
  "data": {
    "taskList": [
      { "taskStatus": "SUCCESS", "executeResult": "https://oss.example.com/1.jpg" },
      { "taskStatus": "SUCCESS", "executeResult": "https://oss.example.com/2.jpg" }
    ]
  }
}
```
`taskStatus` 枚举：`RUNNING`、`SUCCESS`、`FAILED`

**查询成功返回给用户（进行中）：**
```json
{
  "id": "local_task_abc123",
  "object": "image.task",
  "status": "processing",
  "model": "poster-generate",
  "created": 1720000000
}
```

**查询成功返回给用户（完成）：**
```json
{
  "id": "local_task_abc123",
  "object": "image.task",
  "status": "succeeded",
  "model": "poster-generate",
  "created": 1720000000,
  "result": {
    "data": [
      { "url": "https://oss.example.com/1.jpg" },
      { "url": "https://oss.example.com/2.jpg" }
    ]
  }
}
```

**查询失败返回给用户：**
```json
{
  "id": "local_task_abc123",
  "object": "image.task",
  "status": "failed",
  "model": "poster-generate",
  "created": 1720000000,
  "error": {
    "message": "上游任务失败原因",
    "code": "task_failed"
  }
}
```

---

### 4.3 自由创作异步（poster-free-creation）

**用户请求：**
```json
POST /v1/images/tasks
{
  "model": "poster-free-creation",
  "metadata": {
    "query": "科技感十足的蓝色电子产品展示图，金属质感背景",
    "detailPictureNumber": 2,
    "aspectRatio": "16:9",
    "apiImgUrlList": ["https://example.com/ref1.jpg"]
  }
}
```

**透传到上游：**
```json
POST /openapi/v1/poster/allAroundCreation
{
  "query": "科技感十足的蓝色电子产品展示图，金属质感背景",
  "detailPictureNumber": 2,
  "aspectRatio": "16:9",
  "apiImgUrlList": ["https://example.com/ref1.jpg"]
}
```

**提交成功返回给用户：**（同 4.2，异步任务）
```json
{
  "id": "local_task_xyz789",
  "object": "image.task",
  "status": "processing",
  "model": "poster-free-creation",
  "created": 1720000000
}
```

**查询完成返回给用户：**
```json
{
  "id": "local_task_xyz789",
  "object": "image.task",
  "status": "succeeded",
  "model": "poster-free-creation",
  "created": 1720000000,
  "result": {
    "data": [
      { "url": "https://oss.example.com/free1.jpg" },
      { "url": "https://oss.example.com/free2.jpg" }
    ]
  }
}
```

---

### 4.4 智能延展（poster-extension）— 无文本

**用户请求：**
```json
POST /v1/images/generations
{
  "model": "poster-extension",
  "metadata": {
    "imgUrlList": [
      "https://example.com/image1.jpg",
      "https://example.com/image2.jpg"
    ],
    "ratio": "16:9"
  }
}
```

**透传到上游：**
```json
POST /openapi/v1/ai/extension
{
  "imgUrlList": ["https://example.com/image1.jpg", "https://example.com/image2.jpg"],
  "ratio": "16:9"
}
```

**返回给用户：**
```json
{
  "created": 1720000000,
  "data": [
    { "url": "https://oss.example.com/extended1.jpg" },
    { "url": "https://oss.example.com/extended2.jpg" }
  ]
}
```

| 客户端字段 | 上游字段 | 必填 | 说明 |
|-----------|---------|------|------|
| `metadata.imgUrlList` | `imgUrlList` | 是 | 最多6张 |
| `metadata.ratio` | `ratio` | 是 | 如 `1:1`、`16:9`、`9:16` |

---

### 4.5 图片翻译（poster-translate）— 无文本

**用户请求：**
```json
POST /v1/images/generations
{
  "model": "poster-translate",
  "metadata": {
    "imageUrl": "https://example.com/banner_cn.jpg",
    "to": 1,
    "from": "auto"
  }
}
```

**透传到上游：**
```json
POST /openapi/v1/ai/translate
{
  "imageUrl": "https://example.com/banner_cn.jpg",
  "to": 1,
  "from": "auto"
}
```

**返回给用户：**
```json
{
  "created": 1720000000,
  "data": [
    { "url": "https://oss.example.com/banner_en.jpg" }
  ]
}
```

`to` 语言编号：0=中文，1=英文，2=俄语，3=西班牙语，4=法语，5=德语，6=意大利语，7=荷兰语，8=葡萄牙语，9=越南语，10=土耳其语，11=马来语，12=泰语，13=波兰语，14=印尼语，15=日语，16=韩语，17=繁体中文

---

### 4.6 无损放大（poster-enlarge）— 无文本

**用户请求：**
```json
POST /v1/images/generations
{
  "model": "poster-enlarge",
  "metadata": {
    "imgUrls": "https://example.com/img1.jpg,https://example.com/img2.jpg",
    "scalingRatio": 2
  }
}
```

**透传到上游：**
```json
POST /openapi/v1/ai/enlarge
{
  "imgUrls": "https://example.com/img1.jpg,https://example.com/img2.jpg",
  "scalingRatio": 2
}
```

**返回给用户：**
```json
{
  "created": 1720000000,
  "data": [
    { "url": "https://oss.example.com/enlarged1.jpg" },
    { "url": "https://oss.example.com/enlarged2.jpg" }
  ]
}
```

`scalingRatio`：1=轻度，2=标准，3=强力

---

### 4.7 AI 抠图（poster-matting）— 无文本

**用户请求：**
```json
POST /v1/images/generations
{
  "model": "poster-matting",
  "metadata": {
    "imgUrls": "https://example.com/product.jpg"
  }
}
```

**透传到上游：**
```json
POST /openapi/v1/ai/matting
{
  "imgUrls": "https://example.com/product.jpg"
}
```

**返回给用户：**
```json
{
  "created": 1720000000,
  "data": [
    { "url": "https://oss.example.com/matted.png" }
  ]
}
```

---

### 4.8 AI 超清（poster-enhance）— 无文本

**用户请求：**
```json
POST /v1/images/generations
{
  "model": "poster-enhance",
  "metadata": {
    "imgUrls": "https://example.com/blurry.jpg",
    "enhanceStrength": "standard"
  }
}
```

**透传到上游：**
```json
POST /openapi/v1/ai/enhance
{
  "imgUrls": "https://example.com/blurry.jpg",
  "enhanceStrength": "standard"
}
```

**返回给用户：**
```json
{
  "created": 1720000000,
  "data": [
    { "url": "https://oss.example.com/enhanced.jpg" }
  ]
}
```

`enhanceStrength`：`light`=轻度，`standard`=标准，`strong`=强力

---

### 4.9 局部重绘（poster-partial-redraw）

**用户请求：**
```json
POST /v1/images/generations
{
  "model": "poster-partial-redraw",
  "metadata": {
    "sourceUrl": "https://example.com/product.jpg",
    "textPrompt": "将背景替换为森林场景，保持产品不变",
    "replaceImageUrl": "https://example.com/forest_ref.jpg"
  }
}
```

**透传到上游：**
```json
POST /openapi/v1/ai/partialRedrawing
{
  "sourceUrl": "https://example.com/product.jpg",
  "textPrompt": "将背景替换为森林场景，保持产品不变",
  "replaceImageUrl": "https://example.com/forest_ref.jpg"
}
```

**返回给用户：**
```json
{
  "created": 1720000000,
  "data": [
    { "url": "https://oss.example.com/redrawn.jpg" }
  ]
}
```

| 客户端字段 | 上游字段 | 必填 |
|-----------|---------|------|
| `metadata.sourceUrl` | `sourceUrl` | 是 |
| `metadata.textPrompt` | `textPrompt` | 是 |
| `metadata.replaceImageUrl` | `replaceImageUrl` | 否 |

---

### 4.10 场景替换（poster-scene-replace）

**用户请求：**
```json
POST /v1/images/generations
{
  "model": "poster-scene-replace",
  "metadata": {
    "sourceUrl": "https://example.com/product.jpg",
    "replaceImageUrl": "https://example.com/beach_scene.jpg",
    "textPrompt": "将背景替换为海滩场景",
    "modelType": 0
  }
}
```

**透传到上游：**
```json
POST /openapi/v1/ai/sceneReplace
{
  "sourceUrl": "https://example.com/product.jpg",
  "replaceImageUrl": "https://example.com/beach_scene.jpg",
  "textPrompt": "将背景替换为海滩场景",
  "modelType": 0
}
```

**返回给用户：**
```json
{
  "created": 1720000000,
  "data": [
    { "url": "https://oss.example.com/scene_replaced.jpg" }
  ]
}
```

`modelType`：0=gemini-2.5（默认），1=gemini-3-pro

---

### 4.11 商品替换（poster-product-replace）

**用户请求：**
```json
POST /v1/images/generations
{
  "model": "poster-product-replace",
  "metadata": {
    "sourceUrl": "https://example.com/scene_with_old_product.jpg",
    "replaceImageUrl": "https://example.com/new_product.jpg",
    "textPrompt": "将场景中的商品替换为新商品，保持场景一致",
    "modelType": 0
  }
}
```

**透传到上游：**
```json
POST /openapi/v1/ai/productReplace
{
  "sourceUrl": "https://example.com/scene_with_old_product.jpg",
  "replaceImageUrl": "https://example.com/new_product.jpg",
  "textPrompt": "将场景中的商品替换为新商品，保持场景一致",
  "modelType": 0
}
```

**返回给用户：**
```json
{
  "created": 1720000000,
  "data": [
    { "url": "https://oss.example.com/product_replaced.jpg" }
  ]
}
```

---

### 4.12 商品换色（poster-color-change）

**用户请求：**
```json
POST /v1/images/generations
{
  "model": "poster-color-change",
  "metadata": {
    "sourceUrl": "https://example.com/bag_blue.jpg",
    "textPrompt": "将包包颜色换成玫瑰红色",
    "modelType": 0
  }
}
```

**透传到上游：**
```json
POST /openapi/v1/ai/colorChange
{
  "sourceUrl": "https://example.com/bag_blue.jpg",
  "textPrompt": "将包包颜色换成玫瑰红色",
  "modelType": 0
}
```

**返回给用户：**
```json
{
  "created": 1720000000,
  "data": [
    { "url": "https://oss.example.com/bag_red.jpg" }
  ]
}
```

---

### 4.13 AI 帮写（poster-assisted）

**用户请求：**
```json
POST /v1/images/generations
{
  "model": "poster-assisted",
  "metadata": {
    "query": "为一款保湿面霜生成亚马逊产品描述文案，突出天然成分和长效保湿",
    "fileUrlList": ["https://example.com/cream.jpg"],
    "generateType": "image"
  }
}
```

**透传到上游：**
```json
POST /openapi/v1/ai/assisted
{
  "query": "为一款保湿面霜生成亚马逊产品描述文案，突出天然成分和长效保湿",
  "fileUrlList": ["https://example.com/cream.jpg"],
  "generateType": "image"
}
```

上游返回（文案，非图片）：
```json
{
  "code": 200,
  "data": {
    "options": [
      "24-Hour Deep Moisture Cream | Natural Ingredients | Dermatologist Tested",
      "Intense Hydration Face Cream with Plant Extract | All Day Moisture Lock"
    ]
  }
}
```

**返回给用户（文案放入 `revised_prompt`，`url` 为空）：**
```json
{
  "created": 1720000000,
  "data": [
    { "url": "", "revised_prompt": "24-Hour Deep Moisture Cream | Natural Ingredients | Dermatologist Tested" },
    { "url": "", "revised_prompt": "Intense Hydration Face Cream with Plant Extract | All Day Moisture Lock" }
  ]
}
```

---

## 五、响应格式统一

### 5.1 同步图片接口（`POST /v1/images/generations`）

上游返回 `{"code":200,"data":"url1,url2"}` 或 `{"code":200,"data":"url"}` 时，统一转为 OpenAI image 格式：

**正常返回：**
```json
{
  "created": 1720000000,
  "data": [
    { "url": "https://oss.example.com/result1.jpg" },
    { "url": "https://oss.example.com/result2.jpg" }
  ]
}
```

**上游报错返回：**
```json
{
  "error": {
    "message": "未配置 secretKey，请在渠道配置中添加",
    "type": "upstream_error",
    "code": 500
  }
}
```

**特殊：poster-assisted 返回文本（非图片）：**
```json
{
  "created": 1720000000,
  "data": [
    { "url": "", "revised_prompt": "24-Hour Deep Moisture Cream | Natural Ingredients | Dermatologist Tested" },
    { "url": "", "revised_prompt": "Intense Hydration Face Cream with Plant Extract | All Day Moisture Lock" }
  ]
}
```
> 文案选项放入 `revised_prompt` 字段，`url` 为空字符串

### 5.2 异步接口（`POST /v1/images/tasks` / `GET /v1/images/tasks/:task_id`）

见 4.2 节中的完整响应示例。状态枚举：

| status | 说明 |
|--------|------|
| `processing` | 任务进行中 |
| `succeeded` | 任务成功，`result.data` 中包含图片列表 |
| `failed` | 任务失败，`error.message` 中包含原因 |

---

## 六、需要新增/修改的文件

### 新增文件

```
relay/channel/task/poster/
    adaptor.go      ← TaskAdaptor 实现（异步海报）
    constants.go    ← 模型列表、端点常量

relay/channel/poster/
    adaptor.go      ← Adaptor 实现（同步 AI 工具）
    dto.go          ← 上游请求/响应结构体
```

### 修改文件

```
constant/channel.go                  ← 新增 ChannelTypePoster = 58，补 ChannelBaseURLs[58] 和 ChannelTypeNames
constant/api_type.go                 ← 新增 APITypePoster
relay/constant/relay_mode.go         ← 新增 RelayModeImageTaskSubmit、RelayModeImageTaskFetchByID 常量
relay/relay_adaptor.go               ← GetAdaptor 和 GetTaskAdaptor 注册 poster
router/relay-router.go               ← 新增 POST /v1/images/tasks 和 GET /v1/images/tasks/:task_id 路由
```

### 路由新增详情

```go
// relay-router.go 新增（image task 异步路由）
httpRouter.POST("/images/tasks", controller.RelayTask)
httpRouter.GET("/images/tasks/:task_id", controller.RelayTaskFetch)
```

```go
// relay_mode.go 新增常量
RelayModeImageTaskSubmit
RelayModeImageTaskFetchByID

// Path2RelayMode 新增映射
} else if strings.HasPrefix(path, "/v1/images/tasks") && method == http.MethodGet {
    relayMode = RelayModeImageTaskFetchByID
} else if strings.HasPrefix(path, "/v1/images/tasks") {
    relayMode = RelayModeImageTaskSubmit
}
```

---

## 七、计费配置

后台「模型价格」中按模型名配置单价，按次扣费：

| 模型名 | 需要文本 | 说明 |
|--------|---------|------|
| `poster-generate` | 是 | 异步 |
| `poster-free-creation` | 是 | 异步 |
| `poster-generate-sync` | 是 | 同步，直接返回图片 URL |
| `poster-extension` | 否 | 同步，最多6张 |
| `poster-translate` | 否 | 同步 |
| `poster-enlarge` | 否 | 同步，最多6张 |
| `poster-matting` | 否 | 同步，最多6张 |
| `poster-enhance` | 否 | 同步，最多6张 |
| `poster-partial-redraw` | 是 | 同步 |
| `poster-scene-replace` | 是 | 同步 |
| `poster-product-replace` | 是 | 同步 |
| `poster-color-change` | 是 | 同步 |
| `poster-assisted` | 是 | 同步，返回文本 |

---

## 八、上游路径可配置化（已实现）

通过渠道「其他配置（Other）」字段的 JSON 覆盖上游路径，无需重启服务。

### 8.1 字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `poster_api_version` | string | 批量替换版本号，如 `"v2"` 将所有 `/v1/` 改为 `/v2/` |
| `poster_endpoints` | map | 按模型精确覆盖完整路径，优先级高于 `poster_api_version` |

### 8.2 配置示例

**场景一：只改版本号（全部接口批量生效）**
```json
{
  "poster_api_version": "v2"
}
```

**场景二：精确覆盖某个模型路径**
```json
{
  "poster_endpoints": {
    "poster-matting": "/openapi/v2/ai/matting_pro",
    "poster-generate": "/openapi/v2/poster/generateAsync"
  }
}
```

**场景三：版本批量改 + 个别路径单独覆盖**
```json
{
  "poster_api_version": "v2",
  "poster_endpoints": {
    "poster-matting": "/openapi/v2/ai/matting_pro",
    "poster-query": "/openapi/v2/poster/queryTaskResult"
  }
}
```

### 8.3 所有可覆盖的 key 及默认路径

| key | 默认路径 |
|-----|---------|
| `poster-extension` | `/openapi/v1/ai/extension` |
| `poster-translate` | `/openapi/v1/ai/translate` |
| `poster-enlarge` | `/openapi/v1/ai/enlarge` |
| `poster-matting` | `/openapi/v1/ai/matting` |
| `poster-enhance` | `/openapi/v1/ai/enhance` |
| `poster-partial-redraw` | `/openapi/v1/ai/partialRedrawing` |
| `poster-scene-replace` | `/openapi/v1/ai/sceneReplace` |
| `poster-product-replace` | `/openapi/v1/ai/productReplace` |
| `poster-color-change` | `/openapi/v1/ai/colorChange` |
| `poster-assisted` | `/openapi/v1/ai/assisted` |
| `poster-generate` | `/openapi/v1/poster/generateAsync` |
| `poster-free-creation` | `/openapi/v1/poster/allAroundCreation` |
| `poster-query` | `/openapi/v1/poster/queryTaskResult`（轮询接口） |

> 优先级：`poster_endpoints[model]` > `poster_api_version` 版本替换 > 默认路径

---

## 九、待确认问题

1. **同步接口路由**：走 `/v1/images/generations` ✅ 已确认
2. **poster-assisted 返回文本**：文案放入 `revised_prompt`，`url` 为空 ✅ 已实现
3. **多图计费**：`detailPictureNumber` 影响实际生成张数，是否按张数乘以单价？（待确认）
