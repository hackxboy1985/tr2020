package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// ============================================================
// helpers
// ============================================================

func seedanceGetGW(c *gin.Context) (*service.SeedanceGatewayChannel, bool) {
	userGroup := c.GetString("group")
	gw, err := service.GetSeedanceGatewayChannel(userGroup)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": err.Error()})
		return nil, false
	}
	return gw, true
}

// proxyAndPassthrough sends the request to upstream and writes response back.
// It also calls the onSuccess callback (with status code and body) before writing.
func proxyAndPassthrough(c *gin.Context, gw *service.SeedanceGatewayChannel, method, path string, query url.Values, body []byte, onSuccess func(statusCode int, body []byte)) {
	statusCode, respBody, err := service.SeedanceProxyRequest(gw, method, path, query, body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": err.Error()})
		return
	}
	if onSuccess != nil && statusCode >= 200 && statusCode < 300 {
		onSuccess(statusCode, respBody)
	}
	c.Data(statusCode, "application/json; charset=utf-8", respBody)
}

func readBody(c *gin.Context) []byte {
	body, _ := c.GetRawData()
	return body
}

func forwardQuery(c *gin.Context) url.Values {
	return c.Request.URL.Query()
}

// ============================================================
// Asset Groups
// ============================================================

// POST /api/seedance/asset-groups
func SeedanceCreateAssetGroup(c *gin.Context) {
	gw, ok := seedanceGetGW(c)
	if !ok {
		return
	}
	userID := c.GetInt("id")
	body := readBody(c)

	proxyAndPassthrough(c, gw, http.MethodPost, "/api/seedance/proxy/assets/groups", nil, body, func(_ int, respBody []byte) {
		// parse upstream_group_id
		var resp struct {
			Result struct {
				ID   string `json:"Id"`
				Name string `json:"Name"`
				Desc string `json:"Description"`
				Type string `json:"GroupType"`
			} `json:"Result"`
		}
		if err := common.Unmarshal(respBody, &resp); err != nil || resp.Result.ID == "" {
			return
		}
		// parse name/desc/type from request body for local record
		var req struct {
			Name        string `json:"Name"`
			Description string `json:"Description"`
			GroupType   string `json:"GroupType"`
		}
		_ = common.Unmarshal(body, &req)

		g := &model.SeedanceAssetGroup{
			UserID:          userID,
			ChannelID:       gw.Channel.Id,
			UpstreamGroupID: resp.Result.ID,
			Name:            req.Name,
			Description:     req.Description,
			GroupType:       req.GroupType,
			RawData:         string(respBody),
		}
		_ = model.CreateSeedanceAssetGroup(g)
	})
}

// GET /api/seedance/asset-groups
func SeedanceListAssetGroups(c *gin.Context) {
	gw, ok := seedanceGetGW(c)
	if !ok {
		return
	}
	proxyAndPassthrough(c, gw, http.MethodGet, "/api/seedance/proxy/assets/groups", forwardQuery(c), nil, nil)
}

// GET /api/seedance/asset-groups/:id
func SeedanceGetAssetGroup(c *gin.Context) {
	gw, ok := seedanceGetGW(c)
	if !ok {
		return
	}
	userID := c.GetInt("id")
	localID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	g, err := model.GetSeedanceAssetGroupByID(localID, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "asset group not found"})
		return
	}
	proxyAndPassthrough(c, gw, http.MethodGet, "/api/seedance/proxy/assets/groups/"+g.UpstreamGroupID, forwardQuery(c), nil, nil)
}

// PUT /api/seedance/asset-groups/:id
func SeedancePutAssetGroup(c *gin.Context) {
	seedanceModifyAssetGroup(c, http.MethodPut)
}

// PATCH /api/seedance/asset-groups/:id
func SeedancePatchAssetGroup(c *gin.Context) {
	seedanceModifyAssetGroup(c, http.MethodPatch)
}

