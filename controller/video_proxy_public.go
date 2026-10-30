package controller

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// VideoProxyPublic 提供可配置鉴权的成片下载入口。
//
// 语义：
//   - 渠道未开启 sdos2_video_content_public：等价于 VideoProxy，需要 Token 鉴权且只能读自己的任务；
//   - 渠道开启该开关：允许匿名访问，按 task_id 直接定位任务并代理拉流。
//
// 该入口用于 sdos2（seedance-gateway）这类上游：上游成片地址是受 Bearer 保护的接口路径，
// 需要由服务端代理，才能把「上游接口地址」转换成「下游可访问的绝对地址」。
func VideoProxyPublic(c *gin.Context) {
	taskID := c.Param("task_id")
	if taskID == "" {
		videoProxyError(c, http.StatusBadRequest, "invalid_request_error", "task_id is required")
		return
	}

	// 已通过 TokenOrUserAuth 的请求（带有效 Key）走原逻辑，保证用户隔离
	if userID, ok := c.Get("id"); ok && userID != nil {
		VideoProxy(c)
		return
	}

	// 匿名访问：仅当任务所属渠道显式开启免鉴权时才放行
	task, exists, err := model.GetByTaskIdPublic(taskID)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to query task %s: %s", taskID, err.Error()))
		videoProxyError(c, http.StatusInternalServerError, "server_error", "Failed to query task")
		return
	}
	if !exists || task == nil {
		videoProxyError(c, http.StatusNotFound, "invalid_request_error", "Task not found")
		return
	}

	channel, err := model.CacheGetChannel(task.ChannelId)
	if err != nil || channel == nil {
		videoProxyError(c, http.StatusInternalServerError, "server_error", "Failed to retrieve channel information")
		return
	}
	settings := channel.GetOtherSettings()
	if !settings.Sdos2VideoContentPublic || !service.IsSdos2Upstream(channel) {
		// 未开启公开访问：按未认证处理
		videoProxyError(c, http.StatusUnauthorized, "invalid_request_error", "Authentication is required")
		return
	}

	// 借用统一代理实现：此时 c 中没有 id，VideoProxy 会按 user_id=0 查不到任务，
	// 因此这里直接把已解析的任务写入上下文，供代理分支使用。
	c.Set("id", task.UserId)
	VideoProxy(c)
}

// BuildVideoContentURL 返回下游可访问的成片绝对地址。
//
// 下游拿到的不再是上游的相对路径，而是本服务自己的下载入口，
// 请求不带上游 Key，上游 Key 仅保存在服务端。
func BuildVideoContentURL(basePath string, taskID string) string {
	base := strings.TrimRight(basePath, "/")
	return fmt.Sprintf("%s/v1/videos/%s/content", base, taskID)
}
