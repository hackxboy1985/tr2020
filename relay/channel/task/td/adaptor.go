package td

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relay/channel"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// ============================
// Adaptor implementation
// ============================

type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType int
	apiKey      string
	baseURL     string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = info.ChannelBaseUrl
	a.apiKey = info.ApiKey
}

// ValidateRequestAndSetAction 验证请求参数
func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	var req relaycommon.TaskSubmitReq
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}

	// 验证必填参数
	if strings.TrimSpace(req.Prompt) == "" {
		return service.TaskErrorWrapperLocal(
			fmt.Errorf("prompt is required"),
			"invalid_request",
			http.StatusBadRequest,
		)
	}

	// 获取并验证 resolution（默认 1k）
	resolution := req.Resolution
	if resolution == "" {
		resolution = "1k"
	}
	if !isValidResolution(resolution) {
		return service.TaskErrorWrapperLocal(
			fmt.Errorf("invalid resolution: %s, must be one of: 1k, 2k, 4k", resolution),
			"invalid_request",
			http.StatusBadRequest,
		)
	}

	// 获取并验证 quality（默认 medium）
	quality := normalizeQuality(req.Quality)
	if !isValidQuality(quality) {
		return service.TaskErrorWrapperLocal(
			fmt.Errorf("invalid quality: %s, must be one of: low, medium, high, standard, hd", req.Quality),
			"invalid_request",
			http.StatusBadRequest,
		)
	}

	c.Set("task_request", req)
	info.Action = constant.TaskActionGenerate
	return nil
}

// BuildRequestURL 构建上游URL
func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	fullUrl := fmt.Sprintf("%s%s", a.baseURL, EndpointGenerateAsync)
	logger.LogInfo(nil, fmt.Sprintf("Td upstream request URL: %s", fullUrl))
	// 保存上游请求路径到 context
	return fullUrl, nil
}

// BuildRequestHeader 设置请求头
func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	return nil
}

// BuildRequestBody 构建请求体
func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	v, exists := c.Get("task_request")
	if !exists {
		return nil, fmt.Errorf("request not found in context")
	}
	req, ok := v.(relaycommon.TaskSubmitReq)
	if !ok {
		return nil, fmt.Errorf("invalid request type in context")
	}

	// 构建上游请求
	upstreamReq := GenerateRequest{
		Model:      "gpt-image-2-all", // 固定模型
		Prompt:     req.Prompt,
		Size:       req.Size,
		Resolution: req.Resolution,
		Quality:    normalizeQuality(req.Quality),
	}

	// 如果 Size 为空，使用 Ratio 字段
	if upstreamReq.Size == "" && req.Ratio != "" {
		upstreamReq.Size = req.Ratio
	}

	// 默认值
	if upstreamReq.Size == "" {
		upstreamReq.Size = "1:1"
	}
	if upstreamReq.Resolution == "" {
		upstreamReq.Resolution = "1k"
	}
	if upstreamReq.Quality == "" {
		upstreamReq.Quality = "medium"
	}

	// 处理参考图（图生图模式）
	if len(req.Images) > 0 {
		upstreamReq.Images = req.Images
	} else if len(req.InputReference) > 0 {
		upstreamReq.Images = req.InputReference
	}

	data, err := common.Marshal(upstreamReq)
	if err != nil {
		return nil, err
	}

	// 打印请求体用于调试
	logger.LogInfo(nil, fmt.Sprintf("Td upstream request: %s", string(data)))

	// 保存到 context，供 LogTaskConsumption 写入 other["request_body"]
	c.Set(string(constant.ContextKeyVideoRequestBody), string(data))
	// 保存上游请求路径到 context，供 LogTaskConsumption 写入 other["request_path"]
	c.Set(string(constant.ContextKeyVideoRequestPath), EndpointGenerateAsync)

	return bytes.NewReader(data), nil
}

// DoRequest 发送请求
func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

