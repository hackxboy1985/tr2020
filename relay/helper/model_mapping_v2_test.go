package helper

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
)

func TestIsV2Mapping(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		expected bool
	}{
		{
			name:     "V2配置",
			json:     `{"version": 2, "rules": []}`,
			expected: true,
		},
		{
			name:     "V1配置",
			json:     `{"gpt-4": "gpt-4-turbo"}`,
			expected: false,
		},
		{
			name:     "空配置",
			json:     `{}`,
			expected: false,
		},
		{
			name:     "无效JSON",
			json:     `{invalid}`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isV2Mapping(tt.json)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractFirstLevelParam(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model:       "gpt-4",
		Temperature: lo.ToPtr(0.9),
		MaxTokens:   lo.ToPtr(uint(1000)),
		Stream:      lo.ToPtr(true),
	}

	tests := []struct {
		name     string
		field    string
		expected ParamValue
	}{
		{
			name:     "提取temperature",
			field:    "temperature",
			expected: ParamValue{Value: 0.9, Exists: true},
		},
		{
			name:     "提取max_tokens",
			field:    "max_tokens",
			expected: ParamValue{Value: float64(1000), Exists: true},
		},
		{
			name:     "提取stream",
			field:    "stream",
			expected: ParamValue{Value: true, Exists: true},
		},
		{
			name:     "提取不存在的字段",
			field:    "top_p",
			expected: ParamValue{Exists: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractFirstLevelParam(request, tt.field)
			assert.Equal(t, tt.expected.Exists, result.Exists)
			if result.Exists {
				assert.Equal(t, tt.expected.Value, result.Value)
			}
		})
	}
}

func TestCompareGreaterThan(t *testing.T) {
	tests := []struct {
		name     string
		actual   interface{}
		expected interface{}
		result   bool
	}{
		{"0.9 > 0.8", 0.9, 0.8, true},
		{"0.5 > 0.8", 0.5, 0.8, false},
		{"1000 > 500", 1000, 500, true},
		{"整数比较", 100, 50, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _ := compareGreaterThan(tt.actual, tt.expected)
			assert.Equal(t, tt.result, result)
		})
	}
}

func TestCompareEqual(t *testing.T) {
	tests := []struct {
		name     string
		actual   interface{}
		expected interface{}
		result   bool
	}{
		{"数值相等", 0.9, 0.9, true},
		{"数值不等", 0.9, 0.8, false},
		{"布尔值相等", true, true, true},
		{"字符串相等", "gpt-4", "gpt-4", true},
		{"字符串不等", "gpt-4", "gpt-3.5", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _ := compareEqual(tt.actual, tt.expected)
			assert.Equal(t, tt.result, result)
		})
	}
}

func TestCompareIn(t *testing.T) {
	tests := []struct {
		name     string
		actual   interface{}
		expected interface{}
		result   bool
	}{
		{
			"值在数组中",
			"low",
			[]interface{}{"low", "medium", "high"},
			true,
		},
		{
			"值不在数组中",
			"ultra",
			[]interface{}{"low", "medium", "high"},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _ := compareIn(tt.actual, tt.expected)
			assert.Equal(t, tt.result, result)
		})
	}
}

func TestApplyModelMappingV2_SimpleCondition(t *testing.T) {
	config := `{
		"version": 2,
		"rules": [
			{
				"source_model": "gpt-4",
				"conditions": [
					{"param": "temperature", "operator": ">", "value": 0.8}
				],
				"target_model": "gpt-4-turbo"
			}
		]
	}`

	// 测试条件匹配
	request := &dto.GeneralOpenAIRequest{
		Model:       "gpt-4",
		Temperature: lo.ToPtr(0.9),
	}

	mapped, isMapped, err := applyModelMappingV2(config, request, "gpt-4")
	assert.NoError(t, err)
	assert.True(t, isMapped)
	assert.Equal(t, "gpt-4-turbo", mapped)

	// 测试条件不匹配
	request2 := &dto.GeneralOpenAIRequest{
		Model:       "gpt-4",
		Temperature: lo.ToPtr(0.5),
	}

	mapped2, isMapped2, err2 := applyModelMappingV2(config, request2, "gpt-4")
	assert.NoError(t, err2)
	assert.False(t, isMapped2)
	assert.Equal(t, "gpt-4", mapped2)
}

