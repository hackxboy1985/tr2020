package controller

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/xuri/excelize/v2"

	"github.com/gin-gonic/gin"
)

// ExportAllTasks 管理员导出任务
func ExportAllTasks(c *gin.Context) {
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)

	queryParams := model.SyncTaskQueryParams{
		Platform:       constant.TaskPlatform(c.Query("platform")),
		TaskID:         c.Query("task_id"),
		Status:         c.Query("status"),
		Action:         c.Query("action"),
		StartTimestamp: startTimestamp,
		EndTimestamp:   endTimestamp,
		ChannelID:      c.Query("channel_id"),
		UpstreamTaskID: c.Query("upstream_task_id"),
	}

	// 查询所有任务（不分页）
	tasks := model.TaskGetAllTasks(0, 999999, queryParams)

	common.SysLog(fmt.Sprintf("ExportAllTasks: found %d tasks from task table", len(tasks)))

	// 收集所有任务ID，批量查询logs
	taskIDMap := make(map[string]*model.Task)
	taskIDs := make([]string, 0, len(tasks))
	for _, task := range tasks {
		taskIDs = append(taskIDs, task.TaskID)
		taskIDMap[task.TaskID] = task
	}

	// 批量查询消费记录和退款记录
	type TaskLogSum struct {
		TaskID string
		Type   int
		Quota  int
	}

	var taskLogSums []TaskLogSum
	if len(taskIDs) > 0 {
		model.LOG_DB.Table("logs").
			Select("task_id, type, SUM(quota) as quota").
			Where("task_id IN (?) AND type IN (?, ?)", taskIDs, model.LogTypeConsume, model.LogTypeRefund).
			Group("task_id, type").
			Find(&taskLogSums)
	}

	// 构建任务日志映射: taskID -> {type -> quota}
	taskLogMap := make(map[string]map[int]int)
	for _, logSum := range taskLogSums {
		if taskLogMap[logSum.TaskID] == nil {
			taskLogMap[logSum.TaskID] = make(map[int]int)
		}
		taskLogMap[logSum.TaskID][logSum.Type] = logSum.Quota
	}

	// 对于预扣配额为0的任务，从 logs 表补充（早期记录，task_id 在 other 中）
	tasksWithZeroQuota := make([]*model.Task, 0)
	for _, task := range tasks {
		logs, ok := taskLogMap[task.TaskID]
		if !ok || logs[model.LogTypeConsume] == 0 {
			tasksWithZeroQuota = append(tasksWithZeroQuota, task)
		}
	}

	if len(tasksWithZeroQuota) > 0 {
		common.SysLog(fmt.Sprintf("ExportAllTasks: %d tasks with zero quota, searching in logs.other", len(tasksWithZeroQuota)))

		// 查询 logs 表中 task_id 在 other 字段的记录
		type LogRecord struct {
			TaskID string `json:"task_id"`
			Type   int    `json:"type"`
			Quota  int    `json:"quota"`
			Other  string `json:"other"`
		}

		var logRecords []LogRecord
		query := model.LOG_DB.Table("logs").
			Where("type IN (?, ?)", model.LogTypeConsume, model.LogTypeRefund).
			Where("(task_id = '' OR task_id IS NULL) AND other != ''")

		if startTimestamp > 0 {
			query = query.Where("created_at >= ?", startTimestamp)
		}
		if endTimestamp > 0 {
			query = query.Where("created_at <= ?", endTimestamp)
		}

		query.Find(&logRecords)

		// 从 other 提取 task_id 并聚合
		otherTaskLogMap := make(map[string]map[int]int)
		for _, log := range logRecords {
			var otherData map[string]interface{}
			if err := json.Unmarshal([]byte(log.Other), &otherData); err == nil {
				if tid, ok := otherData["task_id"].(string); ok && tid != "" {
					if otherTaskLogMap[tid] == nil {
						otherTaskLogMap[tid] = make(map[int]int)
					}
					otherTaskLogMap[tid][log.Type] += log.Quota

					// 如果是退款/补扣记录，且找不到消费记录，从 other 提取 pre_consumed_quota
					if log.Type == model.LogTypeRefund && otherTaskLogMap[tid][model.LogTypeConsume] == 0 {
						if preQuota, ok := otherData["pre_consumed_quota"].(float64); ok && preQuota > 0 {
							otherTaskLogMap[tid][model.LogTypeConsume] = int(preQuota)
						}
					}
				}
			}
		}

		// 补充到 taskLogMap（累加而不是替换）
		for taskID, logs := range otherTaskLogMap {
			if taskLogMap[taskID] == nil {
				taskLogMap[taskID] = make(map[int]int)
			}
			for typ, quota := range logs {
				taskLogMap[taskID][typ] += quota  // 累加
			}
		}

		common.SysLog(fmt.Sprintf("ExportAllTasks: supplemented %d tasks from logs.other", len(otherTaskLogMap)))
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
	for i, task := range tasks {
		row := i + 2

		// 获取上游任务ID
		upstreamTaskID := task.GetUpstreamTaskID()

		// 获取分组倍率
		groupRatio := ""
		if task.PrivateData.BillingContext != nil {
			groupRatio = fmt.Sprintf("%.2f", task.PrivateData.BillingContext.GroupRatio)
		}

		// 获取该任务的消耗和退款记录
		consumeQuota := 0
		refundQuota := 0
		if logs, ok := taskLogMap[task.TaskID]; ok {
			consumeQuota = logs[model.LogTypeConsume]
			refundQuota = logs[model.LogTypeRefund]
		}

		// 如果 logs 表找不到消费记录，使用 tasks 表的 quota 字段
		if consumeQuota == 0 && task.Quota > 0 {
			consumeQuota = task.Quota
		}

		// 计算金额（配额/500000）
		consumeAmount := float64(consumeQuota) / 500000
		refundAmount := float64(refundQuota) / 500000
		finalQuota := consumeQuota + refundQuota // refundQuota 是负数
		finalAmount := float64(finalQuota) / 500000

		// 时间格式化
		submitTime := time.Unix(task.SubmitTime, 0).Format("2006-01-02 15:04:05")

		// 填充行数据
		f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), task.TaskID)
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), upstreamTaskID)
		f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), task.Username)
		f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), task.Group)
		f.SetCellValue(sheetName, fmt.Sprintf("E%d", row), task.Status)
		f.SetCellValue(sheetName, fmt.Sprintf("F%d", row), submitTime)
		f.SetCellValue(sheetName, fmt.Sprintf("G%d", row), consumeQuota)
		f.SetCellValue(sheetName, fmt.Sprintf("H%d", row), fmt.Sprintf("%.4f", consumeAmount))
		f.SetCellValue(sheetName, fmt.Sprintf("I%d", row), refundQuota)
		f.SetCellValue(sheetName, fmt.Sprintf("J%d", row), fmt.Sprintf("%.4f", refundAmount))
		f.SetCellValue(sheetName, fmt.Sprintf("K%d", row), finalQuota)
		f.SetCellValue(sheetName, fmt.Sprintf("L%d", row), fmt.Sprintf("%.4f", finalAmount))
		f.SetCellValue(sheetName, fmt.Sprintf("M%d", row), groupRatio)
	}

	common.SysLog(fmt.Sprintf("ExportAllTasks: exported %d tasks", len(tasks)))

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
