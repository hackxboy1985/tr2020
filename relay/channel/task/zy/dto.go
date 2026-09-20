package zy

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
	ID          string `json:"id"` // 任务 ID，格式 task_xxxx
	Object      string `json:"object"`
	Model       string `json:"model"`
	Status      string `json:"status"` // queued / in_progress / completed / failed
	Progress    int    `json:"progress"`
	CreatedAt   int64  `json:"created_at"`
	CompletedAt int64  `json:"completed_at,omitempty"`
	URL         string `json:"url,omitempty"` // 仅 completed 返回
	Metadata    struct {
		Image    string `json:"image,omitempty"`
		ImageURL string `json:"image_url,omitempty"`
	} `json:"metadata,omitempty"`
	Error *struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// resultURL 依次读取顶层 url、metadata.image_url、metadata.image（三者同值）。
func (r *taskResponse) resultURL() string {
	if r.URL != "" {
		return r.URL
	}
	if r.Metadata.ImageURL != "" {
		return r.Metadata.ImageURL
	}
	return r.Metadata.Image
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
