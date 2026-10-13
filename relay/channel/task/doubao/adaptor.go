package doubao

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
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
		VideoURL     string `json:"video_url"`
		LastFrameURL string `json:"last_frame_url,omitempty"`
	} `json:"content"`
	// Metadata 兼容下游为 new-api 时返回的 OpenAIVideo 格式（metadata.url 存视频地址）
	Metadata              map[string]interface{} `json:"metadata,omitempty"`
	Seed                  int                    `json:"seed,omitempty"`
	Resolution            string                 `json:"resolution,omitempty"`
	Duration              int                    `json:"duration,omitempty"`
	Frames                int                    `json:"frames,omitempty"`
	Ratio                 string                 `json:"ratio,omitempty"`
	FramesPerSecond       int                    `json:"framespersecond,omitempty"`
	ServiceTier           string                 `json:"service_tier,omitempty"`
	GenerateAudio         bool                   `json:"generate_audio,omitempty"`
	OutputFormat          string                 `json:"output_format,omitempty"`
	Draft                 bool                   `json:"draft,omitempty"`
	DraftTaskID           string                 `json:"draft_task_id,omitempty"`
	SafetyIdentifier      string                 `json:"safety_identifier,omitempty"`
	ExecutionExpiresAfter int                    `json:"execution_expires_after,omitempty"`
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
	ChannelType       int
	apiKey            string
	baseURL           string
	videoGeneratePath string
	videoFetchPath    string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = info.ChannelBaseUrl
	a.apiKey = info.ApiKey

	common.SysLog(fmt.Sprintf("[Init] 开始初始化 adaptor, ChannelType=%d", info.ChannelType))

	if info.ChannelMeta == nil {
		common.SysLog("[Init] WARNING: info.ChannelMeta 是 nil!")
		return
	}

	common.SysLog("[Init] ChannelMeta 不为空，继续初始化")
	settings := info.ChannelMeta.ChannelOtherSettings

	common.SysLog(fmt.Sprintf("[Init] asset_upstream_version=%s", settings.AssetUpstreamVersion))
	common.SysLog(fmt.Sprintf("[Init] doubao_video_generate_path=%s", settings.DoubaoVideoGeneratePath))
	common.SysLog(fmt.Sprintf("[Init] doubao_video_fetch_path=%s", settings.DoubaoVideoFetchPath))

	// 优先使用显式配置的路径
	a.videoGeneratePath = settings.DoubaoVideoGeneratePath
	a.videoFetchPath = settings.DoubaoVideoFetchPath

	// 如果没有显式配置，根据 asset_upstream_version 自动推断
	upstreamVersion := settings.AssetUpstreamVersion
	if upstreamVersion == "" {
		upstreamVersion = "gateway" // 默认
	}

	common.SysLog(fmt.Sprintf("[Init] 使用 upstream_version=%s 进行路径推断", upstreamVersion))

	switch upstreamVersion {
	case "kwjm":
		// KWJM 上游使用简化路径
		if a.videoGeneratePath == "" {
			a.videoGeneratePath = "/v1/videos/generations"
			common.SysLog("[Init] 推断 videoGeneratePath=/v1/videos/generations")
		}
		if a.videoFetchPath == "" {
			a.videoFetchPath = "/v1/videos/generations"
			common.SysLog("[Init] 推断 videoFetchPath=/v1/videos/generations")
		}
	case "gateway":
		fallthrough
	default:
		// Gateway 上游使用完整路径
		if a.videoGeneratePath == "" {
			a.videoGeneratePath = "/api/v3/contents/generations/tasks"
			common.SysLog("[Init] 推断 videoGeneratePath=/api/v3/contents/generations/tasks")
		}
		if a.videoFetchPath == "" {
			a.videoFetchPath = "/api/v3/contents/generations/tasks"
			common.SysLog("[Init] 推断 videoFetchPath=/api/v3/contents/generations/tasks")
		}
	}

	common.SysLog(fmt.Sprintf("[Init] 最终 videoGeneratePath=%s", a.videoGeneratePath))
	common.SysLog(fmt.Sprintf("[Init] 最终 videoFetchPath=%s", a.videoFetchPath))
}

