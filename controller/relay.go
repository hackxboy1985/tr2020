package controller

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/samber/lo"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// channelTestHistoryThrottle 限制每个渠道写入测试历史的频率（30秒内最多写一次）
var channelTestHistoryThrottle sync.Map // map[int]int64: channelId -> lastWriteUnixSec

// shouldWriteChannelTestHistory 判断是否应该写入测试历史（30秒内每渠道最多写一次）
func shouldWriteChannelTestHistory(channelId int) bool {
	now := time.Now().Unix()
	if last, ok := channelTestHistoryThrottle.Load(channelId); ok {
		if now-last.(int64) < 30 {
			return false
		}
	}
	channelTestHistoryThrottle.Store(channelId, now)
	return true
}

func relayHandler(c *gin.Context, info *relaycommon.RelayInfo) *types.NewAPIError {
	var err *types.NewAPIError
	switch info.RelayMode {
	case relayconstant.RelayModeImagesGenerations, relayconstant.RelayModeImagesEdits:
		err = relay.ImageHelper(c, info)
	case relayconstant.RelayModeAudioSpeech:
		fallthrough
	case relayconstant.RelayModeAudioTranslation:
		fallthrough
	case relayconstant.RelayModeAudioTranscription:
		err = relay.AudioHelper(c, info)
	case relayconstant.RelayModeRerank:
		err = relay.RerankHelper(c, info)
	case relayconstant.RelayModeEmbeddings:
		err = relay.EmbeddingHelper(c, info)
	case relayconstant.RelayModeResponses, relayconstant.RelayModeResponsesCompact:
		err = relay.ResponsesHelper(c, info)
	default:
		err = relay.TextHelper(c, info)
	}
	return err
}

func geminiRelayHandler(c *gin.Context, info *relaycommon.RelayInfo) *types.NewAPIError {
	var err *types.NewAPIError
	if strings.Contains(c.Request.URL.Path, "embed") {
		err = relay.GeminiEmbeddingHandler(c, info)
	} else {
		err = relay.GeminiHelper(c, info)
	}
	return err
}

