package operation_setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// withFallbackModels 临时设置备用模型配置，测试结束后恢复原值。
func withFallbackModels(t *testing.T, raw string) {
	t.Helper()
	original := overloadSleepSetting
	t.Cleanup(func() { overloadSleepSetting = original })
	overloadSleepSetting.FallbackModels = raw
}

func TestParseFallbackModelMappings_Normal(t *testing.T) {
	mappings := ParseFallbackModelMappings("gemini-3.6-flash=gemini-3.7-flash\ngpt-4o=gpt-4o-mini")
	require.Equal(t, map[string]string{
		"gemini-3.6-flash": "gemini-3.7-flash",
		"gpt-4o":           "gpt-4o-mini",
	}, mappings)
}

func TestParseFallbackModelMappings_Empty(t *testing.T) {
	require.Empty(t, ParseFallbackModelMappings(""))
	require.Empty(t, ParseFallbackModelMappings("   \n\t\n  "))
}

func TestParseFallbackModelMappings_IgnoresCommentsAndBlankLines(t *testing.T) {
	raw := "# 这是注释\n\n  \na=b\n  # 缩进的注释也被忽略\nc=d\n"
	mappings := ParseFallbackModelMappings(raw)
	require.Equal(t, map[string]string{"a": "b", "c": "d"}, mappings)
}

func TestParseFallbackModelMappings_TrimsWhitespace(t *testing.T) {
	mappings := ParseFallbackModelMappings("  a  =  b  \r\n\tc=\td\t")
	require.Equal(t, map[string]string{"a": "b", "c": "d"}, mappings)
}

func TestParseFallbackModelMappings_DuplicateOriginLastWins(t *testing.T) {
	mappings := ParseFallbackModelMappings("a=b\na=c\na=d")
	require.Equal(t, map[string]string{"a": "d"}, mappings)
}

func TestParseFallbackModelMappings_DropsSelfMapping(t *testing.T) {
	mappings := ParseFallbackModelMappings("a=a\nb=c")
	require.Equal(t, map[string]string{"b": "c"}, mappings)
	require.NotContains(t, mappings, "a")
}

func TestParseFallbackModelMappings_DropsLinesWithoutEqualSign(t *testing.T) {
	// 运行时解析器静默丢弃非法行（配置期校验由 controller 层负责）
	mappings := ParseFallbackModelMappings("no-equals-sign\na=b")
	require.Equal(t, map[string]string{"a": "b"}, mappings)
}

func TestParseFallbackModelMappings_DropsEmptySides(t *testing.T) {
	mappings := ParseFallbackModelMappings("a=\n=b\n= \na=b")
	require.Equal(t, map[string]string{"a": "b"}, mappings)
}

func TestParseFallbackModelMappings_SplitsOnFirstEqualSign(t *testing.T) {
	mappings := ParseFallbackModelMappings("a=b=c")
	require.Equal(t, map[string]string{"a": "b=c"}, mappings)
}

func TestGetFallbackModel(t *testing.T) {
	withFallbackModels(t, "a=b\nc=d")

	require.Equal(t, "b", GetFallbackModel("a"))
	require.Equal(t, "d", GetFallbackModel("c"))
	require.Equal(t, "", GetFallbackModel("not-configured"))
	require.Equal(t, "", GetFallbackModel(""))
}

func TestGetFallbackModel_EmptyConfig(t *testing.T) {
	withFallbackModels(t, "")
	require.Equal(t, "", GetFallbackModel("a"))
}

func TestFallbackModelsToString(t *testing.T) {
	require.Equal(t, "", FallbackModelsToString(nil))
	require.Equal(t, "", FallbackModelsToString(map[string]string{}))
	// 输出按原模型名排序，保证配置回显稳定
	require.Equal(t,
		"a=b\nc=d",
		FallbackModelsToString(map[string]string{"c": "d", "a": "b"}),
	)
}

func TestFallbackModelsToString_RoundTrip(t *testing.T) {
	raw := "b=y\na=x"
	mappings := ParseFallbackModelMappings(raw)
	require.Equal(t, "a=x\nb=y", FallbackModelsToString(mappings))
}

func TestDescribeFallbackModelCycle_NoCycle(t *testing.T) {
	withFallbackModels(t, "a=b\nb=c")

	require.Equal(t, "", DescribeFallbackModelCycle("a"))
	require.Equal(t, "", DescribeFallbackModelCycle("b"))
	require.Equal(t, "", DescribeFallbackModelCycle("c"))
	require.Equal(t, "", DescribeFallbackModelCycle("unrelated"))
	require.Equal(t, "", DescribeFallbackModelCycle(""))
}

func TestDescribeFallbackModelCycle_TwoNodeCycle(t *testing.T) {
	// 需求明确允许 3.6 -> 3.7、3.7 -> 3.6 这样配置，环只在运行时中断
	withFallbackModels(t, "a=b\nb=a")

	require.Equal(t, "a -> b -> a", DescribeFallbackModelCycle("a"))
	require.Equal(t, "b -> a -> b", DescribeFallbackModelCycle("b"))
}

func TestDescribeFallbackModelCycle_ThreeNodeCycle(t *testing.T) {
	withFallbackModels(t, "a=b\nb=c\nc=a")

	require.Equal(t, "a -> b -> c -> a", DescribeFallbackModelCycle("a"))
	require.Equal(t, "b -> c -> a -> b", DescribeFallbackModelCycle("b"))
	require.Equal(t, "c -> a -> b -> c", DescribeFallbackModelCycle("c"))
}

func TestDescribeFallbackModelCycle_SelfLoopDroppedByParser(t *testing.T) {
	// 自映射在解析阶段就被丢弃，因此不会构成环
	withFallbackModels(t, "a=a")
	require.Equal(t, "", DescribeFallbackModelCycle("a"))
}

func TestDescribeFallbackModelCycle_ChainEnteringCycle(t *testing.T) {
	// x -> a -> b -> a：链路内部存在 a<->b 的环，但不会绕回 x 本身，
	// 因此 x 仍可安全兜底到 a 一次（a 的渠道可用就能成功）。
	// 只有当兜底到 a 后 a 也全部休眠时，才会触及 a<->b 的环，
	// 那时由 a 这一跳自身的环检测中断（见 TestDescribeFallbackModelCycle_DirectCycle）。
	withFallbackModels(t, "x=a\na=b\nb=a")
	require.Equal(t, "", DescribeFallbackModelCycle("x"))
	// 而 a 自身处于环中，必须被拦截
	require.NotEmpty(t, DescribeFallbackModelCycle("a"))
}

func TestDescribeFallbackModelCycle_EmptyConfig(t *testing.T) {
	withFallbackModels(t, "")
	require.Equal(t, "", DescribeFallbackModelCycle("a"))
}
