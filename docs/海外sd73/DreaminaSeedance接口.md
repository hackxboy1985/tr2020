# Dreamina Seedance 2.0 API 对接文档（海外版）

**版本**：v1.1 ｜ **更新日期**：2026-08-14
**协议**：BytePlus ModelArk 内容生成协议（视频生成由平台网关封装为 OpenAI 兼容接口）
**适用模型**：`dreamina-seedance-2-0-260128`（Seedance 2.0）

> **v1.1 变更**：仅保留标准版 `dreamina-seedance-2-0-260128`，移除 fast 版本相关内容。
>
> 本文档为**海外版（BytePlus）**对接说明，与平台国内版（`doubao-seedance-*`）相互独立：同一网关地址、同一接口路径，仅模型名与计费不同。请按业务选用对应模型。

---

## 一、通用说明

### 平台地址

| 模块 | BaseURL |
| --- | --- |
| **视频生成 API** | `http://106.54.45.168`（下文记作 `<BaseURL>`，后续可能替换为域名，以控制台公告为准） |

> 当前为 HTTP（未启用 HTTPS），客户端若强制校验证书请关闭或使用反向代理地址。

### 认证方式

所有接口使用 **Bearer Token（API Key）** 鉴权，请求头统一携带：

```
Authorization: Bearer <你的API Key>
Content-Type: application/json
```

> **API Key** 在控制台「令牌」页面创建（形如 `sk-xxxxxxxx`）。请妥善保管，不要在前端或客户端代码中硬编码。

---

## 二、模型说明

| 模型名 | 说明 | 分辨率档位 |
| --- | --- | --- |
| `dreamina-seedance-2-0-260128` | Seedance 2.0 标准版，画质最高 | 480p / 720p / 1080p / 4k |

---

## 三、视频生成 API

### 3.1 接口概览

| 项目 | 说明 |
| --- | --- |
| **BaseURL** | `http://106.54.45.168` |
| **创建任务** | `POST <BaseURL>/v1/video/generations` |
| **查询任务** | `GET <BaseURL>/v1/video/generations/{task_id}` |
| **交互模式** | 异步轮询（创建任务 → 轮询状态 → 下载视频） |

### 3.2 支持的生成模式

| 模式 | 输入 | 是否支持 | 关键参数 |
| --- | --- | --- | --- |
| **文生视频** | 文本 | ✅ 支持 | `prompt` |
| **图生视频（单张参考图）** | 图片 + 文本 | ✅ 支持 | `prompt` + `image` |
| **图生视频 · 首尾帧** | 2 张图 + 文本 | ✅ 支持 | `metadata.content`，role: `first_frame` + `last_frame` |
| **视频生视频** | 视频 + 文本 | ✅ 支持 | `metadata.content`，`video_url` + role |
| **多模态参考** | 多图 / 视频 / 音频 + 文本 | ✅ 支持 | `metadata.content`，见 3.6 |

### 3.3 请求参数

`POST <BaseURL>/v1/video/generations`，请求体：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `model` | string | 是 | 固定值 `dreamina-seedance-2-0-260128` |
| `prompt` | string | 是 | 文本描述（建议具体、有画面感） |
| `image` | string | 否 | **图生视频**：图片公网 URL / Base64 Data URL（< 2MB） |
| `metadata` | object | 否 | 生成参数，详见下表 |

**`metadata` 生成参数：**

| 字段 | 类型 | 默认 | 说明 |
| --- | --- | --- | --- |
| `resolution` | string | `720p` | ⭐ **分辨率**：`480p` / `720p` / `1080p` / `4k`（**必须放在 `metadata` 内，见下方警告**） |
| `ratio` | string | `16:9` | 画幅：`16:9`、`9:16`、`3:4`、`1:1`、`4:3`、`21:9`、`adaptive` |
| `duration` | integer | `5` | 时长（秒），默认 5 |
| `watermark` | boolean | `false` | 是否加水印 |
| `seed` | integer | 随机 | 随机种子，用于结果复现 |
| `generate_audio` | boolean | 上游默认 | 是否生成有声视频 |
| `return_last_frame` | boolean | `false` | 是否返回最后一帧图片 |
| `execution_expires_after` | integer | `172800` | 任务过期时间（秒），默认 48 小时 |
| `content` | array | — | 多模态输入（首尾帧 / 视频生视频 / 多模态参考），见 3.6 |

