package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestHasAPICredential(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name    string
		headers map[string]string
		want    bool
	}{
		{"authorization", map[string]string{"Authorization": "Bearer sk-test"}, true},
		{"x-api-key", map[string]string{"x-api-key": "sk-test"}, true},
		{"mj-api-secret", map[string]string{"mj-api-secret": "sk-test"}, true},
		{"x-goog-api-key", map[string]string{"x-goog-api-key": "sk-test"}, true},
		{"websocket", map[string]string{"Sec-WebSocket-Protocol": "realtime"}, true},
		{"no credential", map[string]string{}, false},
		{"blank authorization", map[string]string{"Authorization": "   "}, false},
	}
	for _, tc := range cases {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/api/seedance/assets", nil)
		for key, value := range tc.headers {
			c.Request.Header.Set(key, value)
		}
		if got := requestHasAPICredential(c); got != tc.want {
			t.Fatalf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}
