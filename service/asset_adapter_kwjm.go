package service

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/QuantumNous/new-api/common"
)

// KwjmAssetAdapter KWJM 上游的素材管理适配器
type KwjmAssetAdapter struct {
	baseURL string
	token   string
	model   string // KWJM 要求所有请求带 model 参数
}

// NewKwjmAssetAdapter 创建 KWJM 适配器
func NewKwjmAssetAdapter(baseURL, token, model string) *KwjmAssetAdapter {
	return &KwjmAssetAdapter{
		baseURL: baseURL,
		token:   token,
		model:   model,
	}
}

// CreateAssetGroup 创建素材组
func (a *KwjmAssetAdapter) CreateAssetGroup(req CreateAssetGroupRequest) (*AssetGroupResponse, error) {
	// KWJM 使用 Action 格式
	action := "CreateAssetGroup"
	path := "/v3/open/" + action

	// 注入 model 参数
	reqBody := map[string]interface{}{
		"model":       a.model,
		"Name":        req.Name,
		"Description": req.Description,
		"GroupType":   req.GroupType,
	}

	body, _ := common.Marshal(reqBody)
	statusCode, respBody, err := a.doRequest(http.MethodPost, path, nil, body)
	if err != nil {
		return nil, err
	}
	if statusCode < 200 || statusCode >= 300 {
		return nil, fmt.Errorf("upstream error %d: %s", statusCode, string(respBody))
	}

	// 解析 KWJM 格式响应：{ "Id": "xxx" }（只有 Id）
	var resp struct {
		ID string `json:"Id"`
	}
	if err := common.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parse response failed: %w", err)
	}

	// KWJM 只返回 Id，其他字段从请求中获取
	return &AssetGroupResponse{
		ID:          resp.ID,
		Name:        req.Name,
		Description: req.Description,
		GroupType:   req.GroupType,
		RawData:     string(respBody),
	}, nil
}

// GetAssetGroup 查询素材组
func (a *KwjmAssetAdapter) GetAssetGroup(groupID string) (*AssetGroupResponse, error) {
	action := "GetAssetGroup"
	path := "/v3/open/" + action

	reqBody := map[string]interface{}{
		"model": a.model,
		"Id":    groupID,
	}

	body, _ := common.Marshal(reqBody)
	statusCode, respBody, err := a.doRequest(http.MethodPost, path, nil, body)
	if err != nil {
		return nil, err
	}
	if statusCode < 200 || statusCode >= 300 {
		return nil, fmt.Errorf("upstream error %d: %s", statusCode, string(respBody))
	}

	// KWJM 查询接口返回完整信息
	var resp struct {
		ID          string `json:"Id"`
		Name        string `json:"Name"`
		Description string `json:"Description"`
		GroupType   string `json:"GroupType"`
	}
	if err := common.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parse response failed: %w", err)
	}

	return &AssetGroupResponse{
		ID:          resp.ID,
		Name:        resp.Name,
		Description: resp.Description,
		GroupType:   resp.GroupType,
		RawData:     string(respBody),
	}, nil
}

// UpdateAssetGroup 更新素材组
func (a *KwjmAssetAdapter) UpdateAssetGroup(groupID string, req UpdateAssetGroupRequest) error {
	action := "UpdateAssetGroup"
	path := "/v3/open/" + action

	reqBody := map[string]interface{}{
		"model": a.model,
		"Id":    groupID,
	}
	if req.Name != "" {
		reqBody["Name"] = req.Name
	}
	if req.Description != "" {
		reqBody["Description"] = req.Description
	}

	body, _ := common.Marshal(reqBody)
	statusCode, respBody, err := a.doRequest(http.MethodPost, path, nil, body)
	if err != nil {
		return err
	}
	if statusCode < 200 || statusCode >= 300 {
		return fmt.Errorf("upstream error %d: %s", statusCode, string(respBody))
	}
	return nil
}

// DeleteAssetGroup 删除素材组
func (a *KwjmAssetAdapter) DeleteAssetGroup(groupID string) error {
	action := "DeleteAssetGroup"
	path := "/v3/open/" + action

	reqBody := map[string]interface{}{
		"model": a.model,
		"Id":    groupID,
	}

	body, _ := common.Marshal(reqBody)
	statusCode, respBody, err := a.doRequest(http.MethodPost, path, nil, body)
	if err != nil {
		return err
	}
	if statusCode < 200 || statusCode >= 300 {
		return fmt.Errorf("upstream error %d: %s", statusCode, string(respBody))
	}
	return nil
}

