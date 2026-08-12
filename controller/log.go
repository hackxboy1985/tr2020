package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/xuri/excelize/v2"

	"github.com/gin-gonic/gin"
)

func GetPromptLog(c *gin.Context) {
	logId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	promptLog, err := model.GetPromptLogByLogId(logId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, promptLog)
}

func GetAllLogs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	username := c.Query("username")
	tokenName := c.Query("token_name")
	if tokenName == "" {
		tokenName = c.Query("token")
	}
	modelName := c.Query("model_name")
	if modelName == "" {
		modelName = c.Query("model")
	}
	channel, _ := strconv.Atoi(c.Query("channel"))
	channelType, _ := strconv.Atoi(c.Query("channelType"))
	group := c.Query("group")
	requestId := c.Query("request_id")
	upstreamRequestId := c.Query("upstream_request_id")
	taskId := c.Query("task_id")
	logs, total, err := model.GetAllLogs(logType, startTimestamp, endTimestamp, modelName, username, tokenName, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), channel, channelType, group, requestId, upstreamRequestId, taskId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	// Attach prompt_text for admin users
	if common.SavePromptEnabled {
		logIds := make([]int, 0, len(logs))
		for _, l := range logs {
			if l.Type == model.LogTypeConsume {
				logIds = append(logIds, l.Id)
			}
		}
		if len(logIds) > 0 {
			promptMap, err := model.SearchPromptLogsByLogIds(logIds)
			if err == nil {
				for _, l := range logs {
					if pl, ok := promptMap[l.Id]; ok {
						l.PromptText = pl.PromptText
						l.RequestBody = pl.RequestBody
						l.ResponseBody = pl.ResponseBody
					}
				}
			}
		}
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
	return
}

func GetUserLogs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	userId := c.GetInt("id")
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	tokenName := c.Query("token_name")
	if tokenName == "" {
		tokenName = c.Query("token")
	}
	modelName := c.Query("model_name")
	if modelName == "" {
		modelName = c.Query("model")
	}
	group := c.Query("group")
	requestId := c.Query("request_id")
	upstreamRequestId := c.Query("upstream_request_id")
	taskId := c.Query("task_id")
	logs, total, err := model.GetUserLogs(userId, logType, startTimestamp, endTimestamp, modelName, tokenName, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), group, requestId, upstreamRequestId, taskId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
	return
}

// Deprecated: SearchAllLogs 已废弃，前端未使用该接口。
func SearchAllLogs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": false,
		"message": "该接口已废弃",
	})
}

// Deprecated: SearchUserLogs 已废弃，前端未使用该接口。
func SearchUserLogs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": false,
		"message": "该接口已废弃",
	})
}

func GetLogByKey(c *gin.Context) {
	tokenId := c.GetInt("token_id")
	if tokenId == 0 {
		c.JSON(200, gin.H{
			"success": false,
			"message": "无效的令牌",
		})
		return
	}
	logs, err := model.GetLogByTokenId(tokenId)
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(200, gin.H{
		"success": true,
		"message": "",
		"data":    logs,
	})
}

func GetLogsStat(c *gin.Context) {
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	tokenName := c.Query("token_name")
	username := c.Query("username")
	modelName := c.Query("model_name")
	channel, _ := strconv.Atoi(c.Query("channel"))
	channelType, _ := strconv.Atoi(c.Query("channelType"))
	group := c.Query("group")
	requestId := c.Query("request_id")
	upstreamRequestId := c.Query("upstream_request_id")
	taskId := c.Query("task_id")
	stat, err := model.SumUsedQuota(logType, startTimestamp, endTimestamp, modelName, username, tokenName, channel, channelType, group, requestId, upstreamRequestId, taskId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	//tokenNum := model.SumUsedToken(logType, startTimestamp, endTimestamp, modelName, username, "")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"quota": stat.Quota,
			"rpm":   stat.Rpm,
			"tpm":   stat.Tpm,
		},
	})
	return
}

func GetLogsSelfStat(c *gin.Context) {
	username := c.GetString("username")
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	tokenName := c.Query("token_name")
	if tokenName == "" {
		tokenName = c.Query("token")
	}
	modelName := c.Query("model_name")
	if modelName == "" {
		modelName = c.Query("model")
	}
	channel, _ := strconv.Atoi(c.Query("channel"))
	channelType, _ := strconv.Atoi(c.Query("channelType"))
	group := c.Query("group")
	requestId := c.Query("request_id")
	upstreamRequestId := c.Query("upstream_request_id")
	taskId := c.Query("task_id")
	quotaNum, err := model.SumUsedQuota(logType, startTimestamp, endTimestamp, modelName, username, tokenName, channel, channelType, group, requestId, upstreamRequestId, taskId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	//tokenNum := model.SumUsedToken(logType, startTimestamp, endTimestamp, modelName, username, tokenName)
	c.JSON(200, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"quota": quotaNum.Quota,
			"rpm":   quotaNum.Rpm,
			"tpm":   quotaNum.Tpm,
			//"token": tokenNum,
		},
	})
	return
}

func DeleteHistoryLogs(c *gin.Context) {
	targetTimestamp, _ := strconv.ParseInt(c.Query("target_timestamp"), 10, 64)
	if targetTimestamp == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "target timestamp is required",
		})
		return
	}
	count, err := model.DeleteOldLog(c.Request.Context(), targetTimestamp, 100)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    count,
	})
	return
}

