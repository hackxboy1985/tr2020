package helper

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	rootcommon "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

// modelMappingLogPrefix 统一日志前缀，便于排查时 grep "[ModelMapping]"
const modelMappingLogPrefix = "[ModelMapping]"

func ModelMappedHelper(c *gin.Context, info *common.RelayInfo, request dto.Request) error {
	if info.ChannelMeta == nil {
		info.ChannelMeta = &common.ChannelMeta{}
	}

	isResponsesCompact := info.RelayMode == relayconstant.RelayModeResponsesCompact
	originModelName := info.OriginModelName
	mappingModelName := originModelName
	// 备用模型兜底：当本次请求因「原模型全部渠道休眠」而切换到备用模型时，
	// 上游实际请求应使用备用模型名（并继续套用该渠道的模型映射），
	// 但 info.OriginModelName 保持为原模型，用于计费与用户侧的模型标识。
	// 这样既能把请求正确发到备用模型，又不会让预扣费/结算口径错乱。
	fallbackModelName := rootcommon.GetContextKeyString(c, constant.ContextKeyFallbackModelName)
	if fallbackModelName != "" {
		mappingModelName = fallbackModelName
	} else if isResponsesCompact && strings.HasSuffix(originModelName, ratio_setting.CompactModelSuffix) {
		mappingModelName = strings.TrimSuffix(originModelName, ratio_setting.CompactModelSuffix)
	}

	// map model name
	modelMapping := c.GetString("model_mapping")
	if modelMapping != "" && modelMapping != "{}" {
		modelMap := make(map[string]string)
		err := json.Unmarshal([]byte(modelMapping), &modelMap)
		if err != nil {
			return fmt.Errorf("unmarshal_model_mapping_failed")
		}

		// 支持链式模型重定向，最终使用链尾的模型
		currentModel := mappingModelName
		visitedModels := map[string]bool{
			currentModel: true,
		}
		for {
			if mappedModel, exists := modelMap[currentModel]; exists && mappedModel != "" {
				// 模型重定向循环检测，避免无限循环
				if visitedModels[mappedModel] {
					if mappedModel == currentModel {
						if currentModel == info.OriginModelName {
							info.IsModelMapped = false
							return nil
						} else {
							info.IsModelMapped = true
							break
						}
					}
					return errors.New("model_mapping_contains_cycle")
				}
				visitedModels[mappedModel] = true
				currentModel = mappedModel
				info.IsModelMapped = true
			} else {
				break
			}
		}
		if info.IsModelMapped {
			info.UpstreamModelName = currentModel
		}
	} else if fallbackModelName != "" {
		// 渠道没有配置模型映射，但本次请求已切换到备用模型：
		// 上游仍应使用备用模型名，而不是原模型名。
		info.UpstreamModelName = fallbackModelName
	}

	if isResponsesCompact {
		finalUpstreamModelName := mappingModelName
		if info.IsModelMapped && info.UpstreamModelName != "" {
			finalUpstreamModelName = info.UpstreamModelName
		}
		info.UpstreamModelName = finalUpstreamModelName
		info.OriginModelName = ratio_setting.WithCompactModelSuffix(finalUpstreamModelName)
	}

	// 确保 UpstreamModelName 始终有值
	if info.UpstreamModelName == "" {
		info.UpstreamModelName = mappingModelName
	}

	if request != nil {
		request.SetModelName(info.UpstreamModelName)
	}

	// 出口日志：映射的最终结果
	logger.LogInfo(c, fmt.Sprintf(
		"%s 结束: channelId=%d IsModelMapped=%v UpstreamModelName=%q OriginModelName=%q",
		modelMappingLogPrefix, info.ChannelId, info.IsModelMapped, info.UpstreamModelName, info.OriginModelName,
	))

	return nil
}
