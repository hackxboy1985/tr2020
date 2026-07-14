package model

import (
	"time"
)

// SeedanceFaceVerification 人脸认证任务
type SeedanceFaceVerification struct {
	ID             int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID         int    `json:"user_id" gorm:"index"`
	ChannelID      int    `json:"channel_id" gorm:"index"`
	VerificationID string `json:"verification_id" gorm:"type:varchar(191);uniqueIndex"`
	Status         string `json:"status" gorm:"type:varchar(30);index"` // waiting_user/callback_received/resolving/verified/failed/expired
	H5URL          string `json:"h5_url" gorm:"type:text"`
	GroupID        string `json:"group_id" gorm:"type:varchar(191)"` // verified 后上游返回的 group_id
	ExpiresAt      int64  `json:"expires_at"`
	RawData        string `json:"raw_data" gorm:"type:text"`
	CreatedAt      int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      int64  `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt      int64  `json:"deleted_at" gorm:"index;default:0"`
}

func (SeedanceFaceVerification) TableName() string {
	return "seedance_face_verifications"
}

func CreateSeedanceFaceVerification(v *SeedanceFaceVerification) error {
	return DB.Create(v).Error
}

func GetSeedanceFaceVerificationByID(id int64, userID int) (*SeedanceFaceVerification, error) {
	var v SeedanceFaceVerification
	err := DB.Where("id = ? AND user_id = ? AND deleted_at = 0", id, userID).First(&v).Error
	return &v, err
}

func GetSeedanceFaceVerificationByVerificationID(verificationID string, userID int) (*SeedanceFaceVerification, error) {
	var v SeedanceFaceVerification
	err := DB.Where("verification_id = ? AND user_id = ? AND deleted_at = 0", verificationID, userID).First(&v).Error
	return &v, err
}

func UpdateSeedanceFaceVerification(v *SeedanceFaceVerification) error {
	return DB.Save(v).Error
}

func UpdateSeedanceFaceVerificationStatus(id int64, status, groupID, rawData string) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now().Unix(),
	}
	if groupID != "" {
		updates["group_id"] = groupID
	}
	if rawData != "" {
		updates["raw_data"] = rawData
	}
	return DB.Model(&SeedanceFaceVerification{}).Where("id = ?", id).Updates(updates).Error
}

func ListSeedanceFaceVerifications(userID int, page, pageSize int) ([]*SeedanceFaceVerification, int64, error) {
	var items []*SeedanceFaceVerification
	var total int64
	query := DB.Model(&SeedanceFaceVerification{}).Where("user_id = ? AND deleted_at = 0", userID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error
	return items, total, err
}

func ListAllSeedanceFaceVerifications(page, pageSize int, userID int) ([]*SeedanceFaceVerification, int64, error) {
	var items []*SeedanceFaceVerification
	var total int64
	query := DB.Model(&SeedanceFaceVerification{}).Where("deleted_at = 0")
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error
	return items, total, err
}

func SoftDeleteSeedanceFaceVerification(id int64, userID int) error {
	return DB.Model(&SeedanceFaceVerification{}).
		Where("id = ? AND user_id = ? AND deleted_at = 0", id, userID).
		Update("deleted_at", time.Now().Unix()).Error
}