func ExportAllLogs(c *gin.Context) {
	// 解析查询参数
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	modelName := c.Query("model_name")
	if modelName == "" {
		modelName = c.Query("model")
	}
	tokenName := c.Query("token_name")
	if tokenName == "" {
		tokenName = c.Query("token")
	}
	username := c.Query("username")
	channel, _ := strconv.Atoi(c.Query("channel"))
	channelType, _ := strconv.Atoi(c.Query("channelType"))
	group := c.Query("group")
	requestId := c.Query("request_id")
	upstreamRequestId := c.Query("upstream_request_id")
	taskId := c.Query("task_id")

	// 查询日志数据（不分页，获取所有数据）
	logs, _, err := model.GetAllLogs(logType, startTimestamp, endTimestamp, modelName, username, tokenName, 0, 10000, channel, channelType, group, requestId, upstreamRequestId, taskId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "查询日志失败: " + err.Error(),
		})
		return
	}

	// 创建 Excel 文件
	f := excelize.NewFile()
	sheetName := "Logs"
	index, _ := f.NewSheet(sheetName)
	f.SetActiveSheet(index)
	f.DeleteSheet("Sheet1")

	// 设置表头
	headers := []string{
		"时间", "token_name", "模型", "内容", "原始quota", "平台计费_元", "分组倍率",
		"分组", "渠道", "渠道类型", "request_id", "prompt_tokens", "completion_tokens",
		"耗时_ms", "是否流式", "IP", "用户名", "上游请求ID", "任务ID",
	}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheetName, cell, header)
	}

	// 填充数据
	for i, log := range logs {
		row := i + 2

		// 时间
		cell, _ := excelize.CoordinatesToCellName(1, row)
		f.SetCellValue(sheetName, cell, time.Unix(log.CreatedAt, 0).Format("2006-01-02 15:04:05"))

		// token_name
		cell, _ = excelize.CoordinatesToCellName(2, row)
		f.SetCellValue(sheetName, cell, log.TokenName)

		// 模型
		cell, _ = excelize.CoordinatesToCellName(3, row)
		f.SetCellValue(sheetName, cell, log.ModelName)

		// 内容
		cell, _ = excelize.CoordinatesToCellName(4, row)
		f.SetCellValue(sheetName, cell, log.Content)

		// 原始quota
		cell, _ = excelize.CoordinatesToCellName(5, row)
		f.SetCellValue(sheetName, cell, log.Quota)

		// 平台计费_元
		cell, _ = excelize.CoordinatesToCellName(6, row)
		platformCost := float64(log.Quota) / 500000.0
		f.SetCellValue(sheetName, cell, fmt.Sprintf("%.6f", platformCost))

		// 分组倍率 - 从 Other 字段解析
		cell, _ = excelize.CoordinatesToCellName(7, row)
		groupRatio := common.ParseGroupRatioFromOther(log.Other)
		f.SetCellValue(sheetName, cell, groupRatio)

		// 分组
		cell, _ = excelize.CoordinatesToCellName(8, row)
		f.SetCellValue(sheetName, cell, log.Group)

		// 渠道
		cell, _ = excelize.CoordinatesToCellName(9, row)
		f.SetCellValue(sheetName, cell, log.ChannelId)

		// 渠道类型
		cell, _ = excelize.CoordinatesToCellName(10, row)
		channelTypeStr := "未知"
		if log.ChannelType == 1 {
			channelTypeStr = "普通"
		} else if log.ChannelType == 2 {
			channelTypeStr = "广告"
		}
		f.SetCellValue(sheetName, cell, channelTypeStr)

		// request_id
		cell, _ = excelize.CoordinatesToCellName(11, row)
		f.SetCellValue(sheetName, cell, log.RequestId)

		// prompt_tokens
		cell, _ = excelize.CoordinatesToCellName(12, row)
		f.SetCellValue(sheetName, cell, log.PromptTokens)

		// completion_tokens
		cell, _ = excelize.CoordinatesToCellName(13, row)
		f.SetCellValue(sheetName, cell, log.CompletionTokens)

		// 耗时_ms
		cell, _ = excelize.CoordinatesToCellName(14, row)
		f.SetCellValue(sheetName, cell, log.UseTime)

		// 是否流式
		cell, _ = excelize.CoordinatesToCellName(15, row)
		streamStr := "否"
		if log.IsStream {
			streamStr = "是"
		}
		f.SetCellValue(sheetName, cell, streamStr)

		// IP
		cell, _ = excelize.CoordinatesToCellName(16, row)
		f.SetCellValue(sheetName, cell, log.Ip)

		// 用户名
		cell, _ = excelize.CoordinatesToCellName(17, row)
		f.SetCellValue(sheetName, cell, log.Username)

		// 上游请求ID
		cell, _ = excelize.CoordinatesToCellName(18, row)
		f.SetCellValue(sheetName, cell, log.UpstreamRequestId)

		// 任务ID
		cell, _ = excelize.CoordinatesToCellName(19, row)
		f.SetCellValue(sheetName, cell, log.TaskId)
	}

	// 设置响应头
	fileName := fmt.Sprintf("logs_%s.xlsx", time.Now().Format("20060102_150405"))
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
	c.Header("Content-Transfer-Encoding", "binary")

	// 写入响应
	if err := f.Write(c.Writer); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "生成Excel失败: " + err.Error(),
		})
		return
	}
}
