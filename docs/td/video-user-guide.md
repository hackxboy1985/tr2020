# 视频生成 API 用户文档

> **面向读者**：接入本平台视频生成能力的下游开发者
> **版本**：v1（2026-10-13）
> **关联文档**：[图像生成 API 用户文档](./user-guide.md)、[Td 渠道视频模型接入设计](../channel/td-video-models.md)（内部实现文档，下游无需阅读）

---

## 一、概述

本服务提供**异步视频生成**能力，提交任务后立即返回任务 ID，客户端轮询查询接口获取生成进度和最终视频地址。

- **认证方式**：`Authorization: Bearer YOUR_API_KEY`
- **请求格式**：`application/json`
- **任务模型**：提交即返回，异步生成，轮询取结果
- **适用场景**：文生视频、图生视频、带参考视频/音频的视频生成

当前支持三个模型：

| 模型名 | 说明 |
|---|---|
| `minimax-h3` | 支持参考图；不支持参考视频、参考音频与音效同步 |
| `seedance-2.0` | 支持参考图/视频/音频与音效同步 |
| `seedance-2.5` | 同上，时长与素材上限更高 |

> **模型名以你的账号实际可用为准**。请联系管理员确认可用模型清单与计费规则；若管理员配置了模型别名，提交时使用别名即可，无需关心内部分辨率映射。

---

## 二、接口端点

| 接口 | 方法 | 说明 |
|---|---|---|
| `/v1/videos/generations` | POST | 提交视频生成任务 |
| `/v1/videos/tasks/{task_id}` | GET | 查询视频任务状态与结果 |

Base URL 请替换为你的服务地址，例如 `https://your-new-api-domain`。

---

## 三、提交视频任务

```
POST {BASE_URL}/v1/videos/generations
```

**请求头**

```
Authorization: Bearer YOUR_API_KEY
Content-Type: application/json
```

**请求参数**

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `model` | string | 是 | - | 视频模型名，见上表 |
| `prompt` | string | 是 | - | 视频描述。不能为空或纯空白 |
| `duration` | integer | 否 | `4` | 视频时长（秒），取值范围随模型而定 |
| `resolution` | string | 否 | `720p` | 清晰度档位，取值随模型而定 |
| `aspect_ratio` | string | 否 | `16:9` | 画面比例，只能取 `16:9` 或 `9:16` |
| `images` | array&lt;string&gt; | 否 | - | 参考图数组，数量上限随模型而定 |
| `videos` | array&lt;string&gt; | 否 | - | 参考视频数组，数量上限随模型而定 |
| `audios` | array&lt;string&gt; | 否 | - | 参考音频数组，数量上限随模型而定 |
| `generate_audio` | boolean | 否 | - | 是否同步生成音效。仅部分模型支持 |

> `images` / `videos` / `audios` 中的每一项都应是**可公网访问的 HTTP/HTTPS URL**。
> `generate_audio` 显式传 `false` 与不传是两种不同语义，需要关闭时请显式传 `false`。

### 3.1 能力矩阵（重要）

三个模型的参数边界**不相同**，超出边界的请求会被直接拒绝（HTTP 400），不会静默忽略：

| 能力 | minimax-h3 | seedance-2.0 | seedance-2.5 |
|---|---|---|---|
| `duration` 范围（秒） | 3 – 15 | 3 – 15 | 4 – 30 |
| `duration` 默认值 | 4 | 4 | 4 |
| `resolution` 可选值 | `720p` / `1080p` / `2k` | 仅 `720p` | 仅 `720p` |
| `resolution` 默认值 | `720p` | `720p` | `720p` |
| `images` 上限 | 9 | 9 | 30 |
| `videos` 上限 | 不支持 | 3 | 10 |
| `audios` 上限 | 不支持 | 3 | 10 |
| `generate_audio` | 不支持 | 支持 | 支持 |

通用约束：

- `aspect_ratio` 只能取 `16:9` / `9:16`，默认 `16:9`；
- `audios` **不能单独使用**，必须同时提供 `images` 或 `videos`；
- `model` 不在支持列表中的请求一律 400 拒绝。

> **常见踩坑**：向 `seedance-2.0` 传 `duration: 16` 会失败（上限 15），需要长视频请改用 `seedance-2.5`；向 `seedance-2.0` / `seedance-2.5` 传 `resolution: 1080p` 也会失败（仅支持 `720p`）。

### 3.2 请求示例

#### 文生视频

