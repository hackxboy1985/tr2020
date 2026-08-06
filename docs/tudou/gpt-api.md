可用模型（异步）
异步请求固定使用统一模型 gpt-image-2-all。平台需配置「默认分组」；分辨率通过 resolution 控制，质量档位通过 quality 控制。

域名：domain.com -> api.ai.net
模型请求列表
模型名称	输出分辨率	质量档位	平台分组	说明
gpt-image-2-all	resolution：1k / 2k / 4k	quality：low / medium / high	default分组、默认分组	提交后返回 task_id，轮询查询结果；推荐用于 2K / 4K 和高并发场景。
请勿在模型名中追加分辨率或 task 后缀；固定传 gpt-image-2-all，分辨率通过 resolution 控制。
通用请求参数
字段	类型	默认值	说明
model	string	必填	固定传 gpt-image-2-all；异步请求需配置「default分组」。
prompt	string	必填	图像描述，支持中英文，建议详细描述
size	string	1:1	画面比例，如 1:1 16:9 3:2 等，见尺寸对照表；auto 按 1:1 处理；也可传像素如 2048x1152
resolution	string	必填	清晰度档位，支持 1k 2k 4k
quality	string	必填	质量档位，支持 low medium high
images	array<string>	可选	参考图数组，传入后走图生图模式。每一项支持公网 HTTP/HTTPS URL，或带 MIME 前缀的 base64：data:image/png;base64,...。不支持 raw base64。
size（比例）× resolution（清晰度）→ 实际像素
不同 resolution 档位在同一 size 比例下输出的像素如下。异步任务固定使用模型 gpt-image-2-all。

size（比例）	resolution=1k	resolution=2k	resolution=4k
1:1	1024×1024	2048×2048	2880×2880
3:2	1536×1024	2048×1360	3520×2336
2:3	1024×1536	1360×2048	2336×3520
4:3	1024×768	2048×1536	3312×2480
3:4	768×1024	1536×2048	2480×3312
5:4	1280×1024	2560×2048	3216×2576
4:5	1024×1280	2048×2560	2576×3216
16:9	1536×864	2048×1152	3840×2160
9:16	864×1536	1152×2048	2160×3840
2:1	2048×1024	2688×1344	3840×1920
1:2	1024×2048	1344×2688	1920×3840
3:1	1881×836	3072×1024	3840×1280
1:3	887×1774	1024×3072	1280×3840
21:9	2016×864	2688×1152	3840×1648
9:21	864×2016	1152×2688	1648×3840
异步任务
POST
https://domain.com/v1/images/generations/async
异步处理模式：提交后立即返回任务 ID，无需保持长连接，轮询查询结果即可。异步请求固定使用模型 gpt-image-2-all，并确保平台已配置「defalut分组」。

文生图（最简请求）
curl -X POST "https://domain.com/v1/images/generations/async" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2-all",
    "prompt": "一只橘猫坐在窗台上看夕阳，水彩画风格",
    "size": "1:1",
    "resolution": "1k",
    "quality": "medium"
  }'
文生图（指定比例 + 4K）
curl -X POST "https://domain.com/v1/images/generations/async" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2-all",
    "prompt": "星空下的古老城堡，电影感",
    "size": "16:9",
    "resolution": "4k",
    "quality": "high"
  }'
立即返回
{
  "code": 200,
  "data": {
    "id": "task_bSPHAaYDWZIUXM0YkfXgtiWUlPnGnare",
    "status": "submitted",
    "progress": 0,
    "created": 1780979971,
    "estimated_time": 100
  }
}
返回的 data.id 即任务 ID（task_ 前缀），用于后续查询。
图生图（异步）
在 JSON 请求体中传入 images 数组即触发图生图模式。参考图支持公网 HTTP/HTTPS URL，或 data:image/png;base64,... / data:image/jpeg;base64,...。不要传 raw base64，也不要用 multipart。

单张参考图（URL）
curl -X POST "https://domain.com/v1/images/generations/async" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2-all",
    "prompt": "把这张照片变成水彩画风格",
    "size": "4:3",
    "resolution": "2k",
    "quality": "medium",
    "images": ["https://example.net/photo.jpg"]
  }'
