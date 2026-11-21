package service

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

// 备用模型兜底用到的 context key 别名，便于本文件内使用
const (
	constantContextKeyTokenFallbackModelEnabled = constant.ContextKeyTokenFallbackModelEnabled
	constantContextKeyFallbackOriginModel       = constant.ContextKeyFallbackOriginModel
	constantContextKeyFallbackModelName         = constant.ContextKeyFallbackModelName
)

// 备用模型兜底（Fallback Model）
//
// 需求场景：
//   - 模型 A 配置了备用模型 B（运维 -> 系统行为 -> 备用模型，形如 A=B）；
//   - 请求 A 时，A 的全部渠道都在过载休眠池中；
//   - 且本次请求所用 API 密钥开启了「使用相近模型兜底」；
//   则把请求改走 B，使用 B 自己的渠道。
//
// 死循环防护（关键）：
//   - 配置层面：A=B、B=A 是允许的（运维可以这样配），但运行时要能检测并中断；
//   - 渠道层面：A 兜底到 B 后，若某渠道把 B 映射回 A（渠道自带的 ModelMapping），
//     说明走该渠道必然绕回原模型形成死循环 → 跳过该渠道，换下一个渠道尝试；
//   - 最终目的：既不因为循环无限重试，又能尽可能用相近模型兜底成功。

// CurrentFallbackOriginModel 返回本次请求原始模型（用户请求的模型），
// 用于判断「是否已经处于备用模型兜底状态」以及日志记录。
func CurrentFallbackOriginModel(c *gin.Context) string {
	if c == nil {
		return ""
	}
	return common.GetContextKeyString(c, constantContextKeyFallbackOriginModel)
}

// IsFallbackModelEnabledForToken 本次请求所用 API 密钥是否开启了「使用相近模型兜底」。
func IsFallbackModelEnabledForToken(c *gin.Context) bool {
	if c == nil {
		return false
	}
	return common.GetContextKeyBool(c, constantContextKeyTokenFallbackModelEnabled)
}

// ResolveFallbackModel 判断当前请求是否需要切换到备用模型。
//
// 返回 (备用模型名, 是否切换)。只有同时满足以下条件才切换：
//  1. 过载休眠池总开关打开；
//  2. 本次请求的密钥开启了「使用相近模型兜底」；
//  3. 该模型配置了备用模型；
//  4. 尚未处于兜底状态（避免兜底套兜底）；
//  5. 备用模型与原模型不同，且不会构成「备用模型 -> 原模型」的配置环。
func ResolveFallbackModel(c *gin.Context, originModel string) (string, bool) {
	if c == nil || originModel == "" {
		return "", false
	}
	if !operation_setting.IsOverloadSleepEnabled() {
		return "", false
	}
	if !IsFallbackModelEnabledForToken(c) {
		return "", false
	}
	// 已经兜底过一次，不再二次兜底（防止 A->B->C->... 无限链）
	if CurrentFallbackOriginModel(c) != "" {
		return "", false
	}
	fallback := operation_setting.GetFallbackModel(originModel)
	if fallback == "" || fallback == originModel {
		return "", false
	}
	// 配置环检测：A=B 且 B=A 时，切换过去没有意义（备用模型的渠道也全在休眠时
	// 会再次切回 A），直接放弃兜底并记录，避免运行时反复来回。
	if cycle := operation_setting.DescribeFallbackModelCycle(originModel); cycle != "" {
		logger.LogInfo(c, fmt.Sprintf("备用模型兜底：检测到备用模型配置存在环路（%s），本次不做兜底切换", cycle))
		return "", false
	}
	return fallback, true
}

// MarkFallbackSwitched 标记本次请求已切换到备用模型，并记录原始模型。
func MarkFallbackSwitched(c *gin.Context, originModel string, fallbackModel string) {
	if c == nil {
		return
	}
	common.SetContextKey(c, constantContextKeyFallbackOriginModel, originModel)
	common.SetContextKey(c, constantContextKeyFallbackModelName, fallbackModel)
	logger.LogInfo(c, fmt.Sprintf("备用模型兜底：模型 %s 的全部渠道均在过载休眠池中，改用备用模型 %s 重试", originModel, fallbackModel))
}

// GetFallbackModelName 返回本次请求切换到的备用模型（未切换返回空）。
func GetFallbackModelName(c *gin.Context) string {
	if c == nil {
		return ""
	}
	return common.GetContextKeyString(c, constantContextKeyFallbackModelName)
}

// ChannelModelMappingReturnsToOrigin 判断「渠道的模型映射」是否会把备用模型
// 映射回原模型，从而形成死循环。
//
// 例：原模型 gemini-3.6-flash，备用模型 gemini-3.7-flash；
// 某渠道配置了 {"gemini-3.7-flash": "gemini-3.6-flash"}，
// 那么走该渠道实际请求的仍是 3.6，等于没兜底还可能再次过载 → 视为循环，跳过该渠道。
//
// 用链式解析（与 relay.ModelMappedHelper 一致的语义）判断最终落点是否为原模型。
func ChannelModelMappingReturnsToOrigin(mappingJSON string, fallbackModel string, originModel string) bool {
	if mappingJSON == "" || mappingJSON == "{}" || fallbackModel == "" || originModel == "" {
		return false
	}
	if fallbackModel == originModel {
		return false
	}
	modelMap := make(map[string]string)
	if err := common.UnmarshalJsonStr(mappingJSON, &modelMap); err != nil {
		return false
	}
	if len(modelMap) == 0 {
		return false
	}
	// 链式映射解析，带环检测（与 ModelMappedHelper 语义保持一致）
	current := fallbackModel
	visited := map[string]struct{}{current: {}}
	for {
		next, ok := modelMap[current]
		if !ok || next == "" {
			break
		}
		if next == originModel {
			return true
		}
		if _, seen := visited[next]; seen {
			break
		}
		visited[next] = struct{}{}
		current = next
	}
	return false
}

// FilterChannelsCausingFallbackCycle 从候选渠道中剔除「会把备用模型映射回原模型」的渠道。
//
// 返回可用于兜底的渠道 ID 列表，以及被剔除的渠道信息（供日志说明）。
func FilterChannelsCausingFallbackCycle(
	c *gin.Context,
	candidates []model.ChannelCandidate,
	originModel string,
	fallbackModel string,
) ([]model.ChannelCandidate, []string) {
	kept := make([]model.ChannelCandidate, 0, len(candidates))
	skipped := make([]string, 0)
	for _, candidate := range candidates {
		channel, err := model.CacheGetChannel(candidate.ChannelId)
		if err != nil || channel == nil {
			// 取不到渠道详情时不因循环检测剔除它，交给后续正常流程处理
			kept = append(kept, candidate)
			continue
		}
		if ChannelModelMappingReturnsToOrigin(channel.GetModelMapping(), fallbackModel, originModel) {
			skipped = append(skipped, fmt.Sprintf("#%d(%s)", candidate.ChannelId, channel.Name))
			continue
		}
		kept = append(kept, candidate)
	}
	if len(skipped) > 0 && c != nil {
		logger.LogInfo(c, fmt.Sprintf(
			"备用模型兜底：备用模型 %s 在以下渠道会被映射回原模型 %s（形成循环），已跳过：%s",
			fallbackModel, originModel, strings.Join(skipped, ",")))
	}
	return kept, skipped
}