func Relay(c *gin.Context, relayFormat types.RelayFormat) {

	requestId := c.GetString(common.RequestIdKey)
	//group := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	//originalModel := common.GetContextKeyString(c, constant.ContextKeyOriginalModel)

	var (
		newAPIError *types.NewAPIError
		ws          *websocket.Conn
	)

	if relayFormat == types.RelayFormatOpenAIRealtime {
		var err error
		ws, err = upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			helper.WssError(c, ws, types.NewError(err, types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry()).ToOpenAIError())
			return
		}
		defer ws.Close()
	}

	defer func() {
		if newAPIError != nil {
			logger.LogError(c, fmt.Sprintf("relay error: %s", common.LocalLogPreview(newAPIError.Error())))
			newAPIError.SetMessage(common.MessageWithRequestId(newAPIError.Error(), requestId))
			switch relayFormat {
			case types.RelayFormatOpenAIRealtime:
				helper.WssError(c, ws, newAPIError.ToOpenAIError())
			case types.RelayFormatClaude:
				c.JSON(newAPIError.StatusCode, gin.H{
					"type":  "error",
					"error": newAPIError.ToClaudeError(),
				})
			default:
				c.JSON(newAPIError.StatusCode, gin.H{
					"error": newAPIError.ToOpenAIError(),
				})
			}
		}
	}()

	request, err := helper.GetAndValidateRequest(c, relayFormat)
	if err != nil {
		// Map "request body too large" to 413 so clients can handle it correctly
		if common.IsRequestBodyTooLargeError(err) || errors.Is(err, common.ErrRequestBodyTooLarge) {
			newAPIError = types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusRequestEntityTooLarge, types.ErrOptionWithSkipRetry())
		} else {
			newAPIError = types.NewError(err, types.ErrorCodeInvalidRequest)
		}
		return
	}

	relayInfo, err := relaycommon.GenRelayInfo(c, relayFormat, request, ws)
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeGenRelayInfoFailed)
		return
	}

	needSensitiveCheck := setting.ShouldCheckPromptSensitive()
	needCountToken := constant.CountToken
	// Avoid building huge CombineText (strings.Join) when token counting and sensitive check are both disabled.
	var meta *types.TokenCountMeta
	if needSensitiveCheck || needCountToken {
		meta = request.GetTokenCountMeta()
	} else {
		meta = fastTokenCountMetaForPricing(request)
	}

	if needSensitiveCheck && meta != nil {
		contains, words := service.CheckSensitiveText(meta.CombineText)
		if contains {
			logger.LogWarn(c, fmt.Sprintf("user sensitive words detected: %s", strings.Join(words, ", ")))
			newAPIError = types.NewError(err, types.ErrorCodeSensitiveWordsDetected)
			return
		}
	}

	// Store prompt text in context for optional prompt saving (checked later in RecordConsumeLog)
	if meta != nil && meta.CombineText != "" {
		c.Set(string(constant.ContextKeyPromptToSave), meta.CombineText)
	}

	tokens, err := service.EstimateRequestToken(c, meta, relayInfo)

	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeCountTokenFailed)
		return
	}

	relayInfo.SetEstimatePromptTokens(tokens)

	priceData, err := helper.ModelPriceHelper(c, relayInfo, tokens, meta)
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeModelPriceError, types.ErrOptionWithStatusCode(http.StatusBadRequest))
		return
	}

	// common.SetContextKey(c, constant.ContextKeyTokenCountMeta, meta)

	if priceData.FreeModel {
		logger.LogInfo(c, fmt.Sprintf("模型 %s 免费，跳过预扣费", relayInfo.OriginModelName))
	} else {
		newAPIError = service.PreConsumeBilling(c, priceData.QuotaToPreConsume, relayInfo)
		if newAPIError != nil {
			return
		}
	}

	defer func() {
		// Only return quota if downstream failed and quota was actually pre-consumed
		if newAPIError != nil {
			newAPIError = service.NormalizeViolationFeeError(newAPIError)
			if relayInfo.Billing != nil {
				relayInfo.Billing.Refund(c)
			}
			service.ChargeViolationFeeIfNeeded(c, relayInfo, newAPIError)
		}
	}()

	retryParam := &service.RetryParam{
		Ctx:        c,
		TokenGroup: relayInfo.TokenGroup,
		ModelName:  relayInfo.OriginModelName,
		Retry:      common.GetPointer(0),
	}
	relayInfo.RetryIndex = 0
	relayInfo.LastError = nil

	for ; retryParam.GetRetry() <= common.RetryTimes; retryParam.IncreaseRetry() {
		relayInfo.RetryIndex = retryParam.GetRetry()
		channel, channelErr := getChannel(c, relayInfo, retryParam)
		if channelErr != nil {
			logger.LogError(c, channelErr.Error())
			// 如果之前已经有上游返回的错误，保留上游错误而不是用渠道错误覆盖
			// If there's already an upstream error, keep it instead of overwriting with channel error
			if relayInfo.LastError != nil {
				newAPIError = relayInfo.LastError
			} else {
				newAPIError = channelErr
			}
			break
		}

		addUsedChannel(c, channel.Id)
		bodyStorage, bodyErr := common.GetBodyStorage(c)
		if bodyErr != nil {
			// Ensure consistent 413 for oversized bodies even when error occurs later (e.g., retry path)
			if common.IsRequestBodyTooLargeError(bodyErr) || errors.Is(bodyErr, common.ErrRequestBodyTooLarge) {
				newAPIError = types.NewErrorWithStatusCode(bodyErr, types.ErrorCodeReadRequestBodyFailed, http.StatusRequestEntityTooLarge, types.ErrOptionWithSkipRetry())
			} else {
				newAPIError = types.NewErrorWithStatusCode(bodyErr, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
			}
			break
		}
		c.Request.Body = io.NopCloser(bodyStorage)

		switch relayFormat {
		case types.RelayFormatOpenAIRealtime:
			newAPIError = relay.WssHelper(c, relayInfo)
		case types.RelayFormatClaude:
			newAPIError = relay.ClaudeHelper(c, relayInfo)
		case types.RelayFormatGemini:
			newAPIError = geminiRelayHandler(c, relayInfo)
		default:
			newAPIError = relayHandler(c, relayInfo)
		}

		if newAPIError == nil {
			relayInfo.LastError = nil
			// 将用户真实请求数据写入测试历史，供分组监控使用（30秒内每渠道最多写一次）
			startTime := common.GetContextKeyTime(c, constant.ContextKeyRequestStartTime)
			if !startTime.IsZero() && shouldWriteChannelTestHistory(channel.Id) {
				responseTime := int(time.Since(startTime).Milliseconds())
				modelName := common.GetContextKeyString(c, constant.ContextKeyOriginalModel)
				go func(chId, rt int, m string) {
					logger.LogInfo(c, fmt.Sprintf("渠道 #%d 用户请求成功（%dms），写入测试历史", chId, rt))
					model.RecordChannelTestHistory(chId, true, rt, m)
				}(channel.Id, responseTime, modelName)
			}
			return
		}

		newAPIError = service.NormalizeViolationFeeError(newAPIError)
		relayInfo.LastError = newAPIError

		processChannelError(c, *types.NewChannelError(channel.Id, channel.Type, channel.Name, channel.ChannelInfo.IsMultiKey, common.GetContextKeyString(c, constant.ContextKeyChannelKey), channel.GetAutoBan()), newAPIError)

		// 过载休眠池：上游返回配置的过载状态码（默认 429/503）时，把 (模型, 渠道) 放入休眠池，
		// 后续请求将跳过该渠道，直接使用下一渠道。
		//
		// 注意：若本次请求已兜底切换到备用模型，则必须用**备用模型名**标记休眠。
		// 否则会把「备用模型的渠道」错误地记到原模型名下：既污染原模型的休眠池
		// （该渠道对原模型可能完全健康），又导致备用模型自身永不被标记，
		// 后续请求会反复命中同一个已过载的备用模型渠道。
		if operation_setting.IsOverloadStatusCode(newAPIError.StatusCode) {
			service.MarkOverloadSleep(c, overloadSleepModelName(c, relayInfo), channel,
				common.GetContextKeyString(c, constant.ContextKeyUsingGroup), newAPIError.StatusCode)
		}

		if !shouldRetry(c, newAPIError, common.RetryTimes-retryParam.GetRetry()) {
			break
		}
	}

	useChannel := c.GetStringSlice("use_channel")
	if len(useChannel) > 1 {
		retryLogStr := fmt.Sprintf("重试：%s", strings.Trim(strings.Join(strings.Fields(fmt.Sprint(useChannel)), "->"), "[]"))
		logger.LogInfo(c, retryLogStr)
	}
	if newAPIError != nil {
		gopool.Go(func() {
			perfmetrics.RecordRelaySample(relayInfo, false, 0)
		})
	}
}

