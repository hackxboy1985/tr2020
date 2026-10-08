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

// 档位大小写兼容：Td 约定小写（1k/2k/4k/8k），用户传大写必须被接受并归一化为小写。
// 三个出口都要是小写：task_request、计费输入、上游请求体。
// 若只放行不归一化，会出现「校验通过但 param("resolution") 匹配不上」而落到兜底档位。
func TestResolutionCaseInsensitiveNormalizedToLower(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"1k", "1k"}, {"1K", "1k"},
		{"2k", "2k"}, {"2K", "2k"},
		{"4k", "4k"}, {"4K", "4k"},
		{"8k", "8k"}, {"8K", "8k"},
		{" 4K ", "4k"}, // 首尾空白一并归一化
	}

	for _, tc := range cases {
		body := `{"model":"gpt-image-2-all","prompt":"p","size":"16:9","resolution":"` + tc.input + `","quality":"high"}`
		c := mkCtx(t, body)
		info := mkInfo()
		info.UpstreamModelName = "gpt-image-2-all"
		info.OriginModelName = "gpt-image-2-all"

		a := &TaskAdaptor{}
		a.Init(info)
		if taskErr := a.ValidateRequestAndSetAction(c, info); taskErr != nil {
			t.Fatalf("resolution=%q 被拒绝: %s", tc.input, taskErr.Message)
		}

		// ① task_request 已写回小写
		got, _ := relaycommon.GetTaskRequest(c)
		if got.Resolution != tc.want {
			t.Fatalf("resolution=%q: task_request 应为 %q, 实际 %q", tc.input, tc.want, got.Resolution)
		}

		// ② 计费表达式输入为小写，才能命中 param("resolution") 的字面量
		a.InjectBillingParams(c, info)
		if body := string(info.BillingRequestInput.Body); !strings.Contains(body, `"resolution":"`+tc.want+`"`) {
			t.Fatalf("resolution=%q: 计费输入应为 %q, 实际 %s", tc.input, tc.want, body)
		}

		// ③ 上游请求体为小写
		r, err := a.BuildRequestBody(c, info)
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		raw, err := io.ReadAll(r)
		if err != nil {
			t.Fatal(err)
		}
		if got := string(raw); !strings.Contains(got, `"resolution":"`+tc.want+`"`) {
			t.Fatalf("resolution=%q: 上游请求体应为 %q, 实际 %s", tc.input, tc.want, got)
		}
		t.Logf("resolution=%-6q → 归一化 %q", tc.input, tc.want)
	}
}

// 8k 档位（小写）通过校验并可被计费表达式识别。
func TestResolution8KAccepted(t *testing.T) {
	body := `{"model":"gpt-image-2-all","prompt":"p","size":"16:9","resolution":"8k","quality":"high"}`
	c := mkCtx(t, body)
	info := mkInfo()
	info.UpstreamModelName = "gpt-image-2-all"
	info.OriginModelName = "gpt-image-2-all"

	a := &TaskAdaptor{}
	a.Init(info)
	if taskErr := a.ValidateRequestAndSetAction(c, info); taskErr != nil {
		t.Fatalf("resolution=8k 被拒绝: %s", taskErr.Message)
	}

	// 计费表达式输入必须拿到 8k，才能按 8k 档定价
	a.InjectBillingParams(c, info)
	if got := string(info.BillingRequestInput.Body); !strings.Contains(got, `"resolution":"8k"`) {
		t.Fatalf("计费输入未注入 8k: %s", got)
	}

	// 上游请求体必须下发 8k
	r, err := a.BuildRequestBody(c, info)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	raw, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(raw); !strings.Contains(got, `"resolution":"8k"`) {
		t.Fatalf("上游请求体未包含 8k: %s", got)
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
