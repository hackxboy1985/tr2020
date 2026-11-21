package relay

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
)

func TestRewriteDoubaoVideoContentURL(t *testing.T) {
	allowCh := &model.Channel{Type: 54, Other: `{"allow_non_official_video_url":true}`}
	denyCh := &model.Channel{Type: 54, Other: `{}`}
	sdos2Ch := &model.Channel{Type: 54, Other: `{"asset_upstream_version":"sdos2"}`}
	nonOfficial := "https://platform.n-axis.com/v1/videos/x.mp4"
	official := "https://ark-acg-cn-beijing.tos-cn-beijing.volces.com/a.mp4?X-Tos-Signature=x"

	// 绝对 http(s) 非官链 + 渠道允许 → 原样返回，不代理
	data := map[string]interface{}{"content": map[string]interface{}{"video_url": nonOfficial}}
	rewriteDoubaoVideoContentURL(data, "task_abc", allowCh)
	if data["content"].(map[string]interface{})["video_url"] != nonOfficial {
		t.Fatalf("absolute non-official url must stay untouched")
	}

	// 相对路径 + 渠道允许 → 替换为本服务代理入口
	data = map[string]interface{}{"content": map[string]interface{}{"video_url": "/v1/videos/x/content"}}
	rewriteDoubaoVideoContentURL(data, "task_abc", allowCh)
	got := data["content"].(map[string]interface{})["video_url"].(string)
	if !strings.Contains(got, "/v1/videos/task_abc/content") {
		t.Fatalf("relative url must be proxied, got %q", got)
	}

	// 官链 → 直连，不代理
	data = map[string]interface{}{"content": map[string]interface{}{"video_url": official}}
	rewriteDoubaoVideoContentURL(data, "task_abc", allowCh)
	if data["content"].(map[string]interface{})["video_url"] != official {
		t.Fatalf("official url must stay untouched")
	}

	// 非官链 + 渠道不允许 → 不代理
	data = map[string]interface{}{"content": map[string]interface{}{"video_url": nonOfficial}}
	rewriteDoubaoVideoContentURL(data, "task_abc", denyCh)
	if data["content"].(map[string]interface{})["video_url"] != nonOfficial {
		t.Fatalf("deny channel must not rewrite")
	}

	// sdos2 受保护接口 → 总是代理
	data = map[string]interface{}{"content": map[string]interface{}{"video_url": "http://106.54.45.168/v1/videos/x/content"}}
	rewriteDoubaoVideoContentURL(data, "task_abc", sdos2Ch)
	got = data["content"].(map[string]interface{})["video_url"].(string)
	if !strings.Contains(got, "/v1/videos/task_abc/content") {
		t.Fatalf("sdos2 must be proxied, got %q", got)
	}

	// 相对路径且空 base 时回退 BuildProxyURL
	data = map[string]interface{}{"content": map[string]interface{}{"video_url": "/v1/videos/x/content"}}
	rewriteDoubaoVideoContentURL(data, "task_abc", &model.Channel{Type: 54, Other: `{"allow_non_official_video_url":true}`})
	got = data["content"].(map[string]interface{})["video_url"].(string)
	if !strings.Contains(got, "/v1/videos/task_abc/content") {
		t.Fatalf("fallback url unexpected: %q", got)
	}
}
