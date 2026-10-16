package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/oss"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
)

// ossTransferMaxAttempts 单个任务允许的最大转存尝试次数。
//
// 设置上限的原因：若上游 URL 已失效或对象存储持续不可用，无限重试只会
// 刷满失败记录表、持续占用带宽与 goroutine、并污染日志。
// 3 次足以覆盖网络抖动等瞬时故障。
const ossTransferMaxAttempts = 3

// ossTransferScanLimit 单次补扫最多处理的任务数。
// 限制的目的是避免大量积压任务同时上传拖垮带宽。
const ossTransferScanLimit = 20

// 同一进程内实时转存与补扫共用执行门禁；重复调用直接跳过，不排队重试。
var ossTransfersInFlight sync.Map

func acquireOssTransfer(taskID string) (func(), bool) {
	if _, loaded := ossTransfersInFlight.LoadOrStore(taskID, struct{}{}); loaded {
		return nil, false
	}
	return func() { ossTransfersInFlight.Delete(taskID) }, true
}

// ShouldTransferToOss 判断某任务结果是否需要转存到对象存储。
//
// 全部前置条件（须同时满足）：
//  1. 渠道类型为 Td 或 Zy；
//  2. 对象存储配置完整 —— 未配置时全部短路，行为与未引入该功能时一致；
//  3. 对应媒体类型的渠道开关已开启（图片/视频各一个开关）；
//  4. 尚未成功转存 —— 保证幂等，避免轮询重复上传；
//  5. 未达重试上限且上游 URL 未过期；
//  6. URL 有效（非空、非 data: 前缀、非本系统代理地址）。
//
// isVideo 由响应体结构判定（见 mediaTypeIsVideo），不能用适配器状态——
// 轮询会重建适配器实例，提交阶段的标记不会带过来。
func ShouldTransferToOss(ch *model.Channel, task *model.Task, srcURL string, isVideo bool) bool {
	if ch == nil || task == nil {
		return false
	}
	// 1. 仅显式接入的渠道类型
	if ch.Type != constant.ChannelTypeTudou && ch.Type != constant.ChannelTypeZy {
		return false
	}
	// 2. 对象存储配置完整
	if !oss.IsConfigured() {
		return false
	}
	// 3. 对应开关已开启
	settings := ch.GetOtherSettings()
	if !settings.OssTransferEnabled(ch.Type, isVideo) {
		return false
	}
	// 4/5. 幂等、重试上限、过期检查
	if !task.CanRetryOssTransfer(ossTransferMaxAttempts) {
		return false
	}
	// 6. URL 有效性
	if !isTransferableURL(srcURL) {
		return false
	}
	return true
}

// isTransferableURL 判断 URL 是否为可下载的真实文件地址。
//
// 排除的情况：
//   - 空值
//   - data: URI（内联数据，不是可下载资源）
//   - 本系统代理地址（形如 {ServerAddress}/v1/videos/{id}/content）——
//     这类地址指向本系统自身，转存会产生自引用请求。
func isTransferableURL(srcURL string) bool {
	u := strings.TrimSpace(srcURL)
	if u == "" {
		return false
	}
	if strings.HasPrefix(u, "data:") {
		return false
	}
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		return false
	}
	return true
}

// MediaTypeIsVideo 按查询响应体结构判定结果是否为视频。
//
// 复用 taskcommon 中的同一份判据（单一数据源），与 Td 适配器的结果解析
// 使用完全一致的逻辑，避免两处判定不一致。
func MediaTypeIsVideo(respBody []byte) bool {
	return taskcommon.IsVideoTaskResponse(respBody)
}

// taskResultIsVideo 实时转存与补扫按同一渠道判据识别媒体。
func taskResultIsVideo(channelType int, respBody []byte, resultURL string) bool {
	if channelType == constant.ChannelTypeZy {
		var result taskcommon.ZyTaskResult
		if len(respBody) > 0 {
			_ = common.Unmarshal(respBody, &result)
		}
		if result.URL == "" {
			result.URL = resultURL
		}
		return result.IsVideo()
	}
	return MediaTypeIsVideo(respBody) || looksLikeVideoURL(resultURL)
}

