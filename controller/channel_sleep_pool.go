package controller

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/sleeppool"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
)

// SleepPoolNextChannel 跳过休眠渠道后「会走到」的候选渠道
type SleepPoolNextChannel struct {
	ChannelId   int    `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	Priority    int64  `json:"priority"`
}

// SleepPoolEntryResponse 休眠池条目（管理页面展示用）
type SleepPoolEntryResponse struct {
	ModelName           string                 `json:"model_name"`
	ChannelId           int                    `json:"channel_id"`
	ChannelName         string                 `json:"channel_name"`
	ChannelType         int                    `json:"channel_type"`
	Group               string                 `json:"group"`
	SleepUntil          int64                  `json:"sleep_until"`
	RemainingSeconds    int64                  `json:"remaining_seconds"`
	TriggerStatusCode   int                    `json:"trigger_status_code"`
	TriggerCount        int                    `json:"trigger_count"`
	TotalChannelCount   int                    `json:"total_channel_count"`
	AllChannelsSleeping bool                   `json:"all_channels_sleeping"`
	NextChannels        []SleepPoolNextChannel `json:"next_channels"`
	FallbackChannel     *SleepPoolNextChannel  `json:"fallback_channel"`
}

// GetChannelSleepPool 查询休眠池（管理员）
//
// 支持 ?model_name=xxx 按模型精确检索。由于 key 是
// overload_sleep_pool:model:<模型名>:channel:<渠道ID>，按模型前缀 SCAN 即可
// 列出该模型下所有休眠渠道，无需扫描整个库。
func GetChannelSleepPool(c *gin.Context) {
	modelFilter := strings.TrimSpace(c.Query("model_name"))
	var (
		entries []*sleeppool.Entry
		err     error
	)
	if modelFilter != "" {
		var modelEntries map[int]*sleeppool.Entry
		modelEntries, err = sleeppool.ListModelEntries(modelFilter)
		if err == nil {
			entries = make([]*sleeppool.Entry, 0, len(modelEntries))
			for _, entry := range modelEntries {
				entries = append(entries, entry)
			}
			sort.Slice(entries, func(i, j int) bool {
				return entries[i].ChannelId < entries[j].ChannelId
			})
		}
	} else {
		entries, err = sleeppool.ListAllEntries()
	}
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "获取休眠池失败：" + err.Error(),
			"data":    gin.H{},
		})
		return
	}
	now := common.GetTimestamp()

	// 按模型分组，便于计算「跳过休眠后会走哪个渠道」
	entriesByModel := make(map[string][]*sleeppool.Entry)
	for _, entry := range entries {
		entriesByModel[entry.ModelName] = append(entriesByModel[entry.ModelName], entry)
	}

	// 同一模型可能来自多个分组（auto 分组），这里按模型中出现的分组逐一计算
	modelNextChannels := make(map[string][]SleepPoolNextChannel)
	modelTotalCount := make(map[string]int)
	modelAllSleeping := make(map[string]bool)
	modelFallback := make(map[string]*SleepPoolNextChannel)

	for modelName, modelEntries := range entriesByModel {
		groups := collectEntryGroups(modelEntries)
		if len(groups) == 0 {
			// 条目里没有分组信息（例如写入时分组为空），退回按模型反查分组
			groups = model.GetModelGroups(modelName)
		}
		sleepingIds := make(map[int]struct{}, len(modelEntries))
		for _, entry := range modelEntries {
			sleepingIds[entry.ChannelId] = struct{}{}
		}
		nextChannels, total, allSleeping, fallback := computeModelNextChannels(groups, modelName, sleepingIds)
		modelNextChannels[modelName] = nextChannels
		modelTotalCount[modelName] = total
		modelAllSleeping[modelName] = allSleeping
		modelFallback[modelName] = fallback
	}

	result := make([]SleepPoolEntryResponse, 0, len(entries))
	for _, entry := range entries {
		nextChannels := modelNextChannels[entry.ModelName]
		if nextChannels == nil {
			nextChannels = []SleepPoolNextChannel{}
		}
		result = append(result, SleepPoolEntryResponse{
			ModelName:           entry.ModelName,
			ChannelId:           entry.ChannelId,
			ChannelName:         entry.ChannelName,
			ChannelType:         entry.ChannelType,
			Group:               entry.Group,
			SleepUntil:          entry.SleepUntil,
			RemainingSeconds:    entry.RemainingSeconds(now),
			TriggerStatusCode:   entry.TriggerStatusCode,
			TriggerCount:        entry.TriggerCount,
			TotalChannelCount:   modelTotalCount[entry.ModelName],
			AllChannelsSleeping: modelAllSleeping[entry.ModelName],
			NextChannels:        nextChannels,
			FallbackChannel:     modelFallback[entry.ModelName],
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"enabled":       operation_setting.IsOverloadSleepEnabled(),
			"redis_enabled": sleeppool.Available(),
			"sleep_minutes": operation_setting.GetOverloadSleepMinutes(),
			"status_codes":  operation_setting.GetOverloadSleepSetting().StatusCodes,
			"server_time":   now,
			"entries":       result,
		},
	})
}

func collectEntryGroups(entries []*sleeppool.Entry) []string {
	seen := make(map[string]struct{}, len(entries))
	groups := make([]string, 0, len(entries))
	for _, entry := range entries {
		group := strings.TrimSpace(entry.Group)
		if group == "" {
			continue
		}
		if _, ok := seen[group]; ok {
			continue
		}
		seen[group] = struct{}{}
		groups = append(groups, group)
	}
	sort.Strings(groups)
	return groups
}

// computeModelNextChannels 计算某模型在给定分组下、跳过休眠渠道后会走到的候选渠道。
//
// 规则：
//   - 候选渠道来自渠道缓存（该分组下该模型的全部可用渠道）；
//   - 去掉休眠中的渠道；
//   - 按优先级降序、渠道 ID 升序，最多取 5 个；
//   - 若全部渠道都在休眠，则回落到「最后一个渠道」（优先级最低、同优先级内 channel_id 最大），
//     与请求时的兜底策略保持一致。
func computeModelNextChannels(groups []string, modelName string, sleepingIds map[int]struct{}) ([]SleepPoolNextChannel, int, bool, *SleepPoolNextChannel) {
	nextChannels := make([]SleepPoolNextChannel, 0, 5)
	total := 0
	allSleeping := true
	var fallback *SleepPoolNextChannel

	seenChannel := make(map[int]struct{})
	for _, group := range groups {
		infos := model.GetGroupModelChannelInfos(group, modelName)
		for _, info := range infos {
			if _, ok := seenChannel[info.ChannelId]; ok {
				continue
			}
			seenChannel[info.ChannelId] = struct{}{}
			total++
			if _, sleeping := sleepingIds[info.ChannelId]; sleeping {
				continue
			}
			allSleeping = false
			nextChannels = append(nextChannels, SleepPoolNextChannel{
				ChannelId:   info.ChannelId,
				ChannelName: info.ChannelName,
				Priority:    info.Priority,
			})
		}
	}

	sort.Slice(nextChannels, func(i, j int) bool {
		if nextChannels[i].Priority == nextChannels[j].Priority {
			return nextChannels[i].ChannelId < nextChannels[j].ChannelId
		}
		return nextChannels[i].Priority > nextChannels[j].Priority
	})
	if len(nextChannels) > 5 {
		nextChannels = nextChannels[:5]
	}

	if total == 0 {
		// 渠道缓存里查不到（例如渠道已被删除），不宣称「全部休眠」
		allSleeping = false
	}
	if allSleeping && total > 0 {
		// 全部休眠：按兜底策略给出最后一个渠道
		var last *SleepPoolNextChannel
		for _, group := range groups {
			infos := model.GetGroupModelChannelInfos(group, modelName)
			for _, info := range infos {
				candidate := SleepPoolNextChannel{
					ChannelId:   info.ChannelId,
					ChannelName: info.ChannelName,
					Priority:    info.Priority,
				}
				if last == nil ||
					candidate.Priority < last.Priority ||
					(candidate.Priority == last.Priority && candidate.ChannelId > last.ChannelId) {
					lastCopy := candidate
					last = &lastCopy
				}
			}
		}
		fallback = last
	}
	if nextChannels == nil {
		nextChannels = []SleepPoolNextChannel{}
	}
	return nextChannels, total, allSleeping, fallback
}

// DeleteChannelSleepPoolEntry 从休眠池移除单条记录
func DeleteChannelSleepPoolEntry(c *gin.Context) {
	if strings.TrimSpace(c.Query("all")) == "true" {
		removed, err := sleeppool.ClearAll()
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "清空休眠池失败：" + err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"removed": removed}})
		return
	}

	modelName := strings.TrimSpace(c.Query("model_name"))
	channelIdRaw := strings.TrimSpace(c.Query("channel_id"))
	if modelName == "" || channelIdRaw == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "缺少参数：model_name 和 channel_id，或使用 all=true 清空全部"})
		return
	}
	channelId, err := strconv.Atoi(channelIdRaw)
	if err != nil || channelId <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "参数 channel_id 非法"})
		return
	}
	if !sleeppool.Available() {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": sleeppool.ErrRedisUnavailable.Error()})
		return
	}
	removed, err := sleeppool.Remove(modelName, channelId)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "移除失败：" + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"removed": removed}})
}
