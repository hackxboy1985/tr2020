package oss

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/setting/system_setting"
)

func TestSanitizeKeySegment(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"普通任务ID", "task_abc123", "task_abc123"},
		{"含斜杠", "a/b/c", "a_b_c"},
		{"含路径穿越", "../../etc/passwd", "____etc_passwd"},
		{"空字符串", "", "unknown"},
		{"仅空白", "   ", "unknown"},
		{"含特殊字符", "task:1*2?3", "task_1_2_3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeKeySegment(tt.in)
			if got != tt.want {
				t.Errorf("sanitizeKeySegment(%q) = %q, want %q", tt.in, got, tt.want)
			}
			// 关键安全断言：结果绝不能包含 .. 或 /
			if containsAny(got, "..", "/") {
				t.Errorf("sanitizeKeySegment(%q) = %q 含危险字符", tt.in, got)
			}
		})
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
	}
	return false
}

func TestExtFromURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"mp4", "https://cdn.example.com/a/b/video.mp4", ".mp4"},
		{"带query", "https://cdn.example.com/v.mp4?auth=xxx&t=1", ".mp4"},
		{"带fragment", "https://cdn.example.com/v.mov#frag", ".mov"},
		{"png", "https://cdn.example.com/img.png", ".png"},
		{"无扩展名", "https://cdn.example.com/video", ""},
		{"仅域名", "https://cdn.example.com", ""},
		{"大写扩展名", "https://cdn.example.com/A.MP4", ".mp4"},
		{"非法URL", "://bad", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extFromURL(tt.url); got != tt.want {
				t.Errorf("extFromURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestExtFromContentType(t *testing.T) {
	tests := []struct {
		name string
		ct   string
		want string
	}{
		{"mp4", "video/mp4", ".mp4"},
		{"mp4带参数", "video/mp4; charset=utf-8", ".mp4"},
		{"大写", "VIDEO/MP4", ".mp4"},
		{"png", "image/png", ".png"},
		{"jpeg", "image/jpeg", ".jpg"},
		{"未知类型", "application/octet-stream", ""},
		{"空", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extFromContentType(tt.ct); got != tt.want {
				t.Errorf("extFromContentType(%q) = %q, want %q", tt.ct, got, tt.want)
			}
		})
	}
}

func TestBuildObjectKey(t *testing.T) {
	origPrefix := system_setting.ObjectStoragePathPrefix
	defer func() { system_setting.ObjectStoragePathPrefix = origPrefix }()

	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		prefix      string
		taskID      string
		srcURL      string
		contentType string
		isVideo     bool
		want        string
	}{
		{
			name: "视频_从URL推断扩展名", prefix: "td/",
			taskID: "task_abc", srcURL: "https://c.com/v.mp4",
			isVideo: true, want: "td/2026/10/14/task_abc.mp4",
		},
		{
			name: "视频_从ContentType推断", prefix: "td/",
			taskID: "task_abc", srcURL: "https://c.com/download",
			contentType: "video/quicktime", isVideo: true,
			want: "td/2026/10/14/task_abc.mov",
		},
		{
			name: "视频_无任何线索则兜底mp4", prefix: "td/",
			taskID: "task_abc", srcURL: "https://c.com/download",
			isVideo: true, want: "td/2026/10/14/task_abc.mp4",
		},
		{
			name: "图片_无任何线索则兜底png", prefix: "td/",
			taskID: "task_img", srcURL: "https://c.com/download",
			isVideo: false, want: "td/2026/10/14/task_img.png",
		},
		{
			name: "空前缀", prefix: "",
			taskID: "task_x", srcURL: "https://c.com/a.png",
			want: "2026/10/14/task_x.png",
		},
		{
			name: "前缀首尾斜杠被规整", prefix: "/td/",
			taskID: "task_x", srcURL: "https://c.com/a.png",
			want: "td/2026/10/14/task_x.png",
		},
		{
			name: "任务ID含危险字符被清理", prefix: "td/",
			taskID: "../../evil", srcURL: "https://c.com/a.mp4",
			isVideo: true, want: "td/2026/10/14/____evil.mp4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			system_setting.ObjectStoragePathPrefix = tt.prefix
			got := BuildObjectKey(tt.taskID, tt.srcURL, tt.contentType, tt.isVideo, now)
			if got != tt.want {
				t.Errorf("BuildObjectKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPublicURL(t *testing.T) {
	origBase := system_setting.ObjectStoragePublicBaseURL
	origEndpoint := system_setting.ObjectStorageEndpoint
	origBucket := system_setting.ObjectStorageBucket
	defer func() {
		system_setting.ObjectStoragePublicBaseURL = origBase
		system_setting.ObjectStorageEndpoint = origEndpoint
		system_setting.ObjectStorageBucket = origBucket
	}()

	t.Run("优先使用PublicBaseURL", func(t *testing.T) {
		system_setting.ObjectStoragePublicBaseURL = "https://oss.example.com"
		system_setting.ObjectStorageEndpoint = "http://127.0.0.1:9000"
		system_setting.ObjectStorageBucket = "td-results"

		got := PublicURL("td/2026/10/14/a.mp4")
		want := "https://oss.example.com/td/2026/10/14/a.mp4"
		if got != want {
			t.Errorf("PublicURL() = %q, want %q", got, want)
		}
	})

	t.Run("PublicBaseURL带尾斜杠", func(t *testing.T) {
		system_setting.ObjectStoragePublicBaseURL = "https://oss.example.com/"
		got := PublicURL("td/a.mp4")
		want := "https://oss.example.com/td/a.mp4"
		if got != want {
			t.Errorf("PublicURL() = %q, want %q", got, want)
		}
	})

	t.Run("回退到虚拟主机风格", func(t *testing.T) {
		system_setting.ObjectStoragePublicBaseURL = ""
		system_setting.ObjectStorageEndpoint = "oss-cn-hangzhou.aliyuncs.com"
		system_setting.ObjectStorageBucket = "mybucket"

		got := PublicURL("td/a.mp4")
		want := "https://mybucket.oss-cn-hangzhou.aliyuncs.com/td/a.mp4"
		if got != want {
			t.Errorf("PublicURL() = %q, want %q", got, want)
		}
	})

	t.Run("回退时剥离scheme", func(t *testing.T) {
		system_setting.ObjectStoragePublicBaseURL = ""
		system_setting.ObjectStorageEndpoint = "https://oss.example.com"
		system_setting.ObjectStorageBucket = "b"

		got := PublicURL("k")
		want := "https://b.oss.example.com/k"
		if got != want {
			t.Errorf("PublicURL() = %q, want %q", got, want)
		}
	})
}

func TestIsConfigured(t *testing.T) {
	origEndpoint := system_setting.ObjectStorageEndpoint
	origBucket := system_setting.ObjectStorageBucket
	origAK := system_setting.ObjectStorageAccessKeyId
	origSK := system_setting.ObjectStorageAccessKeySecret
	defer func() {
		system_setting.ObjectStorageEndpoint = origEndpoint
		system_setting.ObjectStorageBucket = origBucket
		system_setting.ObjectStorageAccessKeyId = origAK
		system_setting.ObjectStorageAccessKeySecret = origSK
	}()

	// 全部齐全
	system_setting.ObjectStorageEndpoint = "http://127.0.0.1:9000"
	system_setting.ObjectStorageBucket = "b"
	system_setting.ObjectStorageAccessKeyId = "ak"
	system_setting.ObjectStorageAccessKeySecret = "sk"
	if !IsConfigured() {
		t.Error("四要素齐全时应为 true")
	}

	// 逐一缺失，都应判定为未配置
	cases := []struct {
		name string
		set  func()
	}{
		{"缺Endpoint", func() { system_setting.ObjectStorageEndpoint = "" }},
		{"缺Bucket", func() { system_setting.ObjectStorageBucket = "" }},
		{"缺AK", func() { system_setting.ObjectStorageAccessKeyId = "" }},
		{"缺SK", func() { system_setting.ObjectStorageAccessKeySecret = "" }},
		{"仅空白视为缺失", func() { system_setting.ObjectStorageEndpoint = "   " }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// 重置为齐全
			system_setting.ObjectStorageEndpoint = "http://127.0.0.1:9000"
			system_setting.ObjectStorageBucket = "b"
			system_setting.ObjectStorageAccessKeyId = "ak"
			system_setting.ObjectStorageAccessKeySecret = "sk"
			// 施加缺失
			tc.set()
			if IsConfigured() {
				t.Errorf("%s 时应为 false", tc.name)
			}
		})
	}
}

func TestNormalizeContentType(t *testing.T) {
	tests := []struct{ in, want string }{
		{"video/mp4", "video/mp4"},
		{"video/mp4; charset=utf-8", "video/mp4"},
		{"  VIDEO/MP4  ", "video/mp4"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := normalizeContentType(tt.in); got != tt.want {
			t.Errorf("normalizeContentType(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestIsMediaContentType(t *testing.T) {
	tests := []struct {
		ct   string
		want bool
	}{
		{"video/mp4", true},
		{"image/png", true},
		{"text/html", false},
		{"application/json", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isMediaContentType(tt.ct); got != tt.want {
			t.Errorf("isMediaContentType(%q) = %v, want %v", tt.ct, got, tt.want)
		}
	}
}
