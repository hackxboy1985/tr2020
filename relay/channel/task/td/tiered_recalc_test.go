package td

// 本文件锁定 Td 视频任务的「按上游实际秒数重算」契约。
//
// 背景：表达式 v2: tier("base", 0.06 * max(isnull(param("duration"),0),1))
// 在提交阶段只能拿到**用户请求**的 duration，预扣的是请求时长对应的额度；
// 上游会按自身规则收敛时长（请求 8 秒、实际出片 2 秒），
// 若不在完成时重算就会多收用户的钱。
//
// 关键不变式：
//  1. 重算 = 用新 duration **重跑表达式**，绝不是在预扣额上乘倍率
//     （表达式已含 duration，乘倍率会算两遍）；
//  2. 上游没给 seconds 时，必须保持预扣额不变 —— 拿不到实际值就不乱动钱；
//  3. 非表达式计费、非 Td 的任务一律不参与重算。

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

// 用户诉求的表达式：0.06 $/秒，兜底至少 1 秒
const perSecondExpr = `v2: tier("base", 0.06 * max(isnull(param("duration"), 0.0), 1))`

// QuotaPerUnit 的测试值（与项目一致：1 元 = 500000 积分）
const testQuotaPerUnit = 500000.0

// mkTieredTask 构造一个使用表达式计费的 Td 视频任务。
// requestBody 为提交时下发给上游的报文（含 resolution 等，供重算时保持一致）。
func mkTieredTask(requestBody string, preConsumedQuota int) *model.Task {
	return &model.Task{
		TaskID: "task_test_recalc",
		Quota:  preConsumedQuota,
		PrivateData: model.TaskPrivateData{
			RequestBody: requestBody,
			BillingContext: &model.TaskBillingContext{
				OriginModelName:    "minimax-h3",
				GroupRatio:         1.0,
				PerCallBilling:     true,
				TieredExprString:   perSecondExpr,
				TieredExprVersion:  2,
				TieredQuotaPerUnit: testQuotaPerUnit,
			},
		},
	}
}

// upstreamCompletedBody 构造上游查询响应，seconds 为 nil 时不写该字段。
func upstreamCompletedBody(seconds any) []byte {
	data := map[string]any{
		"status":   "completed",
		"progress": 100,
		"result": map[string]any{
			"video_url": "https://example.com/v.mp4",
		},
	}
	if seconds != nil {
		data["seconds"] = seconds
	}
	body, _ := json.Marshal(map[string]any{"code": 200, "data": data})
	return body
}

// upstreamInfo 走真实解析链路构造 TaskInfo：
// 上游响应 → parseVideoTaskResult → TaskInfo.Seconds。
//
// 这样测试覆盖的是生产代码实际经过的路径；若绕过解析直接塞 Seconds，
// 就无法发现「DTO 字段名写错」「类型不兼容导致整段解析失败」这类真实缺陷。
func upstreamInfo(seconds int) *relaycommon.TaskInfo {
	var body []byte
	if seconds > 0 {
		body = upstreamCompletedBody(seconds)
	} else {
		body = upstreamCompletedBody(nil)
	}
	info, err := parseVideoTaskResult(body)
	if err != nil {
		panic(err)
	}
	return info
}

// TestRecalcUsesActualSeconds 上游实际 2 秒、预扣按请求 8 秒计 —— 必须重算为 2 秒的价格。
//
// 这是核心用例：0.06 × 8 = 0.48（预扣 240000 积分），
// 实际 0.06 × 2 = 0.12（应扣 60000 积分）。
// 若错误地把 seconds 当倍率相乘，会得到 480000 —— 本用例专门防这个回归。
func TestRecalcUsesActualSeconds(t *testing.T) {
	// 提交时下发的报文：请求 8 秒
	task := mkTieredTask(`{"model":"minimax-h3","prompt":"p","duration":8,"resolution":"720p"}`, 240000)

	quota := (&TaskAdaptor{}).AdjustBillingOnComplete(task, upstreamInfo(2))
	t.Logf("预扣=%d 重算后应扣=%d", task.Quota, quota)

	if quota != 60000 {
		t.Fatalf("应扣额度错误：got %d, want 60000 (0.06 × 2 × 500000)", quota)
	}
	if quota == 480000 {
		t.Fatal("检测到 duration 被计算两次：说明走了「乘倍率」而非「重跑表达式」")
	}
}

