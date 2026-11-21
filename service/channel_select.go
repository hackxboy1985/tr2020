package service

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/sleeppool"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
)

type RetryParam struct {
	Ctx          *gin.Context
	TokenGroup   string
	ModelName    string
	Retry        *int
	resetNextTry bool
	// AllChannelsSleeping 本次选择时该模型（该分组下）全部渠道都处于过载休眠池，
	// 供上层决定是否切换到备用模型。
	AllChannelsSleeping bool
}

func (p *RetryParam) GetRetry() int {
	if p.Retry == nil {
		return 0
	}
	return *p.Retry
}

func (p *RetryParam) SetRetry(retry int) {
	p.Retry = &retry
}

func (p *RetryParam) IncreaseRetry() {
	if p.resetNextTry {
		p.resetNextTry = false
		return
	}
	if p.Retry == nil {
		p.Retry = new(int)
	}
	*p.Retry++
}

func (p *RetryParam) ResetRetryNextTry() {
	p.resetNextTry = true
}

// CacheGetRandomSatisfiedChannel tries to get a random channel that satisfies the requirements.
// 尝试获取一个满足要求的随机渠道。
//
// For "auto" tokenGroup with cross-group Retry enabled:
// 对于启用了跨分组重试的 "auto" tokenGroup：
//
//   - Each group will exhaust all its priorities before moving to the next group.
//     每个分组会用完所有优先级后才会切换到下一个分组。
//
//   - Uses ContextKeyAutoGroupIndex to track current group index.
//     使用 ContextKeyAutoGroupIndex 跟踪当前分组索引。
//
//   - Uses ContextKeyAutoGroupRetryIndex to track the global Retry count when current group started.
//     使用 ContextKeyAutoGroupRetryIndex 跟踪当前分组开始时的全局重试次数。
//
//   - priorityRetry = Retry - startRetryIndex, represents the priority level within current group.
//     priorityRetry = Retry - startRetryIndex，表示当前分组内的优先级级别。
//
//   - When GetRandomSatisfiedChannel returns nil (priorities exhausted), moves to next group.
//     当 GetRandomSatisfiedChannel 返回 nil（优先级用完）时，切换到下一个分组。
//
// 过载休眠池（overload sleep pool）：
// 当总开关打开时，选择渠道会跳过处于休眠中的 (模型, 渠道) 组合，
// 并把这些跳过记录写入 gin.Context，供上层写日志表；
// 若该模型所有渠道都在休眠，则按需求兜底取「最后一个渠道」，保证至少发起一次请求。
//
// Example flow (2 groups, each with 2 priorities, RetryTimes=3):
// 示例流程（2个分组，每个有2个优先级，RetryTimes=3）：
//
//	Retry=0: GroupA, priority0 (startRetryIndex=0, priorityRetry=0)
//	         分组A, 优先级0
//
//	Retry=1: GroupA, priority1 (startRetryIndex=0, priorityRetry=1)
//	         分组A, 优先级1
//
//	Retry=2: GroupA exhausted → GroupB, priority0 (startRetryIndex=2, priorityRetry=0)
//	         分组A用完 → 分组B, 优先级0
//
//	Retry=3: GroupB, priority1 (startRetryIndex=2, priorityRetry=1)
//	         分组B, 优先级1
func CacheGetRandomSatisfiedChannel(param *RetryParam) (*model.Channel, string, error) {
	var channel *model.Channel
	var err error
	selectGroup := param.TokenGroup
	userGroup := common.GetContextKeyString(param.Ctx, constant.ContextKeyUserGroup)

	// 休眠池开关打开时，选择渠道会跳过处于休眠中的 (模型, 渠道)。
	// 注意：休眠条目是「一个 (模型, 渠道) 一个 key」，无法凭模型名直接枚举，
	// 所以这里先取候选渠道（内存缓存，无 IO），再用一次 MGET 精确查询哪些在休眠。
	sleepPoolActive := IsOverloadSleepPoolActive()

	if param.TokenGroup == "auto" {
		if len(setting.GetAutoGroups()) == 0 {
			return nil, selectGroup, errors.New("auto groups is not enabled")
		}
		autoGroups := GetUserAutoGroup(userGroup)

		// startGroupIndex: the group index to start searching from
		// startGroupIndex: 开始搜索的分组索引
		startGroupIndex := 0
		crossGroupRetry := common.GetContextKeyBool(param.Ctx, constant.ContextKeyTokenCrossGroupRetry)

		if lastGroupIndex, exists := common.GetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex); exists {
			if idx, ok := lastGroupIndex.(int); ok {
				startGroupIndex = idx
			}
		}

		for i := startGroupIndex; i < len(autoGroups); i++ {
			autoGroup := autoGroups[i]
			// Calculate priorityRetry for current group
			// 计算当前分组的 priorityRetry
			priorityRetry := param.GetRetry()
			// If moved to a new group, reset priorityRetry and update startRetryIndex
			// 如果切换到新分组，重置 priorityRetry 并更新 startRetryIndex
			if i > startGroupIndex {
				priorityRetry = 0
			}
			logger.LogDebug(param.Ctx, "Auto selecting group: %s, priorityRetry: %d", autoGroup, priorityRetry)

			if sleepPoolActive {
				// auto 分组的语义是「本分组取不到渠道就换下一个分组」，
				// 因此这里的错误与原逻辑一致地吞掉，只记 debug 日志，继续尝试下一个分组。
				channel, err = selectChannelSkippingSleeping(param, autoGroup, priorityRetry)
				if err != nil {
					logger.LogDebug(param.Ctx, "Select channel skipping overload sleeping failed in group %s: %s", autoGroup, err.Error())
					channel = nil
				}
			} else {
				channel, _ = model.GetRandomSatisfiedChannel(autoGroup, param.ModelName, priorityRetry)
			}
			if channel == nil {
				// Current group has no available channel for this model, try next group
				// 当前分组没有该模型的可用渠道，尝试下一个分组
				logger.LogDebug(param.Ctx, "No available channel in group %s for model %s at priorityRetry %d, trying next group", autoGroup, param.ModelName, priorityRetry)
				// 重置状态以尝试下一个分组
				common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, i+1)
				common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupRetryIndex, 0)
				// Reset retry counter so outer loop can continue for next group
				// 重置重试计数器，以便外层循环可以为下一个分组继续
				param.SetRetry(0)
				continue
			}
			common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroup, autoGroup)
			selectGroup = autoGroup
			logger.LogDebug(param.Ctx, "Auto selected group: %s", autoGroup)

			// Prepare state for next retry
			// 为下一次重试准备状态
			if crossGroupRetry && priorityRetry >= common.RetryTimes {
				// Current group has exhausted all retries, check if there's a next group
				// 当前分组已用完所有重试次数，检查是否还有下一个分组
				if i+1 < len(autoGroups) {
					// Prepare to switch to next group
					// This request still uses current group, but next retry will use next group
					// 准备切换到下一个分组
					// 本次请求仍使用当前分组，但下次重试将使用下一个分组
					logger.LogDebug(param.Ctx, "Current group %s retries exhausted (priorityRetry=%d >= RetryTimes=%d), preparing switch to next group for next retry", autoGroup, priorityRetry, common.RetryTimes)
					common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, i+1)
					// Reset retry counter so outer loop can continue for next group
					// 重置重试计数器，以便外层循环可以为下一个分组继续
					param.SetRetry(0)
					param.ResetRetryNextTry()
				} else {
					// No more groups, stay in current group and let retry counter continue
					// 没有更多分组，保持在当前分组，让重试计数器继续递增
					logger.LogDebug(param.Ctx, "Current group %s retries exhausted but no more groups available, will stop retrying", autoGroup)
					common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, i)
				}
			} else {
				// Stay in current group, save current state
				// 保持在当前分组，保存当前状态
				common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, i)
			}
			break
		}
	} else {
		if sleepPoolActive {
			channel, err = selectChannelSkippingSleeping(param, param.TokenGroup, param.GetRetry())
			if err != nil {
				return nil, param.TokenGroup, err
			}
		} else {
			channel, err = model.GetRandomSatisfiedChannel(param.TokenGroup, param.ModelName, param.GetRetry())
			if err != nil {
				return nil, param.TokenGroup, err
			}
		}
	}
	return channel, selectGroup, nil
}

