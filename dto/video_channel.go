package dto

// VideoChannelRequest 创建/更新渠道请求
type VideoChannelRequest struct {
	Name                string `json:"name" binding:"required"`
	ChannelType         string `json:"channel_type" binding:"required"`
	BaseURL             string `json:"base_url"`
	ApiKey              string `json:"api_key"`
	ApiSecret           string `json:"api_secret"`
	WorkflowId          string `json:"workflow_id"`
	CreatePath          string `json:"create_path"`
	StatusQueryPath     string `json:"status_query_path"`
	Groups              string `json:"groups" binding:"required"`
	Weight              int    `json:"weight"`
	Enabled             int    `json:"enabled"`
	Remark              string `json:"remark"`
	SaveRequestResponse int    `json:"save_request_response"`
	ModelMapping        string  `json:"model_mapping"`
	ModelPrices         string  `json:"model_prices"`
	PricePerSecond      float64 `json:"price_per_second"`
	PreDeductQuota      int     `json:"pre_deduct_quota"`
	RateLimit           int     `json:"rate_limit"`
}

// VideoChannelResponse 渠道响应（不暴露密钥）
type VideoChannelResponse struct {
	Id                  int    `json:"id"`
	Name                string `json:"name"`
	ChannelType         string `json:"channel_type"`
	BaseURL             string `json:"base_url"`
	WorkflowId          string `json:"workflow_id"`
	CreatePath          string `json:"create_path"`
	StatusQueryPath     string `json:"status_query_path"`
	Groups              string `json:"groups"`
	Weight              int    `json:"weight"`
	Enabled             int    `json:"enabled"`
	Remark              string `json:"remark"`
	SaveRequestResponse int    `json:"save_request_response"`
	ModelMapping        string  `json:"model_mapping"`
	ModelPrices         string  `json:"model_prices"`
	PricePerSecond      float64 `json:"price_per_second"`
	PreDeductQuota      int     `json:"pre_deduct_quota"`
	RateLimit           int     `json:"rate_limit"`
	CreatedAt           int64   `json:"created_at"`
	UpdatedAt           int64   `json:"updated_at"`
}

// VideoChannelStatusRequest 启用/禁用渠道
type VideoChannelStatusRequest struct {
	Enabled int `json:"enabled" binding:"required"`
}
