package helper

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/samber/lo"
	"github.com/tidwall/gjson"
)

// 模拟小请求：简单对话
func createSmallRequest() *dto.GeneralOpenAIRequest {
	return &dto.GeneralOpenAIRequest{
		Model:       "gpt-4",
		Temperature: lo.ToPtr(0.9),
		MaxTokens:   lo.ToPtr(uint(1000)),
		Messages: []dto.Message{
			{Role: "user", Content: "Hello, how are you?"},
		},
	}
}

// 模拟中等请求：多轮对话
func createMediumRequest() *dto.GeneralOpenAIRequest {
	messages := make([]dto.Message, 10)
	for i := 0; i < 10; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		messages[i] = dto.Message{
			Role:    role,
			Content: strings.Repeat("This is a test message. ", 20), // ~500字符
		}
	}
	return &dto.GeneralOpenAIRequest{
		Model:       "gpt-4",
		Temperature: lo.ToPtr(0.9),
		MaxTokens:   lo.ToPtr(uint(4000)),
		TopP:        lo.ToPtr(0.95),
		Messages:    messages,
	}
}

// 模拟大请求：长上下文
func createLargeRequest() *dto.GeneralOpenAIRequest {
	messages := make([]dto.Message, 50)
	for i := 0; i < 50; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		messages[i] = dto.Message{
			Role:    role,
			Content: strings.Repeat("This is a longer test message with more content. ", 100), // ~5000字符
		}
	}
	return &dto.GeneralOpenAIRequest{
		Model:       "gpt-4",
		Temperature: lo.ToPtr(0.7),
		MaxTokens:   lo.ToPtr(uint(8000)),
		TopP:        lo.ToPtr(0.9),
		TopK:        lo.ToPtr(50),
		Messages:    messages,
	}
}

// 方案1: 完整序列化 + gjson
func extractParamWithGjson(request *dto.GeneralOpenAIRequest, path string) (interface{}, bool) {
	requestJSON, _ := json.Marshal(request)
	result := gjson.GetBytes(requestJSON, path)
	if !result.Exists() {
		return nil, false
	}
	return result.Value(), true
}

// 方案2: 直接访问字段（理想情况）
func extractParamDirect(request *dto.GeneralOpenAIRequest, path string) (interface{}, bool) {
	switch path {
	case "temperature":
		if request.Temperature != nil {
			return *request.Temperature, true
		}
	case "max_tokens":
		if request.MaxTokens != nil {
			return *request.MaxTokens, true
		}
	case "top_p":
		if request.TopP != nil {
			return *request.TopP, true
		}
	}
	return nil, false
}

// 基准测试：小请求
func BenchmarkSmallRequest_Gjson(b *testing.B) {
	request := createSmallRequest()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractParamWithGjson(request, "temperature")
	}
}

func BenchmarkSmallRequest_Direct(b *testing.B) {
	request := createSmallRequest()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractParamDirect(request, "temperature")
	}
}

// 基准测试：中等请求
func BenchmarkMediumRequest_Gjson(b *testing.B) {
	request := createMediumRequest()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractParamWithGjson(request, "temperature")
	}
}

func BenchmarkMediumRequest_Direct(b *testing.B) {
	request := createMediumRequest()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractParamDirect(request, "temperature")
	}
}

// 基准测试：大请求
func BenchmarkLargeRequest_Gjson(b *testing.B) {
	request := createLargeRequest()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractParamWithGjson(request, "temperature")
	}
}

func BenchmarkLargeRequest_Direct(b *testing.B) {
	request := createLargeRequest()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractParamDirect(request, "temperature")
	}
}

// 基准测试：多次参数提取（真实场景）
func BenchmarkLargeRequest_Gjson_MultipleParams(b *testing.B) {
	request := createLargeRequest()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractParamWithGjson(request, "temperature")
		extractParamWithGjson(request, "max_tokens")
		extractParamWithGjson(request, "top_p")
	}
}

func BenchmarkLargeRequest_Direct_MultipleParams(b *testing.B) {
	request := createLargeRequest()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractParamDirect(request, "temperature")
		extractParamDirect(request, "max_tokens")
		extractParamDirect(request, "top_p")
	}
}

// 基准测试：只序列化一次（优化方案）
func BenchmarkLargeRequest_Gjson_CachedJSON(b *testing.B) {
	request := createLargeRequest()
	requestJSON, _ := json.Marshal(request) // 提前序列化

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gjson.GetBytes(requestJSON, "temperature")
		gjson.GetBytes(requestJSON, "max_tokens")
		gjson.GetBytes(requestJSON, "top_p")
	}
}
