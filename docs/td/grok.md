grok-imagine-video-1.5 API接入文档
支持文生视频、单图首帧、首尾帧、多参考图。


目录
快速开始
认证
可用模型
请求参数
文生视频
图生视频
查询任务
完整调用示例
错误码
实际请求土豆的路径是 POST /v1/videos/generations 提交、 GET /v1/tasks/{task_id} 轮询。 图片必须是公网可访问 URL，不接受本地路径或 base64。
快速开始
# 1. 提交文生视频
curl -X POST https://api.ai-tudou.net/v1/videos/generations \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "grok-imagine-video-1.5",
    "prompt": "一只橘猫在阳光草地上追蝴蝶，温暖治愈",
    "duration": 6,
    "aspect_ratio": "16:9",
    "resolution": "720p",
    "extra": {"aspect_ratio": "16:9", "resolution": "720p"}
  }'

# 2. 轮询任务
curl https://api.ai-tudou.net/v1/tasks/{task_id} \
  -H "Authorization: Bearer YOUR_API_KEY"

# 3. 任务 completed 后，直接从 data.result.video_url 下载视频
curl -L "VIDEO_URL" -o out.mp4
认证
所有接口均需在请求头携带 API Key：

请求头	格式
Authorization	Bearer YOUR_API_KEY
可用模型
模型	说明
grok-imagine-video-1.5	文生 / 单图首帧 / 首尾帧 / 多参考图。时长 6 / 10 / 15 秒，分辨率仅 720p。
请求参数
POST https://api.ai-tudou.net/v1/videos/generations   application/json

参数	类型	必填	说明
model	string	是	固定填 grok-imagine-video-1.5
prompt	string	是	视频描述文本
duration	number	是	时长（秒），仅 6 / 10 / 15
aspect_ratio	string	是	16:9 / 9:16 / 1:1
resolution	string	否	仅支持 720p，建议同时写进 extra
extra	object	是	至少包含 aspect_ratio、resolution
input_reference	string / object	否	单张首帧图。字符串 URL，或 {"url":"..."}
extra.reference_images	array	否	首尾帧或多参考图。单图不要走这个字段
图片必须是公网 URL。不要用 extra.reference_images 传单图。
文生视频
curl -X POST https://api.ai-tudou.net/v1/videos/generations \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "grok-imagine-video-1.5",
    "prompt": "城市夜景延时，赛博朋克风格，流光溢彩",
    "duration": 10,
    "aspect_ratio": "16:9",
    "resolution": "720p",
    "extra": {
      "aspect_ratio": "16:9",
      "resolution": "720p"
    }
  }'
提交响应
{
  "code": 200,
  "message": "ok",
  "data": {
    "id": "task_3f2ae9...",
    "task_id": "task_3f2ae9...",
    "status": "submitted",
    "created": 1787591493,
    "estimated_time": 120
  }
}
任务 ID 取 data.id 或 data.task_id。部分中继也会把 ID 放在顶层 id。

图生视频
单张首帧 — 用 input_reference
{
  "model": "grok-imagine-video-1.5",
  "prompt": "人物抬头向前走",
  "duration": 10,
  "aspect_ratio": "16:9",
  "resolution": "720p",
  "input_reference": "https://example.com/first.jpg",
  "extra": {
    "aspect_ratio": "16:9",
    "resolution": "720p"
  }
}
input_reference 也支持对象写法：{"url":"https://example.com/first.jpg"}。

首尾帧（2 图）
{
  "model": "grok-imagine-video-1.5",
  "prompt": "自然过渡，保持主体一致",
  "duration": 10,
  "aspect_ratio": "16:9",
  "resolution": "720p",
  "extra": {
    "aspect_ratio": "16:9",
    "resolution": "720p",
    "reference_images": [
      {"url": "https://example.com/first.jpg", "role": "first_frame"},
      {"url": "https://example.com/last.jpg", "role": "last_frame"}
    ]
  }
}
多参考图（2~7 图）
{
  "model": "grok-imagine-video-1.5",
  "prompt": "保持人物身份和服装一致",
  "duration": 6,
  "aspect_ratio": "16:9",
  "resolution": "720p",
  "extra": {
    "aspect_ratio": "16:9",
    "resolution": "720p",
    "reference_images": [
      {"url": "https://example.com/ref1.jpg", "role": "reference_image"},
      {"url": "https://example.com/ref2.jpg", "role": "reference_image"}
    ]
  }
}
场景	字段	role
单张首帧	input_reference	—
首尾帧	extra.reference_images（2 张）	first_frame / last_frame
多参考图	extra.reference_images（2~7 张）	reference_image
查询任务
GET https://api.ai-tudou.net/v1/tasks/{task_id}

