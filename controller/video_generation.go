package controller

import (
	"io"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// CreateVideoProject 创建视频生成项目
func CreateVideoProject(c *gin.Context) {
	var req dto.CreateVideoProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": http.StatusBadRequest,
			"msg":  "invalid request: " + err.Error(),
			"data": nil,
		})
		return
	}

	userId := c.GetInt("id")

	// 获取渠道类型：优先使用请求参数，否则使用系统默认配置
	channelType := req.ChannelType
	if channelType == "" {
		channelType = service.GetDefaultChannelType()
	}

	// 创建服务实例
	videoService, err := service.NewVideoGenerationService(channelType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": http.StatusInternalServerError,
			"msg":  "failed to initialize service: " + err.Error(),
			"data": nil,
		})
		return
	}

	// 创建项目（包含调用上游API）
	project, err := videoService.CreateProject(c.Request.Context(), userId, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": http.StatusInternalServerError,
			"msg":  "failed to create video project: " + err.Error(),
			"data": nil,
		})
		return
	}

	resp := dto.VideoProjectResponse{
		ProjectId:   project.Id,
		ProjectName: project.ProjectName,
		Status:      project.Status,
		CreatedAt:   project.CreatedAt.Unix(),
	}

	c.JSON(http.StatusOK, gin.H{
		"code": http.StatusOK,
		"msg":  "video project created successfully",
		"data": resp,
	})
}

// GetVideoProject 获取视频项目详情
func GetVideoProject(c *gin.Context) {
	projectId, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": http.StatusBadRequest,
			"msg":  "invalid project id",
			"data": nil,
		})
		return
	}

	userId := c.GetInt("id")
	isAdmin := c.GetBool("is_admin")

	// 获取项目（会自动从数据库获取channel_type）
	var project *model.VideoProject
	if isAdmin {
		project, err = model.GetVideoProjectByIdAdmin(projectId)
	} else {
		project, err = model.GetVideoProjectById(projectId, userId)
	}

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code": http.StatusNotFound,
			"msg":  "video project not found",
			"data": nil,
		})
		return
	}

	// 创建对应渠道的服务实例
	videoService, err := service.NewVideoGenerationService(project.ChannelType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": http.StatusInternalServerError,
			"msg":  "failed to initialize service: " + err.Error(),
			"data": nil,
		})
		return
	}

	// 获取最新状态（会自动同步上游状态）
	project, err = videoService.GetProject(c.Request.Context(), projectId, userId, isAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": http.StatusInternalServerError,
			"msg":  "failed to get project: " + err.Error(),
			"data": nil,
		})
		return
	}

	resp := dto.VideoProjectDetailResponse{
		ProjectId:        project.Id,
		ProjectName:      project.ProjectName,
		Status:           project.Status,
		ErrorMsg:         project.ErrorMsg,
		Progress:         project.Progress,
		ProductImgUrl:    project.ProductImgUrl,
		Brand:            project.Brand,
		ProductName:      project.ProductName,
		MainImageUrl:     project.MainImageUrl,
		MainImageAssetId: project.MainImageAssetId,
		GeneratedResult:  project.GeneratedResult,
		FirstVideoUrl:    project.FirstVideoUrl,
		CreatedAt:        project.CreatedAt.Unix(),
		UpdatedAt:        project.UpdatedAt.Unix(),
	}

	c.JSON(http.StatusOK, gin.H{
		"code": http.StatusOK,
		"msg":  "success",
		"data": resp,
	})
}

// ListVideoProjects 获取视频项目列表
func ListVideoProjects(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	userId := c.GetInt("id")
	isAdmin := c.GetBool("is_admin")
	statusFilter := c.Query("status")

	// 使用默认渠道服务
	videoService, err := service.NewVideoGenerationService(service.GetDefaultChannelType())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": http.StatusInternalServerError,
			"msg":  "failed to initialize service: " + err.Error(),
			"data": nil,
		})
		return
	}

	resp, err := videoService.ListProjects(c.Request.Context(), userId, page, pageSize, isAdmin, statusFilter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": http.StatusInternalServerError,
			"msg":  "failed to list projects: " + err.Error(),
			"data": nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": http.StatusOK,
		"msg":  "success",
		"data": resp,
	})
}

// DeleteVideoProject 删除视频项目
func DeleteVideoProject(c *gin.Context) {
	projectId, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": http.StatusBadRequest,
			"msg":  "invalid project id",
			"data": nil,
		})
		return
	}

	userId := c.GetInt("id")
	isAdmin := c.GetBool("is_admin")

	// 使用默认渠道服务
	videoService, err := service.NewVideoGenerationService(service.GetDefaultChannelType())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": http.StatusInternalServerError,
			"msg":  "failed to initialize service: " + err.Error(),
			"data": nil,
		})
		return
	}

	err = videoService.DeleteProject(c.Request.Context(), projectId, userId, isAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": http.StatusInternalServerError,
			"msg":  "failed to delete project: " + err.Error(),
			"data": nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": http.StatusOK,
		"msg":  "video project deleted successfully",
		"data": nil,
	})
}

// UpdateVideoProjectStatus 管理员更新项目状态
func UpdateVideoProjectStatus(c *gin.Context) {
	projectId, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": http.StatusBadRequest,
			"msg":  "invalid project id",
			"data": nil,
		})
		return
	}

	var req dto.UpdateVideoProjectStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": http.StatusBadRequest,
			"msg":  "invalid request: " + err.Error(),
			"data": nil,
		})
		return
	}

	// 使用默认渠道服务
	videoService, err := service.NewVideoGenerationService(service.GetDefaultChannelType())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": http.StatusInternalServerError,
			"msg":  "failed to initialize service: " + err.Error(),
			"data": nil,
		})
		return
	}

	err = videoService.UpdateProjectStatus(c.Request.Context(), projectId, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": http.StatusInternalServerError,
			"msg":  "failed to update project status: " + err.Error(),
			"data": nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": http.StatusOK,
		"msg":  "project status updated successfully",
		"data": nil,
	})
}

// HandleWebhook 处理 Webhook 回调（通用）
func HandleWebhook(c *gin.Context) {
	channelType := c.Param("channel") // 'coze' 或 'platform'

	// 读取请求体
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": http.StatusBadRequest,
			"msg":  "failed to read request body",
			"data": nil,
		})
		return
	}

	signature := c.GetHeader("X-Signature")

	// 创建对应渠道的服务实例
	videoService, err := service.NewVideoGenerationService(channelType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": http.StatusInternalServerError,
			"msg":  "failed to initialize service: " + err.Error(),
			"data": nil,
		})
		return
	}

	// 处理 webhook
	err = videoService.HandleWebhook(c.Request.Context(), signature, body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": http.StatusInternalServerError,
			"msg":  "failed to handle webhook: " + err.Error(),
			"data": nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": http.StatusOK,
		"msg":  "webhook processed successfully",
		"data": nil,
	})
}
