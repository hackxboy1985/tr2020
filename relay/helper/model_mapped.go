package helper

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

// modelMappingLogPrefix 统一日志前缀，便于排查时 grep "[ModelMapping]"
const modelMappingLogPrefix = "[ModelMapping]"

// truncateForLog 截断过长的映射配置，避免刷屏；按 rune 截断避免中文乱码
func truncateForLog(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + fmt.Sprintf("...(已截断,共%d字符)", len(runes))
}

// getRequestModelName 读取请求体里的模型名，用于校验 SetModelName 是否真正写入。
// 注意：部分请求类型（如 GeminiChatRequest）没有 model 字段，SetModelName 是空实现，
// 此时上游模型名完全依赖 info.UpstreamModelName。
func getRequestModelName(request dto.Request) (string, bool) {
	switch r := request.(type) {
	case *dto.GeneralOpenAIRequest:
		return r.Model, true
	case *dto.ClaudeRequest:
		return r.Model, true
	case *dto.ImageRequest:
		return r.Model, true
	case *dto.EmbeddingRequest:
		return r.Model, true
	case *dto.RerankRequest:
		return r.Model, true
	case *dto.AudioRequest:
		return r.Model, true
	case *dto.OpenAIResponsesRequest:
		return r.Model, true
	case *dto.OpenAIResponsesCompactionRequest:
		return r.Model, true
	case *dto.GeminiEmbeddingRequest:
		return r.Model, true
	case nil:
		return "", false
	default:
		return "", false
	}
}