// selectChannelSkippingSleeping 在指定分组内选择渠道，跳过处于过载休眠池中的渠道。
//
// 若该分组下该模型的所有渠道都在休眠，则兜底选择「最后一个渠道」（优先级最低、同优先级内最后一个），
// 以保证不会出现「一个渠道都不请求」的情况。
//
// 性能说明：休眠条目是「一个 (模型, 渠道) 一个 key」，无法凭模型名枚举，
// 所以这里先用内存缓存取候选渠道，再对候选渠道做一次 MGET，固定 1 次 Redis 往返。
func selectChannelSkippingSleeping(param *RetryParam, group string, retry int) (*model.Channel, error) {
	candidates := model.GetSatisfiedChannelCandidates(group, param.ModelName)
	if len(candidates) == 0 {
		// 缓存/数据库都没有渠道时，回落到原有逻辑（保持错误语义一致）
		return model.GetRandomSatisfiedChannel(group, param.ModelName, retry)
	}

	// 只查「当前分组该模型的候选渠道」，避免 SCAN。
	candidateIds := make([]int, 0, len(candidates))
	for _, candidate := range candidates {
		candidateIds = append(candidateIds, candidate.ChannelId)
	}
	sleeping := GetOverloadSleepExcludedChannels(param.ModelName, candidateIds)

	excluded := make(map[int]struct{}, len(sleeping))
	for channelId := range sleeping {
		excluded[channelId] = struct{}{}
	}

	result, err := model.SelectChannelAvoidingExcluded(candidates, retry, excluded)
	if err != nil {
		return nil, err
	}
	if result == nil || result.Channel == nil {
		return nil, errors.New("no available channel after skipping overload sleeping channels")
	}

	if len(result.SkippedChannelIds) > 0 {
		skipped := make(map[int]*sleeppool.Entry, len(result.SkippedChannelIds))
		for _, channelId := range result.SkippedChannelIds {
			if entry, ok := sleeping[channelId]; ok {
				skipped[channelId] = entry
				continue
			}
			skipped[channelId] = &sleeppool.Entry{ChannelId: channelId, ModelName: param.ModelName}
		}
		RecordOverloadSleepSkip(param.Ctx, param.ModelName, group, skipped)
	}
	if result.AllSleeping {
		SetOverloadSleepAllSleeping(param.Ctx, param.ModelName)
		param.AllChannelsSleeping = true
	}
	return result.Channel, nil
}
