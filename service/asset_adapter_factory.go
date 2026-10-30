package service

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

// UpstreamVersionSdos2 sdos2 上游版本标识（第 2 个海外渠道，seedance-gateway）
const UpstreamVersionSdos2 = "sdos2"

// IsSdos2Upstream 判断渠道是否使用 sdos2 上游契约。
//
// sdos2 上游的成片地址是受 Bearer 保护的接口路径（/v1/videos/{id}/content），
// 与直接返回可访问 URL 的上游不同，下载时必须由服务端携带渠道 Key 代理拉流。
func IsSdos2Upstream(ch *model.Channel) bool {
	if ch == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(ch.GetOtherSettings().AssetUpstreamVersion), UpstreamVersionSdos2)
}

// ResolveSdos2RelayBaseURL 返回返回给下游的成片下载地址所用的本服务基址。
//
// 优先使用渠道 other 配置 sdos2_relay_base_url（支持每个渠道指向不同入口），
// 未配置时回退到系统设置 ServerAddress。两者都为空时返回空串，由调用方兜底。
func ResolveSdos2RelayBaseURL(ch *model.Channel) string {
	if ch != nil {
		if v := strings.TrimSpace(ch.GetOtherSettings().Sdos2RelayBaseUrl); v != "" {
			return strings.TrimRight(v, "/")
		}
	}
	if v := strings.TrimSpace(system_setting.ServerAddress); v != "" {
		return strings.TrimRight(v, "/")
	}
	return ""
}

// IsSdos2UpstreamByID 按渠道 ID 判断是否为 sdos2 上游。
//
// 供无法直接拿到渠道对象的调用方使用（如任务适配器的响应转换）。
// 查询失败时按非 sdos2 处理，保持其他上游行为不变。
func IsSdos2UpstreamByID(channelID int) bool {
	ch, err := model.CacheGetChannel(channelID)
	if err != nil || ch == nil {
		return false
	}
	return IsSdos2Upstream(ch)
}

// GetAssetAdapter 根据用户分组获取素材管理适配器
func GetAssetAdapter(userGroup string) (AssetAdapter, *model.Channel, error) {
	common.SysLog(fmt.Sprintf("[GetAssetAdapter] 开始查找分组 '%s' 的渠道", userGroup))

	channels, err := model.GetChannelsByType(0, 500, false, constant.ChannelTypeDoubaoVideo)
	if err != nil {
		return nil, nil, fmt.Errorf("query channels failed: %w", err)
	}

	common.SysLog(fmt.Sprintf("[GetAssetAdapter] 查询到 %d 个 DoubaoVideo 渠道", len(channels)))

	for _, ch := range channels {
		common.SysLog(fmt.Sprintf("[GetAssetAdapter] 检查渠道 #%d: name=%s, status=%d, group=%s",
			ch.Id, ch.Name, ch.Status, ch.Group))

		if ch.Status != common.ChannelStatusEnabled {
			common.SysLog(fmt.Sprintf("[GetAssetAdapter] 渠道 #%d 未启用，跳过", ch.Id))
			continue
		}
		// check group
		if !isGroupAllowed(ch, userGroup) {
			common.SysLog(fmt.Sprintf("[GetAssetAdapter] 渠道 #%d 分组不匹配 (需要: %s, 渠道: %s)，跳过",
				ch.Id, userGroup, ch.Group))
			continue
		}

		common.SysLog(fmt.Sprintf("[GetAssetAdapter] 渠道 #%d 分组匹配，获取完整信息", ch.Id))

		// 获取完整渠道信息（包含 key）
		fullCh, err := model.GetChannelById(ch.Id, true)
		if err != nil {
			common.SysLog(fmt.Sprintf("[GetAssetAdapter] 渠道 #%d 获取完整信息失败: %v", ch.Id, err))
			continue
		}

		key, _, apiErr := fullCh.GetNextEnabledKey()
		if apiErr != nil {
			common.SysLog(fmt.Sprintf("[GetAssetAdapter] 渠道 #%d 没有可用 Key: %v", ch.Id, apiErr))
			continue
		}

		common.SysLog(fmt.Sprintf("[GetAssetAdapter] 渠道 #%d Key 可用", ch.Id))

		settings := fullCh.GetOtherSettings()

		// 读取上游版本配置
		version := settings.AssetUpstreamVersion
		if version == "" {
			version = "gateway" // 默认使用 gateway
		}

		common.SysLog(fmt.Sprintf("[GetAssetAdapter] 渠道 #%d 上游版本: %s", ch.Id, version))

		var adapter AssetAdapter

		switch version {
		case "kwjm":
			// KWJM 适配器
			baseURL := settings.KwjmAssetBaseUrl
			model := settings.KwjmAssetModel
			if model == "" {
				model = "sd-video-v2" // 默认模型
			}

			common.SysLog(fmt.Sprintf(
				"[AssetAdapter] selected KWJM adapter for group '%s': channel=%d, url=%s, model=%s",
				userGroup, fullCh.Id, baseURL, model,
			))

			adapter = NewKwjmAssetAdapter(
				strings.TrimRight(baseURL, "/"),
				key,
				model,
			)

		case "gateway":
			// Gateway 适配器
			baseURL := settings.SeedanceAssetBaseUrl
			relayMode := settings.SeedanceRelayMode

			common.SysLog(fmt.Sprintf(
				"[AssetAdapter] selected Gateway adapter for group '%s': channel=%d, url=%s, relay=%v",
				userGroup, fullCh.Id, baseURL, relayMode,
			))

			adapter = NewGatewayAssetAdapter(
				strings.TrimRight(baseURL, "/"),
				key,
				relayMode,
			)

		case "sdos2":
			// seedance-gateway 原生契约适配器
			baseURL := settings.SeedanceAssetBaseUrl
			if baseURL == "" {
				baseURL = fullCh.GetBaseURL()
			}

			common.SysLog(fmt.Sprintf(
				"[AssetAdapter] selected Sdos2 adapter for group '%s': channel=%d, url=%s",
				userGroup, fullCh.Id, baseURL,
			))

			adapter = NewSdos2AssetAdapter(baseURL, key)

		default:
			common.SysLog(fmt.Sprintf(
				"[AssetAdapter] unsupported upstream version '%s' for channel %d, fallback to gateway",
				version, fullCh.Id,
			))

			// fallback 到 Gateway
			baseURL := settings.SeedanceAssetBaseUrl
			adapter = NewGatewayAssetAdapter(
				strings.TrimRight(baseURL, "/"),
				key,
				settings.SeedanceRelayMode,
			)
		}

		return adapter, fullCh, nil
	}

	common.SysLog(fmt.Sprintf("[GetAssetAdapter] 未找到可用渠道，分组: %s", userGroup))
	return nil, nil, fmt.Errorf("no available asset adapter for group %s", userGroup)
}

