package sleeppool

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

// TestKeyTTLNotShorterThanSleepUntil 这是「单个渠道刚进入就因 key 到期被恢复」问题的直接回归测试。
//
// 安全性要求：key 的物理过期时间必须 >= 条目自身的 sleep_until，
// 否则会出现「条目还没睡够，key 就没了」→ 渠道被提前恢复。
func TestKeyTTLNotShorterThanSleepUntil(t *testing.T) {
	setupMiniRedis(t)
	ctx := t.Context()

	cfg := operation_setting.GetOverloadSleepSetting()
	for _, minutes := range []int{1, 2, 5, 60, 1440} {
		cfg.SleepMinutes = minutes
		operation_setting.NormalizeOverloadSleepSetting()

		channelId := minutes // 用分钟数当渠道 ID，避免 id 冲突
		require.NoError(t, Mark(Entry{ModelName: "m", ChannelId: channelId}, 0))

		sleeping, entry := IsSleeping("m", channelId)
		require.True(t, sleeping)
		require.NotNil(t, entry)

		ttl, err := common.RDB.TTL(ctx, ModelKey("m", channelId)).Result()
		require.NoError(t, err)

		// key 的剩余 TTL 必须不短于条目剩余休眠时间
		remainingSleep := time.Duration(entry.SleepUntil-common.GetTimestamp()) * time.Second
		require.GreaterOrEqual(t, ttl, remainingSleep,
			"minutes=%d：key TTL(%v) 短于 sleep_until 剩余(%v)，会导致提前恢复", minutes, ttl, remainingSleep)
	}
}

// TestRenewalDoesNotShortenNewlyEnteredChannel 验证调小休眠时长并续期时，
// 不会把已经存在的「长休眠」渠道缩短。
func TestRenewalDoesNotShortenNewlyEnteredChannel(t *testing.T) {
	setupMiniRedis(t)
	ctx := t.Context()

	cfg := operation_setting.GetOverloadSleepSetting()

	// 渠道 1 以 10 分钟进入
	cfg.SleepMinutes = 10
	operation_setting.NormalizeOverloadSleepSetting()
	require.NoError(t, Mark(Entry{ModelName: "m", ChannelId: 1}, 0))
	_, entry1 := IsSleeping("m", 1)
	require.NotNil(t, entry1)
	longUntil := entry1.SleepUntil

	ttlBefore, err := common.RDB.TTL(ctx, ModelKey("m", 1)).Result()
	require.NoError(t, err)

	// 配置调小到 1 分钟，另一个渠道进入
	cfg.SleepMinutes = 1
	operation_setting.NormalizeOverloadSleepSetting()
	require.NoError(t, Mark(Entry{ModelName: "m", ChannelId: 2}, 0))

	// 渠道 1 的 sleep_until 与 TTL 都不应被影响
	_, entry1After := IsSleeping("m", 1)
	require.NotNil(t, entry1After)
	require.Equal(t, longUntil, entry1After.SleepUntil, "渠道 1 的 sleep_until 不应被改变")

	ttlAfter, err := common.RDB.TTL(ctx, ModelKey("m", 1)).Result()
	require.NoError(t, err)
	require.InDelta(t, float64(ttlBefore), float64(ttlAfter), 2, "渠道 1 的 key TTL 不应被其它渠道的写入改变")
}

// TestSubMinutePrecisionNotLost 确保「分钟换算成秒」没有丢精度导致提前恢复。
func TestSubMinutePrecisionNotLost(t *testing.T) {
	setupMiniRedis(t)

	cfg := operation_setting.GetOverloadSleepSetting()
	cfg.SleepMinutes = 2
	operation_setting.NormalizeOverloadSleepSetting()

	before := common.GetTimestamp()
	require.NoError(t, Mark(Entry{ModelName: "m", ChannelId: 1}, 0))
	_, entry := IsSleeping("m", 1)
	require.NotNil(t, entry)

	// 至少睡满 120 秒
	require.GreaterOrEqual(t, entry.SleepUntil, before+120)
	// 但不应明显超出（避免 TTL 逻辑写错成分钟当秒用等）
	require.LessOrEqual(t, entry.SleepUntil, common.GetTimestamp()+120)
}
