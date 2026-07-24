# OpenAPI 上游 图像处理接口文档 v1

**Base URL**: `http://your-host:9096`  
**鉴权**: `Authorization: Bearer <token>`  
**Content-Type**: `application/json`

---

## 一、海报生成

### 1.1 同步生成海报

`POST /openapi/v1/poster/generate`

**请求示例**：
```json
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

**响应示例**：
```json
{
  "code": 200,
  "msg": "操作成功",
  "data": "https://oss.example.com/result1.jpg,https://oss.example.com/result2.jpg"
}
```

**字段说明**：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| query | string | 是 | 需求描述 |
| generateType | int | 是 | 100-产品单图，200-产品详情图 |
| posterType | int | 是 | 5-跨境电商，6-中文电商 |
| platformType | string | 是 | 平台类型，如 `Amazon`、`天猫` |
| languageType | string | 是 | 语种，如 `英语`、`中文` |
| detailPictureNumber | int | 是 | 图片数量（1-6 或 1-15） |
| modelEdition | int | 是 | 2-v2.0，3-v3.0，9-Image 2 |
| needText | boolean | 是 | 是否带文案 |
| fileUrlList | array | 否 | 参考图片URL列表 |
| aspectRatio | string | 否 | 比例，如 `1:1`、`16:9` |
| userId | long | 否 | 不传则取当前登录用户 |

---

### 1.2 异步生成海报

`POST /openapi/v1/poster/generateAsync`

**请求示例**：
```json
{
  "query": "运动鞋产品主图，背景为户外运动场景",
  "generateType": 100,
  "posterType": 5,
  "platformType": "Amazon",
  "languageType": "英语",
  "detailPictureNumber": 3,
  "modelEdition": 3,
  "needText": false,
  "aspectRatio": "1:1",
  "fileUrlList": ["https://example.com/shoe.jpg"]
}
```

**响应示例**：
```json
{
  "code": 200,
  "msg": "操作成功",
  "data": "2080593662147178497"
}
```

> `data` 为任务 ID 字符串，通过接口 1.5 轮询结果

---

### 1.3 自由创作异步

`POST /openapi/v1/poster/allAroundCreation`

**请求示例**：
```json
{
  "query": "科技感十足的蓝色电子产品展示图，金属质感背景",
  "detailPictureNumber": 2,
  "aspectRatio": "16:9",
  "apiImgUrlList": ["https://example.com/ref1.jpg"]
}
```

**响应示例**：
```json
{
  "code": 200,
  "msg": "操作成功",
  "data": "2080593662147178498"
}
```

---

### 1.4 查询任务结果（旧）

`GET /openapi/v1/poster/queryResult?taskId=task_abc123456`

**响应示例**：
```json
{
  "code": 200,
  "msg": "操作成功",
  "data": [
    {
      "taskStatus": "SUCCESS",
      "executeResult": "https://oss.example.com/output1.jpg,https://oss.example.com/output2.jpg"
    }
  ]
}
```

`taskStatus` 枚举：`RUNNING`/`1`-进行中，`SUCCESS`/`2`-成功，`FAILED`/`3`-失败（实际返回数字）

---

### 1.5 查询任务结果（推荐）

`GET /openapi/v1/poster/queryTaskResult?taskId=task_abc123456`

**响应示例**：
```json
{
  "code": 200,
  "msg": "操作成功",
  "data": {
    "taskList": [
      {
        "taskStatus": "SUCCESS",
        "executeResult": "https://oss.example.com/output1.jpg"
      },
      {
        "taskStatus": "SUCCESS",
        "executeResult": "https://oss.example.com/output2.jpg"
      }
    ]
  }
}
```

---

## 二、AI 工具

### 2.1 智能延展

`POST /openapi/v1/ai/extension`

**请求示例**：
```json
{
  "imgUrlList": [
    "https://example.com/image1.jpg",
    "https://example.com/image2.jpg"
  ],
  "ratio": "16:9"
}
```

**响应示例**：
```json
{
  "code": 200,
  "msg": "操作成功",
  "data": [
    "https://oss.example.com/extended1.jpg",
    "https://oss.example.com/extended2.jpg"
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| imgUrlList | array | 是 | 源图片列表（最多6张） |
| ratio | string | 是 | 目标比例，如 `1:1`、`16:9`、`9:16` |

---

### 2.2 图片翻译

`POST /openapi/v1/ai/translate`

**请求示例**：
```json
{
  "imageUrl": "https://example.com/banner_cn.jpg",
  "to": 1,
  "from": "auto"
}
```

**响应示例**：
```json
{
  "code": 200,
  "msg": "操作成功",
  "data": "https://oss.example.com/banner_en.jpg"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| imageUrl | string | 是 | 待翻译图片链接 |
| to | int | 是 | 目标语言：0-中文，1-英文，2-俄语，3-西班牙语，4-法语，5-德语，6-意大利语，7-荷兰语，8-葡萄牙语，9-越南语，10-土耳其语，11-马来语，12-泰语，13-波兰语，14-印尼语，15-日语，16-韩语，17-繁体中文 |
| from | string | 否 | 原语言，默认 `auto` |

---

### 2.3 局部重绘

`POST /openapi/v1/ai/partialRedrawing`

**请求示例**：
```json
{
  "sourceUrl": "https://example.com/product.jpg",
  "textPrompt": "将背景替换为森林场景，保持产品不变",
  "replaceImageUrl": "https://example.com/forest_ref.jpg"
}
```

**响应示例**：
```json
{
  "code": 200,
  "msg": "操作成功",
  "data": "https://oss.example.com/redrawn.jpg"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| sourceUrl | string | 是 | 原图链接 |
| textPrompt | string | 是 | 重绘提示词 |
| replaceImageUrl | string | 否 | 参考替换图片 |

---

### 2.4 无损放大

`POST /openapi/v1/ai/enlarge`

**请求示例**：
```json
{
  "imgUrls": "https://example.com/img1.jpg,https://example.com/img2.jpg",
  "scalingRatio": 2
}
```

**响应示例**：
```json
{
  "code": 200,
  "msg": "操作成功",
  "data": "https://oss.example.com/enlarged1.jpg,https://oss.example.com/enlarged2.jpg"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| imgUrls | string | 是 | 图片链接，多张逗号分隔（最多6张） |
| scalingRatio | int | 是 | 1-轻度，2-标准，3-强力 |

---

### 2.5 AI 抠图

`POST /openapi/v1/ai/matting`

**请求示例**：
```json
{
  "imgUrls": "https://example.com/product.jpg"
}
```

**响应示例**：
```json
{
  "code": 200,
  "msg": "操作成功",
  "data": "https://oss.example.com/matted.png"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| imgUrls | string | 是 | 图片链接，多张逗号分隔（最多6张） |

---

### 2.6 场景替换

`POST /openapi/v1/ai/sceneReplace`

**请求示例**：
```json
{
  "sourceUrl": "https://example.com/product.jpg",
  "replaceImageUrl": "https://example.com/beach_scene.jpg",
  "textPrompt": "将背景替换为海滩场景",
  "modelType": 0
}
```

**响应示例**：
```json
{
  "code": 200,
  "msg": "操作成功",
  "data": "https://oss.example.com/scene_replaced.jpg"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| sourceUrl | string | 是 | 原图链接 |
| replaceImageUrl | string | 是 | 场景参考图链接 |
| textPrompt | string | 是 | 场景描述提示词 |
| modelType | int | 否 | 0-gemini-2.5，1-gemini-3-pro，默认0 |

---

### 2.7 商品替换

`POST /openapi/v1/ai/productReplace`

**请求示例**：
```json
{
  "sourceUrl": "https://example.com/scene_with_old_product.jpg",
  "replaceImageUrl": "https://example.com/new_product.jpg",
  "textPrompt": "将场景中的商品替换为新商品，保持场景一致",
  "modelType": 0
}
```

**响应示例**：
```json
{
  "code": 200,
  "msg": "操作成功",
  "data": "https://oss.example.com/product_replaced.jpg"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| sourceUrl | string | 是 | 原图链接（含旧商品的场景图） |
| replaceImageUrl | string | 是 | 目标商品图链接 |
| textPrompt | string | 是 | 替换描述提示词 |
| modelType | int | 否 | 0-gemini-2.5，1-gemini-3-pro，默认0 |

---

### 2.8 商品换色

`POST /openapi/v1/ai/colorChange`

**请求示例**：
```json
{
  "sourceUrl": "https://example.com/bag_blue.jpg",
  "textPrompt": "将包包颜色换成玫瑰红色",
  "modelType": 0
}
```

**响应示例**：
```json
{
  "code": 200,
  "msg": "操作成功",
  "data": "https://oss.example.com/bag_red.jpg"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| sourceUrl | string | 是 | 原图链接 |
| textPrompt | string | 是 | 换色描述，如 `换成红色` |
| modelType | int | 否 | 0-gemini-2.5，1-gemini-3-pro，默认0 |

---

### 2.9 AI 超清

`POST /openapi/v1/ai/enhance`

**请求示例**：
```json
{
  "imgUrls": "https://example.com/blurry.jpg",
  "enhanceStrength": "standard"
}
```

**响应示例**：
```json
{
  "code": 200,
  "msg": "操作成功",
  "data": "https://oss.example.com/enhanced.jpg"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| imgUrls | string | 是 | 图片链接，多张逗号分隔（最多6张） |
| enhanceStrength | string | 是 | `light`-轻度，`standard`-标准，`strong`-强力 |

---

### 2.10 AI 帮写

`POST /openapi/v1/ai/assisted`

**请求示例**：
```json
{
  "query": "为一款保湿面霜生成亚马逊产品描述文案，突出天然成分和长效保湿",
  "fileUrlList": ["https://example.com/cream.jpg"],
  "generateType": "image"
}
```

**响应示例**：
```json
{
  "code": 200,
  "msg": "操作成功",
  "data": {
    "options": [
      "24-Hour Deep Moisture Cream | Natural Ingredients | Dermatologist Tested",
      "Intense Hydration Face Cream with Plant Extract | All Day Moisture Lock",
      "Natural Moisturizing Cream | Clinically Proven 24hr Hydration | Sensitive Skin Safe"
    ]
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| query | string | 是 | 需求描述（最多1000字符） |
| fileUrlList | array | 否 | 参考图片列表（最多6张） |
| generateType | string | 否 | `image`-图片文案，`video`-视频文案 |

---


## 三、统一返回格式

```json
{
  "code": 200,
  "msg": "操作成功",
  "data": { ... }
}
```

**失败响应示例**：
```json
{
  "code": 500,
  "msg": "未配置 secretKey，请在渠道配置中添加",
  "data": null
}
```

| code | 说明 |
|------|------|
| 200 | 成功 |
| 401 | 未授权，Token 无效或过期 |
| 500 | 业务错误，见 msg 字段 |
