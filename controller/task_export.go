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

	// 查询所有任务（不分页）
	tasks := model.TaskGetAllTasks(0, 999999, queryParams)

	common.SysLog(fmt.Sprintf("ExportAllTasks: found %d tasks", len(tasks)))

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
		"任务ID", "上游任务ID", "平台", "用户名", "分组", "渠道ID",
		"状态", "动作", "提交时间", "开始时间", "完成时间",
		"预扣消耗", "最终消耗", "分组倍率", "进度", "失败原因",
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

		// 填充行数据
		f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), task.TaskID)
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), upstreamTaskID)
		f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), task.Platform)
		f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), task.Username)
		f.SetCellValue(sheetName, fmt.Sprintf("E%d", row), task.Group)
		f.SetCellValue(sheetName, fmt.Sprintf("F%d", row), task.ChannelId)
		f.SetCellValue(sheetName, fmt.Sprintf("G%d", row), task.Status)
		f.SetCellValue(sheetName, fmt.Sprintf("H%d", row), task.Action)
		f.SetCellValue(sheetName, fmt.Sprintf("I%d", row), submitTime)
		f.SetCellValue(sheetName, fmt.Sprintf("J%d", row), startTime)
		f.SetCellValue(sheetName, fmt.Sprintf("K%d", row), finishTime)
		f.SetCellValue(sheetName, fmt.Sprintf("L%d", row), task.Quota)
		f.SetCellValue(sheetName, fmt.Sprintf("M%d", row), task.Quota) // 最终消耗，暂时使用Quota
		f.SetCellValue(sheetName, fmt.Sprintf("N%d", row), groupRatio)
		f.SetCellValue(sheetName, fmt.Sprintf("O%d", row), task.Progress)
		f.SetCellValue(sheetName, fmt.Sprintf("P%d", row), task.FailReason)
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
