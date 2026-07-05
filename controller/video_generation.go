package controller

import (
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"fmt"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// videoQPSLimiter 视频创建 QPS 限流器（单机内存模式）
var videoQPSLimiter common.InMemoryRateLimiter

func init() {
	videoQPSLimiter.Init(30 * time.Second)
}

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

	// 获取用户分组对应的渠道预扣配置
	userGroup := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	if userGroup == "" {
		userGroup = "default"
	}
	// 根据渠道配置计算预扣金额
	preDeductQuota := 0
	if chs, err := model.GetEnabledVideoChannelsForGroup(userGroup, ""); err == nil && len(chs) > 0 {
		ch := chs[0]
		// 预扣 = duration * price_per_second
		if ch.GetPricePerSecond(req.VideoModel) > 0 {
			preDeductQuota = req.Duration * ch.GetPricePerSecond(req.VideoModel)
		} else if ch.PreDeductQuota > 0 {
			preDeductQuota = ch.PreDeductQuota
		}
		// 应用 QPS 限制
		if ch.RateLimit > 0 {
			qpsKey := fmt.Sprintf("vc_qps_%d", userId)
			if !videoQPSLimiter.Request(qpsKey, ch.RateLimit, 1) {
				c.JSON(http.StatusTooManyRequests, gin.H{
					"code": 429, "msg": "请求频率超过限制，请稍后再试", "data": nil,
				})
				return
			}
		}
	}

	// 检查余额是否足够
	if quota, _ := model.GetUserQuota(userId, false); quota < preDeductQuota {
		c.JSON(http.StatusOK, gin.H{
			"code": 400,
			"msg":  fmt.Sprintf("余额不足，至少需要 %d 积分，当前余额 %d 积分", preDeductQuota, quota),
			"data": nil,
		})
		return
	}

	// 预扣积分
	if err := model.DecreaseUserQuota(userId, preDeductQuota, false); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "预扣积分失败: " + err.Error(), "data": nil})
		return
	}

	// 与 relay 统一：使用 TokenAuth 解析后的分组
	req.TokenGroup = common.GetContextKeyString(c, constant.ContextKeyUsingGroup)

	project, rawReq, rawResp, _, err := service.CreateProject(c.Request.Context(), userId, isAdmin, &req)
	if err != nil {
		// 创建失败退还预扣积分
		if refundErr := model.DecreaseUserQuota(userId, -preDeductQuota, false); refundErr != nil {
			common.SysLog(fmt.Sprintf("video pre-deduct refund failed: user=%d, amount=%d: %v", userId, preDeductQuota, refundErr))
		}
		return
	}

	// 从上游响应中解析扣费金额（仅用于日志记录和退款比例计算，不直接扣本系统积分）
	creditAmount := 0
	if len(rawResp) > 0 {
		var respData map[string]interface{}
		if err := common.Unmarshal(rawResp, &respData); err == nil {
			if data, ok := respData["data"].(map[string]interface{}); ok {
				if ca, ok := data["creditAmount"].(float64); ok {
					creditAmount = int(ca)
				}
			}
		}
	}

	// 保存预扣金额和上游积分到项目
	_ = model.UpdateVideoProjectFields(project.Id, map[string]interface{}{
		"pre_deducted_quota":     preDeductQuota,
		"upstream_credit_amount": creditAmount,
	})

	model.RecordConsumeLog(c, userId, model.RecordConsumeLogParams{
		ModelName: req.VideoModel,
		TokenName: c.GetString("token_name"),
		TokenId:   c.GetInt("token_id"),
		Content:   fmt.Sprintf("视频生成成功 [%s/%s/%s] id=%d", req.ProductName, req.Brand, req.Vtype, project.Id),
		Quota:     preDeductQuota,
		Other: map[string]interface{}{
			"product_name":           req.ProductName,
			"brand":                  req.Brand,
			"prompt":                 req.Prompt,
			"vtype":                  req.Vtype,
			"video_model":            req.VideoModel,
			"resolution":             req.Resolution,
			"duration":               req.Duration,
			"whstr":                  req.Whstr,
			"project_id":             project.Id,
			"status":                 project.Status,
			"pre_deducted_quota":     preDeductQuota,
			"upstream_credit_amount": creditAmount,
			"request_body":           string(rawReq),
			"response_body":          string(rawResp),
		},
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

	detail, rawResponse, err := service.GetProject(c.Request.Context(), projectId, userId, isAdmin)
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
		Other: map[string]interface{}{
			"request_path":           c.Request.URL.String(),
			"response_body":          string(rawResponse), // 上游原始响应
			"project_id":             detail.Id,
			"project_name":           detail.ProjectName,
			"status":                 detail.Status,
			"error_msg":              detail.ErrorMsg,
			"progress":               detail.Progress,
			"first_video_url":        detail.FirstVideoUrl,
			"main_image_url":         detail.MainImageUrl,
			"pre_deducted_quota":     detail.PreDeductedQuota,
			"real_quota":             detail.RealQuota,
			"settled":                detail.Settled,
			"upstream_credit_amount": detail.UpstreamCreditAmount,
			"upstream_credit_refund": detail.UpstreamCreditRefund,
			"upstream_credit_net":    detail.UpstreamCreditNet,
		},
	})

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "success",
		"data": dto.VideoProjectDetailResponse{
			ProjectId:            detail.Id,
			ProjectName:          detail.ProjectName,
			Status:               detail.Status,
			ErrorMsg:             detail.ErrorMsg,
			Progress:             detail.Progress,
			ProductImgUrl:        detail.ProductImgUrl,
			Brand:                detail.Brand,
			ProductName:          detail.ProductName,
			MainImageUrl:         detail.MainImageUrl,
			MainImageAssetId:     detail.MainImageAssetId,
			GeneratedResult:      detail.GeneratedResult,
			FirstVideoUrl:        detail.FirstVideoUrl,
			PreDeductedQuota:     detail.PreDeductedQuota,
			RealQuota:            detail.RealQuota,
			Settled:              detail.Settled,
			UpstreamCreditAmount: detail.UpstreamCreditAmount,
			UpstreamCreditRefund: detail.UpstreamCreditRefund,
			UpstreamCreditNet:    detail.UpstreamCreditNet,
			CreatedAt:            detail.CreatedAt.Unix(),
			UpdatedAt:            detail.UpdatedAt.Unix(),
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
			Other: map[string]interface{}{
				"project_id": strconv.FormatInt(projectId, 10),
			},
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
