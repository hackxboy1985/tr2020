package doubao

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/samber/lo"
)

// ============================
// Request / Response structures
// ============================

type ContentItem struct {
	Type     string    `json:"type,omitempty"`
	Text     string    `json:"text,omitempty"`
	ImageURL *MediaURL `json:"image_url,omitempty"`
	VideoURL *MediaURL `json:"video_url,omitempty"`
	AudioURL *MediaURL `json:"audio_url,omitempty"`
	Role     string    `json:"role,omitempty"`
}

type MediaURL struct {
	URL string `json:"url,omitempty"`
}

type requestPayload struct {
	Model                 string         `json:"model"`
	Content               []ContentItem  `json:"content,omitempty"`
	CallbackURL           string         `json:"callback_url,omitempty"`
	ReturnLastFrame       *dto.BoolValue `json:"return_last_frame,omitempty"`
	ServiceTier           string         `json:"service_tier,omitempty"`
	ExecutionExpiresAfter *dto.IntValue  `json:"execution_expires_after,omitempty"`
	GenerateAudio         *dto.BoolValue `json:"generate_audio,omitempty"`
	Draft                 *dto.BoolValue `json:"draft,omitempty"`
	Tools                 []struct {
		Type string `json:"type,omitempty"`
	} `json:"tools,omitempty"`
	Resolution  string         `json:"resolution,omitempty"`
	Ratio       string         `json:"ratio,omitempty"`
	Duration    *dto.IntValue  `json:"duration,omitempty"`
	Frames      *dto.IntValue  `json:"frames,omitempty"`
	Seed        *dto.IntValue  `json:"seed,omitempty"`
	CameraFixed *dto.BoolValue `json:"camera_fixed,omitempty"`
	Watermark   *dto.BoolValue `json:"watermark,omitempty"`
}

type responsePayload struct {
	ID string `json:"id"` // task_id
}

type responseTask struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Status  string `json:"status"`
	Content struct {
		VideoURL      string `json:"video_url"`
		LastFrameURL  string `json:"last_frame_url,omitempty"`
	} `json:"content"`
	// Metadata 兼容下游为 new-api 时返回的 OpenAIVideo 格式（metadata.url 存视频地址）
	Metadata               map[string]interface{} `json:"metadata,omitempty"`
	Seed                   int                    `json:"seed,omitempty"`
	Resolution             string                 `json:"resolution,omitempty"`
	Duration               int                    `json:"duration,omitempty"`
	Frames                 int                    `json:"frames,omitempty"`
	Ratio                  string                 `json:"ratio,omitempty"`
	FramesPerSecond        int                    `json:"framespersecond,omitempty"`
	ServiceTier            string                 `json:"service_tier,omitempty"`
	GenerateAudio          bool                   `json:"generate_audio,omitempty"`
	OutputFormat           string                 `json:"output_format,omitempty"`
	Draft                  bool                   `json:"draft,omitempty"`
	DraftTaskID            string                 `json:"draft_task_id,omitempty"`
	SafetyIdentifier       string                 `json:"safety_identifier,omitempty"`
	ExecutionExpiresAfter  int                    `json:"execution_expires_after,omitempty"`
	// UpstreamTaskID 是豆包内部的源头任务 ID（如 cgt-... 格式）
	UpstreamTaskID string `json:"upstream_task_id,omitempty"`
	Tools          []struct {
		Type string `json:"type"`
	} `json:"tools,omitempty"`
	Usage struct {
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
		ToolUsage        struct {
			WebSearch int `json:"web_search,omitempty"`
		} `json:"tool_usage,omitempty"`
	} `json:"usage"`
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at"`
}

// ============================
// Adaptor implementation
// ============================

type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType          int
	apiKey               string
	baseURL              string
	videoGeneratePath    string
	videoFetchPath       string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = info.ChannelBaseUrl
	a.apiKey = info.ApiKey
	if info.ChannelMeta != nil {
		a.videoGeneratePath = info.ChannelMeta.ChannelOtherSettings.DoubaoVideoGeneratePath
		a.videoFetchPath = info.ChannelMeta.ChannelOtherSettings.DoubaoVideoFetchPath
	}
}