多张参考图融合（URL）
curl -X POST "https://domain.com/v1/images/generations/async" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2-all",
    "prompt": "把这两张照片融合成一张海报，电影感打光",
    "size": "16:9",
    "resolution": "4k",
    "quality": "high",
    "images": [
      "https://example.net/photo-a.jpg",
      "https://example.net/photo-b.jpg"
    ]
  }'
多张参考图融合（base64）
base64 必须带 data:image/png;base64, 或 data:image/jpeg;base64, 前缀。

curl -X POST "https://domain.com/v1/images/generations/async" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2-all",
    "prompt": "严格参考两张输入图，只生成由红色和蓝色组成的极简抽象图，不要动物，不要人物，不要文字",
    "size": "1:1",
    "resolution": "2k",
    "quality": "medium",
    "images": [
      "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAACAAAAAgCAYAAABzenr0AAAAAXNSR0IArs4c6QAAAARnQU1BAACxjwv8YQUAAAAJcEhZcwAADsMAAA7DAcdvqGQAAAAsSURBVFhH7c4hAQAADASh61/6F2MGgafVPgkICAgICAgICAgICAgICDwH2gFUDfhqkE3cCgAAAABJRU5ErkJggg==",
      "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAACAAAAAgCAYAAABzenr0AAAAAXNSR0IArs4c6QAAAARnQU1BAACxjwv8YQUAAAAJcEhZcwAADsMAAA7DAcdvqGQAAABTSURBVFhHxcghAQAwDASx+jf9MxB8AyG5u+0vZolZYpaYJWaJWWKWmCVmiVlilpglZolZYpaYJWaJWWKWmCVmiVlilpglZolZYpaYJWaJWWJWtgdblfhqbhbl8QAAAABJRU5ErkJggg=="
    ]
  }'
查询任务结果
异步任务提交后返回 task_id，使用查询接口轮询生成进度和结果。

查询单个任务
GET
https://domain.com/v1/tasks/{task_id}
curl "https://domain.com/v1/tasks/task_o90cQvQv0N21J47sjUtXztgqGGZohRrr" \
  -H "Authorization: Bearer YOUR_API_KEY"
处理中
{
  "code": 200,
  "data": {
    "id": "task_o90cQvQv0N21J47sjUtXztgqGGZohRrr",
    "status": "processing",
    "progress": 30,
    "created": 1780635359
  }
}
成功结果
{
  "code": 200,
  "data": {
    "id": "task_o90cQvQv0N21J47sjUtXztgqGGZohRrr",
    "status": "completed",
    "progress": 100,
    "created": 1780635359,
    "completed": 1780635413,
    "actual_time": 54,
    "result": {
      "images": [
        {
          "url": ["https://xxxxxx/image/xxxxxxxx_0.png"],
          "expires_at": 1780721813
        }
      ]
    }
  }
}
取图方式：data.result.images[0].url[0]。url 字段本身是数组。
批量查询任务
POST
https://domain.com/v1/tasks/batch
curl -X POST "https://domain.com/v1/tasks/batch" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "task_ids": ["task_xxx", "task_yyy"]
  }'
任务状态说明
status	含义
submitted	已提交
processing	上游处理中
completed	成功，result.images 可用
failed	失败，查看 error.message
轮询建议
首次查询延迟：提交后等 10–20s 再查
轮询间隔：3–5s 一次，避免无脑毫秒级轮询
取图：data.result.images[0].url[0]
错误与提示
失败时 data.status 为 failed，错误原因读取 data.error.message：

{
  "code": 200,
  "data": {
    "id": "task_xxx",
    "status": "failed",
    "error": {
      "code": "task_failed",
      "message": "moderation failed",
      "type": "task_failed"
    }
  }
}
场景	说明
模型分组未配置	异步请求需配置「默认分组」。
参数缺失或非法	请确认请求体包含 resolution 和 quality，且取值分别为 1k / 2k / 4k 与 low / medium / high
内容审核未通过	prompt 命中违规，已拒绝且不计费
鉴权失败	API Key 无效或额度不足，请联系管理员
参考图格式错误	images 每一项必须是公网 HTTP/HTTPS URL，或 data:image/png;base64,... / data:image/jpeg;base64,...；不支持 raw base64 和本地文件路径
结果链接过期	结果 URL 通常 24 小时后过期，请及时下载或转存到自己的存储
计费按 resolution 与 quality 对应档位区分，失败与审核未通过不计费。