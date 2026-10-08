package service

// 本文件锁定异步任务日志的动态计费展示契约：
// LogTaskConsumption 必须注入 billing_mode / expr_b64 / matched_tier，
// 前端 details-dialog 依据 billing_mode === "tiered_expr" 决定是否渲染
// 「命中阶梯」区块。任务路径拿不到 TieredResult，档位取自预扣费时的 snapshot。

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
		BillingMode:   "tiered_expr",
		ExprString:    expr,
		EstimatedTier: "4k+xhigh",
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
	b64, _ := other["expr_b64"].(string)
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("expr_b64 解码失败: %v", err)
	}
	if string(decoded) != expr {
		t.Fatalf("expr 不一致: %s", string(decoded))
	}
	t.Logf("前端将展示: 计费模式=动态计费, 命中阶梯=%s", other["matched_tier"])
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
