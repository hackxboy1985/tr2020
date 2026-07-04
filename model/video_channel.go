package model

import (
	"math/rand"
	"strings"

	"gorm.io/gorm"
)

// VideoChannel 视频生成渠道配置
type VideoChannel struct {
	Id              int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Name            string `json:"name" gorm:"type:varchar(100);not null"`
	ChannelType     string `json:"channel_type" gorm:"type:varchar(20);index;not null"` // 'coze' 或 'platform'
	BaseURL         string `json:"base_url" gorm:"type:varchar(512)"`
	ApiKey          string `json:"api_key" gorm:"type:text"`
	ApiSecret       string `json:"api_secret" gorm:"type:text"`           // webhook 签名密钥
	WorkflowId      string `json:"workflow_id" gorm:"type:varchar(255)"`   // Coze 专用
	CreatePath      string `json:"create_path" gorm:"type:varchar(512)"`   // 创建项目接口路径，如 /v1/workflow/run
	StatusQueryPath string `json:"status_query_path" gorm:"type:varchar(512)"` // 状态查询路径模板，{id} 替换为 remote_project_id
	Groups          string `json:"groups" gorm:"type:varchar(255);default:''"` // 逗号分隔，空=所有组可用
	Weight          int    `json:"weight" gorm:"default:1"`
	Enabled         int    `json:"enabled" gorm:"type:tinyint;default:1;index"`
	ModelMapping    string `json:"model_mapping" gorm:"type:text"` // JSON映射: {"seedance2.0":"42","seedance2.0fast":"44"}
	PricePerSecond  int    `json:"price_per_second" gorm:"type:int;default:1"`      // 每秒价格（积分），1≈1元
	PreDeductQuota  int    `json:"pre_deduct_quota" gorm:"type:int;default:0"`      // 预扣积分，0则用duration*price_per_second
	RateLimit       int    `json:"rate_limit" gorm:"type:int;default:1"`            // QPS限制，0不限制
	Remark          string `json:"remark" gorm:"type:varchar(255)"`
	SaveRequestResponse int `json:"save_request_response" gorm:"type:tinyint;default:0"` // 1=保存请求和响应体
	CreatedAt       int64  `json:"created_at" gorm:"bigint;autoCreateTime"`
	UpdatedAt       int64  `json:"updated_at" gorm:"bigint;autoUpdateTime"`
}

func (VideoChannel) TableName() string {
	return "video_channels"
}

// HasGroup 判断渠道是否对指定用户组开放
// groups 为空字符串时不匹配任何组（与现有 AI 渠道 Group 字段语义一致）
func (ch *VideoChannel) HasGroup(group string) bool {
	if ch.Groups == "" {
		return false
	}
	for _, g := range strings.Split(ch.Groups, ",") {
		if strings.TrimSpace(g) == group {
			return true
		}
	}
	return false
}

// GetCreateURL 返回完整的创建接口 URL
func (ch *VideoChannel) GetCreateURL() string {
	path := ch.CreatePath
	if path == "" {
		if ch.ChannelType == "coze" {
			path = "/v1/workflow/run"
		} else {
			path = "/api/video/create"
		}
	}
	return ch.BaseURL + path
}

// GetStatusQueryURL 返回完整的状态查询 URL（替换 {id} 占位符）
func (ch *VideoChannel) GetStatusQueryURL(remoteId string) string {
	path := ch.StatusQueryPath
	if path == "" {
		if ch.ChannelType == "coze" {
			path = "/v1/workflow/run/{id}"
		} else {
			path = "/api/video/projects/{id}"
		}
	}
	return ch.BaseURL + strings.ReplaceAll(path, "{id}", remoteId)
}

func CreateVideoChannel(ch *VideoChannel) error {
	return DB.Create(ch).Error
}

func GetVideoChannelById(id int) (*VideoChannel, error) {
	var ch VideoChannel
	err := DB.First(&ch, id).Error
	return &ch, err
}

func GetAllVideoChannels() ([]*VideoChannel, error) {
	var channels []*VideoChannel
	err := DB.Order("id ASC").Find(&channels).Error
	return channels, err
}

// GetEnabledVideoChannelsForGroup 获取指定用户组可用的启用渠道
// 注意：组匹配使用 HasGroup() 精确比较，不使用 SQL LIKE，避免子串误匹配（如 supervip 匹配 vip）
func GetEnabledVideoChannelsForGroup(userGroup, channelType string) ([]*VideoChannel, error) {
	var channels []*VideoChannel
	err := DB.Where("enabled = 1").Order("id ASC").Find(&channels).Error
	if err != nil {
		return nil, err
	}

	// 过滤：渠道类型 + 用户组权限
	var result []*VideoChannel
	for _, ch := range channels {
		if channelType != "" && ch.ChannelType != channelType {
			continue
		}
		if ch.HasGroup(userGroup) {
			result = append(result, ch)
		}
	}
	return result, nil
}

// SelectVideoChannel 按用户组和可选渠道类型，按权重随机选一个渠道
func SelectVideoChannel(userGroup, channelType string) (*VideoChannel, error) {
	channels, err := GetEnabledVideoChannelsForGroup(userGroup, channelType)
	if err != nil {
		return nil, err
	}
	if len(channels) == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	// 计算总权重
	totalWeight := 0
	for _, ch := range channels {
		w := ch.Weight
		if w <= 0 {
			w = 1
		}
		totalWeight += w
	}

	// 按权重随机选
	r := rand.Intn(totalWeight)
	cumulative := 0
	for _, ch := range channels {
		w := ch.Weight
		if w <= 0 {
			w = 1
		}
		cumulative += w
		if r < cumulative {
			return ch, nil
		}
	}

	return channels[len(channels)-1], nil
}

func UpdateVideoChannel(ch *VideoChannel) error {
	return DB.Save(ch).Error
}

func UpdateVideoChannelFields(id int, updates map[string]interface{}) error {
	return DB.Model(&VideoChannel{}).Where("id = ?", id).Updates(updates).Error
}

func DeleteVideoChannel(id int) error {
	return DB.Delete(&VideoChannel{}, id).Error
}
