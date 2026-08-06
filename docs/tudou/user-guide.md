# 图像生成 API 用户文档

## 概述

本服务提供异步图像生成功能，支持文生图和图生图两种模式。使用 OpenAI 兼容的接口格式。

## 接口端点

### 提交图像生成任务

```
POST /v1/images/generations
```

**请求头：**

```
Authorization: Bearer YOUR_API_KEY
Content-Type: application/json
```

**请求参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| model | string | 是 | 固定传 `gpt-image-2-all` |
| prompt | string | 是 | 图像描述，支持中英文，建议详细描述 |
| size | string | 否 | 画面比例，如 `1:1` `16:9` `3:2` 等，默认 `1:1` |
| images | array[string] | 否 | 参考图数组（图生图模式），支持 HTTP/HTTPS URL 或 base64 |
| metadata.resolution | string | 是 | 清晰度档位：`1k` / `2k` / `4k` |
| metadata.quality | string | 是 | 质量档位：`low` / `medium` / `high` |

**支持的画面比例 (size)：**

- `1:1` - 正方形
- `16:9` - 宽屏（推荐视频封面）
- `9:16` - 竖屏（推荐手机壁纸）
- `3:2`, `2:3`, `4:3`, `3:4` - 经典照片比例
- `21:9`, `9:21` - 超宽屏
- 也可直接传像素如 `2048x1152`

**分辨率与像素对照：**

| size | resolution=1k | resolution=2k | resolution=4k |
|------|---------------|---------------|---------------|
| 1:1 | 1024×1024 | 2048×2048 | 2880×2880 |
| 16:9 | 1536×864 | 2048×1152 | 3840×2160 |
| 3:2 | 1536×1024 | 2048×1360 | 3520×2336 |

### 请求示例

#### 文生图（基础）

```bash
curl -X POST "/v1/images/generations" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2-all",
    "prompt": "一只橘猫坐在窗台上看夕阳，水彩画风格",
    "size": "1:1",
    "metadata": {
      "resolution": "2k",
      "quality": "high"
    }
  }'
```

#### 文生图（4K高质量）

```bash
curl -X POST "/v1/images/generations" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2-all",
    "prompt": "星空下的古老城堡，电影感，细节丰富",
    "size": "16:9",
    "metadata": {
      "resolution": "4k",
      "quality": "high"
    }
  }'
```

#### 图生图（单张参考图 URL）

```bash
curl -X POST "/v1/images/generations" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2-all",
    "prompt": "把这张照片变成水彩画风格",
    "size": "4:3",
    "images": ["https://example.com/photo.jpg"],
    "metadata": {
      "resolution": "2k",
      "quality": "medium"
    }
  }'
```

#### 图生图（多张参考图融合）

```bash
curl -X POST "/v1/images/generations" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2-all",
    "prompt": "融合这两张照片，创造一个梦幻场景",
    "size": "16:9",
    "images": [
      "https://example.com/photo1.jpg",
      "https://example.com/photo2.jpg"
    ],
    "metadata": {
      "resolution": "4k",
      "quality": "high"
    }
  }'
```

#### 图生图（base64 格式）

```bash
curl -X POST "/v1/images/generations" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2-all",
    "prompt": "参考输入图，生成抽象艺术作品",
    "size": "1:1",
    "images": [
      "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAA..."
    ],
    "metadata": {
      "resolution": "2k",
      "quality": "medium"
    }
  }'
```

**注意：** base64 格式必须带 `data:image/png;base64,` 或 `data:image/jpeg;base64,` 前缀。

### 响应格式

#### 提交成功响应

```json
{
  "id": "task_bSPHAaYDWZIUXM0YkfXgtiWUlPnGnare",
  "status": "submitted",
  "created": 1780979971
}
```

**响应字段说明：**

| 字段 | 类型 | 说明 |
|------|------|------|
| id | string | 任务ID（task_ 前缀），用于后续查询 |
| status | string | 任务状态：`submitted` / `processing` / `completed` / `failed` |
| created | integer | 创建时间戳（Unix时间戳） |

---

## 查询任务结果

### 查询单个任务

```
GET /v1/tasks/{task_id}
```

**请求头：**

```
Authorization: Bearer YOUR_API_KEY
```

**路径参数：**

- `task_id`: 提交任务时返回的任务ID

### 查询示例

```bash
curl "/v1/tasks/task_bSPHAaYDWZIUXM0YkfXgtiWUlPnGnare" \
  -H "Authorization: Bearer YOUR_API_KEY"
```

### 响应格式

#### 处理中

```json
{
  "task_id": "task_bSPHAaYDWZIUXM0YkfXgtiWUlPnGnare",
  "status": "processing",
  "progress": "30%"
}
```

