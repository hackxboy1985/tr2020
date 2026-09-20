package zy

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

// ValidateRequestAndSetAction 校验下游请求。
// 上游要求 prompt 必填、model 必填；aspect_ratio 与 size 二选一。
func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	if taskErr := relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionGenerate); taskErr != nil {
		return taskErr
	}

	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}

	// aspect_ratio 优先校验；未提供时再校验 size
	if ratio := strings.TrimSpace(req.AspectRatio); ratio != "" {
		if !IsSupportedAspectRatio(ratio) {
			return service.TaskErrorWrapperLocal(
				fmt.Errorf("invalid aspect_ratio: %s", req.AspectRatio),
				"invalid_request",
				http.StatusBadRequest,
			)
		}
	} else if size := strings.TrimSpace(req.Size); size != "" {
		if !IsPixelSize(size) && !IsRatioSize(size) {
			return service.TaskErrorWrapperLocal(
				fmt.Errorf("invalid size: %s, expected pixel size (e.g. 1280x720) or ratio (e.g. 16:9)", req.Size),
				"invalid_request",
				http.StatusBadRequest,
			)
		}
		if IsRatioSize(size) && !IsSupportedAspectRatio(size) {
			return service.TaskErrorWrapperLocal(
				fmt.Errorf("invalid size ratio: %s", req.Size),
				"invalid_request",
				http.StatusBadRequest,
			)
		}
	}

	// image_size 与 resolution 等价，只传其中一个；冲突与非法档位由 resolveSizeConfig 统一校验
	if _, _, err := resolveSizeConfig(req.AspectRatio, req.Size, req.ImageSize, req.Resolution); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}

	// gpt-image-2.5 仅支持 1K。
	// 注意：此时 info.UpstreamModelName 尚未完成模型映射（映射在 ValidateXXX 之后），
	// 因此以请求中的模型名判断；映射后若模型能力更严格，Step 2.5 的映射结果
	// 会在 BuildRequestBody 中再次生效。
	if err := validateTierSupport(resolvedModelForTier(req.Model, info.UpstreamModelName), req.ImageSize, req.Resolution); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}

	if len(req.Images) > MaxReferenceImages {
		return service.TaskErrorWrapperLocal(
			fmt.Errorf("too many reference images: %d, at most %d allowed", len(req.Images), MaxReferenceImages),
			"invalid_request",
			http.StatusBadRequest,
		)
	}

	return nil
}

// InjectBillingParams 在计费表达式求值前注入归一化后的 aspect_ratio 与 resolution，
// 便于管理员用 param("resolution") / param("aspect_ratio") 按档位定价。
func (a *TaskAdaptor) InjectBillingParams(c *gin.Context, info *relaycommon.RelayInfo) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return
	}

	cfg, passthroughSize, err := resolveSizeConfig(req.AspectRatio, req.Size, req.ImageSize, req.Resolution)
	if err != nil {
		return
	}

	if info.BillingRequestInput == nil {
		info.BillingRequestInput = &billingexpr.RequestInput{Body: []byte("{}")}
	}

	body := info.BillingRequestInput.Body
	// 归一化档位：未命中对照表的像素尺寸直接记为像素，便于按像素定价
	resolution := cfg.Resolution
	if passthroughSize != "" {
		resolution = passthroughSize
	}
	body = billingexpr.InjectBodyParam(body, "resolution", resolution)
	body = billingexpr.InjectBodyParam(body, "aspect_ratio", cfg.AspectRatio)
	info.BillingRequestInput.Body = body
}

// BuildRequestURL 构建上游提交地址
func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	fullURL := fmt.Sprintf("%s%s", a.baseURL, EndpointSubmit)
	if common.LogUpstreamRequestEnabled {
		logger.LogInfo(nil, fmt.Sprintf("Zy upstream request URL: %s", fullURL))
	}
	return fullURL, nil
}

// BuildRequestHeader 设置请求头
func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	return nil
}