> ⚠️ **分辨率必须写在 `metadata.resolution` 里**。如果把 `resolution` 放在请求体顶层，会被忽略，导致：① 实际按 720p 生成；② **按 720p 基准价计费**（即使你想要 1080p/4k）。正确写法见下文示例。

### 3.4 模式一：文生视频

```bash
curl -X POST '<BaseURL>/v1/video/generations' \
  -H 'Authorization: Bearer sk-xxxxxxxx' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "dreamina-seedance-2-0-260128",
    "prompt": "一只金色柴犬在樱花树下奔跑，镜头缓缓上升，电影级画质",
    "metadata": {"resolution": "1080p", "ratio": "16:9", "duration": 5}
  }'
```

### 3.5 模式二：图生视频

```bash
curl -X POST '<BaseURL>/v1/video/generations' \
  -H 'Authorization: Bearer sk-xxxxxxxx' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "dreamina-seedance-2-0-260128",
    "prompt": "镜头缓缓推进，猫微微转头，毛发随风轻动",
    "image": "https://example.com/cat.jpg",
    "metadata": {"resolution": "720p", "ratio": "16:9", "duration": 5}
  }'
```

**图片输入方式**
- **公网 URL**（推荐）：`"image": "https://example.com/photo.jpg"`
- **Base64 Data URL**：`"image": "data:image/jpeg;base64,/9j/4AAQ..."`（图片 < 2MB 时可用；超过 2MB 建议先上传至对象存储再用 URL）

> ⚠️ **真人照片可能被内容审核拒绝**：上游会拦截含真人的图片。图生视频建议使用**非真人图片**（风景、动物、动漫、物体等）。

### 3.6 高级模式：多模态输入（metadata.content）

首尾帧、视频生视频、多模态参考等高级模式，通过把原生 `content` 数组放进 `metadata.content` 传递（平台原样转发给上游）。数组元素支持 `type`、`image_url` / `video_url` / `audio_url`、`role`。

**支持的 role（同一请求内互斥，不能混用）：**

| 媒体 | type | role 值 | 说明 |
| --- | --- | --- | --- |
| 图片 | `image_url` | `first_frame` | 首帧（传 1 张） |
| 图片 | `image_url` | `last_frame` | 尾帧（配合 `first_frame`，共 2 张） |
| 图片 | `image_url` | `reference_image` | 多模态参考图 |
| 视频 | `video_url` | `reference_video` | 参考视频 |
| 视频 | `video_url` | `source_video` | 编辑 / 延长视频 |
| 音频 | `audio_url` | `reference_audio` | 参考音频 |

**① 图生视频 · 首尾帧（2 张图）**

```bash
curl -X POST '<BaseURL>/v1/video/generations' \
  -H 'Authorization: Bearer sk-xxxxxxxx' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "dreamina-seedance-2-0-260128",
    "prompt": "镜头从首帧缓缓推进，自然过渡到尾帧，电影质感",
    "metadata": {
      "resolution": "1080p", "ratio": "16:9", "duration": 5,
      "content": [
        {"type": "image_url", "image_url": {"url": "https://example.com/first.jpg"}, "role": "first_frame"},
        {"type": "image_url", "image_url": {"url": "https://example.com/last.jpg"}, "role": "last_frame"}
      ]
    }
  }'
```

**② 视频生视频（参考视频）**

```bash
curl -X POST '<BaseURL>/v1/video/generations' \
  -H 'Authorization: Bearer sk-xxxxxxxx' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "dreamina-seedance-2-0-260128",
    "prompt": "保持画面主体不变，镜头缓缓拉远",
    "metadata": {
      "resolution": "1080p", "ratio": "16:9", "duration": 5,
      "content": [
        {"type": "video_url", "video_url": {"url": "https://example.com/ref.mp4"}, "role": "reference_video"}
      ]
    }
  }'
```

