package service

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
)

// newNaxisTestAdapter 起一个假上游并返回指向它的 n-axis 适配器
func newNaxisTestAdapter(t *testing.T, handler http.HandlerFunc) *NaxisAssetAdapter {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewNaxisAssetAdapter(srv.URL, "sk-test-token")
}

func readNaxisTestJSON(t *testing.T, r *http.Request) map[string]interface{} {
	t.Helper()
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read request body failed: %v", err)
	}
	var body map[string]interface{}
	if err := common.Unmarshal(raw, &body); err != nil {
		t.Fatalf("decode request body failed: %v, raw=%s", err, string(raw))
	}
	return body
}

func TestNaxisCreateAssetGroup(t *testing.T) {
	adapter := newNaxisTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/asset-groups" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test-token" {
			t.Fatalf("unexpected authorization: %s", got)
		}
		body := readNaxisTestJSON(t, r)
		if body["name"] != "女主-林夏" || body["group_type"] != "AIGC" || body["description"] != "定妆参考" {
			t.Fatalf("unexpected request body: %+v", body)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{
  "id": "group-3f9d2c1b8e7a6054d2c1",
  "object": "asset_group",
  "name": "女主-林夏",
  "description": "定妆参考",
  "group_type": "AIGC",
  "project_name": "default",
  "resource_owner": "sk:9a1c4e7b2d5f",
  "status": "active",
  "created_at": "2026-10-14T07:20:00.000Z",
  "updated_at": "2026-10-14T07:20:00.000Z"
}`)
	})

	resp, err := adapter.CreateAssetGroup(CreateAssetGroupRequest{
		Name:        "女主-林夏",
		Description: "定妆参考",
		GroupType:   "AIGC",
	})
	if err != nil {
		t.Fatalf("create asset group failed: %v", err)
	}
	if resp.ID != "group-3f9d2c1b8e7a6054d2c1" {
		t.Fatalf("unexpected group id: %s", resp.ID)
	}
	if resp.GroupType != "AIGC" || resp.ProjectName != "default" {
		t.Fatalf("unexpected mapped group: %+v", resp)
	}
	if resp.CreateTime == "" || resp.RawData == "" {
		t.Fatalf("expected times and raw data, got: %+v", resp)
	}
}

func TestNaxisUpdateAssetGroupUsesPatch(t *testing.T) {
	adapter := newNaxisTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/v1/asset-groups/group-abc" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		body := readNaxisTestJSON(t, r)
		if body["name"] != "新名字" {
			t.Fatalf("unexpected request body: %+v", body)
		}
		if _, ok := body["group_type"]; ok {
			t.Fatalf("group_type must not be sent on update")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"id":"group-abc","name":"新名字"}`)
	})

	if err := adapter.UpdateAssetGroup("group-abc", UpdateAssetGroupRequest{Name: "新名字"}); err != nil {
		t.Fatalf("update asset group failed: %v", err)
	}
}

