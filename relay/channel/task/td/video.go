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
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// ============================
// 视频分支：校验
// ============================

// isVideoRequest 判断当前请求是否走视频分支。
//
// 判别依据（按可靠性排序）：
//  1. 请求路径为视频提交入口 —— 最可靠，由路由直接决定；
//  2. 请求体含视频专有字段（videos / audios / generate_audio / aspect_ratio 等）；
//  3. 模型名命中 videoModelSpecs —— 覆盖纯文生视频（无任何视频专有字段）的情形。
//
// 注意：模型名判定必须放在最后，因为图片模型（gpt-image-*）不会命中 videoModelSpecs，
// 而视频模型即使请求体只有 prompt+duration 也应判为视频。
func isVideoRequest(c *gin.Context, model string) bool {
	// 1. 路径判定：/v1/videos/generations 入口
	if path := c.Request.URL.Path; strings.HasPrefix(path, "/v1/videos/") {
		return true
	}
	// 3. 模型判定：命中视频能力矩阵
	if _, ok := lookupVideoModelSpec(model); ok {
		return true
	}
	return false
}

// resolveVideoModelName 解析校验阶段用于查表的模型名。
//
// 为什么需要它：RelayTaskSubmit 的执行顺序是
//
//	步骤1 ValidateRequestAndSetAction   ← 校验发生在这里
//	步骤2.5 helper.ModelMappedHelper    ← 模型映射发生在这里
//
// 即校验早于映射，此时 info.UpstreamModelName 尚为空。而 videoModelSpecs 的 key
// 是**映射后的上游模型名**，直接读 info.UpstreamModelName 会拿到空串而误拒
// （用户配了 model_mapping 时尤其明显）。
//
// 因此这里提前按 c 中的 model_mapping 解析出上游名，语义与
// helper.ModelMappedHelper 保持一致（含链式重定向与环检测），避免两处逻辑漂移。
func resolveVideoModelName(c *gin.Context, info *relaycommon.RelayInfo, reqModel string) string {
	// 复核顺序与 relay_task.go 步骤 2 一致：OriginModelName 优先，回退请求体模型名
	model := info.OriginModelName
	if strings.TrimSpace(model) == "" {
		model = reqModel
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return ""
	}

	// 已在映射后（remix / 重试等路径可能已填充），直接使用
	if strings.TrimSpace(info.UpstreamModelName) != "" && info.IsModelMapped {
		return strings.TrimSpace(info.UpstreamModelName)
	}

	mapping := c.GetString("model_mapping")
	if mapping == "" || mapping == "{}" {
		return model
	}

	var modelMap map[string]string
	if err := common.UnmarshalJsonStr(mapping, &modelMap); err != nil {
		return model
	}

	// 链式映射解析，与 helper.ModelMappedHelper 同构
	current := model
	visited := map[string]bool{current: true}
	for {
		mapped, exists := modelMap[current]
		if !exists || mapped == "" {
			break
		}
		if visited[mapped] {
			// 环：与 ModelMappedHelper 一致，回到起点则视为未映射
			break
		}
		visited[mapped] = true
		current = mapped
	}
	return current
}

