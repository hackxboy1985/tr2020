package service

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/QuantumNous/new-api/common"
)

// GatewayAssetAdapter Gateway 上游的素材管理适配器
type GatewayAssetAdapter struct {
	baseURL   string
	token     string
	relayMode bool
}

// NewGatewayAssetAdapter 创建 Gateway 适配器
func NewGatewayAssetAdapter(baseURL, token string, relayMode bool) *GatewayAssetAdapter {
	return &GatewayAssetAdapter{
		baseURL:   baseURL,
		token:     token,
		relayMode: relayMode,
	}
}

// CreateAssetGroup 创建素材组
func (a *GatewayAssetAdapter) CreateAssetGroup(req CreateAssetGroupRequest) (*AssetGroupResponse, error) {
	path := "/api/seedance/proxy/assets/groups"
	if a.relayMode {
		path = RewritePathForRelay(path)
	}

	reqBody, _ := common.Marshal(req)
	statusCode, respBody, err := a.doRequest(http.MethodPost, path, nil, reqBody)
	if err != nil {
		return nil, err
	}
	if statusCode < 200 || statusCode >= 300 {
		return nil, fmt.Errorf("upstream error %d: %s", statusCode, string(respBody))
	}

	// 解析 Gateway 格式响应：{ "Result": { "Id": "xxx", "Name": "xxx", ... } }
	var resp struct {
		Result struct {
			ID   string `json:"Id"`
			Name string `json:"Name"`
			Desc string `json:"Description"`
			Type string `json:"GroupType"`
		} `json:"Result"`
	}
	if err := common.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parse response failed: %w", err)
	}

	return &AssetGroupResponse{
		ID:          resp.Result.ID,
		Name:        resp.Result.Name,
		Description: resp.Result.Desc,
		GroupType:   resp.Result.Type,
		RawData:     string(respBody),
	}, nil
}

// GetAssetGroup 查询素材组
func (a *GatewayAssetAdapter) GetAssetGroup(groupID string) (*AssetGroupResponse, error) {
	path := fmt.Sprintf("/api/seedance/proxy/assets/groups/%s", groupID)
	if a.relayMode {
		path = RewritePathForRelay(path)
	}

	statusCode, respBody, err := a.doRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	if statusCode < 200 || statusCode >= 300 {
		return nil, fmt.Errorf("upstream error %d: %s", statusCode, string(respBody))
	}

	var resp struct {
		Result struct {
			ID   string `json:"Id"`
			Name string `json:"Name"`
			Desc string `json:"Description"`
			Type string `json:"GroupType"`
		} `json:"Result"`
	}
	if err := common.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parse response failed: %w", err)
	}

	return &AssetGroupResponse{
		ID:          resp.Result.ID,
		Name:        resp.Result.Name,
		Description: resp.Result.Desc,
		GroupType:   resp.Result.Type,
		RawData:     string(respBody),
	}, nil
}

// UpdateAssetGroup 更新素材组
func (a *GatewayAssetAdapter) UpdateAssetGroup(groupID string, req UpdateAssetGroupRequest) error {
	path := fmt.Sprintf("/api/seedance/proxy/assets/groups/%s", groupID)
	if a.relayMode {
		path = RewritePathForRelay(path)
	}

	reqBody, _ := common.Marshal(req)
	statusCode, respBody, err := a.doRequest(http.MethodPut, path, nil, reqBody)
	if err != nil {
		return err
	}
	if statusCode < 200 || statusCode >= 300 {
		return fmt.Errorf("upstream error %d: %s", statusCode, string(respBody))
	}
	return nil
}

// DeleteAssetGroup 删除素材组
func (a *GatewayAssetAdapter) DeleteAssetGroup(groupID string) error {
	path := fmt.Sprintf("/api/seedance/proxy/assets/groups/%s", groupID)
	if a.relayMode {
		path = RewritePathForRelay(path)
	}

	statusCode, respBody, err := a.doRequest(http.MethodDelete, path, nil, nil)
	if err != nil {
		return err
	}
	if statusCode < 200 || statusCode >= 300 {
		return fmt.Errorf("upstream error %d: %s", statusCode, string(respBody))
	}
	return nil
}

