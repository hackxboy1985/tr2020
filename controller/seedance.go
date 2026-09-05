package controller

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
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
		logger.LogError(c, fmt.Sprintf("seedance: no gateway channel for group %s: %v", userGroup, err))
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": err.Error()})
		return nil, false
	}
	return gw, true
}

// proxyAndPassthrough sends the request to upstream and writes response back.
// It also calls the onSuccess callback (with status code and body) before writing.
// If the callback writes a response, proxyAndPassthrough will not write again.
func proxyAndPassthrough(c *gin.Context, gw *service.SeedanceGatewayChannel, method, path string, query url.Values, body []byte, onSuccess func(statusCode int, body []byte)) {
	statusCode, respBody, err := service.AssetProxyRequest(gw, method, path, query, body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": err.Error()})
		return
	}
	if statusCode < 200 || statusCode >= 300 {
		c.JSON(statusCode, gin.H{"success": false, "message": extractUpstreamErrMsg(respBody)})
		return
	}
	if onSuccess != nil {
		onSuccess(statusCode, respBody)
	}
	// 检查回调是否已经写入响应，如果是则不再写入
	if !c.Writer.Written() {
		c.Data(statusCode, "application/json; charset=utf-8", respBody)
	}
}

// extractUpstreamErrMsg 从上游标准错误结构中提取 Message，解析失败时返回通用提示。
func extractUpstreamErrMsg(body []byte) string {
	var errResp struct {
		ResponseMetadata struct {
			Error struct {
				Message string `json:"Message"`
			} `json:"Error"`
		} `json:"ResponseMetadata"`
	}
	if common.Unmarshal(body, &errResp) == nil && errResp.ResponseMetadata.Error.Message != "" {
		return errResp.ResponseMetadata.Error.Message
	}
	return "upstream request failed"
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
		// 兼容两种响应格式：
		// Gateway: { "Result": { "Id": "xxx", "Name": "xxx", ... } }
		// KWJM: { "Id": "xxx", "Name": "xxx", ... }
		var groupID, groupName, groupDesc, groupType string

		// 尝试 Gateway 格式
		var gatewayResp struct {
			Result struct {
				ID   string `json:"Id"`
				Name string `json:"Name"`
				Desc string `json:"Description"`
				Type string `json:"GroupType"`
			} `json:"Result"`
		}
		if err := common.Unmarshal(respBody, &gatewayResp); err == nil && gatewayResp.Result.ID != "" {
			groupID = gatewayResp.Result.ID
			groupName = gatewayResp.Result.Name
			groupDesc = gatewayResp.Result.Desc
			groupType = gatewayResp.Result.Type
		} else {
			// 尝试 KWJM 格式
			var kwjmResp struct {
				ID   string `json:"Id"`
				Name string `json:"Name"`
				Desc string `json:"Description"`
				Type string `json:"GroupType"`
			}
			if err2 := common.Unmarshal(respBody, &kwjmResp); err2 == nil && kwjmResp.ID != "" {
				groupID = kwjmResp.ID
				groupName = kwjmResp.Name
				groupDesc = kwjmResp.Desc
				groupType = kwjmResp.Type
			}
		}

		if groupID == "" {
			return
		}
		// parse name/desc/type from request body for local record (fallback)
		var req struct {
			Name        string `json:"Name"`
			Description string `json:"Description"`
			GroupType   string `json:"GroupType"`
		}
		_ = common.Unmarshal(body, &req)

		// 优先使用响应中的值，否则使用请求中的值
		if groupName == "" {
			groupName = req.Name
		}
		if groupDesc == "" {
			groupDesc = req.Description
		}
		if groupType == "" {
			groupType = req.GroupType
		}

		g := &model.SeedanceAssetGroup{
			UserID:          userID,
			ChannelID:       gw.Channel.Id,
			UpstreamGroupID: groupID,
			Name:            groupName,
			Description:     groupDesc,
			GroupType:       groupType,
			RawData:         string(respBody),
		}
		_ = model.CreateSeedanceAssetGroup(g)
		// 在响应里追加 LocalId（业务 ID），方便中继链路直接使用
		var raw map[string]interface{}
		if err2 := common.Unmarshal(respBody, &raw); err2 == nil {
			// Gateway 格式: { "Result": { "Id": "xxx" } }
			if result, ok := raw["Result"].(map[string]interface{}); ok {
				result["LocalId"] = groupID
				if merged, err3 := common.Marshal(raw); err3 == nil {
					c.Data(http.StatusOK, "application/json; charset=utf-8", merged)
					return
				}
			} else {
				// KWJM 格式: { "Id": "xxx" }，直接在顶层添加 LocalId
				raw["LocalId"] = groupID
				if merged, err3 := common.Marshal(raw); err3 == nil {
					c.Data(http.StatusOK, "application/json; charset=utf-8", merged)
					return
				}
			}
		}
	})
}

