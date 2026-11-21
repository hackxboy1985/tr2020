package model

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"

	"github.com/gin-gonic/gin"
)

// OverloadSleepSkippedChannel 「因过载休眠池而跳过渠道」的记录项。
//
// 设计说明：不再为每一次跳过单独写一条日志（那样一个请求失败重试多次会产生大量冗余行），
// 而是把跳过信息作为一个字段塞进「实际要请求的那个渠道」的日志 other 里，
// 既能在日志详情里看到「为什么跳过了某些渠道」，又不增加日志条数。
type OverloadSleepSkippedChannel struct {
	ModelName        string `json:"model_name"`
	ChannelId        int    `json:"channel_id"`
	ChannelName      string `json:"channel_name"`
	ChannelType      int    `json:"channel_type"`
	Group            string `json:"group"`
	RemainingSeconds int64  `json:"remaining_seconds"`
}

// 日志 other 中使用的字段名（前端日志详情按此展示）
const (
	OtherKeyOverloadSleepSkipped     = "overload_sleep_skipped"
	OtherKeyOverloadSleepAllSleeping = "overload_sleep_all_sleeping"
)

// AddOverloadSleepSkippedChannels 记录本次请求中被跳过的渠道（同请求内同一渠道只记一次）。
func AddOverloadSleepSkippedChannels(c *gin.Context, items []OverloadSleepSkippedChannel) {
	if c == nil || len(items) == 0 {
		return
	}
	existing := GetOverloadSleepSkippedChannels(c)
	seen := make(map[int]struct{}, len(existing))
	for _, item := range existing {
		seen[item.ChannelId] = struct{}{}
	}
	added := false
	for _, item := range items {
		if item.ChannelId <= 0 {
			continue
		}
		if _, ok := seen[item.ChannelId]; ok {
			continue
		}
		seen[item.ChannelId] = struct{}{}
		existing = append(existing, item)
		added = true
	}
	if !added {
		return
	}
	common.SetContextKey(c, constant.ContextKeyOverloadSleepSkipped, existing)
}

// GetOverloadSleepSkippedChannels 返回本次请求中被跳过的渠道列表。
func GetOverloadSleepSkippedChannels(c *gin.Context) []OverloadSleepSkippedChannel {
	if c == nil {
		return nil
	}
	value, exists := common.GetContextKey(c, constant.ContextKeyOverloadSleepSkipped)
	if !exists {
		return nil
	}
	items, ok := value.([]OverloadSleepSkippedChannel)
	if !ok {
		return nil
	}
	return items
}

// SetOverloadSleepAllSleeping 标记本次请求触发了「全部渠道休眠，兜底取最后一个渠道」。
func SetOverloadSleepAllSleeping(c *gin.Context) {
	if c == nil {
		return
	}
	common.SetContextKey(c, constant.ContextKeyOverloadSleepAllSleeping, true)
}

// IsOverloadSleepAllSleeping 本次请求是否触发了「全部渠道休眠」兜底。
func IsOverloadSleepAllSleeping(c *gin.Context) bool {
	if c == nil {
		return false
	}
	return common.GetContextKeyBool(c, constant.ContextKeyOverloadSleepAllSleeping)
}

// InjectOverloadSleepInfoIntoOther 把「跳过休眠渠道」的信息注入日志 other 字段。
//
// 在写日志前调用：只有本次请求确实跳过过渠道（或触发了全部休眠兜底）才会注入，
// 正常情况下（绝大多数请求）不会给 other 增加任何字段。
func InjectOverloadSleepInfoIntoOther(c *gin.Context, other map[string]interface{}) {
	if c == nil || other == nil {
		return
	}
	skipped := GetOverloadSleepSkippedChannels(c)
	allSleeping := IsOverloadSleepAllSleeping(c)
	if len(skipped) == 0 && !allSleeping {
		return
	}
	if len(skipped) > 0 {
		other[OtherKeyOverloadSleepSkipped] = skipped
	}
	if allSleeping {
		other[OtherKeyOverloadSleepAllSleeping] = true
	}
}
