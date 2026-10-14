package td

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

// ============================
// Td 视频任务：按上游实际秒数重算表达式计费
// ============================
//
// 背景
// ----
// Td 视频（minimax-h3 / seedance 等）支持按秒计费，典型表达式：
//
//	v2: tier("base", 0.06 * max(isnull(param("duration"), 0.0), 1))
//
// 提交阶段只能拿到**用户请求的 duration**，因此预扣的是「请求时长」对应的额度。
// 而 minimax-h3 这类上游会按自身规则收敛时长（请求 8 秒，实际出片可能只有 2 秒），
// 若不做结算就会按 8 秒收费 —— 属于多收。
//
// 为什么重算必须「重跑表达式」而不是「乘倍率」
// ------------------------------------------
// 表达式的输出是**已经包含 duration 的价格**（0.06 × duration）。
// 系统既有的 recalcQuotaFromRatios 是「基础额度 × 倍率」模型，
// 若把 seconds 塞进 OtherRatios 相乘，会得到 0.06 × 8 × 2 —— duration 被算两遍。
// 唯一正确的做法是把新的 duration 注入计费输入后，用同一表达式重新求值，
// 这也符合 expr.md 的「One expression, one truth」：表达式是唯一真相源。
//
// 结算范围
// --------
// 仅 Td 视频任务、且任务冻结了表达式快照（TaskBillingContext.HasTieredExpr）时才重算。
// 其他渠道、其他任务类型完全不受影响：它们的 BillingContext 不含表达式，
// HasTieredExpr() 为 false，直接跳过。

// maxSecondsSanity 实际秒数的合理上限。
//
// 为什么需要：上游若返回异常大的秒数（字段语义变更、脏数据），
// 直接重算会造成巨额扣费。超过上限时判定为不可信，放弃重算并保持预扣额，
// 宁可少收也不错收。
const maxSecondsSanity = 3600

// coerceSeconds 把 JSON 中可能出现的多种数值形态统一成整数秒。
//
// 上游对 seconds 的类型不稳定：可能是数字（2 / 2.0），也可能是字符串（"2"），
// 还见过小数（如 2.5，按秒计费需向下取整到实际可计费秒数）。
// 无法解析、或值非正时返回 ok=false。
func coerceSeconds(v any) (int, bool) {
	switch t := v.(type) {
	case nil:
		return 0, false
	case float64:
		if t <= 0 {
			return 0, false
		}
		return int(t), true
	case float32:
		if t <= 0 {
			return 0, false
		}
		return int(t), true
	case int:
		if t <= 0 {
			return 0, false
		}
		return t, true
	case int64:
		if t <= 0 {
			return 0, false
		}
		return int(t), true
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return 0, false
		}
		// 兼容 "2" 与 "2.5"：按 float 解析后再取整，避免直接 Atoi 失败
		f, err := strconv.ParseFloat(s, 64)
		if err != nil || f <= 0 {
			return 0, false
		}
		return int(f), true
	default:
		return 0, false
	}
}