// GET /api/seedance/asset-groups
func SeedanceListAssetGroups(c *gin.Context) {
	userID := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)
	groups, total, err := model.ListSeedanceAssetGroups(userID, pageInfo.GetPage(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 检查是否需要返回火山官方格式（通过 RelayAsset 调用）
	if c.GetBool("doubao_official_format") {
		// 转换为火山官方格式
		items := []map[string]interface{}{}
		for _, g := range groups {
			item := map[string]interface{}{
				"Id":          g.UpstreamGroupID,
				"Name":        g.Name,
				"GroupType":   g.GroupType,
				"CreateTime":  formatTime(g.CreatedAt),
				"UpdateTime":  formatTime(g.UpdatedAt),
			}
			if g.Description != "" {
				item["Description"] = g.Description
			}
			items = append(items, item)
		}

		c.JSON(http.StatusOK, gin.H{
			"ResponseMetadata": gin.H{
				"RequestId": generateRequestID(),
				"Action":    "ListAssetGroups",
				"Version":   "2024-01-01",
				"Service":   "ark",
				"Region":    "cn-beijing",
			},
			"Result": gin.H{
				"Items":      items,
				"TotalCount": total,
				"PageNumber": pageInfo.GetPage(),
				"PageSize":   pageInfo.GetPageSize(),
			},
		})
		return
	}

	// 默认返回本地格式（RESTful API）
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(groups)
	common.ApiSuccess(c, pageInfo)
}

// GET /api/seedance/asset-groups/:id
func SeedanceGetAssetGroup(c *gin.Context) {
	gw, ok := seedanceGetGW(c)
	if !ok {
		return
	}
	userID := c.GetInt("id")
	g, err := resolveAssetGroup(c, userID, gw.Channel.Id)
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
	g, err := resolveAssetGroup(c, userID, gw.Channel.Id)
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
		_ = model.UpdateSeedanceAssetGroupRaw(g.ID, req.Name, req.Description, string(respBody))
	})
}

// DELETE /api/seedance/asset-groups/:id
func SeedanceDeleteAssetGroup(c *gin.Context) {
	gw, ok := seedanceGetGW(c)
	if !ok {
		return
	}
	userID := c.GetInt("id")
	g, err := resolveAssetGroup(c, userID, gw.Channel.Id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "asset group not found"})
		return
	}
	proxyAndPassthrough(c, gw, http.MethodDelete, "/api/seedance/proxy/assets/groups/"+g.UpstreamGroupID, nil, nil, func(_ int, _ []byte) {
		_ = model.SoftDeleteSeedanceAssetGroup(g.ID, userID)
	})
}

// ============================================================
// Assets
// ============================================================

