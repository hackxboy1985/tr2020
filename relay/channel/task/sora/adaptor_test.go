package sora

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
)

func TestParseTaskResultCompletedVideoURL(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{
			name: "Tudou through OpenAI prefers original video_url",
			body: `{"code":200,"data":{"id":"upstream","status":"completed","result":{"video_url":"https://mid.ai-tudou.net/generated/x.mp4","videos":[{"url":["https://mid.ai-tudou.net/generated/fallback.mp4"]}]}},"id":"upstream","status":"completed"}`,
			want: "https://mid.ai-tudou.net/generated/x.mp4",
		},
		{
			name: "Tudou falls back to first usable videos URL",
			body: `{"code":200,"data":{"id":"upstream","status":"completed","result":{"video_url":" ","videos":[{"url":[null,"/v1/videos/upstream/content"]},{"url":["https://cdn.example.com/video?signature=a%2Fb&expires=123","https://cdn.example.com/later.mp4"]}]}},"id":"upstream","status":"completed"}`,
			want: "https://cdn.example.com/video?signature=a%2Fb&expires=123",
		},
		{
			name: "flat video_url",
			body: `{"id":"upstream","status":"completed","video_url":"https://cdn.example.com/flat.mp4"}`,
			want: "https://cdn.example.com/flat.mp4",
		},
		{
			name: "flat download_url",
			body: `{"id":"upstream","status":"completed","download_url":"https://cdn.example.com/download"}`,
			want: "https://cdn.example.com/download",
		},
		{
			name: "unenveloped result with string videos URL",
			body: `{"id":"upstream","status":"completed","result":{"videos":[{"url":"https://cdn.example.com/result.mp4"}]}}`,
			want: "https://cdn.example.com/result.mp4",
		},
		{
			name: "url alias",
			body: `{"id":"upstream","status":"completed","url":"https://cdn.example.com/flat.mp4"}`,
			want: "https://cdn.example.com/flat.mp4",
		},
		{
			name: "OpenAI without direct URL leaves proxy fallback to caller",
			body: `{"id":"video_123","object":"video","status":"completed","progress":100}`,
		},
		{
			name: "unusable optional metadata leaves proxy fallback to caller",
			body: `{"id":"upstream","status":"completed","video_url":42,"url":"data:video/mp4;base64,AAAA","data":{"result":["unexpected"]}}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := (&TaskAdaptor{}).ParseTaskResult([]byte(tc.body))
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != model.TaskStatusSuccess || got.Url != tc.want {
				t.Fatalf("got status %q and URL %q; want %q and %q", got.Status, got.Url, model.TaskStatusSuccess, tc.want)
			}
		})
	}
}

func TestParseTaskResultPreservesStatusParsing(t *testing.T) {
	for _, tc := range []struct {
		body     string
		status   string
		progress string
		reason   string
	}{
		{`{"status":"pending","video_url":"https://cdn.example.com/preview.mp4"}`, model.TaskStatusQueued, "", ""},
		{`{"status":"in_progress","progress":25,"data":{"status":"completed","result":{"video_url":"https://cdn.example.com/preview.mp4"}}}`, model.TaskStatusInProgress, "25%", ""},
		{`{"status":"failed","error":{"message":"upstream failed"},"video_url":"https://cdn.example.com/preview.mp4"}`, model.TaskStatusFailure, "", "upstream failed"},
		{`{"status":"cancelled"}`, model.TaskStatusFailure, "", "task failed"},
	} {
		got, err := (&TaskAdaptor{}).ParseTaskResult([]byte(tc.body))
		if err != nil {
			t.Fatal(err)
		}
		if got.Status != tc.status || got.Progress != tc.progress || got.Reason != tc.reason || got.Url != "" {
			t.Errorf("%s: got %+v; want status %q, progress %q, reason %q and an empty URL", tc.body, got, tc.status, tc.progress, tc.reason)
		}
	}
}
