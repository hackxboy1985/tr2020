package sleeppool

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEntryRemainingSeconds(t *testing.T) {
	entry := &Entry{SleepUntil: 1000}
	require.Equal(t, int64(100), entry.RemainingSeconds(900))
	require.Equal(t, int64(0), entry.RemainingSeconds(1000))
	require.Equal(t, int64(0), entry.RemainingSeconds(1200))
}

func TestEntryIsExpired(t *testing.T) {
	entry := &Entry{SleepUntil: 1000}
	require.False(t, entry.IsExpired(999))
	require.True(t, entry.IsExpired(1000))
	require.True(t, entry.IsExpired(1001))
}

func TestModelKeyFormat(t *testing.T) {
	// key 形如 overload_sleep_pool:model:gpt-4o:channel:4
	require.Equal(t, "overload_sleep_pool:model:gpt-4o:channel:4", ModelKey("gpt-4o", 4))
	require.Equal(t, "overload_sleep_pool:model:gpt-4o:channel:", FormatKey("gpt-4o"))
}

func TestParseKeyRoundTrip(t *testing.T) {
	cases := []struct {
		modelName string
		channelId int
	}{
		{"gpt-4o", 4},
		{"gpt-4o-mini", 123},
		{"claude-3-5-sonnet-20240620", 7},
		{"model:with:colons", 9},
	}
	for _, tc := range cases {
		key := ModelKey(tc.modelName, tc.channelId)
		modelName, channelId, ok := parseKey(key)
		require.True(t, ok, "key=%s", key)
		require.Equal(t, tc.modelName, modelName, "key=%s", key)
		require.Equal(t, tc.channelId, channelId, "key=%s", key)
	}
}

func TestParseKeyRejectsInvalid(t *testing.T) {
	invalid := []string{
		"",
		"overload_sleep_pool:model:gpt-4o",
		"overload_sleep_pool:model::channel:4",
		"overload_sleep_pool:model:gpt-4o:channel:abc",
		"overload_sleep_pool:model:gpt-4o:channel:0",
		"other:key",
	}
	for _, key := range invalid {
		_, _, ok := parseKey(key)
		require.False(t, ok, "key=%s", key)
	}
}

func TestEscapeGlob(t *testing.T) {
	require.Equal(t, "gpt-4o", escapeGlob("gpt-4o"))
	require.Equal(t, "a\\*b", escapeGlob("a*b"))
	require.Equal(t, "a\\?b", escapeGlob("a?b"))
	require.Equal(t, "a\\[b\\]", escapeGlob("a[b]"))
}

func TestAvailableWithoutRedis(t *testing.T) {
	// 测试环境默认没有初始化 Redis 客户端
	require.False(t, Available())
	require.False(t, Enabled())
}

func TestMarkWithoutRedisReturnsError(t *testing.T) {
	err := Mark(Entry{ModelName: "gpt-4o", ChannelId: 1}, 2)
	require.ErrorIs(t, err, ErrRedisUnavailable)
}

func TestRemoveWithoutRedisReturnsError(t *testing.T) {
	_, err := Remove("gpt-4o", 1)
	require.ErrorIs(t, err, ErrRedisUnavailable)
}

func TestClearAllWithoutRedisReturnsError(t *testing.T) {
	_, err := ClearAll()
	require.ErrorIs(t, err, ErrRedisUnavailable)
}

func TestIsSleepingWithoutRedis(t *testing.T) {
	sleeping, entry := IsSleeping("gpt-4o", 1)
	require.False(t, sleeping)
	require.Nil(t, entry)
}

func TestListAllEntriesWithoutRedis(t *testing.T) {
	entries, err := ListAllEntries()
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestGetSleepingChannelsWithoutRedis(t *testing.T) {
	require.Empty(t, GetSleepingChannels("gpt-4o", []int{1, 2}))
}
