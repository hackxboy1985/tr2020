package service

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// UpstreamVersionNaxis 是 n-axis 平台（原咪咕 Seedance Gateway 重构版）的素材库契约标识。
//
// 渠道 other 配置里写 asset_upstream_version: "naxis" 即启用火山方舟原生 Action
// 适配器（NaxisActionAssetAdapter，POST /?Action=…&Version=2024-01-01），与官方 SDK 同构；
// 写 "naxis_rest" 则使用平台 REST 适配器（本文件的 NaxisAssetAdapter，/v1/asset-groups、
// /v1/assets）。两者共享同一份上游数据，上游地址复用 seedance_asset_base_url
// （如 https://platform.n-axis.com）。
//
// 与老咪咕 Gateway 适配器（GatewayAssetAdapter，/api/seedance/proxy/assets*）相比，
// n-axis 是两个全新契约；老适配器保持原样，按渠道配置独立选择，互不影响。
//
// 契约来源：https://platform.n-axis.com/docs/api/assets.html
const UpstreamVersionNaxis = "naxis"

// NaxisAssetAdapter n-axis 素材库适配器
type NaxisAssetAdapter struct {
	baseURL string
	token   string
}

// NewNaxisAssetAdapter 创建 n-axis 素材库适配器
func NewNaxisAssetAdapter(baseURL, token string) *NaxisAssetAdapter {
	return &NaxisAssetAdapter{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
	}
}

// ===== 上游响应对象（小写蛇形） =====

