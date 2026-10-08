package td

// 本文件锁定 Td 渠道的 quality 契约：
// 历史上 isValidQuality 用白名单（low/medium/high）在本地拒绝未知档位，
// 导致上游已支持的档位（如 xhigh）被 400 拦截。
// 现在 quality 不做本地校验，原样交给上游判断。

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
)

func mkCtx(t *testing.T, body string) *gin.Context {
	t.Helper()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/tasks", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	storage, err := common.CreateBodyStorage([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = storage.Close() })
	c.Set(common.KeyBodyStorage, storage)
	return c
}

func mkInfo() *relaycommon.RelayInfo {
	info := &relaycommon.RelayInfo{}
	info.ChannelMeta = &relaycommon.ChannelMeta{
		ChannelType:    constant.ChannelTypeTudou,
		ChannelBaseUrl: "https://up.example.com",
		ApiKey:         "sk-test",
	}
	info.TaskRelayInfo = &relaycommon.TaskRelayInfo{}
	return info
}

// 用户真实请求：quality=xhigh 现在必须通过校验
func TestQualityNoLongerRejected(t *testing.T) {
	for _, q := range []string{"xhigh", "ultra", "high", "medium", "standard", "hd", ""} {
		body := `{"model":"gpt-image-2-all","prompt":"test","size":"16:9","resolution":"4k","quality":"` + q + `"}`
		c := mkCtx(t, body)
		info := mkInfo()
		a := &TaskAdaptor{}
		a.Init(info)
		taskErr := a.ValidateRequestAndSetAction(c, info)
		if taskErr != nil {
			t.Fatalf("quality=%q 仍被拒绝: %s", q, taskErr.Message)
		}
		t.Logf("quality=%-9q → PASS", q)
	}
}

// xhigh 必须原样透传给上游，不能被改写或丢弃
func TestQualityPassthroughToUpstream(t *testing.T) {
	body := `{"size":"16:9","model":"gpt-image-2-all","prompt":"p","resolution":"4k","quality":"xhigh"}`
	c := mkCtx(t, body)
	info := mkInfo()
	info.UpstreamModelName = "gpt-image-2-all"
	info.OriginModelName = "gpt-image-2-all"

	a := &TaskAdaptor{}
	a.Init(info)
	if taskErr := a.ValidateRequestAndSetAction(c, info); taskErr != nil {
		t.Fatalf("validate: %s", taskErr.Message)
	}
	r, err := a.BuildRequestBody(c, info)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	raw, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)
	t.Logf("上游请求体: %s", got)
	if !strings.Contains(got, `"quality":"xhigh"`) {
		t.Fatalf("xhigh 未原样透传: %s", got)
	}
}

// 去掉白名单校验的同时，OpenAI 兼容的 standard/hd 必须仍被翻译成上游格式，
// 未知档位才原样透传。这两件事不能一起丢掉。
func TestQualityNormalizationStillApplied(t *testing.T) {
	cases := map[string]string{
		"standard": "medium", // OpenAI 风格 -> 上游档位
		"hd":       "high",
		"high":     "high",
		"medium":   "medium",
		"low":      "low",
		"":         "medium", // 默认值
		"xhigh":    "xhigh",  // 未知值原样透传
	}
	for in, want := range cases {
		if got := normalizeQuality(in); got != want {
			t.Fatalf("normalizeQuality(%q) = %q, want %q", in, got, want)
		}
	}
}
