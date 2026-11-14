package deepseek

import (
	"strings"

	"github.com/QuantumNous/new-api/dto"
	"github.com/tidwall/gjson"
)

// deepSeekReasoningPlaceholder is the smallest non-empty value DeepSeek
// accepts when replaying reasoning_content for a historical assistant turn.
// An empty string is rejected by DeepSeek V4+ with:
//
//	400 The reasoning_content in the thinking mode must be passed back to the API.
const deepSeekReasoningPlaceholder = " "

// ensureThinkingReasoningReplay backfills reasoning_content on the historical
// assistant messages of a tools-bearing thinking-mode request.
//
// DeepSeek requires that, once a request carries tools, every historical
// assistant message passes back the reasoning_content it produced; a missing
// field makes the upstream reject the whole request with HTTP 400. Agents and
// context layers (Cursor, Claude Code, ...) routinely drop this non-standard
// field while rebuilding history, so the DeepSeek channel fills the smallest
// placeholder DeepSeek accepts instead of failing the turn.
//
// Existing reasoning is never overwritten: a non-blank reasoning_content is
// kept byte for byte, and a non-blank OpenRouter-style reasoning value is
// promoted to reasoning_content because it carries real model text.
//
// https://api-docs.deepseek.com/guides/thinking_mode
func ensureThinkingReasoningReplay(request *dto.GeneralOpenAIRequest) {
	if request == nil || len(request.Tools) == 0 || !deepSeekThinkingActive(request) {
		return
	}
	for i := range request.Messages {
		message := &request.Messages[i]
		if message.Role != "assistant" {
			continue
		}
		if reasoningContentSet(message.ReasoningContent) {
			continue
		}
		if message.Reasoning != nil && strings.TrimSpace(*message.Reasoning) != "" {
			reasoning := *message.Reasoning
			message.ReasoningContent = &reasoning
			continue
		}
		placeholder := deepSeekReasoningPlaceholder
		message.ReasoningContent = &placeholder
	}
}

func reasoningContentSet(value *string) bool {
	return value != nil && strings.TrimSpace(*value) != ""
}

// deepSeekThinkingActive reports whether the effective request runs DeepSeek
// thinking mode.
//
// DeepSeek models think by default, and this adapter is only selected for the
// DeepSeek channel type, so the channel itself is the scope boundary: every
// model (including future model names this code cannot know about) is treated
// as thinking mode unless the request explicitly opts out via
// thinking.type=disabled, reasoning_effort=none or the -none model suffix.
// A model-name allowlist would silently stop covering new DeepSeek releases.
func deepSeekThinkingActive(request *dto.GeneralOpenAIRequest) bool {
	// DeepSeek gives thinking.type precedence over reasoning_effort, so an
	// explicit thinking object must be honoured before the effort fallback.
	if len(request.THINKING) > 0 {
		switch strings.ToLower(strings.TrimSpace(gjson.GetBytes(request.THINKING, "type").String())) {
		case "disabled":
			return false
		case "enabled":
			return true
		}
	}
	return !strings.EqualFold(strings.TrimSpace(request.ReasoningEffort), "none")
}