// getOrCreateDefaultAssetGroup 获取或创建用户的默认 AIGC 素材组（每用户每渠道一个）
func getOrCreateDefaultAssetGroup(c *gin.Context, gw *service.SeedanceGatewayChannel, userID int) (string, error) {
	// 查本地表有无该用户+渠道的 AIGC 素材组，必须匹配渠道，避免跨渠道引用不存在的 group
	groups, _, err := model.ListSeedanceAssetGroupsByChannel(userID, gw.Channel.Id, 1, 1)
	if err == nil && len(groups) > 0 {
		logger.LogInfo(c, fmt.Sprintf("seedance: found existing asset group %s for user %d", groups[0].UpstreamGroupID, userID))
		return groups[0].UpstreamGroupID, nil
	}

	// 没有则创建，组名用 u{id}-{md5(username)前8位}，保证 ASCII 且可追溯
	user, err2 := model.GetUserById(userID, false)
	groupName := fmt.Sprintf("u%d", userID)
	if err2 == nil && user != nil && user.Username != "" {
		h := fmt.Sprintf("%x", md5.Sum([]byte(user.Username)))
		groupName = fmt.Sprintf("u%d-%s", userID, h[:8])
	}

	logger.LogInfo(c, fmt.Sprintf("seedance: creating default asset group '%s' for user %d", groupName, userID))

	createBody, _ := json.Marshal(map[string]string{
		"Name":        groupName,
		"Description": "auto-created",
		"GroupType":   "AIGC",
	})
	statusCode, respBody, err3 := service.AssetProxyRequest(gw, "POST", "/api/seedance/proxy/assets/groups", nil, createBody)
	if err3 != nil {
		return "", fmt.Errorf("create default asset group failed: %w", err3)
	}
	if statusCode < 200 || statusCode >= 300 {
		return "", fmt.Errorf("create default asset group upstream error %d: %s", statusCode, string(respBody))
	}
	// 兼容两种响应格式：
	// Gateway: { "Result": { "Id": "xxx" } }
	// KWJM: { "Id": "xxx" }
	var groupID string

	// 尝试 Gateway 格式
	var gatewayResp struct {
		Result struct {
			ID string `json:"Id"`
		} `json:"Result"`
	}
	if err4 := common.Unmarshal(respBody, &gatewayResp); err4 == nil && gatewayResp.Result.ID != "" {
		groupID = gatewayResp.Result.ID
	} else {
		// 尝试 KWJM 格式
		var kwjmResp struct {
			ID string `json:"Id"`
		}
		if err5 := common.Unmarshal(respBody, &kwjmResp); err5 == nil && kwjmResp.ID != "" {
			groupID = kwjmResp.ID
		}
	}

	if groupID == "" {
		return "", fmt.Errorf("parse create group response failed: %s", string(respBody))
	}
	g := &model.SeedanceAssetGroup{
		UserID:          userID,
		ChannelID:       gw.Channel.Id,
		UpstreamGroupID: groupID,
		Name:            groupName,
		GroupType:       "AIGC",
		RawData:         string(respBody),
	}
	_ = model.CreateSeedanceAssetGroup(g)
	logger.LogInfo(c, fmt.Sprintf("seedance: created asset group %s for user %d", groupID, userID))
	return groupID, nil
}