// GetAssetAdapterByModel 根据用户分组和模型名获取素材管理适配器
// 如果指定了模型名，优先选择支持该模型的渠道；否则降级为按分组选择
func GetAssetAdapterByModel(userGroup string, modelName string) (AssetAdapter, *model.Channel, error) {
	channels, err := model.GetChannelsByType(0, 500, false, constant.ChannelTypeDoubaoVideo)
	if err != nil {
		return nil, nil, fmt.Errorf("query channels failed: %w", err)
	}

	// 如果指定了模型名，优先查找支持该模型的渠道
	if modelName != "" {
		for _, ch := range channels {
			if ch.Status != common.ChannelStatusEnabled {
				continue
			}
			// check group
			if !isGroupAllowed(ch, userGroup) {
				continue
			}
			// check model
			if !isModelSupported(ch, modelName) {
				continue
			}

			// 获取完整渠道信息（包含 key）
			fullCh, err := model.GetChannelById(ch.Id, true)
			if err != nil {
				continue
			}

			key, _, apiErr := fullCh.GetNextEnabledKey()
			if apiErr != nil {
				continue
			}

			adapter, adapterErr := createAdapter(fullCh, key)
			if adapterErr != nil {
				continue
			}

			common.SysLog(fmt.Sprintf(
				"[AssetAdapter] selected channel %d for model '%s' in group '%s'",
				fullCh.Id, modelName, userGroup,
			))

			return adapter, fullCh, nil
		}
	}

	// 如果没有指定模型或没找到支持该模型的渠道，降级为按分组选择
	return GetAssetAdapter(userGroup)
}

// createAdapter 根据渠道配置创建对应的适配器
func createAdapter(fullCh *model.Channel, key string) (AssetAdapter, error) {
	settings := fullCh.GetOtherSettings()
	version := strings.ToLower(settings.AssetUpstreamVersion)

	switch version {
	case "kwjm":
		baseURL := settings.KwjmAssetBaseUrl
		if baseURL == "" {
			baseURL = fullCh.GetBaseURL()
		}
		modelName := settings.KwjmAssetModel

		common.SysLog(fmt.Sprintf(
			"[AssetAdapter] creating KWJM adapter: channel=%d, url=%s, model=%s",
			fullCh.Id, baseURL, modelName,
		))

		return NewKwjmAssetAdapter(
			strings.TrimRight(baseURL, "/"),
			key,
			modelName,
		), nil

	case "gateway":
		baseURL := settings.SeedanceAssetBaseUrl
		relayMode := settings.SeedanceRelayMode

		common.SysLog(fmt.Sprintf(
			"[AssetAdapter] creating Gateway adapter: channel=%d, url=%s, relay=%v",
			fullCh.Id, baseURL, relayMode,
		))

		return NewGatewayAssetAdapter(
			strings.TrimRight(baseURL, "/"),
			key,
			relayMode,
		), nil

	case "sdos2":
		// seedance-gateway 原生契约：/seedance/media、/seedance/media/groups、/seedance/media/upload
		// 与咪咕 gateway（/api/seedance/proxy/assets）完全不同，字段为小写蛇形
		baseURL := settings.SeedanceAssetBaseUrl
		if baseURL == "" {
			baseURL = fullCh.GetBaseURL()
		}

		common.SysLog(fmt.Sprintf(
			"[AssetAdapter] creating Sdos2 adapter: channel=%d, url=%s",
			fullCh.Id, baseURL,
		))

		return NewSdos2AssetAdapter(baseURL, key), nil

	default:
		common.SysLog(fmt.Sprintf(
			"[AssetAdapter] unsupported upstream version '%s' for channel %d, fallback to gateway",
			version, fullCh.Id,
		))

		// fallback 到 Gateway
		baseURL := settings.SeedanceAssetBaseUrl
		return NewGatewayAssetAdapter(
			strings.TrimRight(baseURL, "/"),
			key,
			settings.SeedanceRelayMode,
		), nil
	}
}

// isModelSupported 检查渠道是否支持指定的模型
func isModelSupported(ch *model.Channel, modelName string) bool {
	models := strings.Split(ch.Models, ",")
	for _, m := range models {
		if strings.TrimSpace(m) == modelName {
			return true
		}
	}
	return false
}
