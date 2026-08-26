package helper

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/assert"
)

func TestRealWorldCompatConfig(t *testing.T) {
	// 真实的兼容配置（你提供的格式）
	config := `{
		"version": 2,
		"rules": [],
		"fallback": {
			"gpt-4": "gpt-4-turbo"
		},
		"gpt-4": "gpt-4-turbo"
	}`

	request := &dto.GeneralOpenAIRequest{
		Model: "gpt-4",
	}

	// A机器（新代码）应该使用V1兼容映射
	mapped, isMapped, err := applyModelMappingV2(config, request, "gpt-4")
	assert.NoError(t, err)
	assert.True(t, isMapped)
	assert.Equal(t, "gpt-4-turbo", mapped)
}

func TestOldMachineCompatibility(t *testing.T) {
	// 模拟B机器（旧代码）的行为
	config := `{
		"version": 2,
		"rules": [],
		"fallback": {
			"gpt-4": "gpt-4-turbo"
		},
		"gpt-4": "gpt-4-turbo",
		"claude-3": "claude-3-sonnet"
	}`

	// 旧代码会这样解析
	var v1Map map[string]string
	err := json.Unmarshal([]byte(config), &v1Map)

	// 不会报错，会提取字符串字段
	assert.NoError(t, err)
	assert.Equal(t, "gpt-4-turbo", v1Map["gpt-4"])
	assert.Equal(t, "claude-3-sonnet", v1Map["claude-3"])

	// version, rules, fallback 被忽略（类型不匹配）
	_, hasVersion := v1Map["version"]
	assert.False(t, hasVersion)
}