// POST /api/seedance/assets
func SeedanceCreateAsset(c *gin.Context) {
	gw, ok := seedanceGetGW(c)
	if !ok {
		return
	}
	userID := c.GetInt("id")
	body := readBody(c)

	// 解析请求体
	var req struct {
		GroupID   string `json:"GroupId"`
		URL       string `json:"URL"`
		AssetType string `json:"AssetType"`
		Name      string `json:"Name"`
		Force     bool   `json:"Force"` // true=强制重新上传，忽略本地缓存
	}
	_ = common.Unmarshal(body, &req)

	// 如果未强制，先按 source_url 查本地表，有 Active 记录直接返回
	if !req.Force && req.URL != "" {
		if existing, err := model.GetSeedanceAssetBySourceURL(req.URL, userID, gw.Channel.Id); err == nil && existing.Status == "Active" {
			logger.LogInfo(c, fmt.Sprintf("seedance: asset already exists for url %s, asset_id=%s", req.URL, existing.UpstreamAssetID))
			result := map[string]interface{}{
				"Id":       existing.UpstreamAssetID,
				"LocalId":  existing.ID,
				"AssetRef": "asset://" + existing.UpstreamAssetID,
				"Status":   existing.Status,
			}
			c.JSON(http.StatusOK, gin.H{"Result": result})
			return
		}
	}

	if req.GroupID == "" {
		groupID, err := getOrCreateDefaultAssetGroup(c, gw, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
			return
		}
		req.GroupID = groupID
	}
	// 重新构建请求体（去掉 Force 字段，上游不认识）
	newBody, _ := json.Marshal(map[string]string{
		"GroupId":   req.GroupID,
		"URL":       req.URL,
		"AssetType": req.AssetType,
		"Name":      req.Name,
	})
	body = newBody

	statusCode, respBody, proxyErr := service.AssetProxyRequest(gw, http.MethodPost, "/api/seedance/proxy/assets", nil, body)
	if proxyErr != nil {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": proxyErr.Error()})
		return
	}

	// 若上游返回 404 且是 group_id 不存在，自动软删本地旧记录并重建 group 后重试一次
	if statusCode == http.StatusNotFound && req.GroupID != "" {
		var errResp struct {
			ResponseMetadata struct {
				Error struct {
					Code string `json:"Code"`
				} `json:"Error"`
			} `json:"ResponseMetadata"`
		}
		if common.Unmarshal(respBody, &errResp) == nil &&
			errResp.ResponseMetadata.Error.Code == "NotFound.group_id" {
			logger.LogWarn(c, fmt.Sprintf("seedance: upstream group %s not found, rebuilding for user %d", req.GroupID, userID))
			_ = model.SoftDeleteSeedanceAssetGroupByUpstreamID(req.GroupID, userID)
			newGroupID, rebuildErr := getOrCreateDefaultAssetGroup(c, gw, userID)
			if rebuildErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": rebuildErr.Error()})
				return
			}
			req.GroupID = newGroupID
			retryBody, _ := json.Marshal(map[string]string{
				"GroupId":   req.GroupID,
				"URL":       req.URL,
				"AssetType": req.AssetType,
				"Name":      req.Name,
			})
			statusCode, respBody, proxyErr = service.AssetProxyRequest(gw, http.MethodPost, "/api/seedance/proxy/assets", nil, retryBody)
			if proxyErr != nil {
				c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": proxyErr.Error()})
				return
			}
		}
	}

	if statusCode < 200 || statusCode >= 300 {
		c.JSON(statusCode, gin.H{"success": false, "message": extractUpstreamErrMsg(respBody)})
		return
	}

	// 解析上游响应取 upstream_asset_id
	// 兼容两种格式：Gateway: { "Result": { "Id": "xxx" } }，KWJM: { "Id": "xxx" }
	var upstreamAssetID, upstreamGroupID string

	// 尝试 Gateway 格式
	var gatewayResp struct {
		Result map[string]interface{} `json:"Result"`
	}
	if err := common.Unmarshal(respBody, &gatewayResp); err == nil && gatewayResp.Result != nil {
		upstreamAssetID, _ = gatewayResp.Result["Id"].(string)
		upstreamGroupID, _ = gatewayResp.Result["GroupId"].(string)
	} else {
		// 尝试 KWJM 格式
		var kwjmResp map[string]interface{}
		if err2 := common.Unmarshal(respBody, &kwjmResp); err2 == nil {
			upstreamAssetID, _ = kwjmResp["Id"].(string)
			upstreamGroupID, _ = kwjmResp["GroupId"].(string)
		}
	}

	if upstreamAssetID == "" {
		c.Data(statusCode, "application/json; charset=utf-8", respBody)
		return
	}

	// fallback 到请求的 GroupID
	if upstreamGroupID == "" {
		upstreamGroupID = req.GroupID
	}

	a := &model.SeedanceAsset{
		UserID:          userID,
		ChannelID:       gw.Channel.Id,
		UpstreamAssetID: upstreamAssetID,
		UpstreamGroupID: upstreamGroupID, // 使用上游返回的实际 GroupID
		Name:            req.Name,
		AssetType:       req.AssetType,
		SourceURL:       req.URL,
		Status:          "Processing",
		RawData:         string(respBody),
	}
	_ = model.CreateSeedanceAsset(a)

	// 在响应里追加 local_id（业务 ID），方便客户端直接用于查询接口
	// 兼容两种格式：Gateway: { "Result": { ... } }，KWJM: { ... }
	var raw map[string]interface{}
	if err2 := common.Unmarshal(respBody, &raw); err2 == nil {
		// Gateway 格式
		if result, ok := raw["Result"].(map[string]interface{}); ok {
			result["LocalId"] = a.UpstreamAssetID
			result["AssetRef"] = "asset://" + upstreamAssetID
			merged, mergeErr := common.Marshal(map[string]interface{}{"Result": result})
			if mergeErr == nil {
				c.Data(statusCode, "application/json; charset=utf-8", merged)
				return
			}
		} else {
			// KWJM 格式，直接在顶层添加
			raw["LocalId"] = a.UpstreamAssetID
			raw["AssetRef"] = "asset://" + upstreamAssetID
			merged, mergeErr := common.Marshal(raw)
			if mergeErr == nil {
				c.Data(statusCode, "application/json; charset=utf-8", merged)
				return
			}
		}
	}
	// fallback：返回原始响应
	c.Data(statusCode, "application/json; charset=utf-8", respBody)
}