// ValidateRequestAndSetAction parses body, validates fields and sets default action.
func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) (taskErr *dto.TaskError) {
	// Accept only POST /v1/video/generations as "generate" action.
	if taskErr = relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionGenerate); taskErr != nil {
		return taskErr
	}

	// 分辨率是计价的必要输入：解析不到（缺失或无法识别）直接拒绝请求，
	// 不猜测、不兜底。解析结果缓存到 context，供 EstimateBilling 复用，
	// 保证校验与计费使用同一个值。
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return service.TaskErrorWrapper(err, "get_task_request_failed", http.StatusBadRequest)
	}
	resolution := ResolveVideoResolution(&req)
	if resolution == "" {
		return service.TaskErrorWrapperLocal(
			fmt.Errorf("resolution is required and must be one of: 480p, 720p, 1080p, 4k"),
			"invalid_resolution", http.StatusBadRequest)
	}
	c.Set(contextKeyResolvedResolution, resolution)
	return nil
}

// contextKeyResolvedResolution 缓存校验阶段解析出的分辨率，
// 供 EstimateBilling 复用，避免二次解析产生不一致。
const contextKeyResolvedResolution = "doubao_resolved_resolution"

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

// EstimateBilling 按「模型 + 输出分辨率 + 输入是否含视频」计算视频计费倍率。
//
// 计费基准：管理员将 ModelRatio / ModelPrice 配置为「480p/720p + 不含视频」标准价，
// 即基准档不含视频价。relay 层用「管理员配的基准价 × 本函数返回的倍率」得出最终价，
// 因此本函数的倍率分母必须取「基准档不含视频价」，才能与管理员配的基准价精确约分：
//
//	倍率 = 该档目标价 / 基准档(720p)不含视频价
//	     = 该档目标价 / 管理员配的基准价
//
// 目标价按「有无视频输入」取该档的含视频价或不含视频价。
// 因为分母已含分辨率溢价（如 1080p 的 77 相对 720p 的 70），
// 单键即可同时表达「分辨率档位」与「视频输入折扣」两个维度，
// 无需额外输出 resolution 键。
//
// 以 doubao-seedance-2.5（管理员基准价 70）为例：
//
//	480p/720p 不含视频 → 70/70 = 1.0      → 70
//	1080p     不含视频 → 77/70 ≈ 1.1000   → 77
//	480p/720p 含视频   → 42/70 = 0.6      → 42
//	1080p     含视频   → 46/70 ≈ 0.6571   → 46
//
// 注意键名 video_input 是历史命名：返回值为「该档目标价相对基准档基准价的倍率」，
// 已包含分辨率因素，并非单纯的视频输入折扣。
//
// 分辨率从两条路径解析：
//   - OpenAI 兼容路径：/v1/videos 的 size / metadata.size
//   - 官方 Ark 原生路径：/api/v3/contents/generations/tasks 的根级 resolution / metadata.resolution
func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil
	}

	// 计费一律使用原始模型名（OriginModelName），与基础费率保持一致：
	// relay_task.go:181 在 ModelMappedHelper 之后显式把 OriginModelName 复原为
	// 用户请求的模型名，ModelPriceHelperPerCall（price.go:175）即按该名查基础费率。
	// 本函数返回的倍率是乘在该基础费率上的，必须查同一套命名，否则会语义错位。
	//
	// 以下为使用上游（映射后）模型名的旧实现，暂时保留备查，不再启用：
	// modelName := info.OriginModelName
	// if info.ChannelMeta != nil && info.ChannelMeta.UpstreamModelName != "" {
	// 	modelName = info.ChannelMeta.UpstreamModelName
	// }
	modelName := info.OriginModelName

	// 1) 取输出分辨率：复用 ValidateRequestAndSetAction 校验阶段解析并缓存的结果，
	//    保证校验与计费使用同一个值（校验阶段已确保非空，否则请求不会走到这里）。
	resolution := c.GetString(contextKeyResolvedResolution)

	// 2) 解析是否携带视频输入（metadata.content[].video_url / metadata.video_url）
	hasVideoInput := hasVideoInMetadata(req.Metadata)

	// 模型未配置视频价格表时返回 nil —— 不附加任何倍率，走原有基础费率计费。
	// ok=false 的唯一原因是 videoPriceMap 没有该模型，与传入的分辨率无关，
	// 因此只需查询一次（基准价由下方按需查询）。
	priceWithoutVideo, priceWithVideo, ok := GetVideoResolutionPrice(modelName, resolution)
	if !ok {
		return nil
	}

	// 基准档价格：作为倍率的分母，必须与管理员配置的基准价（720p 不含视频价）
	// 代表同一个值，才能在下游精确约分得到该档目标价。
	// 基准档缺失或非正说明价格表配置错误：此时无法折算，返回 nil 走基础费率，
	// 避免除以 0 得到 +Inf 导致预扣费失控。
	baselineWithoutVideo, _, baselineOK := GetVideoResolutionPrice(modelName, ResolutionBaseline)
	if !baselineOK || baselineWithoutVideo <= 0 {
		return nil
	}

	// 目标价：按是否携带视频输入，取该分辨率档的含视频价或不含视频价。
	var targetPrice float64
	if hasVideoInput {
		targetPrice = priceWithVideo
	} else {
		targetPrice = priceWithoutVideo
	}

	ratio := targetPrice / baselineWithoutVideo
	// 同时打印原始模型名与上游（映射后）模型名：
	// 实际查表用的是 OriginModelName，UpstreamModelName 仅作对照便于排查映射配置。
	upstreamModelName := ""
	if info.ChannelMeta != nil {
		upstreamModelName = info.ChannelMeta.UpstreamModelName
	}
	// 日志写成 "video_input=46/70=0.657143" 的形式直接展示分子分母，
	// 便于一眼看出倍率取自哪个分辨率档、是否含视频，无需回查价格表。
	// 用 %.0f 打印价格，避免 46.00 这类冗余小数影响可读性。
	common.SysLog(fmt.Sprintf("[Doubao Video] EstimateBilling: originModel=%s, upstreamModel=%s, resolution=%s, hasVideoInput=%t, video_input=%.0f/%.0f=%.6f",
		modelName, upstreamModelName, resolution, hasVideoInput, targetPrice, baselineWithoutVideo, ratio))

	return map[string]float64{"video_input": ratio}
}