// DoResponse 解析上游响应
func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// 打印上游响应体用于调试
	logger.LogInfo(nil, fmt.Sprintf("Td upstream response: %s", string(responseBody)))

	var sResp SubmitResponse
	if err := common.Unmarshal(responseBody, &sResp); err != nil {
		taskErr = service.TaskErrorWrapper(err, "unmarshal_response_failed", http.StatusInternalServerError)
		return
	}

	if sResp.Code != 200 {
		taskErr = service.TaskErrorWrapperLocal(
			fmt.Errorf("upstream error code: %d", sResp.Code),
			"upstream_error",
			http.StatusInternalServerError,
		)
		return
	}

	if sResp.Data.ID == "" {
		taskErr = service.TaskErrorWrapperLocal(
			fmt.Errorf("task_id not found in response"),
			"invalid_response",
			http.StatusInternalServerError,
		)
		return
	}

	taskID = sResp.Data.ID
	taskData = responseBody

	// 保存响应体到 context，供 LogTaskConsumption 写入 other["response_body"]
	c.Set(string(constant.ContextKeyVideoResponseBody), string(responseBody))
	return
}

// GetModelList 返回支持的模型列表
func (a *TaskAdaptor) GetModelList() []string {
	return []string{"gpt-image-2-all"}
}

// GetChannelName 返回渠道名称
func (a *TaskAdaptor) GetChannelName() string {
	return "Td"
}

// FetchTask 查询任务状态（轮询使用）
func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok || taskID == "" {
		return nil, fmt.Errorf("task_id is required")
	}

	url := fmt.Sprintf("%s%s%s", baseUrl, EndpointQueryTask, taskID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	return client.Do(req)
}

// ParseTaskResult 解析任务查询结果
func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var queryResp QueryTaskResponse
	if err := common.Unmarshal(respBody, &queryResp); err != nil {
		return nil, err
	}

	if queryResp.Code != 200 {
		return nil, fmt.Errorf("query task failed, code: %d", queryResp.Code)
	}

	taskInfo := &relaycommon.TaskInfo{
		Status: convertTudouStatus(queryResp.Data.Status),
	}

	// 如果任务完成，提取结果URL
	if queryResp.Data.Status == StatusCompleted && queryResp.Data.Result != nil {
		if len(queryResp.Data.Result.Images) > 0 && len(queryResp.Data.Result.Images[0].URL) > 0 {
			taskInfo.Url = queryResp.Data.Result.Images[0].URL[0]
		}
	}

	// 如果任务失败，提取错误信息
	if queryResp.Data.Status == StatusFailed && queryResp.Data.Error != nil {
		taskInfo.Reason = queryResp.Data.Error.Message
	}

	return taskInfo, nil
}

// ============================
// Helper functions
// ============================

func getStringFromMetadata(metadata map[string]interface{}, key, defaultValue string) string {
	if metadata == nil {
		return defaultValue
	}
	if val, ok := metadata[key].(string); ok && val != "" {
		return val
	}
	return defaultValue
}

func isValidResolution(r string) bool {
	switch r {
	case Resolution1K, Resolution2K, Resolution4K:
		return true
	}
	return false
}

func isValidQuality(q string) bool {
	switch q {
	case QualityLow, QualityMedium, QualityHigh:
		return true
	}
	return false
}

func convertTudouStatus(status string) string {
	switch status {
	case StatusSubmitted:
		return "queued"
	case StatusProcessing:
		return "processing"
	case StatusCompleted:
		return "completed"
	case StatusFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// normalizeQuality 将 OpenAI 格式的 quality 转换为土豆平台格式
// OpenAI: "standard" | "hd"
// 土豆: "low" | "medium" | "high"
func normalizeQuality(quality string) string {
	switch strings.ToLower(quality) {
	case "standard":
		return QualityMedium
	case "hd":
		return QualityHigh
	case "low", "medium", "high":
		return strings.ToLower(quality)
	case "":
		return QualityMedium
	default:
		return quality // 返回原值，让验证函数处理
	}
}