> ⚠️ 图片 / 视频 URL 必须**公网可访问**；视频输入不支持 Base64。首帧、首尾帧、多模态参考三种图片场景**互斥**，不能在同一个请求里混用。

### 3.7 创建任务响应

```json
{
  "id": "task_KPxsbZc6x2znnrk8vbRm359optsYdtx4",
  "task_id": "task_KPxsbZc6x2znnrk8vbRm359optsYdtx4",
  "object": "video",
  "model": "dreamina-seedance-2-0-260128",
  "status": "queued",
  "progress": 0,
  "created_at": 1786681030
}
```

| 字段 | 说明 |
| --- | --- |
| `task_id` | **任务 ID，后续轮询需要此值** |
| `status` | `queued`（已提交） |

### 3.8 查询任务状态

```
GET <BaseURL>/v1/video/generations/{task_id}
```

**响应（成功）：**

```json
{
  "code": "success",
  "message": "",
  "data": {
    "id": 21,
    "task_id": "task_KPxsbZc6x2znnrk8vbRm359optsYdtx4",
    "channel_id": 2,
    "action": "generate",
    "status": "SUCCESS",
    "progress": "100%",
    "result_url": "https://ark-acg-ap-southeast-1.tos-ap-southeast-1.volces.com/.../xxx.mp4?...",
    "fail_reason": "",
    "quota": 943346,
    "submit_time": 1786681030,
    "start_time": 1786681049,
    "finish_time": 1786681170,
    "data": {
      "id": "cgt-20260814121710-wc794",
      "model": "dreamina-seedance-2-0-260128",
      "resolution": "1080p",
      "ratio": "16:9",
      "duration": 5,
      "seed": 72072,
      "framespersecond": 24,
      "output_format": "mp4",
      "generate_audio": true,
      "usage": {"total_tokens": 245025, "completion_tokens": 245025},
      "status": "succeeded",
      "content": {"video_url": "https://ark-acg-ap-southeast-1.tos-ap-southeast-1.volces.com/.../xxx.mp4?..."}
    }
  }
}
```

**关键字段：**

| 字段 | 说明 |
| --- | --- |
| `data.status` | 任务状态，见下表 |
| `data.result_url` | **成功时的视频下载地址**（与 `data.data.content.video_url` 一致） |
| `data.fail_reason` | 失败时的原因 |
| `data.quota` | 本次消耗的额度（500000 额度 = $1） |
| `data.data.usage.total_tokens` | 本次消耗的 token 数 |
| `data.data.resolution` | 实际生成分辨率（可用于核对计费档位） |

### 3.9 状态流转

| `data.status` | 含义 | 处理 |
| --- | --- | --- |
| `NOT_START` / `IN_PROGRESS` | 排队 / 生成中 | 继续轮询 |
| `SUCCESS` | 成功 | 取 `data.result_url` 下载 |
| `FAILED` | 失败 | 看 `data.fail_reason` |

```
创建 → queued → NOT_START → IN_PROGRESS → SUCCESS（下载）
                                       └→ FAILED
```

---

## 四、计费说明

### 4.1 计费规则

- 计费单位为 **token**（由模型按"分辨率 × 时长"估算，非用户输入字数）。
- 换算关系：**500000 额度（quota）= $1**。
- 单次扣费 = `token 数 × 单价倍率`，**单价倍率按「实际分辨率」与「是否含视频输入」自动匹配**（平台已内置各档价）。

### 4.2 单价表（美元 / 每百万 token）

| 分辨率 | 纯生成（不含视频输入） | 含视频输入 |
| --- | --- | --- |
| 480p / 720p（基准） | **$7.00** | $4.30 |
| 1080p | $7.70 | $4.70 |
| 4k | $4.00 | $2.40 |

