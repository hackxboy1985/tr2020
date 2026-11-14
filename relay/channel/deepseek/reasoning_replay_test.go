package deepseek

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func deepSeekTools() []dto.ToolCallRequest {
	return []dto.ToolCallRequest{
		{Type: "function", Function: dto.FunctionRequest{Name: "get_weather"}},
	}
}

func rawJSON(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := common.Marshal(value)
	require.NoError(t, err)
	return encoded
}

// A tools-bearing thinking-mode request replayed after a tool call is the
// shape that DeepSeek rejects with:
//
//	400 The reasoning_content in the thinking mode must be passed back to the API.
//
// Clients rebuild this history without reasoning_content, so the DeepSeek
// channel has to restore the field before the request leaves for the upstream.
func TestEnsureThinkingReasoningReplayBackfillsAssistantTurns(t *testing.T) {
	toolCalls := rawJSON(t, []dto.ToolCallRequest{
		{ID: "call_1", Type: "function", Function: dto.FunctionRequest{Name: "get_weather", Arguments: "{}"}},
	})
	request := &dto.GeneralOpenAIRequest{
		Model: "deepseek-v4-pro",
		Tools: deepSeekTools(),
		Messages: []dto.Message{
			{Role: "user", Content: "What is the weather?"},
			{Role: "assistant", ToolCalls: toolCalls},
			{Role: "tool", Content: "sunny", ToolCallId: "call_1"},
		},
	}

	ensureThinkingReasoningReplay(request)

	require.NotNil(t, request.Messages[1].ReasoningContent)
	assert.Equal(t, deepSeekReasoningPlaceholder, *request.Messages[1].ReasoningContent)
	assert.Nil(t, request.Messages[0].ReasoningContent, "user messages must not be rewritten")
	assert.Nil(t, request.Messages[2].ReasoningContent, "tool messages must not be rewritten")
}

// DeepSeek requires reasoning_content on every historical assistant turn of a
// tools-bearing request, including turns that did not call a tool.
func TestEnsureThinkingReasoningReplayBackfillsTextOnlyAssistantTurns(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model: "deepseek-v4-pro",
		Tools: deepSeekTools(),
		Messages: []dto.Message{
			{Role: "user", Content: "hi"},
			{Role: "assistant", Content: "hello"},
		},
	}

	ensureThinkingReasoningReplay(request)

	require.NotNil(t, request.Messages[1].ReasoningContent)
	assert.Equal(t, " ", *request.Messages[1].ReasoningContent)
}

func TestEnsureThinkingReasoningReplayKeepsExistingReasoning(t *testing.T) {
	reasoning := "Read the weather API response first."
	request := &dto.GeneralOpenAIRequest{
		Model: "deepseek-v4-pro",
		Tools: deepSeekTools(),
		Messages: []dto.Message{
			{Role: "assistant", Content: "checking", ReasoningContent: &reasoning},
		},
	}

	ensureThinkingReasoningReplay(request)

	require.NotNil(t, request.Messages[0].ReasoningContent)
	assert.Equal(t, reasoning, *request.Messages[0].ReasoningContent)
}

// DeepSeek V4+ rejects reasoning_content:"" as well, so an explicitly empty
// value has to be replaced instead of preserved.
func TestEnsureThinkingReasoningReplayReplacesEmptyReasoning(t *testing.T) {
	empty := ""
	request := &dto.GeneralOpenAIRequest{
		Model: "deepseek-v4-pro",
		Tools: deepSeekTools(),
		Messages: []dto.Message{
			{Role: "assistant", Content: "checking", ReasoningContent: &empty},
		},
	}

	ensureThinkingReasoningReplay(request)

	require.NotNil(t, request.Messages[0].ReasoningContent)
	assert.Equal(t, " ", *request.Messages[0].ReasoningContent)
}

// Some OpenAI-compatible clients (OpenRouter convention) put real reasoning in
// reasoning. Promoting it keeps the model's own text instead of a placeholder.
func TestEnsureThinkingReasoningReplayPromotesReasoningField(t *testing.T) {
	reasoning := "The user wants the forecast for Paris."
	request := &dto.GeneralOpenAIRequest{
		Model: "deepseek-v4-pro",
		Tools: deepSeekTools(),
		Messages: []dto.Message{
			{Role: "assistant", Content: "checking", Reasoning: &reasoning},
		},
	}

	ensureThinkingReasoningReplay(request)

	require.NotNil(t, request.Messages[0].ReasoningContent)
	assert.Equal(t, reasoning, *request.Messages[0].ReasoningContent)
}

