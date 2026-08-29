package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// RelayAsset handles POST /api/seedance/assets/v2/?Action=XXX&Version=2024-01-01
func RelayAsset(c *gin.Context) {
	action := c.Query("Action")
	version := c.Query("Version")

	if action == "" {
		c.JSON(http.StatusBadRequest, &dto.TaskError{
			Code:       "invalid_request",
			Message:    "Action parameter is required",
			StatusCode: http.StatusBadRequest,
		})
		return
	}

	if version == "" {
		version = "2024-01-01" // 默认版本
	}

	// 验证 Action 是否支持
	supportedActions := []string{
		// 真人认证
		"CreateVisualValidateSession",
		"GetVisualValidateResult",
		// Asset Group 管理
		"CreateAssetGroup",
		"GetAssetGroup",
		"ListAssetGroups",
		"UpdateAssetGroup",
		"DeleteAssetGroup",
		// Asset 管理
		"CreateAsset",
		"GetAsset",
		"ListAssets",
		"UpdateAsset",
		"DeleteAsset",
	}

	isSupported := false
	for _, a := range supportedActions {
		if action == a {
			isSupported = true
			break
		}
	}

	if !isSupported {
		c.JSON(http.StatusBadRequest, &dto.TaskError{
			Code:       "unsupported_action",
			Message:    fmt.Sprintf("Action %s is not supported", action),
			StatusCode: http.StatusBadRequest,
		})
		return
	}

	// 读取请求体
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, &dto.TaskError{
			Code:       "read_body_failed",
			Message:    err.Error(),
			StatusCode: http.StatusBadRequest,
		})
		return
	}

	// 获取 Seedance Gateway 渠道（不使用 Distribute 中间件）
	userGroup := c.GetString("group")
	if userGroup == "" {
		userGroup = "default"
	}

	gw, err := service.GetSeedanceGatewayChannel(userGroup)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": map[string]interface{}{
				"message": fmt.Sprintf("no available seedance gateway channel: %s", err.Error()),
				"type":    "invalid_request_error",
			},
		})
		return
	}

	// 获取上游 API 格式配置
	format := gw.Channel.GetOtherSettings().SeedanceAssetAPIFormat
	if format == "" {
		format = "gatewayMg" // 默认咪咕格式
	}

	// 根据格式转换请求
	var method, upstreamURL string
	var requestBody []byte

	if format == "gatewayMg" {
		// 转换为咪咕 RESTful 格式
		method, upstreamURL, requestBody, err = convertActionToGatewayMg(gw.GatewayURL, action, bodyBytes)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": map[string]interface{}{
					"message": fmt.Sprintf("convert to gatewayMg format failed: %s", err.Error()),
					"type":    "invalid_request_error",
				},
			})
			return
		}
	} else if format == "official" {
		// 保持火山 Action 格式
		method = "POST"
		upstreamURL = fmt.Sprintf("%s/?Action=%s&Version=%s", strings.TrimSuffix(gw.GatewayURL, "/"), action, version)
		requestBody = bodyBytes
	} else {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": map[string]interface{}{
				"message": fmt.Sprintf("unsupported api format: %s", format),
				"type":    "invalid_request_error",
			},
		})
		return
	}

	// 打印日志
	logger.LogInfo(c, fmt.Sprintf("asset upstream URL: %s", upstreamURL))
	logger.LogInfo(c, fmt.Sprintf("asset upstream method: %s", method))
	if len(requestBody) > 0 {
		logger.LogInfo(c, fmt.Sprintf("asset upstream body: %s", string(requestBody)))
	}

	// 创建上游请求
	req, err := http.NewRequest(method, upstreamURL, bytes.NewBuffer(requestBody))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": map[string]interface{}{
				"message": fmt.Sprintf("failed to create request: %s", err.Error()),
				"type":    "internal_error",
			},
		})
		return
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+gw.Key)
	req.Header.Set("Accept", "application/json")

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"error": map[string]interface{}{
				"message": fmt.Sprintf("upstream request failed: %s", err.Error()),
				"type":    "upstream_error",
			},
		})
		return
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": map[string]interface{}{
				"message": fmt.Sprintf("failed to read response: %s", err.Error()),
				"type":    "internal_error",
			},
		})
		return
	}

	// 返回响应
	c.Data(resp.StatusCode, "application/json", respBody)
}

