package deepseek

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSummarizeOutboundReasoningReplay(t *testing.T) {
	reasoning := "Paris weather check."
	placeholder := deepSeekReasoningPlaceholder
	empty := ""
	request := &dto.GeneralOpenAIRequest{
		Model: "deepseek-v4-pro",
		Tools: []dto.ToolCallRequest{
			{ID: "call_1", Type: "function", Function: dto.FunctionRequest{Name: "get_weather"}},
		},
		Messages: []dto.Message{
			{Role: "user", Content: "weather?"},
			{Role: "assistant", ReasoningContent: &reasoning},
			{Role: "assistant", ReasoningContent: &placeholder},
			{Role: "assistant", ReasoningContent: &empty},
			{Role: "tool", Content: "sunny", ToolCallId: "call_1"},
		},
	}

	body, err := common.Marshal(request)
	require.NoError(t, err)

	summary, ok := summarizeOutboundReasoningReplay(body)
	require.True(t, ok)
	assert.Equal(t, 1, summary.Tools)
	assert.Equal(t, 3, summary.Assistant)
	assert.Equal(t, 3, summary.WithReasoning)
	assert.Equal(t, 1, summary.Placeholder)
	assert.Equal(t, 1, summary.Empty)
	assert.Equal(t, []int{1, 2, 3}, summary.Indexes)
	assert.Equal(t, reasoning, summary.Preview)
}

func TestSummarizeOutboundReasoningReplayWithoutMessages(t *testing.T) {
	body, err := common.Marshal(dto.GeneralOpenAIRequest{Model: "deepseek-v4-pro"})
	require.NoError(t, err)
	_, ok := summarizeOutboundReasoningReplay(body)
	assert.False(t, ok)
}

func TestSummarizeOutboundReasoningReplayWithoutTools(t *testing.T) {
	reasoning := "thinking"
	request := &dto.GeneralOpenAIRequest{
		Model: "deepseek-v4-pro",
		Messages: []dto.Message{
			{Role: "assistant", Content: "hello", ReasoningContent: &reasoning},
		},
	}
	body, err := common.Marshal(request)
	require.NoError(t, err)

	summary, ok := summarizeOutboundReasoningReplay(body)
	require.True(t, ok)
	assert.Equal(t, 0, summary.Tools)
	assert.Equal(t, 1, summary.WithReasoning)
}