var upgrader = websocket.Upgrader{
	Subprotocols: []string{"realtime"}, // WS 握手支持的协议，如果有使用 Sec-WebSocket-Protocol，则必须在此声明对应的 Protocol TODO add other protocol
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许跨域
	},
}

func addUsedChannel(c *gin.Context, channelId int) {
	useChannel := c.GetStringSlice("use_channel")
	useChannel = append(useChannel, fmt.Sprintf("%d", channelId))
	c.Set("use_channel", useChannel)
}

func fastTokenCountMetaForPricing(request dto.Request) *types.TokenCountMeta {
	if request == nil {
		return &types.TokenCountMeta{}
	}
	meta := &types.TokenCountMeta{
		TokenType: types.TokenTypeTokenizer,
	}
	switch r := request.(type) {
	case *dto.GeneralOpenAIRequest:
		maxCompletionTokens := lo.FromPtrOr(r.MaxCompletionTokens, uint(0))
		maxTokens := lo.FromPtrOr(r.MaxTokens, uint(0))
		if maxCompletionTokens > maxTokens {
			meta.MaxTokens = int(maxCompletionTokens)
		} else {
			meta.MaxTokens = int(maxTokens)
		}
	case *dto.OpenAIResponsesRequest:
		meta.MaxTokens = int(lo.FromPtrOr(r.MaxOutputTokens, uint(0)))
	case *dto.ClaudeRequest:
		meta.MaxTokens = int(lo.FromPtr(r.MaxTokens))
	case *dto.ImageRequest:
		// Pricing for image requests depends on ImagePriceRatio; safe to compute even when CountToken is disabled.
		return r.GetTokenCountMeta()
	default:
		// Best-effort: leave CombineText empty to avoid large allocations.
	}
	return meta
}