// convertActionToGatewayMg 将火山 Action 格式转换为咪咕 Gateway RESTful 格式
func convertActionToGatewayMg(baseURL, action string, bodyBytes []byte) (method, url string, newBody []byte, err error) {
	var reqBody map[string]interface{}
	if len(bodyBytes) > 0 {
		if err := json.Unmarshal(bodyBytes, &reqBody); err != nil {
			return "", "", nil, fmt.Errorf("parse request body failed: %w", err)
		}
	} else {
		reqBody = make(map[string]interface{})
	}

	baseURL = strings.TrimSuffix(baseURL, "/")

	switch action {
	// ========== 素材组接口 ==========
	case "CreateAssetGroup":
		return "POST", baseURL + "/api/seedance/proxy/assets/groups", bodyBytes, nil

	case "ListAssetGroups":
		// GET 请求，参数从 body 转为 query
		query := buildQueryFromBody(reqBody)
		url := baseURL + "/api/seedance/proxy/assets/groups"
		if query != "" {
			url += "?" + query
		}
		return "GET", url, nil, nil

	case "GetAssetGroup":
		groupID, ok := reqBody["GroupId"].(string)
		if !ok {
			return "", "", nil, fmt.Errorf("GroupId is required")
		}
		return "GET", fmt.Sprintf("%s/api/seedance/proxy/assets/groups/%s", baseURL, groupID), nil, nil

	case "UpdateAssetGroup":
		groupID, ok := reqBody["GroupId"].(string)
		if !ok {
			return "", "", nil, fmt.Errorf("GroupId is required")
		}
		return "PUT", fmt.Sprintf("%s/api/seedance/proxy/assets/groups/%s", baseURL, groupID), bodyBytes, nil

	case "DeleteAssetGroup":
		groupID, ok := reqBody["GroupId"].(string)
		if !ok {
			return "", "", nil, fmt.Errorf("GroupId is required")
		}
		return "DELETE", fmt.Sprintf("%s/api/seedance/proxy/assets/groups/%s", baseURL, groupID), nil, nil

	// ========== 素材接口 ==========
	case "CreateAsset":
		return "POST", baseURL + "/api/seedance/proxy/assets", bodyBytes, nil

	case "ListAssets":
		query := buildQueryFromBody(reqBody)
		url := baseURL + "/api/seedance/proxy/assets"
		if query != "" {
			url += "?" + query
		}
		return "GET", url, nil, nil

	case "GetAsset":
		assetID, ok := reqBody["AssetId"].(string)
		if !ok {
			return "", "", nil, fmt.Errorf("AssetId is required")
		}
		return "GET", fmt.Sprintf("%s/api/seedance/proxy/assets/%s", baseURL, assetID), nil, nil

	case "UpdateAsset":
		assetID, ok := reqBody["AssetId"].(string)
		if !ok {
			return "", "", nil, fmt.Errorf("AssetId is required")
		}
		return "PUT", fmt.Sprintf("%s/api/seedance/proxy/assets/%s", baseURL, assetID), bodyBytes, nil

	case "DeleteAsset":
		assetID, ok := reqBody["AssetId"].(string)
		if !ok {
			return "", "", nil, fmt.Errorf("AssetId is required")
		}
		return "DELETE", fmt.Sprintf("%s/api/seedance/proxy/assets/%s", baseURL, assetID), nil, nil

	// ========== 真人认证接口 ==========
	case "CreateVisualValidateSession":
		return "POST", baseURL + "/api/seedance/proxy/face-verifications", bodyBytes, nil

	case "GetVisualValidateResult":
		sessionID, ok := reqBody["SessionId"].(string)
		if !ok {
			return "", "", nil, fmt.Errorf("SessionId is required")
		}
		return "GET", fmt.Sprintf("%s/api/seedance/proxy/face-verifications/%s", baseURL, sessionID), nil, nil

	default:
		return "", "", nil, fmt.Errorf("unsupported action: %s", action)
	}
}

// buildQueryFromBody 从请求体中提取查询参数（用于 List 接口）
func buildQueryFromBody(body map[string]interface{}) string {
	params := url.Values{}

	// 分页参数
	if pageNum, ok := body["PageNumber"].(float64); ok {
		params.Add("PageNumber", fmt.Sprintf("%d", int(pageNum)))
	}
	if pageSize, ok := body["PageSize"].(float64); ok {
		params.Add("PageSize", fmt.Sprintf("%d", int(pageSize)))
	}

	// 素材组过滤参数
	if groupID, ok := body["GroupId"].(string); ok && groupID != "" {
		params.Add("GroupId", groupID)
	}

	// 素材类型过滤
	if assetType, ok := body["AssetType"].(string); ok && assetType != "" {
		params.Add("AssetType", assetType)
	}

	// 状态过滤
	if status, ok := body["Status"].(string); ok && status != "" {
		params.Add("Status", status)
	}

	return params.Encode()
}
