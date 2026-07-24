# 海报图像 API 使用指南

## 一、接入说明

- **接口地址**：由服务提供方提供
- **鉴权方式**：`Authorization: Bearer <your_token>`
- **所有参数**：通过 `metadata` 字段传递
- **Content-Type**：`application/json`

---

## 二、模型列表

### 同步接口（`POST /v1/images/generations`）

直接返回结果，无需轮询。

| 模型名 | 功能 | 必填参数 |
|--------|------|---------|
| `poster-matting` | AI 抠图 | `imgUrls` |
| `poster-enlarge` | 无损放大 | `imgUrls` |
| `poster-enhance` | AI 超清 | `imgUrls` |
| `poster-extension` | 智能延展 | `imgUrlList`、`ratio` |
| `poster-translate` | 图片翻译 | `imageUrl`、`to` |
| `poster-partial-redraw` | 局部重绘 | `sourceUrl`、`textPrompt` |
| `poster-scene-replace` | 场景替换 | `sourceUrl`、`replaceImageUrl`、`textPrompt` |
| `poster-product-replace` | 商品替换 | `sourceUrl`、`replaceImageUrl`、`textPrompt` |
| `poster-color-change` | 商品换色 | `sourceUrl`、`textPrompt` |
| `poster-assisted` | AI 帮写 | `query` |
| `poster-generate-sync` | 同步海报生成 | `query` |

### 异步接口（`POST /v1/images/tasks`）

提交后返回 `task_id`，需要轮询 `GET /v1/images/tasks/{task_id}` 获取结果。

| 模型名 | 功能 | 必填参数 |
|--------|------|---------|
| `poster-generate` | 异步海报生成 | `query` |
| `poster-free-creation` | 自由创作 | `query` |

---

## 三、接口详细说明

### 3.1 AI 抠图（poster-matting）

去除图片背景，保留主体。

**请求：**
```bash
curl -X POST https://your-gateway/v1/images/generations \
  -H "Authorization: Bearer sk-xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "poster-matting",
    "metadata": {
      "imgUrls": "https://example.com/product.jpg"
    }
  }'
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `imgUrls` | string | 是 | 图片 URL，多张逗号分隔，最多 6 张 |

**返回：**
```json
{
  "created": 1720000000,
  "data": [
    { "url": "https://oss.example.com/matted.png" }
  ]
}
```

---

### 3.2 无损放大（poster-enlarge）

图片放大 1-3 倍，保持清晰度。

**请求：**
```bash
curl -X POST https://your-gateway/v1/images/generations \
  -H "Authorization: Bearer sk-xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "poster-enlarge",
    "metadata": {
      "imgUrls": "https://example.com/product.jpg",
      "scalingRatio": 2
    }
  }'
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `imgUrls` | string | 是 | 图片 URL，多张逗号分隔，最多 6 张 |
| `scalingRatio` | int | 否 | 1=轻度，2=标准，3=强力（默认 2） |

---

### 3.3 AI 超清（poster-enhance）

提升模糊图片的分辨率和清晰度。

**请求：**
```bash
curl -X POST https://your-gateway/v1/images/generations \
  -H "Authorization: Bearer sk-xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "poster-enhance",
    "metadata": {
      "imgUrls": "https://example.com/blurry.jpg",
      "enhanceStrength": "standard"
    }
  }'
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `imgUrls` | string | 是 | 图片 URL，多张逗号分隔，最多 6 张 |
| `enhanceStrength` | string | 否 | `light`=轻度，`standard`=标准，`strong`=强力 |

---

### 3.4 智能延展（poster-extension）

按目标比例扩展图片画面，AI 补全边缘内容。

**请求：**
```bash
curl -X POST https://your-gateway/v1/images/generations \
  -H "Authorization: Bearer sk-xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "poster-extension",
    "metadata": {
      "imgUrlList": ["https://example.com/banner.jpg"],
      "ratio": "16:9"
    }
  }'
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `imgUrlList` | array | 是 | 图片 URL 数组，最多 6 张 |
| `ratio` | string | 是 | 目标比例，如 `1:1`、`16:9`、`9:16` |

**返回：**
```json
{
  "created": 1720000000,
  "data": [
    { "url": "https://oss.example.com/extended.jpg" }
  ]
}
```

---

### 3.5 图片翻译（poster-translate）

识别图片中的文字并翻译成目标语言。

