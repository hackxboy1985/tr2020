package poster

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
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// ============================
// Request / Response structures
// ============================

// posterGenerateRequest 对应 /openapi/v1/poster/generateAsync
type posterGenerateRequest struct {
	Query               string   `json:"query"`
	GenerateType        int      `json:"generateType,omitempty"`
	PosterType          int      `json:"posterType,omitempty"`
	PlatformType        string   `json:"platformType,omitempty"`
	LanguageType        string   `json:"languageType,omitempty"`
	DetailPictureNumber int      `json:"detailPictureNumber,omitempty"`
	ModelEdition        int      `json:"modelEdition,omitempty"`
	NeedText            *bool    `json:"needText,omitempty"`
	AspectRatio         string   `json:"aspectRatio,omitempty"`
	FileUrlList         []string `json:"fileUrlList,omitempty"`
	UserId              int64    `json:"userId,omitempty"`
}

// posterFreeCreationRequest 对应 /openapi/v1/poster/allAroundCreation
type posterFreeCreationRequest struct {
	Query               string   `json:"query"`
	DetailPictureNumber int      `json:"detailPictureNumber,omitempty"`
	AspectRatio         string   `json:"aspectRatio,omitempty"`
	ApiImgUrlList       []string `json:"apiImgUrlList,omitempty"`
}

// submitResponse 上游提交任务响应
type submitResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		AgentGenerateTaskId string `json:"agentGenerateTaskId"`
	} `json:"data"`
}

// queryTaskResponse 上游轮询任务响应
type queryTaskResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		TaskList []struct {
			TaskStatus    string `json:"taskStatus"`
			ExecuteResult string `json:"executeResult"`
		} `json:"taskList"`
	} `json:"data"`
}

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

// ValidateRequestAndSetAction 自定义验证，支持 query 从 metadata 或外层 prompt 读取
func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	var req relaycommon.TaskSubmitReq
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}

	// query 优先从 metadata 读，兼容外层 prompt
	if strings.TrimSpace(req.Prompt) == "" {
		if q, ok := req.Metadata["query"].(string); ok && strings.TrimSpace(q) != "" {
			req.Prompt = q
		}
	}

	if strings.TrimSpace(req.Prompt) == "" {
		return service.TaskErrorWrapperLocal(
			fmt.Errorf("query is required"),
			"invalid_request",
			http.StatusBadRequest,
		)
	}

	c.Set("task_request", req)
	info.Action = constant.TaskActionGenerate
	return nil
}

// BuildRequestURL 根据模型名路由到不同端点
func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	endpoint := EndpointGenerateAsync
	if info.UpstreamModelName == "poster-free-creation" {
		endpoint = EndpointFreeCreation
	}
	return fmt.Sprintf("%s%s", a.baseURL, endpoint), nil
}

// BuildRequestHeader 设置鉴权头
func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	return nil
}

// BuildRequestBody 构建上游请求体
func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	v, exists := c.Get("task_request")
	if !exists {
		return nil, fmt.Errorf("request not found in context")
	}
	req, ok := v.(relaycommon.TaskSubmitReq)
	if !ok {
		return nil, fmt.Errorf("invalid request type in context")
	}

	var body interface{}
	var err error

	if info.UpstreamModelName == "poster-free-creation" {
		body, err = a.buildFreeCreationBody(&req)
	} else {
		body, err = a.buildGenerateBody(&req)
	}
	if err != nil {
		return nil, err
	}

	data, err := common.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

// DoRequest 发送请求
func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

// DoResponse 解析上游响应，返回 taskID 和原始响应体
func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var sResp submitResponse
	if err := common.Unmarshal(responseBody, &sResp); err != nil {
		taskErr = service.TaskErrorWrapper(err, "unmarshal_response_failed", http.StatusInternalServerError)
		return
	}
	if sResp.Code != 200 {
		taskErr = service.TaskErrorWrapperLocal(
			fmt.Errorf("%s", sResp.Msg),
			"upstream_error",
			http.StatusBadRequest,
		)
		return
	}
	if sResp.Data.AgentGenerateTaskId == "" {
		taskErr = service.TaskErrorWrapperLocal(
			fmt.Errorf("upstream returned empty task id"),
			"upstream_error",
			http.StatusBadRequest,
		)
		return
	}

	// 返回给客户端：image task 格式
	c.JSON(http.StatusOK, gin.H{
		"id":      info.PublicTaskID,
		"object":  "image.task",
		"status":  "processing",
		"model":   info.OriginModelName,
		"created": time.Now().Unix(),
	})

	return sResp.Data.AgentGenerateTaskId, responseBody, nil
}

// FetchTask 轮询上游任务状态
func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok || taskID == "" {
		return nil, fmt.Errorf("invalid task_id")
	}

	url := fmt.Sprintf("%s%s?taskId=%s", baseUrl, EndpointQueryTaskResult, taskID)
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

// ParseTaskResult 解析轮询响应，返回 TaskInfo
func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var qResp queryTaskResponse
	if err := common.Unmarshal(respBody, &qResp); err != nil {
		return nil, fmt.Errorf("unmarshal query result failed: %w", err)
	}

	info := &relaycommon.TaskInfo{}

	if qResp.Code != 200 {
		info.Status = model.TaskStatusFailure
		info.Reason = qResp.Msg
		info.Progress = taskcommon.ProgressComplete
		return info, nil
	}

	if len(qResp.Data.TaskList) == 0 {
		info.Status = model.TaskStatusInProgress
		info.Progress = taskcommon.ProgressInProgress
		return info, nil
	}

	// 以第一个任务状态为准（所有子任务状态一致）
	first := qResp.Data.TaskList[0]
	switch first.TaskStatus {
	case TaskStatusRunning:
		info.Status = model.TaskStatusInProgress
		info.Progress = taskcommon.ProgressInProgress
	case TaskStatusSuccess:
		info.Status = model.TaskStatusSuccess
		info.Progress = taskcommon.ProgressComplete
		// 收集所有成功的图片 URL，逗号分隔存入 Url
		var urls []string
		for _, t := range qResp.Data.TaskList {
			if t.ExecuteResult != "" {
				urls = append(urls, t.ExecuteResult)
			}
		}
		info.Url = strings.Join(urls, ",")
	case TaskStatusFailed:
		info.Status = model.TaskStatusFailure
		info.Progress = taskcommon.ProgressComplete
		info.Reason = "upstream task failed"
	default:
		info.Status = model.TaskStatusInProgress
		info.Progress = taskcommon.ProgressInProgress
	}

	return info, nil
}

func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}

// ============================
// helpers
// ============================

func (a *TaskAdaptor) buildGenerateBody(req *relaycommon.TaskSubmitReq) (*posterGenerateRequest, error) {
	body := &posterGenerateRequest{}
	if err := taskcommon.UnmarshalMetadata(req.Metadata, body); err != nil {
		return nil, fmt.Errorf("unmarshal metadata failed: %w", err)
	}
	// query 优先用 metadata 里的，否则用外层 prompt
	if body.Query == "" {
		body.Query = req.Prompt
	}
	return body, nil
}

func (a *TaskAdaptor) buildFreeCreationBody(req *relaycommon.TaskSubmitReq) (*posterFreeCreationRequest, error) {
	body := &posterFreeCreationRequest{}
	if err := taskcommon.UnmarshalMetadata(req.Metadata, body); err != nil {
		return nil, fmt.Errorf("unmarshal metadata failed: %w", err)
	}
	if body.Query == "" {
		body.Query = req.Prompt
	}
	return body, nil
}
