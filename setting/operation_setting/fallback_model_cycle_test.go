package operation_setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func withCycleTestFallbackModels(t *testing.T, raw string) {
	t.Helper()
	cfg := GetOverloadSleepSetting()
	saved := cfg.FallbackModels
	t.Cleanup(func() { cfg.FallbackModels = saved })
	cfg.FallbackModels = raw
}

// TestDescribeFallbackModelCycle_DirectCycle 直接互指必须识别为环
func TestDescribeFallbackModelCycle_DirectCycle(t *testing.T) {
	withCycleTestFallbackModels(t, "a=b\nb=a")
	require.NotEmpty(t, DescribeFallbackModelCycle("a"))
	require.NotEmpty(t, DescribeFallbackModelCycle("b"))
}

// TestDescribeFallbackModelCycle_CycleNotReachableFromOrigin
//
// 回归测试（重要）：
// 配置 a=b、b=c、c=b 时，b 与 c 之间存在环，但从 a 出发只走一步就到 b，
// 把 a 的请求兜底到 b 是完全有效且有意义的（b 的渠道可用就能成功）。
// 旧实现会从 a 一路走到 b->c->b 的环并拒绝 a 的兜底，属于误杀。
// 正确行为：只有「从 origin 出发最终会绕回 origin」才算 origin 处于环中。
func TestDescribeFallbackModelCycle_CycleNotReachableFromOrigin(t *testing.T) {
	withCycleTestFallbackModels(t, "a=b\nb=c\nc=b")

	require.Empty(t, DescribeFallbackModelCycle("a"),
		"从 a 出发不会绕回 a，不应判定 a 处于环中（旧实现会误报 a -> b -> c -> b）")

	// b、c 之间确实成环，从 b 出发应被识别
	require.NotEmpty(t, DescribeFallbackModelCycle("b"))
}

// TestDescribeFallbackModelCycle_ThreeNodeCycleBackToOrigin 三节点绕回起点必须识别
func TestDescribeFallbackModelCycle_ThreeNodeCycleBackToOrigin(t *testing.T) {
	withCycleTestFallbackModels(t, "a=b\nb=c\nc=a")
	require.NotEmpty(t, DescribeFallbackModelCycle("a"))
}

// TestDescribeFallbackModelCycle_ChainNoCycle 纯链式配置不应误报
func TestDescribeFallbackModelCycle_ChainNoCycle(t *testing.T) {
	withCycleTestFallbackModels(t, "a=b\nb=c\nc=d")
	require.Empty(t, DescribeFallbackModelCycle("a"))
	require.Empty(t, DescribeFallbackModelCycle("b"))
}

// TestGetFallbackModel_Basic 基本取备用模型
func TestGetFallbackModel_Basic(t *testing.T) {
	withCycleTestFallbackModels(t, "gemini-3.6-flash=gemini-3.7-flash")
	require.Equal(t, "gemini-3.7-flash", GetFallbackModel("gemini-3.6-flash"))
	require.Empty(t, GetFallbackModel("not-configured"))
	require.Empty(t, GetFallbackModel(""))
}

// TestParseFallbackModelMappings_IgnoresInvalidLines 解析器容错
func TestParseFallbackModelMappings_IgnoresInvalidLines(t *testing.T) {
	raw := "\n" +
		"# comment\n" +
		"  a = b  \n" +
		"no-equals-line\n" +
		"=only-right\n" +
		"only-left=\n" +
		"self=self\n" +
		"c=d=e\n"
	mappings := ParseFallbackModelMappings(raw)
	require.Equal(t, "b", mappings["a"], "两侧空白应被去除")
	require.NotContains(t, mappings, "no-equals-line")
	require.NotContains(t, mappings, "self", "自映射应被丢弃")
	require.Equal(t, "d=e", mappings["c"], "应以第一个 '=' 切分")
	require.Len(t, mappings, 2)
}

// TestParseFallbackModelMappings_DuplicateLastWins 重复配置后者覆盖
func TestParseFallbackModelMappings_DuplicateLastWins(t *testing.T) {
	mappings := ParseFallbackModelMappings("a=b\na=c")
	require.Equal(t, "c", mappings["a"])
}

// TestFallbackModelsToString_StableAndRoundTrip 序列化稳定且可往返
func TestFallbackModelsToString_StableAndRoundTrip(t *testing.T) {
	raw := "b=y\na=x"
	out := FallbackModelsToString(ParseFallbackModelMappings(raw))
	require.Equal(t, "a=x\nb=y", out, "应按原模型名排序，保证输出稳定")

	roundTrip := ParseFallbackModelMappings(out)
	require.Equal(t, map[string]string{"a": "x", "b": "y"}, roundTrip)
}

// TestFallbackModelsToString_Empty 空输入
func TestFallbackModelsToString_Empty(t *testing.T) {
	require.Empty(t, FallbackModelsToString(nil))
	require.Empty(t, FallbackModelsToString(map[string]string{}))
}
