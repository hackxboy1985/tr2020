package service

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

// AssetGatewayChannel holds the selected channel info for asset management.
// Supports multiple upstream versions: gateway, kwjm
type AssetGatewayChannel struct {
	Channel         *model.Channel
	GatewayURL      string // Base URL from settings
	Key             string // upstream token (first key)
	UpstreamVersion string // "gateway" | "kwjm"
	RelayMode       bool   // true = 下游是 new-api，使用 /api/seedance/* 路径；false = 直连 Gateway (仅 gateway 使用)
	KwjmModel       string // KWJM 默认模型 (仅 kwjm 使用)
}

// SeedanceGatewayChannel is an alias for backward compatibility
type SeedanceGatewayChannel = AssetGatewayChannel

// GetAssetGatewayChannel finds an enabled doubao-video channel for the given
// user group that has asset gateway configured.
func GetAssetGatewayChannel(userGroup string) (*AssetGatewayChannel, error) {
	channels, err := model.GetChannelsByType(0, 500, false, constant.ChannelTypeDoubaoVideo)
	if err != nil {
		return nil, fmt.Errorf("query channels failed: %w", err)
	}

	for _, ch := range channels {
		if ch.Status != common.ChannelStatusEnabled {
			continue
		}
		// check group
		if !isGroupAllowed(ch, userGroup) {
			continue
		}
		settings := ch.GetOtherSettings()

		// 读取上游版本配置
		version := settings.AssetUpstreamVersion
		if version == "" {
			version = "gateway" // 默认使用 gateway
		}

		var gatewayURL string
		var kwjmModel string

		switch version {
		case "kwjm":
			gatewayURL = settings.KwjmAssetBaseUrl
			kwjmModel = settings.KwjmAssetModel
			if kwjmModel == "" {
				kwjmModel = "sd-video-v2" // 默认模型
			}
		case "gateway":
			gatewayURL = settings.SeedanceAssetBaseUrl
			if gatewayURL == "" {
				// relay 模式下允许回退到渠道 Base URL，避免重复配置相同地址
				if !settings.SeedanceRelayMode {
					continue
				}
				gatewayURL = ch.GetBaseURL()
			}
		default:
			// 未知版本，回退到 gateway
			gatewayURL = settings.SeedanceAssetBaseUrl
			version = "gateway"
		}

		if gatewayURL == "" {
			continue
		}

		fullCh, err := model.GetChannelById(ch.Id, true)
		if err != nil {
			continue
		}
		key, _, apiErr := fullCh.GetNextEnabledKey()
		if apiErr != nil {
			continue
		}

		// 调试日志：输出选中的渠道配置
		common.SysLog(fmt.Sprintf(
			"[AssetGateway] selected channel %d for group '%s': version=%s, url=%s, model=%s, relay=%v",
			fullCh.Id, userGroup, version, gatewayURL, kwjmModel, settings.SeedanceRelayMode,
		))

		return &AssetGatewayChannel{
			Channel:         fullCh,
			GatewayURL:      strings.TrimRight(gatewayURL, "/"),
			Key:             key,
			UpstreamVersion: version,
			RelayMode:       settings.SeedanceRelayMode,
			KwjmModel:       kwjmModel,
		}, nil
	}
	return nil, fmt.Errorf("no available asset gateway channel for group %s", userGroup)
}

// GetSeedanceGatewayChannel is an alias for backward compatibility
func GetSeedanceGatewayChannel(userGroup string) (*SeedanceGatewayChannel, error) {
	return GetAssetGatewayChannel(userGroup)
}

// isGroupAllowed checks if the channel is available for the given user group.
func isGroupAllowed(ch *model.Channel, userGroup string) bool {
	groups := ch.Group
	if groups == "" {
		return false
	}
	for _, g := range strings.Split(groups, ",") {
		if strings.TrimSpace(g) == userGroup {
			return true
		}
	}
	return false
}

// RewritePathForRelay 在 relay 模式下将 Gateway 原生路径转换为 new-api 用户侧路径。
//
// Gateway 原生路径 → new-api 路径：
//   /api/seedance/proxy/assets/groups/* → /api/seedance/asset-groups/*
//   /api/seedance/proxy/assets/*        → /api/seedance/assets/*
//   /api/seedance/face-verifications/*  → /api/seedance/face-verifications/*（相同，不变）
func RewritePathForRelay(path string) string {
	if strings.HasPrefix(path, "/api/seedance/proxy/assets/groups") {
		return "/api/seedance/asset-groups" + path[len("/api/seedance/proxy/assets/groups"):]
	}
	if strings.HasPrefix(path, "/api/seedance/proxy/assets") {
		return "/api/seedance/assets" + path[len("/api/seedance/proxy/assets"):]
	}
	return path
}

