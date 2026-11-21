package controller

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// setupFallbackTestContext 准备备用模型测试的配置与 gin.Context。
// 返回的 cleanup 会恢复配置，避免污染其它测试。
func setupFallbackTestContext(t *testing.T, fallbackModels string, tokenSwitch uint8) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)

	cfg := operation_setting.GetOverloadSleepSetting()
	savedEnabled := cfg.Enabled
	savedFallback := cfg.FallbackModels
	t.Cleanup(func() {
		cfg.Enabled = savedEnabled
		cfg.FallbackModels = savedFallback
		operation_setting.NormalizeOverloadSleepSetting()
	})
	cfg.Enabled = true
	cfg.FallbackModels = fallbackModels
	operation_setting.NormalizeOverloadSleepSetting()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	switch tokenSwitch {
	case 1:
		common.SetContextKey(c, constant.ContextKeyTokenFallbackModelEnabled, true)
	case 2:
		common.SetContextKey(c, constant.ContextKeyTokenFallbackModelEnabled, false)
	}
	return c
}

// TestFallbackNotUsedWhenTokenSwitchOff 是本功能最重要的不变式：
//
//	API 密钥的「使用相近模型兜底」开关关闭时，绝对不允许切换到备用模型，
//	必须回退到「原模型休眠池中的最后一个渠道」重试。
//
// 因此 ResolveFallbackModel 必须返回 false，调用方（tryFallbackModel）
// 就会保留原渠道继续请求。
func TestFallbackNotUsedWhenTokenSwitchOff(t *testing.T) {
	c := setupFallbackTestContext(t, "gemini-3.6-flash=gemini-3.7-flash", 2)

	require.False(t, service.IsFallbackModelEnabledForToken(c),
		"密钥开关关闭时应读到 false")

	fallback, ok := service.ResolveFallbackModel(c, "gemini-3.6-flash")
	require.False(t, ok, "密钥开关关闭时不得启用备用模型兜底")
	require.Empty(t, fallback)
}

// TestFallbackUsedWhenTokenSwitchOn 对照测试：开关打开时才允许兜底
func TestFallbackUsedWhenTokenSwitchOn(t *testing.T) {
	c := setupFallbackTestContext(t, "gemini-3.6-flash=gemini-3.7-flash", 1)

	require.True(t, service.IsFallbackModelEnabledForToken(c))

	fallback, ok := service.ResolveFallbackModel(c, "gemini-3.6-flash")
	require.True(t, ok)
	require.Equal(t, "gemini-3.7-flash", fallback)
}

// TestFallbackBlockedWhenSleepPoolDisabled 休眠池总开关关闭时不兜底
func TestFallbackBlockedWhenSleepPoolDisabled(t *testing.T) {
	c := setupFallbackTestContext(t, "a=b", 1)
	operation_setting.GetOverloadSleepSetting().Enabled = false

	_, ok := service.ResolveFallbackModel(c, "a")
	require.False(t, ok)
}

// TestFallbackBlockedByConfigCycle 配置环（a=b 且 b=a）必须在运行时被中断
func TestFallbackBlockedByConfigCycle(t *testing.T) {
	c := setupFallbackTestContext(t, "a=b\nb=a", 1)

	_, ok := service.ResolveFallbackModel(c, "a")
	require.False(t, ok, "互相映射的配置环必须被中断，避免死循环")
}

// TestFallbackOnlyOnce 兜底只做一次，禁止 A->B->C 无限链
func TestFallbackOnlyOnce(t *testing.T) {
	c := setupFallbackTestContext(t, "a=b\nb=c", 1)

	// 模拟已经从 a 兜底到 b
	service.MarkFallbackSwitched(c, "a", "b")

	_, ok := service.ResolveFallbackModel(c, "b")
	require.False(t, ok, "已兜底过就不应再次兜底，否则会形成 A->B->C 的链")
}

// TestFallbackNoFallbackConfigured 未配置备用模型时不兜底
func TestFallbackNoFallbackConfigured(t *testing.T) {
	c := setupFallbackTestContext(t, "", 1)

	_, ok := service.ResolveFallbackModel(c, "a")
	require.False(t, ok)
}
