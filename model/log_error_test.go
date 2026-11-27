package model

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestRecordErrorLogReturnsCreatedLogID 验证错误日志写入后返回日志 ID。
// 该 ID 是失败请求把请求体写入提示词日志（prompt_log）的关联键。
func TestRecordErrorLogReturnsCreatedLogID(t *testing.T) {
	truncateTables(t)
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/videos/generations", nil)
	ctx.Set("username", "error-log-user")

	logId := RecordErrorLog(ctx, 1001, 66, "sora-2", "token-a", "status_code=400", 7, 5, false, "default",
		map[string]interface{}{"status_code": 400, "request_path": "/v1/videos/generations"})
	require.Greater(t, logId, 0)

	var row Log
	require.NoError(t, LOG_DB.First(&row, logId).Error)
	require.Equal(t, LogTypeError, row.Type)
	require.Equal(t, "status_code=400", row.Content)
	require.Equal(t, 66, row.ChannelId)
	require.NotContains(t, row.Other, "request_body")
}