// naxisGroup 上游素材组对象
type naxisGroup struct {
	ID            string `json:"id"`
	Object        string `json:"object"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	GroupType     string `json:"group_type"`
	ProjectName   string `json:"project_name"`
	ResourceOwner string `json:"resource_owner"`
	Status        string `json:"status"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// naxisAsset 上游素材对象
type naxisAsset struct {
	ID                string  `json:"id"`
	Object            string  `json:"object"`
	AssetType         string  `json:"asset_type"`
	GroupID           string  `json:"group_id"`
	ProjectName       string  `json:"project_name"`
	ResourceOwner     string  `json:"resource_owner"`
	Name              string  `json:"name"`
	SourceURL         string  `json:"source_url"`
	Status            string  `json:"status"`
	Error             *string `json:"error"`
	LastInferenceTime *string `json:"last_inference_time"`
	CreatedAt         string  `json:"created_at"`
	UpdatedAt         string  `json:"updated_at"`
}

// ===== 字段映射 =====

// mapNaxisAssetStatus 上游小写状态 → 下游沿用的大写三态 + Archived。
//
// 下游（controller / model.SeedanceAsset.Status）沿用 Active / Processing /
// Failed，且「同 URL 复用」判断是硬比较 Status == "Active"，因此必须在这里归一，
// 不能把 active 原样落库。
func mapNaxisAssetStatus(upstream string) string {
	switch strings.ToLower(strings.TrimSpace(upstream)) {
	case "active":
		return "Active"
	case "processing":
		return "Processing"
	case "failed":
		return "Failed"
	case "archived":
		return "Archived"
	default:
		return strings.TrimSpace(upstream)
	}
}

// mapNaxisAssetType 下游素材类型 → 上游小写 asset_type（缺省 image，与上游默认一致）
func mapNaxisAssetType(assetType string) string {
	switch strings.ToLower(strings.TrimSpace(assetType)) {
	case "video":
		return "video"
	case "audio":
		return "audio"
	default:
		return "image"
	}
}

// toResponse 素材组对象 → 统一响应
func (g *naxisGroup) toResponse(raw string) *AssetGroupResponse {
	return &AssetGroupResponse{
		ID:          g.ID,
		Name:        g.Name,
		Description: g.Description,
		GroupType:   g.GroupType,
		ProjectName: g.ProjectName,
		CreateTime:  g.CreatedAt,
		UpdateTime:  g.UpdatedAt,
		RawData:     raw,
	}
}

// toResponse 素材对象 → 统一响应（URL 同时写入 SourceURL，下游拿 URL 展示）
func (a *naxisAsset) toResponse(raw string) *AssetResponse {
	return &AssetResponse{
		ID:         a.ID,
		GroupID:    a.GroupID,
		Name:       a.Name,
		AssetType:  a.AssetType,
		Status:     mapNaxisAssetStatus(a.Status),
		SourceURL:  a.SourceURL,
		URL:        a.SourceURL,
		CreateTime: a.CreatedAt,
		UpdateTime: a.UpdatedAt,
		RawData:    raw,
	}
}

// ===== 素材组 =====

// CreateAssetGroup 创建素材组
// 下游: POST /api/seedance/asset-groups
// 上游: POST /v1/asset-groups → 201 + 裸对象
func (a *NaxisAssetAdapter) CreateAssetGroup(req CreateAssetGroupRequest) (*AssetGroupResponse, error) {
	payload := map[string]interface{}{"name": req.Name}
	if req.Description != "" {
		payload["description"] = req.Description
	}
	if req.GroupType != "" {
		// 仅 AIGC 合法；LivenessFace 上游直接 400，错误原样透出
		payload["group_type"] = req.GroupType
	}
	if req.ProjectName != "" {
		payload["project_name"] = req.ProjectName
	}

	statusCode, respBody, err := a.doRequest(http.MethodPost, "/v1/asset-groups", nil, payload)
	if err != nil {
		return nil, err
	}
	if statusCode < 200 || statusCode >= 300 {
		return nil, naxisUpstreamError(statusCode, respBody)
	}

	var g naxisGroup
	if err := common.Unmarshal(respBody, &g); err != nil {
		return nil, fmt.Errorf("naxis parse asset group response failed: %w", err)
	}
	if g.ID == "" {
		// 上游契约变化最典型的表现：解析成功但字段为空，必须显式失败，
		// 否则会往本地表写入 upstream_group_id="" 的脏记录。
		return nil, fmt.Errorf("naxis create asset group returned empty id: %s", truncateNaxisBody(respBody))
	}
	return g.toResponse(string(respBody)), nil
}

// GetAssetGroup 查询素材组
// 上游: GET /v1/asset-groups/{id} → 裸对象（不存在 404 {"error":"asset group not found"}）
func (a *NaxisAssetAdapter) GetAssetGroup(groupID string) (*AssetGroupResponse, error) {
	path := "/v1/asset-groups/" + url.PathEscape(groupID)
	statusCode, respBody, err := a.doRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	if statusCode < 200 || statusCode >= 300 {
		return nil, naxisUpstreamError(statusCode, respBody)
	}

	var g naxisGroup
	if err := common.Unmarshal(respBody, &g); err != nil {
		return nil, fmt.Errorf("naxis parse asset group response failed: %w", err)
	}
	if g.ID == "" {
		return nil, fmt.Errorf("naxis get asset group returned empty id: %s", truncateNaxisBody(respBody))
	}
	return g.toResponse(string(respBody)), nil
}

// UpdateAssetGroup 更新素材组
// 上游: PATCH /v1/asset-groups/{id}（只允许 name / description，group_type 与 project_name 不可改）
func (a *NaxisAssetAdapter) UpdateAssetGroup(groupID string, req UpdateAssetGroupRequest) error {
	payload := map[string]interface{}{}
	if req.Name != "" {
		payload["name"] = req.Name
	}
	if req.Description != "" {
		payload["description"] = req.Description
	}

	path := "/v1/asset-groups/" + url.PathEscape(groupID)
	statusCode, respBody, err := a.doRequest(http.MethodPatch, path, nil, payload)
	if err != nil {
		return err
	}
	if statusCode < 200 || statusCode >= 300 {
		return naxisUpstreamError(statusCode, respBody)
	}
	return nil
}

// DeleteAssetGroup 删除素材组
// 上游: DELETE /v1/asset-groups/{id} → {"ok":true}；默认级联归档组内素材
func (a *NaxisAssetAdapter) DeleteAssetGroup(groupID string) error {
	path := "/v1/asset-groups/" + url.PathEscape(groupID)
	statusCode, respBody, err := a.doRequest(http.MethodDelete, path, nil, nil)
	if err != nil {
		return err
	}
	if statusCode < 200 || statusCode >= 300 {
		return naxisUpstreamError(statusCode, respBody)
	}
	return nil
}

// ===== 素材 =====

// CreateAsset 注册素材
// 下游: POST /api/seedance/assets
// 上游: POST /v1/assets → 201 + 裸对象；同账号同组同类型同 URL 重复注册返回 200 + asset-replayed 头
func (a *NaxisAssetAdapter) CreateAsset(req CreateAssetRequest) (*AssetResponse, error) {
	payload := map[string]interface{}{
		"url":        req.URL,
		"asset_type": mapNaxisAssetType(req.AssetType),
	}
	if req.Name != "" {
		payload["name"] = req.Name
	}
	if req.GroupID != "" {
		payload["group_id"] = req.GroupID
	}
	if req.ProjectName != "" {
		payload["project_name"] = req.ProjectName
	}

	statusCode, respBody, err := a.doRequest(http.MethodPost, "/v1/assets", nil, payload)
	if err != nil {
		return nil, err
	}
	if statusCode < 200 || statusCode >= 300 {
		return nil, naxisUpstreamError(statusCode, respBody)
	}

	var asset naxisAsset
	if err := common.Unmarshal(respBody, &asset); err != nil {
		return nil, fmt.Errorf("naxis parse asset response failed: %w", err)
	}
	if asset.ID == "" {
		return nil, fmt.Errorf("naxis create asset returned empty id: %s", truncateNaxisBody(respBody))
	}
	if asset.GroupID == "" {
		asset.GroupID = req.GroupID
	}
	return asset.toResponse(string(respBody)), nil
}

// GetAsset 查询素材
// 上游: GET /v1/assets/{id} → 裸对象；已归档素材 REST 仍返回 200 + status=archived
func (a *NaxisAssetAdapter) GetAsset(assetID string, query url.Values) (*AssetResponse, error) {
	path := "/v1/assets/" + url.PathEscape(assetID)
	statusCode, respBody, err := a.doRequest(http.MethodGet, path, query, nil)
	if err != nil {
		return nil, err
	}
	if statusCode < 200 || statusCode >= 300 {
		return nil, naxisUpstreamError(statusCode, respBody)
	}

	var asset naxisAsset
	if err := common.Unmarshal(respBody, &asset); err != nil {
		return nil, fmt.Errorf("naxis parse asset response failed: %w", err)
	}
	if asset.ID == "" {
		return nil, fmt.Errorf("naxis get asset returned empty id: %s", truncateNaxisBody(respBody))
	}
	return asset.toResponse(string(respBody)), nil
}

// UpdateAsset 更新素材
// 上游: PATCH /v1/assets/{id}（只允许改 name）
func (a *NaxisAssetAdapter) UpdateAsset(assetID string, req UpdateAssetRequest) error {
	payload := map[string]interface{}{}
	if req.Name != "" {
		payload["name"] = req.Name
	}

	path := "/v1/assets/" + url.PathEscape(assetID)
	statusCode, respBody, err := a.doRequest(http.MethodPatch, path, nil, payload)
	if err != nil {
		return err
	}
	if statusCode < 200 || statusCode >= 300 {
		return naxisUpstreamError(statusCode, respBody)
	}
	return nil
}

// DeleteAsset 删除素材（上游归档软删）
// 上游: DELETE /v1/assets/{id} → {"ok":true}；重复删除返回 404
func (a *NaxisAssetAdapter) DeleteAsset(assetID string) error {
	path := "/v1/assets/" + url.PathEscape(assetID)
	statusCode, respBody, err := a.doRequest(http.MethodDelete, path, nil, nil)
	if err != nil {
		return err
	}
	if statusCode < 200 || statusCode >= 300 {
		return naxisUpstreamError(statusCode, respBody)
	}
	return nil
}

// ===== 真人核验（n-axis: /v1/visual-validate/sessions）=====

// naxisVisualValidateSession 创建核验会话的响应。
// 平台 REST 同时返回 snake_case 与 PascalCase 两套键（对齐火山官方 SDK），两套都解析。
type naxisVisualValidateSession struct {
	BytedToken       string `json:"byted_token"`
	BytedTokenPascal string `json:"BytedToken"`
	H5Link           string `json:"h5_link"`
	H5LinkPascal     string `json:"H5Link"`
	CallbackURL      string `json:"callback_url"`
	ExpireTime       string `json:"expire_time"`
	ExpireTimePascal string `json:"ExpireTime"`
}

// naxisVisualValidateResult 查询核验结果的响应（REST 平铺 / 兼容官方 PascalCase）。
type naxisVisualValidateResult struct {
	Status           string `json:"status"`
	StatusPascal     string `json:"Status"`
	GroupID          string `json:"group_id"`
	GroupIDPascal    string `json:"GroupId"`
	Passed           bool   `json:"passed"`
	AssetID          string `json:"asset_id"`
	Reason           string `json:"reason"`
	ExpireTime       string `json:"expire_time"`
	ExpireTimePascal string `json:"ExpireTime"`
}

// CreateFaceVerification 创建真人核验会话
// 上游: POST /v1/visual-validate/sessions（body 可空）→ {byted_token, h5_link, expire_time, ...}
func (a *NaxisAssetAdapter) CreateFaceVerification(callbackURL string) (*FaceVerificationSession, error) {
	payload := map[string]interface{}{}
	if strings.TrimSpace(callbackURL) != "" {
		payload["callback_url"] = strings.TrimSpace(callbackURL)
	}

	statusCode, respBody, err := a.doRequest(http.MethodPost, "/v1/visual-validate/sessions", nil, payload)
	if err != nil {
		return nil, err
	}
	if statusCode < 200 || statusCode >= 300 {
		return nil, naxisUpstreamError(statusCode, respBody)
	}

	var session naxisVisualValidateSession
	if err := common.Unmarshal(respBody, &session); err != nil {
		return nil, fmt.Errorf("naxis parse visual validate session failed: %w", err)
	}

	token := firstNonEmptyNaxis(session.BytedToken, session.BytedTokenPascal)
	if token == "" {
		return nil, fmt.Errorf("naxis create visual validate session returned empty byted token: %s", truncateNaxisBody(respBody))
	}

	return &FaceVerificationSession{
		Token:     token,
		H5URL:     firstNonEmptyNaxis(session.H5Link, session.H5LinkPascal),
		ExpiresAt: parseNaxisISOTime(firstNonEmptyNaxis(session.ExpireTime, session.ExpireTimePascal)),
		RawData:   string(respBody),
	}, nil
}

// GetFaceVerification 查询真人核验结果
// 上游: GET /v1/visual-validate/sessions/{token}
// 上游状态 waiting_user / callback_received / verified / failed / expired 与下游既有口径一致。
func (a *NaxisAssetAdapter) GetFaceVerification(token string) (*FaceVerificationResult, error) {
	path := "/v1/visual-validate/sessions/" + url.PathEscape(token)
	statusCode, respBody, err := a.doRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	if statusCode < 200 || statusCode >= 300 {
		return nil, naxisUpstreamError(statusCode, respBody)
	}

	var result naxisVisualValidateResult
	if err := common.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("naxis parse visual validate result failed: %w", err)
	}

	status := firstNonEmptyNaxis(result.Status, result.StatusPascal)
	if status == "" {
		return nil, fmt.Errorf("naxis get visual validate result returned empty status: %s", truncateNaxisBody(respBody))
	}

	return &FaceVerificationResult{
		Status:     status,
		GroupID:    firstNonEmptyNaxis(result.GroupID, result.GroupIDPascal),
		Passed:     result.Passed,
		AssetID:    result.AssetID,
		Reason:     result.Reason,
		ExpireTime: firstNonEmptyNaxis(result.ExpireTime, result.ExpireTimePascal),
		RawData:    string(respBody),
	}, nil
}

