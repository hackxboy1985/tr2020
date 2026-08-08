package rr

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/QuantumNous/new-api/relay/channel"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type TaskAdaptor struct {
	taskcommon.BaseBilling
	apiKey        string
	baseURL       string
	otherSettings dto.ChannelOtherSettings
	taskReq       *relaycommon.TaskSubmitReq // cached in ValidateRequestAndSetAction
}

// endpointConditions maps a key suffix to the condition that activates it.
// To add a new routing condition:
//  1. Add an entry here.
//  2. Add the corresponding option to the frontend Condition dropdown in rr-path-config-editor.tsx.
var endpointConditions = []struct {
	suffix string
	match  func(req relaycommon.TaskSubmitReq) bool
}{
	{":image", func(req relaycommon.TaskSubmitReq) bool {
		return req.HasImage()
	}},
}

// resolveEndpointPath returns the upstream path for modelName+req by testing
// each condition in order, then falling back to the plain model key.
// Returns "" if no matching entry is found (caller uses DefaultSubmitPath).
func (a *TaskAdaptor) resolveEndpointPath(modelName string, req relaycommon.TaskSubmitReq) string {
	eps := a.otherSettings.RREndpoints
	for _, cond := range endpointConditions {
		if cond.match(req) {
			if ep, ok := eps[modelName+cond.suffix]; ok && ep != "" {
				return ep
			}
		}
	}
	if ep, ok := eps[modelName]; ok && ep != "" {
		return ep
	}
	return ""
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.apiKey = info.ApiKey
	a.baseURL = info.ChannelBaseUrl
	a.otherSettings = info.ChannelOtherSettings
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	if taskErr := relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionGenerate); taskErr != nil {
		return taskErr
	}
	// Cache the task request so BuildRequestURL can evaluate routing conditions
	// once info.UpstreamModelName has been resolved by the relay framework.
	if req, err := relaycommon.GetTaskRequest(c); err == nil {
		a.taskReq = &req
	}
	return nil
}

// InjectBillingParams reads the size from the task request, maps it to a resolution,
// and injects the result into info.BillingRequestInput.Body so that billing
// expressions can use param("resolution") to price by resolution tier.
func (a *TaskAdaptor) InjectBillingParams(c *gin.Context, info *relaycommon.RelayInfo) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return
	}

	cfg := mapSizeWithResolution(req.Size, req.Resolution)

	if info.BillingRequestInput == nil {
		info.BillingRequestInput = &billingexpr.RequestInput{Body: []byte("{}")}
	}

	info.BillingRequestInput.Body = billingexpr.InjectBodyParam(
		info.BillingRequestInput.Body, "resolution", cfg.Resolution,
	)
}

// BuildRequestURL resolves the upstream endpoint.
// Priority: channel RREndpoints[model+condition] > RREndpoints[model] > default path pattern.
func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if a.taskReq != nil {
		if path := a.resolveEndpointPath(info.UpstreamModelName, *a.taskReq); path != "" {
			url := fmt.Sprintf("%s%s", a.baseURL, path)
			common.SysLog(fmt.Sprintf("RR upstream request URL: model=%s, path=%s, url=%s", info.UpstreamModelName, path, url))
			return url, nil
		}
	}
	url := fmt.Sprintf("%s"+DefaultSubmitPath, a.baseURL, info.UpstreamModelName)
	common.SysLog(fmt.Sprintf("RR upstream request URL: model=%s, path=%s, url=%s", info.UpstreamModelName, DefaultSubmitPath, url))
	return url, nil
}

func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	maskedKey := ""
	if len(a.apiKey) <= 8 {
		maskedKey = "***"
	} else {
		maskedKey = a.apiKey[:4] + "***" + a.apiKey[len(a.apiKey)-4:]
	}
	common.SysLog(fmt.Sprintf("RR upstream request headers: content-type=%s, accept=%s, authorization=Bearer %s", req.Header.Get("Content-Type"), req.Header.Get("Accept"), maskedKey))
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, fmt.Errorf("task_request not found in context")
	}

	cfg := mapSizeWithResolution(req.Size, req.Resolution)
	body := submitRequest{
		Prompt:      req.Prompt,
		AspectRatio: cfg.AspectRatio,
		Resolution:  cfg.Resolution,
		Quality:     mapQuality(req.Quality),
	}
	if req.HasImage() {
		if req.Image != "" {
			body.ImageUrls = []string{req.Image}
		} else {
			body.ImageUrls = req.Images
		}
	}

	data, err := common.Marshal(body)
	if err != nil {
		return nil, err
	}
	if common.LogUpstreamRequestEnabled {
		common.SysLog(fmt.Sprintf("RR upstream request body: %s", string(data)))
	}
	return bytes.NewReader(data), nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var sResp submitResponse
	if err := common.Unmarshal(responseBody, &sResp); err != nil {
		common.SysLog(fmt.Sprintf("RR upstream submit response: status=%d, body=%s", resp.StatusCode, string(responseBody)))
		taskErr = service.TaskErrorWrapper(err, "unmarshal_response_failed", http.StatusInternalServerError)
		return
	}
	common.SysLog(fmt.Sprintf("RR upstream submit response: status=%d, body=%s", resp.StatusCode, string(responseBody)))
	if sResp.Code != 0 && sResp.Code != 200 {
		taskErr = service.TaskErrorWrapperLocal(
			fmt.Errorf("%s", sResp.Msg),
			"upstream_error",
			http.StatusBadRequest,
		)
		return
	}
	taskID = sResp.TaskID
	if taskID == "" {
		taskID = sResp.Data.TaskID
	}
	if taskID == "" {
		taskErr = service.TaskErrorWrapperLocal(
			fmt.Errorf("upstream returned empty task id"),
			"upstream_error",
			http.StatusBadRequest,
		)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":      info.PublicTaskID,
		"object":  "image.task",
		"status":  "processing",
		"model":   info.OriginModelName,
		"created": time.Now().Unix(),
	})

	return taskID, responseBody, nil
}

func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok || taskID == "" {
		return nil, fmt.Errorf("invalid task_id")
	}

	reqBody, err := common.Marshal(queryRequest{TaskID: taskID})
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s%s", baseUrl, QueryPath)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var qResp queryResponse
	if err := common.Unmarshal(respBody, &qResp); err != nil {
		return nil, fmt.Errorf("unmarshal query result failed: %w", err)
	}

	info := &relaycommon.TaskInfo{}

	if qResp.Code != 0 && qResp.Code != 200 {
		info.Status = string(model.TaskStatusFailure)
		info.Reason = qResp.Msg
		info.Progress = taskcommon.ProgressComplete
		return info, nil
	}

	status := qResp.Status
	results := qResp.Results
	if status == "" {
		status = qResp.Data.Status
		results = qResp.Data.Results
	}
	errorMessage := qResp.ErrorMessage
	if errorMessage == "" {
		errorMessage = qResp.Msg
	}

	switch status {
	case StatusSuccess:
		info.Status = string(model.TaskStatusSuccess)
		info.Progress = taskcommon.ProgressComplete
		if len(results) > 0 {
			info.Url = results[0].URL
		}
	case StatusFailed:
		info.Status = string(model.TaskStatusFailure)
		info.Progress = taskcommon.ProgressComplete
		if errorMessage != "" {
			info.Reason = errorMessage
		} else {
			info.Reason = "upstream task failed"
		}
	case StatusRunning:
		info.Status = string(model.TaskStatusInProgress)
		info.Progress = "50%"
	default:
		// PENDING or unknown
		info.Status = string(model.TaskStatusInProgress)
		info.Progress = "20%"
	}

	return info, nil
}

func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}
