package deepseek

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func newDeepSeekRelayInfo(model string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		OriginModelName: model,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "https://api.deepseek.com",
			UpstreamModelName: model,
		},
	}
}

func toolCallMessage(t *testing.T) dto.Message {
	t.Helper()
	toolCalls, err := common.Marshal([]dto.ToolCallRequest{
		{ID: "call_1", Type: "function", Function: dto.FunctionRequest{Name: "get_weather", Arguments: "{}"}},
	})
	require.NoError(t, err)
	return dto.Message{Role: "assistant", ToolCalls: toolCalls}
}

// Regression: a replayed tools-bearing thinking-mode request whose assistant
// turn lost reasoning_content used to be forwarded verbatim, so DeepSeek
// rejected it with 400 "reasoning_content must be passed back to the API".
func TestConvertOpenAIRequestBackfillsReasoningContentForToolHistory(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model: "deepseek-v4-pro",
		Tools: []dto.ToolCallRequest{
			{Type: "function", Function: dto.FunctionRequest{Name: "get_weather"}},
		},
		Messages: []dto.Message{
			{Role: "user", Content: "weather?"},
			toolCallMessage(t),
			{Role: "tool", Content: "sunny", ToolCallId: "call_1"},
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, newDeepSeekRelayInfo("deepseek-v4-pro"), request)
	require.NoError(t, err)

	convertedRequest, ok := converted.(*dto.GeneralOpenAIRequest)
	require.True(t, ok)
	require.NotNil(t, convertedRequest.Messages[1].ReasoningContent)
	assert.Equal(t, " ", *convertedRequest.Messages[1].ReasoningContent)

	encoded, err := common.Marshal(convertedRequest)
	require.NoError(t, err)
	assert.Equal(t, " ", gjson.GetBytes(encoded, "messages.1.reasoning_content").String())
}

func TestConvertOpenAIRequestKeepsExistingReasoningContent(t *testing.T) {
	reasoning := "inspect the weather tool result"
	message := toolCallMessage(t)
	message.ReasoningContent = &reasoning
	request := &dto.GeneralOpenAIRequest{
		Model: "deepseek-v4-pro",
		Tools: []dto.ToolCallRequest{
			{Type: "function", Function: dto.FunctionRequest{Name: "get_weather"}},
		},
		Messages: []dto.Message{message},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, newDeepSeekRelayInfo("deepseek-v4-pro"), request)
	require.NoError(t, err)

	convertedRequest, ok := converted.(*dto.GeneralOpenAIRequest)
	require.True(t, ok)
	require.NotNil(t, convertedRequest.Messages[0].ReasoningContent)
	assert.Equal(t, reasoning, *convertedRequest.Messages[0].ReasoningContent)
}

// The -none modifier turns thinking off, so no replay is required and the
// request must be left alone.
func TestConvertOpenAIRequestSkipsReasoningReplayWhenThinkingDisabledBySuffix(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model: "deepseek-v4-pro-none",
		Tools: []dto.ToolCallRequest{
			{Type: "function", Function: dto.FunctionRequest{Name: "get_weather"}},
		},
		Messages: []dto.Message{toolCallMessage(t)},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, newDeepSeekRelayInfo("deepseek-v4-pro-none"), request)
	require.NoError(t, err)

	convertedRequest, ok := converted.(*dto.GeneralOpenAIRequest)
	require.True(t, ok)
	assert.Nil(t, convertedRequest.Messages[0].ReasoningContent)
}