// validateVideoRequest 校验视频请求并归一化。
//
// 所有规则均来自 videoModelSpecs —— 模型不支持的能力（videos / audios /
// generate_audio）与未知模型统一在此拒绝，不静默透传给上游。
func (a *TaskAdaptor) validateVideoRequest(
	c *gin.Context,
	info *relaycommon.RelayInfo,
	req *VideoGenerateRequest,
) *dto.TaskError {
	badRequest := func(format string, args ...any) *dto.TaskError {
		return service.TaskErrorWrapperLocal(
			fmt.Errorf(format, args...), "invalid_request", http.StatusBadRequest,
		)
	}

	if strings.TrimSpace(req.Prompt) == "" {
		return badRequest("prompt is required")
	}

	model := resolveVideoModelName(c, info, req.Model)
	if model == "" {
		return badRequest("model is required")
	}

	spec, ok := lookupVideoModelSpec(model)
	if !ok {
		// 未知模型拒绝而非放行：避免参数边界无法校验时静默透传，
		// 用户误以为参数已生效。新增模型需在 videoModelSpecs 登记。
		return badRequest("unsupported video model: %s", model)
	}

	// 时长。
	// 枚举模式（AllowedDurations 非空，如 grok 的 6/10/15）优先于范围模式。
	// RequireDuration 为真时不允许缺省（grok 上游文档标 duration 必填）。
	// Duration 为指针，故「未传」与「显式传 0」可区分 —— 后者不在枚举集合
	// 或低于下限，一律被拒，不会被误当成缺省。
	switch {
	case len(spec.AllowedDurations) > 0:
		if req.Duration == nil {
			if spec.RequireDuration {
				return badRequest(
					"duration is required, model %s supports: %s seconds",
					model, spec.durationHint(),
				)
			}
			d := spec.DefaultDuration
			req.Duration = &d
		} else if !spec.supportsDuration(*req.Duration) {
			return badRequest(
				"invalid duration: %d, model %s supports: %s seconds",
				*req.Duration, model, spec.durationHint(),
			)
		}
	default:
		duration := spec.DefaultDuration
		if req.Duration != nil {
			if !spec.supportsDuration(*req.Duration) {
				return badRequest(
					"invalid duration: %d, model %s supports %s seconds",
					*req.Duration, model, spec.durationHint(),
				)
			}
			duration = *req.Duration
		}
		req.Duration = &duration
	}

	// 画面比例：缺省 16:9。
	// 校验按模型区分（spec.AllowedAspectRatios），未声明的模型回退通用集合。
	if strings.TrimSpace(req.AspectRatio) == "" {
		req.AspectRatio = AspectRatio16x9
	}
	if !spec.supportsAspectRatio(req.AspectRatio) {
		return badRequest(
			"invalid aspect_ratio: %s, must be one of: %s",
			req.AspectRatio, spec.aspectRatioHint(),
		)
	}

	// 分辨率：缺省首个可用档位（三模型均为 720p）
	if strings.TrimSpace(req.Resolution) == "" {
		req.Resolution = spec.AllowedResolutions[0]
	}
	if !spec.supportsResolution(req.Resolution) {
		return badRequest(
			"invalid resolution: %s, model %s supports: %s",
			req.Resolution, model, spec.resolutionHint(),
		)
	}

	// 素材数量：0 表示该模型不支持此类素材
	if len(req.Images) > spec.MaxImages {
		if spec.MaxImages == 0 {
			return badRequest("model %s does not support images", model)
		}
		return badRequest(
			"too many reference images: %d, model %s allows at most %d",
			len(req.Images), model, spec.MaxImages,
		)
	}
	if len(req.Videos) > spec.MaxVideos {
		if spec.MaxVideos == 0 {
			return badRequest("model %s does not support videos", model)
		}
		return badRequest(
			"too many reference videos: %d, model %s allows at most %d",
			len(req.Videos), model, spec.MaxVideos,
		)
	}
	if len(req.Audios) > spec.MaxAudios {
		if spec.MaxAudios == 0 {
			return badRequest("model %s does not support audios", model)
		}
		return badRequest(
			"too many reference audios: %d, model %s allows at most %d",
			len(req.Audios), model, spec.MaxAudios,
		)
	}

	// 音频不能单独使用：必须有图片或视频作为画面参考
	if len(req.Audios) > 0 && len(req.Images) == 0 && len(req.Videos) == 0 {
		return badRequest("audios cannot be used alone, at least one image or video is required")
	}

	// 音效开关：模型不支持时拒绝（如 minimax-h3 文档未列该参数）
	if req.GenerateAudio != nil && !spec.AllowGenerateAudio {
		return badRequest("model %s does not support generate_audio", model)
	}

	// extra.reference_images 校验。
	// 仅当模型声明了该通道上限（MaxExtraReferenceImages > 0）时进入 ——
	// 存量三模型该字段为 0，天然跳过本段，行为完全不受影响。
	if spec.MaxExtraReferenceImages > 0 {
		refs := extractReferenceImages(req.Extra)
		// 上游约束：单图必须走 input_reference，误用 extra.reference_images 会 400。
		// 提前拦截并给出明确指引，避免用户拿到上游的晦涩报错。
		if len(refs) == 1 {
			return badRequest(
				"single reference image must use input_reference, " +
					"not extra.reference_images (upstream rejects this)",
			)
		}
		if len(refs) > spec.MaxExtraReferenceImages {
			return badRequest(
				"too many reference images: %d, model %s allows at most %d in extra.reference_images",
				len(refs), model, spec.MaxExtraReferenceImages,
			)
		}
	}

	// 记录解析后的上游模型名，供后续建体与查询使用
	info.UpstreamModelName = model
	if info.OriginModelName == "" {
		info.OriginModelName = model
	}

	return nil
}

// extractReferenceImages 从 extra.reference_images 安全取出参考图条目。
//
// extra 是 map[string]any（上游字段集仍在演进），故不能假设元素类型：
// 上游文档中元素为 {"url":..., "role":...} 对象，但为稳健起见同时兼容
// 纯字符串写法。无法识别的元素**不计入**（宁可少算也不误报超限）。
//
// 返回 nil 表示未提供该字段或形态不符。
func extractReferenceImages(extra map[string]any) []any {
	if extra == nil {
		return nil
	}
	raw, ok := extra[ExtraKeyReferenceImages]
	if !ok || raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case []any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			switch item.(type) {
			case map[string]any, string:
				out = append(out, item)
			}
		}
		return out
	case map[string]any:
		// 单元素对象写法：上游要求单图走 input_reference，
		// 这里计入 1 以便触发「单图误用」提示。
		return []any{v}
	default:
		return nil
	}
}

