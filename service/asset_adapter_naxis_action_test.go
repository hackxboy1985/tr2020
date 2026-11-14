package service

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newNaxisActionTestAdapter 起一个假上游，按 Action 分发响应
func newNaxisActionTestAdapter(t *testing.T, handler http.HandlerFunc) *NaxisActionAssetAdapter {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewNaxisActionAssetAdapter(srv.URL, "sk-test-token")
}

// naxisActionOf 取请求里的 Action 参数，并校验基础形态
func naxisActionOf(t *testing.T, r *http.Request) string {
	t.Helper()
	if r.Method != http.MethodPost {
		t.Fatalf("action entry must use POST, got %s", r.Method)
	}
	if got := r.URL.Query().Get("Version"); got != "2024-01-01" {
		t.Fatalf("unexpected Version: %s", got)
	}
	action := r.URL.Query().Get("Action")
	if action == "" {
		t.Fatalf("missing Action query: %s", r.URL.String())
	}
	return action
}

func TestNaxisActionCreateAssetGroup(t *testing.T) {
	adapter := newNaxisActionTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		switch naxisActionOf(t, r) {
		case "CreateAssetGroup":
			body := readNaxisTestJSON(t, r)
			if body["Name"] != "女主-林夏" || body["GroupType"] != "AIGC" || body["Description"] != "定妆参考" {
				t.Fatalf("unexpected create body: %+v", body)
			}
			_, _ = io.WriteString(w, `{"ResponseMetadata":{"RequestId":"r1","Action":"CreateAssetGroup"},"Result":{"Id":"group-abc"}}`)
		case "GetAssetGroup":
			body := readNaxisTestJSON(t, r)
			if body["Id"] != "group-abc" {
				t.Fatalf("unexpected get body: %+v", body)
			}
			_, _ = io.WriteString(w, `{"ResponseMetadata":{"RequestId":"r2","Action":"GetAssetGroup"},"Result":{"Id":"group-abc","Name":"女主-林夏","Description":"定妆参考","GroupType":"AIGC","ProjectName":"default","CreateTime":"2026-11-13T11:23:21.721Z","UpdateTime":"2026-11-13T11:23:21.721Z"}}`)
		default:
			t.Fatalf("unexpected action: %s", r.URL.Query().Get("Action"))
		}
	})

	group, err := adapter.CreateAssetGroup(CreateAssetGroupRequest{
		Name:        "女主-林夏",
		Description: "定妆参考",
		GroupType:   "AIGC",
	})
	if err != nil {
		t.Fatalf("create asset group failed: %v", err)
	}
	if group.ID != "group-abc" || group.GroupType != "AIGC" || group.ProjectName != "default" {
		t.Fatalf("unexpected group: %+v", group)
	}
	if group.CreateTime == "" || group.RawData == "" {
		t.Fatalf("expected times and raw data, got: %+v", group)
	}
}

func TestNaxisActionGetAssetDoesNotExposeSignedURL(t *testing.T) {
	adapter := newNaxisActionTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		if action := naxisActionOf(t, r); action != "GetAsset" {
			t.Fatalf("unexpected action: %s", action)
		}
		_, _ = io.WriteString(w, `{"ResponseMetadata":{"RequestId":"r3"},"Result":{"Id":"asset-1","Name":"主角","URL":"https://platform.n-axis.com/v1/assets/asset-1/source?expires=1795173810&sig=deadbeef","AssetType":"Image","GroupId":"group-abc","ProjectName":"default","Status":"Active","CreateTime":"2026-11-13T11:23:26.027Z","UpdateTime":"2026-11-13T11:23:27.041Z"}}`)
	})

	asset, err := adapter.GetAsset("asset-1", nil)
	if err != nil {
		t.Fatalf("get asset failed: %v", err)
	}
	// Action 入口返回的是平台签名代理地址：不落库、不返回下游
	if asset.URL != "" || asset.SourceURL != "" {
		t.Fatalf("signed url must not be exposed, got url=%q source=%q", asset.URL, asset.SourceURL)
	}
	if asset.Status != "Active" || asset.GroupID != "group-abc" || asset.AssetType != "Image" {
		t.Fatalf("unexpected asset: %+v", asset)
	}
	if asset.RawData == "" {
		t.Fatalf("expected raw response kept")
	}
}

func TestNaxisActionCreateAssetFillsFromGet(t *testing.T) {
	adapter := newNaxisActionTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		switch naxisActionOf(t, r) {
		case "CreateAsset":
			body := readNaxisTestJSON(t, r)
			if body["URL"] != "https://cdn.example.com/a.jpg" || body["AssetType"] != "Image" || body["GroupId"] != "group-abc" {
				t.Fatalf("unexpected create asset body: %+v", body)
			}
			_, _ = io.WriteString(w, `{"Result":{"Id":"asset-1"}}`)
		case "GetAsset":
			_, _ = io.WriteString(w, `{"Result":{"Id":"asset-1","Name":"主角定妆照","URL":"https://platform.n-axis.com/x?sig=1","AssetType":"Image","GroupId":"group-abc","Status":"Active"}}`)
		default:
			t.Fatalf("unexpected action")
		}
	})

	asset, err := adapter.CreateAsset(CreateAssetRequest{
		GroupID:   "group-abc",
		URL:       "https://cdn.example.com/a.jpg",
		AssetType: "image",
		Name:      "主角定妆照",
	})
	if err != nil {
		t.Fatalf("create asset failed: %v", err)
	}
	if asset.ID != "asset-1" || asset.Status != "Active" || asset.GroupID != "group-abc" {
		t.Fatalf("unexpected asset: %+v", asset)
	}
	if asset.URL != "" {
		t.Fatalf("signed url must not be exposed, got %q", asset.URL)
	}
}