// ValidateRequestAndSetAction parses body, validates fields and sets default action.
func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) (taskErr *dto.TaskError) {
	// Accept only POST /v1/video/generations as "generate" action.
	return relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionGenerate)
}

// BuildRequestURL constructs the upstream URL.
func (a *TaskAdaptor) BuildRequestURL(_ *relaycommon.RelayInfo) (string, error) {
	path := a.videoGeneratePath
	if path == "" {
		path = "/api/v3/contents/generations/tasks"
	}
	return fmt.Sprintf("%s%s", a.baseURL, path), nil
}

// BuildRequestHeader sets required headers.
func (a *TaskAdaptor) BuildRequestHeader(_ *gin.Context, req *http.Request, _ *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	return nil
}

// EstimateBilling 检测请求 metadata 中是否包含视频输入，返回视频折扣 OtherRatio。
func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil
	}
	if hasVideoInMetadata(req.Metadata) {
		if ratio, ok := GetVideoInputRatio(info.OriginModelName); ok {
			return map[string]float64{"video_input": ratio}
		}
	}
	return nil
}

// hasVideoInMetadata 直接检查 metadata 的 content 数组是否包含 video_url 条目，
// 避免构建完整的上游 requestPayload。
func hasVideoInMetadata(metadata map[string]interface{}) bool {
	if metadata == nil {
		return false
	}
	contentRaw, ok := metadata["content"]
	if !ok {
		return false
	}
	contentSlice, ok := contentRaw.([]interface{})
	if !ok {
		return false
	}
	for _, item := range contentSlice {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if itemMap["type"] == "video_url" {
			return true
		}
		if _, has := itemMap["video_url"]; has {
			return true
		}
	}
	return false
}

// BuildRequestBody converts request into Doubao specific format.
func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, err
	}

	body, err := a.convertToRequestPayload(&req)
	if err != nil {
		return nil, errors.Wrap(err, "convert request payload failed")
	}
	if info.IsModelMapped {
		body.Model = info.UpstreamModelName
	} else {
		info.UpstreamModelName = body.Model
	}
	data, err := common.Marshal(body)
	if err != nil {
		return nil, err
	}

	// 打印最终发往上游的请求体，方便排查转换问题
	logger.LogInfo(c, fmt.Sprintf("doubao video upstream request body: %s", data))
	// 保存到 context，供 savePrompt 写入 prompt_logs.request_body
	c.Set(string(constant.ContextKeyVideoRequestBody), string(data))

	return bytes.NewReader(data), nil
}

// DoRequest delegates to common helper.
func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

// DoResponse handles upstream response, returns taskID etc.
func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}
	_ = resp.Body.Close()

	// Parse Doubao response
	var dResp responsePayload
	if err := common.Unmarshal(responseBody, &dResp); err != nil {
		taskErr = service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
		return
	}

	if dResp.ID == "" {
		taskErr = service.TaskErrorWrapper(fmt.Errorf("task_id is empty"), "invalid_response", http.StatusInternalServerError)
		return
	}

	// Check if should return Doubao official format
	if c.GetBool("doubao_official_format") {
		// Return Doubao official format: just {"id": "task_id"}
		c.JSON(http.StatusOK, responsePayload{ID: dResp.ID})
	} else {
		// Return OpenAI Video format (default)
		ov := dto.NewOpenAIVideo()
		ov.ID = info.PublicTaskID
		ov.TaskID = info.PublicTaskID
		ov.CreatedAt = time.Now().Unix()
		ov.Model = info.OriginModelName
		c.JSON(http.StatusOK, ov)
	}

	c.Set(string(constant.ContextKeyVideoResponseBody), string(responseBody))
	return dResp.ID, responseBody, nil
}

// FetchTask fetch task status
func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid task_id")
	}

	fetchPath := a.videoFetchPath
	if fetchPath == "" {
		fetchPath = "/api/v3/contents/generations/tasks"
	}
	uri := fmt.Sprintf("%s%s/%s", baseUrl, fetchPath, taskID)

	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}