#### 成功完成

```json
{
  "task_id": "task_bSPHAaYDWZIUXM0YkfXgtiWUlPnGnare",
  "status": "completed",
  "url": "https://xxxxxx/image/xxxxxxxx_0.png"
}
```

#### 失败

```json
{
  "task_id": "task_bSPHAaYDWZIUXM0YkfXgtiWUlPnGnare",
  "status": "failed",
  "reason": "moderation failed"
}
```

**响应字段说明：**

| 字段 | 类型 | 说明 |
|------|------|------|
| task_id | string | 任务ID |
| status | string | 任务状态：`submitted`（已提交）/ `processing`（处理中）/ `completed`（成功）/ `failed`（失败） |
| progress | string | 进度百分比（仅处理中时返回） |
| url | string | 生成的图像URL（仅成功时返回） |
| reason | string | 失败原因（仅失败时返回） |

---

## 任务状态说明

| 状态 | 说明 |
|------|------|
| submitted | 已提交，等待处理 |
| processing | 上游处理中 |
| completed | 成功完成，图像可用 |
| failed | 失败，查看 reason 字段了解原因 |

---

## 轮询建议

由于图像生成是异步任务，建议按以下策略轮询：

1. **首次查询延迟**：提交后等待 10-20 秒再开始查询
2. **轮询间隔**：每 3-5 秒查询一次
3. **超时时间**：建议设置 5 分钟超时
4. **避免无脑轮询**：不要使用毫秒级轮询，会浪费资源

### 轮询示例代码（Python）

```python
import requests
import time

def generate_image_and_wait(prompt, resolution="2k", quality="high"):
    # 1. 提交任务
    submit_url = "/v1/images/generations"
    headers = {
        "Authorization": "Bearer YOUR_API_KEY",
        "Content-Type": "application/json"
    }
    payload = {
        "model": "gpt-image-2-all",
        "prompt": prompt,
        "size": "1:1",
        "metadata": {
            "resolution": resolution,
            "quality": quality
        }
    }
    
    response = requests.post(submit_url, json=payload, headers=headers)
    result = response.json()
    task_id = result["id"]
    print(f"任务已提交: {task_id}")
    
    # 2. 等待 15 秒后开始轮询
    time.sleep(15)
    
    # 3. 轮询查询结果
    query_url = f"/v1/tasks/{task_id}"
    max_attempts = 60  # 最多尝试 60 次（5 分钟）
    
    for i in range(max_attempts):
        response = requests.get(query_url, headers=headers)
        result = response.json()
        status = result["status"]
        
        if status == "completed":
            print(f"生成成功: {result['url']}")
            return result["url"]
        elif status == "failed":
            print(f"生成失败: {result.get('reason', '未知错误')}")
            return None
        else:
            print(f"处理中... ({result.get('progress', 'N/A')})")
            time.sleep(5)  # 等待 5 秒
    
    print("超时：任务未在 5 分钟内完成")
    return None

# 使用示例
image_url = generate_image_and_wait("一只可爱的小猫", resolution="4k", quality="high")
```

---

## 常见错误

| 错误信息 | 说明 | 解决方案 |
|---------|------|---------|
| `prompt is required` | 缺少 prompt 参数 | 确保请求体包含 prompt 字段 |
| `invalid resolution` | resolution 参数非法 | 只能使用 `1k` / `2k` / `4k` |
| `invalid quality` | quality 参数非法 | 只能使用 `low` / `medium` / `high` |
| `moderation failed` | 内容审核未通过 | prompt 包含违规内容，已拒绝且不计费 |
| `insufficient balance` | 余额不足 | 请充值或联系管理员 |
| `task_not_found` | 任务不存在 | 检查 task_id 是否正确 |

---

## 注意事项

1. **图像过期时间**：生成的图像 URL 通常在 24 小时后过期，请及时下载或转存
2. **计费规则**：按 resolution 和 quality 档位计费，失败和审核未通过不计费
3. **参考图格式**：
   - 支持公网 HTTP/HTTPS URL
   - 支持 base64 格式（需带 MIME 前缀）
   - 不支持本地文件路径
4. **并发限制**：根据账户配置的并发数限制，建议高并发场景使用队列
5. **Prompt 优化**：详细的描述能获得更好的生成效果

---

## 定价参考

具体价格请参考管理后台的定价配置。一般来说：

- **1k + low**: 最经济，适合预览和测试
- **2k + medium**: 平衡选择，适合日常使用
- **4k + high**: 最高质量，适合专业用途和打印

---

## 技术支持

如有问题，请联系管理员。