> 说明：① "含视频输入"指请求里带 `video_url`（视频生视频 / 视频参考），**图生视频（仅图片）仍按"纯生成"价**计。② 4k 单价虽低于 1080p，但 4k 的 token 数更多，**总价不一定更低**，请按实际测试评估。

### 4.3 成本示例（实测）

| 场景 | token 数 | 扣费额度 | ≈ 美元 |
| --- | --- | --- | --- |
| 5 秒 720p 纯生成 | 108,900 | 381,150 | **≈ $0.76** |
| 5 秒 1080p 纯生成 | 245,025 | 943,346 | **≈ $1.89** |

> token 数随分辨率和时长线性增长；上述为 5 秒实测参考，10 秒约翻倍。

### 4.4 ⚠️ 计费重要提醒

**分辨率必须通过 `metadata.resolution` 指定**，平台才能按对应档位计价。若放在请求体顶层（或未指定），将默认按 **720p 基准价**计费 —— 这意味着：
- 想要 1080p 却没走 `metadata` → 实际可能只生成 720p，且按 720p 计费；
- 想要 4k 却没走 `metadata` → 同上。

**正确写法**：`"metadata": {"resolution": "1080p"}`（详见 3.3）。

---

## 五、Python 代码示例

### 5.1 文生视频（完整流程）

```python
import requests
import time

BASE_URL = "http://106.54.45.168"   # 即 <BaseURL>
API_KEY  = "sk-xxxxxxxx"
MODEL    = "dreamina-seedance-2-0-260128"

HEADERS = {
    "Authorization": f"Bearer {API_KEY}",
    "Content-Type": "application/json",
}

def create_task(prompt, image=None, metadata=None):
    """创建视频生成任务。传入 image 即为图生视频。"""
    payload = {"model": MODEL, "prompt": prompt}
    if image:
        payload["image"] = image
    # 注意：resolution 必须放在 metadata 里
    payload["metadata"] = metadata or {"resolution": "1080p", "ratio": "16:9", "duration": 5}
    resp = requests.post(f"{BASE_URL}/v1/video/generations",
                         headers=HEADERS, json=payload, timeout=180)
    resp.raise_for_status()
    task_id = resp.json().get("task_id")
    print(f"[创建任务] task_id={task_id}")
    return task_id

def wait_for_result(task_id, max_wait=600, interval=10):
    """轮询任务直到成功或失败。"""
    url = f"{BASE_URL}/v1/video/generations/{task_id}"
    elapsed = 0
    while elapsed < max_wait:
        time.sleep(interval)
        elapsed += interval
        d = (requests.get(url, headers=HEADERS, timeout=60).json().get("data") or {})
        status = d.get("status", "")
        print(f"  [{elapsed}s] status={status}")
        if status == "SUCCESS":
            return d.get("result_url")
        if status == "FAILED":
            raise RuntimeError(f"任务失败: {d.get('fail_reason')}")
    raise TimeoutError(f"任务超时（{max_wait}s）")

if __name__ == "__main__":
    tid = create_task(prompt="一只金色柴犬在樱花树下奔跑，镜头缓缓上升，电影级画质",
                      metadata={"resolution": "1080p", "ratio": "16:9", "duration": 5})
    download_url = wait_for_result(tid)
    print("视频地址：", download_url)
```

### 5.2 图生视频 / 首尾帧（多模态）

```python
# 图生视频
task_id = create_task(
    prompt="镜头缓缓推进，猫微微转头，毛发随风轻动",
    image="https://example.com/cat.jpg",
    metadata={"resolution": "720p", "ratio": "16:9", "duration": 5},
)

# 首尾帧
task_id = create_task(
    prompt="镜头从首帧缓缓推进，自然过渡到尾帧",
    metadata={
        "resolution": "1080p", "ratio": "16:9", "duration": 5,
        "content": [
            {"type": "image_url", "image_url": {"url": "https://example.com/first.jpg"}, "role": "first_frame"},
            {"type": "image_url", "image_url": {"url": "https://example.com/last.jpg"}, "role": "last_frame"},
        ],
    },
)
video_url = wait_for_result(task_id)
```

