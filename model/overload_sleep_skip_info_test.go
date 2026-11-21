package model

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newOverloadSleepTestContext(t *testing.T) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	return c
}

func sampleSkipped(channelId int) OverloadSleepSkippedChannel {
	return OverloadSleepSkippedChannel{
		ModelName:        "gpt-4o",
		ChannelId:        channelId,
		ChannelName:      "ch",
		ChannelType:      1,
		Group:            "default",
		RemainingSeconds: 60,
	}
}

func TestAddOverloadSleepSkippedChannels_Dedupes(t *testing.T) {
	c := newOverloadSleepTestContext(t)

	AddOverloadSleepSkippedChannels(c, []OverloadSleepSkippedChannel{sampleSkipped(1)})
	AddOverloadSleepSkippedChannels(c, []OverloadSleepSkippedChannel{sampleSkipped(1), sampleSkipped(2)})

	items := GetOverloadSleepSkippedChannels(c)
	require.Len(t, items, 2)
	require.ElementsMatch(t, []int{1, 2}, []int{items[0].ChannelId, items[1].ChannelId})
}

func TestAddOverloadSleepSkippedChannels_IgnoresInvalidId(t *testing.T) {
	c := newOverloadSleepTestContext(t)
	AddOverloadSleepSkippedChannels(c, []OverloadSleepSkippedChannel{sampleSkipped(0), sampleSkipped(-1)})
	require.Empty(t, GetOverloadSleepSkippedChannels(c))
}

func TestInjectOverloadSleepInfoIntoOther_NoSkipNoop(t *testing.T) {
	c := newOverloadSleepTestContext(t)
	other := map[string]interface{}{"model_ratio": 1.0}

	InjectOverloadSleepInfoIntoOther(c, other)

	// 普通请求不应给 other 增加任何字段
	require.Len(t, other, 1)
	require.NotContains(t, other, OtherKeyOverloadSleepSkipped)
	require.NotContains(t, other, OtherKeyOverloadSleepAllSleeping)
}

func TestInjectOverloadSleepInfoIntoOther_WithSkip(t *testing.T) {
	c := newOverloadSleepTestContext(t)
	AddOverloadSleepSkippedChannels(c, []OverloadSleepSkippedChannel{sampleSkipped(3)})
	other := map[string]interface{}{}

	InjectOverloadSleepInfoIntoOther(c, other)

	require.Contains(t, other, OtherKeyOverloadSleepSkipped)
	items, ok := other[OtherKeyOverloadSleepSkipped].([]OverloadSleepSkippedChannel)
	require.True(t, ok)
	require.Len(t, items, 1)
	require.Equal(t, 3, items[0].ChannelId)
	// 未触发全部休眠兜底时不写该标记
	require.NotContains(t, other, OtherKeyOverloadSleepAllSleeping)
}

func TestInjectOverloadSleepInfoIntoOther_AllSleeping(t *testing.T) {
	c := newOverloadSleepTestContext(t)
	SetOverloadSleepAllSleeping(c)
	other := map[string]interface{}{}

	InjectOverloadSleepInfoIntoOther(c, other)

	require.Equal(t, true, other[OtherKeyOverloadSleepAllSleeping])
}

func TestInjectOverloadSleepInfoIntoOther_NilSafe(t *testing.T) {
	InjectOverloadSleepInfoIntoOther(nil, map[string]interface{}{})
	c := newOverloadSleepTestContext(t)
	InjectOverloadSleepInfoIntoOther(c, nil)
	require.Nil(t, GetOverloadSleepSkippedChannels(nil))
}

func TestInjectOverloadSleepInfoSerializesIntoJSON(t *testing.T) {
	c := newOverloadSleepTestContext(t)
	AddOverloadSleepSkippedChannels(c, []OverloadSleepSkippedChannel{sampleSkipped(7)})
	other := map[string]interface{}{"x": 1}
	InjectOverloadSleepInfoIntoOther(c, other)

	raw := common.MapToJsonStr(other)
	require.Contains(t, raw, OtherKeyOverloadSleepSkipped)
	require.Contains(t, raw, "\"channel_id\":7")
}
