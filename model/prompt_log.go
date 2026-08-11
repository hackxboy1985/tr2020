package model

import (
	"context"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"

	"github.com/bytedance/gopkg/util/gopool"
)

type PromptLog struct {
	Id           int    `json:"id" gorm:"primaryKey;index:idx_prompt_created_id,priority:2"`
	LogId        int    `json:"log_id" gorm:"uniqueIndex"`
	PromptText   string `json:"prompt_text" gorm:"type:text"`
	RequestBody  string `json:"request_body,omitempty" gorm:"type:text"`
	ResponseBody string `json:"response_body,omitempty" gorm:"type:text"`
	CreatedAt    int64  `json:"created_at" gorm:"bigint;index:idx_prompt_created_id,priority:1"`
}

// ── Buffered batch writer ──────────────────────────────────────────────

const (
	promptLogBatchMaxSize    = 100
	promptLogBatchInterval   = 5 * time.Second
	promptLogChannelCapacity = 2000
	promptLogMaxTextBytes    = 64000 // safe limit for MySQL TEXT (actual max is 65535)
)

var (
	promptLogChan      chan *PromptLog
	promptLogOnce      sync.Once
	promptLogFlushOnce sync.Once
)

// InitPromptLogWriter starts the background batch writer goroutine.
// Called once during server startup.
func InitPromptLogWriter() {
	promptLogOnce.Do(func() {
		promptLogChan = make(chan *PromptLog, promptLogChannelCapacity)
		gopool.Go(promptLogBatchLoop)
	})
}

func promptLogBatchLoop() {
	batch := make([]*PromptLog, 0, promptLogBatchMaxSize)
	ticker := time.NewTicker(promptLogBatchInterval)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := LOG_DB.Create(&batch).Error; err != nil {
			common.SysLog("failed to batch insert prompt logs: " + err.Error())
		}
		// Reset slice but keep underlying array capacity
		batch = batch[:0]
	}

	for {
		select {
		case entry, ok := <-promptLogChan:
			if !ok {
				// Channel closed, flush remaining and exit
				flush()
				return
			}
			batch = append(batch, entry)
			if len(batch) >= promptLogBatchMaxSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func truncateText(text string, maxBytes int) string {
	if len(text) <= maxBytes {
		return text
	}
	for maxBytes > 0 && !utf8.ValidString(text[:maxBytes]) {
		maxBytes--
	}
	return text[:maxBytes]
}

func truncatePromptText(text string) string {
	return truncateText(text, promptLogMaxTextBytes)
}

func truncateBodyText(text string) string {
	return truncateText(text, common.SavePromptBodyMaxBytes)
}

// EnqueuePromptLog queues a prompt log entry for batch insert.
// Returns immediately without blocking (drops entry if channel is full).
func EnqueuePromptLog(logId int, promptText string, requestBody string, responseBody string) {
	if !common.SavePromptEnabled {
		return
	}
	if promptLogChan == nil {
		return
	}
	select {
	case promptLogChan <- &PromptLog{
		LogId:        logId,
		PromptText:   truncatePromptText(promptText),
		RequestBody:  truncateBodyText(requestBody),
		ResponseBody: truncateBodyText(responseBody),
		CreatedAt:    common.GetTimestamp(),
	}:
	default:
		// Channel full, drop the entry silently to avoid blocking the request
		common.SysLog("prompt log channel full, dropping entry")
	}
}

// FlushPromptLogs flushes all remaining entries in the buffer channel.
// Should be called during graceful shutdown to avoid data loss.
// FlushPromptLogs flushes all remaining entries in the buffer channel.
// Should be called during graceful shutdown to avoid data loss.
// Safe to call multiple times.
func FlushPromptLogs() {
	promptLogFlushOnce.Do(func() {
		if promptLogChan != nil {
			close(promptLogChan)
		}
	})
}

// ── Query helpers (admin only) ─────────────────────────────────────────

// GetPromptLogByLogId returns the prompt log for a specific log entry.
func GetPromptLogByLogId(logId int) (*PromptLog, error) {
	var promptLog PromptLog
	err := LOG_DB.Where("log_id = ?", logId).First(&promptLog).Error
	if err != nil {
		return nil, err
	}
	return &promptLog, nil
}

// SearchPromptLogsByLogIds batch-fetches prompt logs by log IDs, returning a map keyed by log_id.
func SearchPromptLogsByLogIds(logIds []int) (map[int]*PromptLog, error) {
	if len(logIds) == 0 {
		return nil, nil
	}
	var logs []*PromptLog
	err := LOG_DB.Where("log_id IN ?", logIds).Find(&logs).Error
	if err != nil {
		return nil, err
	}
	result := make(map[int]*PromptLog, len(logs))
	for _, l := range logs {
		result[l.LogId] = l
	}
	return result, nil
}

// CheckPromptLogsExistence 批量检查提示词日志是否存在，返回 map[log_id]bool
// 优化性能：只查询 log_id，不加载大文本字段
func CheckPromptLogsExistence(logIds []int) (map[int]bool, error) {
	if len(logIds) == 0 {
		return nil, nil
	}
	var existingLogIds []int
	err := LOG_DB.Model(&PromptLog{}).
		Select("log_id").
		Where("log_id IN ?", logIds).
		Pluck("log_id", &existingLogIds).Error
	if err != nil {
		return nil, err
	}
	result := make(map[int]bool, len(existingLogIds))
	for _, logId := range existingLogIds {
		result[logId] = true
	}
	return result, nil
}

// DeleteOldPromptLog deletes prompt logs older than targetTimestamp in batches.
func DeleteOldPromptLog(ctx context.Context, targetTimestamp int64, limit int) (int64, error) {
	var total int64 = 0
	isPostgres := common.LogSqlType == common.DatabaseTypePostgreSQL || common.UsingPostgreSQL
	for {
		if ctx.Err() != nil {
			return total, ctx.Err()
		}
		var rowsAffected int64
		if isPostgres {
			// PostgreSQL 不支持 DELETE ... LIMIT，改用子查询
			var ids []int
			LOG_DB.Model(&PromptLog{}).
				Where("created_at < ?", targetTimestamp).
				Limit(limit).
				Select("id").
				Find(&ids)
			if len(ids) == 0 {
				break
			}
			result := LOG_DB.Where("id IN ?", ids).Delete(&PromptLog{})
			if result.Error != nil {
				return total, result.Error
			}
			rowsAffected = result.RowsAffected
		} else {
			result := LOG_DB.Where("created_at < ?", targetTimestamp).Limit(limit).Delete(&PromptLog{})
			if result.Error != nil {
				return total, result.Error
			}
			rowsAffected = result.RowsAffected
		}
		total += rowsAffected
		if rowsAffected < int64(limit) {
			break
		}
	}
	return total, nil
}
