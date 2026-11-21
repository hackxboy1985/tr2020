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

func newOverloadNameTestContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	return c
}

// TestOverloadSleepModelName_NormalRequest 普通请求（无兜底）应使用原模型名
func TestOverloadSleepModelName_NormalRequest(t *testing.T) {
	c := newOverloadNameTestContext()
	info := &relaycommon.RelayInfo{OriginModelName: "gemini-3.6-flash"}

	require.Equal(t, "gemini-3.6-flash", overloadSleepModelName(c, info))
}

// TestOverloadSleepModelName_UsesFallbackModel
//
// 这是 OCR 发现、经复核确认的真实缺陷的回归测试：
// 当本次请求已兜底切换到备用模型时，上游返回过载状态码必须把休眠记在
// **备用模型**名下。否则会：
//   - 污染原模型的休眠池（该渠道对原模型可能完全健康）；
//   - 导致备用模型自身永不被标记休眠，后续请求反复命中同一个过载渠道。
func TestOverloadSleepModelName_UsesFallbackModel(t *testing.T) {
	c := newOverloadNameTestContext()
	info := &relaycommon.RelayInfo{OriginModelName: "gemini-3.6-flash"}

	// 模拟已兜底切换到 3.7
	service.MarkFallbackSwitched(c, "gemini-3.6-flash", "gemini-3.7-flash")

	require.Equal(t, "gemini-3.7-flash", overloadSleepModelName(c, info),
		"已兜底时必须用备用模型名标记休眠，避免污染原模型休眠池")
}

// TestOverloadSleepModelName_NilSafe 容错
func TestOverloadSleepModelName_NilSafe(t *testing.T) {
	c := newOverloadNameTestContext()
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "from-context")

	require.Equal(t, "from-context", overloadSleepModelName(c, nil))
}
