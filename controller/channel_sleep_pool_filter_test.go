package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/pkg/sleeppool"
	"github.com/stretchr/testify/require"
)

// TestGetChannelSleepPool_FilterByModel 验证界面「按模型搜索列出所有休眠渠道」：
// key 里同时包含模型名与渠道 ID，按模型前缀即可精确列出该模型下的全部休眠渠道。
func TestGetChannelSleepPool_FilterByModel(t *testing.T) {
	setupSleepPoolControllerTest(t)

	require.NoError(t, sleeppool.Mark(sleeppool.Entry{ModelName: "gpt-4o", ChannelId: 4, ChannelName: "ch4"}, 2))
	require.NoError(t, sleeppool.Mark(sleeppool.Entry{ModelName: "gpt-4o", ChannelId: 5, ChannelName: "ch5"}, 2))
	require.NoError(t, sleeppool.Mark(sleeppool.Entry{ModelName: "claude-3", ChannelId: 6, ChannelName: "ch6"}, 2))

	// 按模型过滤
	c, recorder := newSleepPoolContext(t)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/channel_sleep_pool/?model_name=gpt-4o", nil)
	GetChannelSleepPool(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	payload := decodeSleepPoolResponse(t, recorder)
	require.Equal(t, true, payload["success"])
	entries := payload["data"].(map[string]any)["entries"].([]any)
	require.Len(t, entries, 2)
	for _, raw := range entries {
		entry := raw.(map[string]any)
		require.Equal(t, "gpt-4o", entry["model_name"])
	}

	// 不传过滤条件时返回全部
	c, recorder = newSleepPoolContext(t)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/channel_sleep_pool/", nil)
	GetChannelSleepPool(c)
	entries = decodeSleepPoolResponse(t, recorder)["data"].(map[string]any)["entries"].([]any)
	require.Len(t, entries, 3)
}

// TestGetChannelSleepPool_ModelPrefixNotMistaken 验证模型名前缀相同不会误命中
// （例如 gpt-4o 不应匹配到 gpt-4o-mini 的条目）。
func TestGetChannelSleepPool_ModelPrefixNotMistaken(t *testing.T) {
	setupSleepPoolControllerTest(t)

	require.NoError(t, sleeppool.Mark(sleeppool.Entry{ModelName: "gpt-4o", ChannelId: 4}, 2))
	require.NoError(t, sleeppool.Mark(sleeppool.Entry{ModelName: "gpt-4o-mini", ChannelId: 5}, 2))

	c, recorder := newSleepPoolContext(t)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/channel_sleep_pool/?model_name=gpt-4o", nil)
	GetChannelSleepPool(c)

	entries := decodeSleepPoolResponse(t, recorder)["data"].(map[string]any)["entries"].([]any)
	require.Len(t, entries, 1, "不应把 gpt-4o-mini 的条目算进 gpt-4o")
	require.Equal(t, float64(4), entries[0].(map[string]any)["channel_id"])
}
