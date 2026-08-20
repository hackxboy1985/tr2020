package controller

import (
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

	// 查询所有任务（不分页，排除失败任务）
	queryParams.Status = "" // 清空用户传来的status，使用自定义过滤
	allTasks := model.TaskGetAllTasks(0, 999999, queryParams)

	// 过滤掉失败的任务
	tasks := make([]*model.Task, 0, len(allTasks))
	for _, task := range allTasks {
		if task.Status != model.TaskStatusFailure {
			tasks = append(tasks, task)
		}
	}

	common.SysLog(fmt.Sprintf("ExportAllTasks: found %d tasks (excluded failures)", len(tasks)))

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
	var logSums []TaskLogSum
	if len(taskIDs) > 0 {
		model.DB.Table("logs").
			Select("task_id, type, SUM(quota) as quota").
			Where("task_id IN (?) AND type IN (?, ?)", taskIDs, model.LogTypeConsume, model.LogTypeRefund).
			Group("task_id, type").
			Find(&logSums)
	}

	// 构建每个任务的消耗/退款映射
	taskLogMap := make(map[string]map[int]int) // taskID -> {type -> quota}
	for _, logSum := range logSums {
		if taskLogMap[logSum.TaskID] == nil {
			taskLogMap[logSum.TaskID] = make(map[int]int)
		}
		taskLogMap[logSum.TaskID][logSum.Type] = logSum.Quota
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
		"任务ID", "上游任务ID", "用户名", "分组", "渠道ID", "平台",
		"动作", "状态", "进度", "提交时间", "开始时间", "完成时间",
		"预扣配额", "预扣金额(元)", "补扣/退款配额", "补扣/退款金额(元)", "最终配额", "最终金额(元)",
		"分组倍率", "失败原因",
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

		// 计算金额（配额/500000）
		consumeAmount := float64(consumeQuota) / 500000
		refundAmount := float64(refundQuota) / 500000
		finalQuota := consumeQuota + refundQuota // refundQuota 是负数
		finalAmount := float64(finalQuota) / 500000

		// 时间格式化
		submitTime := time.Unix(task.SubmitTime, 0).Format("2006-01-02 15:04:05")
		startTime := ""
		if task.StartTime > 0 {
			startTime = time.Unix(task.StartTime, 0).Format("2006-01-02 15:04:05")
		}
		finishTime := ""
		if task.FinishTime > 0 {
			finishTime = time.Unix(task.FinishTime, 0).Format("2006-01-02 15:04:05")
		}

		// 填充行数据（按新的表头顺序）
		f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), task.TaskID)
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), upstreamTaskID)
		f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), task.Username)
		f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), task.Group)
		f.SetCellValue(sheetName, fmt.Sprintf("E%d", row), task.ChannelId)
		f.SetCellValue(sheetName, fmt.Sprintf("F%d", row), task.Platform)
		f.SetCellValue(sheetName, fmt.Sprintf("G%d", row), task.Action)
		f.SetCellValue(sheetName, fmt.Sprintf("H%d", row), task.Status)
		f.SetCellValue(sheetName, fmt.Sprintf("I%d", row), task.Progress)
		f.SetCellValue(sheetName, fmt.Sprintf("J%d", row), submitTime)
		f.SetCellValue(sheetName, fmt.Sprintf("K%d", row), startTime)
		f.SetCellValue(sheetName, fmt.Sprintf("L%d", row), finishTime)
		f.SetCellValue(sheetName, fmt.Sprintf("M%d", row), consumeQuota)
		f.SetCellValue(sheetName, fmt.Sprintf("N%d", row), fmt.Sprintf("%.4f", consumeAmount))
		f.SetCellValue(sheetName, fmt.Sprintf("O%d", row), refundQuota)
		f.SetCellValue(sheetName, fmt.Sprintf("P%d", row), fmt.Sprintf("%.4f", refundAmount))
		f.SetCellValue(sheetName, fmt.Sprintf("Q%d", row), finalQuota)
		f.SetCellValue(sheetName, fmt.Sprintf("R%d", row), fmt.Sprintf("%.4f", finalAmount))
		f.SetCellValue(sheetName, fmt.Sprintf("S%d", row), groupRatio)
		f.SetCellValue(sheetName, fmt.Sprintf("T%d", row), task.FailReason)
	}

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
