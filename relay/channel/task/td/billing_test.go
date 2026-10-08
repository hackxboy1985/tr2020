package td

// 本文件锁定 Td 渠道的动态计费（tiered_expr）契约：
// InjectBillingParams 必须把用户原始的 resolution / quality 注入 BillingRequestInput，
// 否则 param() 取不到值会落到表达式兜底档位（曾导致固定扣 fallback 价）。

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

const userExpr = `v2: param("resolution") == "2k" && param("quality") == "high" ? tier("resolution == 2k && quality == high", 0.1) : param("resolution") == "4k" && param("quality") == "high" ? tier("resolution == 4k && quality == high", 0.21) : param("resolution") == "2k" && param("quality") == "xhigh" ? tier("resolution == 2k && quality == xhigh", 0.2) : param("resolution") == "4k" && param("quality") == "xhigh" ? tier("resolution == 4k && quality == xhigh", 0.25) : param("resolution") == "2k" && param("quality") == "max" ? tier("resolution == 2k && quality == max", 0.25) : param("resolution") == "4k" && param("quality") == "max" ? tier("resolution == 4k && quality == max", 0.32) : tier("fallback", 0.2)`

func TestInjectBillingParamsHitsCorrectTier(t *testing.T) {
	cases := []struct {
		res, quality string
		wantCost     float64
		wantTier     string
	}{
		{"4k", "high", 0.21, "resolution == 4k && quality == high"},
		{"2k", "high", 0.10, "resolution == 2k && quality == high"},
		{"4k", "xhigh", 0.25, "resolution == 4k && quality == xhigh"},
		{"2k", "xhigh", 0.20, "resolution == 2k && quality == xhigh"},
		{"4k", "max", 0.32, "resolution == 4k && quality == max"},
		{"2k", "max", 0.25, "resolution == 2k && quality == max"},
	}

	for _, tc := range cases {
		body := `{"model":"gpt-image-2.5-sunburst","prompt":"p","size":"16:9","resolution":"` + tc.res + `","quality":"` + tc.quality + `"}`
		c := mkCtx(t, body)
		info := mkInfo()
		a := &TaskAdaptor{}
		a.Init(info)

		if taskErr := a.ValidateRequestAndSetAction(c, info); taskErr != nil {
			t.Fatalf("validate: %s", taskErr.Message)
		}

		// 模拟 relay_task.go 的调用顺序
		a.InjectBillingParams(c, info)

		cost, trace, err := billingexpr.RunExprWithRequest(userExpr, billingexpr.TokenParams{}, *info.BillingRequestInput)
		if err != nil {
			t.Fatalf("run: %v", err)
		}
		t.Logf("resolution=%-4s quality=%-6s → cost=%.4f tier=%q body=%s",
			tc.res, tc.quality, cost, trace.MatchedTier, string(info.BillingRequestInput.Body))

		if trace.MatchedTier != tc.wantTier {
			t.Fatalf("档位不匹配: got %q want %q", trace.MatchedTier, tc.wantTier)
		}
		if cost != tc.wantCost {
			t.Fatalf("价格不匹配: got %.4f want %.4f", cost, tc.wantCost)
		}
	}
}

func TestInjectBillingParamsPreservesRawQuality(t *testing.T) {
	// xhigh 必须原样注入，不能被 normalize 成 high
	body := `{"model":"gpt-image-2-all","prompt":"p","size":"16:9","resolution":"4k","quality":"xhigh"}`
	c := mkCtx(t, body)
	info := mkInfo()
	a := &TaskAdaptor{}
	a.Init(info)
	if taskErr := a.ValidateRequestAndSetAction(c, info); taskErr != nil {
		t.Fatal(taskErr.Message)
	}
	a.InjectBillingParams(c, info)
	got := string(info.BillingRequestInput.Body)
	t.Logf("注入后 body: %s", got)
	if !contains(got, `"quality":"xhigh"`) {
		t.Fatalf("quality 应保留原始值 xhigh, 实际: %s", got)
	}
	if !contains(got, `"resolution":"4k"`) {
		t.Fatalf("resolution 应为 4k, 实际: %s", got)
	}
}

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}


// TestBothEntryPathsConverge 验证两个入口的计费输入一致。
//
// 两条路径最终都经由 RelayTaskSubmit → ValidateRequestAndSetAction（用原始 body
// 重新解析 task_request）→ InjectBillingParams，因此计费输入只取决于客户端原始
// body，与入口无关：
//   /v1/images/generations  handler 预置的 task_request 会被 Validate 覆盖
//   /v1/images/tasks        无 handler 预处理，Validate 直接解析
func TestBothEntryPathsConverge(t *testing.T) {
	rawBody := `{"model":"gpt-image-2.5-sunburst","prompt":"p","size":"16:9","resolution":"4k","quality":"high"}`

	run := func(name string, preSet bool) float64 {
		c := mkCtx(t, rawBody)
		info := mkInfo()
		a := &TaskAdaptor{}
		a.Init(info)

		if preSet {
			// 模拟 handleTudouImageTask 的预置（随后会被 Validate 覆盖）
			c.Set("task_request", relaycommon.TaskSubmitReq{
				Prompt: "p", Size: "16:9", Resolution: "4k", Quality: "high",
			})
		}
		if e := a.ValidateRequestAndSetAction(c, info); e != nil {
			t.Fatalf("%s validate: %s", name, e.Message)
		}
		a.InjectBillingParams(c, info)

		cost, trace, err := billingexpr.RunExprWithRequest(userExpr, billingexpr.TokenParams{}, *info.BillingRequestInput)
		if err != nil {
			t.Fatalf("%s run: %v", name, err)
		}
		t.Logf("%s → cost=%.4f tier=%q body=%s", name, cost, trace.MatchedTier, string(info.BillingRequestInput.Body))
		return cost
	}

	costA := run("路径A /v1/images/generations", true)
	costB := run("路径B /v1/images/tasks", false)

	if costA != costB {
		t.Fatalf("两个入口计费结果不一致: A=%.4f B=%.4f", costA, costB)
	}
	if costA != 0.21 {
		t.Fatalf("应命中 4k+high = 0.21, 实际 %.4f", costA)
	}
}

// TestInjectBillingParamsDefaults 缺省值与发往上游的默认值保持一致
func TestInjectBillingParamsDefaults(t *testing.T) {
	body := `{"model":"gpt-image-2-all","prompt":"p"}`
	c := mkCtx(t, body)
	info := mkInfo()
	a := &TaskAdaptor{}
	a.Init(info)
	if e := a.ValidateRequestAndSetAction(c, info); e != nil {
		t.Fatal(e.Message)
	}
	a.InjectBillingParams(c, info)
	got := string(info.BillingRequestInput.Body)
	t.Logf("缺省注入: %s", got)
	// 与 BuildRequestBody 的默认值一致：resolution=1k, quality=medium
	if !strings.Contains(got, `"resolution":"1k"`) {
		t.Fatalf("resolution 缺省应为 1k: %s", got)
	}
	if !strings.Contains(got, `"quality":"medium"`) {
		t.Fatalf("quality 缺省应为 medium: %s", got)
	}
}