**请求：**
```bash
curl -X POST https://your-gateway/v1/images/generations \
  -H "Authorization: Bearer sk-xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "poster-translate",
    "metadata": {
      "imageUrl": "https://example.com/banner_cn.jpg",
      "to": 1
    }
  }'
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `imageUrl` | string | 是 | 待翻译图片 URL |
| `to` | int | 是 | 目标语言编号（见下表） |
| `from` | string | 否 | 原语言，默认 `auto` 自动识别 |

**`to` 语言编号：**

| 编号 | 语言 | 编号 | 语言 |
|------|------|------|------|
| 0 | 中文 | 9 | 越南语 |
| 1 | 英文 | 10 | 土耳其语 |
| 2 | 俄语 | 11 | 马来语 |
| 3 | 西班牙语 | 12 | 泰语 |
| 4 | 法语 | 13 | 波兰语 |
| 5 | 德语 | 14 | 印尼语 |
| 6 | 意大利语 | 15 | 日语 |
| 7 | 荷兰语 | 16 | 韩语 |
| 8 | 葡萄牙语 | 17 | 繁体中文 |

---

### 3.6 局部重绘（poster-partial-redraw）

根据提示词修改图片局部区域内容。

**请求：**
```bash
curl -X POST https://your-gateway/v1/images/generations \
  -H "Authorization: Bearer sk-xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "poster-partial-redraw",
    "metadata": {
      "sourceUrl": "https://example.com/product.jpg",
      "textPrompt": "将背景替换为森林场景，保持产品不变",
      "replaceImageUrl": "https://example.com/forest_ref.jpg"
    }
  }'
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `sourceUrl` | string | 是 | 原图 URL |
| `textPrompt` | string | 是 | 重绘描述 |
| `replaceImageUrl` | string | 否 | 参考图片 URL |

---

### 3.7 场景替换（poster-scene-replace）

替换图片背景/场景，保留主体商品。

**请求：**
```bash
curl -X POST https://your-gateway/v1/images/generations \
  -H "Authorization: Bearer sk-xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "poster-scene-replace",
    "metadata": {
      "sourceUrl": "https://example.com/product.jpg",
      "replaceImageUrl": "https://example.com/beach_scene.jpg",
      "textPrompt": "将背景替换为海滩场景"
    }
  }'
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `sourceUrl` | string | 是 | 原图 URL（含主体商品） |
| `replaceImageUrl` | string | 是 | 场景参考图 URL |
| `textPrompt` | string | 是 | 场景描述 |
| `modelType` | int | 否 | 0=默认，1=高质量 |

---

### 3.8 商品替换（poster-product-replace）

将场景图中的商品替换为另一个商品。

**请求：**
```bash
curl -X POST https://your-gateway/v1/images/generations \
  -H "Authorization: Bearer sk-xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "poster-product-replace",
    "metadata": {
      "sourceUrl": "https://example.com/scene_with_old_product.jpg",
      "replaceImageUrl": "https://example.com/new_product.jpg",
      "textPrompt": "将场景中的商品替换为新商品，保持场景一致"
    }
  }'
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `sourceUrl` | string | 是 | 含旧商品的场景图 URL |
| `replaceImageUrl` | string | 是 | 目标商品图 URL |
| `textPrompt` | string | 是 | 替换描述 |
| `modelType` | int | 否 | 0=默认，1=高质量 |

---

### 3.9 商品换色（poster-color-change）

根据提示词修改图片中商品的颜色。

**请求：**
```bash
curl -X POST https://your-gateway/v1/images/generations \
  -H "Authorization: Bearer sk-xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "poster-color-change",
    "metadata": {
      "sourceUrl": "https://example.com/bag_blue.jpg",
      "textPrompt": "将包包颜色换成玫瑰红色"
    }
  }'
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `sourceUrl` | string | 是 | 原图 URL |
| `textPrompt` | string | 是 | 换色描述，如"换成玫瑰红" |
| `modelType` | int | 否 | 0=默认，1=高质量 |

---

### 3.10 AI 帮写（poster-assisted）

根据需求生成产品文案，返回文字内容（非图片）。

**请求：**
```bash
curl -X POST https://your-gateway/v1/images/generations \
  -H "Authorization: Bearer sk-xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "poster-assisted",
    "metadata": {
      "query": "为一款保湿面霜生成亚马逊产品描述文案，突出天然成分和长效保湿",
      "fileUrlList": ["https://example.com/cream.jpg"],
      "generateType": "image"
    }
  }'
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `query` | string | 是 | 需求描述，最多 1000 字符 |
| `fileUrlList` | array | 否 | 参考图片 URL 列表，最多 6 张 |
| `generateType` | string | 否 | `image`=图片文案，`video`=视频文案 |

**返回（文案在 `revised_prompt` 字段，`url` 为空）：**
```json
{
  "created": 1720000000,
  "data": [
    { "url": "", "revised_prompt": "24-Hour Deep Moisture Cream | Natural Ingredients" },
    { "url": "", "revised_prompt": "Intense Hydration Face Cream with Plant Extract" }
  ]
}
```

---

### 3.11 同步海报生成（poster-generate-sync）

直接生成完整电商海报图片，同步等待返回结果。