func (a *TaskAdaptor) convertToRequestPayload(req *relaycommon.TaskSubmitReq) (*requestPayload, error) {
	r := requestPayload{
		Model:   req.Model,
		Content: []ContentItem{},
	}

	metadata := req.Metadata
	if err := taskcommon.UnmarshalMetadata(metadata, &r); err != nil {
		return nil, errors.Wrap(err, "unmarshal metadata failed")
	}

	// Re-merge image items extracted during parsing.
	// UnmarshalMetadata may overwrite r.Content with metadata["content"] (audio/video only),
	// so images from req.Images must be prepended afterwards to preserve correct order.
	if req.HasImage() {
		var imageItems []ContentItem
		for _, imgURL := range req.Images {
			imageItems = append(imageItems, ContentItem{
				Type:     "image_url",
				ImageURL: &MediaURL{URL: imgURL},
			})
		}
		r.Content = append(imageItems, r.Content...)
	}

	// Doubao reference-media mode requires role fields for media content items.
	for i := range r.Content {
		if (r.Content[i].Type == "audio_url" || r.Content[i].AudioURL != nil) && r.Content[i].Role == "" {
			r.Content[i].Role = "reference_audio"
		}
		if (r.Content[i].Type == "image_url" || r.Content[i].ImageURL != nil) && r.Content[i].Role == "" {
			r.Content[i].Role = "reference_image"
		}
		if (r.Content[i].Type == "video_url" || r.Content[i].VideoURL != nil) && r.Content[i].Role == "" {
			r.Content[i].Role = "reference_video"
		}
	}

	if sec, _ := strconv.Atoi(req.Seconds); sec > 0 {
		r.Duration = lo.ToPtr(dto.IntValue(sec))
	} else if req.Duration > 0 {
		r.Duration = lo.ToPtr(dto.IntValue(req.Duration))
	}

	r.Content = lo.Reject(r.Content, func(c ContentItem, _ int) bool { return c.Type == "text" })
	r.Content = append(r.Content, ContentItem{
		Type: "text",
		Text: req.Prompt,
	})

	return &r, nil
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	resTask := responseTask{}
	if err := common.Unmarshal(respBody, &resTask); err != nil {
		return nil, errors.Wrap(err, "unmarshal task result failed")
	}

	taskResult := relaycommon.TaskInfo{
		Code:      0,
		OriTaskID: resTask.UpstreamTaskID,
	}

	// Map Doubao status to internal status
	switch resTask.Status {
	case "pending", "queued":
		taskResult.Status = model.TaskStatusQueued
		taskResult.Progress = "10%"
	case "processing", "running":
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "50%"
	case "succeeded":
		taskResult.Status = model.TaskStatusSuccess
		taskResult.Progress = "100%"
		taskResult.Url = resTask.Content.VideoURL
		// 解析 usage 信息用于按倍率计费
		taskResult.CompletionTokens = resTask.Usage.CompletionTokens
		taskResult.TotalTokens = resTask.Usage.TotalTokens
	// 兼容下游为 new-api 时返回的 OpenAIVideo 状态值
	case "completed":
		taskResult.Status = model.TaskStatusSuccess
		taskResult.Progress = "100%"
		taskResult.Url = resTask.Content.VideoURL
		if taskResult.Url == "" {
			if u, ok := resTask.Metadata["url"].(string); ok {
				taskResult.Url = u
			}
		}
	case "in_progress":
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "50%"
	case "failed":
		taskResult.Status = model.TaskStatusFailure
		taskResult.Progress = "100%"
		taskResult.Reason = resTask.Error.Message
	case "cancelled":
		taskResult.Status = model.TaskStatusCancelled
		taskResult.Progress = "100%"
		taskResult.Reason = "cancelled by user"
	default:
		// Unknown status, treat as processing
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "30%"
	}

	return &taskResult, nil
}