func TestNaxisCreateAssetMapsStatusAndFields(t *testing.T) {
	adapter := newNaxisTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/assets" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		body := readNaxisTestJSON(t, r)
		// 下游可能传 PascalCase（Image），上游要求小写
		if body["asset_type"] != "image" {
			t.Fatalf("asset_type must be lower case, got %v", body["asset_type"])
		}
		if body["url"] != "https://cdn.example.com/a.jpg" || body["group_id"] != "group-abc" {
			t.Fatalf("unexpected request body: %+v", body)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{
  "id": "ast_3f9d2c1b8e7a6054d2c1",
  "object": "asset",
  "asset_type": "image",
  "group_id": "group-abc",
  "project_name": "default",
  "resource_owner": "sk:9a1c4e7b2d5f",
  "name": "主角定妆照",
  "source_url": "https://cdn.example.com/a.jpg",
  "status": "active",
  "error": null,
  "last_inference_time": null,
  "created_at": "2026-10-14T07:30:00.000Z",
  "updated_at": "2026-10-14T07:30:00.000Z"
}`)
	})

	resp, err := adapter.CreateAsset(CreateAssetRequest{
		GroupID:   "group-abc",
		URL:       "https://cdn.example.com/a.jpg",
		AssetType: "Image",
		Name:      "主角定妆照",
	})
	if err != nil {
		t.Fatalf("create asset failed: %v", err)
	}
	// 必须归一为下游使用的 Active，否则「同 URL 复用」判断失效
	if resp.Status != "Active" {
		t.Fatalf("unexpected status: %s", resp.Status)
	}
	if resp.ID != "ast_3f9d2c1b8e7a6054d2c1" || resp.GroupID != "group-abc" {
		t.Fatalf("unexpected mapped asset: %+v", resp)
	}
	if resp.URL != "https://cdn.example.com/a.jpg" || resp.SourceURL != resp.URL {
		t.Fatalf("unexpected url mapping: %+v", resp)
	}
}

func TestNaxisGetAssetForwardsQueryAndMapsArchived(t *testing.T) {
	adapter := newNaxisTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/assets/ast_abc" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.URL.Query().Get("X-Ark-Strict"); got != "1" {
			t.Fatalf("query not forwarded: %v", r.URL.Query())
		}
		_, _ = io.WriteString(w, `{"id":"ast_abc","status":"archived","source_url":"https://cdn.example.com/a.jpg"}`)
	})

	query := url.Values{}
	query.Set("X-Ark-Strict", "1")
	resp, err := adapter.GetAsset("ast_abc", query)
	if err != nil {
		t.Fatalf("get asset failed: %v", err)
	}
	if resp.Status != "Archived" {
		t.Fatalf("unexpected status: %s", resp.Status)
	}
}

func TestNaxisAssetStatusMapping(t *testing.T) {
	cases := map[string]string{
		"active":     "Active",
		"Active":     "Active",
		"processing": "Processing",
		"failed":     "Failed",
		"archived":   "Archived",
		"":           "",
	}
	for upstream, want := range cases {
		if got := mapNaxisAssetStatus(upstream); got != want {
			t.Fatalf("mapNaxisAssetStatus(%q) = %q, want %q", upstream, got, want)
		}
	}
}

func TestNaxisUpstreamErrorParsing(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"rest string", `{"error":"asset group not found"}`, "asset group not found"},
		{"rest struct", `{"error":{"code":"invalid_url","message":"url must be http(s)"}}`, "invalid_url: url must be http(s)"},
		{"top struct", `{"ResponseMetadata":{"Error":{"Code":"InvalidParameter.NotFound","Message":"not found"}}}`, "InvalidParameter.NotFound: not found"},
	}
	for _, tc := range cases {
		err := naxisUpstreamError(http.StatusBadRequest, []byte(tc.body))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: got %v, want contains %q", tc.name, err, tc.want)
		}
	}
}

func TestNaxisEmptyUpstreamIDFails(t *testing.T) {
	adapter := newNaxisTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{}`)
	})

	if _, err := adapter.CreateAssetGroup(CreateAssetGroupRequest{Name: "x"}); err == nil {
		t.Fatalf("expected error on empty upstream group id")
	}
}

func TestNaxisCreateFaceVerification(t *testing.T) {
	adapter := newNaxisTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/visual-validate/sessions" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		body := readNaxisTestJSON(t, r)
		if body["callback_url"] != "https://app.example.com/cb" {
			t.Fatalf("unexpected request body: %+v", body)
		}
		_, _ = io.WriteString(w, `{"byted_token":"20261111110851EC2AACFC71344D45143C","h5_link":"https://ark.volcengine.com/x","callback_url":"https://app.example.com/cb","expire_time":"2026-11-11T03:38:51.000Z","BytedToken":"20261111110851EC2AACFC71344D45143C","H5Link":"https://ark.volcengine.com/x"}`)
	})

	session, err := adapter.CreateFaceVerification("https://app.example.com/cb")
	if err != nil {
		t.Fatalf("create face verification failed: %v", err)
	}
	if session.Token != "20261111110851EC2AACFC71344D45143C" {
		t.Fatalf("unexpected token: %s", session.Token)
	}
	if session.H5URL != "https://ark.volcengine.com/x" {
		t.Fatalf("unexpected h5 url: %s", session.H5URL)
	}
	if session.ExpiresAt == 0 {
		t.Fatalf("expected expire time parsed, got 0")
	}
	if session.RawData == "" {
		t.Fatalf("expected raw data")
	}
}

func TestNaxisGetFaceVerification(t *testing.T) {
	adapter := newNaxisTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/visual-validate/sessions/abc123" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"status":"verified","group_id":"group-3f9d","passed":true,"reason":"","expire_time":"2026-11-11T03:38:51.000Z","GroupId":"group-3f9d"}`)
	})

	result, err := adapter.GetFaceVerification("abc123")
	if err != nil {
		t.Fatalf("get face verification failed: %v", err)
	}
	if result.Status != "verified" || result.GroupID != "group-3f9d" || !result.Passed {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.ExpireTime == "" {
		t.Fatalf("expected expire time echo")
	}
}

func TestNaxisCreateFaceVerificationEmptyTokenFails(t *testing.T) {
	adapter := newNaxisTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{}`)
	})

	if _, err := adapter.CreateFaceVerification(""); err == nil {
		t.Fatalf("expected error on empty byted token")
	}
}

func TestParseNaxisISOTime(t *testing.T) {
	if got := parseNaxisISOTime("2026-11-11T03:38:51.000Z"); got == 0 {
		t.Fatalf("expected parsed time")
	}
	if got := parseNaxisISOTime("not-a-time"); got != 0 {
		t.Fatalf("expected 0 on invalid time, got %d", got)
	}
	if got := parseNaxisISOTime("  "); got != 0 {
		t.Fatalf("expected 0 on empty time, got %d", got)
	}
}
