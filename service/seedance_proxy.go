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

// SeedanceGatewayChannel holds the selected channel info for a Gateway request.
type SeedanceGatewayChannel struct {
	Channel    *model.Channel
	GatewayURL string // SeedanceAssetBaseUrl from OtherSettings
	Key        string // upstream token (first key)
}

// GetSeedanceGatewayChannel finds an enabled doubao-video channel for the given
// user group that has SeedanceAssetBaseUrl configured.
func GetSeedanceGatewayChannel(userGroup string) (*SeedanceGatewayChannel, error) {
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
		if settings.SeedanceAssetBaseUrl == "" {
			continue
		}
		// GetChannelsByType omits the key field — reload with key
		fullCh, err := model.GetChannelById(ch.Id, true)
		if err != nil {
			continue
		}
		key, _, apiErr := fullCh.GetNextEnabledKey()
		if apiErr != nil {
			continue
		}
		common.SysLog(fmt.Sprintf("seedance: selected channel %d, key prefix: %s, gateway: %s", ch.Id, func() string {
			if len(key) > 10 {
				return key[:10] + "..."
			}
			return key
		}(), strings.TrimRight(settings.SeedanceAssetBaseUrl, "/")))
		return &SeedanceGatewayChannel{
			Channel:    fullCh,
			GatewayURL: strings.TrimRight(settings.SeedanceAssetBaseUrl, "/"),
			Key:        key,
		}, nil
	}
	return nil, fmt.Errorf("no available seedance gateway channel for group %s", userGroup)
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
