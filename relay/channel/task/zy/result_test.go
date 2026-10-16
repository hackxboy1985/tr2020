package zy

import "testing"

func TestParseTaskResultMediaURLPriority(t *testing.T) {
	for _, tt := range []struct{ body, want string }{
		{`{"status":"completed","object":"video","model":"gpt-image-2-4K","image_url":"https://cdn/image.png","url":"https://cdn/other.png","video_url":"https://cdn/alias.png"}`, "https://cdn/image.png"},
		{`{"status":"completed","url":"https://cdn/image.png","video_url":"https://cdn/alias.png"}`, "https://cdn/image.png"},
		{`{"status":"completed","url":"https://cdn/other.mp4","video_url":"https://cdn/video.mp4"}`, "https://cdn/video.mp4"},
		{`{"status":"completed","url":"https://cdn/video.mp4"}`, "https://cdn/video.mp4"},
	} {
		result, err := (&TaskAdaptor{}).ParseTaskResult([]byte(tt.body))
		if err != nil {
			t.Fatal(err)
		}
		if result.Url != tt.want {
			t.Fatalf("url = %q, want %q", result.Url, tt.want)
		}
	}
}
