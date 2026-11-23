package controller

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newCursorTestContext(t *testing.T, tokenGroup string, userGroup string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(c, constant.ContextKeyTokenGroup, tokenGroup)
	common.SetContextKey(c, constant.ContextKeyUserGroup, userGroup)
	return c
}

// TestResetAutoGroupCursor_SetsIndexToTargetGroup
//
// auto 分组下，备用模型兜底命中某个分组后，后续重试必须从该分组重新开始，
// 不能沿用原模型遗留的游标，否则会跳过本该命中的分组。
func TestResetAutoGroupCursor_SetsIndexToTargetGroup(t *testing.T) {
	c := newCursorTestContext(t, "auto", "default")
	common.SetContextKey(c, constant.ContextKeyAutoGroupIndex, 99)

	service.GetUserAutoGroup("default") // 确保 auto 链已就绪
	resetAutoGroupCursor(c, "default")

	index := common.GetContextKeyInt(c, constant.ContextKeyAutoGroupIndex)
	require.NotEqual(t, 99, index, "游标必须被重置，不能沿用原模型遗留值")
	require.GreaterOrEqual(t, index, 0)
}

// TestResetAutoGroupCursor_NonAutoRequestNoop
//
// 指定分组的请求不涉及 auto 分组游标，不应被改动。
func TestResetAutoGroupCursor_NonAutoRequestNoop(t *testing.T) {
	c := newCursorTestContext(t, "vip", "default")
	common.SetContextKey(c, constant.ContextKeyAutoGroupIndex, 5)

	resetAutoGroupCursor(c, "vip")

	require.Equal(t, 5, common.GetContextKeyInt(c, constant.ContextKeyAutoGroupIndex),
		"指定分组的请求不应改动 auto 分组游标")
}

// TestResetAutoGroupCursor_UnknownGroupFallsBackToZero
// 目标分组不在 auto 链内时回到链首，避免越界索引导致后续选择异常。
func TestResetAutoGroupCursor_UnknownGroupFallsBackToZero(t *testing.T) {
	c := newCursorTestContext(t, "auto", "default")
	common.SetContextKey(c, constant.ContextKeyAutoGroupIndex, 7)

	resetAutoGroupCursor(c, "一个不存在的分组")

	require.Equal(t, 0, common.GetContextKeyInt(c, constant.ContextKeyAutoGroupIndex))
}

// TestApplyFallbackGroupRatio_SyncsGroupEvenIfRatioSame
//
// 即使两个分组倍率恰好相同，也必须把 UsingGroup 与 auto_group
// 同步为目标分组，否则日志与结算记录的分组会与实际渠道不一致。
func TestApplyFallbackGroupRatio_SyncsGroupEvenIfRatioSame(t *testing.T) {
	c := newCursorTestContext(t, "auto", "default")
	info := &relaycommon.RelayInfo{UsingGroup: "default", UserGroup: "default"}
	info.PriceData.GroupRatioInfo.GroupRatio = 1.0

	applyFallbackGroupRatio(c, info, "vip")

	require.Equal(t, "vip", info.UsingGroup)
}
