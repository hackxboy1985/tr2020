package controller

import (
	"encoding/json"
	"fmt"
	"sort"
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

	// 导出逻辑：如果 logType=0（所有类型），只导出消费(2)和退款(6)
	var logs []*model.Log
	var err error

	if logType == 0 {
		// 查询消费记录
		logsConsume, _, errConsume := model.GetAllLogs(2, startTimestamp, endTimestamp, modelName, username, tokenName, 0, 999999, channel, channelType, group, requestId, upstreamRequestId, taskId)
		if errConsume != nil {
			common.ApiError(c, errConsume)
			return
		}

		// 查询退款记录
		logsRefund, _, errRefund := model.GetAllLogs(6, startTimestamp, endTimestamp, modelName, username, tokenName, 0, 999999, channel, channelType, group, requestId, upstreamRequestId, taskId)
		if errRefund != nil {
			common.ApiError(c, errRefund)
			return
		}

		// 合并并按时间排序
		logs = append(logsConsume, logsRefund...)
		// 按创建时间降序排序
		sort.Slice(logs, func(i, j int) bool {
			return logs[i].CreatedAt > logs[j].CreatedAt
		})
	} else {
		// 查询指定类型的日志
		logs, _, err = model.GetAllLogs(logType, startTimestamp, endTimestamp, modelName, username, tokenName, 0, 999999, channel, channelType, group, requestId, upstreamRequestId, taskId)
		if err != nil {
			common.ApiError(c, err)
			return
		}
	}

	// 调试：记录查询到的日志数量
	common.SysLog(fmt.Sprintf("ExportAllLogs: found %d logs", len(logs)))

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
	headers := []string{"时间", "类型", "token_name", "模型", "内容", "原始quota", "平台计费_元", "分组倍率", "分组", "渠道", "渠道类型", "request_id", "prompt_tokens", "completion_tokens", "耗时_ms", "是否流式", "IP", "用户名", "上游请求ID", "任务ID"}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheetName, cell, header)
	}

	// 写入数据
	for i, log := range logs {
		row := i + 2

		// 解析 Other 字段获取 group_ratio
		var otherData map[string]interface{}
		groupRatio := ""
		if log.Other != "" {
			if err := json.Unmarshal([]byte(log.Other), &otherData); err == nil {
				if gr, ok := otherData["group_ratio"]; ok {
					groupRatio = fmt.Sprintf("%v", gr)
				}
			}
		}

		// 计算平台计费（quota / 500000）
		platformFee := float64(log.Quota) / 500000.0

		// 转换类型为中文
		logTypeStr := "未知"
		switch log.Type {
		case 1:
			logTypeStr = "充值"
		case 2:
			logTypeStr = "消费"
		case 3:
			logTypeStr = "管理操作"
		case 4:
			logTypeStr = "系统日志"
		case 5:
			logTypeStr = "错误日志"
		case 6:
			logTypeStr = "退款"
		}

		f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), time.Unix(log.CreatedAt, 0).Format("2006-01-02 15:04:05"))
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), logTypeStr)
		f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), log.TokenName)
		f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), log.ModelName)
		f.SetCellValue(sheetName, fmt.Sprintf("E%d", row), log.Content)
		f.SetCellValue(sheetName, fmt.Sprintf("F%d", row), log.Quota)
		f.SetCellValue(sheetName, fmt.Sprintf("G%d", row), platformFee)
		f.SetCellValue(sheetName, fmt.Sprintf("H%d", row), groupRatio)
		f.SetCellValue(sheetName, fmt.Sprintf("I%d", row), log.Group)
		f.SetCellValue(sheetName, fmt.Sprintf("J%d", row), log.ChannelId)
		channelTypeText := "通用渠道"
		if log.ChannelType == 2 {
			channelTypeText = "视频渠道"
		}
		f.SetCellValue(sheetName, fmt.Sprintf("K%d", row), channelTypeText)
		f.SetCellValue(sheetName, fmt.Sprintf("L%d", row), log.RequestId)
		f.SetCellValue(sheetName, fmt.Sprintf("M%d", row), log.PromptTokens)
		f.SetCellValue(sheetName, fmt.Sprintf("N%d", row), log.CompletionTokens)
		f.SetCellValue(sheetName, fmt.Sprintf("O%d", row), log.UseTime)
		streamText := "否"
		if log.IsStream {
			streamText = "是"
		}
		f.SetCellValue(sheetName, fmt.Sprintf("P%d", row), streamText)
		f.SetCellValue(sheetName, fmt.Sprintf("Q%d", row), log.Ip)
		f.SetCellValue(sheetName, fmt.Sprintf("R%d", row), log.Username)
		f.SetCellValue(sheetName, fmt.Sprintf("S%d", row), log.UpstreamRequestId)
		f.SetCellValue(sheetName, fmt.Sprintf("T%d", row), log.TaskId)
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