// TransferTaskResultToOss 将任务结果转存到对象存储，并更新任务记录。
//
// 该函数设计为在后台 goroutine 中调用，**不得**阻塞轮询主流程，
// 也**不得**因其失败而影响任务结算——失败时仅记录日志并写失败表。
//
// 返回值 error 仅供调用方记录日志，调用方应忽略其对主流程的影响。
func TransferTaskResultToOss(ctx context.Context, task *model.Task, ch *model.Channel, srcURL string, isVideo bool) error {
	if task == nil || ch == nil {
		return errors.New("task or channel is nil")
	}

	release, acquired := acquireOssTransfer(task.TaskID)
	if !acquired {
		return nil
	}
	defer release()

	mediaType := model.OssMediaTypeImage
	if isVideo {
		mediaType = model.OssMediaTypeVideo
	}

	// 重新读取，避免用陈旧副本覆盖并发写入
	if err := task.RefreshPrivateData(); err != nil {
		return fmt.Errorf("refresh oss transfer task %s failed: %w", task.TaskID, err)
	}

	// 二次幂等检查：RefreshPrivateData 后可能发现已被其他路径转存成功
	if task.PrivateData.OssURL != "" {
		return nil
	}
	if !task.CanRetryOssTransfer(ossTransferMaxAttempts) {
		return nil
	}

	attemptNo := task.PrivateData.OssTransferAttempts + 1
	objectKey := oss.BuildObjectKey(task.TaskID, srcURL, "", isVideo, time.Now())

	// 记录尝试次数（无论成败都递增，保证重试上限生效）
	task.PrivateData.OssTransferAttempts = attemptNo

	uploadCtx, cancel := oss.WithUploadTimeout(ctx)
	defer cancel()

	logger.LogInfo(ctx, fmt.Sprintf(
		"oss transfer: start task=%s attempt=%d/%d media=%s key=%s",
		task.TaskID, attemptNo, ossTransferMaxAttempts, mediaType, objectKey,
	))

	result, err := oss.UploadFromURL(uploadCtx, srcURL, objectKey)
	if err != nil {
		stage := classifyOssFailStage(err)
		isFinal := 0
		if attemptNo >= ossTransferMaxAttempts {
			isFinal = 1
		}

		// 持久化尝试次数，使重试上限在下次补扫时生效
		if updateErr := task.UpdateOssTransferFields(); updateErr != nil {
			logger.LogWarn(ctx, fmt.Sprintf(
				"oss transfer: update attempts failed task=%s: %v", task.TaskID, updateErr))
		}

		// 写失败记录。失败表写入失败不应影响主流程，故仅记录日志。
		record := &model.OssTransferFailure{
			TaskId:      task.TaskID,
			ChannelId:   ch.Id,
			ChannelType: ch.Type,
			SourceUrl:   srcURL,
			ObjectKey:   objectKey,
			FailReason:  err.Error(),
			FailStage:   stage,
			AttemptNo:   attemptNo,
			IsFinal:     isFinal,
			MediaType:   mediaType,
		}
		if recErr := model.RecordOssTransferFailure(record); recErr != nil {
			logger.LogWarn(ctx, fmt.Sprintf(
				"oss transfer: record failure failed task=%s: %v", task.TaskID, recErr))
		}

		logger.LogWarn(ctx, fmt.Sprintf(
			"oss transfer: failed task=%s attempt=%d/%d stage=%s final=%d err=%v",
			task.TaskID, attemptNo, ossTransferMaxAttempts, stage, isFinal, err,
		))
		return fmt.Errorf("oss transfer failed at %s: %w", stage, err)
	}

	// 成功：写入 OSS 地址（保留 ResultURL 不动，便于降级与对账）
	task.PrivateData.OssURL = result.PublicURL
	task.PrivateData.OssObjectKey = result.ObjectKey
	task.PrivateData.OssUploadedAt = time.Now().Unix()

	if err := task.UpdateOssTransferFields(); err != nil {
		// 上传已成功但落库失败：地址未能持久化，下次补扫会重试（幂等覆盖同一对象）
		logger.LogError(ctx, fmt.Sprintf(
			"oss transfer: upload succeeded but persist failed task=%s key=%s: %v",
			task.TaskID, result.ObjectKey, err,
		))
		return fmt.Errorf("persist oss result failed: %w", err)
	}

	// 将转存结果写入消费日志的 other 字段，使管理员可在「后台 → 通用日志 → 详情」
	// 中直接看到原 URL 与 OSS 地址。
	//
	// 为什么写日志而不只依赖任务表：任务表的 private_data 是内部字段，后台日志
	// 详情才是运维排查的入口。写入原 URL 的价值在于——转存后对外只返回 OSS 地址，
	// 若不记录原地址，事后无法回溯文件来源、也无法判断上游是否已过期。
	//
	// 该写入失败不影响转存结果（转存本身已成功），仅记警告。
	appendOssTransferLog(task, srcURL, result)

	logger.LogInfo(ctx, fmt.Sprintf(
		"oss transfer: success task=%s attempt=%d size=%d url=%s",
		task.TaskID, attemptNo, result.Size, result.PublicURL,
	))
	return nil
}

