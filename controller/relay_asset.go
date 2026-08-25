package controller

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"

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

	// 从中间件获取渠道信息（Distribute 中间件已设置）
	channelId := c.GetInt("channel_id")
	if channelId == 0 {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": map[string]interface{}{
				"message": "no available channel",
				"type":    "invalid_request_error",
			},
		})
		return
	}

	// 获取渠道详情
	channel, err := model.GetChannelById(channelId, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": map[string]interface{}{
				"message": fmt.Sprintf("failed to get channel: %s", err.Error()),
				"type":    "internal_error",
			},
		})
		return
	}

	// 获取渠道的 BaseURL 和 Key
	baseURL := channel.GetBaseURL()
	apiKey := channel.Key

	// 构建上游请求 URL
	// 火山引擎格式：POST https://ark.cn-beijing.volces.com/?Action=CreateAsset&Version=2024-01-01
	upstreamURL := fmt.Sprintf("%s/?Action=%s&Version=%s", strings.TrimSuffix(baseURL, "/"), action, version)

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
	req.Header.Set("Authorization", apiKey) // 可能是 Bearer Token 或 AK/SK

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
