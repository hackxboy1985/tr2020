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
		key, _, apiErr := ch.GetNextEnabledKey()
		if apiErr != nil {
			continue
		}
		return &SeedanceGatewayChannel{
			Channel:    ch,
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

	resp, err := GetHttpClient().Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("do request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("read response failed: %w", err)
	}
	return resp.StatusCode, respBody, nil
}
