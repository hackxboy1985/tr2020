package controller

import (
	"io"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func CreateVideoProject(c *gin.Context) {
	var req dto.CreateVideoProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error(), "data": nil})
		return
	}

	userId := c.GetInt("id")
	isAdmin := c.GetInt("role") >= common.RoleAdminUser

	project, err := service.CreateProject(c.Request.Context(), userId, isAdmin, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error(), "data": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "video project created successfully",
		"data": dto.VideoProjectResponse{
			ProjectId:   project.Id,
			ProjectName: project.ProjectName,
			Status:      project.Status,
			CreatedAt:   project.CreatedAt.Unix(),
		},
	})
}

func GetVideoProject(c *gin.Context) {
	projectId, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid project id", "data": nil})
		return
	}

	userId := c.GetInt("id")
	isAdmin := c.GetInt("role") >= common.RoleAdminUser

	project, err := service.GetProject(c.Request.Context(), projectId, userId, isAdmin)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "project not found", "data": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "success",
		"data": dto.VideoProjectDetailResponse{
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
		},
	})
}

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
	isAdmin := c.GetInt("role") >= common.RoleAdminUser
	statusFilter := c.Query("status")

	resp, err := service.ListProjects(c.Request.Context(), userId, page, pageSize, isAdmin, statusFilter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error(), "data": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "success", "data": resp})
}

func DeleteVideoProject(c *gin.Context) {
	projectId, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid project id", "data": nil})
		return
	}

	userId := c.GetInt("id")
	isAdmin := c.GetInt("role") >= common.RoleAdminUser

	if err := service.DeleteProject(c.Request.Context(), projectId, userId, isAdmin); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error(), "data": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "project deleted successfully", "data": nil})
}

func UpdateVideoProjectStatus(c *gin.Context) {
	projectId, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid project id", "data": nil})
		return
	}

	var req dto.UpdateVideoProjectStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error(), "data": nil})
		return
	}

	if err := service.UpdateProjectStatus(c.Request.Context(), projectId, &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error(), "data": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "status updated successfully", "data": nil})
}

func HandleWebhook(c *gin.Context) {
	channelId, err := strconv.Atoi(c.Param("channel_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid channel_id", "data": nil})
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "failed to read body", "data": nil})
		return
	}

	signature := c.GetHeader("X-Signature")

	if err := service.HandleWebhook(c.Request.Context(), channelId, signature, body); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": err.Error(), "data": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok", "data": nil})
}
