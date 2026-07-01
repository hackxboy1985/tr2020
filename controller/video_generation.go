package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// CreateVideoProject 创建视频生成项目
func CreateVideoProject(c *gin.Context) {
	var req dto.VideoGenerationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": http.StatusBadRequest,
			"msg":  "invalid request: " + err.Error(),
			"data": nil,
		})
		return
	}

	userId := c.GetInt("id")

	// 生成项目名称（用户名+日期+时间戳）
	username := c.GetString("username")
	projectName := fmt.Sprintf("%s_%s_%d", username, time.Now().Format("20060102"), time.Now().Unix())

	project := &model.VideoProject{
		ProjectName:   projectName,
		UserId:        userId,
		ProductImgUrl: req.ProductImgUrl,
		Brand:         req.Brand,
		ProductName:   req.ProductName,
		Tagline:       req.Tagline,
		SellingPoints: req.SellingPoints,
		Prompt:        req.Prompt,
		Vtype:         req.Vtype,
		VtypeAdd:      req.VtypeAdd,
		Language:      req.Language,
		Platform:      req.Platform,
		Region:        req.Region,
		Roles:         req.Roles,
		SelectAudios:  req.SelectAudios,
		Duration:      req.Duration,
		Resolution:    req.Resolution,
		VideoModel:    req.VideoModel,
		Whstr:         req.Whstr,
		Status:        model.VideoProjectStatusCreated,
		Deleted:       0,
	}

	err := model.CreateVideoProject(project)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": http.StatusInternalServerError,
			"msg":  "failed to create video project: " + err.Error(),
			"data": nil,
		})
		return
	}

	// TODO: 这里应该调用 Coze 工作流接口，触发视频生成
	// 暂时返回项目创建成功，后续需要实现异步调用逻辑

	resp := dto.VideoGenerationResponse{
		ProjectID:   project.Id,
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

	// TODO: 这里可以查询关联的视频 URL（从 good_project_media 表）
	// 暂时不实现，需要时再补充

	resp := &dto.VideoProjectStatus{
		ProjectID:        project.Id,
		ProjectName:      project.ProjectName,
		Status:           project.Status,
		ErrorMsg:         project.ErrorMsg,
		ProductImgUrl:    project.ProductImgUrl,
		Brand:            project.Brand,
		ProductName:      project.ProductName,
		MainImageUrl:     project.MainImageUrl,
		MainImageAssetId: project.MainImageAssetId,
		GeneratedResult:  project.GeneratedResult,
		CreatedAt:        project.CreatedAt.Unix(),
		UpdatedAt:        project.UpdatedAt.Unix(),
	}

	c.JSON(http.StatusOK, gin.H{
		"code": http.StatusOK,
		"msg":  "success",
		"data": resp,
	})
}

// GetUserVideoProjects 获取用户的视频项目列表
func GetUserVideoProjects(c *gin.Context) {
	userId := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)

	projects, total, err := model.GetUserVideoProjects(userId, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": http.StatusInternalServerError,
			"msg":  "failed to get video projects: " + err.Error(),
			"data": nil,
		})
		return
	}

	// 转换为响应格式
	projectList := make([]*dto.VideoProjectStatus, 0, len(projects))
	for _, p := range projects {
		projectList = append(projectList, &dto.VideoProjectStatus{
			ProjectID:   p.Id,
			ProjectName: p.ProjectName,
			Status:      p.Status,
			ErrorMsg:    p.ErrorMsg,
			Brand:       p.Brand,
			ProductName: p.ProductName,
			CreatedAt:   p.CreatedAt.Unix(),
			UpdatedAt:   p.UpdatedAt.Unix(),
		})
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(projectList)

	c.JSON(http.StatusOK, gin.H{
		"code": http.StatusOK,
		"msg":  "success",
		"data": pageInfo,
	})
}

