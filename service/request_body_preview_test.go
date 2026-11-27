package service

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// newBodyStorageContext 构造一个已缓存请求体存储的 gin.Context，
// 模拟中间件阶段 common.GetBodyStorage 之后的上下文状态。
func newBodyStorageContext(t *testing.T, body []byte) (*gin.Context, common.BodyStorage) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/videos/generations", nil)

	storage, err := common.CreateBodyStorage(body)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = storage.Close()
	})
	ctx.Set(common.KeyBodyStorage, storage)
	ctx.Request.Body = io.NopCloser(storage)
	return ctx, storage
}

// TestRequestBodyPreviewShortBody 短请求体原样返回，且读取不消费请求体。
func TestRequestBodyPreviewShortBody(t *testing.T) {
	body := []byte(`{"model":"sora-2","prompt":"a cat"}`)
	ctx, storage := newBodyStorageContext(t, body)

	require.Equal(t, string(body), RequestBodyPreview(ctx))

	// 读取不应该消费请求体：读指针必须复位到开头
	data, err := io.ReadAll(storage)
	require.NoError(t, err)
	require.Equal(t, body, data)
}

// TestRequestBodyPreviewTruncated 超过 SavePromptBodyMaxBytes 时截断并追加省略标记。
func TestRequestBodyPreviewTruncated(t *testing.T) {
	old := common.SavePromptBodyMaxBytes
	common.SavePromptBodyMaxBytes = 10
	t.Cleanup(func() { common.SavePromptBodyMaxBytes = old })

	body := []byte(strings.Repeat("a", 100))
	ctx, _ := newBodyStorageContext(t, body)

	preview := RequestBodyPreview(ctx)
	require.Equal(t, strings.Repeat("a", 10)+"...(truncated)", preview)
}

// TestRequestBodyPreviewFollowsConfiguredLimit 截断长度严格跟随 SavePromptBodyMaxBytes 配置。
func TestRequestBodyPreviewFollowsConfiguredLimit(t *testing.T) {
	old := common.SavePromptBodyMaxBytes
	common.SavePromptBodyMaxBytes = 1000
	t.Cleanup(func() { common.SavePromptBodyMaxBytes = old })

	body := []byte(strings.Repeat("b", 2000))
	ctx, _ := newBodyStorageContext(t, body)

	preview := RequestBodyPreview(ctx)
	require.Equal(t, strings.Repeat("b", 1000)+"...(truncated)", preview)
}

// TestRequestBodyPreviewWithoutStorage 没有缓存请求体存储时返回空字符串（不报错、不去读 Body）。
func TestRequestBodyPreviewWithoutStorage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/videos/generations", strings.NewReader("{}"))

	require.Empty(t, RequestBodyPreview(ctx))
}