curl https://api.ai-tudou.net/v1/tasks/task_3f2ae9... \
  -H "Authorization: Bearer YOUR_API_KEY"
状态说明
status	含义
submitted / queued	已提交，等待处理
processing	生成中，progress 为 0–100
completed	完成，从 data.result 取视频 URL
failed	失败，见 data.error.message
状态流转
submitted → processing → completed
                 ↘ failed
进行中
{
  "code": 200,
  "message": "ok",
  "data": {
    "id": "task_3f2ae9...",
    "status": "processing",
    "progress": 45
  }
}
完成
{
  "code": 200,
  "message": "ok",
  "data": {
    "id": "task_3f2ae9...",
    "status": "completed",
    "progress": 100,
    "result": {
      "video_url": "https://videos.ai-tudou.net/generated/task_3f2ae9..._0.mp4",
      "videos": [{
        "url": ["https://videos.ai-tudou.net/generated/task_3f2ae9..._0.mp4"],
        "expires_at": 1787677893
      }]
    }
  }
}
视频 URL 约 24 小时有效，完成后请及时下载。建议提交后等 5 秒再首次查询，之后每 10 秒轮询一次。
完整调用示例
Python — 文生视频
import requests
import time

API_BASE = "https://api.ai-tudou.net"
HEADERS = {
    "Authorization": "Bearer YOUR_API_KEY",
    "Content-Type": "application/json",
}

resp = requests.post(
    f"{API_BASE}/v1/videos/generations",
    headers=HEADERS,
    json={
        "model": "grok-imagine-video-1.5",
        "prompt": "一只橘猫在阳光草地上追蝴蝶，温暖治愈",
        "duration": 6,
        "aspect_ratio": "16:9",
        "resolution": "720p",
        "extra": {"aspect_ratio": "16:9", "resolution": "720p"},
    },
    timeout=120,
)
payload = resp.json()
task_id = payload.get("id") or payload.get("data", {}).get("id")
print("任务已提交:", task_id)

time.sleep(5)
while True:
    data = requests.get(f"{API_BASE}/v1/tasks/{task_id}", headers=HEADERS, timeout=60).json()
    item = data.get("data") or data
    status = item.get("status")
    print("状态:", status, "进度:", item.get("progress"))
    if status == "completed":
        result = item.get("result") or {}
        video_url = result.get("video_url")
        if not video_url:
            videos = result.get("videos") or []
            first = videos[0] if videos else {}
            urls = first.get("url") if isinstance(first, dict) else None
            video_url = urls[0] if isinstance(urls, list) else urls
        open("output.mp4", "wb").write(requests.get(video_url, timeout=120).content)
        print("已保存 output.mp4")
        break
    if status == "failed":
        print("失败:", item.get("error"))
        break
    time.sleep(10)
Python — 单图首帧
resp = requests.post(
    f"{API_BASE}/v1/videos/generations",
    headers=HEADERS,
    json={
        "model": "grok-imagine-video-1.5",
        "prompt": "人物抬头向前走",
        "duration": 10,
        "aspect_ratio": "16:9",
        "resolution": "720p",
        "input_reference": "https://example.com/first.jpg",
        "extra": {"aspect_ratio": "16:9", "resolution": "720p"},
    },
    timeout=120,
)
错误码
{
  "error": {
    "message": "duration must be at least 6 seconds",
    "type": "invalid_request_error"
  }
}
HTTP	场景
400	参数错误：时长不是 6/10/15、分辨率不是 720p、比例非法、单图误走 reference_images
401	API Key 无效或缺失
404	模型名错误或任务不存在
429	请求过频
5xx	网关或上游异常