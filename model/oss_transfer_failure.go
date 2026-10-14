package model

import "time"

// OssTransferFailure 对象存储转存失败记录。
//
// 记录粒度：**每次失败写入一条**（含重试）。
// 这样做的原因：
//  1. 可看到完整重试轨迹 —— 第 1/2/3 次的失败原因可能不同
//     （例如前两次是上游超时，第三次才是磁盘满），只保留最后一次会误判根因。
//  2. 纯 INSERT，无需先 SELECT 再决定 INSERT/UPDATE，实现简单且无并发竞态。
//
// 表体积：受重试上限（最多 3 次）约束，单任务最多 3 行，不会无限增长。
type OssTransferFailure struct {
	Id        int64 `json:"id" gorm:"primaryKey"`
	CreatedAt int64 `json:"created_at" gorm:"bigint;index:idx_oss_fail_created_id,priority:1"` // 本次失败时间（Unix 秒）

	TaskId      string `json:"task_id" gorm:"type:varchar(191);index;index:idx_oss_fail_task_attempt,priority:1"` // 任务 ID
	ChannelId   int    `json:"channel_id" gorm:"index"`                                                           // 渠道 ID
	ChannelType int    `json:"channel_type" gorm:"index"`                                                         // 渠道类型（预留，当前恒为 Td）

	SourceUrl string `json:"source_url" gorm:"type:text"`         // 上游原始 URL
	ObjectKey string `json:"object_key" gorm:"type:varchar(255)"` // 计划写入的对象键

	FailReason string `json:"fail_reason" gorm:"type:text"`             // 失败原因（完整错误信息）
	FailStage  string `json:"fail_stage" gorm:"type:varchar(30);index"` // 失败阶段：precheck / download / upload

	AttemptNo int `json:"attempt_no" gorm:"index;index:idx_oss_fail_task_attempt,priority:2"` // 第几次尝试（1/2/3）
	IsFinal   int `json:"is_final" gorm:"type:tinyint;default:0;index"`                       // 是否已达重试上限（1=不再重试）

	MediaType string `json:"media_type" gorm:"type:varchar(10);index"` // image / video
	FileSize  int64  `json:"file_size" gorm:"default:0"`               // 源文件大小（字节），0=未知
}

func (OssTransferFailure) TableName() string {
	return "oss_transfer_failures"
}

// 转存失败阶段常量。
// 区分阶段的目的：下载失败多为上游/网络问题，上传失败多为对象存储配置/磁盘问题，
// 两者排查方向完全不同。
const (
	OssFailStagePrecheck = "precheck" // 转存前置检查失败（体积超限、内容类型异常等）
	OssFailStageDownload = "download" // 从上游下载源文件失败
	OssFailStageUpload   = "upload"   // 上传到对象存储失败
)

// 媒体类型常量
const (
	OssMediaTypeImage = "image"
	OssMediaTypeVideo = "video"
)

// RecordOssTransferFailure 写入一条转存失败记录。
//
// 失败记录本身不应影响主流程：调用方应忽略本函数返回的错误（仅记录日志），
// 因为写入失败记录失败不应导致业务逻辑中断。
func RecordOssTransferFailure(f *OssTransferFailure) error {
	if f == nil {
		return nil
	}
	if f.CreatedAt == 0 {
		f.CreatedAt = time.Now().Unix()
	}
	return DB.Create(f).Error
}

// GetOssFailuresByTaskId 查询某任务的全部失败记录（按尝试次数升序）。
// 用于查看同一任务多次失败的原因演变。
func GetOssFailuresByTaskId(taskId string) ([]*OssTransferFailure, error) {
	var records []*OssTransferFailure
	err := DB.Where("task_id = ?", taskId).Order("attempt_no ASC").Find(&records).Error
	return records, err
}

// IsOssTransferFinallyFailed 判断某任务是否已因达到重试上限而最终失败。
// 依据 is_final 标记，用于避免把「重试后成功」的任务误判为失败。
func IsOssTransferFinallyFailed(taskId string) (bool, error) {
	var count int64
	err := DB.Model(&OssTransferFailure{}).
		Where("task_id = ? AND is_final = 1", taskId).
		Count(&count).Error
	return count > 0, err
}