// GetAllVideoProjects 管理员获取所有视频项目列表
func GetAllVideoProjects(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	status := c.Query("status")

	projects, total, err := model.GetAllVideoProjects(pageInfo.GetStartIdx(), pageInfo.GetPageSize(), status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": http.StatusInternalServerError,
			"msg":  "failed to get video projects: " + err.Error(),
			"data": nil,
		})
		return
	}

	// 转换为响应格式
	projectList := make([]*dto.VideoProjectStatus, 0, len(projects))
	for _, p := range projects {
		projectList = append(projectList, &dto.VideoProjectStatus{
			ProjectID:   p.Id,
			ProjectName: p.ProjectName,
			Status:      p.Status,
			ErrorMsg:    p.ErrorMsg,
			Brand:       p.Brand,
			ProductName: p.ProductName,
			CreatedAt:   p.CreatedAt.Unix(),
			UpdatedAt:   p.UpdatedAt.Unix(),
		})
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(projectList)

	c.JSON(http.StatusOK, gin.H{
		"code": http.StatusOK,
		"msg":  "success",
		"data": pageInfo,
	})
}

// UpdateVideoProjectStatus 更新视频项目状态（管理员）
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

	var req struct {
		Status           string `json:"status" binding:"required"`
		ErrorMsg         string `json:"error_msg,omitempty"`
		MainImageUrl     string `json:"main_image_url,omitempty"`
		MainImageAssetId string `json:"main_image_asset_id,omitempty"`
		GeneratedResult  string `json:"generated_result,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": http.StatusBadRequest,
			"msg":  "invalid request: " + err.Error(),
			"data": nil,
		})
		return
	}

	// 如果有 Coze 回调结果，更新相关字段
	if req.MainImageUrl != "" || req.GeneratedResult != "" {
		err = model.UpdateVideoProjectCozeResult(projectId, req.MainImageUrl, req.MainImageAssetId, req.GeneratedResult)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code": http.StatusInternalServerError,
				"msg":  "failed to update coze result: " + err.Error(),
				"data": nil,
			})
			return
		}
	}

	// 更新状态
	err = model.UpdateVideoProjectStatus(projectId, req.Status, req.ErrorMsg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": http.StatusInternalServerError,
			"msg":  "failed to update video project status: " + err.Error(),
			"data": nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": http.StatusOK,
		"msg":  "video project status updated successfully",
		"data": nil,
	})
}

// DeleteVideoProject 删除视频项目（用户）
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

	err = model.DeleteVideoProject(projectId, userId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": http.StatusInternalServerError,
			"msg":  "failed to delete video project: " + err.Error(),
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

// DeleteVideoProjectAdmin 删除视频项目（管理员）
func DeleteVideoProjectAdmin(c *gin.Context) {
	projectId, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": http.StatusBadRequest,
			"msg":  "invalid project id",
			"data": nil,
		})
		return
	}

	err = model.DeleteVideoProjectAdmin(projectId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": http.StatusInternalServerError,
			"msg":  "failed to delete video project: " + err.Error(),
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

// CozeWebhook Coze 工作流回调接口
func CozeWebhook(c *gin.Context) {
	// TODO: 实现签名验证，确保请求来自 Coze

	var req struct {
		ProjectID        int64  `json:"project_id" binding:"required"`
		Status           string `json:"status" binding:"required"`
		ErrorMsg         string `json:"error_msg,omitempty"`
		MainImageUrl     string `json:"main_image_url,omitempty"`
		MainImageAssetId string `json:"main_image_asset_id,omitempty"`
		GeneratedResult  string `json:"generated_result,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": http.StatusBadRequest,
			"msg":  "invalid request: " + err.Error(),
			"data": nil,
		})
		return
	}

	// 如果有 Coze 回调结果，更新相关字段
	if req.MainImageUrl != "" || req.GeneratedResult != "" {
		err := model.UpdateVideoProjectCozeResult(req.ProjectID, req.MainImageUrl, req.MainImageAssetId, req.GeneratedResult)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code": http.StatusInternalServerError,
				"msg":  "failed to update coze result: " + err.Error(),
				"data": nil,
			})
			return
		}
	}

	// 更新状态
	err := model.UpdateVideoProjectStatus(req.ProjectID, req.Status, req.ErrorMsg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": http.StatusInternalServerError,
			"msg":  "failed to update video project status: " + err.Error(),
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
