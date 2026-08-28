package model

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"

	"github.com/bytedance/gopkg/util/gopool"
	"gorm.io/gorm"
)

func applyExplicitLogTextFilter(tx *gorm.DB, column string, value string) (*gorm.DB, error) {
	if value == "" {
		return tx, nil
	}
	if strings.Contains(value, "%") {
		pattern, err := sanitizeLikePattern(value)
		if err != nil {
			return nil, err
		}
		return tx.Where(column+" LIKE ? ESCAPE '!'", pattern), nil
	}
	return tx.Where(column+" = ?", value), nil
}

type Log struct {
	Id                int    `json:"id" gorm:"index:idx_created_at_id,priority:2;index:idx_user_id_id,priority:2"`
	UserId            int    `json:"user_id" gorm:"index;index:idx_user_id_id,priority:1"`
	CreatedAt         int64  `json:"created_at" gorm:"bigint;index:idx_created_at_id,priority:1;index:idx_created_at_type"`
	Type              int    `json:"type" gorm:"index:idx_created_at_type"`
	Content           string `json:"content"`
	Username          string `json:"username" gorm:"index;index:index_username_model_name,priority:2;default:''"`
	TokenName         string `json:"token_name" gorm:"index;default:''"`
	ModelName         string `json:"model_name" gorm:"index;index:index_username_model_name,priority:1;default:''"`
	Quota             int    `json:"quota" gorm:"default:0"`
	PromptTokens      int    `json:"prompt_tokens" gorm:"default:0"`
	CompletionTokens  int    `json:"completion_tokens" gorm:"default:0"`
	UseTime           int    `json:"use_time" gorm:"default:0"`
	IsStream          bool   `json:"is_stream"`
	ChannelId         int    `json:"channel" gorm:"index"`
	ChannelType       int    `json:"channel_type" gorm:"default:1;index"` // 1=通用渠道, 2=视频渠道
	ChannelName       string `json:"channel_name" gorm:"->"`
	TokenId           int    `json:"token_id" gorm:"default:0;index"`
	Group             string `json:"group" gorm:"index"`
	Ip                string `json:"ip" gorm:"index;default:''"`
	RequestId         string `json:"request_id,omitempty" gorm:"type:varchar(64);index:idx_logs_request_id;default:''"`
	UpstreamRequestId string `json:"upstream_request_id,omitempty" gorm:"type:varchar(128);index:idx_logs_upstream_request_id;default:''"`
	TaskId            string `json:"task_id,omitempty" gorm:"type:varchar(64);index:idx_logs_task_id;default:''"`
	UpstreamTaskId    string `json:"upstream_task_id,omitempty" gorm:"type:varchar(128);index:idx_logs_upstream_task_id;default:''"`
	Other             string `json:"other"`
	PromptText        string `json:"prompt_text,omitempty" gorm:"-"`
	RequestBody       string `json:"request_body,omitempty" gorm:"-"`
	ResponseBody      string `json:"response_body,omitempty" gorm:"-"`
}

// don't use iota, avoid change log type value
const (
	LogTypeUnknown = 0
	LogTypeTopup   = 1
	LogTypeConsume = 2
	LogTypeManage  = 3
	LogTypeSystem  = 4
	LogTypeError   = 5
	LogTypeRefund  = 6
)

// ChannelType 定义：用于区分 logs 表中的 channel_id 指向哪张渠道表
const (
	ChannelTypeCommon = 1 // 普通AI渠道 (channels 表)
	ChannelTypeVideo  = 2 // 视频渠道 (video_channels 表)
)

