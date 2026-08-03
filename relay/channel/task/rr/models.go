package rr

// submitRequest 对应上游提交接口请求体
type submitRequest struct {
	Prompt      string   `json:"prompt"`
	AspectRatio string   `json:"aspectRatio,omitempty"`
	Resolution  string   `json:"resolution,omitempty"`
	Quality     string   `json:"quality,omitempty"`
	ImageUrls   []string `json:"imageUrls,omitempty"`
}

// submitResponse 上游提交任务响应，兼容顶层响应和旧包装响应。
type submitResponse struct {
	Code         int    `json:"code"`
	Msg          string `json:"msg"`
	TaskID       string `json:"taskId"`
	Status       string `json:"status"`
	ErrorMessage string `json:"errorMessage"`
	Data         struct {
		TaskID string `json:"taskId"`
	} `json:"data"`
}

// queryRequest 对应上游轮询接口请求体
type queryRequest struct {
	TaskID string `json:"taskId"`
}

// queryResult 上游轮询任务结果中的单条 result
type queryResult struct {
	URL        string `json:"url"`
	NodeID     string `json:"nodeId"`
	OutputType string `json:"outputType"`
	Text       string `json:"text"`
}

// queryResponse 上游轮询任务响应，兼容顶层响应和旧包装响应。
type queryResponse struct {
	Code         int           `json:"code"`
	Msg          string        `json:"msg"`
	TaskID       string        `json:"taskId"`
	Status       string        `json:"status"`
	ErrorMessage string        `json:"errorMessage"`
	Results      []queryResult `json:"results"`
	Data         struct {
		TaskID  string        `json:"taskId"`
		Status  string        `json:"status"`
		Results []queryResult `json:"results"`
	} `json:"data"`
}