func seedanceModifyAssetGroup(c *gin.Context, method string) {
	gw, ok := seedanceGetGW(c)
	if !ok {
		return
	}
	userID := c.GetInt("id")
	localID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	g, err := model.GetSeedanceAssetGroupByID(localID, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "asset group not found"})
		return
	}
	body := readBody(c)
	proxyAndPassthrough(c, gw, method, "/api/seedance/proxy/assets/groups/"+g.UpstreamGroupID, nil, body, func(_ int, respBody []byte) {
		var req struct {
			Name        string `json:"Name"`
			Description string `json:"Description"`
		}
		_ = common.Unmarshal(body, &req)
		_ = model.UpdateSeedanceAssetGroupRaw(localID, req.Name, req.Description, string(respBody))
	})
}

// DELETE /api/seedance/asset-groups/:id
func SeedanceDeleteAssetGroup(c *gin.Context) {
	gw, ok := seedanceGetGW(c)
	if !ok {
		return
	}
	userID := c.GetInt("id")
	localID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	g, err := model.GetSeedanceAssetGroupByID(localID, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "asset group not found"})
		return
	}
	proxyAndPassthrough(c, gw, http.MethodDelete, "/api/seedance/proxy/assets/groups/"+g.UpstreamGroupID, nil, nil, func(_ int, _ []byte) {
		_ = model.SoftDeleteSeedanceAssetGroup(localID, userID)
	})
}

// ============================================================
// Assets
// ============================================================

// getOrCreateDefaultAssetGroup 获取或创建用户的默认 AIGC 素材组（每用户一个）
func getOrCreateDefaultAssetGroup(c *gin.Context, gw *service.SeedanceGatewayChannel, userID int) (string, error) {
	// 查本地表有无该用户的 AIGC 素材组
	groups, _, err := model.ListSeedanceAssetGroups(userID, 1, 1)
	if err == nil && len(groups) > 0 {
		return groups[0].UpstreamGroupID, nil
	}

	// 没有则创建
	user, err2 := model.GetUserById(userID, false)
	groupName := "default"
	if err2 == nil && user != nil {
		groupName = user.Username
	}

	createBody, _ := json.Marshal(map[string]string{
		"Name":        groupName,
		"Description": "auto-created",
		"GroupType":   "AIGC",
	})
	statusCode, respBody, err3 := service.SeedanceProxyRequest(gw, "POST", "/api/seedance/proxy/assets/groups", nil, createBody)
	if err3 != nil || statusCode < 200 || statusCode >= 300 {
		return "", fmt.Errorf("create default asset group failed: %v", err3)
	}
	var resp struct {
		Result struct {
			ID string `json:"Id"`
		} `json:"Result"`
	}
	if err4 := common.Unmarshal(respBody, &resp); err4 != nil || resp.Result.ID == "" {
		return "", fmt.Errorf("parse create group response failed")
	}
	g := &model.SeedanceAssetGroup{
		UserID:          userID,
		ChannelID:       gw.Channel.Id,
		UpstreamGroupID: resp.Result.ID,
		Name:            groupName,
		GroupType:       "AIGC",
		RawData:         string(respBody),
	}
	_ = model.CreateSeedanceAssetGroup(g)
	return resp.Result.ID, nil
}

