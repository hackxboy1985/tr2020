package zy

// 上游接口路径（图片生成与视频生成共用 /v1/videos，通过 model 区分）
const (
	// EndpointSubmit 异步提交任务
	EndpointSubmit = "/v1/videos"
	// EndpointQueryTask 查询任务状态，需拼接 /{task_id}
	EndpointQueryTask = "/v1/videos/"
)

// ChannelName 渠道名称
const ChannelName = "Zy"

// MaxReferenceImages 上游允许的最大参考图数量
const MaxReferenceImages = 8

// 上游任务状态
const (
	StatusQueued     = "queued"
	StatusInProgress = "in_progress"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
)

// ModelList 上游文档支持的模型（档位写在 model 名里，或由参数选档）
var ModelList = []string{
	"gpt-image-2",
	"gpt-image-2-2K",
	"gpt-image-2-4K",
	"gpt-image-2.5",
	"gpt-image-2.5-flare",
	"gpt-image-2.5-sunburst",
}
