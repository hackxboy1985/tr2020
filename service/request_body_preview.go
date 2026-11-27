package service

import (
	"io"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
)

// RequestBodyPreview 返回用于写入提示词日志（prompt_log.request_body）的请求体片段。
//
// 使用场景：请求失败（processChannelError）时，把整个用户请求体写入提示词日志，
// 由 /api/log/:id/prompt 接口查询；不再塞进 logs.other —— other 会随日志列表
// 整体加载，体积变大会放大查询与解析开销。
//
// 不解析请求体里的 prompt 字段：失败场景通常也拿不到 prompt，直接把整个请求体
// 落到 prompt_log.request_body 即可。
//
// 为什么不复用 constant.ContextKeyVideoRequestBody：那个 key 是「上游请求体」，
// 且只由 td / zy / doubao 等少数任务适配器在 BuildRequestBody 中主动写入；
// sora（OpenAI 渠道视频）等适配器从不写该 key。这里统一读取客户端请求体缓存，
// 与成功请求写入提示词日志的口径保持一致。
//
// 直接读取请求体存储（common.KeyBodyStorage，中间件阶段已缓存），只读不消费：
// 读完把读指针复位到 0，避免影响失败重试时对请求体的再次读取。
// 读取量上限与既有配置保持一致（SavePromptBodyMaxBytes，默认 500B），
// 不会把整个请求体载入内存。
func RequestBodyPreview(c *gin.Context) string {
	if c == nil {
		return ""
	}
	value, exists := c.Get(common.KeyBodyStorage)
	if !exists || value == nil {
		return ""
	}
	storage, ok := value.(common.BodyStorage)
	if !ok || storage.Size() <= 0 {
		return ""
	}

	// 与 service.TruncateBody 的取默认值口径保持一致
	limit := common.SavePromptBodyMaxBytes
	if limit <= 0 {
		limit = 2046
	}

	if _, err := storage.Seek(0, io.SeekStart); err != nil {
		return ""
	}
	// 多读 1 字节，用于判断是否发生截断
	buf := make([]byte, limit+1)
	n, readErr := io.ReadFull(storage, buf)
	if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
		return ""
	}
	// 复位读指针：错误日志可能发生在重试循环中，后续还会重新读取请求体
	if _, err := storage.Seek(0, io.SeekStart); err != nil {
		return ""
	}
	if n == 0 {
		return ""
	}
	if n > limit {
		// 与 service.TruncateBody 的截断标记保持一致
		return string(buf[:limit]) + "...(truncated)"
	}
	return string(buf[:n])
}