// POST /api/seedance/assets
func SeedanceCreateAsset(c *gin.Context) {
	gw, ok := seedanceGetGW(c)
	if !ok {
		return
	}
	userID := c.GetInt("id")
	body := readBody(c)

	// 解析请求体，如果没有 GroupId，自动获取或创建默认素材组
	var req struct {
		GroupID   string `json:"GroupId"`
		URL       string `json:"URL"`
		AssetType string `json:"AssetType"`
		Name      string `json:"Name"`
	}
	_ = common.Unmarshal(body, &req)

	if req.GroupID == "" {
		groupID, err := getOrCreateDefaultAssetGroup(c, gw, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
			return
		}
		req.GroupID = groupID
		// 重新构建请求体，加上 GroupId
		newBody, _ := json.Marshal(req)
		body = newBody
	}

	statusCode, respBody, proxyErr := service.SeedanceProxyRequest(gw, http.MethodPost, "/api/seedance/proxy/assets", nil, body)
	if proxyErr != nil {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": proxyErr.Error()})
		return
	}
	if statusCode < 200 || statusCode >= 300 {
		c.Data(statusCode, "application/json; charset=utf-8", respBody)
		return
	}

	// 解析上游响应取 upstream_asset_id
	var resp struct {
		Result map[string]interface{} `json:"Result"`
	}
	if err := common.Unmarshal(respBody, &resp); err != nil || resp.Result == nil {
		c.Data(statusCode, "application/json; charset=utf-8", respBody)
		return
	}
	upstreamAssetID, _ := resp.Result["Id"].(string)
	if upstreamAssetID == "" {
		c.Data(statusCode, "application/json; charset=utf-8", respBody)
		return
	}

	a := &model.SeedanceAsset{
		UserID:          userID,
		ChannelID:       gw.Channel.Id,
		UpstreamAssetID: upstreamAssetID,
		UpstreamGroupID: req.GroupID,
		Name:            req.Name,
		AssetType:       req.AssetType,
		SourceURL:       req.URL,
		Status:          "Processing",
		RawData:         string(respBody),
	}
	_ = model.CreateSeedanceAsset(a)

	// 在响应里追加 local_id，方便客户端直接用于查询接口
	resp.Result["LocalId"] = a.ID
	resp.Result["AssetRef"] = "asset://" + upstreamAssetID
	merged, mergeErr := common.Marshal(map[string]interface{}{"Result": resp.Result})
	if mergeErr != nil {
		c.Data(statusCode, "application/json; charset=utf-8", respBody)
		return
	}
	c.Data(statusCode, "application/json; charset=utf-8", merged)
}

// GET /api/seedance/assets
func SeedanceListAssets(c *gin.Context) {
	gw, ok := seedanceGetGW(c)
	if !ok {
		return
	}
	proxyAndPassthrough(c, gw, http.MethodGet, "/api/seedance/proxy/assets", forwardQuery(c), nil, nil)
}

// resolveAsset 支持本地数字 ID 或上游 asset-xxxxxxxx 格式查询素材
func resolveAsset(c *gin.Context, userID int) (*model.SeedanceAsset, error) {
	rawID := c.Param("id")
	if strings.HasPrefix(rawID, "asset-") {
		return model.GetSeedanceAssetByUpstreamID(rawID, userID)
	}
	localID, _ := strconv.ParseInt(rawID, 10, 64)
	return model.GetSeedanceAssetByID(localID, userID)
}

// GET /api/seedance/assets/:id
func SeedanceGetAsset(c *gin.Context) {
	gw, ok := seedanceGetGW(c)
	if !ok {
		return
	}
	userID := c.GetInt("id")
	a, err := resolveAsset(c, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "asset not found"})
		return
	}
	proxyAndPassthrough(c, gw, http.MethodGet, "/api/seedance/proxy/assets/"+a.UpstreamAssetID, forwardQuery(c), nil, func(_ int, respBody []byte) {
		var resp struct {
			Result struct {
				Status string `json:"Status"`
			} `json:"Result"`
		}
		if err2 := common.Unmarshal(respBody, &resp); err2 == nil && resp.Result.Status != "" {
			_ = model.UpdateSeedanceAssetStatus(a.ID, resp.Result.Status, string(respBody))
		}
	})
}

// PUT /api/seedance/assets/:id
func SeedancePutAsset(c *gin.Context) {
	seedanceModifyAsset(c, http.MethodPut)
}

// PATCH /api/seedance/assets/:id
func SeedancePatchAsset(c *gin.Context) {
	seedanceModifyAsset(c, http.MethodPatch)
}

func seedanceModifyAsset(c *gin.Context, method string) {
	gw, ok := seedanceGetGW(c)
	if !ok {
		return
	}
	userID := c.GetInt("id")
	a, err := resolveAsset(c, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "asset not found"})
		return
	}
	body := readBody(c)
	proxyAndPassthrough(c, gw, method, "/api/seedance/proxy/assets/"+a.UpstreamAssetID, nil, body, func(_ int, respBody []byte) {
		_ = model.UpdateSeedanceAssetStatus(a.ID, a.Status, string(respBody))
	})
}

