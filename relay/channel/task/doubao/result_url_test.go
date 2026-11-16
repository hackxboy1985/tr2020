package doubao

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
)

func TestValidateVideoResultURL(t *testing.T) {
	adaptor := &TaskAdaptor{}

	cases := []struct {
		name    string
		adaptor *TaskAdaptor
		raw     string
		wantErr string
	}{
		{"https ok", adaptor, "https://platform.n-axis.com/v1/videos/x/content", ""},
		{"http ok", adaptor, "http://106.54.45.168/seedance/video.mp4", ""},
		{"empty", adaptor, "  ", "empty result url"},
		{"relative path", adaptor, "/v1/videos/task_x/content", "unsupported scheme"},
		{"non http scheme", adaptor, "ftp://example.com/a.mp4", "unsupported scheme"},
		{"missing host", adaptor, "https:///a.mp4", "missing host"},
	}

	for _, tc := range cases {
		got, err := tc.adaptor.validateVideoResultURL(tc.raw)
		if tc.wantErr == "" {
			if err != nil {
				t.Fatalf("%s: unexpected error: %v", tc.name, err)
			}
			if got != strings.TrimSpace(tc.raw) {
				t.Fatalf("%s: got %q, want %q", tc.name, got, strings.TrimSpace(tc.raw))
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
			t.Fatalf("%s: got %v, want contains %q", tc.name, err, tc.wantErr)
		}
	}
}

func TestParseTaskResultAcceptsAbsoluteHTTPSURL(t *testing.T) {
	adaptor := &TaskAdaptor{}
	result, err := adaptor.ParseTaskResult([]byte(`{"id":"t1","model":"m","status":"succeeded","content":{"video_url":"https://cdn.example.com/a.mp4"}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != model.TaskStatusSuccess {
		t.Fatalf("unexpected status: %s", result.Status)
	}
	if result.Url != "https://cdn.example.com/a.mp4" {
		t.Fatalf("unexpected url: %q", result.Url)
	}
}

func TestParseTaskResultRejectsRelativeURL(t *testing.T) {
	// baseURL 为空时相对路径无法补全 → 判失败，避免把 /v1/videos/xxx 这类地址传给下游
	adaptor := &TaskAdaptor{}
	result, err := adaptor.ParseTaskResult([]byte(`{"id":"t1","model":"m","status":"succeeded","content":{"video_url":"/v1/videos/task_x/content"}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != model.TaskStatusFailure {
		t.Fatalf("expected failure, got %s", result.Status)
	}
	if result.Url != "" {
		t.Fatalf("url must be cleared, got %q", result.Url)
	}
	if !strings.Contains(result.Reason, "invalid video url from upstream") {
		t.Fatalf("unexpected reason: %q", result.Reason)
	}
}

// http 与 https 都接受：只要求是"绝对链接"，不强制 https
func TestParseTaskResultAcceptsHTTPURL(t *testing.T) {
	adaptor := &TaskAdaptor{baseURL: "http://platform.n-axis.com"}
	result, err := adaptor.ParseTaskResult([]byte(`{"id":"t1","model":"m","status":"succeeded","content":{"video_url":"http://platform.n-axis.com/a.mp4"}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != model.TaskStatusSuccess {
		t.Fatalf("expected success for http, got %s", result.Status)
	}
	if result.Url != "http://platform.n-axis.com/a.mp4" {
		t.Fatalf("unexpected url: %q", result.Url)
	}
}

func TestParseTaskResultRejectsEmptyURLOnSuccess(t *testing.T) {
	adaptor := &TaskAdaptor{}
	result, err := adaptor.ParseTaskResult([]byte(`{"id":"t1","model":"m","status":"succeeded"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != model.TaskStatusFailure {
		t.Fatalf("expected failure, got %s", result.Status)
	}
	if !strings.Contains(result.Reason, "empty result url") {
		t.Fatalf("unexpected reason: %q", result.Reason)
	}
}

// 非法成片地址必须留下可筛选的失败码
func TestParseTaskResultSetsFailCodeInvalidVideoURL(t *testing.T) {
	adaptor := &TaskAdaptor{}
	result, err := adaptor.ParseTaskResult([]byte(`{"id":"t1","model":"m","status":"succeeded","content":{"video_url":"/v1/videos/task_x/content"}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != model.TaskStatusFailure {
		t.Fatalf("expected failure, got %s", result.Status)
	}
	if result.FailCode != model.FailCodeUpstreamVideoUnavailable {
		t.Fatalf("unexpected fail code: %q", result.FailCode)
	}
}

// 渠道允许非官链但 Base URL 为空：属配置问题，单独失败码
func TestParseTaskResultSetsFailCodeChannelBaseURLMissing(t *testing.T) {
	adaptor := &TaskAdaptor{allowNonOfficialVideoURL: true}
	result, err := adaptor.ParseTaskResult([]byte(`{"id":"t1","model":"m","status":"succeeded","content":{"video_url":"/v1/videos/task_x/content"}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != model.TaskStatusFailure {
		t.Fatalf("expected failure, got %s", result.Status)
	}
	if result.FailCode != model.FailCodeChannelBaseURLMissing {
		t.Fatalf("unexpected fail code: %q", result.FailCode)
	}
}

// 允许非官链 + 渠道 Base URL 可拼：相对路径补全后通过
func TestParseTaskResultAllowsResolvedRelativeURL(t *testing.T) {
	adaptor := &TaskAdaptor{allowNonOfficialVideoURL: true, baseURL: "https://up.example.com"}
	result, err := adaptor.ParseTaskResult([]byte(`{"id":"t1","model":"m","status":"succeeded","content":{"video_url":"/v1/videos/task_x/content"}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != model.TaskStatusSuccess {
		t.Fatalf("expected success, got %s (%s)", result.Status, result.Reason)
	}
	if result.Url != "https://up.example.com/v1/videos/task_x/content" {
		t.Fatalf("unexpected url: %q", result.Url)
	}
	if result.FailCode != "" {
		t.Fatalf("unexpected fail code: %q", result.FailCode)
	}
}

const officialVideoURL = "https://ark-acg-cn-beijing.tos-cn-beijing.volces.com/a.mp4?X-Tos-Signature=x"

// n-axis 强制官链：绝对地址但非 .volces.com → NON_OFFICIAL_VIDEO_URL
func TestParseTaskResultNaxisRejectsNonOfficialAbsoluteURL(t *testing.T) {
	adaptor := &TaskAdaptor{enforceOfficialVideoURL: true}
	result, err := adaptor.ParseTaskResult([]byte(`{"id":"t1","model":"m","status":"succeeded","content":{"video_url":"https://platform.n-axis.com/v1/videos/x.mp4"}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != model.TaskStatusFailure {
		t.Fatalf("expected failure, got %s", result.Status)
	}
	if result.FailCode != model.FailCodeUpstreamVideoUnavailable {
		t.Fatalf("unexpected fail code: %q", result.FailCode)
	}
}

// n-axis 强制官链：官方 .volces.com 直链通过
func TestParseTaskResultNaxisAcceptsOfficialURL(t *testing.T) {
	adaptor := &TaskAdaptor{enforceOfficialVideoURL: true}
	body := []byte(`{"id":"t1","model":"m","status":"succeeded","content":{"video_url":"` + officialVideoURL + `"}}`)
	result, err := adaptor.ParseTaskResult(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != model.TaskStatusSuccess {
		t.Fatalf("expected success, got %s (%s)", result.Status, result.Reason)
	}
	if result.Url != officialVideoURL {
		t.Fatalf("unexpected url: %q", result.Url)
	}
}

// 老上游（不强制官链）保持原行为：非官方绝对地址照旧通过
func TestParseTaskResultLegacyUpstreamAcceptsNonOfficialURL(t *testing.T) {
	adaptor := &TaskAdaptor{}
	result, err := adaptor.ParseTaskResult([]byte(`{"id":"t1","model":"m","status":"succeeded","content":{"video_url":"https://platform.n-axis.com/v1/videos/x.mp4"}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != model.TaskStatusSuccess {
		t.Fatalf("expected success, got %s (%s)", result.Status, result.Reason)
	}
}

// n-axis 强制官链 + 渠道允许非官链：非官链地址放行
func TestParseTaskResultNaxisAllowsNonOfficialWhenChannelAllows(t *testing.T) {
	adaptor := &TaskAdaptor{enforceOfficialVideoURL: true, allowNonOfficialVideoURL: true}
	result, err := adaptor.ParseTaskResult([]byte(`{"id":"t1","model":"m","status":"succeeded","content":{"video_url":"https://platform.n-axis.com/v1/videos/x.mp4"}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != model.TaskStatusSuccess {
		t.Fatalf("expected success, got %s (%s)", result.Status, result.Reason)
	}
}