```bash
curl -X POST "https://your-new-api-domain/v1/videos/generations" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "seedance-2.5",
    "prompt": "海边日落，无人机缓慢环绕拍摄，画面平稳，电影质感",
    "duration": 5,
    "resolution": "720p",
    "aspect_ratio": "16:9"
  }'
```

#### 图生视频（单张参考图）

```bash
curl -X POST "https://your-new-api-domain/v1/videos/generations" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "seedance-2.5",
    "prompt": "让人物缓慢转头看向镜头，发丝轻微飘动",
    "images": ["https://example.com/portrait.jpg"],
    "duration": 4,
    "aspect_ratio": "9:16"
  }'
```

#### 带参考视频 + 音效同步

```bash
curl -X POST "https://your-new-api-domain/v1/videos/generations" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "seedance-2.5",
    "prompt": "保持参考视频的镜头运动，把场景替换成雨夜街道",
    "videos": ["https://example.com/ref.mp4"],
    "audios": ["https://example.com/bgm.mp3"],
    "generate_audio": true,
    "duration": 8,
    "aspect_ratio": "16:9"
  }'
```

#### 高分辨率（仅 minimax-h3）

```bash
curl -X POST "https://your-new-api-domain/v1/videos/generations" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "minimax-h3",
    "prompt": "城市夜景延时摄影，车流灯轨",
    "duration": 6,
    "resolution": "1080p"
  }'
```

### 3.3 提交响应

```json
{
  "id": "task_bSPHAaYDWZIUXM0YkfXgtiWUlPnGnare",
  "object": "video.task",
  "status": "submitted",
  "model": "seedance-2.5",
  "created": 1780979971
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 任务 ID（`task_` 前缀），用于后续查询 |
| `object` | string | 固定为 `video.task` |
| `status` | string | 提交成功固定为 `submitted` |
| `model` | string | 本次任务使用的模型名 |
| `created` | integer | 创建时间戳（Unix 秒） |

---

## 四、查询任务结果

```
GET {BASE_URL}/v1/videos/tasks/{task_id}
```

**请求头**

```
Authorization: Bearer YOUR_API_KEY
```

**路径参数**

- `task_id`：提交任务时返回的任务 ID

```bash
curl "https://your-new-api-domain/v1/videos/tasks/task_bSPHAaYDWZIUXM0YkfXgtiWUlPnGnare" \
  -H "Authorization: Bearer YOUR_API_KEY"
```

### 4.1 处理中

```json
{
  "id": "task_bSPHAaYDWZIUXM0YkfXgtiWUlPnGnare",
  "object": "video.task",
  "status": "processing",
  "model": "seedance-2.5",
  "created": 1780979971
}
```

### 4.2 成功完成

```json
{
  "id": "task_bSPHAaYDWZIUXM0YkfXgtiWUlPnGnare",
  "object": "video.task",
  "status": "succeeded",
  "model": "seedance-2.5",
  "created": 1780979971,
  "result": {
    "video_url": "https://cdn.example.com/out/xxxx.mp4",
    "videos": [
      { "url": ["https://cdn.example.com/out/xxxx.mp4"] }
    ]
  }
}
```

### 4.3 失败

```json
{
  "id": "task_bSPHAaYDWZIUXM0YkfXgtiWUlPnGnare",
  "object": "video.task",
  "status": "failed",
  "model": "seedance-2.5",
  "created": 1780979971,
  "error": {
    "message": "content moderation failed",
    "code": "task_failed"
  }
}
```

### 4.4 响应字段说明

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 任务 ID |
| `object` | string | 固定为 `video.task` |
| `status` | string | 见下方「任务状态」 |
| `model` | string | 模型名 |
| `created` | integer | 创建时间戳 |
| `result.video_url` | string | **视频地址（主字段，推荐使用）**，仅成功时返回 |
| `result.videos` | array | 视频数组，每项 `url` 为字符串数组；与 `video_url` 内容一致，仅成功时返回 |
| `error.message` | string | 失败原因，仅失败时返回 |
| `error.code` | string | 失败错误码，通常为 `task_failed` |

> 取视频地址建议**优先读 `result.video_url`**，取不到时回退 `result.videos[0].url[0]`。

---

## 五、任务状态

查询接口返回的 `status` 只有三种取值：

| status | 说明 | 建议动作 |
|---|---|---|
| `processing` | 排队中或生成中 | 继续轮询 |
| `succeeded` | 成功完成，视频可用 | 从 `result` 取地址并下载 |
| `failed` | 失败 | 读 `error.message`，停止轮询 |

> 注意：`submitted` **只出现在提交响应中**，查询接口不会返回。查询接口用 `processing` 覆盖「排队中 / 生成中」两种内部状态。

---

## 六、轮询建议

视频生成是异步长任务，建议按下述策略轮询：

1. **首次查询延迟**：提交后等待 15–30 秒再开始查询；
2. **轮询间隔**：每 5 秒查询一次，不要使用毫秒级轮询；
3. **超时时间**：建议设置 10 分钟超时（长时长任务可放宽到 15 分钟）；
4. **退避策略**：连续多次仍为 `processing` 时，可逐步拉长间隔（如 5s → 10s → 15s）。

### 6.1 Python 轮询示例

```python
import time
import requests