// CancelTask cancels a task by calling upstream DELETE API
func (a *TaskAdaptor) CancelTask(upstreamTaskID string) error {
	fetchPath := a.videoFetchPath
	if fetchPath == "" {
		fetchPath = "/api/v3/contents/generations/tasks"
	}
	uri := fmt.Sprintf("%s%s/%s", a.baseURL, fetchPath, upstreamTaskID)

	req, err := http.NewRequest(http.MethodDelete, uri, nil)
	if err != nil {
		return errors.Wrap(err, "create cancel request failed")
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return errors.Wrap(err, "cancel request failed")
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "read cancel response failed")
	}

	// 根据文档，成功时返回 HTTP 200，响应体为 {}
	if resp.StatusCode == http.StatusOK {
		return nil
	}

	// 处理错误响应
	// 任务不存在: HTTP 404
	if resp.StatusCode == http.StatusNotFound {
		return errors.New("task_not_exist")
	}

	// 任务运行中: HTTP 409
	if resp.StatusCode == http.StatusConflict {
		var errResp struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
				Param   string `json:"param"`
				Type    string `json:"type"`
			} `json:"error"`
		}
		if err := common.Unmarshal(responseBody, &errResp); err == nil && errResp.Error.Message != "" {
			return errors.New(errResp.Error.Message)
		}
		return errors.New("task is running, cannot cancel")
	}

	// 其他错误
	return errors.Errorf("cancel task failed with status %d: %s", resp.StatusCode, string(responseBody))
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(originTask *model.Task) ([]byte, error) {
	var dResp responseTask
	if err := common.Unmarshal(originTask.Data, &dResp); err != nil {
		return nil, errors.Wrap(err, "unmarshal doubao task data failed")
	}

	openAIVideo := dto.NewOpenAIVideo()
	openAIVideo.ID = originTask.TaskID
	openAIVideo.TaskID = originTask.TaskID
	openAIVideo.Status = originTask.Status.ToVideoStatus()
	openAIVideo.SetProgressStr(originTask.Progress)
	openAIVideo.CreatedAt = originTask.CreatedAt
	openAIVideo.CompletedAt = originTask.UpdatedAt
	openAIVideo.Model = originTask.Properties.OriginModelName

	// Set metadata with all doubao-specific fields
	openAIVideo.SetMetadata("url", dResp.Content.VideoURL)
	if dResp.Content.VideoURL == "" {
		if u, ok := dResp.Metadata["url"].(string); ok {
			openAIVideo.SetMetadata("url", u)
		}
	}

	if dResp.Content.LastFrameURL != "" {
		openAIVideo.SetMetadata("last_frame_url", dResp.Content.LastFrameURL)
	}
	if dResp.UpdatedAt > 0 {
		openAIVideo.SetMetadata("updated_at", dResp.UpdatedAt)
	}
	if dResp.Duration > 0 {
		openAIVideo.SetMetadata("duration", dResp.Duration)
	}
	if dResp.Ratio != "" {
		openAIVideo.SetMetadata("ratio", dResp.Ratio)
	}
	if dResp.Resolution != "" {
		openAIVideo.SetMetadata("resolution", dResp.Resolution)
	}
	if dResp.FramesPerSecond > 0 {
		openAIVideo.SetMetadata("framespersecond", dResp.FramesPerSecond)
	}
	if dResp.Seed > 0 {
		openAIVideo.SetMetadata("seed", dResp.Seed)
	}
	if dResp.OutputFormat != "" {
		openAIVideo.SetMetadata("output_format", dResp.OutputFormat)
	}
	if dResp.ServiceTier != "" {
		openAIVideo.SetMetadata("service_tier", dResp.ServiceTier)
	}
	openAIVideo.SetMetadata("generate_audio", dResp.GenerateAudio)
	openAIVideo.SetMetadata("draft", dResp.Draft)
	if dResp.DraftTaskID != "" {
		openAIVideo.SetMetadata("draft_task_id", dResp.DraftTaskID)
	}
	if dResp.ExecutionExpiresAfter > 0 {
		openAIVideo.SetMetadata("execution_expires_after", dResp.ExecutionExpiresAfter)
	}

	if dResp.Status == "failed" {
		openAIVideo.Error = &dto.OpenAIVideoError{
			Message: dResp.Error.Message,
			Code:    dResp.Error.Code,
		}
	}

	if dResp.Usage.TotalTokens > 0 {
		openAIVideo.Usage = &dto.OpenAIVideoUsage{
			CompletionTokens: dResp.Usage.CompletionTokens,
			TotalTokens:      dResp.Usage.TotalTokens,
		}
	}

	return common.Marshal(openAIVideo)
}