// TestRecalcUpstreamLongerThanRequest 上游实际比请求长时应补扣。
func TestRecalcUpstreamLongerThanRequest(t *testing.T) {
	task := mkTieredTask(`{"model":"minimax-h3","prompt":"p","duration":3}`, 90000)

	quota := (&TaskAdaptor{}).AdjustBillingOnComplete(task, upstreamInfo(10))
	if quota != 300000 {
		t.Fatalf("应扣额度错误：got %d, want 300000 (0.06 × 10 × 500000)", quota)
	}
}

// TestNoSecondsKeepsPreConsumed 上游未返回 seconds —— 必须放弃重算、保持预扣额。
//
// 这是刻意的保守策略：拿不到可信的实际时长时不猜测、不乱动用户的钱。
func TestNoSecondsKeepsPreConsumed(t *testing.T) {
	task := mkTieredTask(`{"model":"minimax-h3","prompt":"p","duration":8}`, 240000)

	quota := (&TaskAdaptor{}).AdjustBillingOnComplete(task, upstreamInfo(0))
	if quota != 0 {
		t.Fatalf("无 seconds 时应返回 0（保持预扣），got %d", quota)
	}
}

// TestNonTieredTaskNotRecalculated 非表达式计费的任务不参与重算。
//
// 固定价按次模型（如 $0.1/次）重算也是同一个价，且它们没有冻结表达式，
// 必须完全不受本功能影响。
func TestNonTieredTaskNotRecalculated(t *testing.T) {
	task := mkTieredTask(`{"duration":8}`, 240000)
	// 抹掉表达式快照 —— 模拟固定价按次计费任务
	task.PrivateData.BillingContext.TieredExprString = ""
	task.PrivateData.BillingContext.TieredQuotaPerUnit = 0

	quota := (&TaskAdaptor{}).AdjustBillingOnComplete(task, upstreamInfo(2))
	if quota != 0 {
		t.Fatalf("非表达式计费任务不应重算，got %d", quota)
	}
}

// TestNilBillingContextNotRecalculated 没有 BillingContext 的任务不应 panic。
func TestNilBillingContextNotRecalculated(t *testing.T) {
	task := &model.Task{TaskID: "task_no_bc", Quota: 100}

	quota := (&TaskAdaptor{}).AdjustBillingOnComplete(task, upstreamInfo(2))
	if quota != 0 {
		t.Fatalf("无 BillingContext 时应返回 0，got %d", quota)
	}
}

// TestSecondsTypeVariants 上游 seconds 类型不稳定，数字/字符串/小数都要能解析，
// 且不能因类型不兼容而拖垮整段响应解析（状态与结果地址必须照常取到）。
func TestSecondsTypeVariants(t *testing.T) {
	cases := []struct {
		name         string
		secondsValue any
		wantSeconds  int
		wantQuota    int
	}{
		{"数字", float64(5), 5, 150000},  // 0.06 × 5 × 500000
		{"字符串", "5", 5, 150000},        // 上游可能给字符串
		{"小数字符串", "5.0", 5, 150000},    // 同上
		{"小数", float64(2.5), 2, 60000}, // 向下取整为 2 秒
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			info, err := parseVideoTaskResult(upstreamCompletedBody(tc.secondsValue))
			if err != nil {
				t.Fatalf("解析上游响应失败（类型不兼容会拖垮整段解析）：%v", err)
			}
			// 状态与结果地址不受 seconds 类型影响
			if info.Status != "SUCCESS" {
				t.Fatalf("状态解析错误：got %q", info.Status)
			}
			if info.Url == "" {
				t.Fatal("结果地址丢失")
			}
			if info.Seconds != tc.wantSeconds {
				t.Fatalf("seconds=%v 解析为 %d 秒, want %d", tc.secondsValue, info.Seconds, tc.wantSeconds)
			}

			task := mkTieredTask(`{"duration":8}`, 240000)
			quota := (&TaskAdaptor{}).AdjustBillingOnComplete(task, info)
			if quota != tc.wantQuota {
				t.Fatalf("seconds=%v 应扣=%d, want %d", tc.secondsValue, quota, tc.wantQuota)
			}
		})
	}
}

