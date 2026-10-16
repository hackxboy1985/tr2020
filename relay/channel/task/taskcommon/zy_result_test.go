package taskcommon

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
)

func TestZyResultPriorityAndMediaType(t *testing.T) {
	for _, tt := range []struct {
		name, body, want string
		video            bool
		count            int
	}{
		{"image_priority", `{"model":"gpt-image-2-4K","object":"video","image_url":"https://cdn/image.png","url":"https://cdn/other.png","video_url":"https://cdn/alias.png"}`, "https://cdn/image.png", false, 2},
		{"image_url_fallback", `{"model":"gpt-image-2","url":"https://cdn/image.png","video_url":"https://cdn/alias.png"}`, "https://cdn/image.png", false, 1},
		{"image_not_video_alias", `{"model":"gpt-image-2","video_url":"https://cdn/alias.png"}`, "", false, 0},
		{"image_dedup", `{"image_url":" https://cdn/image.png ","url":"https://cdn/image.png","video_url":"https://cdn/image.png"}`, "https://cdn/image.png", false, 1},
		{"png_with_video_wrapper", `{"object":"video","url":"https://cdn/image.png?x=1","video_url":"https://cdn/image.png?x=1"}`, "https://cdn/image.png?x=1", false, 1},
		{"video_priority", `{"model":"sora-2","url":"https://cdn/other.mp4","video_url":"https://cdn/video.mp4"}`, "https://cdn/video.mp4", true, 2},
		{"video_url_fallback", `{"url":"https://cdn/video.MP4?auth=a"}`, "https://cdn/video.MP4?auth=a", true, 1},
		{"video_dedup", `{"url":"https://cdn/video.mp4","video_url":"https://cdn/video.mp4"}`, "https://cdn/video.mp4", true, 1},
		{"legacy_metadata", `{"metadata":{"image_url":"https://cdn/image.png","image":"https://cdn/image.png"}}`, "https://cdn/image.png", false, 1},
		{"empty", `{}`, "", false, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var result ZyTaskResult
			if err := common.Unmarshal([]byte(tt.body), &result); err != nil {
				t.Fatal(err)
			}
			if got := result.ResultURL(); got != tt.want {
				t.Fatalf("url = %q, want %q", got, tt.want)
			}
			if got := result.IsVideo(); got != tt.video {
				t.Fatalf("video = %v, want %v", got, tt.video)
			}
			if got := len(result.ResultURLs()); got != tt.count {
				t.Fatalf("unique urls = %d, want %d", got, tt.count)
			}
		})
	}
}
