package relay

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
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func ImageHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *types.NewAPIError) {
	info.InitChannelMeta(c)

	imageReq, ok := info.Request.(*dto.ImageRequest)
	if !ok {
		return types.NewErrorWithStatusCode(fmt.Errorf("invalid request type, expected dto.ImageRequest, got %T", info.Request), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	// RR is an async image channel: hand off to the task submit pipeline.
	if info.ChannelType == constant.ChannelTypeRR {
		return handleRRImageTask(c, info, imageReq)
	}

	request, err := common.DeepCopy(imageReq)
	if err != nil {
		return types.NewError(fmt.Errorf("failed to copy request to ImageRequest: %w", err), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}

	err = helper.ModelMappedHelper(c, info, request)
	if err != nil {
		return types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
	}

	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)

	var requestBody io.Reader

	if model_setting.GetGlobalSettings().PassThroughRequestEnabled || info.ChannelSetting.PassThroughBodyEnabled {
		storage, err := common.GetBodyStorage(c)
		if err != nil {
			return types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		}
		requestBody = common.ReaderOnly(storage)
	} else {
		convertedRequest, err := adaptor.ConvertImageRequest(c, info, *request)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed)
		}
		relaycommon.AppendRequestConversionFromRequest(info, convertedRequest)

		switch convertedRequest.(type) {
		case *bytes.Buffer:
			requestBody = convertedRequest.(io.Reader)
		default:
			jsonData, err := common.Marshal(convertedRequest)
			if err != nil {
				return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
			}

			// apply param override
			if len(info.ParamOverride) > 0 {
				jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
				if err != nil {
					return newAPIErrorFromParamOverride(err)
				}
			}

			logger.LogDebug(c, "image request body: %s", jsonData)
			body, size, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
			if err != nil {
				return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
			}
			defer closer.Close()
			jsonData = nil
			info.UpstreamRequestBodySize = size
			requestBody = body
		}
	}

	statusCodeMappingStr := c.GetString("status_code_mapping")

	resp, err := adaptor.DoRequest(c, info, requestBody)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}
	var httpResp *http.Response
	if resp != nil {
		httpResp = resp.(*http.Response)
		info.IsStream = info.IsStream || strings.HasPrefix(httpResp.Header.Get("Content-Type"), "text/event-stream")
		if httpResp.StatusCode != http.StatusOK {
			if httpResp.StatusCode == http.StatusCreated && info.ApiType == constant.APITypeReplicate {
				// replicate channel returns 201 Created when using Prefer: wait, treat it as success.
				httpResp.StatusCode = http.StatusOK
			} else {
				newAPIError = service.RelayErrorHandler(c.Request.Context(), httpResp, false)
				// reset status code 重置状态码
				service.ResetStatusCode(newAPIError, statusCodeMappingStr)
				return newAPIError
			}
		}
	}

	usage, newAPIError := adaptor.DoResponse(c, httpResp, info)
	if newAPIError != nil {
		// reset status code 重置状态码
		service.ResetStatusCode(newAPIError, statusCodeMappingStr)
		return newAPIError
	}

	imageN := uint(1)
	if request.N != nil {
		imageN = *request.N
	}

	// n is handled via OtherRatio so it is applied exactly once in quota
	// calculation (both price-based and ratio-based paths).
	// Adaptors may have already set a more accurate count from the
	// upstream response; only set the default when they haven't.
	if info.PriceData.UsePrice { // only price model use N ratio
		if _, hasN := info.PriceData.OtherRatios["n"]; !hasN {
			info.PriceData.AddOtherRatio("n", float64(imageN))
		}
	}

	if usage.(*dto.Usage).TotalTokens == 0 {
		usage.(*dto.Usage).TotalTokens = 1
	}
	if usage.(*dto.Usage).PromptTokens == 0 {
		usage.(*dto.Usage).PromptTokens = 1
	}

	quality := request.Quality
	if quality == "" {
		quality = "standard"
	}

	var logContent []string

	if len(request.Size) > 0 {
		logContent = append(logContent, fmt.Sprintf("大小 %s", request.Size))
	}
	if len(quality) > 0 {
		logContent = append(logContent, fmt.Sprintf("品质 %s", quality))
	}
	if imageN > 0 {
		logContent = append(logContent, fmt.Sprintf("生成数量 %d", imageN))
	}

	// 设置请求体 context key，供 GenerateTextOtherInfo 写入 other 及 savePrompt 写入 prompt_logs
	if bs, bsErr := common.GetBodyStorage(c); bsErr == nil {
		if _, seekErr := bs.Seek(0, io.SeekStart); seekErr == nil {
			if bodyBytes, readErr := io.ReadAll(bs); readErr == nil && len(bodyBytes) > 0 {
				c.Set(string(constant.ContextKeyVideoRequestBody), string(bodyBytes))
				_, _ = bs.Seek(0, io.SeekStart)
			}
		}
	}

	service.PostTextConsumeQuota(c, info, usage.(*dto.Usage), logContent)
	return nil
}