// GET /api/seedance/assets
func SeedanceListAssets(c *gin.Context) {
	userID := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)
	groupID := c.Query("group_id")
	assetID, _ := strconv.ParseInt(c.Query("id"), 10, 64)
	upstreamAssetID := strings.TrimSpace(c.Query("upstream_asset_id"))
	assets, total, err := model.ListSeedanceAssets(userID, groupID, assetID, upstreamAssetID, pageInfo.GetPage(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 检查是否需要返回火山官方格式（通过 RelayAsset 调用）
	if c.GetBool("doubao_official_format") {
		// 转换为火山官方格式
		items := []map[string]interface{}{}
		for _, a := range assets {
			item := map[string]interface{}{
				"Id":          a.UpstreamAssetID,
				"GroupId":     a.UpstreamGroupID,
				"Name":        a.Name,
				"AssetType":   a.AssetType,
				"Status":      a.Status,
				"CreateTime":  formatTime(a.CreatedAt),
				"UpdateTime":  formatTime(a.UpdatedAt),
			}
			if a.SourceURL != "" {
				item["URL"] = a.SourceURL
			}
			items = append(items, item)
		}

		c.JSON(http.StatusOK, gin.H{
			"ResponseMetadata": gin.H{
				"RequestId": generateRequestID(),
				"Action":    "ListAssets",
				"Version":   "2024-01-01",
				"Service":   "ark",
				"Region":    "cn-beijing",
			},
			"Result": gin.H{
				"Items":      items,
				"TotalCount": total,
				"PageNumber": pageInfo.GetPage(),
				"PageSize":   pageInfo.GetPageSize(),
			},
		})
		return
	}

	// 默认返回本地格式（RESTful API）
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(assets)
	common.ApiSuccess(c, pageInfo)
}

// resolveAsset 支持本地数字 ID 或上游 asset-xxxxxxxx 格式查询素材
func resolveAsset(c *gin.Context, userID int, channelID int) (*model.SeedanceAsset, error) {
	rawID := c.Param("id")
	if strings.HasPrefix(rawID, "asset-") {
		return model.GetSeedanceAssetByUpstreamID(rawID, userID, channelID)
	}
	localID, _ := strconv.ParseInt(rawID, 10, 64)
	return model.GetSeedanceAssetByID(localID, userID)
}

// resolveAssetGroup 支持本地数字 ID 或上游 group-xxxxxxxx 格式查询素材组
func resolveAssetGroup(c *gin.Context, userID int, channelID int) (*model.SeedanceAssetGroup, error) {
	rawID := c.Param("id")
	if strings.HasPrefix(rawID, "group-") {
		return model.GetSeedanceAssetGroupByUpstreamID(rawID, userID, channelID)
	}
	localID, _ := strconv.ParseInt(rawID, 10, 64)
	return model.GetSeedanceAssetGroupByID(localID, userID)
}

// resolveFaceVerification 支持本地数字 ID 或上游 fv_xxxxxxxxx 格式查询人脸认证任务
func resolveFaceVerification(c *gin.Context, userID int) (*model.SeedanceFaceVerification, error) {
	rawID := c.Param("id")
	if strings.HasPrefix(rawID, "fv_") {
		return model.GetSeedanceFaceVerificationByVerificationID(rawID, userID)
	}
	localID, _ := strconv.ParseInt(rawID, 10, 64)
	return model.GetSeedanceFaceVerificationByID(localID, userID)
}

// GET /api/seedance/assets/:id
func SeedanceGetAsset(c *gin.Context) {
	gw, ok := seedanceGetGW(c)
	if !ok {
		return
	}
	userID := c.GetInt("id")
	a, err := resolveAsset(c, userID, gw.Channel.Id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "asset not found"})
		return
	}
	proxyAndPassthrough(c, gw, http.MethodGet, "/api/seedance/proxy/assets/"+a.UpstreamAssetID, forwardQuery(c), nil, func(_ int, respBody []byte) {
		// 兼容两种格式：Gateway: { "Result": { "Status": "xxx" } }，KWJM: { "Status": "xxx" }
		var status string

		// 尝试 Gateway 格式
		var gatewayResp struct {
			Result struct {
				Status string `json:"Status"`
			} `json:"Result"`
		}
		if err := common.Unmarshal(respBody, &gatewayResp); err == nil && gatewayResp.Result.Status != "" {
			status = gatewayResp.Result.Status
		} else {
			// 尝试 KWJM 格式
			var kwjmResp struct {
				Status string `json:"Status"`
			}
			if err2 := common.Unmarshal(respBody, &kwjmResp); err2 == nil && kwjmResp.Status != "" {
				status = kwjmResp.Status
			}
		}

		if status != "" {
			_ = model.UpdateSeedanceAssetStatus(a.ID, status, string(respBody))
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
	a, err := resolveAsset(c, userID, gw.Channel.Id)
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
	a, err := resolveAsset(c, userID, gw.Channel.Id)
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
	v, err := resolveFaceVerification(c, userID)
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
			_ = model.UpdateSeedanceFaceVerificationStatus(v.ID, resp.Status, resp.GroupID, string(respBody))
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
	groups, total, err := model.ListAllSeedanceAssetGroups(pageInfo.GetPage(), pageInfo.GetPageSize(), userIDFilter)
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
	assetID, _ := strconv.ParseInt(c.Query("id"), 10, 64)
	upstreamAssetID := strings.TrimSpace(c.Query("upstream_asset_id"))
	assets, total, err := model.ListAllSeedanceAssets(pageInfo.GetPage(), pageInfo.GetPageSize(), userIDFilter, groupID, assetID, upstreamAssetID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(assets)
	common.ApiSuccess(c, pageInfo)
}

// GET /api/seedance/face-verifications
func SeedanceListFaceVerifications(c *gin.Context) {
	userID := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)
	items, total, err := model.ListSeedanceFaceVerifications(userID, pageInfo.GetPage(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

// GET /api/admin/seedance/face-verifications
func SeedanceAdminListFaceVerifications(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	userIDFilter, _ := strconv.Atoi(c.Query("user_id"))
	items, total, err := model.ListAllSeedanceFaceVerifications(pageInfo.GetPage(), pageInfo.GetPageSize(), userIDFilter)
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

// formatTime 将 Unix 时间戳转换为 ISO 8601 格式
func formatTime(timestamp int64) string {
	if timestamp == 0 {
		return ""
	}
	return fmt.Sprintf("%s", time.Unix(timestamp, 0).UTC().Format(time.RFC3339))
}

// generateRequestID 生成火山格式的请求 ID
func generateRequestID() string {
	// 格式：YYYYMMDDHHMMSS + 随机字符串
	now := time.Now()
	randomPart := fmt.Sprintf("%X", md5.Sum([]byte(fmt.Sprintf("%d%d", now.UnixNano(), rand.Int()))))
	return fmt.Sprintf("%s%s", now.Format("20060102150405"), randomPart[:20])
}