// almostEqualFloat 浮点近似相等判断（用于跳过 1.0 的中性倍率）。
func almostEqualFloat(a, b float64) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff < 1e-9
}

// hasVideoInMetadata 检查 metadata 中是否包含视频输入（video_url 条目）。
// 支持两种承载位置：
//   - metadata.content[] 数组（Ark 原生格式）
//   - metadata.video_url 扁平字段
//
// 注意：metadata["content"] 存在两种动态类型 ——
// ValidateBasicTaskRequest 写入的是 []map[string]interface{}（Go 原生），
// 而 JSON 反序列化得到的是 []interface{}。两者都要处理，否则 Ark 原生
// 格式的视频输入会漏判。
func hasVideoInMetadata(metadata map[string]interface{}) bool {
	if metadata == nil {
		return false
	}
	if v, exists := metadata["video_url"]; exists && v != nil && v != "" {
		return true
	}
	contentRaw, ok := metadata["content"]
	if !ok {
		return false
	}

	switch contentSlice := contentRaw.(type) {
	case []map[string]interface{}:
		for _, item := range contentSlice {
			if contentItemHasVideo(item) {
				return true
			}
		}
	case []interface{}:
		for _, item := range contentSlice {
			itemMap, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			if contentItemHasVideo(itemMap) {
				return true
			}
		}
	}
	return false
}

// contentItemHasVideo 判断单个 content 条目是否为视频素材。
func contentItemHasVideo(item map[string]interface{}) bool {
	if item == nil {
		return false
	}
	if item["type"] == "video_url" {
		return true
	}
	if _, has := item["video_url"]; has {
		return true
	}
	return false
}