func TestNaxisActionUpdateAndDeleteActions(t *testing.T) {
	seen := map[string]map[string]interface{}{}
	adapter := newNaxisActionTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		action := naxisActionOf(t, r)
		seen[action] = readNaxisTestJSON(t, r)
		_, _ = io.WriteString(w, `{"Result":{"Id":"x"}}`)
	})

	if err := adapter.UpdateAssetGroup("group-abc", UpdateAssetGroupRequest{Name: "新名字"}); err != nil {
		t.Fatalf("update group failed: %v", err)
	}
	if err := adapter.DeleteAssetGroup("group-abc"); err != nil {
		t.Fatalf("delete group failed: %v", err)
	}
	if err := adapter.UpdateAsset("asset-1", UpdateAssetRequest{Name: "新素材名"}); err != nil {
		t.Fatalf("update asset failed: %v", err)
	}
	if err := adapter.DeleteAsset("asset-1"); err != nil {
		t.Fatalf("delete asset failed: %v", err)
	}

	if seen["UpdateAssetGroup"]["Id"] != "group-abc" || seen["UpdateAssetGroup"]["Name"] != "新名字" {
		t.Fatalf("unexpected UpdateAssetGroup body: %+v", seen["UpdateAssetGroup"])
	}
	if seen["DeleteAssetGroup"]["Id"] != "group-abc" {
		t.Fatalf("unexpected DeleteAssetGroup body: %+v", seen["DeleteAssetGroup"])
	}
	if seen["UpdateAsset"]["Id"] != "asset-1" || seen["UpdateAsset"]["Name"] != "新素材名" {
		t.Fatalf("unexpected UpdateAsset body: %+v", seen["UpdateAsset"])
	}
	if seen["DeleteAsset"]["Id"] != "asset-1" {
		t.Fatalf("unexpected DeleteAsset body: %+v", seen["DeleteAsset"])
	}
}

func TestNaxisActionFaceVerification(t *testing.T) {
	adapter := newNaxisActionTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		switch naxisActionOf(t, r) {
		case "CreateVisualValidateSession":
			body := readNaxisTestJSON(t, r)
			if body["CallbackURL"] != "https://app.example.com/cb" {
				t.Fatalf("unexpected create session body: %+v", body)
			}
			_, _ = io.WriteString(w, `{"Result":{"BytedToken":"20261111110851EC2AACFC71344D45143C","CallbackURL":"https://app.example.com/cb","H5Link":"https://ark.volcengine.com/x"}}`)
		case "GetVisualValidateResult":
			body := readNaxisTestJSON(t, r)
			if body["BytedToken"] != "20261111110851EC2AACFC71344D45143C" {
				t.Fatalf("unexpected get result body: %+v", body)
			}
			_, _ = io.WriteString(w, `{"Result":{"GroupId":"group-abc","Status":"verified","Passed":true,"AssetId":"","Reason":"","ExpireTime":"2026-11-13T11:53:43.164Z","verification_id":"20261111110851EC2AACFC71344D45143C","status":"verified","group_id":"group-abc","expires_at":1794570823}}`)
		default:
			t.Fatalf("unexpected action")
		}
	})

	session, err := adapter.CreateFaceVerification("https://app.example.com/cb")
	if err != nil {
		t.Fatalf("create face verification failed: %v", err)
	}
	if session.Token != "20261111110851EC2AACFC71344D45143C" || session.H5URL == "" {
		t.Fatalf("unexpected session: %+v", session)
	}
	if session.ExpiresAt <= time.Now().Unix() {
		t.Fatalf("expected future expiry, got %d", session.ExpiresAt)
	}

	result, err := adapter.GetFaceVerification(session.Token)
	if err != nil {
		t.Fatalf("get face verification failed: %v", err)
	}
	if result.Status != "verified" || result.GroupID != "group-abc" || !result.Passed {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestNaxisActionErrorEnvelope(t *testing.T) {
	adapter := newNaxisActionTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		naxisActionOf(t, r)
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"ResponseMetadata":{"RequestId":"r9","Action":"GetAsset","Error":{"Code":"InvalidParameter.NotFound","Message":"Asset asset-1 not found"}}}`)
	})

	_, err := adapter.GetAsset("asset-1", nil)
	if err == nil || !strings.Contains(err.Error(), "InvalidParameter.NotFound") {
		t.Fatalf("expected not found error, got %v", err)
	}
}
