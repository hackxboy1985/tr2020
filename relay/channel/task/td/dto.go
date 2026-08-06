package td

// ============================
// Request structures
// ============================

// GenerateRequest 图像生成请求
type GenerateRequest struct {
	Model      string   `json:"model"`                // 固定传 gpt-image-2-all
	Prompt     string   `json:"prompt"`               // 图像描述
	Size       string   `json:"size,omitempty"`       // 画面比例，如 1:1 16:9 等
	Resolution string   `json:"resolution"`           // 清晰度档位：1k 2k 4k
	Quality    string   `json:"quality"`              // 质量档位：low medium high
	Images     []string `json:"images,omitempty"`     // 参考图数组（图生图模式）
}

// ============================
// Response structures
// ============================

// SubmitResponse 提交任务响应
type SubmitResponse struct {
	Code int    `json:"code"`
	Data struct {
		ID            string `json:"id"`             // task_id
		Status        string `json:"status"`         // submitted
		Progress      int    `json:"progress"`       // 0
		Created       int64  `json:"created"`        // 创建时间戳
		EstimatedTime int    `json:"estimated_time"` // 预估时间（秒）
	} `json:"data"`
}

// QueryTaskResponse 查询任务响应
type QueryTaskResponse struct {
	Code int              `json:"code"`
	Data QueryTaskData    `json:"data"`
}

// QueryTaskData 任务详情
type QueryTaskData struct {
	ID         string          `json:"id"`                   // task_id
	Status     string          `json:"status"`               // submitted/processing/completed/failed
	Progress   int             `json:"progress"`             // 0-100
	Created    int64           `json:"created"`              // 创建时间戳
	Completed  int64           `json:"completed,omitempty"`  // 完成时间戳
	ActualTime int             `json:"actual_time,omitempty"` // 实际耗时（秒）
	Result     *TaskResult     `json:"result,omitempty"`     // 成功时返回
	Error      *TaskError      `json:"error,omitempty"`      // 失败时返回
}

// TaskResult 任务结果
type TaskResult struct {
	Images []TaskImage `json:"images"`
}

// TaskImage 生成的图像
type TaskImage struct {
	URL       []string `json:"url"`        // 图像URL数组
	ExpiresAt int64    `json:"expires_at"` // 过期时间戳
}

// TaskError 任务错误信息
type TaskError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Type    string `json:"type"`
}

// BatchQueryRequest 批量查询请求
type BatchQueryRequest struct {
	TaskIDs []string `json:"task_ids"`
}

// BatchQueryResponse 批量查询响应
type BatchQueryResponse struct {
	Code int                      `json:"code"`
	Data map[string]QueryTaskData `json:"data"` // key 为 task_id
}