// TestUnreasonableSecondsRejected 异常大的秒数判定为不可信，保持预扣额。
//
// 防的是上游字段语义变更或脏数据导致的巨额扣费：宁可少收也不错收。
func TestUnreasonableSecondsRejected(t *testing.T) {
	task := mkTieredTask(`{"duration":8}`, 240000)

	quota := (&TaskAdaptor{}).AdjustBillingOnComplete(task, upstreamInfo(999999))
	if quota != 0 {
		t.Fatalf("异常秒数应拒绝重算，got %d", quota)
	}
}

// TestMissingRequestBodyFallsBackToDurationOnly 请求体缺失/损坏时仍能按秒数重算。
//
// 表达式对缺失字段有 isnull 兜底，故退化路径仍须得出正确价格。
func TestMissingRequestBodyFallsBackToDurationOnly(t *testing.T) {
	task := mkTieredTask("", 240000)

	quota := (&TaskAdaptor{}).AdjustBillingOnComplete(task, upstreamInfo(3))
	if quota != 90000 {
		t.Fatalf("请求体缺失时应扣=%d, want 90000", quota)
	}
}

// TestGroupRatioAppliedOnRecalc 分组倍率必须在重算时同样生效（与预扣口径一致）。
func TestGroupRatioAppliedOnRecalc(t *testing.T) {
	task := mkTieredTask(`{"duration":8}`, 240000)
	task.PrivateData.BillingContext.GroupRatio = 2.0

	quota := (&TaskAdaptor{}).AdjustBillingOnComplete(task, upstreamInfo(2))
	// 0.06 × 2 × 500000 × 2.0 = 120000
	if quota != 120000 {
		t.Fatalf("分组倍率未生效：got %d, want 120000", quota)
	}
}

// TestCoerceSeconds 直接锁定秒数归一化函数对各种输入形态的行为。
//
// 上游对 seconds 的类型不稳定，这里把已观测到的形态与边界一次性钉死：
// 非正值、不可解析值一律视为「拿不到实际时长」，由调用方保持预扣额不变。
func TestCoerceSeconds(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want int
		ok   bool
	}{
		{"整数数字", float64(2), 2, true},
		{"小数字符串", "2", 2, true},
		{"带小数点字符串", "2.5", 2, true},
		{"int", int(7), 7, true},
		{"int64", int64(7), 7, true},
		{"nil", nil, 0, false},
		{"零", float64(0), 0, false},
		{"负数", float64(-3), 0, false},
		{"空字符串", "", 0, false},
		{"非数字字符串", "abc", 0, false},
		{"无法识别的类型", []any{1}, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := coerceSeconds(tc.in)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("coerceSeconds(%#v) = (%d,%v), want (%d,%v)", tc.in, got, ok, tc.want, tc.ok)
			}
		})
	}
}

// TestSecondsAbsentYieldsZero 上游响应完全不含 seconds 时，TaskInfo.Seconds 必须为 0。
//
// 0 是「拿不到实际时长」的哨兵值，调用方据此放弃重算。
func TestSecondsAbsentYieldsZero(t *testing.T) {
	info, err := parseVideoTaskResult(upstreamCompletedBody(nil))
	if err != nil {
		t.Fatalf("解析失败：%v", err)
	}
	if info.Seconds != 0 {
		t.Fatalf("无 seconds 时应为 0，got %d", info.Seconds)
	}
	// 状态与地址仍须正常解析
	if info.Status != "SUCCESS" || info.Url == "" {
		t.Fatalf("状态/地址解析异常：status=%q url=%q", info.Status, info.Url)
	}
}

// TestHasTieredExpr 放行判据本身的行为。
func TestHasTieredExpr(t *testing.T) {
	var nilBC *model.TaskBillingContext
	if nilBC.HasTieredExpr() {
		t.Fatal("nil 应返回 false")
	}
	if (&model.TaskBillingContext{TieredExprString: "v2: tier(\"a\",1)"}).HasTieredExpr() {
		t.Fatal("缺少 QuotaPerUnit 应返回 false（避免用 0 换算导致额度归零）")
	}
	if !(&model.TaskBillingContext{
		TieredExprString:   "v2: tier(\"a\",1)",
		TieredQuotaPerUnit: common.QuotaPerUnit,
	}).HasTieredExpr() {
		t.Fatal("完整快照应返回 true")
	}
}
