package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/sleeppool"
	"github.com/stretchr/testify/require"
)

// TestRemoveFromPoolRestoresChannel 验证「前端把某 (模型, 渠道) 从休眠池移除后，
// 该渠道立即回到正常请求流程中」——即渠道选择不再把它当作休眠跳过。
func TestRemoveFromPoolRestoresChannel(t *testing.T) {
	setupOverloadSleepRedis(t)

	require.NoError(t, sleeppool.Mark(sleeppool.Entry{ModelName: "gpt-4o", ChannelId: 42}, 2))
	require.Len(t, GetOverloadSleepExcludedChannels("gpt-4o", []int{42}), 1)
	sleepingBefore, _ := sleeppool.IsSleeping("gpt-4o", 42)
	require.True(t, sleepingBefore)

	// 模拟前端「从休眠池移除」
	removed, err := sleeppool.Remove("gpt-4o", 42)
	require.NoError(t, err)
	require.Equal(t, 1, removed)

	// 移除后不应再被排除，下次选择会照常选中它
	require.Empty(t, GetOverloadSleepExcludedChannels("gpt-4o", []int{42}))
	sleeping, _ := sleeppool.IsSleeping("gpt-4o", 42)
	require.False(t, sleeping)
}

// TestClearAllRestoresEveryChannel 验证「全部清除」后所有渠道恢复正常。
func TestClearAllRestoresEveryChannel(t *testing.T) {
	setupOverloadSleepRedis(t)

	require.NoError(t, sleeppool.Mark(sleeppool.Entry{ModelName: "m1", ChannelId: 1}, 2))
	require.NoError(t, sleeppool.Mark(sleeppool.Entry{ModelName: "m2", ChannelId: 2}, 2))

	removed, err := sleeppool.ClearAll()
	require.NoError(t, err)
	require.Equal(t, 2, removed)

	require.Empty(t, GetOverloadSleepExcludedChannels("m1", []int{1}))
	require.Empty(t, GetOverloadSleepExcludedChannels("m2", []int{2}))
	_ = common.GetTimestamp
}
