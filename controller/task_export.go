package controller

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"

	"github.com/gin-gonic/gin"
)

// ExportAllTasks 管理员导出任务（基于 Logs 表）
func ExportAllTasks(c *gin.Context) {
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	channelID := c.Query("channel_id")
	upstreamTaskID := c.Query("upstream_task_id")
	taskIDFilter := c.Query("task_id")

	// 分别查询：1. task_id 有值的记录，2. task_id 为空但 other 中有 task_id 的记录
	type LogRecord struct {
		TaskID         string `json:"task_id"`
		UpstreamTaskID string `json:"upstream_task_id"`
		Username       string `json:"username"`
		Group          string `json:"group"`
		Channel        int    `json:"channel"`
		Type           int    `json:"type"`
		Quota          int    `json:"quota"`
		Other          string `json:"other"`
		CreatedAt      int64  `json:"created_at"`
	}

	// 构建基础查询条件
	baseQuery := func() *gorm.DB {
		query := model.LOG_DB.Table("logs").
			Where("type IN (?, ?)", model.LogTypeConsume, model.LogTypeRefund).
			Where("channel_type = ?", model.ChannelTypeVideo)

		if startTimestamp > 0 {
			query = query.Where("created_at >= ?", startTimestamp)
		}
		if endTimestamp > 0 {
			query = query.Where("created_at <= ?", endTimestamp)
		}
		if channelID != "" {
			channelIDInt, _ := strconv.Atoi(channelID)
			query = query.Where("channel_id = ?", channelIDInt)
		}
		return query
	}

	var allRecords []LogRecord

	// 查询1：task_id 字段有值的记录
	query1 := baseQuery().Where("task_id != ''")
	if taskIDFilter != "" {
		query1 = query1.Where("task_id = ?", taskIDFilter)
	}
	if upstreamTaskID != "" {
		query1 = query1.Where("upstream_task_id = ?", upstreamTaskID)
	}
	var records1 []LogRecord
	query1.Find(&records1)
	allRecords = append(allRecords, records1...)

	// 查询2：task_id 字段为空，从 other 字段提取
	query2 := baseQuery().Where("task_id = '' OR task_id IS NULL").Where("other != ''")
	var records2 []LogRecord
	query2.Find(&records2)

	// 从 other 提取 task_id 并过滤
	for _, r := range records2 {
		var otherData map[string]interface{}
		if err := json.Unmarshal([]byte(r.Other), &otherData); err == nil {
			if tid, ok := otherData["task_id"].(string); ok && tid != "" {
				r.TaskID = tid
				if utid, ok := otherData["upstream_task_id"].(string); ok {
					r.UpstreamTaskID = utid
				}

				// 应用筛选
				if taskIDFilter != "" && r.TaskID != taskIDFilter {
					continue
				}
				if upstreamTaskID != "" && r.UpstreamTaskID != upstreamTaskID {
					continue
				}

				allRecords = append(allRecords, r)
			}
		}
	}

	common.SysLog(fmt.Sprintf("ExportAllTasks: found %d log records", len(allRecords)))

	// 按任务聚合消费和退款
	type TaskSummary struct {
		TaskID         string
		UpstreamTaskID string
		Username       string
		Group          string
		Status         string
		SubmitTime     string
		ConsumeQuota   int
		RefundQuota    int
		GroupRatio     string
	}

	taskMap := make(map[string]*TaskSummary)

	for _, log := range allRecords {
		if log.TaskID == "" {
			continue
		}

		// 初始化任务汇总
		if taskMap[log.TaskID] == nil {
			taskMap[log.TaskID] = &TaskSummary{
				TaskID:         log.TaskID,
				UpstreamTaskID: log.UpstreamTaskID,
				Username:       log.Username,
				Group:          log.Group,
				SubmitTime:     time.Unix(log.CreatedAt, 0).Format("2006-01-02 15:04:05"),
				Status:         "SUCCESS",
			}

			// 尝试从 other 提取 group_ratio
			if log.Other != "" {
				var otherData map[string]interface{}
				if err := json.Unmarshal([]byte(log.Other), &otherData); err == nil {
					if gr, ok := otherData["group_ratio"].(float64); ok {
						taskMap[log.TaskID].GroupRatio = fmt.Sprintf("%.2f", gr)
					}
				}
			}
		}

		// 累加配额
		if log.Type == model.LogTypeConsume {
			taskMap[log.TaskID].ConsumeQuota += log.Quota
		} else if log.Type == model.LogTypeRefund {
			taskMap[log.TaskID].RefundQuota += log.Quota
		}
	}

	// 创建 Excel 文件
	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			common.SysLog(fmt.Sprintf("failed to close excel file: %v", err))
		}
	}()

	sheetName := "Tasks"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	f.SetActiveSheet(index)

	// 设置表头
	headers := []string{
		"任务ID", "上游任务ID", "用户名", "分组", "状态", "提交时间",
		"预扣配额", "预扣金额(元)", "补扣/退款配额", "补扣/退款金额(元)", "最终配额", "最终金额(元)",
		"分组倍率",
	}

	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheetName, cell, header)
	}

	// 填充数据
	row := 2
	for _, task := range taskMap {
		// 计算金额
		consumeAmount := float64(task.ConsumeQuota) / 500000
		refundAmount := float64(task.RefundQuota) / 500000
		finalQuota := task.ConsumeQuota + task.RefundQuota
		finalAmount := float64(finalQuota) / 500000

		f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), task.TaskID)
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), task.UpstreamTaskID)
		f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), task.Username)
		f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), task.Group)
		f.SetCellValue(sheetName, fmt.Sprintf("E%d", row), task.Status)
		f.SetCellValue(sheetName, fmt.Sprintf("F%d", row), task.SubmitTime)
		f.SetCellValue(sheetName, fmt.Sprintf("G%d", row), task.ConsumeQuota)
		f.SetCellValue(sheetName, fmt.Sprintf("H%d", row), fmt.Sprintf("%.4f", consumeAmount))
		f.SetCellValue(sheetName, fmt.Sprintf("I%d", row), task.RefundQuota)
		f.SetCellValue(sheetName, fmt.Sprintf("J%d", row), fmt.Sprintf("%.4f", refundAmount))
		f.SetCellValue(sheetName, fmt.Sprintf("K%d", row), finalQuota)
		f.SetCellValue(sheetName, fmt.Sprintf("L%d", row), fmt.Sprintf("%.4f", finalAmount))
		f.SetCellValue(sheetName, fmt.Sprintf("M%d", row), task.GroupRatio)
		row++
	}

	common.SysLog(fmt.Sprintf("ExportAllTasks: exported %d tasks from logs", len(taskMap)))

	// 设置响应头
	filename := fmt.Sprintf("tasks_%s.xlsx", time.Now().Format("20060102_150405"))
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Transfer-Encoding", "binary")

	// 输出文件
	if err := f.Write(c.Writer); err != nil {
		common.ApiError(c, err)
		return
	}
}
