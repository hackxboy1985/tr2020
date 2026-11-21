package service

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/sleeppool"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

// 过载休眠池的服务层封装。
//
// 关于「跳过渠道」的记录方式：不再为每次跳过单独写一条日志（会显著增加日志条数），
// 而是把跳过信息暂存在 gin.Context 里，最终由写日志的一侧通过
// model.InjectOverloadSleepInfoIntoOther 注入到「实际请求渠道」的那条日志 other 字段中。

// overloadSleepPoolEnabled 过载休眠池是否可用于当前请求
func overloadSleepPoolEnabled() bool {
	return sleeppool.Enabled()
}

// IsOverloadSleepPoolActive 休眠池是否生效（总开关打开 + Redis 可用）。
// 渠道选择用它来决定是否走「跳过休眠渠道」的路径。
func IsOverloadSleepPoolActive() bool {
	return sleeppool.Enabled()
}

// GetOverloadSleepExcludedChannels 返回当前请求需要跳过的渠道集合。
// modelName 使用「用户请求的模型名」（OriginModelName），与写入休眠池时的模型名保持一致。
//
// 注意：这里无法知道哪些渠道正在休眠（每个 (模型, 渠道) 是独立 key），
// 因此必须由调用方给出「候选渠道 ID」，本函数用一次 MGET 精确查询。
// 这样做的好处是热路径固定 1 次 Redis 往返，不会随休眠池规模增长而变慢。
func GetOverloadSleepExcludedChannels(modelName string, candidateChannelIds []int) map[int]*sleeppool.Entry {
	if modelName == "" || len(candidateChannelIds) == 0 || !overloadSleepPoolEnabled() {
		return nil
	}
	return sleeppool.GetSleepingChannels(modelName, candidateChannelIds)
}

// IsChannelOverloadSleeping 判断亲和/锁定渠道是否正处于过载休眠中。
// 命中时会记录跳过信息，保证「跳过」在日志 other 中可追溯。
func IsChannelOverloadSleeping(c *gin.Context, modelName string, group string, channel *model.Channel) bool {
	if channel == nil || modelName == "" || !overloadSleepPoolEnabled() {
		return false
	}
	sleeping, entry := sleeppool.IsSleeping(modelName, channel.Id)
	if !sleeping {
		return false
	}
	RecordOverloadSleepSkip(c, modelName, group, map[int]*sleeppool.Entry{channel.Id: entry})
	return true
}

// RecordOverloadSleepSkip 在 gin.Context 中记录被跳过的渠道（同一请求内同一渠道只记录一次）。
func RecordOverloadSleepSkip(c *gin.Context, modelName string, group string, entries map[int]*sleeppool.Entry) {
	if c == nil || len(entries) == 0 {
		return
	}
	now := common.GetTimestamp()
	items := make([]model.OverloadSleepSkippedChannel, 0, len(entries))
	for channelId, entry := range entries {
		if channelId <= 0 {
			continue
		}
		remaining := int64(0)
		channelName := ""
		channelType := 0
		if entry != nil {
			remaining = entry.SleepUntil - now
			channelName = entry.ChannelName
			channelType = entry.ChannelType
		}
		if remaining < 0 {
			remaining = 0
		}
		items = append(items, model.OverloadSleepSkippedChannel{
			ModelName:        modelName,
			ChannelId:        channelId,
			ChannelName:      channelName,
			ChannelType:      channelType,
			Group:            group,
			RemainingSeconds: remaining,
		})
	}
	before := len(model.GetOverloadSleepSkippedChannels(c))
	model.AddOverloadSleepSkippedChannels(c, items)
	if after := len(model.GetOverloadSleepSkippedChannels(c)); after > before {
		logger.LogInfo(c, fmt.Sprintf("过载休眠池：模型 %s 命中 %d 个休眠渠道，已跳过：%s",
			modelName, len(entries), formatSkippedChannels(entries)))
	}
}

// SetOverloadSleepAllSleeping 标记「全部渠道休眠，兜底取最后一个渠道」。
func SetOverloadSleepAllSleeping(c *gin.Context, modelName string) {
	if c == nil {
		return
	}
	model.SetOverloadSleepAllSleeping(c)
	logger.LogInfo(c, fmt.Sprintf("过载休眠池：模型 %s 的所有可用渠道均在休眠池中，按兜底策略取最后一个渠道尝试", modelName))
}

// IsOverloadSleepAllSleeping 本次请求是否触发了「全部渠道休眠」兜底。
func IsOverloadSleepAllSleeping(c *gin.Context) bool {
	return model.IsOverloadSleepAllSleeping(c)
}

// MarkOverloadSleep 上游返回过载状态码时，把 (模型, 渠道) 放入休眠池。
// 返回是否实际写入。
func MarkOverloadSleep(c *gin.Context, modelName string, channel *model.Channel, group string, statusCode int) bool {
	if channel == nil || modelName == "" {
		return false
	}
	if !sleeppool.Available() || !operation_setting.IsOverloadSleepEnabled() {
		return false
	}
	entry := sleeppool.Entry{
		ModelName:         modelName,
		ChannelId:         channel.Id,
		ChannelName:       channel.Name,
		ChannelType:       channel.Type,
		Group:             group,
		TriggerStatusCode: statusCode,
	}
	if err := sleeppool.Mark(entry, operation_setting.GetOverloadSleepMinutes()); err != nil {
		if c != nil {
			logger.LogError(c, fmt.Sprintf("过载休眠池：写入失败（模型 %s，渠道 #%d）：%s", modelName, channel.Id, err.Error()))
		} else {
			common.SysError(fmt.Sprintf("过载休眠池：写入失败（模型 %s，渠道 #%d）：%s", modelName, channel.Id, err.Error()))
		}
		return false
	}
	if c != nil {
		logger.LogInfo(c, fmt.Sprintf("过载休眠池：模型 %s 渠道 #%d（%s）因状态码 %d 进入休眠，时长 %d 分钟",
			modelName, channel.Id, channel.Name, statusCode, operation_setting.GetOverloadSleepMinutes()))
	}
	return true
}

func formatSkippedChannels(entries map[int]*sleeppool.Entry) string {
	parts := make([]string, 0, len(entries))
	for channelId, entry := range entries {
		name := ""
		if entry != nil {
			name = entry.ChannelName
		}
		parts = append(parts, fmt.Sprintf("#%d(%s)", channelId, name))
	}
	return strings.Join(parts, ",")
}