func formatUserLogs(logs []*Log, startIdx int) {
	for i := range logs {
		logs[i].ChannelName = ""
		// 普通用户隐藏上游任务 ID
		logs[i].UpstreamTaskId = ""
		logs[i].UpstreamRequestId = ""
		// 普通用户隐藏管理员敏感字段（admin_info、request_body）和上游敏感字段（ori_task_id、upstream_task_id、response_body），保留计费信息
		if logs[i].Other != "" {
			var m map[string]interface{}
			if err := common.UnmarshalJsonStr(logs[i].Other, &m); err == nil {
				delete(m, "admin_info")
				delete(m, "request_body")
				delete(m, "ori_task_id")
				delete(m, "upstream_task_id")
				delete(m, "response_body")
				if b, err2 := common.Marshal(m); err2 == nil {
					logs[i].Other = string(b)
				}
			}
		}
		logs[i].Id = startIdx + i + 1
	}
}

func GetLogByTokenId(tokenId int) (logs []*Log, err error) {
	err = LOG_DB.Model(&Log{}).Where("token_id = ?", tokenId).Order("id desc").Limit(common.MaxRecentItems).Find(&logs).Error
	formatUserLogs(logs, 0)
	return logs, err
}

func RecordLog(userId int, logType int, content string) {
	if logType == LogTypeConsume && !common.LogConsumeEnabled {
		return
	}
	username, _ := GetUsernameById(userId, false)
	log := &Log{
		UserId:    userId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      logType,
		Content:   content,
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		common.SysLog("failed to record log: " + err.Error())
	}
}

// RecordLogWithQuota 记录带积分变动的日志（用于补扣/退款等场景），同时写入数据看板
func RecordLogWithQuota(userId int, logType int, quota int, modelName string, channelId int, channelType int, tokenId int, tokenName string, content string) {
	username, _ := GetUsernameById(userId, false)
	log := &Log{
		UserId:      userId,
		Username:    username,
		CreatedAt:   common.GetTimestamp(),
		Type:        logType,
		Quota:       quota,
		ModelName:   modelName,
		ChannelId:   channelId,
		ChannelType: channelType,
		TokenId:     tokenId,
		TokenName:   tokenName,
		Content:     content,
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		common.SysLog("failed to record log: " + err.Error())
	}
	if common.DataExportEnabled {
		LogQuotaData(userId, username, modelName, quota, common.GetTimestamp(), 0)
	}
}

// RecordLogWithAdminInfo 记录操作日志，并将管理员相关信息存入 Other.admin_info，
func RecordLogWithAdminInfo(userId int, logType int, content string, adminInfo map[string]interface{}) {
	if logType == LogTypeConsume && !common.LogConsumeEnabled {
		return
	}
	username, _ := GetUsernameById(userId, false)
	log := &Log{
		UserId:    userId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      logType,
		Content:   content,
	}
	if len(adminInfo) > 0 {
		other := map[string]interface{}{
			"admin_info": adminInfo,
		}
		log.Other = common.MapToJsonStr(other)
	}
	if err := LOG_DB.Create(log).Error; err != nil {
		common.SysLog("failed to record log: " + err.Error())
	}
}

func RecordTopupLog(userId int, content string, callerIp string, paymentMethod string, callbackPaymentMethod string) {
	username, _ := GetUsernameById(userId, false)
	adminInfo := map[string]interface{}{
		"server_ip":               common.GetIp(),
		"node_name":               common.NodeName,
		"caller_ip":               callerIp,
		"payment_method":          paymentMethod,
		"callback_payment_method": callbackPaymentMethod,
		"version":                 common.Version,
	}
	other := map[string]interface{}{
		"admin_info": adminInfo,
	}
	log := &Log{
		UserId:    userId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      LogTypeTopup,
		Content:   content,
		Ip:        callerIp,
		Other:     common.MapToJsonStr(other),
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		common.SysLog("failed to record topup log: " + err.Error())
	}
}

