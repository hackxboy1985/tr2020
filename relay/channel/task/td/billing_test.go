package td

// 本文件锁定 Td 渠道的动态计费（tiered_expr）契约：
// InjectBillingParams 必须把用户原始的 resolution / quality 注入 BillingRequestInput，
// 否则 param() 取不到值会落到表达式兜底档位（曾导致固定扣 fallback 价）。

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/pkg/billingexpr"
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