// ============================
// 视频分支：解析
// ============================

// parseVideoTaskResult 解析视频任务查询响应。
// 结果地址优先取 result.video_url，回退 result.videos[0].url[0]。
func parseVideoTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var queryResp VideoQueryTaskResponse
	if err := common.Unmarshal(respBody, &queryResp); err != nil {
		return nil, err
	}
	if queryResp.Code != 200 {
		return nil, fmt.Errorf("query task failed, code: %d", queryResp.Code)
	}

	taskInfo := &relaycommon.TaskInfo{
		Status: convertTudouStatus(queryResp.Data.Status),
	}

	// 实际出片时长：供完成时的按秒计费重算使用。
	// 解析不出时为 0，调用方据此放弃重算（拿不到实际值就不动钱）。
	if seconds, ok := coerceSeconds(queryResp.Data.Seconds); ok {
		taskInfo.Seconds = seconds
	}

	if queryResp.Data.Status == StatusCompleted {
		taskInfo.Url = queryResp.Data.Result.FirstVideoURL()
	}
	if queryResp.Data.Status == StatusFailed && queryResp.Data.Error != nil {
		taskInfo.Reason = queryResp.Data.Error.Message
	}

	return taskInfo, nil
}

// unmarshalVideoRequest 解析视频请求体。
// 复用 body 存储以支持重试时重复读取（与 ValidateRequestAndSetAction 一致）。
func unmarshalVideoRequest(c *gin.Context, req *VideoGenerateRequest) error {
	return common.UnmarshalBodyReusable(c, req)
}

// ============================
// 视频分支：构建上游请求体
// ============================

// isVideoTaskResponse 按查询响应体结构判定是否为视频结果。
//
// 实现已下沉到 taskcommon.IsVideoTaskResponse，此处仅做转发。
//
// 为什么下沉：service 包的 OSS 转存判定也需要同一份判据（决定走图片还是视频
// 转存开关），但 td 包已导入 service 包，service 无法反向导入 td（循环依赖）。
// 把判据放在两者都可依赖的 taskcommon，保证**单一数据源**——判据变更时
// 不会出现两处实现不同步的问题。
func isVideoTaskResponse(respBody []byte) bool {
	return taskcommon.IsVideoTaskResponse(respBody)
}

// buildVideoRequestBody 构建视频上游请求体。
//
// 校验阶段已把模型名解析进 info.UpstreamModelName、并补齐了 duration /
// aspect_ratio / resolution 的默认值，这里只需落定模型名并序列化。
func (a *TaskAdaptor) buildVideoRequestBody(
	c *gin.Context,
	info *relaycommon.RelayInfo,
	req *VideoGenerateRequest,
) (io.Reader, error) {
	// 模型名：优先映射后的上游模型名（与图片分支一致）
	model := info.UpstreamModelName
	if strings.TrimSpace(model) == "" {
		model = req.Model
	}
	if strings.TrimSpace(model) == "" {
		return nil, fmt.Errorf("model is required")
	}

	upstreamReq := *req
	upstreamReq.Model = model

	// 兜底默认值：正常路径已在校验阶段填充，此处防御直接调用 BuildRequestBody 的场景
	spec, ok := lookupVideoModelSpec(model)
	if !ok {
		return nil, fmt.Errorf("unsupported video model: %s", model)
	}
	if upstreamReq.Duration == nil {
		d := spec.DefaultDuration
		upstreamReq.Duration = &d
	}
	if strings.TrimSpace(upstreamReq.AspectRatio) == "" {
		upstreamReq.AspectRatio = AspectRatio16x9
	}
	if strings.TrimSpace(upstreamReq.Resolution) == "" {
		upstreamReq.Resolution = spec.AllowedResolutions[0]
	}

	data, err := common.Marshal(upstreamReq)
	if err != nil {
		return nil, err
	}

	logger.LogInfo(c, fmt.Sprintf("Td video upstream request: %s", string(data)))

	// 保存到 context，供 LogTaskConsumption 写入 other["request_body"] / ["request_path"]
	c.Set(string(constant.ContextKeyVideoRequestBody), string(data))
	c.Set(string(constant.ContextKeyVideoRequestPath), EndpointVideoGenerate)

	return bytes.NewReader(data), nil
}

// ============================
// context 访问器
// ============================
// getVideoRequest 从 context 取视频请求。
// task_request 是多态的（图片为 TaskSubmitReq，视频为 VideoGenerateRequest），
// 故用类型断言判别分支，而不是另设标志位 —— 避免标志位与实体不一致。
func getVideoRequest(c *gin.Context) (*VideoGenerateRequest, bool) {
	v, exists := c.Get("task_request")
	if !exists {
		return nil, false
	}
	req, ok := v.(VideoGenerateRequest)
	if !ok {
		return nil, false
	}
	return &req, true
}