// tryFallbackModel 在原模型「全部渠道都在过载休眠池」时，尝试改用备用模型。
//
// 这是「备用模型兜底」的接线点，返回最终应使用的渠道（一定非 nil）。
//
// 关键约束（用户明确要求）：
//   - 只有 API 密钥开启了「使用相近模型兜底」才会切换；关闭时必须回退到
//     「原模型休眠池中的最后一个渠道」重试，绝不能使用备用模型。
//     该判断由 service.ResolveFallbackModel 内部完成。
//   - 备用模型也会跳过它自己处于休眠中的渠道；
//   - 若备用模型的渠道会把请求映射回原模型（形成循环），跳过这些渠道；
//   - 任何一步失败都保留原渠道，保证「不会一个渠道都不请求」。
func tryFallbackModel(
	c *gin.Context,
	info *relaycommon.RelayInfo,
	retryParam *service.RetryParam,
	channel *model.Channel,
	selectGroup string,
) *model.Channel {
	if channel == nil || retryParam == nil || !retryParam.AllChannelsSleeping {
		return channel
	}
	originModel := info.OriginModelName
	if originModel == "" {
		return channel
	}
	// 二次确认：当前渠道确实仍在休眠池中，才说明「原模型没有可用渠道，
	// 这个渠道是被『全部休眠取最后一个』兜底选中的」。
	// 注意不能用 AllChannelsSleeping 单独判断：auto 分组下它是粘性标志，
	// 可能某个分组全休眠、但后续分组已经取到了可用渠道。
	if len(service.GetOverloadSleepExcludedChannels(originModel, []int{channel.Id})) == 0 {
		return channel
	}

	fallbackModel, ok := service.ResolveFallbackModel(c, originModel)
	if !ok {
		// 未开启兜底 / 未配置 / 已兜底过 / 配置存在环：沿用原渠道（最后一个渠道）
		return channel
	}

	// 分组范围门控（关键，用户明确要求）：
	//   - 密钥指定了某个具体分组：只在那个分组内查找备用模型渠道，查不到就放弃兜底，
	//     回退到「原模型休眠池中的最后一个渠道」，绝不跨分组；
	//   - 密钥是 auto 分组：重走 auto 分组链，按顺序在各分组内查找备用模型渠道，
	//     只会在「用户有权限的分组」内查找（与 auto 现有跨分组重试语义一致），
	//     最终走到哪个分组，就按哪个分组的分组倍率计费。
	searchGroups := service.FallbackSearchGroups(c, selectGroup)
	if len(searchGroups) == 0 {
		logger.LogInfo(c, "备用模型兜底：当前请求没有可用于兜底的分组（密钥指定分组时不会跨分组），回退到原模型的最后一个渠道")
		return channel
	}

	var (
		result      *model.ChannelSelectionResult
		resultGroup string
		allSkipped  []string
	)
	for _, group := range searchGroups {
		candidates := model.GetSatisfiedChannelCandidates(group, fallbackModel)
		if len(candidates) == 0 {
			continue
		}
		// 剔除「会把备用模型映射回原模型」的渠道，避免死循环
		groupKept, skipped := service.FilterChannelsCausingFallbackCycle(c, candidates, originModel, fallbackModel)
		allSkipped = append(allSkipped, skipped...)
		if len(groupKept) == 0 {
			continue
		}
		// 备用模型自身也要跳过处于休眠中的渠道
		candidateIds := make([]int, 0, len(groupKept))
		for _, candidate := range groupKept {
			candidateIds = append(candidateIds, candidate.ChannelId)
		}
		sleeping := service.GetOverloadSleepExcludedChannels(fallbackModel, candidateIds)
		exclude := make(map[int]struct{}, len(sleeping))
		for channelId := range sleeping {
			exclude[channelId] = struct{}{}
		}
		selected, selectErr := model.SelectChannelAvoidingExcluded(groupKept, retryParam.GetRetry(), exclude)
		if selectErr != nil || selected == nil || selected.Channel == nil {
			continue
		}
		result = selected
		resultGroup = group
		break
	}

	if result == nil || result.Channel == nil {
		if len(allSkipped) > 0 {
			logger.LogInfo(c, fmt.Sprintf("备用模型兜底：备用模型 %s 在可搜索分组的渠道都会绕回原模型 %s（共跳过 %d 个渠道），放弃兜底，回退到原模型的最后一个渠道", fallbackModel, originModel, len(allSkipped)))
		} else {
			logger.LogInfo(c, fmt.Sprintf("备用模型兜底：备用模型 %s 在可搜索分组 [%s] 下没有可用渠道，放弃兜底，回退到原模型的最后一个渠道", fallbackModel, strings.Join(searchGroups, ",")))
		}
		return channel
	}

	// 切换成功。
	//
	// 关键设计：只把「发给上游的模型名」改成备用模型，**不改 relayInfo.OriginModelName**。
	// 原因：
	//   - 预扣费在进入重试循环之前就已按原模型算好（ModelPriceHelper 用的就是
	//     OriginModelName），而 BillingSettler 只提供 Reserve（只能补不能重算），
	//     若此时改 OriginModelName，会导致「预扣按原模型、结算按备用模型」两边不一致；
	//   - 从用户视角看，他请求的就是原模型，按原模型计费也是合理的。
	//
	// 上游模型名通过 info.ChannelMeta.UpstreamModelName 传递：
	// ModelMappedHelper 会在每次请求时用 info.OriginModelName 重新计算它，
	// 因此这里需要同时把 context 中的「模型映射基准」调整为备用模型。
	service.MarkFallbackSwitched(c, originModel, fallbackModel)

	// 用 context key 承载「本次实际使用的模型」，供 ModelMappedHelper 取用上游模型名。
	common.SetContextKey(c, constant.ContextKeyFallbackModelName, fallbackModel)
	// retryParam.ModelName 决定后续重试选哪个模型的渠道，必须是备用模型。
	retryParam.ModelName = fallbackModel
	retryParam.AllChannelsSleeping = false
	// 标记已兜底，避免二次兜底（ResolveFallbackModel 内部读它）。
	common.SetContextKey(c, constant.ContextKeyFallbackOriginModel, originModel)

	// 分组倍率按「备用模型实际命中的分组」重算并补扣差额。
	//
	// 为什么要重算：分组倍率在选渠道前就已写进 PriceData.GroupRatioInfo，
	// 而 auto 分组下备用模型可能命中另一个分组（例如原模型在 default 组、
	// 备用模型只在 vip 组），两者分组倍率不同。
	// 按约定（用户明确要求）：走到哪个分组，就按哪个分组的分组倍率计费。
	applyFallbackGroupRatio(c, info, resultGroup)

	return result.Channel
}

// applyFallbackGroupRatio 兜底切换后，按「备用模型实际命中的分组」重新计算分组倍率，
// 并在需要补扣时通过 BillingSession.Reserve 补足预扣额度。
//
// 说明：这里只处理「补扣」（备用分组倍率更高的情况）。若备用分组倍率更低，
// 预扣额度不会被主动退还，但结算时 PostConsumeQuota 会按实际额度做差额返还
// （delta 为负时会 IncreaseUserQuota），因此不会多收。
func applyFallbackGroupRatio(c *gin.Context, info *relaycommon.RelayInfo, targetGroup string) {
	if c == nil || info == nil || targetGroup == "" {
		return
	}
	if targetGroup == info.UsingGroup {
		// 同一分组：分组倍率不变，无需处理（绝大多数情况走这里）
		return
	}

	oldGroupRatio := info.PriceData.GroupRatioInfo.GroupRatio
	previousGroup := info.UsingGroup

	// 用「备用模型实际命中的分组」重新计算分组倍率。
	// 注意：即使两个分组倍率恰好相同，也必须把 UsingGroup 与 auto_group 更新为目标分组，
	// 否则日志、结算记录里的分组会与实际使用的渠道不一致。
	info.UsingGroup = targetGroup
	newGroupRatioInfo := helper.HandleGroupRatio(c, info)
	info.PriceData.GroupRatioInfo = newGroupRatioInfo
	common.SetContextKey(c, constant.ContextKeyAutoGroup, targetGroup)

	// auto 分组下，后续重试必须从「备用模型命中的这个分组」重新开始找渠道，
	// 否则会延续原模型遗留的分组游标，跳过本该命中的分组。
	resetAutoGroupCursor(c, targetGroup)

	if newGroupRatioInfo.GroupRatio == oldGroupRatio {
		// 倍率恰好相同：只需同步分组信息，无需补扣
		logger.LogInfo(c, fmt.Sprintf(
			"备用模型兜底：实际使用分组 %s（原分组 %s），分组倍率保持 %v",
			targetGroup, previousGroup, newGroupRatioInfo.GroupRatio))
		return
	}

	logger.LogInfo(c, fmt.Sprintf(
		"备用模型兜底：实际使用分组 %s（原分组 %s），分组倍率 %v -> %v",
		targetGroup, previousGroup, oldGroupRatio, newGroupRatioInfo.GroupRatio))

	// 预扣额度是按旧分组倍率算的；若新倍率更高，补足预扣额度，
	// 避免「预扣不足」在结算时被当作超额消费。
	if info.Billing != nil && info.PriceData.QuotaToPreConsume > 0 && oldGroupRatio > 0 {
		target := int(float64(info.PriceData.QuotaToPreConsume) * (newGroupRatioInfo.GroupRatio / oldGroupRatio))
		if target > 0 {
			if err := info.Billing.Reserve(target); err != nil {
				logger.LogError(c, fmt.Sprintf("备用模型兜底：补足预扣额度失败（分组 %s）：%s", targetGroup, err.Error()))
			}
		}
	}
}

