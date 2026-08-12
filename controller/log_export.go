package controller

import (
	"fmt"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/xuri/excelize/v2"

	"github.com/gin-gonic/gin"
)

func ExportAllLogs(c *gin.Context) {
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

	// 查询所有符合条件的日志（不分页）
	logs, _, err := model.GetAllLogs(logType, startTimestamp, endTimestamp, modelName, username, tokenName, 0, 999999, channel, channelType, group, requestId, upstreamRequestId, taskId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 创建 Excel 文件
	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			common.SysLog(fmt.Sprintf("failed to close excel file: %v", err))
		}
	}()

	sheetName := "Logs"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	f.SetActiveSheet(index)

	// 设置表头
	headers := []string{"时间", "token_name", "模型", "内容", "原始quota", "分组", "渠道", "渠道类型", "request_id", "prompt_tokens", "completion_tokens", "耗时_ms", "是否流式", "IP", "用户名", "上游请求ID", "任务ID"}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheetName, cell, header)
	}

	// 写入数据
	for i, log := range logs {
		row := i + 2
		f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), time.Unix(log.CreatedAt, 0).Format("2006-01-02 15:04:05"))
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), log.TokenName)
		f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), log.ModelName)
		f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), log.Content)
		f.SetCellValue(sheetName, fmt.Sprintf("E%d", row), log.Quota)
		f.SetCellValue(sheetName, fmt.Sprintf("F%d", row), log.Group)
		f.SetCellValue(sheetName, fmt.Sprintf("G%d", row), log.ChannelId)
		channelTypeText := "通用渠道"
		if log.ChannelType == 2 {
			channelTypeText = "视频渠道"
		}
		f.SetCellValue(sheetName, fmt.Sprintf("H%d", row), channelTypeText)
		f.SetCellValue(sheetName, fmt.Sprintf("I%d", row), log.RequestId)
		f.SetCellValue(sheetName, fmt.Sprintf("J%d", row), log.PromptTokens)
		f.SetCellValue(sheetName, fmt.Sprintf("K%d", row), log.CompletionTokens)
		f.SetCellValue(sheetName, fmt.Sprintf("L%d", row), log.UseTime)
		streamText := "否"
		if log.IsStream {
			streamText = "是"
		}
		f.SetCellValue(sheetName, fmt.Sprintf("M%d", row), streamText)
		f.SetCellValue(sheetName, fmt.Sprintf("N%d", row), log.Ip)
		f.SetCellValue(sheetName, fmt.Sprintf("O%d", row), log.Username)
		f.SetCellValue(sheetName, fmt.Sprintf("P%d", row), log.UpstreamRequestId)
		f.SetCellValue(sheetName, fmt.Sprintf("Q%d", row), log.TaskId)
	}

	// 设置响应头
	filename := fmt.Sprintf("logs_%s.xlsx", time.Now().Format("20060102_150405"))
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Transfer-Encoding", "binary")

	// 写入响应
	if err := f.Write(c.Writer); err != nil {
		common.ApiError(c, err)
		return
	}
}
