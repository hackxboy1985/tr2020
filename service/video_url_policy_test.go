package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
)

// 用户提供的真实官链样例
const officialSample = `https://ark-acg-cn-beijing.tos-cn-beijing.volces.com/doubao-seedance-2-5/2131488439/Video/uas20-15556161116202-tgc_593454000251caffff00000000000000000000745853918497120/cgt-20261116165551-02sau.mp4?X-Tos-Algorithm=TOS4-HMAC-SHA256&X-Tos-Credential=AKLTYWJkZTExNjA1ZDUyNDc3YzhjNTM5OGIyNjBhNDcyOTQ%2F20261116%2Fcn-beijing%2Ftos%2Frequest&X-Tos-Date=20261116T085938Z&X-Tos-Expires=86400&X-Tos-Max-Requests=100&X-Tos-Signature=ff98b6830b283a70a7713162e0fd63e992bdeb80ec8c659dfd41a4d94e084d1d&X-Tos-SignedHeaders=host`

func TestIsOfficialVideoURL(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want bool
	}{
		{"用户提供的官链样例", officialSample, true},
		{"tos 子域", "https://ark-acg-cn-beijing.tos-cn-beijing.volces.com/a.mp4", true},
		{"平台签名代理", "https://platform.n-axis.com/v1/assets/asset-1/source?expires=1&sig=x", false},
		{"相对路径", "/v1/videos/task_x/content", false},
		{"非 http 协议", "ftp://ark.tos-cn-beijing.volces.com/a.mp4", false},
		{"伪装域名", "https://volces.com.evil.com/a.mp4", false},
		{"空", "", false},
	}
	for _, tc := range cases {
		if got := IsOfficialVideoURL(tc.url); got != tc.want {
			t.Fatalf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestShouldProxyVideoURL(t *testing.T) {
	allow := &model.Channel{Type: 54, Other: `{"allow_non_official_video_url":true}`}
	deny := &model.Channel{Type: 54, Other: `{}`}
	sdos2 := &model.Channel{Type: 54, Other: `{"asset_upstream_version":"sdos2"}`}

	nonOfficial := "https://platform.n-axis.com/v1/videos/x.mp4"
	if ShouldProxyVideoURL(allow, nonOfficial) {
		t.Fatalf("absolute http(s) non-official url must be returned as-is")
	}
	if !ShouldProxyVideoURL(allow, "/v1/videos/task_x/content") {
		t.Fatalf("relative non-official url must be proxied")
	}
	if ShouldProxyVideoURL(deny, nonOfficial) {
		t.Fatalf("deny channel must not proxy")
	}
	if ShouldProxyVideoURL(allow, officialSample) {
		t.Fatalf("official url must not be proxied even if allow is on")
	}
	if !ShouldProxyVideoURL(sdos2, "http://106.54.45.168/v1/videos/x/content") {
		t.Fatalf("sdos2 must always proxy")
	}
	if ShouldProxyVideoURL(nil, nonOfficial) {
		t.Fatalf("nil channel must not proxy")
	}
}
