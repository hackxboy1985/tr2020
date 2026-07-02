package dto

// CreateVideoProjectRequest 创建视频项目请求
type CreateVideoProjectRequest struct {
	// 广告基础信息（必填）
	ProductImgUrl string `json:"product_img_url" binding:"required"`
	Brand         string `json:"brand" binding:"required"`
	ProductName   string `json:"product_name" binding:"required"`
	Tagline       string `json:"tagline"`
	SellingPoints string `json:"selling_points"`

	// 创意方向（必填）
	Prompt   string `json:"prompt" binding:"required"` // 用户创意描述，核心输入
	Vtype    string `json:"vtype" binding:"required"`  // 视频类型
	VtypeAdd string `json:"vtype_add"`                 // 剧情子类型
	Language string `json:"language"`                  // 广告语言
	Platform string `json:"platform"`                  // 投放平台
	Region   string `json:"region"`                    // 投放地区

	// 角色与参考（可选）
	Roles        string `json:"roles"`         // JSON字符串: [{name, url}]
	SelectAudios string `json:"select_audios"` // JSON字符串: [{url, remark}]

	// 输出配置（必填）
	Duration   int    `json:"duration" binding:"required"`   // 视频时长（秒）
	Resolution string `json:"resolution" binding:"required"` // 分辨率: 2K, 4K
	VideoModel string `json:"video_model"`                   // 视频模型: seedance
	Whstr      string `json:"whstr" binding:"required"`      // 宽高比: 16:9, 9:16

	// 渠道选择（可选）
	// - channel_id: 管理员可指定，普通用户传了静默忽略
	// - channel_type: 过滤渠道类型；channel_id 有效时忽略此字段
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

// VideoProjectDetailResponse 视频项目详情响应
type VideoProjectDetailResponse struct {
	ProjectId        int64  `json:"project_id"`
	ProjectName      string `json:"project_name"`
	Status           string `json:"status"`
	ErrorMsg         string `json:"error_msg,omitempty"`
	Progress         string `json:"progress,omitempty"`
	ProductImgUrl    string `json:"product_img_url"`
	Brand            string `json:"brand"`
	ProductName      string `json:"product_name"`
	MainImageUrl     string `json:"main_image_url,omitempty"`
	MainImageAssetId string `json:"main_image_asset_id,omitempty"`
	GeneratedResult  string `json:"generated_result,omitempty"`
	FirstVideoUrl    string `json:"first_video_url,omitempty"`
	CreatedAt        int64  `json:"created_at"`
	UpdatedAt        int64  `json:"updated_at"`
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
}
