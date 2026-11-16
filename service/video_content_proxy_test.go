package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
)

func TestNeedsVideoContentProxy(t *testing.T) {
	cases := []struct {
		name  string
		other string
		want  bool
	}{
		{"默认不代理", "", false},
		{"sdos2 必需代理", `{"asset_upstream_version":"sdos2"}`, true},
		{"allow_non_official_video_url 打开", `{"allow_non_official_video_url":true}`, true},
		{"n-axis 默认不代理", `{"asset_upstream_version":"naxis"}`, false},
	}

	for _, tc := range cases {
		ch := &model.Channel{Type: 54, Other: tc.other}
		if got := NeedsVideoContentProxy(ch); got != tc.want {
			t.Fatalf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}

	if NeedsVideoContentProxy(nil) {
		t.Fatalf("nil channel must not need proxy")
	}
}
