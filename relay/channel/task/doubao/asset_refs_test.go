package doubao

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
)

// 回归测试：资产错误必须带非空 Error 且 LocalError=true。
// 否则 controller/relay.go 的 types.NewOpenAIError(taskErr.Error, ...) 会 nil panic，
// 且会被当成渠道故障处理。
func TestAssetRefTaskErrorShape(t *testing.T) {
	inner := fmt.Errorf("%w: ast_abc (enable asset_auto_sync to re-register automatically)", service.ErrAssetNotSynced)
	taskErr := assetRefTaskError(&assetRefError{ref: "asset://ast_abc", err: inner})
	if taskErr == nil {
		t.Fatalf("expected task error")
	}
	if taskErr.Error == nil {
		t.Fatalf("Error must be non-nil, otherwise relay path panics")
	}
	if !taskErr.LocalError {
		t.Fatalf("asset error must be LocalError to avoid channel error processing")
	}
	if taskErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("unexpected status code: %d", taskErr.StatusCode)
	}
	body, ok := taskErr.ResponseBody.(map[string]interface{})
	if !ok {
		t.Fatalf("ResponseBody must be a map")
	}
	if body["code"] != "InvalidParameter" || body["type"] != "BadRequest" {
		t.Fatalf("unexpected body: %+v", body)
	}
	if body["param"] != nil {
		t.Fatalf("param must be nil")
	}
}

func TestRewriteTaskRequestAssetRefs(t *testing.T) {
	resolve := func(raw string) (string, error) {
		switch raw {
		case "asset://old-a":
			return "asset://new-a", nil
		case "asset://old-b":
			return "asset://new-b", nil
		case "asset://bad":
			return "", errors.New("asset not ready")
		default:
			return raw, nil
		}
	}

	req := &relaycommon.TaskSubmitReq{
		Image:   "asset://old-a",
		Images:  []string{"asset://old-b", "https://cdn.example.com/x.png"},
		Content: []map[string]interface{}{{"type": "image_url", "image_url": map[string]interface{}{"url": "asset://old-a"}}},
		Metadata: map[string]interface{}{
			"content": []interface{}{map[string]interface{}{"video_url": map[string]interface{}{"url": "asset://old-b"}}},
		},
	}

	changed, err := rewriteTaskRequestAssetRefs(req, resolve)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Fatalf("expected changed=true")
	}
	if req.Image != "asset://new-a" {
		t.Fatalf("image not rewritten: %q", req.Image)
	}
	if req.Images[0] != "asset://new-b" || req.Images[1] != "https://cdn.example.com/x.png" {
		t.Fatalf("images not rewritten: %+v", req.Images)
	}
	if req.Content[0]["image_url"].(map[string]interface{})["url"] != "asset://new-a" {
		t.Fatalf("content not rewritten")
	}
	nested := req.Metadata["content"].([]interface{})[0].(map[string]interface{})
	if nested["video_url"].(map[string]interface{})["url"] != "asset://new-b" {
		t.Fatalf("metadata content not rewritten")
	}

	// 非 asset:// 不动
	plain := &relaycommon.TaskSubmitReq{Image: "https://cdn.example.com/x.png"}
	if changed, err := rewriteTaskRequestAssetRefs(plain, resolve); err != nil || changed {
		t.Fatalf("plain request should not change: changed=%v err=%v", changed, err)
	}

	// 解析失败向上返回
	bad := &relaycommon.TaskSubmitReq{Image: "asset://bad"}
	if _, err := rewriteTaskRequestAssetRefs(bad, resolve); err == nil {
		t.Fatalf("expected resolve error")
	}
}
