package model

import (
	"github.com/QuantumNous/new-api/common"
)

type ChannelTestHistory struct {
	Id           int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	ChannelId    int    `json:"channel_id" gorm:"index:idx_channel_tested,priority:1"`
	Success      bool   `json:"success"`
	ResponseTime int    `json:"response_time"` // 响应时间（毫秒），失败时为 0
	TestModel    string `json:"test_model" gorm:"type:varchar(128)"`
	TestedAt     int64  `json:"tested_at" gorm:"index:idx_channel_tested,priority:2"`
}

func (ChannelTestHistory) TableName() string {
	return "channel_test_histories"
}

// RecordChannelTestHistory 记录渠道测试历史
func RecordChannelTestHistory(channelId int, success bool, responseTime int, testModel string) error {
	history := ChannelTestHistory{
		ChannelId:    channelId,
		Success:      success,
		ResponseTime: responseTime,
		TestModel:    testModel,
		TestedAt:     common.GetTimestamp(),
	}

	err := DB.Create(&history).Error
	if err != nil {
		return err
	}

	// 保留最近 100 条，删除更早的记录
	go cleanupOldHistory(channelId)

	return nil
}

// cleanupOldHistory 清理旧的测试历史，每个渠道保留最近 100 条
func cleanupOldHistory(channelId int) {
	var count int64
	DB.Model(&ChannelTestHistory{}).Where("channel_id = ?", channelId).Count(&count)

	if count > 100 {
		// 找出第 100 条记录的时间戳
		var history ChannelTestHistory
		DB.Where("channel_id = ?", channelId).
			Order("tested_at DESC").
			Offset(100).
			Limit(1).
			First(&history)

		// 删除比第 100 条更早的记录
		DB.Where("channel_id = ? AND tested_at < ?", channelId, history.TestedAt).
			Delete(&ChannelTestHistory{})
	}
}

// GetChannelTestHistories 获取渠道测试历史（最近 N 条）
func GetChannelTestHistories(channelId int, limit int) ([]ChannelTestHistory, error) {
	var histories []ChannelTestHistory
	err := DB.Where("channel_id = ?", channelId).
		Order("tested_at DESC").
		Limit(limit).
		Find(&histories).Error
	return histories, err
}

// GetMultiChannelTestHistories 获取多个渠道的测试历史
func GetMultiChannelTestHistories(channelIds []int, limit int) (map[int][]ChannelTestHistory, error) {
	var histories []ChannelTestHistory
	err := DB.Where("channel_id IN ?", channelIds).
		Order("tested_at DESC").
		Find(&histories).Error

	if err != nil {
		return nil, err
	}

	// 按渠道分组
	result := make(map[int][]ChannelTestHistory)
	for _, h := range histories {
		result[h.ChannelId] = append(result[h.ChannelId], h)
	}

	// 每个渠道限制数量
	for channelId := range result {
		if len(result[channelId]) > limit {
			result[channelId] = result[channelId][:limit]
		}
	}

	return result, nil
}

// GetMostRecentChannelTest 获取渠道最近 N 秒内的成功测试记录（用于定时测试优化）
func GetMostRecentChannelTest(channelId int, withinSeconds int64) (*ChannelTestHistory, error) {
	var history ChannelTestHistory
	cutoff := common.GetTimestamp() - withinSeconds

	err := DB.Where("channel_id = ? AND tested_at >= ? AND success = ?", channelId, cutoff, true).
		Order("tested_at DESC").
		First(&history).Error

	if err != nil {
		return nil, err
	}

	return &history, nil
}