// DELETE /api/seedance/assets/:id
func SeedanceDeleteAsset(c *gin.Context) {
	gw, ok := seedanceGetGW(c)
	if !ok {
		return
	}
	userID := c.GetInt("id")
	a, err := resolveAsset(c, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "asset not found"})
		return
	}
	proxyAndPassthrough(c, gw, http.MethodDelete, "/api/seedance/proxy/assets/"+a.UpstreamAssetID, nil, nil, func(_ int, _ []byte) {
		_ = model.SoftDeleteSeedanceAsset(a.ID, userID)
	})
}

// ============================================================
// Face Verifications
// ============================================================

// POST /api/seedance/face-verifications
func SeedanceCreateFaceVerification(c *gin.Context) {
	gw, ok := seedanceGetGW(c)
	if !ok {
		return
	}
	userID := c.GetInt("id")
	body := readBody(c)

	proxyAndPassthrough(c, gw, http.MethodPost, "/api/seedance/face-verifications", nil, body, func(_ int, respBody []byte) {
		var resp struct {
			VerificationID string `json:"verification_id"`
			H5URL          string `json:"h5_url"`
			ExpiresAt      int64  `json:"expires_at"`
		}
		if err := common.Unmarshal(respBody, &resp); err != nil || resp.VerificationID == "" {
			return
		}
		v := &model.SeedanceFaceVerification{
			UserID:         userID,
			ChannelID:      gw.Channel.Id,
			VerificationID: resp.VerificationID,
			Status:         "waiting_user",
			H5URL:          resp.H5URL,
			ExpiresAt:      resp.ExpiresAt,
			RawData:        string(respBody),
		}
		_ = model.CreateSeedanceFaceVerification(v)
	})
}

// GET /api/seedance/face-verifications/:id
func SeedanceGetFaceVerification(c *gin.Context) {
	gw, ok := seedanceGetGW(c)
	if !ok {
		return
	}
	userID := c.GetInt("id")
	localID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	v, err := model.GetSeedanceFaceVerificationByID(localID, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "face verification not found"})
		return
	}
	proxyAndPassthrough(c, gw, http.MethodGet, "/api/seedance/face-verifications/"+v.VerificationID, nil, nil, func(_ int, respBody []byte) {
		var resp struct {
			Status  string `json:"status"`
			GroupID string `json:"group_id"`
		}
		if err2 := common.Unmarshal(respBody, &resp); err2 == nil && resp.Status != "" {
			_ = model.UpdateSeedanceFaceVerificationStatus(localID, resp.Status, resp.GroupID, string(respBody))
		}
	})
}

// ============================================================
// Admin list endpoints (GET /api/admin/seedance/*)
// ============================================================

// GET /api/admin/seedance/asset-groups
func SeedanceAdminListAssetGroups(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	userIDFilter, _ := strconv.Atoi(c.Query("user_id"))
	groups, total, err := model.ListAllSeedanceAssetGroups(pageInfo.GetStartIdx(), pageInfo.GetPageSize(), userIDFilter)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(groups)
	common.ApiSuccess(c, pageInfo)
}

// GET /api/admin/seedance/assets
func SeedanceAdminListAssets(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	userIDFilter, _ := strconv.Atoi(c.Query("user_id"))
	groupID := c.Query("group_id")
	assets, total, err := model.ListAllSeedanceAssets(pageInfo.GetStartIdx(), pageInfo.GetPageSize(), userIDFilter, groupID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(assets)
	common.ApiSuccess(c, pageInfo)
}

// GET /api/admin/seedance/face-verifications
func SeedanceAdminListFaceVerifications(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	userIDFilter, _ := strconv.Atoi(c.Query("user_id"))
	items, total, err := model.ListAllSeedanceFaceVerifications(pageInfo.GetStartIdx(), pageInfo.GetPageSize(), userIDFilter)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

// ensure json import used
var _ = json.Marshal
