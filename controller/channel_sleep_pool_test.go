package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/sleeppool"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	goredis "github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

func setupSleepPoolControllerTest(t *testing.T) *miniredis.Miniredis {
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

func newSleepPoolContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	return c, recorder
}

func decodeSleepPoolResponse(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var payload map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	return payload
}

func TestGetChannelSleepPool_Empty(t *testing.T) {
	setupSleepPoolControllerTest(t)

	c, recorder := newSleepPoolContext(t)
	GetChannelSleepPool(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	payload := decodeSleepPoolResponse(t, recorder)
	require.Equal(t, true, payload["success"])
	data := payload["data"].(map[string]any)
	require.Equal(t, true, data["enabled"])
	require.Equal(t, true, data["redis_enabled"])
	require.Equal(t, float64(2), data["sleep_minutes"])
	require.Equal(t, "429,503", data["status_codes"])
	require.Greater(t, data["server_time"].(float64), float64(0))
	require.Empty(t, data["entries"])
}

func TestGetChannelSleepPool_ListsEntries(t *testing.T) {
	setupSleepPoolControllerTest(t)

	require.NoError(t, sleeppool.Mark(sleeppool.Entry{
		ModelName:         "gpt-4o",
		ChannelId:         12,
		ChannelName:       "openai-1",
		ChannelType:       1,
		Group:             "default",
		TriggerStatusCode: 429,
	}, 2))

	c, recorder := newSleepPoolContext(t)
	GetChannelSleepPool(c)

	payload := decodeSleepPoolResponse(t, recorder)
	require.Equal(t, true, payload["success"])
	data := payload["data"].(map[string]any)
	entries := data["entries"].([]any)
	require.Len(t, entries, 1)

	entry := entries[0].(map[string]any)
	require.Equal(t, "gpt-4o", entry["model_name"])
	require.Equal(t, float64(12), entry["channel_id"])
	require.Equal(t, "openai-1", entry["channel_name"])
	require.Equal(t, float64(429), entry["trigger_status_code"])
	require.Equal(t, float64(1), entry["trigger_count"])
	require.Greater(t, entry["remaining_seconds"].(float64), float64(0))
	// 未初始化渠道缓存时拿不到可用渠道，不应宣称「全部休眠」
	require.Equal(t, false, entry["all_channels_sleeping"])
	require.Nil(t, entry["fallback_channel"])
}

func TestDeleteChannelSleepPoolEntry_RemovesOne(t *testing.T) {
	setupSleepPoolControllerTest(t)

	require.NoError(t, sleeppool.Mark(sleeppool.Entry{ModelName: "m", ChannelId: 1}, 2))
	require.NoError(t, sleeppool.Mark(sleeppool.Entry{ModelName: "m", ChannelId: 2}, 2))

	c, recorder := newSleepPoolContext(t)
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/channel_sleep_pool/?model_name=m&channel_id=1", nil)
	DeleteChannelSleepPoolEntry(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	payload := decodeSleepPoolResponse(t, recorder)
	require.Equal(t, true, payload["success"])
	require.Equal(t, float64(1), payload["data"].(map[string]any)["removed"])

	sleeping, _ := sleeppool.IsSleeping("m", 1)
	require.False(t, sleeping)
	sleeping, _ = sleeppool.IsSleeping("m", 2)
	require.True(t, sleeping)
}

func TestDeleteChannelSleepPoolEntry_ClearsAll(t *testing.T) {
	setupSleepPoolControllerTest(t)

	require.NoError(t, sleeppool.Mark(sleeppool.Entry{ModelName: "m1", ChannelId: 1}, 2))
	require.NoError(t, sleeppool.Mark(sleeppool.Entry{ModelName: "m2", ChannelId: 2}, 2))

	c, recorder := newSleepPoolContext(t)
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/channel_sleep_pool/?all=true", nil)
	DeleteChannelSleepPoolEntry(c)

	payload := decodeSleepPoolResponse(t, recorder)
	require.Equal(t, true, payload["success"])
	require.Equal(t, float64(2), payload["data"].(map[string]any)["removed"])

	entries, err := sleeppool.ListAllEntries()
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestDeleteChannelSleepPoolEntry_InvalidParams(t *testing.T) {
	setupSleepPoolControllerTest(t)

	cases := []string{
		"/api/channel_sleep_pool/",
		"/api/channel_sleep_pool/?model_name=m",
		"/api/channel_sleep_pool/?channel_id=1",
		"/api/channel_sleep_pool/?model_name=m&channel_id=abc",
		"/api/channel_sleep_pool/?model_name=m&channel_id=0",
	}
	for _, target := range cases {
		c, recorder := newSleepPoolContext(t)
		c.Request = httptest.NewRequest(http.MethodDelete, target, nil)
		DeleteChannelSleepPoolEntry(c)
		require.Equal(t, http.StatusBadRequest, recorder.Code, "target=%s", target)
		payload := decodeSleepPoolResponse(t, recorder)
		require.Equal(t, false, payload["success"])
	}
}
