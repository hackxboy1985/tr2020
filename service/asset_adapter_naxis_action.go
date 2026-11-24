package service

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// UpstreamVersionNaxisRest 是 n-axis 的「平台 REST」入口版本标识。
//
// 两个版本指向同一份上游数据，差异只在协议壳：
//   - naxis（默认）：火山方舟原生 Action 入口 POST /?Action=…&Version=2024-01-01
//   - naxis_rest：平台 REST 入口 /v1/asset-groups、/v1/assets
//
// Action 入口与火山官方 SDK 同构（ResponseMetadata + Result、PascalCase、状态
// Active/Processing/Failed），但如果需要素材的原始 source_url 请用 naxis_rest：
// Action 入口的 GetAsset/ListAssets 返回的 URL 是平台签名代理地址（带 expires/sig）。
const UpstreamVersionNaxisRest = "naxis_rest"

// naxisActionVersion 火山方舟 Action 的固定版本号
const naxisActionVersion = "2024-01-01"

// NaxisActionAssetAdapter 是 n-axis 的火山方舟原生 Action 适配器。
//
// 与 NaxisAssetAdapter（平台 REST）实现同一套 AssetAdapter 接口，由渠道配置
// asset_upstream_version 选择。下游两套对外接口（Action 与 /api/seedance/*）
// 完全不感知协议差异。
type NaxisActionAssetAdapter struct {
	baseURL string
	token   string
	// actionPath 是 Action 入口的路径前缀。
	// 空字符串（默认）表示入口在域名根路径，即 POST /?Action=…&Version=…（n-axis 形态）；
	// 非空时（如 "/api/material"）拼成 POST {baseURL}/api/material?Action=…&Version=…。
	actionPath string
}

// NewNaxisActionAssetAdapter 创建 n-axis Action（火山方舟原生）适配器。
// 默认 Action 入口在域名根路径，与历史行为完全一致（n-axis 渠道无需改配置）。
func NewNaxisActionAssetAdapter(baseURL, token string) *NaxisActionAssetAdapter {
	return NewNaxisActionAssetAdapterWithPath(baseURL, token, "")
}

// NewNaxisActionAssetAdapterWithPath 创建 Action 适配器并指定入口路径前缀。
// actionPath 为空时等价于 NewNaxisActionAssetAdapter（根路径入口）。
func NewNaxisActionAssetAdapterWithPath(baseURL, token, actionPath string) *NaxisActionAssetAdapter {
	return &NaxisActionAssetAdapter{
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      token,
		actionPath: normalizeActionPath(actionPath),
	}
}

