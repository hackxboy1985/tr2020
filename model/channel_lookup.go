package model

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

// GetGroupModelChannelIDs 返回指定分组下某模型当前可用的渠道 ID 列表，
// 顺序与渠道缓存一致（优先级降序，同优先级内按权重降序）。
//
// 用途（过载休眠池）：
//   - 计算某模型所有可用渠道，判断是否「所有渠道都在休眠」；
//   - 计算跳过休眠渠道后实际会走到的候选渠道。
//
// 只读查询，不选择渠道、不改变任何状态。
func GetGroupModelChannelIDs(group string, modelName string) []int {
	if group == "" || modelName == "" {
		return nil
	}
	if common.MemoryCacheEnabled {
		return getGroupModelChannelIDsFromCache(group, modelName)
	}
	return getGroupModelChannelIDsFromDB(group, modelName)
}

func getGroupModelChannelIDsFromCache(group string, modelName string) []int {
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	if group2model2channels == nil {
		return nil
	}
	ids := group2model2channels[group][modelName]
	if len(ids) == 0 {
		normalized := ratio_setting.FormatMatchingModelName(modelName)
		if normalized != "" && normalized != modelName {
			ids = group2model2channels[group][normalized]
		}
	}
	if len(ids) == 0 {
		return nil
	}
	result := make([]int, 0, len(ids))
	for _, id := range ids {
		if _, ok := channelsIDM[id]; !ok {
			continue
		}
		result = append(result, id)
	}
	return result
}

func getGroupModelChannelIDsFromDB(group string, modelName string) []int {
	// 极端情况下（例如单元测试或数据库尚未初始化）DB 为 nil，直接返回空结果而不是 panic。
	if DB == nil {
		return nil
	}
	var abilities []Ability
	err := DB.Model(&Ability{}).
		Where(commonGroupCol+" = ? and model = ? and enabled = ?", group, modelName, true).
		Order("priority DESC, weight DESC, channel_id ASC").
		Find(&abilities).Error
	if err != nil {
		return nil
	}
	result := make([]int, 0, len(abilities))
	seen := make(map[int]struct{}, len(abilities))
	for _, ability := range abilities {
		if ability.ChannelId <= 0 {
			continue
		}
		if _, ok := seen[ability.ChannelId]; ok {
			continue
		}
		seen[ability.ChannelId] = struct{}{}
		result = append(result, ability.ChannelId)
	}
	return result
}

// GetGroupModelChannelInfos 返回渠道 ID 对应的精简渠道信息（名称/类型/优先级/权重）。
// 用于休眠池管理接口展示与「跳过休眠后走哪个渠道」的计算，避免泄露渠道密钥。
func GetGroupModelChannelInfos(group string, modelName string) []ChannelBrief {
	ids := GetGroupModelChannelIDs(group, modelName)
	if len(ids) == 0 {
		return nil
	}
	briefs := make([]ChannelBrief, 0, len(ids))
	for _, id := range ids {
		channel, err := CacheGetChannel(id)
		if err != nil || channel == nil {
			continue
		}
		briefs = append(briefs, ChannelBrief{
			ChannelId:   channel.Id,
			ChannelName: channel.Name,
			ChannelType: channel.Type,
			Priority:    channel.GetPriority(),
			Weight:      channel.GetWeight(),
		})
	}
	return briefs
}

// GetModelGroups 返回当前渠道缓存中包含该模型的所有分组名（用于休眠池页面在缺少分组信息时兜底计算）。
func GetModelGroups(modelName string) []string {
	if modelName == "" {
		return nil
	}
	if !common.MemoryCacheEnabled {
		if DB == nil {
			return nil
		}
		var groups []string
		DB.Model(&Ability{}).
			Where("model = ? and enabled = ?", modelName, true).
			Distinct(commonGroupCol).
			Pluck(commonGroupCol, &groups)
		return groups
	}
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()
	if group2model2channels == nil {
		return nil
	}
	groups := make([]string, 0)
	for group, model2channels := range group2model2channels {
		if len(model2channels[modelName]) > 0 {
			groups = append(groups, group)
			continue
		}
		normalized := ratio_setting.FormatMatchingModelName(modelName)
		if normalized != "" && normalized != modelName && len(model2channels[normalized]) > 0 {
			groups = append(groups, group)
		}
	}
	return groups
}

// ChannelBrief 渠道精简信息（不含密钥等敏感字段）
type ChannelBrief struct {
	ChannelId   int    `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	ChannelType int    `json:"channel_type"`
	Priority    int64  `json:"priority"`
	Weight      int    `json:"weight"`
}