---

## 六、余额查询

- **控制台查看（推荐）**：登录控制台 →「钱包」查看剩余额度与用量明细。
- **API 查询（程序化，OpenAI 兼容）**：

```bash
curl '<BaseURL>/v1/dashboard/billing/subscription' \
  -H 'Authorization: Bearer sk-xxxxxxxx'
```

返回示例：
```json
{"object":"billing_subscription","has_payment_method":true,"hard_limit_usd":100.0,"access_until":0}
```

> `hard_limit_usd` 反映账户剩余额度。余额不足将无法创建任务。

---

## 七、重要注意事项

| # | 事项 | 说明 |
| --- | --- | --- |
| 1 | **分辨率位置** | ⭐ `resolution` **必须**写在 `metadata` 内，否则按 720p 基准生成与计费（见 4.4） |
| 2 | **异步模式** | 所有视频生成均为异步，创建任务返回 `task_id` 后需轮询等待结果 |
| 3 | **轮询间隔** | 建议 **10 秒**一次，视频生成通常 **1~3 分钟**，偶发更长 |
| 4 | **图片 URL** | 图生视频支持公网 URL 或 Base64 Data URL（< 2MB） |
| 5 | **视频 URL** | 视频生视频的参考视频**必须是公网可访问的 URL**，不支持 Base64 |
| 6 | **role 参数** | `image_url`：`first_frame`/`last_frame`/`reference_image`；`video_url`：`reference_video`/`source_video`；`audio_url`：`reference_audio`。三种图片场景**互斥**，不能混用 |
| 7 | **真人图片** | 含真人照片可能被审核拒绝，建议使用非真人素材 |
| 8 | **视频下载** | 生成视频的下载地址**有时效性**（通常 24 小时），请及时下载保存；偶发签名失效（403）时重新生成一次即可 |
| 9 | **分辨率档位** | 支持 480p / 720p / 1080p / 4k；4k 并发较低（独享资源） |
| 10 | **创建超时** | 创建任务建议设置 **180 秒**超时 |
| 11 | **轮询超时** | 查询状态建议设置 **600 秒**超时，偶发超时重试即可 |
| 12 | **与国内版区别** | 海外版模型名为 `dreamina-seedance-*`，美元计价；国内版为 `doubao-seedance-*`。两者同一网关、互不干扰 |

### 最佳实践

1. **分辨率务必走 `metadata`**：这是计费准确的前提。
2. **图片优先用 URL**：Base64 会增加请求体积，建议上传至对象存储后用 URL。
3. **本地视频必须先上传**：视频文件较大，务必上传后用公网 URL 调用。
4. **及时下载结果**：视频地址有时效限制，生成后立即下载。
5. **错误重试**：轮询偶发超时属正常，重试即可，不影响任务执行。
6. **Prompt 技巧**：描述越具体、画面感越强，效果越好；避免抽象或模糊描述。

---

## 八、常见错误码

| HTTP / code | 含义 | 原因与解决 |
| --- | --- | --- |
| `401` Unauthorized | 认证失败 | API Key 错误 / 未填 / 失效 |
| `400` Bad Request | 请求格式错误 | 检查 `model`、`prompt`、`metadata` 结构；确认 `resolution` 在 `metadata` 内 |
| `404` Not Found | 路径错误 | 确认使用 `/v1/video/generations` |
| `408` / `504` Timeout | 请求超时 | 创建任务超时，重试即可 |
| `429` Too Many Requests | 限流 | 降低调用频率（4k 并发更低） |
| `InputImageSensitiveContentDetected` | 图片被内容审核拦截 | 改用非真人 / 合规图片 |
| `TokenModelForbidden` | 令牌无该模型权限 | 联系平台开通 |

---

## 九、联系方式

对接过程中如有问题，或需调整模型权限 / 计费档位，请联系平台技术支持。

— 完 —
