package controller

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateFallbackModelMappingsFormat_Valid(t *testing.T) {
	cases := []string{
		"",
		"   ",
		"\n\n\t\n",
		"a=b",
		"a=b\nc=d",
		"# 注释\n\n  \na=b\n  # 缩进注释\nc=d\n",
		"  a  =  b  ",
		"a=b=c",    // 只用第一个 = 切分，右侧原样保留
		"a=a",      // 自映射允许配置保存，解析器会丢弃
		"a=b\nb=a", // 配置环允许保存，运行时中断
		"3.6=3.7\n3.7=3.6",
	}
	for _, raw := range cases {
		require.NoError(t, validateFallbackModelMappingsFormat(raw), "raw=%q", raw)
	}
}

func TestValidateFallbackModelMappingsFormat_Invalid(t *testing.T) {
	cases := []string{
		"no-equals-sign",
		"a=b\nno-equals-sign",
		"a=",
		"=b",
		"a = ",
		" = ",
		"a=b\n=",
	}
	for _, raw := range cases {
		err := validateFallbackModelMappingsFormat(raw)
		require.Error(t, err, "raw=%q 应判定为非法", raw)
		require.Contains(t, err.Error(), "备用模型配置格式非法")
	}
}

func TestValidateFallbackModelsOptionValue_RejectsNonString(t *testing.T) {
	// /api/option/ 的 value 是 any：数组/对象/数字经统一格式化后会变成
	// "[a=b]"、"[map[...]]"、"1" 之类的字符串，必须在这里拦住，否则会被写坏。
	require.Error(t, validateFallbackModelsOptionValue([]any{"a=b"}))
	require.Error(t, validateFallbackModelsOptionValue(map[string]any{"a": "b"}))
	require.Error(t, validateFallbackModelsOptionValue(float64(1)))
	require.Error(t, validateFallbackModelsOptionValue(true))
	require.Error(t, validateFallbackModelsOptionValue(nil))

	require.NoError(t, validateFallbackModelsOptionValue(""))
	require.NoError(t, validateFallbackModelsOptionValue("a=b"))
}

func TestValidateFallbackModelMappingsFormat_ReportsLineNumber(t *testing.T) {
	err := validateFallbackModelMappingsFormat("a=b\n\nbad-line")
	require.Error(t, err)
	require.Contains(t, err.Error(), "第 3 行")
}
