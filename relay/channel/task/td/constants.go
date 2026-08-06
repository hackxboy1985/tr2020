package td

const (
	EndpointGenerateAsync = "/v1/images/generations/async"
	EndpointQueryTask     = "/v1/tasks/"
	EndpointBatchQuery    = "/v1/tasks/batch"
)

const (
	StatusSubmitted = "submitted"
	StatusProcessing = "processing"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)

// Resolution 清晰度档位
const (
	Resolution1K = "1k"
	Resolution2K = "2k"
	Resolution4K = "4k"
)

// Quality 质量档位
const (
	QualityLow    = "low"
	QualityMedium = "medium"
	QualityHigh   = "high"
)