func RecordErrorLog(c *gin.Context, userId int, channelId int, modelName string, tokenName string, content string, tokenId int, useTimeSeconds int,
	isStream bool, group string, other map[string]interface{}) {
	logger.LogInfo(c, fmt.Sprintf("record error log: userId=%d, channelId=%d, modelName=%s, tokenName=%s, content=%s", userId, channelId, modelName, tokenName, common.LocalLogPreview(content)))
	username := c.GetString("username")
	requestId := c.GetString(common.RequestIdKey)
	upstreamRequestId := c.GetString(common.UpstreamRequestIdKey)
	otherStr := common.MapToJsonStr(other)
	// 判断是否需要记录 IP
	needRecordIp := false
	if settingMap, err := GetUserSetting(userId, false); err == nil {
		if settingMap.RecordIpLog {
			needRecordIp = true
		}
	}
	log := &Log{
		UserId:           userId,
		Username:         username,
		CreatedAt:        common.GetTimestamp(),
		Type:             LogTypeError,
		Content:          content,
		PromptTokens:     0,
		CompletionTokens: 0,
		TokenName:        tokenName,
		ModelName:        modelName,
		Quota:            0,
		ChannelId:        channelId,
		TokenId:          tokenId,
		UseTime:          useTimeSeconds,
		IsStream:         isStream,
		Group:            group,
		Ip: func() string {
			if needRecordIp {
				return c.ClientIP()
			}
			return ""
		}(),
		RequestId:         requestId,
		UpstreamRequestId: upstreamRequestId,
		Other:             otherStr,
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		logger.LogError(c, "failed to record log: "+err.Error())
	}
}

type RecordConsumeLogParams struct {
	ChannelId        int                    `json:"channel_id"`
	PromptTokens     int                    `json:"prompt_tokens"`
	CompletionTokens int                    `json:"completion_tokens"`
	ModelName        string                 `json:"model_name"`
	TokenName        string                 `json:"token_name"`
	Quota            int                    `json:"quota"`
	Content          string                 `json:"content"`
	TokenId          int                    `json:"token_id"`
	UseTimeSeconds   int                    `json:"use_time_seconds"`
	IsStream         bool                   `json:"is_stream"`
	Group            string                 `json:"group"`
	Other            map[string]interface{} `json:"other"`
	TaskId           string                 `json:"task_id"`
}

// GetLastVideoProjectQueryStatus 查询指定广告项目最近一条"查询"日志中记录的状态。
// 若未找到记录则返回 ("", nil)。
func GetLastVideoProjectQueryStatus(projectId int64) (string, error) {
	prefix := fmt.Sprintf("广告任务查询%%: id=%d", projectId)
	var log Log
	err := LOG_DB.Model(&Log{}).
		Where("content LIKE ? AND type = ?", prefix, LogTypeConsume).
		Order("id DESC").
		Limit(1).
		Select("other").
		First(&log).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}
	if log.Other == "" {
		return "", nil
	}
	var other map[string]interface{}
	if err2 := common.UnmarshalJsonStr(log.Other, &other); err2 != nil {
		return "", nil
	}
	if status, ok := other["status"].(string); ok {
		return status, nil
	}
	return "", nil
}