BASE_URL = "https://your-new-api-domain"
API_KEY = "YOUR_API_KEY"

HEADERS = {
    "Authorization": f"Bearer {API_KEY}",
    "Content-Type": "application/json",
}


def generate_video_and_wait(prompt, model="seedance-2.5", duration=5,
                            resolution="720p", aspect_ratio="16:9",
                            images=None, max_wait=600):
    # 1. 提交任务
    payload = {
        "model": model,
        "prompt": prompt,
        "duration": duration,
        "resolution": resolution,
        "aspect_ratio": aspect_ratio,
    }
    if images:
        payload["images"] = images

    resp = requests.post(f"{BASE_URL}/v1/videos/generations",
                         json=payload, headers=HEADERS, timeout=60)
    if resp.status_code != 200:
        print(f"提交失败 HTTP {resp.status_code}: {resp.text}")
        return None

    data = resp.json()
    task_id = data["id"]
    print(f"任务已提交: {task_id}")

    # 2. 等待 15 秒后开始轮询
    time.sleep(15)

    # 3. 轮询直到终态
    query_url = f"{BASE_URL}/v1/videos/tasks/{task_id}"
    deadline = time.time() + max_wait
    interval = 5

    while time.time() < deadline:
        r = requests.get(query_url, headers=HEADERS, timeout=60)
        if r.status_code != 200:
            print(f"查询失败 HTTP {r.status_code}: {r.text}")
            return None

        body = r.json()
        status = body.get("status")

        if status == "succeeded":
            result = body.get("result") or {}
            url = result.get("video_url")
            if not url:
                videos = result.get("videos") or []
                if videos and videos[0].get("url"):
                    url = videos[0]["url"][0]
            print(f"生成成功: {url}")
            return url

        if status == "failed":
            err = body.get("error") or {}
            print(f"生成失败: {err.get('message', '未知错误')}")
            return None

        print(f"处理中... ({status})")
        time.sleep(interval)

    print("超时：任务未在限定时间内完成")
    return None


if __name__ == "__main__":
    video_url = generate_video_and_wait("海边日落，无人机缓慢环绕拍摄")
    print(video_url)
