package sleeppool

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

// TestEachChannelHasIndependentTTL 验证核心需求：
// 每个 (模型, 渠道) 是独立 key，Redis 原生保证 TTL 相互独立，
// 因此「后进入的渠道」不会影响「先进入的渠道」的到期时间。
func TestEachChannelHasIndependentTTL(t *testing.T) {
	server := setupMiniRedis(t)
	ctx := t.Context()

	cfg := operation_setting.GetOverloadSleepSetting()

	// 渠道 1：休眠 10 分钟
	cfg.SleepMinutes = 10
	operation_setting.NormalizeOverloadSleepSetting()
	require.NoError(t, Mark(Entry{ModelName: "m", ChannelId: 1}, 0))

	key1 := ModelKey("m", 1)
	ttl1, err := common.RDB.TTL(ctx, key1).Result()
	require.NoError(t, err)
	require.Greater(t, ttl1, 9*time.Minute, "渠道 1 的 TTL 应约为 10 分钟+宽限")

	// 渠道 2：休眠 10 分钟
	require.NoError(t, Mark(Entry{ModelName: "m", ChannelId: 2}, 0))
	key2 := ModelKey("m", 2)
	ttl2, err := common.RDB.TTL(ctx, key2).Result()
	require.NoError(t, err)
	require.Greater(t, ttl2, 9*time.Minute)

	// 两个 key 相互独立
	require.NotEqual(t, key1, key2)

	// 把配置调小到 1 分钟后，渠道 3 进入休眠：
	cfg.SleepMinutes = 1
	operation_setting.NormalizeOverloadSleepSetting()
	require.NoError(t, Mark(Entry{ModelName: "m", ChannelId: 3}, 0))

	// 渠道 1、2 的 TTL 不应因为渠道 3 的写入而缩短
	ttl1After, err := common.RDB.TTL(ctx, key1).Result()
	require.NoError(t, err)
	require.Greater(t, ttl1After, 9*time.Minute, "渠道 1 的 TTL 被渠道 3 影响了")

	key3 := ModelKey("m", 3)
	ttl3, err := common.RDB.TTL(ctx, key3).Result()
	require.NoError(t, err)
	// 渠道 3 按「1 分钟 + keyTTLGrace」写入，应明显短于渠道 1 的 10 分钟
	require.Less(t, ttl3, ttl1After, "渠道 3 的 TTL 应短于渠道 1（各自按写入时的配置独立计算）")
	require.LessOrEqual(t, ttl3, time.Minute+keyTTLGrace+5*time.Second, "渠道 3 应按新的 1 分钟配置休眠")

	// 时间推进：渠道 3 先过期，渠道 1/2 仍在休眠
	server.FastForward(5 * time.Minute)
	base := common.GetTimestamp()
	offset := int64(5 * 60)
	previousNow := nowFunc
	nowFunc = func() int64 { return base + offset }
	t.Cleanup(func() { nowFunc = previousNow })

	sleeping3, _ := IsSleeping("m", 3)
	require.False(t, sleeping3, "渠道 3（1 分钟）应已过期")
	sleeping1, _ := IsSleeping("m", 1)
	require.True(t, sleeping1, "渠道 1（10 分钟）应仍在休眠")
}

// TestChannelKeysAreIsolatedAcrossModels 验证同一渠道 ID 在不同模型下互不影响。
func TestChannelKeysAreIsolatedAcrossModels(t *testing.T) {
	setupMiniRedis(t)

	require.NoError(t, Mark(Entry{ModelName: "model-a", ChannelId: 4, ChannelName: "ch4"}, 2))

	sleeping, _ := IsSleeping("model-a", 4)
	require.True(t, sleeping)
	sleeping, _ = IsSleeping("model-b", 4)
	require.False(t, sleeping, "不同模型下相同渠道 ID 必须互不影响")

	// 移除 model-a 的渠道 4，不影响 model-b（即使 model-b 也有）
	require.NoError(t, Mark(Entry{ModelName: "model-b", ChannelId: 4}, 2))
	removed, err := Remove("model-a", 4)
	require.NoError(t, err)
	require.Equal(t, 1, removed)

	sleeping, _ = IsSleeping("model-a", 4)
	require.False(t, sleeping)
	sleeping, _ = IsSleeping("model-b", 4)
	require.True(t, sleeping)
}

// TestKeyContainsModelAndChannel 验证 key 格式便于按模型检索
// （界面「按模型搜索列出所有休眠渠道」依赖这个前缀）。
func TestKeyContainsModelAndChannel(t *testing.T) {
	setupMiniRedis(t)

	require.NoError(t, Mark(Entry{ModelName: "gpt-4o", ChannelId: 4}, 2))
	require.NoError(t, Mark(Entry{ModelName: "gpt-4o", ChannelId: 5}, 2))
	require.NoError(t, Mark(Entry{ModelName: "gpt-4o-mini", ChannelId: 6}, 2))

	ctx := t.Context()

	// 按模型检索：只应命中原模型的 key
	keys, err := common.RDB.Keys(ctx, modelKeyPattern("gpt-4o")).Result()
	require.NoError(t, err)
	require.Len(t, keys, 2)

	// 按模型列出全部休眠渠道
	entries, err := ListModelEntries("gpt-4o")
	require.NoError(t, err)
	require.Len(t, entries, 2)
	require.Contains(t, entries, 4)
	require.Contains(t, entries, 5)

	// 另一个模型独立
	entries, err = ListModelEntries("gpt-4o-mini")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Contains(t, entries, 6)
}
