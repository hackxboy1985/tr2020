package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func ListVideoChannels(c *gin.Context) {
	channels, err := model.GetAllVideoChannels()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error(), "data": nil})
		return
	}

	resp := make([]dto.VideoChannelResponse, len(channels))
	for i, ch := range channels {
		resp[i] = channelToResponse(ch)
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "success", "data": resp})
}

func CreateVideoChannel(c *gin.Context) {
	var req dto.VideoChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error(), "data": nil})
		return
	}

	weight := req.Weight
	if weight <= 0 {
		weight = 1
	}
	enabled := req.Enabled
	if enabled == 0 {
		enabled = 1
	}

	ch := &model.VideoChannel{
		Name:                req.Name,
		ChannelType:         req.ChannelType,
		BaseURL:             req.BaseURL,
		ApiKey:              req.ApiKey,
		ApiSecret:           req.ApiSecret,
		WorkflowId:          req.WorkflowId,
		CreatePath:          req.CreatePath,
		StatusQueryPath:     req.StatusQueryPath,
		Groups:              req.Groups,
		Weight:              weight,
		Enabled:             enabled,
		Remark:              req.Remark,
		SaveRequestResponse: req.SaveRequestResponse,
		ModelMapping:        req.ModelMapping,
	}

	if err := model.CreateVideoChannel(ch); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error(), "data": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "channel created successfully", "data": channelToResponse(ch)})
}

func UpdateVideoChannel(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid channel id", "data": nil})
		return
	}

	ch, err := model.GetVideoChannelById(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "channel not found", "data": nil})
		return
	}

	var req dto.VideoChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error(), "data": nil})
		return
	}

	ch.Name = req.Name
	ch.ChannelType = req.ChannelType
	ch.BaseURL = req.BaseURL
	ch.WorkflowId = req.WorkflowId
	ch.CreatePath = req.CreatePath
	ch.StatusQueryPath = req.StatusQueryPath
	ch.Groups = req.Groups
	ch.Remark = req.Remark
	ch.SaveRequestResponse = req.SaveRequestResponse
	ch.ModelMapping = req.ModelMapping
	if req.Weight > 0 {
		ch.Weight = req.Weight
	}
	// 只在有值时更新密钥（避免前端编辑时清空密钥）
	if req.ApiKey != "" {
		ch.ApiKey = req.ApiKey
	}
	if req.ApiSecret != "" {
		ch.ApiSecret = req.ApiSecret
	}

	if err := model.UpdateVideoChannel(ch); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error(), "data": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "channel updated successfully", "data": channelToResponse(ch)})
}

func DeleteVideoChannel(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid channel id", "data": nil})
		return
	}

	if err := model.DeleteVideoChannel(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error(), "data": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "channel deleted successfully", "data": nil})
}

func UpdateVideoChannelStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "invalid channel id", "data": nil})
		return
	}

	var req dto.VideoChannelStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error(), "data": nil})
		return
	}

	if err := model.UpdateVideoChannelFields(id, map[string]interface{}{"enabled": req.Enabled}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error(), "data": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "channel status updated", "data": nil})
}

func channelToResponse(ch *model.VideoChannel) dto.VideoChannelResponse {
	return dto.VideoChannelResponse{
		Id:                  ch.Id,
		Name:                ch.Name,
		ChannelType:         ch.ChannelType,
		BaseURL:             ch.BaseURL,
		WorkflowId:          ch.WorkflowId,
		CreatePath:          ch.CreatePath,
		StatusQueryPath:     ch.StatusQueryPath,
		Groups:              ch.Groups,
		Weight:              ch.Weight,
		Enabled:             ch.Enabled,
		Remark:              ch.Remark,
		SaveRequestResponse: ch.SaveRequestResponse,
		ModelMapping:        ch.ModelMapping,
		CreatedAt:           ch.CreatedAt,
		UpdatedAt:           ch.UpdatedAt,
	}
}