// normalizeActionPath 归一化 Action 入口前缀：空→""（根路径）；否则补前导 "/" 并去掉尾部 "/"。
//
// 之所以要单独归一化，是因为 URL 拼接用的是 baseURL+actionPath+"?"+query：
// 若前缀带尾斜杠会拼出 "/api/material?" 之外的 "/api/material/?"，多数上游会 404。
func normalizeActionPath(p string) string {
	trimmed := strings.TrimSpace(p)
	if trimmed == "" || trimmed == "/" {
		return ""
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	return strings.TrimRight(trimmed, "/")
}

// ===== 上游响应结构（PascalCase + Result 外壳）=====

type naxisActionIDResult struct {
	Id string `json:"Id"`
}

type naxisActionGroupResult struct {
	Id          string `json:"Id"`
	Name        string `json:"Name"`
	Description string `json:"Description"`
	GroupType   string `json:"GroupType"`
	ProjectName string `json:"ProjectName"`
	CreateTime  string `json:"CreateTime"`
	UpdateTime  string `json:"UpdateTime"`
}

type naxisActionAssetResult struct {
	Id                string      `json:"Id"`
	Name              string      `json:"Name"`
	URL               string      `json:"URL"`
	AssetType         string      `json:"AssetType"`
	GroupId           string      `json:"GroupId"`
	ProjectName       string      `json:"ProjectName"`
	Status            string      `json:"Status"`
	Moderation        interface{} `json:"Moderation"`
	LastInferenceTime *string     `json:"LastInferenceTime"`
	CreateTime        string      `json:"CreateTime"`
	UpdateTime        string      `json:"UpdateTime"`
}

type naxisActionSessionResult struct {
	BytedToken  string `json:"BytedToken"`
	CallbackURL string `json:"CallbackURL"`
	H5Link      string `json:"H5Link"`
}

type naxisActionValidateResult struct {
	GroupId    string `json:"GroupId"`
	Status     string `json:"Status"`
	Passed     bool   `json:"Passed"`
	AssetId    string `json:"AssetId"`
	Reason     string `json:"Reason"`
	ExpireTime string `json:"ExpireTime"`

	// 平台在 Result 里额外混入的 snake_case 扩展字段（与官方字段同义，兜底用）
	StatusSnake  string `json:"status"`
	GroupIdSnake string `json:"group_id"`
	AssetIdSnake string `json:"asset_id"`
}

// ===== 素材组 =====

// CreateAssetGroup 创建素材组
// 上游: POST /?Action=CreateAssetGroup&Version=2024-01-01 → Result:{Id}
// Action 入口只回 Id，这里补一次 GetAssetGroup 取回完整对象（本地表要存名称等）。
func (a *NaxisActionAssetAdapter) CreateAssetGroup(req CreateAssetGroupRequest) (*AssetGroupResponse, error) {
	payload := map[string]interface{}{"Name": req.Name}
	if req.Description != "" {
		payload["Description"] = req.Description
	}
	if req.GroupType != "" {
		payload["GroupType"] = req.GroupType
	}
	if req.ProjectName != "" {
		payload["ProjectName"] = req.ProjectName
	}

	statusCode, respBody, err := a.doAction("CreateAssetGroup", nil, payload)
	if err != nil {
		return nil, err
	}
	if statusCode < 200 || statusCode >= 300 {
		return nil, naxisUpstreamError(statusCode, respBody)
	}

	created, err := parseNaxisActionResult[naxisActionIDResult](respBody)
	if err != nil {
		return nil, err
	}
	if created.Id == "" {
		return nil, fmt.Errorf("naxis action create asset group returned empty id: %s", truncateNaxisBody(respBody))
	}

	group, getErr := a.GetAssetGroup(created.Id)
	if getErr != nil {
		// 回查失败时用请求参数兜底，至少保证本地表有可读信息
		return &AssetGroupResponse{
			ID:          created.Id,
			Name:        req.Name,
			Description: req.Description,
			GroupType:   req.GroupType,
			ProjectName: req.ProjectName,
			RawData:     string(respBody),
		}, nil
	}
	if group.RawData == "" {
		group.RawData = string(respBody)
	}
	return group, nil
}

// GetAssetGroup 查询素材组
// 上游: POST /?Action=GetAssetGroup → Result:{Id, Name, Description, GroupType, ProjectName, CreateTime, UpdateTime}
func (a *NaxisActionAssetAdapter) GetAssetGroup(groupID string) (*AssetGroupResponse, error) {
	statusCode, respBody, err := a.doAction("GetAssetGroup", nil, map[string]interface{}{"Id": groupID})
	if err != nil {
		return nil, err
	}
	if statusCode < 200 || statusCode >= 300 {
		return nil, naxisUpstreamError(statusCode, respBody)
	}

	group, err := parseNaxisActionResult[naxisActionGroupResult](respBody)
	if err != nil {
		return nil, err
	}
	if group.Id == "" {
		return nil, fmt.Errorf("naxis action get asset group returned empty id: %s", truncateNaxisBody(respBody))
	}

	return &AssetGroupResponse{
		ID:          group.Id,
		Name:        group.Name,
		Description: group.Description,
		GroupType:   group.GroupType,
		ProjectName: group.ProjectName,
		CreateTime:  group.CreateTime,
		UpdateTime:  group.UpdateTime,
		RawData:     string(respBody),
	}, nil
}

// UpdateAssetGroup 更新素材组
// 上游: POST /?Action=UpdateAssetGroup → Result:{Id}
func (a *NaxisActionAssetAdapter) UpdateAssetGroup(groupID string, req UpdateAssetGroupRequest) error {
	payload := map[string]interface{}{"Id": groupID}
	if req.Name != "" {
		payload["Name"] = req.Name
	}
	if req.Description != "" {
		payload["Description"] = req.Description
	}

	statusCode, respBody, err := a.doAction("UpdateAssetGroup", nil, payload)
	if err != nil {
		return err
	}
	if statusCode < 200 || statusCode >= 300 {
		return naxisUpstreamError(statusCode, respBody)
	}
	return nil
}

// DeleteAssetGroup 删除素材组
// 上游: POST /?Action=DeleteAssetGroup → Result:{Id}；默认级联归档组内素材（Force 默认 true）
func (a *NaxisActionAssetAdapter) DeleteAssetGroup(groupID string) error {
	statusCode, respBody, err := a.doAction("DeleteAssetGroup", nil, map[string]interface{}{"Id": groupID})
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
// 上游: POST /?Action=CreateAsset → Result:{Id}；随后 GetAsset 取回状态。
// 注意：Action 入口的 GetAsset 返回的 URL 是平台签名代理地址，按约定不落库、不返回下游，
// 原始响应保存在 RawData 里即可。
func (a *NaxisActionAssetAdapter) CreateAsset(req CreateAssetRequest) (*AssetResponse, error) {
	payload := map[string]interface{}{
		"URL":       req.URL,
		"AssetType": mapNaxisActionAssetType(req.AssetType),
	}
	if req.GroupID != "" {
		payload["GroupId"] = req.GroupID
	}
	if req.Name != "" {
		payload["Name"] = req.Name
	}
	if req.ProjectName != "" {
		payload["ProjectName"] = req.ProjectName
	}

	statusCode, respBody, err := a.doAction("CreateAsset", nil, payload)
	if err != nil {
		return nil, err
	}
	if statusCode < 200 || statusCode >= 300 {
		return nil, naxisUpstreamError(statusCode, respBody)
	}

	created, err := parseNaxisActionResult[naxisActionIDResult](respBody)
	if err != nil {
		return nil, err
	}
	if created.Id == "" {
		return nil, fmt.Errorf("naxis action create asset returned empty id: %s", truncateNaxisBody(respBody))
	}

	common.SysLog(fmt.Sprintf("[naxis adapter] CreateAsset ok: id=%s", created.Id))

	asset, getErr := a.GetAsset(created.Id, nil)
	if getErr != nil {
		common.SysLog(fmt.Sprintf("[naxis adapter] CreateAsset 回查 GetAsset 失败: id=%s err=%v", created.Id, getErr))
		// 回查失败时先落 Processing，等下游轮询再收敛为 Active
		return &AssetResponse{
			ID:        created.Id,
			GroupID:   req.GroupID,
			Name:      req.Name,
			AssetType: mapNaxisAssetType(req.AssetType),
			Status:    "Processing",
			RawData:   string(respBody),
		}, nil
	}
	if asset.GroupID == "" {
		asset.GroupID = req.GroupID
	}
	if asset.Name == "" {
		asset.Name = req.Name
	}
	if asset.RawData == "" {
		asset.RawData = string(respBody)
	}
	common.SysLog(fmt.Sprintf("[naxis adapter] CreateAsset result: id=%s group_id=%s status=%s asset_type=%s",
		asset.ID, asset.GroupID, asset.Status, asset.AssetType))
	return asset, nil
}

// GetAsset 查询素材
// 上游: POST /?Action=GetAsset → Result:{Id, Name, URL, AssetType, GroupId, Status, ...}
// URL/SourceURL 不映射：上游给的是平台签名代理地址，不返回下游；原始响应存 RawData。
func (a *NaxisActionAssetAdapter) GetAsset(assetID string, _ url.Values) (*AssetResponse, error) {
	statusCode, respBody, err := a.doAction("GetAsset", nil, map[string]interface{}{"Id": assetID})
	if err != nil {
		return nil, err
	}
	if statusCode < 200 || statusCode >= 300 {
		return nil, naxisUpstreamError(statusCode, respBody)
	}

	asset, err := parseNaxisActionResult[naxisActionAssetResult](respBody)
	if err != nil {
		return nil, err
	}
	if asset.Id == "" {
		return nil, fmt.Errorf("naxis action get asset returned empty id: %s", truncateNaxisBody(respBody))
	}

	common.SysLog(fmt.Sprintf("[naxis adapter] GetAsset result: id=%s status=%s asset_type=%s group_id=%s",
		asset.Id, asset.Status, asset.AssetType, asset.GroupId))
	return &AssetResponse{
		ID:         asset.Id,
		GroupID:    asset.GroupId,
		Name:       asset.Name,
		AssetType:  asset.AssetType,
		Status:     mapNaxisAssetStatus(asset.Status),
		CreateTime: asset.CreateTime,
		UpdateTime: asset.UpdateTime,
		RawData:    string(respBody),
	}, nil
}

// UpdateAsset 更新素材
// 上游: POST /?Action=UpdateAsset → Result:{Id}（只允许改 Name）
func (a *NaxisActionAssetAdapter) UpdateAsset(assetID string, req UpdateAssetRequest) error {
	payload := map[string]interface{}{"Id": assetID}
	if req.Name != "" {
		payload["Name"] = req.Name
	}

	statusCode, respBody, err := a.doAction("UpdateAsset", nil, payload)
	if err != nil {
		return err
	}
	if statusCode < 200 || statusCode >= 300 {
		return naxisUpstreamError(statusCode, respBody)
	}
	return nil
}

// DeleteAsset 删除素材
// 上游: POST /?Action=DeleteAsset → Result:{Id}；删除后 GetAsset 返回 404
func (a *NaxisActionAssetAdapter) DeleteAsset(assetID string) error {
	statusCode, respBody, err := a.doAction("DeleteAsset", nil, map[string]interface{}{"Id": assetID})
	if err != nil {
		return err
	}
	if statusCode < 200 || statusCode >= 300 {
		return naxisUpstreamError(statusCode, respBody)
	}
	return nil
}

// ===== 真人核验（Action：CreateVisualValidateSession / GetVisualValidateResult）=====

// CreateFaceVerification 创建真人核验会话
// 上游: POST /?Action=CreateVisualValidateSession → Result:{BytedToken, CallbackURL, H5Link}
// Action 入口不返回过期时间；按文档固定 30 分钟有效期计算。
func (a *NaxisActionAssetAdapter) CreateFaceVerification(callbackURL string) (*FaceVerificationSession, error) {
	payload := map[string]interface{}{}
	if strings.TrimSpace(callbackURL) != "" {
		payload["CallbackURL"] = strings.TrimSpace(callbackURL)
	}

	statusCode, respBody, err := a.doAction("CreateVisualValidateSession", nil, payload)
	if err != nil {
		return nil, err
	}
	if statusCode < 200 || statusCode >= 300 {
		return nil, naxisUpstreamError(statusCode, respBody)
	}

	session, err := parseNaxisActionResult[naxisActionSessionResult](respBody)
	if err != nil {
		return nil, err
	}
	if session.BytedToken == "" {
		return nil, fmt.Errorf("naxis action create visual validate session returned empty byted token: %s", truncateNaxisBody(respBody))
	}

	return &FaceVerificationSession{
		Token:     session.BytedToken,
		H5URL:     session.H5Link,
		ExpiresAt: time.Now().Add(30 * time.Minute).Unix(),
		RawData:   string(respBody),
	}, nil
}

// GetFaceVerification 查询真人核验结果
// 上游: POST /?Action=GetVisualValidateResult → Result:{GroupId, Status, Passed, AssetId, Reason, ExpireTime}
// 平台还在 Result 里附带 snake_case 同义字段，两者都解析。
func (a *NaxisActionAssetAdapter) GetFaceVerification(token string) (*FaceVerificationResult, error) {
	statusCode, respBody, err := a.doAction("GetVisualValidateResult", nil, map[string]interface{}{"BytedToken": token})
	if err != nil {
		return nil, err
	}
	if statusCode < 200 || statusCode >= 300 {
		return nil, naxisUpstreamError(statusCode, respBody)
	}

	result, err := parseNaxisActionResult[naxisActionValidateResult](respBody)
	if err != nil {
		return nil, err
	}

	status := firstNonEmptyNaxis(result.Status, result.StatusSnake)
	if status == "" {
		return nil, fmt.Errorf("naxis action get visual validate result returned empty status: %s", truncateNaxisBody(respBody))
	}

	return &FaceVerificationResult{
		Status:     status,
		GroupID:    firstNonEmptyNaxis(result.GroupId, result.GroupIdSnake),
		Passed:     result.Passed,
		AssetID:    firstNonEmptyNaxis(result.AssetId, result.AssetIdSnake),
		Reason:     result.Reason,
		ExpireTime: result.ExpireTime,
		RawData:    string(respBody),
	}, nil
}

// ===== 基础设施 =====

// doAction 调用火山方舟原生 Action 入口：POST /?Action=<action>&Version=2024-01-01
func (a *NaxisActionAssetAdapter) doAction(action string, extraQuery url.Values, payload interface{}) (int, []byte, error) {
	query := url.Values{}
	query.Set("Action", action)
	query.Set("Version", naxisActionVersion)
	for key, values := range extraQuery {
		for _, value := range values {
			query.Add(key, value)
		}
	}
	return naxisDoRequest(a.baseURL, a.token, http.MethodPost, a.actionPath+"?"+query.Encode(), nil, payload)
}

// parseNaxisActionResult 解析 {ResponseMetadata, Result} 外壳中的 Result
func parseNaxisActionResult[T any](respBody []byte) (*T, error) {
	var envelope struct {
		Result T `json:"Result"`
	}
	if err := common.Unmarshal(respBody, &envelope); err != nil {
		return nil, fmt.Errorf("naxis parse action result failed: %w", err)
	}
	return &envelope.Result, nil
}

// mapNaxisActionAssetType 下游素材类型 → Action 入口的 PascalCase 取值
func mapNaxisActionAssetType(assetType string) string {
	switch strings.ToLower(strings.TrimSpace(assetType)) {
	case "image":
		return "Image"
	case "video":
		return "Video"
	case "audio":
		return "Audio"
	default:
		return "Image"
	}
}