func TestEnsureThinkingReasoningReplaySkipsWithoutTools(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model: "deepseek-v4-pro",
		Messages: []dto.Message{
			{Role: "assistant", Content: "hello"},
		},
	}

	ensureThinkingReasoningReplay(request)

	assert.Nil(t, request.Messages[0].ReasoningContent)
}

func TestEnsureThinkingReasoningReplaySkipsDisabledThinking(t *testing.T) {
	tests := []struct {
		name  string
		model string
		setup func(*dto.GeneralOpenAIRequest)
	}{
		{
			name:  "thinking disabled",
			model: "deepseek-v4-pro",
			setup: func(request *dto.GeneralOpenAIRequest) {
				request.THINKING = rawJSON(t, map[string]string{"type": "disabled"})
			},
		},
		{
			name:  "reasoning effort none",
			model: "deepseek-v4-pro",
			setup: func(request *dto.GeneralOpenAIRequest) {
				request.ReasoningEffort = "none"
			},
		},
		{
			name:  "thinking object disabled wins over effort",
			model: "deepseek-v4-pro",
			setup: func(request *dto.GeneralOpenAIRequest) {
				request.THINKING = rawJSON(t, map[string]string{"type": "disabled"})
				request.ReasoningEffort = "high"
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := &dto.GeneralOpenAIRequest{
				Model: tt.model,
				Tools: deepSeekTools(),
				Messages: []dto.Message{
					{Role: "assistant", Content: "hello"},
				},
			}
			tt.setup(request)

			ensureThinkingReasoningReplay(request)

			assert.Nil(t, request.Messages[0].ReasoningContent)
		})
	}
}

// The DeepSeek channel treats every model as thinking-capable by default, so
// model names this code has never seen (a future DeepSeek release, or a
// channel-side model alias) are still covered without code changes.
func TestEnsureThinkingReasoningReplayCoversUnknownModelNames(t *testing.T) {
	for _, model := range []string{"deepseek-v5-pro", "deepseek-flash-2027", "my-private-deepseek"} {
		t.Run(model, func(t *testing.T) {
			request := &dto.GeneralOpenAIRequest{
				Model: model,
				Tools: deepSeekTools(),
				Messages: []dto.Message{
					{Role: "assistant", Content: "hello"},
				},
			}

			ensureThinkingReasoningReplay(request)

			require.NotNil(t, request.Messages[0].ReasoningContent)
			assert.Equal(t, " ", *request.Messages[0].ReasoningContent)
		})
	}
}

// An explicit thinking object is still honoured; it only matters as a guard
// against future changes to the default.
func TestEnsureThinkingReasoningReplayExplicitThinking(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model: "my-private-deepseek",
		Tools: deepSeekTools(),
		Messages: []dto.Message{
			{Role: "assistant", Content: "hello"},
		},
	}
	request.THINKING = rawJSON(t, map[string]string{"type": "enabled"})

	ensureThinkingReasoningReplay(request)

	require.NotNil(t, request.Messages[0].ReasoningContent)
	assert.Equal(t, " ", *request.Messages[0].ReasoningContent)
}

// DeepSeek documents thinking.type as taking precedence over reasoning_effort,
// so an enabled object must win over an effort=none fallback.
func TestEnsureThinkingReasoningReplayThinkingTypeWinsOverEffort(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model:           "deepseek-v4-pro",
		Tools:           deepSeekTools(),
		ReasoningEffort: "none",
		Messages: []dto.Message{
			{Role: "assistant", Content: "hello"},
		},
	}
	request.THINKING = rawJSON(t, map[string]string{"type": "enabled"})

	ensureThinkingReasoningReplay(request)

	require.NotNil(t, request.Messages[0].ReasoningContent)
	assert.Equal(t, " ", *request.Messages[0].ReasoningContent)
}

func TestEnsureThinkingReasoningReplayIsIdempotent(t *testing.T) {
	reasoning := "kept"
	request := &dto.GeneralOpenAIRequest{
		Model: "deepseek-v4-pro",
		Tools: deepSeekTools(),
		Messages: []dto.Message{
			{Role: "assistant", Content: "a", ReasoningContent: &reasoning},
			{Role: "assistant", Content: "b"},
		},
	}

	ensureThinkingReasoningReplay(request)
	first := *request.Messages[1].ReasoningContent
	ensureThinkingReasoningReplay(request)

	assert.Equal(t, reasoning, *request.Messages[0].ReasoningContent)
	assert.Equal(t, first, *request.Messages[1].ReasoningContent)
	assert.Equal(t, " ", first)
}

func TestEnsureThinkingReasoningReplayNilRequest(t *testing.T) {
	assert.NotPanics(t, func() { ensureThinkingReasoningReplay(nil) })
}
