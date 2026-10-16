package zy

import "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"

// ============================
// Request structures
// ============================

// submitRequest 对应上游 POST /v1/videos 请求体。
// 图片生成与视频生成共用该接口，通过 model 区分。
// aspect_ratio 与 size 二选一；image_size 仅在模型支持参数选档时下发。
type submitRequest struct {
	Model       string   `json:"model"`
	Prompt      string   `json:"prompt"`
	AspectRatio string   `json:"aspect_ratio,omitempty"`
	Size        string   `json:"size,omitempty"`
	ImageSize   string   `json:"image_size,omitempty"`
	Images      []string `json:"images,omitempty"`
}

// ============================
// Response structures
// ============================

// taskResponse 上游提交（POST）与查询（GET）接口的响应体，二者字段一致。
type taskResponse struct {
	ID string `json:"id"` // 任务 ID，格式 task_xxxx
	taskcommon.ZyTaskResult
	Status      string `json:"status"` // queued / in_progress / completed / failed
	Progress    int    `json:"progress"`
	CreatedAt   int64  `json:"created_at"`
	CompletedAt int64  `json:"completed_at,omitempty"`
	Error       *struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// resultURL 按媒体类型选择地址，和 OSS 实时转存、补扫共用同一判据。
func (r *taskResponse) resultURL() string {
	return r.ZyTaskResult.ResultURL()
}

// errorMessage 返回失败原因；上游未给出时回退到错误码。
func (r *taskResponse) errorMessage() string {
	if r.Error == nil {
		return ""
	}
	if r.Error.Message != "" {
		return r.Error.Message
	}
	return r.Error.Code
}
