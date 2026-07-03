package controller

import (
	"io"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// resolveRole 获取用户角色，Token 鉴权时不设置 role 需从 DB 查
func resolveRole(c *gin.Context) int {
	if role := c.GetInt("role"); role > 0 {
		return role
	}
	if u, err := model.GetUserById(c.GetInt("id"), false); err == nil {
		return u.Role
	}
	return 0
}

func isAdminUser(c *gin.Context) bool {
	return resolveRole(c) >= common.RoleAdminUser
}

func CreateVideoProject(c *gin.Context) {
	var req dto.CreateVideoProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error(), "data": nil})
		return
	}

	userId := c.GetInt("id")
	isAdmin := isAdminUser(c)

	// 与 relay 统一：使用 TokenAuth 解析后的分组
	req.TokenGroup = common.GetContextKeyString(c, constant.ContextKeyUsingGroup)

	project, err := service.CreateProject(c.Request.Context(), userId, isAdmin, &req)
	if err != nil {
		model.RecordConsumeLog(c, userId, model.RecordConsumeLogParams{
			ModelName: req.VideoModel,
			TokenName: c.GetString("token_name"),
			TokenId:   c.GetInt("token_id"),
			Content:   "视频生成失败: " + err.Error(),
			Quota:     0,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error(), "data": nil})
		return
	}

	model.RecordConsumeLog(c, userId, model.RecordConsumeLogParams{
		ModelName: req.VideoModel,
		TokenName: c.GetString("token_name"),
		TokenId:   c.GetInt("token_id"),
		Content:   "视频生成成功: id=" + strconv.FormatInt(project.Id, 10),
		Quota:     0,
	})

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
	isAdmin := isAdminUser(c)

	detail, err := service.GetProject(c.Request.Context(), projectId, userId, isAdmin)
	if err != nil {
		model.RecordConsumeLog(c, userId, model.RecordConsumeLogParams{
			ModelName: "",
			TokenName: c.GetString("token_name"),
			TokenId:   c.GetInt("token_id"),
			Content:   "视频查询失败: project=" + c.Param("id"),
			Quota:     0,
		})
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "project not found", "data": nil})
		return
	}

	model.RecordConsumeLog(c, userId, model.RecordConsumeLogParams{
		ModelName: "",
		TokenName: c.GetString("token_name"),
		TokenId:   c.GetInt("token_id"),
		Content:   "视频查询成功: id=" + strconv.FormatInt(detail.Id, 10),
		Quota:     0,
	})

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "success",
		"data": dto.VideoProjectDetailResponse{
			ProjectId:        detail.Id,
			ProjectName:      detail.ProjectName,
			Status:           detail.Status,
			ErrorMsg:         detail.ErrorMsg,
			Progress:         detail.Progress,
			ProductImgUrl:    detail.ProductImgUrl,
			Brand:            detail.Brand,
			ProductName:      detail.ProductName,
			MainImageUrl:     detail.MainImageUrl,
			MainImageAssetId: detail.MainImageAssetId,
			GeneratedResult:  detail.GeneratedResult,
			FirstVideoUrl:    detail.FirstVideoUrl,
			CreatedAt:        detail.CreatedAt.Unix(),
			UpdatedAt:        detail.UpdatedAt.Unix(),
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
	isAdmin := isAdminUser(c)
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
	isAdmin := isAdminUser(c)

	if err := service.DeleteProject(c.Request.Context(), projectId, userId, isAdmin); err != nil {
		model.RecordConsumeLog(c, userId, model.RecordConsumeLogParams{
			ModelName: "",
			TokenName: c.GetString("token_name"),
			TokenId:   c.GetInt("token_id"),
			Content:   "视频删除失败: project=" + c.Param("id") + " " + err.Error(),
			Quota:     0,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error(), "data": nil})
		return
	}

	model.RecordConsumeLog(c, userId, model.RecordConsumeLogParams{
		ModelName: "",
		TokenName: c.GetString("token_name"),
		TokenId:   c.GetInt("token_id"),
		Content:   "视频删除成功: id=" + c.Param("id"),
		Quota:     0,
	})

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
