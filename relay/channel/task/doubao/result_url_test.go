package doubao

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
)

func TestValidateVideoResultURL(t *testing.T) {
	plain := &TaskAdaptor{}
	naxis := &TaskAdaptor{resultURLRequireHTTPS: true}

	cases := []struct {
		name    string
		adaptor *TaskAdaptor
		raw     string
		wantErr string
	}{
		{"https ok", plain, "https://platform.n-axis.com/v1/videos/x/content", ""},
		{"http ok for non-naxis", plain, "http://106.54.45.168/seedance/video.mp4", ""},
		{"empty", plain, "  ", "empty result url"},
		{"relative path", plain, "/v1/videos/task_x/content", "unsupported scheme"},
		{"non http scheme", plain, "ftp://example.com/a.mp4", "unsupported scheme"},
		{"missing host", plain, "https:///a.mp4", "missing host"},
		{"naxis rejects http", naxis, "http://platform.n-axis.com/a.mp4", "requires https"},
		{"naxis accepts https", naxis, "https://platform.n-axis.com/a.mp4", ""},
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

func TestParseTaskResultNaxisRequiresHTTPS(t *testing.T) {
	adaptor := &TaskAdaptor{resultURLRequireHTTPS: true, baseURL: "http://platform.n-axis.com"}
	result, err := adaptor.ParseTaskResult([]byte(`{"id":"t1","model":"m","status":"succeeded","content":{"video_url":"https://platform.n-axis.com/a.mp4"}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != model.TaskStatusSuccess {
		t.Fatalf("expected success for https, got %s", result.Status)
	}

	result, err = adaptor.ParseTaskResult([]byte(`{"id":"t2","model":"m","status":"succeeded","content":{"video_url":"http://platform.n-axis.com/a.mp4"}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != model.TaskStatusFailure {
		t.Fatalf("expected failure for http on naxis, got %s", result.Status)
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
