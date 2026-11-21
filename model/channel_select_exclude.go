package model

import (
	"errors"
	"math/rand"
	"sort"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

// ChannelCandidate 渠道选择候选：渠道 ID + 优先级 + 权重
type ChannelCandidate struct {
	ChannelId int
	Priority  int64
	Weight    int
}

// ChannelSelectionPlan 纯选择结果（不含渠道详情，便于单元测试）
type ChannelSelectionPlan struct {
	// ChannelId 选中的渠道 ID
	ChannelId int
	// SkippedChannelIds 因休眠被跳过的渠道 ID（按优先级从高到低）
	SkippedChannelIds []int
	// AllSleeping 该模型所有可用渠道都处于休眠，此时兜底返回「最后一个渠道」
	AllSleeping bool
	// TotalChannels 该模型可用渠道总数
	TotalChannels int
}

// ChannelSelectionResult 渠道选择结果
type ChannelSelectionResult struct {
	// Channel 选中的渠道
	Channel *Channel
	// SkippedChannelIds 因休眠被跳过的渠道（按优先级从高到低）
	SkippedChannelIds []int
	// AllSleeping 该模型所有可用渠道都处于休眠，此时返回「最后一个渠道」
	AllSleeping bool
	// TotalChannels 该模型可用渠道总数
	TotalChannels int
}

// errNoChannelCandidate 没有任何可用渠道
var errNoChannelCandidate = errors.New("no available channel")

// GetSatisfiedChannelCandidates 返回指定 (分组, 模型) 的全部可用渠道候选，
// 顺序为优先级降序（同优先级内维持缓存/数据库返回顺序，因此末位即「最后一个渠道」）。
func GetSatisfiedChannelCandidates(group string, modelName string) []ChannelCandidate {
	if common.MemoryCacheEnabled {
		return getCandidatesFromCache(group, modelName)
	}
	return getCandidatesFromDB(group, modelName)
}

func getCandidatesFromCache(group string, modelName string) []ChannelCandidate {
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
	candidates := make([]ChannelCandidate, 0, len(ids))
	for _, id := range ids {
		channel, ok := channelsIDM[id]
		if !ok {
			continue
		}
		candidates = append(candidates, ChannelCandidate{
			ChannelId: channel.Id,
			Priority:  channel.GetPriority(),
			Weight:    channel.GetWeight(),
		})
	}
	return candidates
}

func getCandidatesFromDB(group string, modelName string) []ChannelCandidate {
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
	candidates := make([]ChannelCandidate, 0, len(abilities))
	for _, ability := range abilities {
		if ability.ChannelId <= 0 {
			continue
		}
		priority := int64(0)
		if ability.Priority != nil {
			priority = *ability.Priority
		}
		candidates = append(candidates, ChannelCandidate{
			ChannelId: ability.ChannelId,
			Priority:  priority,
			Weight:    int(ability.Weight),
		})
	}
	return candidates
}

// PlanChannelAvoidingExcluded 纯选择逻辑：按「优先级 + 权重」选择渠道，但跳过 exclude 中的渠道。
//
// 语义与 GetRandomSatisfiedChannel 保持一致：
//   - retry 依次映射到「优先级从高到低」的第 retry 档；
//   - 在同一优先级内按权重随机；
//
// 在此基础上：
//   - 当前优先级内所有渠道都被 exclude 时，继续向更低优先级寻找；
//   - 如果所有渠道都被 exclude（即全部休眠），返回该模型「最后一个渠道」（候选列表末位），
//     保证不会出现「一个渠道都不请求」的情况。
func PlanChannelAvoidingExcluded(candidates []ChannelCandidate, retry int, exclude map[int]struct{}) *ChannelSelectionPlan {
	if len(candidates) == 0 {
		return nil
	}
	if retry < 0 {
		retry = 0
	}
	priorities := uniquePrioritiesDesc(candidates)
	if retry >= len(priorities) {
		retry = len(priorities) - 1
	}

	plan := &ChannelSelectionPlan{TotalChannels: len(candidates)}
	if len(exclude) == 0 {
		targetPriority := priorities[retry]
		levelCandidates := filterByPriority(candidates, targetPriority)
		plan.ChannelId = pickWeightedChannelId(levelCandidates)
		return plan
	}

	for level := retry; level < len(priorities); level++ {
		priority := priorities[level]
		levelCandidates := make([]ChannelCandidate, 0, len(candidates))
		for _, candidate := range candidates {
			if candidate.Priority != priority {
				continue
			}
			if _, excluded := exclude[candidate.ChannelId]; excluded {
				plan.SkippedChannelIds = append(plan.SkippedChannelIds, candidate.ChannelId)
				continue
			}
			levelCandidates = append(levelCandidates, candidate)
		}
		if len(levelCandidates) == 0 {
			continue
		}
		plan.ChannelId = pickWeightedChannelId(levelCandidates)
		return plan
	}

	// 走到这里说明：从当前重试档位到最低优先级，所有渠道都在排除列表中。
	// 为了兑现「不能一个都不请求」，退回该模型全部候选中的「最后一个渠道」。
	// 此时保留所有被跳过的渠道（除兜底渠道外），便于日志完整记录。
	plan.AllSleeping = true
	fallback := candidates[len(candidates)-1]
	plan.ChannelId = fallback.ChannelId
	plan.SkippedChannelIds = nil
	for _, candidate := range candidates {
		if candidate.ChannelId == fallback.ChannelId {
			continue
		}
		if _, excluded := exclude[candidate.ChannelId]; excluded {
			plan.SkippedChannelIds = append(plan.SkippedChannelIds, candidate.ChannelId)
		}
	}
	return plan
}

// SelectChannelAvoidingExcluded 在 PlanChannelAvoidingExcluded 基础上加载渠道详情
func SelectChannelAvoidingExcluded(candidates []ChannelCandidate, retry int, exclude map[int]struct{}) (*ChannelSelectionResult, error) {
	plan := PlanChannelAvoidingExcluded(candidates, retry, exclude)
	if plan == nil {
		return nil, errNoChannelCandidate
	}
	channel, err := loadChannelForSelection(plan.ChannelId)
	if err != nil {
		return nil, err
	}
	return &ChannelSelectionResult{
		Channel:           channel,
		SkippedChannelIds: plan.SkippedChannelIds,
		AllSleeping:       plan.AllSleeping,
		TotalChannels:     plan.TotalChannels,
	}, nil
}

func uniquePrioritiesDesc(candidates []ChannelCandidate) []int64 {
	seen := make(map[int64]struct{}, len(candidates))
	priorities := make([]int64, 0, len(candidates))
	for _, candidate := range candidates {
		if _, ok := seen[candidate.Priority]; ok {
			continue
		}
		seen[candidate.Priority] = struct{}{}
		priorities = append(priorities, candidate.Priority)
	}
	sort.Slice(priorities, func(i, j int) bool { return priorities[i] > priorities[j] })
	return priorities
}

func filterByPriority(candidates []ChannelCandidate, priority int64) []ChannelCandidate {
	filtered := make([]ChannelCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.Priority != priority {
			continue
		}
		filtered = append(filtered, candidate)
	}
	return filtered
}

// pickWeightedChannelId 按权重挑选渠道（与 GetRandomSatisfiedChannel 的平滑算法一致）
func pickWeightedChannelId(candidates []ChannelCandidate) int {
	if len(candidates) == 0 {
		return 0
	}
	sumWeight := 0
	for _, candidate := range candidates {
		sumWeight += candidate.Weight
	}
	smoothingFactor := 1
	smoothingAdjustment := 0
	if sumWeight == 0 {
		sumWeight = len(candidates) * 100
		smoothingAdjustment = 100
	} else if sumWeight/len(candidates) < 10 {
		smoothingFactor = 100
	}
	totalWeight := sumWeight * smoothingFactor
	if totalWeight <= 0 {
		return candidates[0].ChannelId
	}
	randomWeight := rand.Intn(totalWeight)
	for _, candidate := range candidates {
		randomWeight -= candidate.Weight*smoothingFactor + smoothingAdjustment
		if randomWeight < 0 {
			return candidate.ChannelId
		}
	}
	return candidates[len(candidates)-1].ChannelId
}

// loadChannelForSelection 读取渠道详情（内存缓存优先）
func loadChannelForSelection(channelId int) (*Channel, error) {
	channel, err := CacheGetChannel(channelId)
	if err == nil && channel != nil {
		return channel, nil
	}
	return GetChannelById(channelId, true)
}