// convertImageRequestToTaskSubmitReq converts an OpenAI ImageRequest to a TaskSubmitReq
// so that the async task pipeline can process it.
func convertImageRequestToTaskSubmitReq(req *dto.ImageRequest) relaycommon.TaskSubmitReq {
	taskReq := relaycommon.TaskSubmitReq{
		Prompt: req.Prompt,
		Size:   req.Size,
		Mode:   req.Quality, // quality maps to Mode field; RR adaptor reads it via req.Mode
	}
	// Extract image URLs for image-to-image requests (user passes "images": [...])
	if len(req.Images) > 0 {
		var imgs []string
		if err := common.Unmarshal(req.Images, &imgs); err == nil {
			taskReq.Images = imgs
		}
	}
	return taskReq
}

// handleRRImageTask handles image generation for the RR async channel.
// It refunds the sync pre-consume done by Relay, then runs the full async task pipeline.
func handleRRImageTask(c *gin.Context, info *relaycommon.RelayInfo, imageReq *dto.ImageRequest) *types.NewAPIError {
	// Refund the pre-consume that Relay's sync billing path already charged.
	// RelayTaskSubmit will perform its own billing (ModelPriceHelperPerCall path).
	if info.Billing != nil {
		info.Billing.Refund(c)
		info.Billing = nil
	}

	// Convert and store task request in context so ValidateBasicTaskRequest (inside
	// RelayTaskSubmit → ValidateRequestAndSetAction) can read it.
	taskReq := convertImageRequestToTaskSubmitReq(imageReq)
	c.Set("task_request", taskReq)

	result, taskErr := RelayTaskSubmit(c, info)
	if taskErr != nil {
		return types.NewErrorWithStatusCode(taskErr.Error, types.ErrorCodeBadResponseStatusCode, taskErr.StatusCode)
	}

	// Settle billing and log consumption (mirrors RelayTask controller).
	if settleErr := service.SettleBilling(c, info, result.Quota); settleErr != nil {
		common.SysError("settle RR image task billing error: " + settleErr.Error())
	}

	// Store request body for logging.
	if bs, bsErr := common.GetBodyStorage(c); bsErr == nil {
		if _, seekErr := bs.Seek(0, io.SeekStart); seekErr == nil {
			if bodyBytes, readErr := io.ReadAll(bs); readErr == nil && len(bodyBytes) > 0 {
				c.Set(string(constant.ContextKeyVideoRequestBody), string(bodyBytes))
			}
		}
	}
	if taskReq.Prompt != "" {
		c.Set(string(constant.ContextKeyPromptToSave), taskReq.Prompt)
	}

	service.LogTaskConsumption(c, info)

	task := model.InitTask(result.Platform, info)
	task.PrivateData.UpstreamTaskID = result.UpstreamTaskID
	task.PrivateData.BillingSource = info.BillingSource
	task.PrivateData.SubscriptionId = info.SubscriptionId
	task.PrivateData.TokenId = info.TokenId
	task.PrivateData.BillingContext = &model.TaskBillingContext{
		ModelPrice:      info.PriceData.ModelPrice,
		GroupRatio:      info.PriceData.GroupRatioInfo.GroupRatio,
		ModelRatio:      info.PriceData.ModelRatio,
		OtherRatios:     info.PriceData.OtherRatios,
		OriginModelName: info.OriginModelName,
		PerCallBilling:  info.PriceData.UsePrice,
	}
	task.Quota = result.Quota
	task.Data = result.TaskData
	task.Action = info.Action
	task.Properties.Input = taskReq.Prompt
	if rb := c.GetString(string(constant.ContextKeyVideoRequestBody)); rb != "" {
		task.PrivateData.RequestBody = service.TruncateBody(rb)
	}
	if len(result.TaskData) > 0 {
		task.PrivateData.SubmitRespBody = service.TruncateBody(string(result.TaskData))
	}
	if insertErr := task.Insert(); insertErr != nil {
		common.SysError("insert RR image task error: " + insertErr.Error())
	}

	logger.LogInfo(c, fmt.Sprintf("RR image task submitted: taskId=%s upstreamId=%s quota=%d",
		info.PublicTaskID, result.UpstreamTaskID, result.Quota))

	return nil
}