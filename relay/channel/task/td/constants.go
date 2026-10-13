package td

const (
	EndpointGenerateAsync = "/v1/images/generations/async"
	// EndpointVideoGenerate 视频任务提交入口。
	// 三个视频模型（minimax-h3 / seedance-2.0 / seedance-2.5）共用同一路径，
	// 差异只体现在参数取值边界上，由 videoModelSpecs 描述。
	EndpointVideoGenerate = "/v1/videos/generations"
	EndpointQueryTask     = "/v1/tasks/"
	EndpointBatchQuery    = "/v1/tasks/batch"
)

// 视频模型名（上游模型名 / videoModelSpecs 的 key）
const (
	VideoModelMiniMaxH3  = "minimax-h3"
	VideoModelSeedance20 = "seedance-2.0"
	VideoModelSeedance25 = "seedance-2.5"
	// VideoModelGrokImagineVideo15 grok 视频模型。
	// 与前三者路径相同（/v1/videos/generations + /v1/tasks/{id}），
	// 但参数协议不同（extra 必填、duration 枚举、支持 1:1），
	// 详见 docs/channel/td-grok-video-design.md。
	VideoModelGrokImagineVideo15 = "grok-imagine-video-1.5"
)

// 视频参数取值
const (
	Resolution720P    = "720p"
	Resolution1080P   = "1080p"
	Resolution2KVideo = "2k"

	AspectRatio16x9 = "16:9"
	AspectRatio9x16 = "9:16"
	// AspectRatio1x1 grok 专有比例，另外三个视频模型不支持。
	AspectRatio1x1 = "1:1"
)

const (
	StatusSubmitted  = "submitted"
	StatusQueued     = "queued"
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
)

// extra 协议相关
const (
	// ExtraKeyReferenceImages grok 通过 extra.reference_images 传首尾帧/多参考图。
	ExtraKeyReferenceImages = "reference_images"
	// ExtraKeyAspectRatio / ExtraKeyResolution grok 要求 extra 至少含这两项。
	ExtraKeyAspectRatio = "aspect_ratio"
	ExtraKeyResolution  = "resolution"
)

// Resolution 清晰度档位
const (
	Resolution1K = "1k"
	Resolution2K = "2k"
	Resolution4K = "4k"
	Resolution8K = "8k"
)

// Quality 质量档位
const (
	QualityLow    = "low"
	QualityMedium = "medium"
	QualityHigh   = "high"
)