**请求：**
```bash
curl -X POST https://your-gateway/v1/images/generations \
  -H "Authorization: Bearer sk-xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "poster-generate-sync",
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
  }'
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `query` | string | 是 | 需求描述 |
| `generateType` | int | 是 | 100=产品单图，200=产品详情图 |
| `posterType` | int | 是 | 5=跨境电商，6=中文电商 |
| `platformType` | string | 是 | 平台，如 `Amazon`、`天猫`、`京东` |
| `languageType` | string | 是 | 语种，如 `英语`、`中文` |
| `detailPictureNumber` | int | 是 | 生成图片数量（1-6 或 1-15） |
| `modelEdition` | int | 是 | 2=v2.0，3=v3.0，9=Image 2 |
| `needText` | boolean | 是 | 是否在海报上添加文案 |
| `aspectRatio` | string | 否 | 比例，如 `1:1`、`16:9`、`9:16` |
| `fileUrlList` | array | 否 | 参考图片列表 |

**返回：**
```json
{
  "created": 1720000000,
  "data": [
    { "url": "https://oss.example.com/poster1.jpg" },
    { "url": "https://oss.example.com/poster2.jpg" }
  ]
}
```

---

### 3.12 异步海报生成（poster-generate）

提交生成任务，异步返回结果。适合生成数量多、质量要求高的场景。

#### 提交任务

```bash
curl -X POST https://your-gateway/v1/images/tasks \
  -H "Authorization: Bearer sk-xxx" \
  -H "Content-Type: application/json" \
  -d '{
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
  }'
```

参数说明同 3.11。

**提交返回：**
```json
{
  "id": "task_abc123",
  "object": "image.task",
  "status": "processing",
  "model": "poster-generate",
  "created": 1720000000
}
```

#### 轮询结果

```bash
curl https://your-gateway/v1/images/tasks/task_abc123 \
  -H "Authorization: Bearer sk-xxx"
```

**进行中：**
```json
{
  "id": "task_abc123",
  "object": "image.task",
  "status": "processing",
  "model": "poster-generate",
  "created": 1720000000
}
```

**完成：**
```json
{
  "id": "task_abc123",
  "object": "image.task",
  "status": "succeeded",
  "model": "poster-generate",
  "created": 1720000000,
  "result": {
    "data": [
      { "url": "https://oss.example.com/poster1.jpg" },
      { "url": "https://oss.example.com/poster2.jpg" }
    ]
  }
}
```

**失败：**
```json
{
  "id": "task_abc123",
  "object": "image.task",
  "status": "failed",
  "model": "poster-generate",
  "created": 1720000000,
  "error": {
    "message": "任务失败原因",
    "code": "task_failed"
  }
}
```

| status | 说明 |
|--------|------|
| `processing` | 任务进行中，继续轮询 |
| `succeeded` | 任务成功，图片在 `result.data` |
| `failed` | 任务失败，原因在 `error.message` |

---

### 3.13 自由创作（poster-free-creation）

自由描述生成海报，不受模板限制，异步返回。

#### 提交任务

```bash
curl -X POST https://your-gateway/v1/images/tasks \
  -H "Authorization: Bearer sk-xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "poster-free-creation",
    "metadata": {
      "query": "科技感十足的蓝色电子产品展示图，金属质感背景",
      "detailPictureNumber": 2,
      "aspectRatio": "16:9",
      "apiImgUrlList": ["https://example.com/ref.jpg"]
    }
  }'
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `query` | string | 是 | 需求描述 |
| `detailPictureNumber` | int | 否 | 生成图片数量 |
| `aspectRatio` | string | 否 | 比例，如 `1:1`、`16:9` |
| `apiImgUrlList` | array | 否 | 参考图片列表 |

轮询方式同 3.12。

---

## 四、统一响应格式

### 同步成功
```json
{
  "created": 1720000000,
  "data": [
    { "url": "https://oss.example.com/result.jpg" }
  ]
}
```

### 同步失败
```json
{
  "error": {
    "message": "错误描述",
    "type": "upstream_error",
    "code": "500"
  }
}
```

### 异步状态枚举

| status | 含义 |
|--------|------|
| `processing` | 进行中 |
| `succeeded` | 成功 |
| `failed` | 失败 |

---

## 五、注意事项

1. 所有业务参数通过 `metadata` 字段传递
2. 同步接口走 `POST /v1/images/generations`，异步接口走 `POST /v1/images/tasks`
3. `poster-assisted` 返回的是文案文字，不是图片，内容在 `revised_prompt` 字段，`url` 为空
4. 不需要文本描述的接口：`poster-matting`、`poster-enlarge`、`poster-enhance`、`poster-extension`、`poster-translate`，只传图片参数即可
5. 异步接口建议每 5-10 秒轮询一次，超时时间建议设为 120 秒