// firstNonEmptyNaxis 返回第一个非空（去空白）值
func firstNonEmptyNaxis(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

// parseNaxisISOTime 解析上游 ISO 8601 时间为 unix 秒（失败返回 0）
func parseNaxisISOTime(raw string) int64 {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0
	}
	// Go 的 time.Parse 允许输入带小数秒（布局可不写），RFC3339 足够覆盖
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Unix()
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.Unix()
	}
	return 0
}

// ===== 基础设施 =====

// doRequest 执行一次 n-axis 请求（平台 REST 入口）。
func (a *NaxisAssetAdapter) doRequest(method, path string, query url.Values, payload interface{}) (int, []byte, error) {
	return naxisDoRequest(a.baseURL, a.token, method, path, query, payload)
}

// naxisDoRequest 执行一次 n-axis 请求：Bearer sk- 鉴权、JSON 收发。
// REST（/v1/*）与 Action（/?Action=…）两个适配器共用。
func naxisDoRequest(baseURL, token, method, path string, query url.Values, payload interface{}) (int, []byte, error) {
	targetURL := baseURL + path
	if len(query) > 0 {
		targetURL += "?" + query.Encode()
	}

	var bodyReader io.Reader
	if payload != nil {
		body, err := common.Marshal(payload)
		if err != nil {
			return 0, nil, fmt.Errorf("naxis marshal request failed: %w", err)
		}
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, targetURL, bodyReader)
	if err != nil {
		return 0, nil, fmt.Errorf("naxis build request failed: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client, err := GetHttpClientWithProxy("")
	if err != nil {
		return 0, nil, err
	}
	if client == nil {
		// 单元测试等未初始化全局客户端时兜底，避免 nil pointer panic
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		common.SysError(fmt.Sprintf("naxis adapter request failed: %s %s: %v", method, targetURL, err))
		return 0, nil, fmt.Errorf("naxis do request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("naxis read response failed: %w", err)
	}

	common.SysLog(fmt.Sprintf("naxis adapter: %s %s -> %d", method, targetURL, resp.StatusCode))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		common.SysError(fmt.Sprintf("naxis adapter upstream error: %s %s -> %d: %s",
			method, targetURL, resp.StatusCode, truncateNaxisBody(respBody)))
	}
	return resp.StatusCode, respBody, nil
}

// naxisUpstreamError 把 n-axis 三种错误体解析成可读错误：
//  1. REST: {"error":"asset group not found"}
//  2. REST 结构体错误: {"error":{"code":"...","message":"..."}}
//  3. TOP: {"ResponseMetadata":{"Error":{"Code":"...","Message":"..."}}}
func naxisUpstreamError(statusCode int, respBody []byte) error {
	var restErr struct {
		Error string `json:"error"`
	}
	if common.Unmarshal(respBody, &restErr) == nil && strings.TrimSpace(restErr.Error) != "" {
		return fmt.Errorf("naxis upstream error %d: %s", statusCode, restErr.Error)
	}

	var structErr struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if common.Unmarshal(respBody, &structErr) == nil && structErr.Error.Message != "" {
		if structErr.Error.Code != "" {
			return fmt.Errorf("naxis upstream error %d: %s: %s", statusCode, structErr.Error.Code, structErr.Error.Message)
		}
		return fmt.Errorf("naxis upstream error %d: %s", statusCode, structErr.Error.Message)
	}

	var topErr struct {
		ResponseMetadata struct {
			Error struct {
				Code    string `json:"Code"`
				Message string `json:"Message"`
			} `json:"Error"`
		} `json:"ResponseMetadata"`
	}
	if common.Unmarshal(respBody, &topErr) == nil && topErr.ResponseMetadata.Error.Message != "" {
		code := topErr.ResponseMetadata.Error.Code
		if code != "" {
			return fmt.Errorf("naxis upstream error %d: %s: %s", statusCode, code, topErr.ResponseMetadata.Error.Message)
		}
		return fmt.Errorf("naxis upstream error %d: %s", statusCode, topErr.ResponseMetadata.Error.Message)
	}

	return fmt.Errorf("naxis upstream error %d: %s", statusCode, truncateNaxisBody(respBody))
}

// truncateNaxisBody 截断日志/错误里的响应体，避免超长输出
func truncateNaxisBody(body []byte) string {
	const maxLen = 512
	s := string(body)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