func RecordConsumeLog(c *gin.Context, userId int, params RecordConsumeLogParams) {
	if !common.LogConsumeEnabled {
		return
	}
	logger.LogInfo(c, fmt.Sprintf("record consume log: userId=%d, params=%s", userId, common.GetJsonString(params)))
	username := c.GetString("username")
	requestId := c.GetString(common.RequestIdKey)
	upstreamRequestId := c.GetString(common.UpstreamRequestIdKey)
	otherStr := common.MapToJsonStr(params.Other)
	// 判断是否需要记录 IP
	needRecordIp := false
	if settingMap, err := GetUserSetting(userId, false); err == nil {
		if settingMap.RecordIpLog {
			needRecordIp = true
		}
	}
	log := &Log{
		UserId:           userId,
		Username:         username,
		CreatedAt:        common.GetTimestamp(),
		Type:             LogTypeConsume,
		Content:          params.Content,
		PromptTokens:     params.PromptTokens,
		CompletionTokens: params.CompletionTokens,
		TokenName:        params.TokenName,
		ModelName:        params.ModelName,
		Quota:            params.Quota,
		ChannelId:        params.ChannelId,
		TokenId:          params.TokenId,
		UseTime:          params.UseTimeSeconds,
		IsStream:         params.IsStream,
		Group:            params.Group,
		Ip: func() string {
			if needRecordIp {
				return c.ClientIP()
			}
			return ""
		}(),
		RequestId:         requestId,
		UpstreamRequestId: upstreamRequestId,
		TaskId:            params.TaskId,
		UpstreamTaskId: func() string {
			// 从 Other 字段中提取 upstream_task_id 写入独立字段
			if params.Other != nil {
				if upstreamTaskId, ok := params.Other["upstream_task_id"].(string); ok {
					return upstreamTaskId
				}
			}
			return ""
		}(),
		Other: otherStr,
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		logger.LogError(c, "failed to record log: "+err.Error())
	}
	if common.DataExportEnabled {
		gopool.Go(func() {
			LogQuotaData(userId, username, params.ModelName, params.Quota, common.GetTimestamp(), params.PromptTokens+params.CompletionTokens)
		})
	}
	if err == nil && log.Id > 0 {
		savePrompt(c, log.Id, userId)
	}
}

// savePrompt checks settings and enqueues prompt text for storage.
func savePrompt(c *gin.Context, logId int, userId int) {
	// 1. Check global master switch
	if !common.SavePromptEnabled {
		return
	}

	promptText := c.GetString(string(constant.ContextKeyPromptToSave))
	if promptText == "" {
		return
	}

	requestBody := c.GetString(string(constant.ContextKeyVideoRequestBody))
	responseBody := c.GetString(string(constant.ContextKeyVideoResponseBody))

	// 2. Video channel save flag bypasses user visibility (admin-configured per channel)
	if requestBody != "" || responseBody != "" {
		EnqueuePromptLog(logId, promptText, requestBody, responseBody)
		return
	}

	// 3. If user visibility is disabled, force save for all users
	if !common.SavePromptUserVisible {
		EnqueuePromptLog(logId, promptText, "", "")
		return
	}

	// 4. User-visible mode: check token and user settings
	// Check token-level override (highest priority)
	if common.GetContextKeyBool(c, constant.ContextKeyTokenSavePrompt) {
		EnqueuePromptLog(logId, promptText, "", "")
		return
	}

	// Check user setting
	settingMap, err := GetUserSetting(userId, false)
	if err != nil || !settingMap.SavePrompt {
		return
	}

	EnqueuePromptLog(logId, promptText, "", "")
}

type RecordTaskBillingLogParams struct {
	UserId            int
	LogType           int
	Content           string
	ChannelId         int
	ModelName         string
	Quota             int
	TokenId           int
	Group             string
	Other             map[string]interface{}
	UpstreamRequestId string
	TaskId            string
}

func RecordTaskBillingLog(params RecordTaskBillingLogParams) {
	if params.LogType == LogTypeConsume && !common.LogConsumeEnabled {
		return
	}
	username, _ := GetUsernameById(params.UserId, false)
	tokenName := ""
	if params.TokenId > 0 {
		if token, err := GetTokenById(params.TokenId); err == nil {
			tokenName = token.Name
		}
	}
	log := &Log{
		UserId:            params.UserId,
		Username:          username,
		CreatedAt:         common.GetTimestamp(),
		Type:              params.LogType,
		Content:           params.Content,
		TokenName:         tokenName,
		ModelName:         params.ModelName,
		Quota:             params.Quota,
		ChannelId:         params.ChannelId,
		TokenId:           params.TokenId,
		Group:             params.Group,
		Other:             common.MapToJsonStr(params.Other),
		UpstreamRequestId: params.UpstreamRequestId,
		TaskId:            params.TaskId,
		UpstreamTaskId: func() string {
			// 从 Other 字段中提取 upstream_task_id 写入独立字段
			if params.Other != nil {
				if upstreamTaskId, ok := params.Other["upstream_task_id"].(string); ok {
					return upstreamTaskId
				}
			}
			return ""
		}(),
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		common.SysLog("failed to record task billing log: " + err.Error())
	}
	if common.DataExportEnabled {
		LogQuotaData(params.UserId, username, params.ModelName, params.Quota, common.GetTimestamp(), 0)
	}
}