// resetAutoGroupCursor 把 auto 分组的选择游标重置到「目标分组」，
// 保证备用模型兜底后的后续重试仍从该分组开始，而不是沿用原模型留下的游标。
// 仅对 auto 分组请求生效；指定分组的请求不涉及分组游标。
func resetAutoGroupCursor(c *gin.Context, targetGroup string) {
	if c == nil || targetGroup == "" || !service.IsAutoGroupRequest(c) {
		return
	}
	userGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup)
	autoGroups := service.GetUserAutoGroup(userGroup)
	for index, group := range autoGroups {
		if group == targetGroup {
			common.SetContextKey(c, constant.ContextKeyAutoGroupIndex, index)
			common.SetContextKey(c, constant.ContextKeyAutoGroupRetryIndex, 0)
			return
		}
	}
	// 目标分组不在 auto 链里（理论上不会发生），回到链首，避免卡在越界索引
	common.SetContextKey(c, constant.ContextKeyAutoGroupIndex, 0)
	common.SetContextKey(c, constant.ContextKeyAutoGroupRetryIndex, 0)
}

func getChannel(c *gin.Context, info *relaycommon.RelayInfo, retryParam *service.RetryParam) (*model.Channel, *types.NewAPIError) {
	if info.ChannelMeta == nil {
		autoBan := c.GetBool("auto_ban")
		autoBanInt := 1
		if !autoBan {
			autoBanInt = 0
		}
		return &model.Channel{
			Id:      c.GetInt("channel_id"),
			Type:    c.GetInt("channel_type"),
			Name:    c.GetString("channel_name"),
			AutoBan: &autoBanInt,
		}, nil
	}
	channel, selectGroup, err := service.CacheGetRandomSatisfiedChannel(retryParam)

	info.PriceData.GroupRatioInfo = helper.HandleGroupRatio(c, info)

	if err != nil {
		return nil, types.NewError(fmt.Errorf("获取分组 %s 下模型 %s 的可用渠道失败（retry）: %s", selectGroup, info.OriginModelName, err.Error()), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}
	if channel == nil {
		return nil, types.NewError(fmt.Errorf("分组 %s 下模型 %s 的可用渠道不存在（retry）", selectGroup, info.OriginModelName), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}

	// 备用模型兜底：原模型全部渠道都在休眠池中时，按需切换到备用模型。
	// 密钥「使用相近模型兜底」开关关闭时，此调用内部会直接返回原渠道，
	// 即继续使用「休眠池中的最后一个渠道」重试。
	channel = tryFallbackModel(c, info, retryParam, channel, selectGroup)

	// 兜底过程可能已把 info.OriginModelName 切换为备用模型，
	// 这里必须用最新的模型名写入渠道上下文，保证上游请求与渠道模型映射一致。
	newAPIError := middleware.SetupContextForSelectedChannel(c, channel, info.OriginModelName)
	if newAPIError != nil {
		return nil, newAPIError
	}
	return channel, nil
}

// overloadSleepModelName 返回「标记过载休眠时应使用的模型名」。
//
// 正常情况下就是用户请求的模型名；若本次请求因原模型全渠道休眠而兜底切换到了
// 备用模型，则实际请求的是备用模型，休眠必须记在备用模型名下，
// 否则会污染原模型的休眠池且备用模型自身不会被休眠。
func overloadSleepModelName(c *gin.Context, info *relaycommon.RelayInfo) string {
	if fallbackModel := service.GetFallbackModelName(c); fallbackModel != "" {
		return fallbackModel
	}
	if info != nil && info.OriginModelName != "" {
		return info.OriginModelName
	}
	return common.GetContextKeyString(c, constant.ContextKeyOriginalModel)
}

func shouldRetry(c *gin.Context, openaiErr *types.NewAPIError, retryTimes int) bool {
	if openaiErr == nil {
		return false
	}
	if service.ShouldSkipRetryAfterChannelAffinityFailure(c) {
		return false
	}
	if types.IsChannelError(openaiErr) {
		return true
	}
	if types.IsSkipRetryError(openaiErr) {
		return false
	}
	if retryTimes <= 0 {
		return false
	}
	if _, ok := c.Get("specific_channel_id"); ok {
		return false
	}
	code := openaiErr.StatusCode
	if code >= 200 && code < 300 {
		return false
	}
	if code < 100 || code > 599 {
		return true
	}
	if operation_setting.IsAlwaysSkipRetryCode(openaiErr.GetErrorCode()) {
		return false
	}
	return operation_setting.ShouldRetryByStatusCode(code)
}

func processChannelError(c *gin.Context, channelError types.ChannelError, err *types.NewAPIError) {
	logger.LogError(c, fmt.Sprintf("channel error (channel #%d, status code: %d): %s", channelError.ChannelId, err.StatusCode, common.LocalLogPreview(err.Error())))
	// 不要使用context获取渠道信息，异步处理时可能会出现渠道信息不一致的情况
	// do not use context to get channel info, there may be inconsistent channel info when processing asynchronously
	if service.ShouldDisableChannel(err, channelError.ChannelId) && channelError.AutoBan {
		gopool.Go(func() {
			service.DisableChannel(channelError, err.ErrorWithStatusCode())
		})
	}

	if constant.ErrorLogEnabled && types.IsRecordErrorLog(err) {
		// 保存错误日志到mysql中
		userId := c.GetInt("id")
		tokenName := c.GetString("token_name")
		modelName := c.GetString("original_model")
		tokenId := c.GetInt("token_id")
		userGroup := c.GetString("group")
		channelId := c.GetInt("channel_id")
		other := make(map[string]interface{})
		if c.Request != nil && c.Request.URL != nil {
			other["request_path"] = c.Request.URL.Path
		}
		other["error_type"] = err.GetErrorType()
		other["error_code"] = err.GetErrorCode()
		other["status_code"] = err.StatusCode
		other["channel_id"] = channelId
		other["channel_name"] = c.GetString("channel_name")
		other["channel_type"] = c.GetInt("channel_type")
		adminInfo := make(map[string]interface{})
		adminInfo["use_channel"] = c.GetStringSlice("use_channel")
		isMultiKey := common.GetContextKeyBool(c, constant.ContextKeyChannelIsMultiKey)
		if isMultiKey {
			adminInfo["is_multi_key"] = true
			adminInfo["multi_key_index"] = common.GetContextKeyInt(c, constant.ContextKeyChannelMultiKeyIndex)
		}
		service.AppendChannelAffinityAdminInfo(c, adminInfo)
		other["admin_info"] = adminInfo
		// 添加 request_body 和 response_body 到 other 中，以便在日志详情中显示
		if requestBody := c.GetString(string(constant.ContextKeyVideoRequestBody)); requestBody != "" {
			other["request_body"] = service.TruncateBody(requestBody)
		}
		if responseBody := c.GetString(string(constant.ContextKeyVideoResponseBody)); responseBody != "" {
			other["response_body"] = service.TruncateBody(responseBody)
		}
		startTime := common.GetContextKeyTime(c, constant.ContextKeyRequestStartTime)
		if startTime.IsZero() {
			startTime = time.Now()
		}
		useTimeSeconds := int(time.Since(startTime).Seconds())
		model.RecordErrorLog(c, userId, channelId, modelName, tokenName, err.MaskSensitiveErrorWithStatusCode(), tokenId, useTimeSeconds, common.GetContextKeyBool(c, constant.ContextKeyIsStream), userGroup, other)
	}

}

func RelayMidjourney(c *gin.Context) {
	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatMjProxy, nil, nil)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"description": fmt.Sprintf("failed to generate relay info: %s", err.Error()),
			"type":        "upstream_error",
			"code":        4,
		})
		return
	}

	var mjErr *dto.MidjourneyResponse
	switch relayInfo.RelayMode {
	case relayconstant.RelayModeMidjourneyNotify:
		mjErr = relay.RelayMidjourneyNotify(c)
	case relayconstant.RelayModeMidjourneyTaskFetch, relayconstant.RelayModeMidjourneyTaskFetchByCondition:
		mjErr = relay.RelayMidjourneyTask(c, relayInfo.RelayMode)
	case relayconstant.RelayModeMidjourneyTaskImageSeed:
		mjErr = relay.RelayMidjourneyTaskImageSeed(c)
	case relayconstant.RelayModeSwapFace:
		mjErr = relay.RelaySwapFace(c, relayInfo)
	default:
		mjErr = relay.RelayMidjourneySubmit(c, relayInfo)
	}
	//err = relayMidjourneySubmit(c, relayMode)
	log.Println(mjErr)
	if mjErr != nil {
		statusCode := http.StatusBadRequest
		if mjErr.Code == 30 {
			mjErr.Result = "当前分组负载已饱和，请稍后再试，或升级账户以提升服务质量。"
			statusCode = http.StatusTooManyRequests
		}
		c.JSON(statusCode, gin.H{
			"description": fmt.Sprintf("%s %s", mjErr.Description, mjErr.Result),
			"type":        "upstream_error",
			"code":        mjErr.Code,
		})
		channelId := c.GetInt("channel_id")
		logger.LogError(c, fmt.Sprintf("relay error (channel #%d, status code %d): %s", channelId, statusCode, fmt.Sprintf("%s %s", mjErr.Description, mjErr.Result)))
	}
}

