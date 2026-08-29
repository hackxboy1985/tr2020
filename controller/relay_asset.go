package controller

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/dto"
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

	// 构建上游请求 URL
	// 使用 Gateway URL + Action 参数
	upstreamURL := fmt.Sprintf("%s/?Action=%s&Version=%s", strings.TrimSuffix(gw.GatewayURL, "/"), action, version)

	// 创建上游请求
	req, err := http.NewRequest("POST", upstreamURL, bytes.NewBuffer(bodyBytes))
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
