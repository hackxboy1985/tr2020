package service

// 本文件锁定异步任务日志的动态计费展示契约：
// LogTaskConsumption 必须注入 billing_mode / expr_b64 / matched_tier / matched_price，
// 前端 details-dialog 依据 billing_mode === "tiered_expr" 决定是否渲染
// 「命中阶梯」区块。任务路径拿不到 TieredResult，档位与价格取自预扣费时的 snapshot。
// matched_price 缺失会导致 per-call 档位只显示「按次计费」而无金额。

import (
	"encoding/base64"
	"testing"

	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func TestInjectTaskTieredBillingInfo(t *testing.T) {
	expr := `v2: param("resolution") == "4k" && param("quality") == "xhigh" ? tier("4k+xhigh", 0.25) : tier("fallback", 0.2)`

	info := &relaycommon.RelayInfo{}
	info.TieredBillingSnapshot = &billingexpr.BillingSnapshot{
		BillingMode:    "tiered_expr",
		ExprString:     expr,
		EstimatedTier:  "4k+xhigh",
		EstimatedPrice: 0.25,
	}

	other := map[string]interface{}{}
	InjectTaskTieredBillingInfo(other, info)

	t.Logf("other = %#v", other)

	if other["billing_mode"] != "tiered_expr" {
		t.Fatalf("billing_mode 缺失: %v", other["billing_mode"])
	}
	if other["matched_tier"] != "4k+xhigh" {
		t.Fatalf("matched_tier 缺失: %v", other["matched_tier"])
	}
	// matched_price 必须与命中档位一同写入，供前端展示档位金额。
	if other["matched_price"] != 0.25 {
		t.Fatalf("matched_price 缺失或错误: %v", other["matched_price"])
	}
	b64, _ := other["expr_b64"].(string)
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("expr_b64 解码失败: %v", err)
	}
	if string(decoded) != expr {
		t.Fatalf("expr 不一致: %s", string(decoded))
	}
	t.Logf("前端将展示: 计费模式=动态计费, 命中阶梯=%s, 价格=%v/次",
		other["matched_tier"], other["matched_price"])
}

// TestInjectTaskTieredBillingInfoZeroPrice 锁定「价格为 0 不写入」的守卫，
// 与同步路径 InjectTieredBillingInfo 的 `result.MatchedPrice > 0` 保持一致，
// 避免日志出现 $0 的误导性金额（免费档位只展示档位名）。
func TestInjectTaskTieredBillingInfoZeroPrice(t *testing.T) {
	info := &relaycommon.RelayInfo{}
	info.TieredBillingSnapshot = &billingexpr.BillingSnapshot{
		BillingMode:    "tiered_expr",
		ExprString:     `v2: tier("free", 0)`,
		EstimatedTier:  "free",
		EstimatedPrice: 0,
	}

	other := map[string]interface{}{}
	InjectTaskTieredBillingInfo(other, info)

	if other["matched_tier"] != "free" {
		t.Fatalf("matched_tier 应仍写入: %v", other["matched_tier"])
	}
	if _, exists := other["matched_price"]; exists {
		t.Fatalf("价格为 0 时不应写入 matched_price: %#v", other)
	}
}

func TestInjectTaskTieredBillingInfoNoop(t *testing.T) {
	// 非 tiered 计费不应写入任何字段
	info := &relaycommon.RelayInfo{}
	other := map[string]interface{}{}
	InjectTaskTieredBillingInfo(other, info)
	if len(other) != 0 {
		t.Fatalf("非 tiered 场景不应写入字段: %#v", other)
	}
}
