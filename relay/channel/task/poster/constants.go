package poster

const (
	ChannelName = "poster"

	EndpointGenerateAsync   = "/openapi/v1/poster/generateAsync"
	EndpointFreeCreation    = "/openapi/v1/poster/allAroundCreation"
	EndpointQueryTaskResult = "/openapi/v1/poster/queryTaskResult"
)

const (
	TaskStatusRunning = "RUNNING"
	TaskStatusSuccess = "SUCCESS"
	TaskStatusFailed  = "FAILED"

	// 数字状态（上游实际返回）
	TaskStatusRunningNum = "1"
	TaskStatusSuccessNum = "2"
	TaskStatusFailedNum  = "3"
)

var ModelList = []string{
	"poster-generate",
	"poster-free-creation",
}