func RelayNotImplemented(c *gin.Context) {
	err := types.OpenAIError{
		Message: "API not implemented",
		Type:    "new_api_error",
		Param:   "",
		Code:    "api_not_implemented",
	}
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": err,
	})
}

func RelayNotFound(c *gin.Context) {
	err := types.OpenAIError{
		Message: fmt.Sprintf("Invalid URL (%s %s)", c.Request.Method, c.Request.URL.Path),
		Type:    "invalid_request_error",
		Param:   "",
		Code:    "",
	}
	c.JSON(http.StatusNotFound, gin.H{
		"error": err,
	})
}

func RelayTaskFetch(c *gin.Context) {
	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatTask, nil, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, &dto.TaskError{
			Code:       "gen_relay_info_failed",
			Message:    err.Error(),
			StatusCode: http.StatusInternalServerError,
		})
		return
	}
	if taskErr := relay.RelayTaskFetch(c, relayInfo.RelayMode); taskErr != nil {
		respondTaskError(c, taskErr)
	}
}

func RelayTask(c *gin.Context) {
	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatTask, nil, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, &dto.TaskError{
			Code:       "gen_relay_info_failed",
			Message:    err.Error(),
			StatusCode: http.StatusInternalServerError,
		})
		return
	}

	if taskErr := relay.ResolveOriginTask(c, relayInfo); taskErr != nil {
		respondTaskError(c, taskErr)
		return
	}

	var result *relay.TaskSubmitResult
	var taskErr *dto.TaskError
	defer func() {
		if taskErr != nil && relayInfo.Billing != nil {
			relayInfo.Billing.Refund(c)
		}
	}()

	retryParam := &service.RetryParam{
		Ctx:        c,
		TokenGroup: relayInfo.TokenGroup,
		ModelName:  relayInfo.OriginModelName,
		Retry:      common.GetPointer(0),
	}

	for ; retryParam.GetRetry() <= common.RetryTimes; retryParam.IncreaseRetry() {
		var channel *model.Channel

		if lockedCh, ok := relayInfo.LockedChannel.(*model.Channel); ok && lockedCh != nil {
			channel = lockedCh
			if retryParam.GetRetry() > 0 {
				if setupErr := middleware.SetupContextForSelectedChannel(c, channel, relayInfo.OriginModelName); setupErr != nil {
					taskErr = service.TaskErrorWrapperLocal(setupErr.Err, "setup_locked_channel_failed", http.StatusInternalServerError)
					break
				}
			}
		} else {
			var channelErr *types.NewAPIError
			channel, channelErr = getChannel(c, relayInfo, retryParam)
			if channelErr != nil {
				logger.LogError(c, channelErr.Error())
				taskErr = service.TaskErrorWrapperLocal(channelErr.Err, "get_channel_failed", http.StatusInternalServerError)
				break
			}
		}

		addUsedChannel(c, channel.Id)
		bodyStorage, bodyErr := common.GetBodyStorage(c)
		if bodyErr != nil {
			if common.IsRequestBodyTooLargeError(bodyErr) || errors.Is(bodyErr, common.ErrRequestBodyTooLarge) {
				taskErr = service.TaskErrorWrapperLocal(bodyErr, "read_request_body_failed", http.StatusRequestEntityTooLarge)
			} else {
				taskErr = service.TaskErrorWrapperLocal(bodyErr, "read_request_body_failed", http.StatusBadRequest)
			}
			break
		}
		c.Request.Body = io.NopCloser(bodyStorage)

		result, taskErr = relay.RelayTaskSubmit(c, relayInfo)
		if taskErr == nil {
			break
		}

		if !taskErr.LocalError {
			processChannelError(c,
				*types.NewChannelError(channel.Id, channel.Type, channel.Name, channel.ChannelInfo.IsMultiKey,
					common.GetContextKeyString(c, constant.ContextKeyChannelKey), channel.GetAutoBan()),
				types.NewOpenAIError(taskErr.Error, types.ErrorCodeBadResponseStatusCode, taskErr.StatusCode))
		}

		// 过载休眠池：任务提交同样按过载状态码休眠 (模型, 渠道)
		if !taskErr.LocalError && operation_setting.IsOverloadStatusCode(taskErr.StatusCode) {
			var triggerChannel *model.Channel = channel
			service.MarkOverloadSleep(c, overloadSleepModelName(c, relayInfo), triggerChannel,
				common.GetContextKeyString(c, constant.ContextKeyUsingGroup), taskErr.StatusCode)
		}

		if !shouldRetryTaskRelay(c, channel.Id, taskErr, common.RetryTimes-retryParam.GetRetry()) {
			break
		}
	}

	useChannel := c.GetStringSlice("use_channel")
	if len(useChannel) > 1 {
		retryLogStr := fmt.Sprintf("重试：%s", strings.Trim(strings.Join(strings.Fields(fmt.Sprint(useChannel)), "->"), "[]"))
		logger.LogInfo(c, retryLogStr)
	}

	// ── 成功：结算 + 日志 + 插入任务 ──
	if taskErr == nil {
		if settleErr := service.SettleBilling(c, relayInfo, result.Quota); settleErr != nil {
			common.SysError("settle task billing error: " + settleErr.Error())
		}

		// 设置请求体和 prompt，供 LogTaskConsumption → savePrompt 使用
		if bs, bsErr := common.GetBodyStorage(c); bsErr == nil {
			if _, seekErr := bs.Seek(0, io.SeekStart); seekErr == nil {
				if bodyBytes, readErr := io.ReadAll(bs); readErr == nil && len(bodyBytes) > 0 {
					c.Set(string(constant.ContextKeyVideoRequestBody), string(bodyBytes))
				}
			}
		}
		if v, exists := c.Get("task_request"); exists {
			if req, ok := v.(relaycommon.TaskSubmitReq); ok && req.Prompt != "" {
				c.Set(string(constant.ContextKeyPromptToSave), req.Prompt)
			}
		}

		service.LogTaskConsumption(c, relayInfo)

		task := model.InitTask(result.Platform, relayInfo)
		task.PrivateData.UpstreamTaskID = result.UpstreamTaskID
		task.PrivateData.BillingSource = relayInfo.BillingSource
		task.PrivateData.SubscriptionId = relayInfo.SubscriptionId
		task.PrivateData.TokenId = relayInfo.TokenId
		task.PrivateData.BillingContext = &model.TaskBillingContext{
			ModelPrice:      relayInfo.PriceData.ModelPrice,
			GroupRatio:      relayInfo.PriceData.GroupRatioInfo.GroupRatio,
			ModelRatio:      relayInfo.PriceData.ModelRatio,
			OtherRatios:     relayInfo.PriceData.OtherRatios,
			OriginModelName: relayInfo.OriginModelName,
			PerCallBilling:  common.StringsContains(constant.TaskPricePatches, relayInfo.OriginModelName) || relayInfo.PriceData.UsePrice,
		}
		// Td 视频：冻结计费输入快照，供轮询阶段按上游实际秒数重跑表达式。
		//
		// 为什么只对 Td 生效：按上游实际参数重算是 Td 视频专属需求
		// （其上游会收敛时长）。其他渠道无此诉求，不应承担额外存储与行为变化。
		//
		// 为什么单独存一份而不复用 RequestBody：RequestBody 用于日志展示，
		// 会经 TruncateBody 按 SavePromptBodyMaxBytes(500B) 截断；而重算需要
		// 完整可解析的 JSON（含 resolution / aspect_ratio 等）。
		// 复用截断后的体在长提示词下会退化为残缺 JSON，造成结算口径漂移。
		if relayInfo.ChannelType == constant.ChannelTypeTudou {
			if snap := relayInfo.TieredBillingSnapshot; snap != nil && snap.BillingMode == "tiered_expr" {
				task.PrivateData.BillingContext.TieredExprString = snap.ExprString
				task.PrivateData.BillingContext.TieredExprVersion = snap.ExprVersion
				task.PrivateData.BillingContext.TieredQuotaPerUnit = snap.QuotaPerUnit
				// 计费输入体（adaptor 注入后的结果），完整保存不截断
				if input := relayInfo.BillingRequestInput; input != nil && len(input.Body) > 0 {
					task.PrivateData.BillingContext.TieredInputBody = string(input.Body)
				}
			}
		}
		task.Quota = result.Quota
		task.Data = result.TaskData
		task.Action = relayInfo.Action
		if v, exists := c.Get("task_request"); exists {
			if req, ok := v.(relaycommon.TaskSubmitReq); ok && req.Prompt != "" {
				task.Properties.Input = req.Prompt
			}
		}
		if rb := c.GetString(string(constant.ContextKeyVideoRequestBody)); rb != "" {
			task.PrivateData.RequestBody = service.TruncateBody(rb)
		}
		if len(result.TaskData) > 0 {
			task.PrivateData.SubmitRespBody = service.TruncateBody(string(result.TaskData))
		}
		if insertErr := task.Insert(); insertErr != nil {
			// 落库失败不中断响应，但必须能事后关联到具体任务：
			// 带上 taskId 才能在排查「查询接口查不到任务 / 轮询不推进」时
			// 区分是「根本没落库」还是「落库了但状态没推进」。
			common.SysError(fmt.Sprintf("insert task error: taskId=%s err=%s", relayInfo.PublicTaskID, insertErr.Error()))
		}
	}

	if taskErr != nil {
		respondTaskError(c, taskErr)
	}
}