func GetAllLogs(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, startIdx int, num int, channel int, channelType int, group string, requestId string, upstreamRequestId string, taskId string, upstreamTaskId string) (logs []*Log, total int64, err error) {
	var tx *gorm.DB
	if logType == LogTypeUnknown {
		tx = LOG_DB
	} else {
		tx = LOG_DB.Where("logs.type = ?", logType)
	}

	if tx, err = applyExplicitLogTextFilter(tx, "logs.model_name", modelName); err != nil {
		return nil, 0, err
	}
	if tx, err = applyExplicitLogTextFilter(tx, "logs.username", username); err != nil {
		return nil, 0, err
	}
	if tokenName != "" {
		tx = tx.Where("logs.token_name = ?", tokenName)
	}
	if requestId != "" {
		tx = tx.Where("logs.request_id = ?", requestId)
	}
	if upstreamRequestId != "" {
		tx = tx.Where("logs.upstream_request_id = ?", upstreamRequestId)
	}
	if taskId != "" {
		tx = tx.Where("logs.task_id = ?", taskId)
	}
	if upstreamTaskId != "" {
		// 混合查询：优先使用独立字段，回退到JSON查询（兼容存量数据）
		if common.UsingPostgreSQL {
			tx = tx.Where("(logs.upstream_task_id = ? OR (logs.upstream_task_id = '' AND logs.other IS NOT NULL AND logs.other::text != '' AND logs.other->>'upstream_task_id' = ?))", upstreamTaskId, upstreamTaskId)
		} else if common.UsingMySQL {
			tx = tx.Where("(logs.upstream_task_id = ? OR (logs.upstream_task_id = '' AND logs.other IS NOT NULL AND logs.other != '' AND JSON_UNQUOTE(JSON_EXTRACT(logs.other, '$.upstream_task_id')) = ?))", upstreamTaskId, upstreamTaskId)
		} else {
			// SQLite
			tx = tx.Where("(logs.upstream_task_id = ? OR (logs.upstream_task_id = '' AND logs.other IS NOT NULL AND logs.other != '' AND json_extract(logs.other, '$.upstream_task_id') = ?))", upstreamTaskId, upstreamTaskId)
		}
	}
	if startTimestamp != 0 {
		tx = tx.Where("logs.created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("logs.created_at <= ?", endTimestamp)
	}
	if channel != 0 {
		tx = tx.Where("logs.channel_id = ?", channel)
	}
	if channelType != 0 {
		tx = tx.Where("logs.channel_type = ?", channelType)
	}
	if group != "" {
		tx = tx.Where("logs."+logGroupCol+" = ?", group)
	}
	err = tx.Model(&Log{}).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}
	err = tx.Order("logs.created_at desc, logs.id desc").Limit(num).Offset(startIdx).Find(&logs).Error
	if err != nil {
		return nil, 0, err
	}

	channelIds := types.NewSet[int]()
	for _, log := range logs {
		if log.ChannelId != 0 {
			channelIds.Add(log.ChannelId)
		}
	}

	if channelIds.Len() > 0 {
		var channels []struct {
			Id   int    `gorm:"column:id"`
			Name string `gorm:"column:name"`
		}
		if common.MemoryCacheEnabled {
			// Cache get channel
			for _, channelId := range channelIds.Items() {
				if cacheChannel, err := CacheGetChannel(channelId); err == nil {
					channels = append(channels, struct {
						Id   int    `gorm:"column:id"`
						Name string `gorm:"column:name"`
					}{
						Id:   channelId,
						Name: cacheChannel.Name,
					})
				}
			}
		} else {
			// Bulk query channels from DB
			if err = DB.Table("channels").Select("id, name").Where("id IN ?", channelIds.Items()).Find(&channels).Error; err != nil {
				return logs, total, err
			}
		}
		channelMap := make(map[int]string, len(channels))
		for _, channel := range channels {
			channelMap[channel.Id] = channel.Name
		}
		for i := range logs {
			logs[i].ChannelName = channelMap[logs[i].ChannelId]
		}
	}

	return logs, total, err
}