// recalcQuotaByActualSeconds 用上游实际秒数重跑表达式，返回应扣额度。
//
// 返回 ok=false 表示「不重算」：调用方应保持预扣额度不变。
// 触发不重算的情形：
//   - 任务未冻结表达式（非表达式计费，或非本渠道的任务）；
//   - 上游未返回可用秒数（actualSeconds <= 0）；
//   - 秒数超出合理范围；
//   - 表达式重跑失败（配置被改坏等）。
//
// 结算失败只记录日志、不中断轮询：计费口径异常不应阻塞任务状态推进。
func recalcQuotaByActualSeconds(
	ctx context.Context,
	task *model.Task,
	actualSeconds int,
) (int, bool) {
	bc := task.PrivateData.BillingContext
	if !bc.HasTieredExpr() {
		return 0, false
	}

	if actualSeconds <= 0 {
		// 上游未提供实际时长：保持预扣额度，不做任何调整。
		// 这是刻意的选择 —— 拿不到可信的实际值时不猜测（例如尚未完成的轮询中间态）。
		logger.LogInfo(ctx, fmt.Sprintf(
			"Td 任务 %s 未获得上游实际秒数，保持预扣额度 %d 不变",
			task.TaskID, task.Quota,
		))
		return 0, false
	}
	if actualSeconds > maxSecondsSanity {
		logger.LogWarn(ctx, fmt.Sprintf(
			"Td 任务 %s 上游秒数 %d 超出合理上限 %d，判定不可信，保持预扣额度 %d 不变",
			task.TaskID, actualSeconds, maxSecondsSanity, task.Quota,
		))
		return 0, false
	}

	// 注入实际秒数后重跑表达式。
	body := buildRecalcInputBody(bc, task, actualSeconds)

	rawCost, trace, err := billingexpr.RunExprWithRequest(
		bc.TieredExprString,
		billingexpr.TokenParams{},
		billingexpr.RequestInput{Body: body},
	)
	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf(
			"Td 任务 %s 按实际秒数重算表达式失败，保持预扣额度 %d：%s",
			task.TaskID, task.Quota, err.Error(),
		))
		return 0, false
	}
	if rawCost < 0 {
		logger.LogWarn(ctx, fmt.Sprintf(
			"Td 任务 %s 重算得到负值 %.4f，判定异常，保持预扣额度 %d 不变",
			task.TaskID, rawCost, task.Quota,
		))
		return 0, false
	}

	// 版本决定换算公式：v2 为 $/次（不除 1e6），v1 为 $/1M tokens。
	// 使用任务冻结的 QuotaPerUnit 与 GroupRatio，确保与预扣口径一致。
	quotaBeforeGroup := billingexpr.QuotaConversion(rawCost, &billingexpr.BillingSnapshot{
		ExprVersion:  bc.TieredExprVersion,
		QuotaPerUnit: bc.TieredQuotaPerUnit,
	})
	actualQuota := billingexpr.QuotaRound(quotaBeforeGroup * bc.GroupRatio)

	logger.LogInfo(ctx, fmt.Sprintf(
		"Td 任务 %s 按上游实际秒数重算：seconds=%d cost=%.4f tier=%s 应扣=%d 预扣=%d",
		task.TaskID, actualSeconds, rawCost, trace.MatchedTier, actualQuota, task.Quota,
	))

	return actualQuota, true
}

// buildRecalcInputBody 构造重算用的计费输入体。
//
// 起点优先取 TieredInputBody —— 预扣时真正用于求值的计费输入体，完整保存未截断。
// 它保证「重算输入 = 预扣输入 + 新的 duration」，除 duration 外的任何差异都不会引入。
//
// 回退顺序：
//  1. TieredInputBody（首选：预扣时的真实输入，完整）；
//  2. PrivateData.RequestBody（旧任务兼容：该字段按 500B 截断，可能残缺）；
//  3. {}（表达式对缺失字段有 isnull 兜底，仍能得到基于时长的正确价格）。
//
// 为什么第 2 步不能作为首选：RequestBody 是日志字段，长提示词下会被截断成
// 残缺 JSON，此时 resolution 等维度会丢失，导致重算与预扣口径不一致。
func buildRecalcInputBody(bc *model.TaskBillingContext, task *model.Task, seconds int) []byte {
	body := bodyAsObject([]byte(bc.TieredInputBody))
	if body == nil {
		body = bodyAsObject([]byte(task.PrivateData.RequestBody))
	}
	if body == nil {
		body = []byte("{}")
	}
	return billingexpr.InjectBodyParam(body, "duration", seconds)
}

// bodyAsObject 判定 body 是否为合法 JSON 对象，是则原样返回，否则返回 nil。
//
// 目的：避免把非 JSON（空串、被截断的片段）交给 sjson 造成注入失败。
// 注意 RequestBody 落库时经过 TruncateBody 截断，超长报文可能已是残缺 JSON，
// 这种情况下必须退回 {}（或改用完整的 TieredInputBody），
// 否则 duration 注入不进去，重算会取到兜底价。
func bodyAsObject(body []byte) []byte {
	if len(body) == 0 {
		return nil
	}
	var probe map[string]any
	if err := common.Unmarshal(body, &probe); err != nil {
		return nil
	}
	return body
}

// AdjustBillingOnComplete 覆盖 taskcommon.BaseBilling 的空实现，
// 在任务到达终态时按上游实际秒数重算应扣额度。
//
// 注意：本方法只负责「算出应扣额度」，实际的钱包/令牌/订阅调整由
// service.RecalculateTaskQuota 统一执行（含日志与统计），
// 因此这里不做任何扣费动作，避免与既有结算路径重复记账。
//
// 返回值：
//   - 正数 → 作为实际应扣额度，交由轮询循环做差额结算（多退少补）；
//   - 0    → 保持预扣额度不变。
func (a *TaskAdaptor) AdjustBillingOnComplete(
	task *model.Task,
	taskResult *relaycommon.TaskInfo,
) int {
	if taskResult == nil {
		return 0
	}
	quota, ok := recalcQuotaByActualSeconds(context.Background(), task, taskResult.Seconds)
	if !ok {
		return 0
	}
	return quota
}
