package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func cands() []ChannelCandidate {
	return []ChannelCandidate{
		{ChannelId: 1, Priority: 10, Weight: 0},
		{ChannelId: 2, Priority: 10, Weight: 0},
		{ChannelId: 3, Priority: 0, Weight: 0},
	}
}

func TestPlanChannelAvoidingExcluded_NoExclusionRespectsPriority(t *testing.T) {
	plan := PlanChannelAvoidingExcluded(cands(), 0, nil)
	require.NotNil(t, plan)
	require.False(t, plan.AllSleeping)
	require.Contains(t, []int{1, 2}, plan.ChannelId)
	require.Empty(t, plan.SkippedChannelIds)
	require.Equal(t, 3, plan.TotalChannels)

	// retry=1 对应第二档优先级（0）
	plan = PlanChannelAvoidingExcluded(cands(), 1, nil)
	require.Equal(t, 3, plan.ChannelId)
}

func TestPlanChannelAvoidingExcluded_SkipsToLowerPriority(t *testing.T) {
	// 最高优先级两个渠道都休眠 -> 应跳到低优先级渠道 3
	exclude := map[int]struct{}{1: {}, 2: {}}
	plan := PlanChannelAvoidingExcluded(cands(), 0, exclude)
	require.NotNil(t, plan)
	require.False(t, plan.AllSleeping)
	require.Equal(t, 3, plan.ChannelId)
	require.ElementsMatch(t, []int{1, 2}, plan.SkippedChannelIds)
}

func TestPlanChannelAvoidingExcluded_SkipsWithinSamePriority(t *testing.T) {
	exclude := map[int]struct{}{1: {}}
	plan := PlanChannelAvoidingExcluded(cands(), 0, exclude)
	require.False(t, plan.AllSleeping)
	require.Equal(t, 2, plan.ChannelId)
	require.Equal(t, []int{1}, plan.SkippedChannelIds)
}

func TestPlanChannelAvoidingExcluded_AllExcludedFallsBackToLastChannel(t *testing.T) {
	exclude := map[int]struct{}{1: {}, 2: {}, 3: {}}
	plan := PlanChannelAvoidingExcluded(cands(), 0, exclude)
	require.NotNil(t, plan)
	require.True(t, plan.AllSleeping)
	// 「最后一个渠道」= 候选列表末位
	require.Equal(t, 3, plan.ChannelId)
	// 其余休眠渠道仍要记录跳过日志
	require.ElementsMatch(t, []int{1, 2}, plan.SkippedChannelIds)
}

func TestPlanChannelAvoidingExcluded_AllExcludedFallbackIgnoresRetry(t *testing.T) {
	exclude := map[int]struct{}{1: {}, 2: {}, 3: {}}
	for retry := 0; retry <= 3; retry++ {
		plan := PlanChannelAvoidingExcluded(cands(), retry, exclude)
		require.True(t, plan.AllSleeping, "retry=%d", retry)
		require.Equal(t, 3, plan.ChannelId, "retry=%d", retry)
	}
}

func TestPlanChannelAvoidingExcluded_EmptyCandidates(t *testing.T) {
	require.Nil(t, PlanChannelAvoidingExcluded(nil, 0, map[int]struct{}{1: {}}))
}

func TestPlanChannelAvoidingExcluded_SingleChannelSleepingStillRequestsIt(t *testing.T) {
	candidates := []ChannelCandidate{{ChannelId: 9, Priority: 5, Weight: 0}}
	plan := PlanChannelAvoidingExcluded(candidates, 0, map[int]struct{}{9: {}})
	require.True(t, plan.AllSleeping)
	require.Equal(t, 9, plan.ChannelId)
	require.Empty(t, plan.SkippedChannelIds)
}

func TestPickWeightedChannelId_RespectsWeights(t *testing.T) {
	// 权重全部集中在渠道 2 上时必然选它
	candidates := []ChannelCandidate{
		{ChannelId: 1, Priority: 0, Weight: 0},
		{ChannelId: 2, Priority: 0, Weight: 1000},
	}
	for i := 0; i < 50; i++ {
		require.Equal(t, 2, pickWeightedChannelId(candidates))
	}
}

func TestPickWeightedChannelId_EmptyReturnsZero(t *testing.T) {
	require.Equal(t, 0, pickWeightedChannelId(nil))
}
