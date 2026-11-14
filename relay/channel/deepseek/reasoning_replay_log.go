package deepseek

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// reasoningReplaySummary describes what the final outbound JSON sends as
// reasoning_content. It is used for the INFO diagnostic line that lets
// operators confirm DeepSeek's replay requirement is met without dumping the
// whole request body into the log.
type reasoningReplaySummary struct {
	Tools         int
	Assistant     int
	WithReasoning int
	Placeholder   int
	Empty         int
	Indexes       []int
	Preview       string
}

func summarizeOutboundReasoningReplay(body []byte) (reasoningReplaySummary, bool) {
	var summary reasoningReplaySummary
	messages := gjson.GetBytes(body, "messages")
	if !messages.IsArray() {
		return summary, false
	}
	summary.Tools = len(gjson.GetBytes(body, "tools").Array())
	for index, message := range messages.Array() {
		if message.Get("role").String() != "assistant" {
			continue
		}
		summary.Assistant++
		reasoning := message.Get("reasoning_content")
		if !reasoning.Exists() {
			continue
		}
		summary.WithReasoning++
		text := reasoning.String()
		switch {
		case text == deepSeekReasoningPlaceholder:
			summary.Placeholder++
		case strings.TrimSpace(text) == "":
			summary.Empty++
		case summary.Preview == "":
			summary.Preview = truncateReasoningPreview(text, 32)
		}
		if len(summary.Indexes) < 8 {
			summary.Indexes = append(summary.Indexes, index)
		}
	}
	return summary, true
}

func truncateReasoningPreview(text string, maxRunes int) string {
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	return string(runes[:maxRunes]) + "..."
}

// LogOutboundReasoningReplay writes one INFO line for tools-bearing DeepSeek
// requests, reporting whether reasoning_content actually made it into the final
// outbound body. It runs after channel parameter overrides, so it observes the
// exact JSON that is sent upstream. Requests without tools are skipped because
// DeepSeek does not require reasoning replay there.
func LogOutboundReasoningReplay(c *gin.Context, info *relaycommon.RelayInfo, body []byte) {
	if c == nil || info == nil || len(body) == 0 {
		return
	}
	summary, ok := summarizeOutboundReasoningReplay(body)
	if !ok || summary.Tools == 0 {
		return
	}
	channelId := 0
	if info.ChannelMeta != nil {
		channelId = info.ChannelId
	}
	indexes := "[]"
	if len(summary.Indexes) > 0 {
		parts := make([]string, 0, len(summary.Indexes))
		for _, index := range summary.Indexes {
			parts = append(parts, strconv.Itoa(index))
		}
		indexes = "[" + strings.Join(parts, " ") + "]"
	}
	if summary.WithReasoning > 0 {
		logger.LogInfo(c, fmt.Sprintf(
			"[DeepSeek] 出站请求已携带 reasoning_content：channelId=%d model=%s stream=%t tools=%d assistant=%d withReasoning=%d placeholder=%d empty=%d indexes=%s preview=%q",
			channelId, info.UpstreamModelName, info.IsStream, summary.Tools, summary.Assistant,
			summary.WithReasoning, summary.Placeholder, summary.Empty, indexes, summary.Preview))
		return
	}
	logger.LogInfo(c, fmt.Sprintf(
		"[DeepSeek] 出站请求带 tools 但未携带 reasoning_content（thinking 已关闭，或请求体透传生效）：channelId=%d model=%s stream=%t tools=%d assistant=%d，如仍报 400 请检查渠道/全局透传设置",
		channelId, info.UpstreamModelName, info.IsStream, summary.Tools, summary.Assistant))
}

// LogSkippedReasoningReplayByPassthrough reports that request body passthrough
// bypassed the replay backfill, which is the most common reason a DeepSeek
// channel still returns 400 after the fix is deployed.
func LogSkippedReasoningReplayByPassthrough(c *gin.Context, info *relaycommon.RelayInfo) {
	if c == nil || info == nil || info.ChannelMeta == nil {
		return
	}
	logger.LogInfo(c, fmt.Sprintf(
		"[DeepSeek] 请求体透传已启用，跳过 reasoning_content 检查与补全：channelId=%d model=%s，如仍报 400 请关闭渠道/全局透传",
		info.ChannelId, info.UpstreamModelName))
}