const logSearchCountLimit = 10000

func GetUserLogs(userId int, logType int, startTimestamp int64, endTimestamp int64, modelName string, tokenName string, startIdx int, num int, group string, requestId string, upstreamRequestId string, taskId string, upstreamTaskId string) (logs []*Log, total int64, err error) {
	var tx *gorm.DB
	if logType == LogTypeUnknown {
		tx = LOG_DB.Where("logs.user_id = ?", userId)
	} else {
		tx = LOG_DB.Where("logs.user_id = ? and logs.type = ?", userId, logType)
	}

	if tx, err = applyExplicitLogTextFilter(tx, "logs.model_name", modelName); err != nil {
		return nil, 0, err
	}
	if tokenName != "" {
		tx = tx.Where("logs.token_name = ?", tokenName)
	}
	if requestId != "" {
		tx = tx.Where("logs.request_id = ?", requestId)
	}
	if upstreamRequestId != "" {
		tx = tx.Where("logs.upstream_request_id = ?", upstreamRequestId)
	}
	if taskId != "" {
		tx = tx.Where("logs.task_id = ?", taskId)
	}
	if upstreamTaskId != "" {
		// 混合查询：优先使用独立字段，回退到JSON查询（兼容存量数据）
		if common.UsingPostgreSQL {
			tx = tx.Where("(logs.upstream_task_id = ? OR (logs.upstream_task_id = '' AND logs.other IS NOT NULL AND logs.other::text != '' AND logs.other->>'upstream_task_id' = ?))", upstreamTaskId, upstreamTaskId)
		} else if common.UsingMySQL {
			tx = tx.Where("(logs.upstream_task_id = ? OR (logs.upstream_task_id = '' AND logs.other IS NOT NULL AND logs.other != '' AND JSON_UNQUOTE(JSON_EXTRACT(logs.other, '$.upstream_task_id')) = ?))", upstreamTaskId, upstreamTaskId)
		} else {
			// SQLite
			tx = tx.Where("(logs.upstream_task_id = ? OR (logs.upstream_task_id = '' AND logs.other IS NOT NULL AND logs.other != '' AND json_extract(logs.other, '$.upstream_task_id') = ?))", upstreamTaskId, upstreamTaskId)
		}
	}
	if startTimestamp != 0 {
		tx = tx.Where("logs.created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("logs.created_at <= ?", endTimestamp)
	}
	if group != "" {
		tx = tx.Where("logs."+logGroupCol+" = ?", group)
	}
	err = tx.Model(&Log{}).Limit(logSearchCountLimit).Count(&total).Error
	if err != nil {
		common.SysError("failed to count user logs: " + err.Error())
		return nil, 0, errors.New("查询日志失败")
	}
	err = tx.Order("logs.id desc").Limit(num).Offset(startIdx).Find(&logs).Error
	if err != nil {
		common.SysError("failed to search user logs: " + err.Error())
		return nil, 0, errors.New("查询日志失败")
	}

	formatUserLogs(logs, startIdx)
	return logs, total, err
}

type Stat struct {
	Quota int `json:"quota"`
	Rpm   int `json:"rpm"`
	Tpm   int `json:"tpm"`
}

func SumUsedQuota(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, channel int, channelType int, group string, requestId string, upstreamRequestId string, taskId string, upstreamTaskId string) (stat Stat, err error) {
	tx := LOG_DB.Table("logs").Select("sum(quota) quota")

	// 为rpm和tpm创建单独的查询
	rpmTpmQuery := LOG_DB.Table("logs").Select("count(*) rpm, sum(prompt_tokens) + sum(completion_tokens) tpm")

	if tx, err = applyExplicitLogTextFilter(tx, "username", username); err != nil {
		return stat, err
	}
	if rpmTpmQuery, err = applyExplicitLogTextFilter(rpmTpmQuery, "username", username); err != nil {
		return stat, err
	}
	if tokenName != "" {
		tx = tx.Where("token_name = ?", tokenName)
		rpmTpmQuery = rpmTpmQuery.Where("token_name = ?", tokenName)
	}
	if requestId != "" {
		tx = tx.Where("request_id = ?", requestId)
		rpmTpmQuery = rpmTpmQuery.Where("request_id = ?", requestId)
	}
	if upstreamRequestId != "" {
		tx = tx.Where("upstream_request_id = ?", upstreamRequestId)
		rpmTpmQuery = rpmTpmQuery.Where("upstream_request_id = ?", upstreamRequestId)
	}
	if taskId != "" {
		tx = tx.Where("task_id = ?", taskId)
		rpmTpmQuery = rpmTpmQuery.Where("task_id = ?", taskId)
	}
	if upstreamTaskId != "" {
		// 混合查询：优先使用独立字段，回退到JSON查询（兼容存量数据）
		if common.UsingPostgreSQL {
			tx = tx.Where("(upstream_task_id = ? OR (upstream_task_id = '' AND other IS NOT NULL AND other::text != '' AND other->>'upstream_task_id' = ?))", upstreamTaskId, upstreamTaskId)
			rpmTpmQuery = rpmTpmQuery.Where("(upstream_task_id = ? OR (upstream_task_id = '' AND other IS NOT NULL AND other::text != '' AND other->>'upstream_task_id' = ?))", upstreamTaskId, upstreamTaskId)
		} else if common.UsingMySQL {
			tx = tx.Where("(upstream_task_id = ? OR (upstream_task_id = '' AND other IS NOT NULL AND other != '' AND JSON_UNQUOTE(JSON_EXTRACT(other, '$.upstream_task_id')) = ?))", upstreamTaskId, upstreamTaskId)
			rpmTpmQuery = rpmTpmQuery.Where("(upstream_task_id = ? OR (upstream_task_id = '' AND other IS NOT NULL AND other != '' AND JSON_UNQUOTE(JSON_EXTRACT(other, '$.upstream_task_id')) = ?))", upstreamTaskId, upstreamTaskId)
		} else {
			// SQLite
			tx = tx.Where("(upstream_task_id = ? OR (upstream_task_id = '' AND other IS NOT NULL AND other != '' AND json_extract(other, '$.upstream_task_id') = ?))", upstreamTaskId, upstreamTaskId)
			rpmTpmQuery = rpmTpmQuery.Where("(upstream_task_id = ? OR (upstream_task_id = '' AND other IS NOT NULL AND other != '' AND json_extract(other, '$.upstream_task_id') = ?))", upstreamTaskId, upstreamTaskId)
		}
	}
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	if tx, err = applyExplicitLogTextFilter(tx, "model_name", modelName); err != nil {
		return stat, err
	}
	if rpmTpmQuery, err = applyExplicitLogTextFilter(rpmTpmQuery, "model_name", modelName); err != nil {
		return stat, err
	}
	if channel != 0 {
		tx = tx.Where("channel_id = ?", channel)
		rpmTpmQuery = rpmTpmQuery.Where("channel_id = ?", channel)
	}
	if channelType != 0 {
		tx = tx.Where("channel_type = ?", channelType)
		rpmTpmQuery = rpmTpmQuery.Where("channel_type = ?", channelType)
	}
	if group != "" {
		tx = tx.Where(logGroupCol+" = ?", group)
		rpmTpmQuery = rpmTpmQuery.Where(logGroupCol+" = ?", group)
	}

	// 根据 logType 过滤
	if logType != 0 {
		tx = tx.Where("type = ?", logType)
		// rpm/tpm 只统计消费类型
		if logType == LogTypeConsume {
			rpmTpmQuery = rpmTpmQuery.Where("type = ?", logType)
		} else {
			// 非消费类型，rpm/tpm 无意义，设为0
			rpmTpmQuery = rpmTpmQuery.Where("1 = 0") // 返回空结果
		}
	} else {
		// logType=0 表示统计所有消费和退款
		tx = tx.Where("type IN ?", []int{LogTypeConsume, LogTypeRefund})
		rpmTpmQuery = rpmTpmQuery.Where("type = ?", LogTypeConsume)
	}

	// 只统计最近60秒的rpm和tpm
	rpmTpmQuery = rpmTpmQuery.Where("created_at >= ?", time.Now().Add(-60*time.Second).Unix())

	// 执行查询
	if err := tx.Scan(&stat).Error; err != nil {
		common.SysError("failed to query log stat: " + err.Error())
		return stat, errors.New("查询统计数据失败")
	}
	if err := rpmTpmQuery.Scan(&stat).Error; err != nil {
		common.SysError("failed to query rpm/tpm stat: " + err.Error())
		return stat, errors.New("查询统计数据失败")
	}

	return stat, nil
}

func SumUsedToken(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string) (token int) {
	tx := LOG_DB.Table("logs").Select("ifnull(sum(prompt_tokens),0) + ifnull(sum(completion_tokens),0)")
	if username != "" {
		tx = tx.Where("username = ?", username)
	}
	if tokenName != "" {
		tx = tx.Where("token_name = ?", tokenName)
	}
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	if modelName != "" {
		tx = tx.Where("model_name = ?", modelName)
	}
	tx.Where("type = ?", LogTypeConsume).Scan(&token)
	return token
}

// TokenQuotaStat 按令牌汇总的用量统计
type TokenQuotaStat struct {
	TokenName string `json:"token_name"`
	Quota     int    `json:"quota"`
	Count     int    `json:"count"`
	TokenUsed int    `json:"token_used"`
	CreatedAt int64  `json:"created_at"`
}

// GetQuotaStatByToken 查询在时间范围内各令牌的用量，按小时聚合
// userId=0 时查全量（管理员视角）
func GetQuotaStatByToken(userId int, startTimestamp int64, endTimestamp int64) ([]TokenQuotaStat, error) {
	var results []TokenQuotaStat
	query := LOG_DB.Table("logs").
		Select("token_name, sum(quota) as quota, count(*) as count, sum(prompt_tokens)+sum(completion_tokens) as token_used, (created_at - (created_at % 3600)) as created_at").
		Where("type IN ? AND created_at >= ? AND created_at <= ?",
			[]int{LogTypeConsume, LogTypeRefund}, startTimestamp, endTimestamp).
		Where("token_name != ''")
	if userId != 0 {
		query = query.Where("user_id = ?", userId)
	}
	err := query.
		Group("token_name, (created_at - (created_at % 3600))").
		Order("created_at asc").
		Scan(&results).Error
	if err != nil {
		return nil, err
	}
	return results, nil
}

func DeleteOldLog(ctx context.Context, targetTimestamp int64, limit int) (int64, error) {
	var total int64 = 0

	for {
		if nil != ctx.Err() {
			return total, ctx.Err()
		}

		result := LOG_DB.Where("created_at < ?", targetTimestamp).Limit(limit).Delete(&Log{})
		if nil != result.Error {
			return total, result.Error
		}

		total += result.RowsAffected

		if result.RowsAffected < int64(limit) {
			break
		}
	}

	// Cascade: also clean up prompt_logs with the same time range
	if total > 0 {
		promptCtx, promptCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer promptCancel()
		if _, err := DeleteOldPromptLog(promptCtx, targetTimestamp, limit); err != nil {
			common.SysLog("failed to delete old prompt logs: " + err.Error())
		}
	}

	return total, nil
}
