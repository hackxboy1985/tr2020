package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/dto"

	"github.com/gin-gonic/gin"
)

// RelayAsset handles POST /api/seedance/assets/v2/?Action=XXX&Version=2024-01-01
// 将火山官方 Action 格式的请求转换为调用现有的 Seedance RESTful 函数
// 这样可以复用现有的用户隔离逻辑和数据库同步逻辑
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

	// 设置标志，让 List 接口返回火山官方格式
	c.Set("doubao_official_format", true)

	// 根据 Action 调用对应的函数
	// 这些函数已经实现了用户隔离和数据库同步逻辑
	switch action {
	// ========== 素材组接口 ==========
	case "CreateAssetGroup":
		SeedanceCreateAssetGroup(c)
	case "ListAssetGroups":
		SeedanceListAssetGroups(c)
	case "GetAssetGroup":
		handleGetAssetGroup(c)
	case "UpdateAssetGroup":
		handleUpdateAssetGroup(c)
	case "DeleteAssetGroup":
		handleDeleteAssetGroup(c)

	// ========== 素材接口 ==========
	case "CreateAsset":
		SeedanceCreateAsset(c)
	case "ListAssets":
		SeedanceListAssets(c)
	case "GetAsset":
		handleGetAsset(c)
	case "UpdateAsset":
		handleUpdateAsset(c)
	case "DeleteAsset":
		handleDeleteAsset(c)

	// ========== 真人认证接口 ==========
	case "CreateVisualValidateSession":
		SeedanceCreateFaceVerification(c)
	case "GetVisualValidateResult":
		handleGetVisualValidateResult(c)

	default:
		c.JSON(http.StatusBadRequest, &dto.TaskError{
			Code:       "unsupported_action",
			Message:    fmt.Sprintf("Action '%s' is not implemented", action),
			StatusCode: http.StatusBadRequest,
		})
	}
}

// handleGetAssetGroup 从 body 中提取 GroupId 或 Id 并调用 SeedanceGetAssetGroup
func handleGetAssetGroup(c *gin.Context) {
	bodyBytes, _ := io.ReadAll(c.Request.Body)
	var reqBody map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": map[string]interface{}{
			"message": "invalid request body",
			"type":    "invalid_request_error",
		}})
		return
	}

	// 兼容官方格式：优先使用 GroupId，其次使用 Id
	var groupID string
	if id, ok := reqBody["GroupId"].(string); ok && id != "" {
		groupID = id
	} else if id, ok := reqBody["Id"].(string); ok && id != "" {
		groupID = id
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": map[string]interface{}{
			"message": "GroupId or Id is required",
			"type":    "invalid_request_error",
		}})
		return
	}

	// 将 GroupId 设置到 URL 参数中，供 SeedanceGetAssetGroup 使用
	c.Params = append(c.Params, gin.Param{Key: "id", Value: groupID})
	SeedanceGetAssetGroup(c)
}

// handleUpdateAssetGroup 从 body 中提取 GroupId 或 Id 并调用 SeedancePutAssetGroup
func handleUpdateAssetGroup(c *gin.Context) {
	bodyBytes, _ := io.ReadAll(c.Request.Body)
	var reqBody map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": map[string]interface{}{
			"message": "invalid request body",
			"type":    "invalid_request_error",
		}})
		return
	}

	// 兼容官方格式：优先使用 GroupId，其次使用 Id
	var groupID string
	if id, ok := reqBody["GroupId"].(string); ok && id != "" {
		groupID = id
	} else if id, ok := reqBody["Id"].(string); ok && id != "" {
		groupID = id
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": map[string]interface{}{
			"message": "GroupId or Id is required",
			"type":    "invalid_request_error",
		}})
		return
	}

	// 将 GroupId 设置到 URL 参数中
	c.Params = append(c.Params, gin.Param{Key: "id", Value: groupID})
	SeedancePutAssetGroup(c)
}

// handleDeleteAssetGroup 从 body 中提取 GroupId 或 Id 并调用 SeedanceDeleteAssetGroup
func handleDeleteAssetGroup(c *gin.Context) {
	bodyBytes, _ := io.ReadAll(c.Request.Body)
	var reqBody map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": map[string]interface{}{
			"message": "invalid request body",
			"type":    "invalid_request_error",
		}})
		return
	}

	// 兼容官方格式：优先使用 GroupId，其次使用 Id
	var groupID string
	if id, ok := reqBody["GroupId"].(string); ok && id != "" {
		groupID = id
	} else if id, ok := reqBody["Id"].(string); ok && id != "" {
		groupID = id
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": map[string]interface{}{
			"message": "GroupId or Id is required",
			"type":    "invalid_request_error",
		}})
		return
	}

	// 将 GroupId 设置到 URL 参数中
	c.Params = append(c.Params, gin.Param{Key: "id", Value: groupID})
	SeedanceDeleteAssetGroup(c)
}