// CreateAsset 创建素材
func (a *KwjmAssetAdapter) CreateAsset(req CreateAssetRequest) (*AssetResponse, error) {
	action := "CreateAsset"
	path := "/v3/open/" + action

	// 注入 model 参数
	reqBody := map[string]interface{}{
		"model":     a.model,
		"GroupId":   req.GroupID,
		"URL":       req.URL,
		"AssetType": req.AssetType,
		"Name":      req.Name,
	}

	body, _ := common.Marshal(reqBody)
	statusCode, respBody, err := a.doRequest(http.MethodPost, path, nil, body)
	if err != nil {
		return nil, err
	}
	if statusCode < 200 || statusCode >= 300 {
		return nil, fmt.Errorf("upstream error %d: %s", statusCode, string(respBody))
	}

	// 解析 KWJM 格式响应：{ "Id": "xxx" }（只有 Id）
	var resp struct {
		ID string `json:"Id"`
	}
	if err := common.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parse response failed: %w", err)
	}

	// KWJM 只返回 Id，其他字段从请求中获取
	return &AssetResponse{
		ID:        resp.ID,
		GroupID:   req.GroupID,
		Name:      req.Name,
		AssetType: req.AssetType,
		Status:    "Processing", // 默认状态
		SourceURL: req.URL,
		RawData:   string(respBody),
	}, nil
}

// GetAsset 查询素材
func (a *KwjmAssetAdapter) GetAsset(assetID string, query url.Values) (*AssetResponse, error) {
	action := "GetAsset"
	path := "/v3/open/" + action

	reqBody := map[string]interface{}{
		"model": a.model,
		"Id":    assetID,
	}

	body, _ := common.Marshal(reqBody)
	statusCode, respBody, err := a.doRequest(http.MethodPost, path, nil, body)
	if err != nil {
		return nil, err
	}
	if statusCode < 200 || statusCode >= 300 {
		return nil, fmt.Errorf("upstream error %d: %s", statusCode, string(respBody))
	}

	// KWJM 查询接口返回完整信息
	var resp struct {
		ID        string `json:"Id"`
		Name      string `json:"Name"`
		Status    string `json:"Status"`
		GroupID   string `json:"GroupId"`
		AssetType string `json:"AssetType"`
	}
	if err := common.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parse response failed: %w", err)
	}

	return &AssetResponse{
		ID:        resp.ID,
		Name:      resp.Name,
		Status:    resp.Status,
		GroupID:   resp.GroupID,
		AssetType: resp.AssetType,
		RawData:   string(respBody),
	}, nil
}

// UpdateAsset 更新素材
func (a *KwjmAssetAdapter) UpdateAsset(assetID string, req UpdateAssetRequest) error {
	action := "UpdateAsset"
	path := "/v3/open/" + action

	reqBody := map[string]interface{}{
		"model": a.model,
		"Id":    assetID,
	}
	if req.Name != "" {
		reqBody["Name"] = req.Name
	}

	body, _ := common.Marshal(reqBody)
	statusCode, respBody, err := a.doRequest(http.MethodPost, path, nil, body)
	if err != nil {
		return err
	}
	if statusCode < 200 || statusCode >= 300 {
		return fmt.Errorf("upstream error %d: %s", statusCode, string(respBody))
	}
	return nil
}

// DeleteAsset 删除素材
func (a *KwjmAssetAdapter) DeleteAsset(assetID string) error {
	action := "DeleteAsset"
	path := "/v3/open/" + action

	reqBody := map[string]interface{}{
		"model": a.model,
		"Id":    assetID,
	}

	body, _ := common.Marshal(reqBody)
	statusCode, respBody, err := a.doRequest(http.MethodPost, path, nil, body)
	if err != nil {
		return err
	}
	if statusCode < 200 || statusCode >= 300 {
		return fmt.Errorf("upstream error %d: %s", statusCode, string(respBody))
	}
	return nil
}

// doRequest 执行 HTTP 请求
func (a *KwjmAssetAdapter) doRequest(method, path string, query url.Values, body []byte) (int, []byte, error) {
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

	common.SysLog(fmt.Sprintf("kwjm adapter: %s %s", method, targetURL))

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
