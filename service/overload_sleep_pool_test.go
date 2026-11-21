package service

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/sleeppool"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	goredis "github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

// setupOverloadSleepRedis 为休眠池相关测试准备内存 Redis 与开启的总开关。
func setupOverloadSleepRedis(t *testing.T) *miniredis.Miniredis {
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

func newTestContext(t *testing.T) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	return c
}

func testChannel(id int, name string) *model.Channel {
	return &model.Channel{Id: id, Name: name, Type: 1}
}

func TestRecordOverloadSleepSkip_DedupesWithinRequest(t *testing.T) {
	c := newTestContext(t)
	entries := map[int]*sleeppool.Entry{
		1: {ChannelId: 1, ChannelName: "ch-1", SleepUntil: common.GetTimestamp() + 60},
		2: {ChannelId: 2, ChannelName: "ch-2", SleepUntil: common.GetTimestamp() + 60},
	}

	RecordOverloadSleepSkip(c, "gpt-4o", "default", entries)
	RecordOverloadSleepSkip(c, "gpt-4o", "default", entries)

	skipped := model.GetOverloadSleepSkippedChannels(c)
	require.Len(t, skipped, 2)
	require.ElementsMatch(t, []int{1, 2}, []int{skipped[0].ChannelId, skipped[1].ChannelId})
	require.Greater(t, skipped[0].RemainingSeconds, int64(0))
}

func TestRecordOverloadSleepSkip_AddsNewChannelsAcrossRetries(t *testing.T) {
	c := newTestContext(t)

	RecordOverloadSleepSkip(c, "m", "g", map[int]*sleeppool.Entry{
		1: {ChannelId: 1, SleepUntil: common.GetTimestamp() + 30},
	})
	RecordOverloadSleepSkip(c, "m", "g", map[int]*sleeppool.Entry{
		1: {ChannelId: 1, SleepUntil: common.GetTimestamp() + 30},
		3: {ChannelId: 3, SleepUntil: common.GetTimestamp() + 30},
	})

	skipped := model.GetOverloadSleepSkippedChannels(c)
	require.Len(t, skipped, 2)
	require.ElementsMatch(t, []int{1, 3}, []int{skipped[0].ChannelId, skipped[1].ChannelId})
}

func TestOverloadSleepAllSleepingFlag(t *testing.T) {
	c := newTestContext(t)
	require.False(t, IsOverloadSleepAllSleeping(c))

	SetOverloadSleepAllSleeping(c, "m")
	require.True(t, IsOverloadSleepAllSleeping(c))
}

func TestGetOverloadSleepExcludedChannels_DisabledSwitch(t *testing.T) {
	setupOverloadSleepRedis(t)
	require.NoError(t, sleeppool.Mark(sleeppool.Entry{ModelName: "m", ChannelId: 1}, 2))

	operation_setting.GetOverloadSleepSetting().Enabled = false
	require.Empty(t, GetOverloadSleepExcludedChannels("m", []int{1, 2}))
}

func TestGetOverloadSleepExcludedChannels_ReturnsSleeping(t *testing.T) {
	setupOverloadSleepRedis(t)
	require.NoError(t, sleeppool.Mark(sleeppool.Entry{ModelName: "m", ChannelId: 1}, 2))
	require.NoError(t, sleeppool.Mark(sleeppool.Entry{ModelName: "m", ChannelId: 2}, 2))

	entries := GetOverloadSleepExcludedChannels("m", []int{1, 2})
	require.Len(t, entries, 2)
	require.Contains(t, entries, 1)
	require.Contains(t, entries, 2)

	require.Empty(t, GetOverloadSleepExcludedChannels("other", []int{1, 2}))
}

func TestMarkOverloadSleep_WritesPoolEntry(t *testing.T) {
	setupOverloadSleepRedis(t)
	c := newTestContext(t)

	require.True(t, MarkOverloadSleep(c, "gpt-4o", testChannel(7, "ch-7"), "default", 429))

	sleeping, entry := sleeppool.IsSleeping("gpt-4o", 7)
	require.True(t, sleeping)
	require.Equal(t, "ch-7", entry.ChannelName)
	require.Equal(t, 429, entry.TriggerStatusCode)
}

func TestMarkOverloadSleep_SkipsWhenDisabled(t *testing.T) {
	setupOverloadSleepRedis(t)
	c := newTestContext(t)
	operation_setting.GetOverloadSleepSetting().Enabled = false

	require.False(t, MarkOverloadSleep(c, "gpt-4o", testChannel(8, "ch-8"), "default", 429))

	sleeping, _ := sleeppool.IsSleeping("gpt-4o", 8)
	require.False(t, sleeping)
}

func TestMarkOverloadSleep_IgnoresNilChannel(t *testing.T) {
	setupOverloadSleepRedis(t)
	c := newTestContext(t)
	require.False(t, MarkOverloadSleep(c, "gpt-4o", nil, "default", 429))
}

func TestIsChannelOverloadSleeping(t *testing.T) {
	setupOverloadSleepRedis(t)
	c := newTestContext(t)

	require.NoError(t, sleeppool.Mark(sleeppool.Entry{ModelName: "m", ChannelId: 5, ChannelName: "ch-5"}, 2))

	require.True(t, IsChannelOverloadSleeping(c, "m", "default", testChannel(5, "ch-5")))
	require.Len(t, model.GetOverloadSleepSkippedChannels(c), 1)

	require.False(t, IsChannelOverloadSleeping(c, "m", "default", testChannel(6, "ch-6")))
	require.False(t, IsChannelOverloadSleeping(c, "m", "default", nil))
}

func TestRecordOverloadSleepSkip_ClampsRemainingSeconds(t *testing.T) {
	c := newTestContext(t)
	past := common.GetTimestamp() - 100
	RecordOverloadSleepSkip(c, "m", "g", map[int]*sleeppool.Entry{
		1: {ChannelId: 1, SleepUntil: past},
	})
	skipped := model.GetOverloadSleepSkippedChannels(c)
	require.Len(t, skipped, 1)
	require.Equal(t, int64(0), skipped[0].RemainingSeconds)
}
