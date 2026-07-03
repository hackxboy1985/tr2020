package model

import (
	"time"
)

// VideoProject 视频项目模型
type VideoProject struct {
	Id        int64     `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// 项目基础信息
	ProjectName     string `gorm:"type:varchar(255);index" json:"project_name"`
	UserId          int    `gorm:"index" json:"user_id"`
	Username        string `gorm:"index;default:''" json:"username"`
	ChannelId       int    `gorm:"index;default:0" json:"channel_id"`              // 关联 video_channels.id
	ChannelType     string `gorm:"type:varchar(20);index;default:'platform'" json:"channel_type"` // 创建时快照，后续不更新
	RemoteProjectId string `gorm:"type:varchar(255);index" json:"remote_project_id"`

	// 广告基础信息
	ProductImgUrl string `gorm:"type:text" json:"product_img_url"` // 产品图 OSS URL
	Brand         string `gorm:"type:varchar(50)" json:"brand"`    // 产品品牌
	ProductName   string `gorm:"type:varchar(50)" json:"product_name"` // 产品名
	Tagline       string `gorm:"type:varchar(255)" json:"tagline,omitempty"` // 宣传语
	SellingPoints string `gorm:"type:text" json:"selling_points,omitempty"`  // 产品卖点

	// 创意方向
	Prompt   string `gorm:"type:text" json:"prompt"`                // 创意 prompt
	Vtype    string `gorm:"type:varchar(50)" json:"vtype"`          // 视频类型
	VtypeAdd string `gorm:"type:varchar(50)" json:"vtype_add,omitempty"` // 剧情子类型
	Language string `gorm:"type:varchar(20)" json:"language,omitempty"`  // 广告语言
	Platform string `gorm:"type:varchar(50)" json:"platform,omitempty"`  // 投放平台
	Region   string `gorm:"type:varchar(50)" json:"region,omitempty"`    // 投放地区

	// 角色与参考
	Roles        string `gorm:"type:text" json:"roles,omitempty"`         // 出镜角色列表 JSON（旧格式）
	SelectAudios string `gorm:"type:text" json:"select_audios,omitempty"` // 可选音色列表 JSON（旧格式）
	MediaList    string `gorm:"type:text" json:"media_list,omitempty"`    // OpenAPI 媒体列表 JSON

	// 输出配置
	Duration   int    `gorm:"type:int" json:"duration"`            // 目标视频时长（秒）
	Resolution string `gorm:"type:varchar(20)" json:"resolution"`  // 输出分辨率
	VideoModel string `gorm:"type:varchar(50)" json:"video_model,omitempty"` // AI视频模型
	Whstr      string `gorm:"type:varchar(20)" json:"whstr"`       // 视频宽高比

	// 回调结果
	MainImageUrl     string `gorm:"type:text" json:"main_image_url,omitempty"`      // 主分镜图 URL
	MainImageAssetId string `gorm:"type:varchar(255)" json:"main_image_asset_id,omitempty"` // 主分镜图资产 ID
	GeneratedResult  string `gorm:"type:text" json:"generated_result,omitempty"`    // 回调原始 JSON
	FirstVideoUrl    string `gorm:"type:text" json:"first_video_url,omitempty"`     // 第一个视频 URL

	// 系统字段
	Status   string `gorm:"type:varchar(50);index" json:"status"`      // 项目状态
	ErrorMsg string `gorm:"type:text" json:"error_msg,omitempty"`      // 失败原因
	Progress string `gorm:"type:varchar(255)" json:"progress,omitempty"` // 进度信息
	Deleted  int    `gorm:"type:tinyint;default:0;index" json:"deleted"` // 0=未删 1=软删
}

// VideoProjectStatus 视频项目状态常量
const (
	VideoProjectStatusCreated           = "CREATED"              // 已创建，等待 Coze 处理
	VideoProjectStatusRunning       = "RUNNING"         // 上游工作流执行中
	VideoProjectStatusVideoProcessing   = "VIDEO_PROCESSING"     // 视频已生成，等待拼接
	VideoProjectStatusVideoConcat       = "VIDEO_CONCAT"         // 拼接完成，等待 OSS 上传
	VideoProjectStatusOneClickGenerated = "ONE_CLICK_GENERATED"  // OSS 上传完成，全流程结束
	VideoProjectStatusVideoPreparing    = "VIDEO_PREPARING"      // 拼接失败，需手动重试
	VideoProjectStatusFailed            = "FAILED"               // 生成失败
)

func (VideoProject) TableName() string {
	return "video_projects"
}

// CreateVideoProject 创建视频项目
func CreateVideoProject(project *VideoProject) error {
	return DB.Create(project).Error
}

// GetVideoProjectById 根据 ID 获取视频项目
func GetVideoProjectById(id int64, userId int) (*VideoProject, error) {
	var project VideoProject
	err := DB.Where("id = ? AND user_id = ? AND deleted = 0", id, userId).First(&project).Error
	if err != nil {
		return nil, err
	}
	return &project, nil
}

// GetVideoProjectByIdAdmin 管理员根据 ID 获取视频项目
func GetVideoProjectByIdAdmin(id int64) (*VideoProject, error) {
	var project VideoProject
	err := DB.Where("id = ? AND deleted = 0", id).First(&project).Error
	if err != nil {
		return nil, err
	}
	return &project, nil
}

// GetVideoProjectByRemoteId 根据渠道类型和远程ID获取项目（旧接口，保留兼容）
func GetVideoProjectByRemoteId(channelType, remoteId string) (*VideoProject, error) {
	var project VideoProject
	err := DB.Where("channel_type = ? AND remote_project_id = ? AND deleted = 0",
		channelType, remoteId).First(&project).Error
	if err != nil {
		return nil, err
	}
	return &project, nil
}

// GetVideoProjectByChannelAndRemoteId 根据渠道ID和远程ID获取项目（精确匹配）
func GetVideoProjectByChannelAndRemoteId(channelId int, remoteId string) (*VideoProject, error) {
	var project VideoProject
	err := DB.Where("channel_id = ? AND remote_project_id = ? AND deleted = 0",
		channelId, remoteId).First(&project).Error
	if err != nil {
		return nil, err
	}
	return &project, nil
}

// GetUserVideoProjects 获取用户的视频项目列表
func GetUserVideoProjects(userId int, offset int, limit int) ([]*VideoProject, int64, error) {
	var projects []*VideoProject
	var total int64

	db := DB.Model(&VideoProject{}).Where("user_id = ? AND deleted = 0", userId)

	err := db.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = db.Order("created_at DESC").Offset(offset).Limit(limit).Find(&projects).Error
	if err != nil {
		return nil, 0, err
	}

	return projects, total, nil
}

// GetAllVideoProjects 管理员获取所有视频项目列表
func GetAllVideoProjects(offset int, limit int, status string) ([]*VideoProject, int64, error) {
	var projects []*VideoProject
	var total int64

	db := DB.Model(&VideoProject{}).Where("deleted = 0")

	if status != "" {
		db = db.Where("status = ?", status)
	}

	err := db.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = db.Order("created_at DESC").Offset(offset).Limit(limit).Find(&projects).Error
	if err != nil {
		return nil, 0, err
	}

	return projects, total, nil
}

// UpdateVideoProject 更新视频项目
func UpdateVideoProject(project *VideoProject) error {
	return DB.Save(project).Error
}

// UpdateVideoProjectStatus 更新视频项目状态
func UpdateVideoProjectStatus(id int64, status string, errorMsg string) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}
	if errorMsg != "" {
		updates["error_msg"] = errorMsg
	}
	return DB.Model(&VideoProject{}).Where("id = ?", id).Updates(updates).Error
}

// UpdateVideoProjectFields 更新视频项目的指定字段
func UpdateVideoProjectFields(id int64, updates map[string]interface{}) error {
	if updates == nil {
		updates = make(map[string]interface{})
	}
	updates["updated_at"] = time.Now()
	return DB.Model(&VideoProject{}).Where("id = ?", id).Updates(updates).Error
}

// UpdateVideoProjectCozeResult 更新 Coze 回调结果
func UpdateVideoProjectCozeResult(id int64, mainImageUrl string, mainImageAssetId string, generatedResult string) error {
	updates := map[string]interface{}{
		"main_image_url":      mainImageUrl,
		"main_image_asset_id": mainImageAssetId,
		"generated_result":    generatedResult,
		"updated_at":          time.Now(),
	}
	return DB.Model(&VideoProject{}).Where("id = ?", id).Updates(updates).Error
}

// DeleteVideoProject 软删除视频项目
func DeleteVideoProject(id int64, userId int) error {
	return DB.Model(&VideoProject{}).Where("id = ? AND user_id = ?", id, userId).Update("deleted", 1).Error
}

// DeleteVideoProjectAdmin 管理员软删除视频项目
func DeleteVideoProjectAdmin(id int64) error {
	return DB.Model(&VideoProject{}).Where("id = ?", id).Update("deleted", 1).Error
}

// InitVideoProjectTable 初始化视频项目表
func InitVideoProjectTable() error {
	if !DB.Migrator().HasTable(&VideoProject{}) {
		if err := DB.Migrator().CreateTable(&VideoProject{}); err != nil {
			return err
		}
	}
	return nil
}
