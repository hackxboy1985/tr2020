package td

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
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

	// 获取并验证 resolution（默认 1k）。
	// 归一化为小写写回 req，使计费注入与上游请求体统一使用小写字面量，
	// 避免用户传 4K 时校验通过却在 param("resolution") 匹配不上。
	resolution := req.Resolution
	if strings.TrimSpace(resolution) == "" {
		resolution = Resolution1K
	}
	if !isValidResolution(resolution) {
		return service.TaskErrorWrapperLocal(
			fmt.Errorf("invalid resolution: %s, must be one of: 1k, 2k, 4k, 8k", resolution),
			"invalid_request",
			http.StatusBadRequest,
		)
	}
	req.Resolution = NormalizeResolution(resolution)

	// quality 不做本地白名单校验：上游对档位的支持会变化（例如 xhigh），
	// 本地拒绝会误伤上游已支持的值，交由上游校验并返回其原始错误。
	// normalizeQuality 仍会把 standard/hd 转换为上游格式，未知值原样透传。

	c.Set("task_request", req)
	info.Action = constant.TaskActionGenerate
	return nil
}

// InjectBillingParams 在计费表达式求值前注入 resolution 与 quality，
// 便于管理员用 param("resolution") / param("quality") 按档位定价。
//
// 为什么必须显式注入：/v1/images/tasks 入口下计费发生在 RelayTaskSubmit 内部，
// 若不注入则退回读取原始 HTTP body；而 /v1/images/generations 入口的 body 结构与
// task 入口不同（resolution 是非标准字段），会导致 param() 取不到值而落到兜底档位。
// 显式注入使两个入口的计费输入完全一致，与 Zy / RR 渠道的做法对齐。
//
// 注入"用户原始值"而非归一化值：
//   - quality 保留 xhigh / max 等原始档位，管理员才能按档位区分定价
//     （若归一化会把 hd→high，与用户直接传 high 无法区分）
//   - resolution 原样下发，1k/2k/4k 与表达式中的字面量保持一致
func (a *TaskAdaptor) InjectBillingParams(c *gin.Context, info *relaycommon.RelayInfo) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return
	}

	if info.BillingRequestInput == nil {
		info.BillingRequestInput = &billingexpr.RequestInput{Body: []byte("{}")}
	}

	body := info.BillingRequestInput.Body

	resolution := NormalizeResolution(req.Resolution)
	body = billingexpr.InjectBodyParam(body, "resolution", resolution)

	quality := strings.TrimSpace(req.Quality)
	if quality == "" {
		quality = QualityMedium
	}
	body = billingexpr.InjectBodyParam(body, "quality", quality)

	info.BillingRequestInput.Body = body
}

// BuildRequestURL 构建上游URL
func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	fullUrl := fmt.Sprintf("%s%s", a.baseURL, EndpointGenerateAsync)
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

	// 模型名：优先取模型映射后的上游模型名，未映射时回退到请求体中的模型名
	model := info.UpstreamModelName
	if model == "" {
		model = req.Model
	}
	if strings.TrimSpace(model) == "" {
		return nil, fmt.Errorf("model is required")
	}

	// 构建上游请求
	upstreamReq := GenerateRequest{
		Model:      model,
		Prompt:     req.Prompt,
		Size:       req.Size,
		Resolution: NormalizeResolution(req.Resolution),
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

	// 始终打印上游请求体，便于排查异步生图请求
	logger.LogInfo(c, fmt.Sprintf("Td upstream request: %s", string(data)))

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

	// 返回任务提交响应给用户：公开 task_id 与 submitted 状态。
	// 说明：本 adaptor 是唯一写响应体的一方，覆盖两个入口——
	//   /v1/images/tasks          → controller.RelayTask（其成功分支不写响应体）
	//   /v1/images/generations    → handleTudouImageTask（不再自行写响应体，避免重复写入）
	c.JSON(http.StatusOK, gin.H{
		"id":      info.PublicTaskID,
		"object":  "image.task",
		"status":  "submitted",
		"model":   info.OriginModelName,
		"created": time.Now().Unix(),
	})

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

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, err
	}
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

// NormalizeResolution 归一化分辨率档位为 Td 要求的小写形式 1k/2k/4k/8k。
// 上游与计费表达式均以小写字面量为准，因此下游传大写（如 4K/8K）时必须转换，
// 否则会出现「校验通过但 param("resolution") 匹配不上」而落到兜底档位。
// 兼容任意大小写与首尾空白；空值返回 1k（Td 默认档）。
func NormalizeResolution(resolution string) string {
	switch strings.ToUpper(strings.TrimSpace(resolution)) {
	case "1K":
		return Resolution1K
	case "2K":
		return Resolution2K
	case "4K":
		return Resolution4K
	case "8K":
		return Resolution8K
	default:
		return Resolution1K
	}
}

// isValidResolution 校验分辨率档位是否合法（大小写不敏感，空值合法取默认 1k）
func isValidResolution(r string) bool {
	switch strings.ToUpper(strings.TrimSpace(r)) {
	case "", "1K", "2K", "4K", "8K":
		return true
	}
	return false
}

func convertTudouStatus(status string) string {
	switch status {
	case StatusSubmitted:
		return string(model.TaskStatusSubmitted)
	case StatusProcessing:
		return string(model.TaskStatusInProgress)
	case StatusCompleted:
		return string(model.TaskStatusSuccess)
	case StatusFailed:
		return string(model.TaskStatusFailure)
	default:
		return string(model.TaskStatusUnknown)
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
