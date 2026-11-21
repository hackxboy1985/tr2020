package helper

import (
	"net/http/httptest"
	"testing"

	rootcommon "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newMappingTestContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	return c
}

// TestModelMappedHelper_UsesFallbackModelNameForUpstream
//
// 验证备用模型兜底的关键行为：
//   - 上游实际请求使用备用模型名（否则上游会收到不认识的模型名）；
//   - 但 info.OriginModelName 保持不变（保证计费口径与用户请求一致）。
func TestModelMappedHelper_UsesFallbackModelNameForUpstream(t *testing.T) {
	c := newMappingTestContext()
	rootcommon.SetContextKey(c, constant.ContextKeyFallbackModelName, "gemini-3.7-flash")

	info := &relaycommon.RelayInfo{OriginModelName: "gemini-3.6-flash"}
	err := ModelMappedHelper(c, info, nil)
	require.NoError(t, err)

	require.Equal(t, "gemini-3.7-flash", info.UpstreamModelName,
		"上游应使用备用模型名")
	require.Equal(t, "gemini-3.6-flash", info.OriginModelName,
		"计费用模型名必须保持为原模型，避免预扣/结算口径不一致")
}

// TestModelMappedHelper_NoFallbackKeepsOriginModel 未兜底时行为保持不变
func TestModelMappedHelper_NoFallbackKeepsOriginModel(t *testing.T) {
	c := newMappingTestContext()

	info := &relaycommon.RelayInfo{OriginModelName: "gpt-4o"}
	err := ModelMappedHelper(c, info, nil)
	require.NoError(t, err)

	require.Equal(t, "gpt-4o", info.UpstreamModelName)
	require.Equal(t, "gpt-4o", info.OriginModelName)
}

// TestModelMappedHelper_FallbackRespectsChannelMapping
//
// 兜底到备用模型后，仍要应用该渠道自己的模型映射。
// 例：兜底到 3.7，而渠道把 3.7 映射为 3.5，则上游应收到 3.5。
func TestModelMappedHelper_FallbackRespectsChannelMapping(t *testing.T) {
	c := newMappingTestContext()
	rootcommon.SetContextKey(c, constant.ContextKeyFallbackModelName, "gemini-3.7-flash")
	c.Set("model_mapping", `{"gemini-3.7-flash":"gemini-3.5-flash"}`)

	info := &relaycommon.RelayInfo{OriginModelName: "gemini-3.6-flash"}
	err := ModelMappedHelper(c, info, nil)
	require.NoError(t, err)

	require.Equal(t, "gemini-3.5-flash", info.UpstreamModelName,
		"备用模型也应套用渠道的模型映射")
	require.Equal(t, "gemini-3.6-flash", info.OriginModelName,
		"计费模型不因映射而改变")
}