// handleGetAsset 从 body 中提取 AssetId 或 Id 并调用 SeedanceGetAsset
func handleGetAsset(c *gin.Context) {
	bodyBytes, _ := io.ReadAll(c.Request.Body)
	var reqBody map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": map[string]interface{}{
			"message": "invalid request body",
			"type":    "invalid_request_error",
		}})
		return
	}

	// 兼容官方格式：优先使用 AssetId，其次使用 Id
	var assetID string
	if id, ok := reqBody["AssetId"].(string); ok && id != "" {
		assetID = id
	} else if id, ok := reqBody["Id"].(string); ok && id != "" {
		assetID = id
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": map[string]interface{}{
			"message": "AssetId or Id is required",
			"type":    "invalid_request_error",
		}})
		return
	}

	// 将 AssetId 设置到 URL 参数中
	c.Params = append(c.Params, gin.Param{Key: "id", Value: assetID})
	SeedanceGetAsset(c)
}

// handleUpdateAsset 从 body 中提取 AssetId 或 Id 并调用 SeedancePutAsset
func handleUpdateAsset(c *gin.Context) {
	bodyBytes, _ := io.ReadAll(c.Request.Body)
	var reqBody map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": map[string]interface{}{
			"message": "invalid request body",
			"type":    "invalid_request_error",
		}})
		return
	}

	// 兼容官方格式：优先使用 AssetId，其次使用 Id
	var assetID string
	if id, ok := reqBody["AssetId"].(string); ok && id != "" {
		assetID = id
	} else if id, ok := reqBody["Id"].(string); ok && id != "" {
		assetID = id
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": map[string]interface{}{
			"message": "AssetId or Id is required",
			"type":    "invalid_request_error",
		}})
		return
	}

	// 从 reqBody 中移除 Id 和 AssetId，只保留 Name、ProjectName（忽略 Tags 等其他字段）
	delete(reqBody, "Id")
	delete(reqBody, "AssetId")
	delete(reqBody, "Tags") // 忽略 Tags 字段，保持与官方一致

	// 重新编码 body 供后续 BindJSON 使用
	newBodyBytes, _ := json.Marshal(reqBody)
	c.Request.Body = io.NopCloser(bytes.NewReader(newBodyBytes))

	// 将 AssetId 设置到 URL 参数中
	c.Params = append(c.Params, gin.Param{Key: "id", Value: assetID})
	SeedancePutAsset(c)
}

// handleDeleteAsset 从 body 中提取 AssetId 或 Id 并调用 SeedanceDeleteAsset
func handleDeleteAsset(c *gin.Context) {
	bodyBytes, _ := io.ReadAll(c.Request.Body)
	var reqBody map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": map[string]interface{}{
			"message": "invalid request body",
			"type":    "invalid_request_error",
		}})
		return
	}

	// 兼容官方格式：优先使用 AssetId，其次使用 Id
	var assetID string
	if id, ok := reqBody["AssetId"].(string); ok && id != "" {
		assetID = id
	} else if id, ok := reqBody["Id"].(string); ok && id != "" {
		assetID = id
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": map[string]interface{}{
			"message": "AssetId or Id is required",
			"type":    "invalid_request_error",
		}})
		return
	}

	// 将 AssetId 设置到 URL 参数中
	c.Params = append(c.Params, gin.Param{Key: "id", Value: assetID})
	SeedanceDeleteAsset(c)
}

// handleGetVisualValidateResult 从 body 中提取 SessionId 并调用 SeedanceGetFaceVerification
func handleGetVisualValidateResult(c *gin.Context) {
	bodyBytes, _ := io.ReadAll(c.Request.Body)
	var reqBody map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": map[string]interface{}{
			"message": "invalid request body",
			"type":    "invalid_request_error",
		}})
		return
	}

	sessionID, ok := reqBody["SessionId"].(string)
	if !ok || sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": map[string]interface{}{
			"message": "SessionId is required",
			"type":    "invalid_request_error",
		}})
		return
	}

	// 将 SessionId 设置到 URL 参数中
	c.Params = append(c.Params, gin.Param{Key: "id", Value: sessionID})
	SeedanceGetFaceVerification(c)
}