// CreateAsset 创建素材
func (a *GatewayAssetAdapter) CreateAsset(req CreateAssetRequest) (*AssetResponse, error) {
	path := "/api/seedance/proxy/assets"
	if a.relayMode {
		path = RewritePathForRelay(path)
	}

	reqBody, _ := common.Marshal(req)
	statusCode, respBody, err := a.doRequest(http.MethodPost, path, nil, reqBody)
	if err != nil {
		return nil, err
	}
	if statusCode < 200 || statusCode >= 300 {
		return nil, fmt.Errorf("upstream error %d: %s", statusCode, string(respBody))
	}

	// 解析 Gateway 格式响应：{ "Result": { "Id": "xxx", "GroupId": "xxx", ... } }
	var resp struct {
		Result map[string]interface{} `json:"Result"`
	}
	if err := common.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parse response failed: %w", err)
	}

	assetID, _ := resp.Result["Id"].(string)
	groupID, _ := resp.Result["GroupId"].(string)
	status, _ := resp.Result["Status"].(string)

	return &AssetResponse{
		ID:      assetID,
		GroupID: groupID,
		Status:  status,
		RawData: string(respBody),
	}, nil
}

// GetAsset 查询素材
func (a *GatewayAssetAdapter) GetAsset(assetID string, query url.Values) (*AssetResponse, error) {
	path := fmt.Sprintf("/api/seedance/proxy/assets/%s", assetID)
	if a.relayMode {
		path = RewritePathForRelay(path)
	}

	statusCode, respBody, err := a.doRequest(http.MethodGet, path, query, nil)
	if err != nil {
		return nil, err
	}
	if statusCode < 200 || statusCode >= 300 {
		return nil, fmt.Errorf("upstream error %d: %s", statusCode, string(respBody))
	}

	var resp struct {
		Result struct {
			ID     string `json:"Id"`
			Status string `json:"Status"`
			Name   string `json:"Name"`
		} `json:"Result"`
	}
	if err := common.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parse response failed: %w", err)
	}

	return &AssetResponse{
		ID:      resp.Result.ID,
		Name:    resp.Result.Name,
		Status:  resp.Result.Status,
		RawData: string(respBody),
	}, nil
}

// UpdateAsset 更新素材
func (a *GatewayAssetAdapter) UpdateAsset(assetID string, req UpdateAssetRequest) error {
	path := fmt.Sprintf("/api/seedance/proxy/assets/%s", assetID)
	if a.relayMode {
		path = RewritePathForRelay(path)
	}

	reqBody, _ := common.Marshal(req)
	statusCode, respBody, err := a.doRequest(http.MethodPut, path, nil, reqBody)
	if err != nil {
		return err
	}
	if statusCode < 200 || statusCode >= 300 {
		return fmt.Errorf("upstream error %d: %s", statusCode, string(respBody))
	}
	return nil
}

// DeleteAsset 删除素材
func (a *GatewayAssetAdapter) DeleteAsset(assetID string) error {
	path := fmt.Sprintf("/api/seedance/proxy/assets/%s", assetID)
	if a.relayMode {
		path = RewritePathForRelay(path)
	}

	statusCode, respBody, err := a.doRequest(http.MethodDelete, path, nil, nil)
	if err != nil {
		return err
	}
	if statusCode < 200 || statusCode >= 300 {
		return fmt.Errorf("upstream error %d: %s", statusCode, string(respBody))
	}
	return nil
}

// doRequest 执行 HTTP 请求
func (a *GatewayAssetAdapter) doRequest(method, path string, query url.Values, body []byte) (int, []byte, error) {
	targetURL := a.baseURL + path
	if len(query) > 0 {
		targetURL += "?" + query.Encode()
	}

	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, targetURL, bodyReader)
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+a.token)
	req.Header.Set("Content-Type", "application/json")

	common.SysLog(fmt.Sprintf("gateway adapter: %s %s", method, targetURL))

	client, err := GetHttpClientWithProxy("")
	if err != nil {
		return 0, nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}

	return resp.StatusCode, respBody, nil
}