```

---

## 七、计费说明

- 视频任务采用**预扣费 + 完成后结算**：提交时按你请求的参数预扣额度，任务完成时按实际结果结算；
- 计费与 `duration`、`resolution` 相关，不同模型、不同档位价格不同；
- **任务失败会退还预扣额度**，失败任务不产生费用；
- 任务长时间无进展会被系统判定超时并置为 `failed`，同样会退款；
- 具体单价请以管理后台的定价配置为准。

---

## 八、产物地址与有效期

- 成功响应中的 `result.video_url` 是可直接下载的视频地址；
- 该地址**可能带有有效期**（由平台管理员配置，默认按 24 小时口径处理）。**请在任务成功后及时下载并转存到自己的存储**，不要长期依赖该地址；
- 管理员也可能配置对外代理域名，此时返回的地址域名与上游不同，属正常现象，直接使用即可；
- 地址过期或已失效后重新拉取同一地址可能返回错误，此时需要重新提交任务。

---

## 九、错误响应

### 9.1 提交阶段（参数类）

参数校验失败统一返回 **HTTP 400**，响应体形如：

```json
{
  "code": "invalid_request",
  "message": "invalid duration: 16, model seedance-2.0 supports 3-15 seconds",
  "data": null
}
```

常见错误对照表：

| `message` | 触发场景 | 处理方式 |
|---|---|---|
| `prompt is required` | `prompt` 缺失或为空白 | 补充 `prompt` |
| `model is required` | `model` 缺失 | 补充 `model` |
| `unsupported video model: xxx` | 模型名不在支持列表 | 改用受支持的模型名，或联系管理员开通 |
| `invalid duration: N, model X supports A-B seconds` | 时长越界（含显式传 `0`） | 调整到模型允许范围内 |
| `invalid resolution: X, model Y supports: ...` | 分辨率档位不属于该模型 | 按该模型允许的档位传值 |
| `invalid aspect_ratio: X, must be one of: 16:9, 9:16` | 比例非法 | 只能传 `16:9` 或 `9:16` |
| `too many reference images: N, model X allows at most M` | 参考图超上限 | 减少参考图 |
| `model X does not support images` | 该模型不支持参考图 | 去掉 `images` |
| `too many reference videos: ...` / `model X does not support videos` | 参考视频超限或不支持 | 减少或去掉 `videos` |
| `too many reference audios: ...` / `model X does not support audios` | 参考音频超限或不支持 | 减少或去掉 `audios` |
| `audios cannot be used alone, at least one image or video is required` | 只传了音频 | 同时提供 `images` 或 `videos` |
| `model X does not support generate_audio` | 该模型不支持音效同步 | 去掉 `generate_audio`，或改用支持的模型 |

### 9.2 提交阶段（账号 / 路由类）

此类错误响应体为 OpenAI 风格：

```json
{
  "error": {
    "message": "No available channel for model seedance-2.5 under group default (distributor)",
    "type": "new_api_error",
    "code": "model_not_found"
  }
}
```

| HTTP | `code` | 含义 | 处理方式 |
|---|---|---|---|
| 401 | - | API Key 缺失或无效 | 检查 `Authorization` 头 |
| 400 | `invalid_request` | 请求体不是合法 JSON | 检查请求体格式 |
| 503 | `model_not_found` | 当前分组下该模型无可用渠道 | 确认模型名正确、渠道已配置且令牌分组可访问 |
| 429 | - | 触发限流或上游负载饱和 | 稍后重试 |

### 9.3 查询阶段

| HTTP | `code` | 含义 | 处理方式 |
|---|---|---|---|
| 400 | `task_not_exist` | 任务不存在，或不属于当前 API Key 对应的账号 | 核对 `task_id`；任务只能由提交它的账号查询 |
| 400 | `invalid_relay_mode` | 请求路径不匹配任何查询接口 | 核对 URL 是否为 `/v1/videos/tasks/{task_id}` |
| 500 | `get_task_failed` / `marshal_response_failed` | 服务端异常 | 稍后重试，持续失败请联系管理员 |

---

## 十、注意事项

1. **素材必须是公网可访问的 URL**：`images` / `videos` / `audios` 均只接受 HTTP/HTTPS 地址，不支持本地文件路径，也不使用 multipart 上传；
2. **模型能力边界不会静默忽略**：参数超界会直接 400，请按「能力矩阵」传参；
3. **`task_id` 与账号绑定**：换一个 API Key 查询同一 `task_id` 会返回 `task_not_exist`；
4. **并发限制**：按账号配置的并发数限制，建议高并发场景自行做任务队列；
5. **不要把轮询做成忙等**：5 秒级间隔足够，毫秒级轮询会浪费资源并可能触发限流；
6. **`duration` 越短并不一定越快**，任务耗时主要取决于上游排队情况；
7. **prompt 越具体效果越稳定**，建议写明主体、动作、镜头运动、画面风格与光线。

---

## 十一、完整调用流程（curl 版）

```bash
BASE_URL="https://your-new-api-domain"
API_KEY="YOUR_API_KEY"

# 1) 提交任务
SUBMIT=$(curl -sS -X POST "$BASE_URL/v1/videos/generations" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "seedance-2.5",
    "prompt": "海边日落，无人机缓慢环绕拍摄",
    "duration": 5,
    "aspect_ratio": "16:9"
  }')
echo "$SUBMIT"

TASK_ID=$(echo "$SUBMIT" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')
echo "task_id=$TASK_ID"

# 2) 轮询直到终态
while true; do
  QUERY=$(curl -sS "$BASE_URL/v1/videos/tasks/$TASK_ID" \
    -H "Authorization: Bearer $API_KEY")
  STATUS=$(echo "$QUERY" | python3 -c 'import json,sys; print(json.load(sys.stdin)["status"])')
  echo "status=$STATUS"
  if [ "$STATUS" = "succeeded" ] || [ "$STATUS" = "failed" ]; then
    echo "$QUERY"
    break
  fi
  sleep 5
done
```

---

## 十二、技术支持

如遇接口异常、模型不可用或计费疑问，请联系管理员，并附上 `task_id` 与完整的原始响应体。