// appendOssTransferLog 把转存结果追加到任务关联的消费日志 other 字段。
//
// 追加的字段：
//   - oss_url：转存后的对象存储地址（对外返回的地址）
//   - oss_origin_url：上游原始 URL（转存后不再对外返回，仅后台留档）
//   - oss_object_key：对象键，便于在对象存储中定位文件
//   - oss_uploaded_at：转存完成时间
//
// 失败仅记录警告：日志写入失败不应影响已成功的转存。
func appendOssTransferLog(task *model.Task, srcURL string, result *oss.UploadResult) {
	if task == nil || result == nil {
		return
	}

	extra := map[string]interface{}{
		"oss_url":         result.PublicURL,
		"oss_origin_url":  srcURL,
		"oss_object_key":  result.ObjectKey,
		"oss_uploaded_at": task.PrivateData.OssUploadedAt,
	}
	if result.Size > 0 {
		extra["oss_file_size"] = result.Size
	}

	if err := model.AppendTaskLogOther(task.TaskID, extra, 0); err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf(
			"oss transfer: append log other failed task=%s: %v", task.TaskID, err))
	}
}

// classifyOssFailStage 将错误归类到失败阶段，便于按阶段统计与排查。
//
// 下载失败多为上游/网络问题，上传失败多为对象存储配置/磁盘问题，
// 两者排查方向完全不同，故必须区分。
func classifyOssFailStage(err error) string {
	if err == nil {
		return model.OssFailStageUpload
	}
	switch {
	case errors.Is(err, oss.ErrNotConfigured),
		errors.Is(err, oss.ErrFileTooLarge),
		errors.Is(err, oss.ErrUnsupportedContentType):
		return model.OssFailStagePrecheck
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "download source failed"),
		strings.Contains(msg, "request reject"),
		strings.Contains(msg, "build request failed"):
		return model.OssFailStageDownload
	default:
		return model.OssFailStageUpload
	}
}

// RunOssTransferCompensation 补扫「已成功但尚未转存」的任务并重试。
//
// 背景：转存是异步 goroutine，进程重启会导致未完成的转存丢失。
// 本函数由轮询任务定期调用，使这类任务能被重新转存。
//
// 实现说明（三库兼容）：候选集由 GetPendingOssTransferTasks 用普通列筛选，
// 再在 Go 侧解析 private_data 做精确判断，避免 SQL 中过滤 JSON 内部字段。
func RunOssTransferCompensation(ctx context.Context) {
	if !oss.IsConfigured() {
		return
	}

	// 收集开启了转存开关的 Td / Zy 渠道
	channels := getOssEnabledChannels()
	if len(channels) == 0 {
		return
	}

	channelIds := make([]int, 0, len(channels))
	channelByID := make(map[int]*model.Channel, len(channels))
	for _, ch := range channels {
		channelIds = append(channelIds, ch.Id)
		channelByID[ch.Id] = ch
	}

	tasks := model.GetPendingOssTransferTasks(channelIds, ossTransferScanLimit)
	if len(tasks) == 0 {
		return
	}

	transferred := 0
	for _, task := range tasks {
		ch, ok := channelByID[task.ChannelId]
		if !ok {
			continue
		}

		// Go 侧精确判断：幂等、重试上限、过期检查
		if task.PrivateData.OssURL != "" {
			continue
		}
		if !task.CanRetryOssTransfer(ossTransferMaxAttempts) {
			continue
		}

		srcURL := task.GetResultURL()
		if !isTransferableURL(srcURL) {
			continue
		}

		// 按响应体结构判定媒体类型（与结果解析使用同一判据）。
		// task.Data 保存的是脱敏后的查询响应体。
		isVideo := taskResultIsVideo(ch.Type, task.Data, srcURL)
		if !ShouldTransferToOss(ch, task, srcURL, isVideo) {
			continue
		}

		// 串行执行补扫，避免并发上传打满带宽
		if err := TransferTaskResultToOss(ctx, task, ch, srcURL, isVideo); err != nil {
			continue
		}
		transferred++
	}

	if transferred > 0 {
		logger.LogInfo(ctx, fmt.Sprintf("oss transfer compensation: retried %d tasks, succeeded %d", len(tasks), transferred))
	}
}

// getOssEnabledChannels 返回开启了对象存储转存开关的 Td / Zy 渠道。
// 查询失败时返回空列表（补扫直接跳过），不影响其他逻辑。
func getOssEnabledChannels() []*model.Channel {
	channels, err := model.GetChannelsWithOssTransferEnabled()
	if err != nil {
		common.SysLog(fmt.Sprintf("oss transfer: query channels failed: %v", err))
		return nil
	}
	return channels
}

// videoExts 常见视频扩展名，用于 URL 兜底判定。
var videoExts = []string{".mp4", ".mov", ".avi", ".webm", ".mkv", ".flv", ".wmv", ".m4v"}

// looksLikeVideoURL 按 URL 扩展名粗略判断是否为视频。
func looksLikeVideoURL(srcURL string) bool {
	lower := strings.ToLower(srcURL)
	// 去掉 query 部分，避免 ?a=.mp4 之类的误判
	if idx := strings.Index(lower, "?"); idx >= 0 {
		lower = lower[:idx]
	}
	for _, ext := range videoExts {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}