// respondTaskError 统一输出 Task 错误响应（含 429 限流提示改写）
func respondTaskError(c *gin.Context, taskErr *dto.TaskError) {
	if taskErr.StatusCode == http.StatusTooManyRequests {
		taskErr.Message = "当前分组上游负载已饱和，请稍后再试"
	}
	c.JSON(taskErr.StatusCode, taskErr)
}

func shouldRetryTaskRelay(c *gin.Context, channelId int, taskErr *dto.TaskError, retryTimes int) bool {
	if taskErr == nil {
		return false
	}
	if service.ShouldSkipRetryAfterChannelAffinityFailure(c) {
		return false
	}
	if retryTimes <= 0 {
		return false
	}
	if _, ok := c.Get("specific_channel_id"); ok {
		return false
	}
	if taskErr.StatusCode == http.StatusTooManyRequests {
		return true
	}
	if taskErr.StatusCode == 307 {
		return true
	}
	if taskErr.StatusCode/100 == 5 {
		// 超时不重试
		if operation_setting.IsAlwaysSkipRetryStatusCode(taskErr.StatusCode) {
			return false
		}
		return true
	}
	if taskErr.StatusCode == http.StatusBadRequest {
		return false
	}
	if taskErr.StatusCode == 408 {
		// azure处理超时不重试
		return false
	}
	if taskErr.LocalError {
		return false
	}
	if taskErr.StatusCode/100 == 2 {
		return false
	}
	return true
}