// BuildRequestBody 组装上游请求体。
// aspect_ratio 与 size 二选一；档位按模型能力决定是否以 image_size 下发。
func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, fmt.Errorf("task_request not found in context: %w", err)
	}

	cfg, passthroughSize, err := resolveSizeConfig(req.AspectRatio, req.Size, req.ImageSize, req.Resolution)
	if err != nil {
		return nil, err
	}

	model := info.UpstreamModelName
	if model == "" {
		model = req.Model
	}

	// 图生图：images 非空即进入图生图模式
	var images []string
	if len(req.Images) > 0 {
		images = req.Images
	} else if len(req.InputReference) > 0 {
		images = req.InputReference
	}

	upstreamReq, err := buildSubmitRequest(model, req.Prompt, cfg, passthroughSize, images)
	if err != nil {
		return nil, err
	}

	data, err := common.Marshal(upstreamReq)
	if err != nil {
		return nil, err
	}

	if common.LogUpstreamRequestEnabled {
		logger.LogInfo(nil, fmt.Sprintf("Zy upstream request: %s", string(data)))
	}

	// 保存到 context，供 LogTaskConsumption 写入 other["request_body"] / request_path
	c.Set(string(constant.ContextKeyVideoRequestBody), string(data))
	c.Set(string(constant.ContextKeyVideoRequestPath), EndpointSubmit)

	return bytes.NewReader(data), nil
}

// DoRequest 发送提交请求
func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

// DoResponse 解析提交响应，返回上游 task_id 与原始响应体。
func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if common.LogUpstreamRequestEnabled {
		logger.LogInfo(nil, fmt.Sprintf("Zy upstream response: %s", string(responseBody)))
	}

	var sResp taskResponse
	if err := common.Unmarshal(responseBody, &sResp); err != nil {
		taskErr = service.TaskErrorWrapper(err, "unmarshal_response_failed", http.StatusInternalServerError)
		return
	}

	if sResp.ID == "" {
		taskErr = service.TaskErrorWrapperLocal(
			fmt.Errorf("task_id not found in response"),
			"invalid_response",
			http.StatusInternalServerError,
		)
		return
	}

	// 保存响应体到 context，供 LogTaskConsumption 写入 other["response_body"]
	c.Set(string(constant.ContextKeyVideoResponseBody), string(responseBody))

	// 响应体由 handleZyImageTask 在任务落库后统一返回（含公开 task_id 与 created）
	return sResp.ID, responseBody, nil
}

// FetchTask 查询任务状态（轮询使用）
func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok || taskID == "" {
		return nil, fmt.Errorf("invalid task_id")
	}

	url := fmt.Sprintf("%s%s%s", baseUrl, EndpointQueryTask, taskID)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

// ParseTaskResult 解析查询结果。
// 完成态的图片地址在顶层 url，metadata.image_url / metadata.image 同值。
func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var qResp taskResponse
	if err := common.Unmarshal(respBody, &qResp); err != nil {
		return nil, fmt.Errorf("unmarshal query result failed: %w", err)
	}

	info := &relaycommon.TaskInfo{}

	switch qResp.Status {
	case StatusQueued:
		info.Status = string(model.TaskStatusQueued)
		info.Progress = taskcommon.ProgressQueued
	case StatusInProgress:
		info.Status = string(model.TaskStatusInProgress)
		info.Progress = taskcommon.ProgressInProgress
	case StatusCompleted:
		info.Status = string(model.TaskStatusSuccess)
		info.Progress = taskcommon.ProgressComplete
		info.Url = qResp.resultURL()
	case StatusFailed:
		info.Status = string(model.TaskStatusFailure)
		info.Progress = taskcommon.ProgressComplete
		if reason := qResp.errorMessage(); reason != "" {
			info.Reason = reason
		} else {
			info.Reason = "upstream task failed"
		}
	default:
		// 未知状态：保持进行中，等待下一轮轮询
		info.Status = string(model.TaskStatusInProgress)
		info.Progress = taskcommon.ProgressInProgress
	}

	return info, nil
}

// GetModelList 返回支持的模型列表
func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

// GetChannelName 返回渠道名称
func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}