func ModelMappedHelper(c *gin.Context, info *common.RelayInfo, request dto.Request) error {
	if info.ChannelMeta == nil {
		info.ChannelMeta = &common.ChannelMeta{}
	}

	isResponsesCompact := info.RelayMode == relayconstant.RelayModeResponsesCompact
	originModelName := info.OriginModelName
	mappingModelName := originModelName
	if isResponsesCompact && strings.HasSuffix(originModelName, ratio_setting.CompactModelSuffix) {
		mappingModelName = strings.TrimSuffix(originModelName, ratio_setting.CompactModelSuffix)
	}

	// map model name
	modelMapping := c.GetString("model_mapping")

	// 入口日志：记录映射计算的输入与上下文
	reqModelBefore, hasModelField := getRequestModelName(request)
	logger.LogInfo(c, fmt.Sprintf(
		"%s 开始: channelId=%d channelType=%d relayMode=%d | OriginModelName=%q UpstreamModelName(进入时)=%q IsModelMapped(进入时)=%v | mappingModelName=%q isResponsesCompact=%v | request.Model=%q 有Model字段=%v | 映射配置=%q",
		modelMappingLogPrefix,
		info.ChannelId, info.ChannelType, info.RelayMode,
		originModelName, info.UpstreamModelName, info.IsModelMapped,
		mappingModelName, isResponsesCompact,
		reqModelBefore, hasModelField,
		truncateForLog(modelMapping, 500),
	))

	if modelMapping != "" && modelMapping != "{}" {
		modelMap := make(map[string]string)
		err := json.Unmarshal([]byte(modelMapping), &modelMap)
		if err != nil {
			logger.LogError(c, fmt.Sprintf(
				"%s 映射配置解析失败: channelId=%d model=%q err=%v 配置=%q",
				modelMappingLogPrefix, info.ChannelId, mappingModelName, err, truncateForLog(modelMapping, 500),
			))
			return fmt.Errorf("unmarshal_model_mapping_failed")
		}
		logger.LogInfo(c, fmt.Sprintf(
			"%s 配置解析成功: channelId=%d 条目数=%d 内容=%v",
			modelMappingLogPrefix, info.ChannelId, len(modelMap), modelMap,
		))

		// 支持链式模型重定向，最终使用链尾的模型
		currentModel := mappingModelName
		visitedModels := map[string]bool{
			currentModel: true,
		}
		for {
			if mappedModel, exists := modelMap[currentModel]; exists && mappedModel != "" {
				logger.LogInfo(c, fmt.Sprintf(
					"%s 链式匹配: %q -> %q (channelId=%d)",
					modelMappingLogPrefix, currentModel, mappedModel, info.ChannelId,
				))
				// 模型重定向循环检测，避免无限循环
				if visitedModels[mappedModel] {
					if mappedModel == currentModel {
						if currentModel == info.OriginModelName {
							logger.LogInfo(c, fmt.Sprintf(
								"%s 命中自身映射(映射回自己), 视为未映射: %q (channelId=%d)",
								modelMappingLogPrefix, currentModel, info.ChannelId,
							))
							info.IsModelMapped = false
							return nil
						} else {
							logger.LogInfo(c, fmt.Sprintf(
								"%s 检测到自环, 以链尾为准: %q -> %q (channelId=%d)",
								modelMappingLogPrefix, mappingModelName, currentModel, info.ChannelId,
							))
							info.IsModelMapped = true
							break
						}
					}
					logger.LogError(c, fmt.Sprintf(
						"%s 检测到映射循环: %q -> %q (channelId=%d) 配置=%q",
						modelMappingLogPrefix, currentModel, mappedModel, info.ChannelId, truncateForLog(modelMapping, 500),
					))
					return errors.New("model_mapping_contains_cycle")
				}
				visitedModels[mappedModel] = true
				currentModel = mappedModel
				info.IsModelMapped = true
			} else {
				logger.LogInfo(c, fmt.Sprintf(
					"%s 链式匹配结束: %q 无后续映射 (channelId=%d)",
					modelMappingLogPrefix, currentModel, info.ChannelId,
				))
				break
			}
		}
		if info.IsModelMapped {
			info.UpstreamModelName = currentModel
		}
	} else {
		logger.LogInfo(c, fmt.Sprintf(
			"%s 无映射配置(为空或\"{}\"), 跳过映射: channelId=%d model=%q",
			modelMappingLogPrefix, info.ChannelId, mappingModelName,
		))
	}

	if isResponsesCompact {
		finalUpstreamModelName := mappingModelName
		if info.IsModelMapped && info.UpstreamModelName != "" {
			finalUpstreamModelName = info.UpstreamModelName
		}
		info.UpstreamModelName = finalUpstreamModelName
		info.OriginModelName = ratio_setting.WithCompactModelSuffix(finalUpstreamModelName)
		logger.LogInfo(c, fmt.Sprintf(
			"%s responsesCompact 分支: UpstreamModelName=%q OriginModelName=%q (channelId=%d)",
			modelMappingLogPrefix, info.UpstreamModelName, info.OriginModelName, info.ChannelId,
		))
	}

	// 确保 UpstreamModelName 始终有值
	if info.UpstreamModelName == "" {
		info.UpstreamModelName = mappingModelName
		logger.LogInfo(c, fmt.Sprintf(
			"%s UpstreamModelName 为空, 兜底为 %q (channelId=%d)",
			modelMappingLogPrefix, mappingModelName, info.ChannelId,
		))
	}

	if request != nil {
		request.SetModelName(info.UpstreamModelName)

		// 校验 SetModelName 是否真正写入请求体：
		// 无 model 字段的请求类型（如 GeminiChatRequest）SetModelName 是空实现，此处会暴露"设了但没写进去"
		if afterModel, ok := getRequestModelName(request); ok {
			status := "已写入"
			if afterModel != info.UpstreamModelName {
				status = "未写入(异常)"
			}
			logger.LogInfo(c, fmt.Sprintf(
				"%s SetModelName: request.Model %q -> %q [%s] (channelId=%d)",
				modelMappingLogPrefix, reqModelBefore, afterModel, status, info.ChannelId,
			))
		} else {
			logger.LogInfo(c, fmt.Sprintf(
				"%s 该请求类型无 model 字段, SetModelName 为空实现, 上游模型名仅依赖 info.UpstreamModelName=%q (channelId=%d)",
				modelMappingLogPrefix, info.UpstreamModelName, info.ChannelId,
			))
		}
	}

	// 出口日志：映射的最终结果
	logger.LogInfo(c, fmt.Sprintf(
		"%s 结束: channelId=%d IsModelMapped=%v UpstreamModelName=%q OriginModelName=%q",
		modelMappingLogPrefix, info.ChannelId, info.IsModelMapped, info.UpstreamModelName, info.OriginModelName,
	))

	return nil
}
