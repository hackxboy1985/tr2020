package sleeppool

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/alicebob/miniredis/v2"
	goredis "github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

// setupMiniRedis 启动一个内存 Redis 并注入到 common.RDB，测试结束后清理。
// 这里直接构造 go-redis 客户端，避免为了测试改动生产代码。
func setupMiniRedis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	server, err := miniredis.Run()
	require.NoError(t, err)

	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	previousClient := common.RDB
	previousEnabled := common.RedisEnabled
	common.RDB = client
	common.RedisEnabled = true
	t.Cleanup(func() {
		_ = client.Close()
		server.Close()
		common.RDB = previousClient
		common.RedisEnabled = previousEnabled
	})

	// 打开总开关，确保 Enabled() 为 true
	cfg := operation_setting.GetOverloadSleepSetting()
	saved := *cfg
	t.Cleanup(func() {
		*cfg = saved
		operation_setting.NormalizeOverloadSleepSetting()
	})
	cfg.Enabled = true
	cfg.SleepMinutes = 2
	cfg.StatusCodes = "429,503"
	operation_setting.NormalizeOverloadSleepSetting()

	return server
}

func TestMarkAndIsSleeping(t *testing.T) {
	setupMiniRedis(t)

	now := common.GetTimestamp()
	require.NoError(t, Mark(Entry{
		ModelName:         "gpt-4o",
		ChannelId:         12,
		ChannelName:       "openai-1",
		ChannelType:       1,
		Group:             "default",
		TriggerStatusCode: 429,
	}, 2))

	sleeping, entry := IsSleeping("gpt-4o", 12)
	require.True(t, sleeping)
	require.NotNil(t, entry)
	require.Equal(t, "openai-1", entry.ChannelName)
	require.Equal(t, 1, entry.TriggerCount)
	require.GreaterOrEqual(t, entry.SleepUntil, now+60)

	// 不在池中的渠道
	sleeping, entry = IsSleeping("gpt-4o", 13)
	require.False(t, sleeping)
	require.Nil(t, entry)

	// 不同模型同名渠道互不影响
	sleeping, _ = IsSleeping("gpt-4o-mini", 12)
	require.False(t, sleeping)
}

func TestMarkAccumulatesTriggerCount(t *testing.T) {
	setupMiniRedis(t)

	for i := 0; i < 3; i++ {
		require.NoError(t, Mark(Entry{ModelName: "gpt-4o", ChannelId: 12, TriggerStatusCode: 503}, 2))
	}
	sleeping, entry := IsSleeping("gpt-4o", 12)
	require.True(t, sleeping)
	require.Equal(t, 3, entry.TriggerCount)
}

func TestMarkRenewsSleepUntil(t *testing.T) {
	setupMiniRedis(t)

	require.NoError(t, Mark(Entry{ModelName: "m", ChannelId: 1}, 2))
	_, first := IsSleeping("m", 1)
	require.NotNil(t, first)

	require.NoError(t, Mark(Entry{ModelName: "m", ChannelId: 1}, 2))
	_, second := IsSleeping("m", 1)
	require.NotNil(t, second)
	require.GreaterOrEqual(t, second.SleepUntil, first.SleepUntil)
}

func TestEntryExpiresIndividually(t *testing.T) {
	server := setupMiniRedis(t)

	base := common.GetTimestamp()
	offset := int64(0)
	previousNow := nowFunc
	nowFunc = func() int64 { return base + offset }
	t.Cleanup(func() { nowFunc = previousNow })

	require.NoError(t, Mark(Entry{ModelName: "m", ChannelId: 1}, 1))
	require.NoError(t, Mark(Entry{ModelName: "m", ChannelId: 2}, 2))

	// 时间推进 90 秒：渠道 1（1 分钟）过期，渠道 2（2 分钟）仍休眠
	offset = 90
	server.FastForward(90 * time.Second)

	sleeping, _ := IsSleeping("m", 1)
	require.False(t, sleeping)
	sleeping, _ = IsSleeping("m", 2)
	require.True(t, sleeping)

	entries, err := ListAllEntries()
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, 2, entries[0].ChannelId)
}