func TestApplyModelMappingV2_MultipleConditions(t *testing.T) {
	config := `{
		"version": 2,
		"rules": [
			{
				"source_model": "gpt-4",
				"conditions": [
					{"param": "temperature", "operator": ">", "value": 0.8},
					{"param": "max_tokens", "operator": ">=", "value": 4000}
				],
				"target_model": "gpt-4-turbo"
			}
		]
	}`

	// 所有条件都满足
	request1 := &dto.GeneralOpenAIRequest{
		Model:       "gpt-4",
		Temperature: lo.ToPtr(0.9),
		MaxTokens:   lo.ToPtr(uint(5000)),
	}

	mapped1, isMapped1, err1 := applyModelMappingV2(config, request1, "gpt-4")
	assert.NoError(t, err1)
	assert.True(t, isMapped1)
	assert.Equal(t, "gpt-4-turbo", mapped1)

	// 只满足一个条件
	request2 := &dto.GeneralOpenAIRequest{
		Model:       "gpt-4",
		Temperature: lo.ToPtr(0.9),
		MaxTokens:   lo.ToPtr(uint(1000)),
	}

	mapped2, isMapped2, err2 := applyModelMappingV2(config, request2, "gpt-4")
	assert.NoError(t, err2)
	assert.False(t, isMapped2)
	assert.Equal(t, "gpt-4", mapped2)
}

func TestApplyModelMappingV2_Priority(t *testing.T) {
	config := `{
		"version": 2,
		"rules": [
			{
				"source_model": "gpt-4",
				"conditions": [
					{"param": "temperature", "operator": ">", "value": 0.8}
				],
				"target_model": "gpt-4-turbo",
				"priority": 2
			},
			{
				"source_model": "gpt-4",
				"conditions": [
					{"param": "temperature", "operator": ">", "value": 0.5}
				],
				"target_model": "gpt-4-standard",
				"priority": 1
			}
		]
	}`

	request := &dto.GeneralOpenAIRequest{
		Model:       "gpt-4",
		Temperature: lo.ToPtr(0.9),
	}

	// 应该匹配priority=1的规则
	mapped, isMapped, err := applyModelMappingV2(config, request, "gpt-4")
	assert.NoError(t, err)
	assert.True(t, isMapped)
	assert.Equal(t, "gpt-4-standard", mapped)
}

func TestApplyModelMappingV2_Fallback(t *testing.T) {
	config := `{
		"version": 2,
		"rules": [
			{
				"source_model": "gpt-4",
				"conditions": [
					{"param": "temperature", "operator": ">", "value": 0.8}
				],
				"target_model": "gpt-4-turbo"
			}
		],
		"fallback": {
			"gpt-4": "gpt-4o",
			"claude-3": "claude-3-sonnet"
		}
	}`

	// 条件不匹配，使用fallback
	request := &dto.GeneralOpenAIRequest{
		Model:       "gpt-4",
		Temperature: lo.ToPtr(0.5),
	}

	mapped, isMapped, err := applyModelMappingV2(config, request, "gpt-4")
	assert.NoError(t, err)
	assert.True(t, isMapped)
	assert.Equal(t, "gpt-4o", mapped)

	// 没有规则，直接使用fallback
	request2 := &dto.GeneralOpenAIRequest{
		Model: "claude-3",
	}

	mapped2, isMapped2, err2 := applyModelMappingV2(config, request2, "claude-3")
	assert.NoError(t, err2)
	assert.True(t, isMapped2)
	assert.Equal(t, "claude-3-sonnet", mapped2)
}

func TestApplyModelMappingV2_NoCondition(t *testing.T) {
	config := `{
		"version": 2,
		"rules": [
			{
				"source_model": "gpt-4",
				"target_model": "gpt-4-turbo"
			}
		]
	}`

	request := &dto.GeneralOpenAIRequest{
		Model: "gpt-4",
	}

	// 无条件规则，直接匹配
	mapped, isMapped, err := applyModelMappingV2(config, request, "gpt-4")
	assert.NoError(t, err)
	assert.True(t, isMapped)
	assert.Equal(t, "gpt-4-turbo", mapped)
}

func TestApplyModelMappingV2_ParamNotExists(t *testing.T) {
	config := `{
		"version": 2,
		"rules": [
			{
				"source_model": "gpt-4",
				"conditions": [
					{"param": "temperature", "operator": ">", "value": 0.8}
				],
				"target_model": "gpt-4-turbo"
			}
		]
	}`

	// 请求中没有temperature参数
	request := &dto.GeneralOpenAIRequest{
		Model: "gpt-4",
	}

	mapped, isMapped, err := applyModelMappingV2(config, request, "gpt-4")
	assert.NoError(t, err)
	assert.False(t, isMapped)
	assert.Equal(t, "gpt-4", mapped)
}
