package oss

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/setting/system_setting"
)

func TestSpoolSource(t *testing.T) {
	for _, tt := range []struct {
		name     string
		body     string
		max      int64
		tooLarge bool
	}{
		{"unknown-length", "image bytes", 100, false},
		{"exact-limit", "12345", 5, false},
		{"over-limit", "123456", 5, true},
		{"empty", "", 5, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("TMPDIR", dir)
			file, size, err := spoolSource(strings.NewReader(tt.body), tt.max)
			if tt.tooLarge {
				if !errors.Is(err, ErrFileTooLarge) {
					t.Fatalf("error = %v", err)
				}
				entries, _ := os.ReadDir(dir)
				if len(entries) != 0 {
					t.Fatal("failed download leaked temporary file")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(file.Name())
			defer file.Close()
			if size != int64(len(tt.body)) {
				t.Fatalf("size = %d", size)
			}
			for i := 0; i < 2; i++ {
				if _, err := file.Seek(0, io.SeekStart); err != nil {
					t.Fatal(err)
				}
				body, err := io.ReadAll(file)
				if err != nil || string(body) != tt.body {
					t.Fatalf("read = %q, %v", body, err)
				}
			}
		})
	}
}

// 使用真实 S3 SDK 和 HTTP 服务，验证 HTTP 端点签名及 SDK 失败重试可回读 Body。
func TestUploadFromURLSeekableBody(t *testing.T) {
	for _, unknownLength := range []bool{false, true} {
		name := "known-length"
		if unknownLength {
			name = "chunked-source"
		}
		t.Run(name, func(t *testing.T) {
			const payload = "generated image contents"
			source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "image/png")
				if unknownLength {
					w.(http.Flusher).Flush()
				}
				io.WriteString(w, payload)
			}))
			defer source.Close()
			var calls atomic.Int32
			store := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil || string(body) != payload {
					t.Errorf("uploaded body = %q, %v", body, err)
				}
				if r.Method != http.MethodPut || r.URL.Path != "/results/td/task.png" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				if r.Header.Get("Authorization") == "" || r.Header.Get("X-Amz-Content-Sha256") == "" {
					t.Error("missing S3 signature")
				}
				if calls.Add(1) == 1 {
					w.Header().Set("Content-Type", "application/xml")
					w.WriteHeader(http.StatusServiceUnavailable)
					io.WriteString(w, "<Error><Code>SlowDown</Code><Message>retry</Message></Error>")
					return
				}
				w.Header().Set("ETag", "\"test-etag\"")
			}))
			defer store.Close()

			endpoint, bucket := system_setting.ObjectStorageEndpoint, system_setting.ObjectStorageBucket
			ak, sk := system_setting.ObjectStorageAccessKeyId, system_setting.ObjectStorageAccessKeySecret
			base, pathStyle := system_setting.ObjectStoragePublicBaseURL, system_setting.ObjectStorageForcePathStyle
			max := system_setting.ObjectStorageMaxFileSize
			fetch := system_setting.GetFetchSetting()
			ssrf := fetch.EnableSSRFProtection
			t.Cleanup(func() {
				system_setting.ObjectStorageEndpoint, system_setting.ObjectStorageBucket = endpoint, bucket
				system_setting.ObjectStorageAccessKeyId, system_setting.ObjectStorageAccessKeySecret = ak, sk
				system_setting.ObjectStoragePublicBaseURL, system_setting.ObjectStorageForcePathStyle = base, pathStyle
				system_setting.ObjectStorageMaxFileSize = max
				fetch.EnableSSRFProtection = ssrf
			})
			system_setting.ObjectStorageEndpoint, system_setting.ObjectStorageBucket = store.URL, "results"
			system_setting.ObjectStorageAccessKeyId, system_setting.ObjectStorageAccessKeySecret = "test-ak", "test-sk"
			system_setting.ObjectStoragePublicBaseURL, system_setting.ObjectStorageForcePathStyle = "https://storage.example", true
			system_setting.ObjectStorageMaxFileSize = 1024
			fetch.EnableSSRFProtection = false
			dir := t.TempDir()
			t.Setenv("TMPDIR", dir)

			result, err := UploadFromURL(context.Background(), source.URL, "td/task.png")
			if err != nil {
				t.Fatal(err)
			}
			if result.Size != int64(len(payload)) || result.PublicURL != "https://storage.example/results/td/task.png" {
				t.Fatalf("unexpected result: %+v", result)
			}
			if calls.Load() != 2 {
				t.Fatalf("S3 attempts = %d, want 2", calls.Load())
			}
			entries, _ := os.ReadDir(dir)
			if len(entries) != 0 {
				t.Fatal("upload leaked temporary file")
			}
		})
	}
}