func TestRemoveAndClearAll(t *testing.T) {
	setupMiniRedis(t)

	require.NoError(t, Mark(Entry{ModelName: "m1", ChannelId: 1}, 2))
	require.NoError(t, Mark(Entry{ModelName: "m1", ChannelId: 2}, 2))
	require.NoError(t, Mark(Entry{ModelName: "m2", ChannelId: 3}, 2))

	removed, err := Remove("m1", 1)
	require.NoError(t, err)
	require.Equal(t, 1, removed)

	sleeping, _ := IsSleeping("m1", 1)
	require.False(t, sleeping)
	sleeping, _ = IsSleeping("m1", 2)
	require.True(t, sleeping)

	removed, err = Remove("m1", 1)
	require.NoError(t, err)
	require.Equal(t, 0, removed)

	all, err := ListAllEntries()
	require.NoError(t, err)
	require.Len(t, all, 2)

	total, err := ClearAll()
	require.NoError(t, err)
	require.Equal(t, 2, total)

	all, err = ListAllEntries()
	require.NoError(t, err)
	require.Empty(t, all)
}

func TestListAllEntriesSorted(t *testing.T) {
	setupMiniRedis(t)

	require.NoError(t, Mark(Entry{ModelName: "zeta", ChannelId: 5}, 2))
	require.NoError(t, Mark(Entry{ModelName: "alpha", ChannelId: 9}, 2))
	require.NoError(t, Mark(Entry{ModelName: "alpha", ChannelId: 3}, 2))

	entries, err := ListAllEntries()
	require.NoError(t, err)
	require.Len(t, entries, 3)
	require.Equal(t, "alpha", entries[0].ModelName)
	require.Equal(t, 3, entries[0].ChannelId)
	require.Equal(t, "alpha", entries[1].ModelName)
	require.Equal(t, 9, entries[1].ChannelId)
	require.Equal(t, "zeta", entries[2].ModelName)
}

func TestMarkRejectsInvalidEntry(t *testing.T) {
	setupMiniRedis(t)

	require.Error(t, Mark(Entry{ModelName: "", ChannelId: 1}, 2))
	require.Error(t, Mark(Entry{ModelName: "m", ChannelId: 0}, 2))
	require.Error(t, Mark(Entry{ModelName: "m", ChannelId: -1}, 2))
}

func TestMarkUsesConfiguredMinutesWhenZero(t *testing.T) {
	setupMiniRedis(t)

	cfg := operation_setting.GetOverloadSleepSetting()
	cfg.SleepMinutes = 5
	operation_setting.NormalizeOverloadSleepSetting()

	before := common.GetTimestamp()
	require.NoError(t, Mark(Entry{ModelName: "m", ChannelId: 1}, 0))
	_, entry := IsSleeping("m", 1)
	require.NotNil(t, entry)
	require.GreaterOrEqual(t, entry.SleepUntil, before+5*60)
	require.LessOrEqual(t, entry.SleepUntil, common.GetTimestamp()+5*60)
}

func TestEnabledRequiresSwitch(t *testing.T) {
	setupMiniRedis(t)
	require.True(t, Enabled())

	operation_setting.GetOverloadSleepSetting().Enabled = false
	require.False(t, Enabled())
	require.False(t, operation_setting.IsOverloadStatusCode(429))
}

func TestSleepingChannelIDs(t *testing.T) {
	setupMiniRedis(t)

	require.NoError(t, Mark(Entry{ModelName: "m", ChannelId: 1}, 2))
	require.NoError(t, Mark(Entry{ModelName: "m", ChannelId: 2}, 2))
	require.NoError(t, Mark(Entry{ModelName: "other", ChannelId: 3}, 2))

	ids := SleepingChannelIDs("m")
	require.Len(t, ids, 2)
	require.Contains(t, ids, 1)
	require.Contains(t, ids, 2)
	require.NotContains(t, ids, 3)
}
