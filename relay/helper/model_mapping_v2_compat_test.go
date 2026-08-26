package helper

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
)

func TestParseV2ConfigWithV1Compat(t *testing.T) {
	// V2配置中混合V1格式
	config := `{
		"version": 2,
		"gpt-3.5-turbo": "gpt-3.5-turbo-16k",
		"claude-2": "claude-2.1",
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

	parsed, err := parseV2ConfigWithV1Compat(config)
	assert.NoError(t, err)
	assert.Equal(t, 2, parsed.Version)
	assert.Equal(t, 1, len(parsed.Rules))

	// 验证V1兼容映射被提取
	assert.Equal(t, 2, len(parsed.V1Compat))
	assert.Equal(t, "gpt-3.5-turbo-16k", parsed.V1Compat["gpt-3.5-turbo"])
	assert.Equal(t, "claude-2.1", parsed.V1Compat["claude-2"])
}

func TestApplyModelMappingV2_WithV1Compat(t *testing.T) {
	config := `{
		"version": 2,
		"gpt-3.5-turbo": "gpt-3.5-turbo-16k",
		"claude-3": "claude-3-sonnet",
		"gpt-4": "gpt-4o",
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

	// 测试1: V1兼容映射（无规则的模型）
	request1 := &dto.GeneralOpenAIRequest{
		Model: "gpt-3.5-turbo",
	}
	mapped1, isMapped1, err1 := applyModelMappingV2(config, request1, "gpt-3.5-turbo")
	assert.NoError(t, err1)
	assert.True(t, isMapped1)
	assert.Equal(t, "gpt-3.5-turbo-16k", mapped1)

	// 测试2: V2规则匹配
	request2 := &dto.GeneralOpenAIRequest{
		Model:       "gpt-4",
		Temperature: lo.ToPtr(0.9),
	}
	mapped2, isMapped2, err2 := applyModelMappingV2(config, request2, "gpt-4")
	assert.NoError(t, err2)
	assert.True(t, isMapped2)
	assert.Equal(t, "gpt-4-turbo", mapped2)

	// 测试3: V2规则不匹配，使用V1兼容
	request3 := &dto.GeneralOpenAIRequest{
		Model:       "gpt-4",
		Temperature: lo.ToPtr(0.5),
	}
	mapped3, isMapped3, err3 := applyModelMappingV2(config, request3, "gpt-4")
	assert.NoError(t, err3)
	assert.True(t, isMapped3)
	assert.Equal(t, "gpt-4o", mapped3)

	// 测试4: V1兼容映射（另一个模型）
	request4 := &dto.GeneralOpenAIRequest{
		Model: "claude-3",
	}
	mapped4, isMapped4, err4 := applyModelMappingV2(config, request4, "claude-3")
	assert.NoError(t, err4)
	assert.True(t, isMapped4)
	assert.Equal(t, "claude-3-sonnet", mapped4)
}

func TestV1V2Priority(t *testing.T) {
	// 验证优先级：V2规则 > V1兼容
	config := `{
		"version": 2,
		"gpt-4": "gpt-4-v1-compat",
		"rules": [
			{
				"source_model": "gpt-4",
				"target_model": "gpt-4-v2-rule"
			}
		]
	}`

	request := &dto.GeneralOpenAIRequest{
		Model: "gpt-4",
	}

	// V2规则应该优先（无条件规则）
	mapped, isMapped, err := applyModelMappingV2(config, request, "gpt-4")
	assert.NoError(t, err)
	assert.True(t, isMapped)
	assert.Equal(t, "gpt-4-v2-rule", mapped)
}

func TestV1CompatAsLastResort(t *testing.T) {
	// V1兼容作为最后兜底
	config := `{
		"version": 2,
		"gemini-pro": "gemini-pro-v1-compat",
		"rules": []
	}`

	request := &dto.GeneralOpenAIRequest{
		Model: "gemini-pro",
	}

	// 没有规则，使用V1兼容
	mapped, isMapped, err := applyModelMappingV2(config, request, "gemini-pro")
	assert.NoError(t, err)
	assert.True(t, isMapped)
	assert.Equal(t, "gemini-pro-v1-compat", mapped)
}