// responseResolution 从上游响应的嵌套字段中提取分辨率。
// 部分上游把分辨率放在 metadata.resolution / content.resolution 中。
func responseResolution(resTask responseTask) string {
	if resTask.Metadata != nil {
		if r := resolveResolutionFromMetadata(resTask.Metadata); r != "" {
			return r
		}
	}
	return ""
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

	// 打印上游请求 URL
	upstreamPath := a.videoGeneratePath
	if upstreamPath == "" {
		upstreamPath = "/api/v3/contents/generations/tasks"
	}
	upstreamURL := fmt.Sprintf("%s%s", a.baseURL, upstreamPath)

	// 只在渠道ID为8时打印调试信息
	if info.ChannelId == 8 {
		common.SysLog(fmt.Sprintf("[Doubao Video] ========== 上游请求信息 =========="))
		common.SysLog(fmt.Sprintf("[Doubao Video] Channel ID: %d", info.ChannelId))
		common.SysLog(fmt.Sprintf("[Doubao Video] Base URL: %s", a.baseURL))
		common.SysLog(fmt.Sprintf("[Doubao Video] Generate Path: %s", a.videoGeneratePath))
		common.SysLog(fmt.Sprintf("[Doubao Video] Fetch Path: %s", a.videoFetchPath))
		common.SysLog(fmt.Sprintf("[Doubao Video] 完整上游 URL: %s", upstreamURL))
		common.SysLog(fmt.Sprintf("[Doubao Video] 原始模型名: %s", info.OriginModelName))
		common.SysLog(fmt.Sprintf("[Doubao Video] 上游模型名: %s", info.UpstreamModelName))
		common.SysLog(fmt.Sprintf("[Doubao Video] 是否模型映射: %v", info.IsModelMapped))
		common.SysLog(fmt.Sprintf("[Doubao Video] 请求体: %s", string(data)))
		common.SysLog(fmt.Sprintf("[Doubao Video] ====================================="))
	}

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

	common.SysLog(fmt.Sprintf("[FetchTask] 构造的请求URL: %s", uri))
	common.SysLog(fmt.Sprintf("[FetchTask] baseUrl=%s, fetchPath=%s, taskID=%s", baseUrl, fetchPath, taskID))

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

	resp, err := client.Do(req)
	if err != nil {
		common.SysLog(fmt.Sprintf("[FetchTask] 请求失败: %v", err))
		return nil, err
	}

	common.SysLog(fmt.Sprintf("[FetchTask] 响应状态码: %d", resp.StatusCode))

	return resp, nil
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

	// 分辨率兜底：metadata 未携带 resolution 时，从请求的其它写法补齐（如
	// 根级 resolution 或 content[] 内的 resolution），确保上游请求与计费同一口径。
	if strings.TrimSpace(r.Resolution) == "" {
		if resolution := ResolveVideoResolution(req); resolution != "" {
			r.Resolution = resolution
		}
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

	fmt.Printf("[CancelTask] 上游取消请求 - URI: %s, Method: DELETE\n", uri)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("[CancelTask] 上游请求失败: %v\n", err)
		return errors.Wrap(err, "cancel request failed")
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "read cancel response failed")
	}

	fmt.Printf("[CancelTask] 上游响应 - StatusCode: %d, Body: %s\n", resp.StatusCode, string(responseBody))

	// 根据文档，成功时返回 HTTP 200，响应体为 {}
	if resp.StatusCode == http.StatusOK {
		fmt.Printf("[CancelTask] 取消成功\n")
		return nil
	}

	// 处理错误响应
	// 任务不存在: HTTP 404
	if resp.StatusCode == http.StatusNotFound {
		fmt.Printf("[CancelTask] 任务不存在 (404)\n")
		return errors.New("task_not_exist")
	}

	// 任务运行中: HTTP 409
	if resp.StatusCode == http.StatusConflict {
		fmt.Printf("[CancelTask] 任务不可取消 (409)，开始解析错误响应\n")
		// 尝试两种可能的响应格式

		// 格式1: 嵌套格式 {"error": {"code": "...", "message": "..."}}
		var nestedResp struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
				Param   string `json:"param"`
				Type    string `json:"type"`
			} `json:"error"`
		}

		// 格式2: 扁平格式 {"code": "...", "message": "..."}
		var flatResp struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Param   string `json:"param"`
			Type    string `json:"type"`
		}

		// 先尝试扁平格式，再尝试嵌套格式
		errorCode := ""
		if err := common.Unmarshal(responseBody, &flatResp); err == nil && flatResp.Code != "" {
			fmt.Printf("[CancelTask] 扁平格式解析成功，错误码: %s\n", flatResp.Code)
			errorCode = flatResp.Code
		} else if err := common.Unmarshal(responseBody, &nestedResp); err == nil && nestedResp.Error.Code != "" {
			fmt.Printf("[CancelTask] 嵌套格式解析成功，错误码: %s\n", nestedResp.Error.Code)
			errorCode = nestedResp.Error.Code
		} else {
			fmt.Printf("[CancelTask] JSON解析失败或错误码为空\n")
		}

		// 根据错误码返回用户友好的消息，避免泄露上游任务ID
		if errorCode != "" {
			switch errorCode {
			case "InvalidAction.RunningTaskDeletion":
				fmt.Printf("[CancelTask] 匹配到RunningTaskDeletion，返回友好消息\n")
				return errors.New("task is currently running, cannot be cancelled")
			default:
				fmt.Printf("[CancelTask] 错误码: %s，返回通用消息\n", errorCode)
				return errors.New("task cannot be cancelled at this time")
			}
		}

		fmt.Printf("[CancelTask] 无法解析错误码，返回默认消息\n")
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
	} else if nested := responseResolution(dResp); nested != "" {
		openAIVideo.SetMetadata("resolution", nested)
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
