# 海报图像渠道使用说明

## 一、建渠道

进入管理后台 → 渠道 → 新建渠道，填写以下字段：

| 字段 | 值 | 说明 |
|------|-----|------|
| 渠道类型 | `Poster` | 下拉选择 |
| 名称 | 自定义，如 `海报API` | 仅用于标识 |
| Base URL | `http://your-host:9096` | 上游服务地址，不带末尾斜杠 |
| 密钥 | `your_api_key` | 上游 API Key，鉴权用 `Authorization: Bearer <key>` |
| 模型 | 见下方模型列表 | 填写该渠道支持的模型名 |

---

## 二、模型列表

### 异步接口（提交后需轮询结果）

| 模型名 | 功能 | 对应上游接口 |
|--------|------|-------------|
| `poster-generate` | 海报生成 | `/openapi/v1/poster/generateAsync` |
| `poster-free-creation` | 自由创作 | `/openapi/v1/poster/allAroundCreation` |

### 同步接口（直接返回结果）

| 模型名 | 功能 | 对应上游接口 |
|--------|------|-------------|
| `poster-extension` | 智能延展 | `/openapi/v1/ai/extension` |
| `poster-translate` | 图片翻译 | `/openapi/v1/ai/translate` |
| `poster-enlarge` | 无损放大 | `/openapi/v1/ai/enlarge` |
| `poster-matting` | AI 抠图 | `/openapi/v1/ai/matting` |
| `poster-enhance` | AI 超清 | `/openapi/v1/ai/enhance` |
| `poster-partial-redraw` | 局部重绘 | `/openapi/v1/ai/partialRedrawing` |
| `poster-scene-replace` | 场景替换 | `/openapi/v1/ai/sceneReplace` |
| `poster-product-replace` | 商品替换 | `/openapi/v1/ai/productReplace` |
| `poster-color-change` | 商品换色 | `/openapi/v1/ai/colorChange` |
| `poster-assisted` | AI 帮写（返回文案） | `/openapi/v1/ai/assisted` |

---

## 三、配置模型价格

进入管理后台 → 模型价格 → 新建，按次计费：

| 模型名 | 计费类型 | 建议配置 |
|--------|---------|---------|
| `poster-generate` | 按次 | 根据上游实际费率设置 |
| `poster-free-creation` | 按次 | 同上 |
| `poster-extension` | 按次 | 同上 |
| 其他模型 | 按次 | 同上 |

> 系统默认 `1元 = 500,000 积分`，按需换算。

---

## 四、调用示例

### 4.1 异步海报生成

**提交任务：**
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

**返回：**
```json
{
  "id": "task_abc123",
  "object": "image.task",
  "status": "processing",
  "model": "poster-generate",
  "created": 1720000000
}
```

**轮询结果：**
```bash
curl https://your-gateway/v1/images/tasks/task_abc123 \
  -H "Authorization: Bearer sk-xxx"
```

**完成返回：**
```json
{
  "id": "task_abc123",
  "object": "image.task",
  "status": "succeeded",
  "model": "poster-generate",
  "created": 1720000000,
  "result": {
    "data": [
      { "url": "https://oss.example.com/result1.jpg" },
      { "url": "https://oss.example.com/result2.jpg" }
    ]
  }
}
```

---

### 4.2 AI 抠图（同步，无文本）

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

### 4.3 商品换色（同步，有文本）

```bash
curl -X POST https://your-gateway/v1/images/generations \
  -H "Authorization: Bearer sk-xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "poster-color-change",
    "metadata": {
      "sourceUrl": "https://example.com/bag_blue.jpg",
      "textPrompt": "将包包颜色换成玫瑰红色",
      "modelType": 0
    }
  }'
```

---

### 4.4 图片翻译

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

`to` 语言编号：0=中文，1=英文，2=俄语，3=西班牙语，4=法语，5=德语，6=意大利语，7=荷兰语，8=葡萄牙语，9=越南语，10=土耳其语，11=马来语，12=泰语，13=波兰语，14=印尼语，15=日语，16=韩语，17=繁体中文

---

### 4.5 AI 帮写（返回文案）

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

**返回（文案在 `revised_prompt` 字段）：**
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

## 五、注意事项

1. **所有参数通过 `metadata` 传递**，不要放在请求根级别（除非与外层 `model` 字段冲突）
2. **异步接口**（poster-generate / poster-free-creation）走 `/v1/images/tasks`，需要轮询
3. **同步接口**走 `/v1/images/generations`，直接返回结果
4. **poster-matting / poster-enlarge / poster-enhance / poster-extension / poster-translate** 不需要文本描述，只传图片参数即可
5. **poster-assisted** 返回的是文案字符串，`url` 字段为空，内容在 `revised_prompt` 里
6. 渠道 Base URL 和密钥与广告视频上游相同，可以配置相同的值共用同一个上游服务
