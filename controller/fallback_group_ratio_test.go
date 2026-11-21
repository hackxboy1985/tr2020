package controller

import (
	"net/http/httptest"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestApplyFallbackGroupRatio_SameGroupNoop 同分组时不应改动任何计费状态。
//
// 这是最常见的情况：备用模型的候选渠道就是从原分组里检索的，
// 因此分组倍率天然不变，不应有任何副作用。
func TestApplyFallbackGroupRatio_SameGroupNoop(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{UsingGroup: "default"}
	info.PriceData.GroupRatioInfo.GroupRatio = 1.5

	applyFallbackGroupRatio(c, info, "default")

	require.Equal(t, "default", info.UsingGroup)
	require.Equal(t, 1.5, info.PriceData.GroupRatioInfo.GroupRatio)
}

// TestApplyFallbackGroupRatio_DifferentGroupReprices
// 跨分组兜底时，分组倍率必须按「实际命中分组」重算，否则会用错倍率。
func TestApplyFallbackGroupRatio_DifferentGroupReprices(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{UsingGroup: "default", UserGroup: "default"}
	info.PriceData.GroupRatioInfo.GroupRatio = 1.0

	applyFallbackGroupRatio(c, info, "vip")

	require.Equal(t, "vip", info.UsingGroup, "应切到实际命中的分组")
	require.Equal(t, ratio_setting.GetGroupRatio("vip"), info.PriceData.GroupRatioInfo.GroupRatio,
		"分组倍率必须按实际命中分组重算")
}

// TestApplyFallbackGroupRatio_EmptyGroupNoop 空分组是异常输入，不应改动状态
func TestApplyFallbackGroupRatio_EmptyGroupNoop(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{UsingGroup: "default"}
	info.PriceData.GroupRatioInfo.GroupRatio = 1.0

	applyFallbackGroupRatio(c, info, "")

	require.Equal(t, "default", info.UsingGroup)
	require.Equal(t, 1.0, info.PriceData.GroupRatioInfo.GroupRatio)
}
