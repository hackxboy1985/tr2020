package oss

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/setting/system_setting"
)

// 源站以 application/octet-stream 返回真实图片时，应按 URL 后缀识别并成功转存。
func TestUploadFromURLUsesSourceExtension(t *testing.T) {
	for _, tc := range []struct {
		name     string
		path     string
		body     string
		wantMIME string
	}{
		{"png", "cb67f52a.png", "\x89PNG\r\n\x1a\nfake-image-bytes", "image/png"},
		{"jpeg", "image.jpg", "\xff\xd8\xff\xe0fake-image-bytes", "image/jpeg"},
		{"mp4", "clip.mp4", "\x00\x00\x00\x18ftypisomfake-video", "video/mp4"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/octet-stream")
				io.WriteString(w, tc.body)
			}))
			defer source.Close()

			var uploadedType string
			var calls atomic.Int32
			store := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				uploadedType = r.Header.Get("Content-Type")
				w.Header().Set("ETag", "\"etag\"")
			}))
			defer store.Close()

			restore := configureTestStorage(t, store.URL)
			defer restore()

			result, err := UploadFromURL(context.Background(), source.URL+"/media/"+tc.path, "td/2026/10/21/task_x"+extFromURL(tc.path))
			if err != nil {
				t.Fatalf("upload failed: %v", err)
			}
			if uploadedType != tc.wantMIME || result.ContentType != tc.wantMIME {
				t.Fatalf("content-type = %q / %q, want %q", uploadedType, result.ContentType, tc.wantMIME)
			}
			if calls.Load() != 1 {
				t.Fatalf("upload calls = %d, want 1", calls.Load())
			}
		})
	}
}

// 明确声明为非媒体的响应仍应被拦截，避免把错误页写入对象存储。
func TestUploadFromURLRejectsDeclaredNonMedia(t *testing.T) {
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		io.WriteString(w, "<html>error</html>")
	}))
	defer source.Close()

	var calls atomic.Int32
	store := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
	}))
	defer store.Close()
	restore := configureTestStorage(t, store.URL)
	defer restore()

	if _, err := UploadFromURL(context.Background(), source.URL+"/a.png", "td/x.png"); err == nil {
		t.Fatal("declared text/html must be rejected")
	}
	if calls.Load() != 0 {
		t.Fatal("rejected content must not be uploaded")
	}
}

func TestBuildChannelObjectKey(t *testing.T) {
	orig := system_setting.ObjectStoragePathPrefix
	defer func() { system_setting.ObjectStoragePathPrefix = orig }()
	now := time.Date(2026, 10, 21, 0, 0, 0, 0, time.Local)

	system_setting.ObjectStoragePathPrefix = "td/"
	if got, want := BuildChannelObjectKey("task_x", "https://a/b.png", false, "zy", now), "td/zy/2026/10/21/task_x.png"; got != want {
		t.Fatalf("zy key = %q, want %q", got, want)
	}
	if got, want := BuildChannelObjectKey("task_x", "https://a/b.mp4", true, "td", now), "td/td/2026/10/21/task_x.mp4"; got != want {
		t.Fatalf("td key = %q, want %q", got, want)
	}
	system_setting.ObjectStoragePathPrefix = ""
	if got, want := BuildChannelObjectKey("task_x", "https://a/b.png", false, "zy", now), "zy/2026/10/21/task_x.png"; got != want {
		t.Fatalf("empty prefix key = %q, want %q", got, want)
	}
}

func configureTestStorage(t *testing.T, endpoint string) func() {
	t.Helper()
	endpoint0, bucket := system_setting.ObjectStorageEndpoint, system_setting.ObjectStorageBucket
	ak, sk := system_setting.ObjectStorageAccessKeyId, system_setting.ObjectStorageAccessKeySecret
	base, pathStyle := system_setting.ObjectStoragePublicBaseURL, system_setting.ObjectStorageForcePathStyle
	max := system_setting.ObjectStorageMaxFileSize
	fetch := system_setting.GetFetchSetting()
	ssrf := fetch.EnableSSRFProtection

	system_setting.ObjectStorageEndpoint, system_setting.ObjectStorageBucket = endpoint, "results"
	system_setting.ObjectStorageAccessKeyId, system_setting.ObjectStorageAccessKeySecret = "test-ak", "test-sk"
	system_setting.ObjectStoragePublicBaseURL, system_setting.ObjectStorageForcePathStyle = "https://storage.example", true
	system_setting.ObjectStorageMaxFileSize = 1024
	fetch.EnableSSRFProtection = false
	t.Setenv("TMPDIR", t.TempDir())

	return func() {
		system_setting.ObjectStorageEndpoint, system_setting.ObjectStorageBucket = endpoint0, bucket
		system_setting.ObjectStorageAccessKeyId, system_setting.ObjectStorageAccessKeySecret = ak, sk
		system_setting.ObjectStoragePublicBaseURL, system_setting.ObjectStorageForcePathStyle = base, pathStyle
		system_setting.ObjectStorageMaxFileSize = max
		fetch.EnableSSRFProtection = ssrf
	}
}
