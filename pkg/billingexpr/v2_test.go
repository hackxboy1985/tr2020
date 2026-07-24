package billingexpr

import (
	"testing"
)

// TestV2PerCallExpr 验证 v2: 按次计费表达式的完整链路：
// 解析版本 → 运行表达式 → quota 转换（不除 1M）
func TestV2PerCallExpr(t *testing.T) {
	const quotaPerUnit = 500000.0 // 1$ = 500000 积分

	tests := []struct {
		name        string
		expr        string
		requestBody string
		wantCost    float64
		wantTier    string
		wantVersion int
	}{
		{
			name:        "modelEdition=3 命中高价规则",
			expr:        `v2: tier("result", max(param("metadata.modelEdition") == 3.0 ? 0.15 : 0.0, max(param("metadata.modelEdition") == 2.0 ? 0.09 : 0.0, 0.06)))`,
			requestBody: `{"metadata":{"modelEdition":3}}`,
			wantCost:    0.15,
			wantTier:    "result",
			wantVersion: 2,
		},
		{
			name:        "modelEdition=2 命中中价规则",
			expr:        `v2: tier("result", max(param("metadata.modelEdition") == 3.0 ? 0.15 : 0.0, max(param("metadata.modelEdition") == 2.0 ? 0.09 : 0.0, 0.06)))`,
			requestBody: `{"metadata":{"modelEdition":2}}`,
			wantCost:    0.09,
			wantTier:    "result",
			wantVersion: 2,
		},
		{
			name:        "modelEdition=1 走兜底价",
			expr:        `v2: tier("result", max(param("metadata.modelEdition") == 3.0 ? 0.15 : 0.0, max(param("metadata.modelEdition") == 2.0 ? 0.09 : 0.0, 0.06)))`,
			requestBody: `{"metadata":{"modelEdition":1}}`,
			wantCost:    0.06,
			wantTier:    "result",
			wantVersion: 2,
		},
		{
			name:        "无 modelEdition 走兜底价",
			expr:        `v2: tier("result", max(param("metadata.modelEdition") == 3.0 ? 0.15 : 0.0, max(param("metadata.modelEdition") == 2.0 ? 0.09 : 0.0, 0.06)))`,
			requestBody: `{"metadata":{}}`,
			wantCost:    0.06,
			wantTier:    "result",
			wantVersion: 2,
		},
		{
			name:        "无规则纯兜底",
			expr:        `v2: tier("base", 0.06)`,
			requestBody: `{}`,
			wantCost:    0.06,
			wantTier:    "base",
			wantVersion: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 1. 验证版本解析
			version, _ := ParseExprVersion(tt.expr)
			if version != tt.wantVersion {
				t.Errorf("版本解析: got %d, want %d", version, tt.wantVersion)
			}

			// 2. 运行表达式
			input := RequestInput{Body: []byte(tt.requestBody)}
			cost, trace, err := RunExprWithRequest(tt.expr, TokenParams{}, input)
			if err != nil {
				t.Fatalf("RunExprWithRequest 失败: %v", err)
			}
			if cost != tt.wantCost {
				t.Errorf("表达式输出: got %f, want %f", cost, tt.wantCost)
			}
			if trace.MatchedTier != tt.wantTier {
				t.Errorf("命中档位: got %q, want %q", trace.MatchedTier, tt.wantTier)
			}

			// 3. 验证 quota 转换：v2 不除 1M
			snap := &BillingSnapshot{
				ExprVersion:  2,
				QuotaPerUnit: quotaPerUnit,
			}
			quota := quotaConversion(cost, snap)
			wantQuota := tt.wantCost * quotaPerUnit
			if quota != wantQuota {
				t.Errorf("quota 转换: got %f, want %f", quota, wantQuota)
			}

			// 4. 对比 v1 转换确认差异（v1 会除以 1M，结果应极小）
			snapV1 := &BillingSnapshot{
				ExprVersion:  1,
				QuotaPerUnit: quotaPerUnit,
			}
			quotaV1 := quotaConversion(cost, snapV1)
			if quotaV1 >= quota {
				t.Errorf("v1 quota(%f) 应远小于 v2 quota(%f)", quotaV1, quota)
			}
		})
	}
}
