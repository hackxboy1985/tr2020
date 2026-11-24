package service

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// asset_action_path 支持：某些上游把火山方舟 Action 入口挂在子路径下
// （如 https://globalaiclient.com/api/material），而不是域名根路径。
// 这两个测试锁定 URL 拼装形态，防止未来重构把 "/?" 硬编码写回去。

// newActionPathTestAdapter 起一个假上游，记录请求路径，按 Action 返回最小可用响应。
func newActionPathTestAdapter(t *testing.T, baseURL, actionPath string, gotPath *string) *NaxisActionAssetAdapter {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*gotPath = r.URL.Path
		if r.Method != http.MethodPost {
			t.Errorf("action entry must use POST, got %s", r.Method)
		}
		if v := r.URL.Query().Get("Version"); v != "2024-01-01" {
			t.Errorf("unexpected Version: %s", v)
		}
		if r.URL.Query().Get("Action") == "" {
			t.Errorf("missing Action query: %s", r.URL.String())
		}
		switch r.URL.Query().Get("Action") {
		case "CreateAssetGroup":
			_, _ = io.WriteString(w, `{"Result":{"Id":"group-1"}}`)
		case "GetAssetGroup":
			_, _ = io.WriteString(w, `{"Result":{"Id":"group-1","Name":"测试组","GroupType":"AIGC","ProjectName":"default"}}`)
		default:
			_, _ = io.WriteString(w, `{"Result":{"Id":"asset-1"}}`)
		}
	}))
	t.Cleanup(srv.Close)
	return NewNaxisActionAssetAdapterWithPath(baseURL+actionPath, "sk-test", "")
}

// TestActionPathSubPath 子路径入口：URL 必须是 /api/material?Action=…（不能有尾斜杠）
func TestActionPathSubPath(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = io.WriteString(w, `{"Result":{"Id":"group-1"}}`)
	}))
	t.Cleanup(srv.Close)

	// baseURL 是域名，actionPath 是子路径
	a := NewNaxisActionAssetAdapterWithPath(srv.URL, "sk-test", "/api/material")
	if _, err := a.CreateAssetGroup(CreateAssetGroupRequest{Name: "x"}); err != nil {
		t.Fatalf("CreateAssetGroup failed: %v", err)
	}
	if gotPath != "/api/material" {
		t.Fatalf("expected path /api/material, got %q", gotPath)
	}
}

// TestActionPathNormalization 覆盖各种写法的归一化：尾斜杠/无前导斜杠/空白/根路径
func TestActionPathNormalization(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},   // 默认：根路径（n-axis）
		{"/", ""},  // 显式根路径
		{"  ", ""}, // 纯空白
		{"/api/material", "/api/material"},
		{"/api/material/", "/api/material"},  // 尾斜杠被去掉（否则拼出 //）
		{"api/material", "/api/material"},    // 补前导斜杠
		{" api/material/ ", "/api/material"}, // 首尾空白
		{"/api/material/v1/", "/api/material/v1"},
	}
	for _, c := range cases {
		if got := normalizeActionPath(c.in); got != c.want {
			t.Fatalf("normalizeActionPath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestActionPathDefaultKeepsNaxisRootEntry 默认（不配 asset_action_path）必须保持 n-axis 的根路径行为。
func TestActionPathDefaultKeepsNaxisRootEntry(t *testing.T) {
	var gotPath string
	var gotRawQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotRawQuery = r.URL.RawQuery
		_, _ = io.WriteString(w, `{"Result":{"Id":"group-1"}}`)
	}))
	t.Cleanup(srv.Close)

	a := NewNaxisActionAssetAdapter(srv.URL, "sk-test")
	if _, err := a.CreateAssetGroup(CreateAssetGroupRequest{Name: "x"}); err != nil {
		t.Fatalf("CreateAssetGroup failed: %v", err)
	}
	// n-axis 形态：POST http://host/?Action=…（path 为 "/"）
	if gotPath != "/" {
		t.Fatalf("naxis default entry must stay at root path, got %q", gotPath)
	}
	if gotRawQuery == "" {
		t.Fatalf("expected Action query, got empty")
	}
}
