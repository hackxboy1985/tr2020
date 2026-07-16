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
)

var ModelList = []string{
	"poster-generate",
	"poster-free-creation",
}