// SeedanceProxyRequest proxies a request to the Seedance Gateway and returns
// the raw response body and HTTP status code.
// upstreamPath must start with "/" e.g. "/api/seedance/proxy/assets/groups"
func SeedanceProxyRequest(
	gc *SeedanceGatewayChannel,
	method string,
	upstreamPath string,
	queryParams url.Values,
	body []byte,
) (int, []byte, error) {
	if gc.RelayMode {
		upstreamPath = RewritePathForRelay(upstreamPath)
	}
	targetURL := gc.GatewayURL + upstreamPath
	if len(queryParams) > 0 {
		targetURL += "?" + queryParams.Encode()
	}

	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, targetURL, bodyReader)
	if err != nil {
		return 0, nil, fmt.Errorf("build request failed: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+gc.Key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	common.SysLog(fmt.Sprintf("seedance proxy: %s %s", method, targetURL))

	resp, err := GetHttpClient().Do(req)
	if err != nil {
		common.SysError(fmt.Sprintf("seedance proxy do request failed: %s %s: %v", method, targetURL, err))
		return 0, nil, fmt.Errorf("do request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("read response failed: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		common.SysError(fmt.Sprintf("seedance proxy upstream error: %s %s -> %d: %s", method, targetURL, resp.StatusCode, string(respBody)))
	}

	return resp.StatusCode, respBody, nil
}

// KwjmProxyRequest proxies a request to the KWJM upstream and returns
// the raw response body and HTTP status code.
func KwjmProxyRequest(
	gc *AssetGatewayChannel,
	action string,
	queryParams url.Values,
	body []byte,
) (int, []byte, error) {
	// 构造完整 URL
	targetURL := gc.GatewayURL + "/v3/open/" + action
	if len(queryParams) > 0 {
		targetURL += "?" + queryParams.Encode()
	}

	// 注入 model 字段到请求体
	if body != nil && len(body) > 0 {
		var reqBody map[string]interface{}
		if err := common.Unmarshal(body, &reqBody); err == nil {
			// 如果请求体没有 model 字段，注入默认 model
			if _, ok := reqBody["model"]; !ok {
				reqBody["model"] = gc.KwjmModel
			}
			body, _ = common.Marshal(reqBody)
		}
	}

	// 创建 HTTP 请求
	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(http.MethodPost, targetURL, bodyReader)
	if err != nil {
		return 0, nil, fmt.Errorf("create request failed: %w", err)
	}

	// 设置请求头
	req.Header.Set("Authorization", "Bearer "+gc.Key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	common.SysLog(fmt.Sprintf("kwjm proxy: POST %s", targetURL))

	// 发送请求
	client, err := GetHttpClientWithProxy("")
	if err != nil {
		return 0, nil, fmt.Errorf("get http client failed: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		common.SysError(fmt.Sprintf("kwjm proxy do request failed: POST %s: %v", targetURL, err))
		return 0, nil, fmt.Errorf("do request failed: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("read response failed: %w", err)
	}

	// 记录错误响应
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		common.SysError(fmt.Sprintf("kwjm proxy upstream error: POST %s -> %d: %s", targetURL, resp.StatusCode, string(respBody)))
	}

	return resp.StatusCode, respBody, nil
}

// pathToKwjmAction converts RESTful path and method to KWJM Action
func pathToKwjmAction(path, method string) string {
	// 资产分组相关
	if strings.Contains(path, "/assets/groups") {
		hasID := strings.Count(path, "/") > 5 // 路径包含 ID

		switch method {
		case http.MethodPost:
			return "CreateAssetGroup"
		case http.MethodGet:
			if hasID {
				return "GetAssetGroup"
			}
			return "ListAssetGroups"
		case http.MethodPut, http.MethodPatch:
			return "UpdateAssetGroup"
		case http.MethodDelete:
			return "DeleteAssetGroup"
		}
	}

	// 资产相关
	if strings.Contains(path, "/assets") && !strings.Contains(path, "/assets/groups") {
		hasID := strings.Count(path, "/") > 4 // 路径包含 ID

		switch method {
		case http.MethodPost:
			return "CreateAsset"
		case http.MethodGet:
			if hasID {
				return "GetAsset"
			}
			return "ListAssets"
		case http.MethodPut, http.MethodPatch:
			return "UpdateAsset"
		case http.MethodDelete:
			return "DeleteAsset"
		}
	}

	// 真人认证相关
	if strings.Contains(path, "/face-verifications") {
		hasID := strings.Count(path, "/") > 3

		switch method {
		case http.MethodPost:
			return "CreateVisualValidateSession"
		case http.MethodGet:
			if hasID {
				return "GetVisualValidateResult"
			}
		}
	}

	return ""
}

// AssetProxyRequest routes the request to the appropriate upstream based on version
func AssetProxyRequest(
	gc *AssetGatewayChannel,
	method string,
	upstreamPath string,
	queryParams url.Values,
	body []byte,
) (int, []byte, error) {
	switch gc.UpstreamVersion {
	case "kwjm":
		// KWJM 上游：转换路径为 Action
		action := pathToKwjmAction(upstreamPath, method)
		if action == "" {
			return 0, nil, fmt.Errorf("unsupported path for kwjm: %s %s", method, upstreamPath)
		}
		return KwjmProxyRequest(gc, action, queryParams, body)

	case "gateway":
		// Gateway 上游：保持原有逻辑
		return SeedanceProxyRequest(gc, method, upstreamPath, queryParams, body)

	default:
		return 0, nil, fmt.Errorf("unsupported upstream version: %s", gc.UpstreamVersion)
	}
}

