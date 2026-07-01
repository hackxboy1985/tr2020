package dto

// VideoGenerationRequest 视频生成请求参数
type VideoGenerationRequest struct {
	// ========== 广告基础信息 ==========
	ProductImgUrl string `json:"product_img_url" binding:"required"` // 产品图 OSS URL（主图）
	Brand         string `json:"brand" binding:"required,max=15"`    // 产品品牌（15字上限）
	ProductName   string `json:"product_name" binding:"required,max=15"` // 产品名（15字上限）
	Tagline       string `json:"tagline,omitempty"`                  // 宣传语/slogan（非必填）
	SellingPoints string `json:"selling_points,omitempty"`           // 产品卖点

	// ========== 创意方向 ==========
	Prompt   string `json:"prompt" binding:"required"` // 用户创意 prompt（核心输入）
	Vtype    string `json:"vtype" binding:"required"`  // 视频类型（如：产品展示/剧情短片/口播等）
	VtypeAdd string `json:"vtype_add,omitempty"`       // 剧情子类型（可选，如：搞笑/温情/悬疑）
	Language string `json:"language,omitempty"`        // 广告语言
	Platform string `json:"platform,omitempty"`        // 投放平台（如：抖音/淘宝/TikTok）
	Region   string `json:"region,omitempty"`          // 投放地区（如：国内电商/跨境电商）

	// ========== 角色与参考 ==========
	Roles        string `json:"roles,omitempty"`         // 出镜角色列表 JSON: [{name, url, audio, assetId, text, remark}]
	SelectAudios string `json:"select_audios,omitempty"` // 可选音色列表 JSON: [{url, remark}]

	// ========== 输出配置 ==========
	Duration   int    `json:"duration" binding:"required,min=1"`          // 目标视频时长（秒，如 30/60）
	Resolution string `json:"resolution" binding:"required"`              // 输出分辨率（如 2K/4K）
	VideoModel string `json:"video_model,omitempty"`                      // AI视频模型：seedance / seedance_fast
	Whstr      string `json:"whstr" binding:"required"`                   // 视频宽高比（如 16:9 / 9:16 / 3:2）
}

// VideoGenerationResponse 视频生成响应
type VideoGenerationResponse struct {
	ProjectID   int64  `json:"project_id"`   // 项目 ID
	ProjectName string `json:"project_name"` // 项目名称
	Status      string `json:"status"`       // 项目状态
	CreatedAt   int64  `json:"created_at"`   // 创建时间（Unix 时间戳）
}

// VideoProjectStatus 视频项目状态查询响应
type VideoProjectStatus struct {
	ProjectID   int64  `json:"project_id"`   // 项目 ID
	ProjectName string `json:"project_name"` // 项目名称
	Status      string `json:"status"`       // 项目状态
	ErrorMsg    string `json:"error_msg,omitempty"` // 失败原因
	Progress    string `json:"progress,omitempty"`  // 进度描述

	// 基础信息
	ProductImgUrl string `json:"product_img_url,omitempty"` // 产品图
	Brand         string `json:"brand,omitempty"`           // 品牌
	ProductName   string `json:"product_name,omitempty"`    // 产品名

	// Coze 回调结果
	MainImageUrl    string `json:"main_image_url,omitempty"`     // 主分镜图 URL
	MainImageAssetId string `json:"main_image_asset_id,omitempty"` // 主分镜图资产 ID
	GeneratedResult string `json:"generated_result,omitempty"`   // Coze 回调原始 JSON

	// 视频结果
	FirstVideoUrl string `json:"first_video_url,omitempty"` // 第一段视频 URL

	CreatedAt int64 `json:"created_at"` // 创建时间
	UpdatedAt int64 `json:"updated_at"` // 更新时间
}
