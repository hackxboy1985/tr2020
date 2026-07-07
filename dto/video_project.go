package dto

// VideoMediaItem mediaList 元素
type VideoMediaItem struct {
	MediaType string `json:"mediaType"` // PRODUCT / ROLE / OTHER
	MediaUrl  string `json:"mediaUrl"`
	AssetId   int64  `json:"assetId,omitempty"`
	RoleName  string `json:"roleName,omitempty"`
	SortOrder int    `json:"sortOrder,omitempty"`
}

// CreateVideoProjectRequest 创建视频项目请求
type CreateVideoProjectRequest struct {
	// TokenGroup Token 鉴权时的分组（优先于 users.group），controller 自动注入
	TokenGroup string `json:"-"`
	// 广告基础信息（必填）
	// ProductImgUrl: 旧格式兼容，优先使用 MediaList
	ProductImgUrl string `json:"product_img_url"`
	Brand         string `json:"brand" binding:"required"`
	ProductName   string `json:"product_name" binding:"required"`
	Tagline       string `json:"tagline"`
	SellingPoints string `json:"selling_points"`

	// 创意方向（必填）
	Prompt   string `json:"prompt" binding:"required"`
	Vtype    string `json:"vtype" binding:"required"`
	VtypeAdd string `json:"vtype_add"`
	Language string `json:"language"`
	Platform string `json:"platform"`
	Region   string `json:"region"`

	// 媒体列表（OpenAPI 格式，优先于旧字段）
	// 至少 1 张 PRODUCT 图；为空时从 ProductImgUrl + Roles 自动转换
	MediaList []VideoMediaItem `json:"mediaList"`

	// 角色与参考（旧格式兼容）
	Roles        string `json:"roles"`         // JSON字符串: [{name, url}]
	SelectAudios string `json:"select_audios"` // JSON字符串: [{url, remark}]

	// 输出配置（必填）
	Duration   int    `json:"duration" binding:"required,oneof=15 30 45 60"`
	Resolution string `json:"resolution" binding:"required"`
	VideoModel string `json:"video_model"`
	Whstr      string `json:"whstr" binding:"required"`

	// 渠道选择（可选）
	ChannelId   int    `json:"channel_id"`
	ChannelType string `json:"channel_type"`
}

// VideoProjectResponse 视频项目响应
type VideoProjectResponse struct {
	ProjectId   int64  `json:"project_id"`
	ProjectName string `json:"project_name"`
	Status      string `json:"status"`
	CreatedAt   int64  `json:"created_at"`
}

// VideoProjectBilling 视频项目计费信息（仅成功结算后有值）
type VideoProjectBilling struct {
	UpstreamAmount string `json:"upstream_amount"` // 上游预扣金额（元）
	UpstreamRefund string `json:"upstream_refund"` // 上游退款金额（元）
	UpstreamNet    string `json:"upstream_net"`    // 上游实际净扣（元）= amount - refund
	RefundRatio    string `json:"refund_ratio"`    // 退费比例 = refund / amount，用于下游按比例计费
}

// VideoProjectDetailResponse 视频项目详情响应
type VideoProjectDetailResponse struct {
	ProjectId        int64                `json:"project_id"`
	ProjectName      string               `json:"project_name"`
	Status           string               `json:"status"`
	ErrorMsg         string               `json:"error_msg,omitempty"`
	Progress         string               `json:"progress,omitempty"`
	ProductImgUrl    string               `json:"product_img_url"`
	Brand            string               `json:"brand"`
	ProductName      string               `json:"product_name"`
	MainImageUrl     string               `json:"main_image_url,omitempty"`
	GeneratedResult  string               `json:"generated_result,omitempty"`
	FirstVideoUrl    string               `json:"first_video_url,omitempty"`
	CreatedAt        int64                `json:"created_at"`
	UpdatedAt        int64                `json:"updated_at"`
	Billing          *VideoProjectBilling `json:"billing,omitempty"` // 计费信息，仅成功结算后有值
}

// VideoProjectListResponse 视频项目列表响应
type VideoProjectListResponse struct {
	Items    []VideoProjectItemResponse `json:"items"`
	Total    int64                      `json:"total"`
	Page     int                        `json:"page"`
	PageSize int                        `json:"page_size"`
}

// VideoProjectItemResponse 视频项目列表项
type VideoProjectItemResponse struct {
	ProjectId   int64  `json:"project_id"`
	ProjectName string `json:"project_name"`
	Status      string `json:"status"`
	Brand       string `json:"brand"`
	ProductName string `json:"product_name"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

// UpdateVideoProjectStatusRequest 管理员更新项目状态请求
type UpdateVideoProjectStatusRequest struct {
	Status           string `json:"status"`
	ErrorMsg         string `json:"error_msg"`
	MainImageUrl     string `json:"main_image_url"`
	MainImageAssetId string `json:"main_image_asset_id"`
	GeneratedResult  string `json:"generated_result"`
}

// WebhookPayload Webhook回调载荷（通用格式）
type WebhookPayload struct {
	ProjectId        string `json:"project_id"`         // 本地项目ID或远程项目ID
	RemoteProjectId  string `json:"remote_project_id"`  // 三方平台项目ID
	Status           string `json:"status"`
	ErrorMsg         string `json:"error_msg"`
	MainImageUrl     string `json:"main_image_url"`
	MainImageAssetId string `json:"main_image_asset_id"`
	GeneratedResult  string `json:"generated_result"`
	FirstVideoUrl    string `json:"first_video_url"`
	Progress         string `json:"progress"`
}

// AdapterCreateResponse 渠道适配器创建项目响应
type AdapterCreateResponse struct {
	RemoteProjectId string `json:"remote_project_id"` // 三方平台返回的项目ID
	Status          string `json:"status"`
	Message         string `json:"message"`
	RawRequest      []byte `json:"-"` // 发送给上游的原始请求体
	RawResponse     []byte `json:"-"` // 上游返回的原始响应体
}

// AdapterStatusResponse 渠道适配器查询状态响应
type AdapterStatusResponse struct {
	Status           string `json:"status"`
	ErrorMsg         string `json:"error_msg"`
	Progress         string `json:"progress"`
	MainImageUrl     string `json:"main_image_url"`
	MainImageAssetId string `json:"main_image_asset_id"`
	GeneratedResult  string `json:"generated_result"`
	FirstVideoUrl    string `json:"first_video_url"`
	// 上游积分字段（终态时有效）
	CreditAmount int     `json:"credit_amount"` // 上游扣除积分
	CreditRefund int     `json:"credit_refund"` // 上游退回积分
	CreditNet    int     `json:"credit_net"`    // 上游净消耗积分
	// 上游金额字段（终态时有效，1积分=0.1元）
	MoneyAmount  float64 `json:"money_amount"`  // 上游扣除金额（元）
	MoneyRefund  float64 `json:"money_refund"`  // 上游退回金额（元）
	MoneyNet     float64 `json:"money_net"`     // 上游净消耗金额（元）
	RawRequest   []byte  `json:"-"`
	RawResponse  []byte  `json:"-"`
}
