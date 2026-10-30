package service

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// Sdos2AssetAdapter 上游 sdos2（第 2 个海外渠道）的素材管理适配器
//
// sdos2 上游即 seedance-gateway（http://106.54.45.168/seedance-gateway）。
// 与第 1 个海外渠道 GatewayAssetAdapter（咪咕 RESTful 风格）的区别：
//   - 路径：/seedance/media、/seedance/media/groups、/seedance/media/upload
//   - 请求：除查询/列表外统一 POST JSON 并按 action 字段区分操作
//   - 字段：小写蛇形（id/asset_uri/media_type/group_id/title）
//   - 状态：queued/running/succeeded/failed，需映射为下游的 Processing/Active/Failed
//
// 契约来源：docs/大尺度sd/CLIENT_API.md、docs/大尺度sd/docs/VIDEO_API.md
type Sdos2AssetAdapter struct {
	baseURL string
	token   string
}

// NewSdos2AssetAdapter 创建 sdos2（seedance-gateway）素材适配器
func NewSdos2AssetAdapter(baseURL, token string) *Sdos2AssetAdapter {
	return &Sdos2AssetAdapter{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
	}
}

// ===== 上游响应结构 =====

// sdos2Group 上游素材组对象
type sdos2Group struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	GroupType   string `json:"group_type"`
	ProjectName string `json:"project_name"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

// sdos2Asset 上游素材对象
type sdos2Asset struct {
	ID        string   `json:"id"`
	Model     string   `json:"model"`
	Status    string   `json:"status"`
	AssetURI  string   `json:"asset_uri"`
	Title     string   `json:"title"`
	GroupID   string   `json:"group_id"`
	Tags      []string `json:"tags"`
	MediaType string   `json:"media_type"`
	Duration  *int     `json:"duration"`
	CreatedAt int64    `json:"created_at"`
	UpdatedAt int64    `json:"updated_at"`
}

// sdos2Error 上游错误体
type sdos2Error struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// ===== 状态映射 =====

// mapSdos2AssetStatus 将上游素材状态映射为下游状态值
//
// 下游沿用 Processing / Active / Failed 三态（见 model.SeedanceAsset.Status 注释），
// 上游种子素材状态为 queued / running / succeeded / failed；
// 只有 succeeded 才可被生成接口引用，因此只有它映射为 Active。
func mapSdos2AssetStatus(upstream string) string {
	switch strings.ToLower(strings.TrimSpace(upstream)) {
	case "succeeded", "success", "active", "completed":
		return "Active"
	case "failed", "error":
		return "Failed"
	case "queued", "running", "processing", "pending", "unknown", "":
		return "Processing"
	default:
		return "Processing"
	}
}

// mapSdos2MediaType 下游素材类型 → 上游 media_type
func mapSdos2MediaType(assetType string) string {
	switch strings.ToLower(strings.TrimSpace(assetType)) {
	case "image", "img", "picture", "photo":
		return "image"
	case "video", "movie":
		return "video"
	case "audio", "sound", "music":
		return "audio"
	default:
		// 上游要求明确类型；缺省按图片处理，与下游常见调用一致
		return "image"
	}
}

// ===== 素材组 =====

// CreateAssetGroup 创建素材组
// 下游: POST /api/seedance/asset-groups {Name, Description}
// 上游: POST /seedance/media/groups {"action":"create","name":..,"description":..}
func (a *Sdos2AssetAdapter) CreateAssetGroup(req CreateAssetGroupRequest) (*AssetGroupResponse, error) {
	body := map[string]interface{}{
		"action": "create",
		"name":   req.Name,
	}
	if req.Description != "" {
		body["description"] = req.Description
	}

	reqBody, err := common.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %w", err)
	}

	statusCode, respBody, err := a.doJSON(http.MethodPost, "/seedance/media/groups", reqBody)
	if err != nil {
		return nil, err
	}
	if statusCode < 200 || statusCode >= 300 {
		return nil, upstreamError("create asset group", statusCode, respBody)
	}

	var g sdos2Group
	if err := common.Unmarshal(respBody, &g); err != nil {
		return nil, fmt.Errorf("parse response failed: %w (body: %s)", err, string(respBody))
	}
	if g.ID == "" {
		return nil, fmt.Errorf("upstream returned empty group id: %s", string(respBody))
	}

	return &AssetGroupResponse{
		ID:          g.ID,
		Name:        g.Name,
		Description: g.Description,
		GroupType:   g.GroupType,
		ProjectName: g.ProjectName,
		RawData:     string(respBody),
	}, nil
}

// GetAssetGroup 查询素材组
// 上游: POST /seedance/media/groups {"action":"get","id":..}
func (a *Sdos2AssetAdapter) GetAssetGroup(groupID string) (*AssetGroupResponse, error) {
	reqBody, _ := common.Marshal(map[string]interface{}{
		"action": "get",
		"id":     groupID,
	})

	statusCode, respBody, err := a.doJSON(http.MethodPost, "/seedance/media/groups", reqBody)
	if err != nil {
		return nil, err
	}
	if statusCode < 200 || statusCode >= 300 {
		return nil, upstreamError("get asset group", statusCode, respBody)
	}

	var g sdos2Group
	if err := common.Unmarshal(respBody, &g); err != nil {
		return nil, fmt.Errorf("parse response failed: %w (body: %s)", err, string(respBody))
	}

	return &AssetGroupResponse{
		ID:          g.ID,
		Name:        g.Name,
		Description: g.Description,
		GroupType:   g.GroupType,
		ProjectName: g.ProjectName,
		RawData:     string(respBody),
	}, nil
}

// UpdateAssetGroup 更新素材组
// 上游: POST /seedance/media/groups {"action":"update","id":..,"name":..,"description":..}
func (a *Sdos2AssetAdapter) UpdateAssetGroup(groupID string, req UpdateAssetGroupRequest) error {
	body := map[string]interface{}{
		"action": "update",
		"id":     groupID,
	}
	if req.Name != "" {
		body["name"] = req.Name
	}
	if req.Description != "" {
		body["description"] = req.Description
	}

	reqBody, _ := common.Marshal(body)
	statusCode, respBody, err := a.doJSON(http.MethodPost, "/seedance/media/groups", reqBody)
	if err != nil {
		return err
	}
	if statusCode < 200 || statusCode >= 300 {
		return upstreamError("update asset group", statusCode, respBody)
	}
	return nil
}

// DeleteAssetGroup 删除素材组
// 上游: POST /seedance/media/groups {"action":"delete","id":..}
func (a *Sdos2AssetAdapter) DeleteAssetGroup(groupID string) error {
	reqBody, _ := common.Marshal(map[string]interface{}{
		"action": "delete",
		"id":     groupID,
	})

	statusCode, respBody, err := a.doJSON(http.MethodPost, "/seedance/media/groups", reqBody)
	if err != nil {
		return err
	}
	if statusCode < 200 || statusCode >= 300 {
		return upstreamError("delete asset group", statusCode, respBody)
	}
	return nil
}

// ===== 素材 =====

// CreateAsset 创建素材（URL 导入）
// 下游: POST /api/seedance/assets {GroupId, URL, AssetType, Name}
// 上游: POST /seedance/media {"action":"import","url":..,"group_id":..,"media_type":..,"title":..}
//
// 注意：上游是异步的，返回 status=queued；须由下游再次查询 GetAsset 拿到 succeeded。
func (a *Sdos2AssetAdapter) CreateAsset(req CreateAssetRequest) (*AssetResponse, error) {
	body := map[string]interface{}{
		"action":     "import",
		"url":        req.URL,
		"media_type": mapSdos2MediaType(req.AssetType),
	}
	if req.GroupID != "" {
		body["group_id"] = req.GroupID
	}
	if req.Name != "" {
		body["title"] = req.Name
	}

	reqBody, err := common.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %w", err)
	}

	statusCode, respBody, err := a.doJSON(http.MethodPost, "/seedance/media", reqBody)
	if err != nil {
		return nil, err
	}
	if statusCode < 200 || statusCode >= 300 {
		return nil, upstreamError("create asset", statusCode, respBody)
	}

	var asset sdos2Asset
	if err := common.Unmarshal(respBody, &asset); err != nil {
		return nil, fmt.Errorf("parse response failed: %w (body: %s)", err, string(respBody))
	}
	if asset.ID == "" {
		return nil, fmt.Errorf("upstream returned empty asset id: %s", string(respBody))
	}

	return toAssetResponse(asset, string(respBody), req), nil
}

// GetAsset 查询素材
// 上游: POST /seedance/media {"action":"get","id":..}
func (a *Sdos2AssetAdapter) GetAsset(assetID string, query url.Values) (*AssetResponse, error) {
	reqBody, _ := common.Marshal(map[string]interface{}{
		"action": "get",
		"id":     assetID,
	})

	statusCode, respBody, err := a.doJSON(http.MethodPost, "/seedance/media", reqBody)
	if err != nil {
		return nil, err
	}
	if statusCode < 200 || statusCode >= 300 {
		return nil, upstreamError("get asset", statusCode, respBody)
	}

	var asset sdos2Asset
	if err := common.Unmarshal(respBody, &asset); err != nil {
		return nil, fmt.Errorf("parse response failed: %w (body: %s)", err, string(respBody))
	}

	return toAssetResponse(asset, string(respBody), CreateAssetRequest{}), nil
}

// UpdateAsset 更新素材
// 上游: POST /seedance/media {"action":"update","id":..,"title":..}
func (a *Sdos2AssetAdapter) UpdateAsset(assetID string, req UpdateAssetRequest) error {
	body := map[string]interface{}{
		"action": "update",
		"id":     assetID,
	}
	if req.Name != "" {
		body["title"] = req.Name
	}

	reqBody, _ := common.Marshal(body)
	statusCode, respBody, err := a.doJSON(http.MethodPost, "/seedance/media", reqBody)
	if err != nil {
		return err
	}
	if statusCode < 200 || statusCode >= 300 {
		return upstreamError("update asset", statusCode, respBody)
	}
	return nil
}

// DeleteAsset 删除素材
// 上游: POST /seedance/media {"action":"delete","ids":[..]}（上游按数组接收）
func (a *Sdos2AssetAdapter) DeleteAsset(assetID string) error {
	reqBody, _ := common.Marshal(map[string]interface{}{
		"action": "delete",
		"ids":    []string{assetID},
	})

	statusCode, respBody, err := a.doJSON(http.MethodPost, "/seedance/media", reqBody)
	if err != nil {
		return err
	}
	if statusCode < 200 || statusCode >= 300 {
		return upstreamError("delete asset", statusCode, respBody)
	}
	return nil
}

// ===== 内部工具 =====

// toAssetResponse 上游素材对象 → 下游统一响应
//
// 关键映射：
//   - URL 字段返回 asset_uri（asset://task_xxx），这是上游生成接口唯一认可的引用形式
//   - Status 做三态归一，下游依赖 Status == "Active" 判断素材可用
func toAssetResponse(asset sdos2Asset, rawBody string, req CreateAssetRequest) *AssetResponse {
	assetRef := asset.AssetURI
	if assetRef == "" && asset.ID != "" {
		assetRef = "asset://" + asset.ID
	}

	name := asset.Title
	if name == "" {
		name = req.Name
	}
	groupID := asset.GroupID
	if groupID == "" {
		groupID = req.GroupID
	}
	mediaType := asset.MediaType
	if mediaType == "" {
		mediaType = mapSdos2MediaType(req.AssetType)
	}

	return &AssetResponse{
		ID:         asset.ID,
		GroupID:    groupID,
		Name:       name,
		AssetType:  mediaType,
		Status:     mapSdos2AssetStatus(asset.Status),
		SourceURL:  req.URL,
		URL:        assetRef,
		Moderation: nil,
		RawData:    rawBody,
	}
}

// doJSON 发送 JSON 请求到上游
func (a *Sdos2AssetAdapter) doJSON(method, path string, body []byte) (int, []byte, error) {
	targetURL := a.baseURL + path

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
	req.Header.Set("Accept", "application/json")

	common.SysLog(fmt.Sprintf("sdos2 asset adapter: %s %s", method, targetURL))

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

// upstreamError 构造带上游错误码的错误
func upstreamError(op string, statusCode int, body []byte) error {
	var e sdos2Error
	if err := common.Unmarshal(body, &e); err == nil && e.Error.Code != "" {
		return fmt.Errorf("%s failed: upstream %d %s: %s", op, statusCode, e.Error.Code, e.Error.Message)
	}
	return fmt.Errorf("%s failed: upstream %d: %s", op, statusCode, string(body))
}
