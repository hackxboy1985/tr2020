package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// newChannelRouteTestEngine 构造只含 API 路由的引擎。
//
// 关键：显式关闭 RedirectTrailingSlash，模拟「尾斜杠跳转不可用」的场景。
//
// 背景（真实回归）：gin 的尾斜杠重定向（301 到带斜杠地址）在同一路由树里
// 存在「共享前缀的兄弟路由」时会失效。/api/channel 与 /api/channel_sleep_pool
// 共享 "/api/channel" 前缀，一旦注册后者，GET /api/channel 就不再 301，
// 而是落到 NoRoute 返回 404，导致渠道列表页整体不可用。
//
// 因此 /api/channel 这类根路径必须「两种形式都显式注册」，不能依赖跳转。
func newChannelRouteTestEngine(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	e := gin.New()
	e.RedirectTrailingSlash = false
	e.Use(sessions.Sessions("test-session", cookie.NewStore([]byte("test-session-secret"))))
	SetApiRouter(e)
	return e
}

// TestChannelRootRoutesRegisteredWithoutTrailingSlashRedirect
// 渠道根路由必须同时支持「带斜杠」与「不带斜杠」，且都不返回 404、不依赖跳转。
func TestChannelRootRoutesRegisteredWithoutTrailingSlashRedirect(t *testing.T) {
	e := newChannelRouteTestEngine(t)

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/channel"},
		{http.MethodGet, "/api/channel/"},
		{http.MethodPost, "/api/channel"},
		{http.MethodPost, "/api/channel/"},
		{http.MethodPut, "/api/channel"},
		{http.MethodPut, "/api/channel/"},
	}

	for _, tc := range cases {
		w := httptest.NewRecorder()
		e.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		require.NotEqual(t, http.StatusNotFound, w.Code,
			"%s %s 不应返回 404（根路由必须两种形式都注册）", tc.method, tc.path)
		require.Empty(t, w.Header().Get("Location"),
			"%s %s 不应依赖尾斜杠跳转", tc.method, tc.path)
		// 未携带管理员身份时应被 AdminAuth 拦截为 401
		require.Equal(t, http.StatusUnauthorized, w.Code,
			"%s %s 应直接命中路由并被鉴权中间件拦截", tc.method, tc.path)
	}
}

// TestChannelSiblingRouteDoesNotBreakRootPath 守护性测试：
// 即使存在 /api/channel_sleep_pool 这类共享前缀的兄弟路由，
// 渠道根路径也必须照常可达（这正是线上 404 的触发条件）。
func TestChannelSiblingRouteDoesNotBreakRootPath(t *testing.T) {
	e := newChannelRouteTestEngine(t)

	// 兄弟路由确实已注册（它是触发 gin 跳转失效的条件）
	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/channel_sleep_pool", nil))
	require.NotEqual(t, http.StatusNotFound, w.Code,
		"/api/channel_sleep_pool 应已注册（不带斜杠形式）")

	// 兄弟路由存在时，渠道根路径仍然可达
	w = httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/channel", nil))
	require.Equal(t, http.StatusUnauthorized, w.Code,
		"兄弟路由存在时 /api/channel 仍必须可达，不能退化为 404")
}
