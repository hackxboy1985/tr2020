package sleeppool

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

// TestExpiredKeyAutoRemovedByRedis 验证：改成「一个 (模型, 渠道) 一个 key」后，
// 过期回收由 Redis 原生 TTL 完成，无需惰性清理，也不会残留内存。
func TestExpiredKeyAutoRemovedByRedis(t *testing.T) {
	server := setupMiniRedis(t)
	ctx := t.Context()

	base := common.GetTimestamp()
	offset := int64(0)
	previousNow := nowFunc
	nowFunc = func() int64 { return base + offset }
	t.Cleanup(func() { nowFunc = previousNow })

	require.NoError(t, Mark(Entry{ModelName: "m", ChannelId: 1}, 1))
	require.NoError(t, Mark(Entry{ModelName: "m", ChannelId: 2}, 2))

	key1 := ModelKey("m", 1)
	key2 := ModelKey("m", 2)

	// 两个 key 都存在
	exists, err := common.RDB.Exists(ctx, key1, key2).Result()
	require.NoError(t, err)
	require.Equal(t, int64(2), exists)

	// 快进 90 秒 + 逻辑时钟同步推进：渠道 1（1 分钟）逻辑上已过期，
	// 虽因 keyTTLGrace 仍物理存在，但读取时必须被过滤掉。
	offset = 90
	server.FastForward(90 * time.Second)

	entries, err := ListModelEntries("m")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Contains(t, entries, 2)

	sleeping, _ := IsSleeping("m", 1)
	require.False(t, sleeping, "渠道 1 逻辑上已过期，不应再被跳过")

	// 超过「1 分钟 + 宽限」后，Redis 直接回收该 key（无需应用层惰性清理）
	offset = 90 + 240
	server.FastForward(240 * time.Second)
	exists, err = common.RDB.Exists(ctx, key1).Result()
	require.NoError(t, err)
	require.Equal(t, int64(0), exists, "渠道 1 的 key 应由 Redis TTL 自动回收")

	// 再推进足够长时间：渠道 2 也自动回收
	offset = 90 + 240 + 400
	server.FastForward(400 * time.Second)
	entries, err = ListModelEntries("m")
	require.NoError(t, err)
	require.Empty(t, entries)
}

// TestDirtyDataIgnoredSafely 脏数据（非法 JSON / 非法 key）不应导致 panic 或错误条目。
func TestDirtyDataIgnoredSafely(t *testing.T) {
	setupMiniRedis(t)
	ctx := t.Context()

	require.NoError(t, Mark(Entry{ModelName: "m", ChannelId: 1}, 2))

	// 手工写入非法 JSON
	require.NoError(t, common.RDB.Set(ctx, ModelKey("m", 2), "{invalid-json", time.Minute).Err())
	// 手工写入格式非法、无法解析出模型/渠道的 key
	require.NoError(t, common.RDB.Set(ctx, "overload_sleep_pool:model:m:channel:abc", "{}", time.Minute).Err())

	entries, err := ListAllEntries()
	require.NoError(t, err)
	require.Len(t, entries, 1, "脏数据应被安全忽略")
	require.Equal(t, 1, entries[0].ChannelId)

	// 查询合法渠道不受脏数据影响
	sleeping, entry := IsSleeping("m", 1)
	require.True(t, sleeping)
	require.NotNil(t, entry)
}
