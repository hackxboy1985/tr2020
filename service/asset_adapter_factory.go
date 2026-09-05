package service

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

// GetAssetAdapter 根据用户分组获取素材管理适配器
func GetAssetAdapter(userGroup string) (AssetAdapter, *model.Channel, error) {
	channels, err := model.GetChannelsByType(0, 500, false, constant.ChannelTypeDoubaoVideo)
	if err != nil {
		return nil, nil, fmt.Errorf("query channels failed: %w", err)
	}

	for _, ch := range channels {
		if ch.Status != common.ChannelStatusEnabled {
			continue
		}
		// check group
		if !isGroupAllowed(ch, userGroup) {
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

		settings := fullCh.GetOtherSettings()

		// 读取上游版本配置
		version := settings.AssetUpstreamVersion
		if version == "" {
			version = "gateway" // 默认使用 gateway
		}

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

	return nil, nil, fmt.Errorf("no available asset adapter for group %s", userGroup)
}
