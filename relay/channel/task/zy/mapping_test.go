package zy

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
)

// TestResolveSizeConfigAspectRatioPriority 验证 aspect_ratio 优先于 size
func TestResolveSizeConfigAspectRatioPriority(t *testing.T) {
	cfg, passthrough, err := resolveSizeConfig("16:9", "1024x1024", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AspectRatio != "16:9" {
		t.Errorf("AspectRatio = %q, want 16:9", cfg.AspectRatio)
	}
	if passthrough != "" {
		t.Errorf("passthrough = %q, want empty", passthrough)
	}
	// size 被忽略时，档位应回落到默认 1K
	if cfg.Resolution != Resolution1K {
		t.Errorf("Resolution = %q, want 1K", cfg.Resolution)
	}
}

// TestResolveSizeConfigPixelSize 验证像素尺寸反查比例与档位
func TestResolveSizeConfigPixelSize(t *testing.T) {
	cases := []struct {
		size       string
		wantRatio  string
		wantResols string
	}{
		{"1024x1024", "1:1", Resolution1K},
		{"2048x2048", "1:1", Resolution2K},
		{"2880x2880", "1:1", Resolution4K},
		{"1280x720", "16:9", Resolution1K},
		{"2560x1440", "16:9", Resolution2K},
		{"3840x2160", "16:9", Resolution4K},
		{"1456x624", "21:9", Resolution1K},
		{"3024x1296", "21:9", Resolution2K},
		{"3696x1584", "21:9", Resolution4K},
		{"720x1280", "9:16", Resolution1K},
		{"2160x3840", "9:16", Resolution4K},
	}

	for _, tc := range cases {
		cfg, passthrough, err := resolveSizeConfig("", tc.size, "", "")
		if err != nil {
			t.Fatalf("size=%s unexpected error: %v", tc.size, err)
		}
		if passthrough != "" {
			t.Errorf("size=%s passthrough = %q, want empty", tc.size, passthrough)
		}
		if cfg.AspectRatio != tc.wantRatio {
			t.Errorf("size=%s AspectRatio = %q, want %q", tc.size, cfg.AspectRatio, tc.wantRatio)
		}
		if cfg.Resolution != tc.wantResols {
			t.Errorf("size=%s Resolution = %q, want %q", tc.size, cfg.Resolution, tc.wantResols)
		}
	}
}

// TestResolveSizeConfigUnknownPixel 验证对照表外的像素尺寸原样透传
func TestResolveSizeConfigUnknownPixel(t *testing.T) {
	cfg, passthrough, err := resolveSizeConfig("", "1500x1000", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if passthrough != "1500x1000" {
		t.Errorf("passthrough = %q, want 1500x1000", passthrough)
	}
	if cfg.AspectRatio != "" {
		t.Errorf("AspectRatio = %q, want empty", cfg.AspectRatio)
	}
}

// TestResolveSizeConfigRatioSize 验证 size 传比例字符串等价于 aspect_ratio
func TestResolveSizeConfigRatioSize(t *testing.T) {
	cfg, passthrough, err := resolveSizeConfig("", "16:9", "2K", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AspectRatio != "16:9" {
		t.Errorf("AspectRatio = %q, want 16:9", cfg.AspectRatio)
	}
	if cfg.Resolution != Resolution2K {
		t.Errorf("Resolution = %q, want 2K", cfg.Resolution)
	}
	if passthrough != "" {
		t.Errorf("passthrough = %q, want empty", passthrough)
	}
}

// TestResolveSizeConfigAuto 验证 auto 比例
func TestResolveSizeConfigAuto(t *testing.T) {
	cfg, _, err := resolveSizeConfig("auto", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AspectRatio != AutoAspectRatio {
		t.Errorf("AspectRatio = %q, want auto", cfg.AspectRatio)
	}

	// 大小写与空格应被归一化
	cfg, _, err = resolveSizeConfig("AUTO", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AspectRatio != AutoAspectRatio {
		t.Errorf("AspectRatio = %q, want auto", cfg.AspectRatio)
	}
}

// TestResolveSizeConfigInvalid 验证非法输入被拒绝
func TestResolveSizeConfigInvalid(t *testing.T) {
	cases := []struct {
		name        string
		aspectRatio string
		size        string
		imageSize   string
		resolution  string
	}{
		{"非法比例", "17:9", "", "", ""},
		{"非法 size 比例", "", "17:9", "", ""},
		{"非法 size 格式", "", "abc", "", ""},
		{"非法档位", "", "", "8K", ""},
		{"非法 resolution", "", "", "", "3K"},
		{"image_size 与 resolution 冲突", "", "", "1K", "4K"},
	}

	for _, tc := range cases {
		_, _, err := resolveSizeConfig(tc.aspectRatio, tc.size, tc.imageSize, tc.resolution)
		if err == nil {
			t.Errorf("%s: expected error, got nil", tc.name)
		}
	}
}

// TestResolveSizeConfigImageSizeEqualsResolution 验证 image_size 与 resolution 同值合法
func TestResolveSizeConfigImageSizeEqualsResolution(t *testing.T) {
	// 同为 2K（大小写不同）应视为一致
	if _, _, err := resolveSizeConfig("", "", "2K", "2k"); err != nil {
		t.Errorf("unexpected error for equal tiers: %v", err)
	}
}

// TestResolveSizeConfigEmpty 验证全部为空时不指定比例
func TestResolveSizeConfigEmpty(t *testing.T) {
	cfg, passthrough, err := resolveSizeConfig("", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AspectRatio != "" {
		t.Errorf("AspectRatio = %q, want empty", cfg.AspectRatio)
	}
	if passthrough != "" {
		t.Errorf("passthrough = %q, want empty", passthrough)
	}
}

// TestBuildSubmitRequestFlareParamTier 验证 flare/sunburst 用参数选档
func TestBuildSubmitRequestFlareParamTier(t *testing.T) {
	cfg, passthrough, err := resolveSizeConfig("21:9", "", "4K", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	req, err := buildSubmitRequest("gpt-image-2.5-flare", "a cat", cfg, passthrough, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.AspectRatio != "21:9" {
		t.Errorf("AspectRatio = %q, want 21:9", req.AspectRatio)
	}
	if req.ImageSize != Resolution4K {
		t.Errorf("ImageSize = %q, want 4K", req.ImageSize)
	}
	if req.Size != "" {
		t.Errorf("Size = %q, want empty", req.Size)
	}
}

// TestBuildSubmitRequestFlarePixelSize 验证 flare 传对照表外的像素尺寸时直接下发 size
func TestBuildSubmitRequestFlarePixelSize(t *testing.T) {
	cfg, passthrough, err := resolveSizeConfig("", "1500x1000", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	req, err := buildSubmitRequest("gpt-image-2.5-sunburst", "a cat", cfg, passthrough, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Size != "1500x1000" {
		t.Errorf("Size = %q, want 1500x1000", req.Size)
	}
	// 像素已决定分辨率，不应再传档位
	if req.ImageSize != "" {
		t.Errorf("ImageSize = %q, want empty", req.ImageSize)
	}
}

// TestBuildSubmitRequestFlareMappedPixelSize 验证命中对照表的像素尺寸转为 aspect_ratio + 档位
func TestBuildSubmitRequestFlareMappedPixelSize(t *testing.T) {
	cfg, passthrough, err := resolveSizeConfig("", "3696x1584", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	req, err := buildSubmitRequest("gpt-image-2.5-sunburst", "a cat", cfg, passthrough, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 3696x1584 是 21:9 的 4K，两种下发方式等价，这里走比例+档位
	if req.AspectRatio != "21:9" || req.ImageSize != Resolution4K {
		t.Errorf("got aspect_ratio=%q image_size=%q, want 21:9 / 4K", req.AspectRatio, req.ImageSize)
	}
	if req.Size != "" {
		t.Errorf("Size = %q, want empty", req.Size)
	}
}

// TestBuildSubmitRequestTierInModelName 验证旧档模型不下发档位
func TestBuildSubmitRequestTierInModelName(t *testing.T) {
	for _, model := range []string{"gpt-image-2", "gpt-image-2-2K", "gpt-image-2-4K"} {
		cfg, passthrough, err := resolveSizeConfig("16:9", "", "4K", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		req, err := buildSubmitRequest(model, "a cat", cfg, passthrough, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if req.AspectRatio != "16:9" {
			t.Errorf("model=%s AspectRatio = %q, want 16:9", model, req.AspectRatio)
		}
		// 档位写在模型名里，不能再传 image_size
		if req.ImageSize != "" {
			t.Errorf("model=%s ImageSize = %q, want empty", model, req.ImageSize)
		}
	}
}

// TestBuildSubmitRequestOnly1K 验证 gpt-image-2.5 不下发档位
func TestBuildSubmitRequestOnly1K(t *testing.T) {
	cfg, passthrough, err := resolveSizeConfig("1:1", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	req, err := buildSubmitRequest("gpt-image-2.5", "a cat", cfg, passthrough, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.ImageSize != "" {
		t.Errorf("ImageSize = %q, want empty", req.ImageSize)
	}
	if req.AspectRatio != "1:1" {
		t.Errorf("AspectRatio = %q, want 1:1", req.AspectRatio)
	}
}

// TestValidateTierSupport 验证 gpt-image-2.5 仅支持 1K
func TestValidateTierSupport(t *testing.T) {
	if err := validateTierSupport("gpt-image-2.5", "", "2K"); err == nil {
		t.Error("expected error for gpt-image-2.5 with 2K")
	}
	if err := validateTierSupport("gpt-image-2.5", "", "4K"); err == nil {
		t.Error("expected error for gpt-image-2.5 with 4K")
	}
	if err := validateTierSupport("gpt-image-2.5", "1K", ""); err != nil {
		t.Errorf("unexpected error for gpt-image-2.5 with 1K: %v", err)
	}
	if err := validateTierSupport("gpt-image-2.5", "", ""); err != nil {
		t.Errorf("unexpected error for gpt-image-2.5 with default tier: %v", err)
	}
	if err := validateTierSupport("gpt-image-2.5-flare", "", "4K"); err != nil {
		t.Errorf("unexpected error for flare with 4K: %v", err)
	}
}

// TestResolvedModelForTier 验证模型名回退逻辑
func TestResolvedModelForTier(t *testing.T) {
	if got := resolvedModelForTier("gpt-image-2.5", ""); got != "gpt-image-2.5" {
		t.Errorf("got %q, want request model", got)
	}
	if got := resolvedModelForTier("gpt-image-2.5", "gpt-image-2.5-flare"); got != "gpt-image-2.5-flare" {
		t.Errorf("got %q, want upstream model", got)
	}
}

// TestIsPixelSizeAndRatioSize 验证尺寸格式判定
func TestIsPixelSizeAndRatioSize(t *testing.T) {
	pixelSizes := []string{"1024x1024", "1280X720", " 512x512 "}
	for _, s := range pixelSizes {
		if !IsPixelSize(s) {
			t.Errorf("IsPixelSize(%q) = false, want true", s)
		}
		if IsRatioSize(s) {
			t.Errorf("IsRatioSize(%q) = true, want false", s)
		}
	}
	invalidPixels := []string{"", "abc", "1024", "0x100", "axb", "16:9"}
	for _, s := range invalidPixels {
		if IsPixelSize(s) {
			t.Errorf("IsPixelSize(%q) = true, want false", s)
		}
	}
	ratioSizes := []string{"16:9", "1:1"}
	for _, s := range ratioSizes {
		if !IsRatioSize(s) {
			t.Errorf("IsRatioSize(%q) = false, want true", s)
		}
	}
}

// TestTaskResultURL 验证结果 URL 提取的优先级
func TestTaskResultURL(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "顶层 url 优先",
			body: `{"id":"task_1","status":"completed","url":"https://cdn.example.com/a.png",
				"metadata":{"image_url":"https://cdn.example.com/b.png","image":"https://cdn.example.com/c.png"}}`,
			want: "https://cdn.example.com/a.png",
		},
		{
			name: "回退 metadata.image_url",
			body: `{"id":"task_1","status":"completed",
				"metadata":{"image_url":"https://cdn.example.com/b.png","image":"https://cdn.example.com/c.png"}}`,
			want: "https://cdn.example.com/b.png",
		},
		{
			name: "回退 metadata.image",
			body: `{"id":"task_1","status":"completed","metadata":{"image":"https://cdn.example.com/c.png"}}`,
			want: "https://cdn.example.com/c.png",
		},
		{
			name: "全部为空",
			body: `{"id":"task_1","status":"completed"}`,
			want: "",
		},
	}

	for _, tc := range cases {
		var resp taskResponse
		if err := common.Unmarshal([]byte(tc.body), &resp); err != nil {
			t.Fatalf("%s: unmarshal failed: %v", tc.name, err)
		}
		if got := resp.resultURL(); got != tc.want {
			t.Errorf("%s: resultURL() = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// TestErrorMessage 验证错误信息提取
func TestErrorMessage(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "返回 message",
			body: `{"id":"task_1","status":"failed","error":{"message":"boom","code":"upstream_error"}}`,
			want: "boom",
		},
		{
			name: "无 message 时回退 code",
			body: `{"id":"task_1","status":"failed","error":{"code":"upstream_error"}}`,
			want: "upstream_error",
		},
		{
			name: "无 error 字段",
			body: `{"id":"task_1","status":"failed"}`,
			want: "",
		},
	}

	for _, tc := range cases {
		var resp taskResponse
		if err := common.Unmarshal([]byte(tc.body), &resp); err != nil {
			t.Fatalf("%s: unmarshal failed: %v", tc.name, err)
		}
		if got := resp.errorMessage(); got != tc.want {
			t.Errorf("%s: errorMessage() = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// TestParseTaskResult 验证上游状态到内部状态的映射
func TestParseTaskResult(t *testing.T) {
	cases := []struct {
		name         string
		body         string
		wantStatus   string
		wantURL      string
		wantReason   string
		wantProgress string
	}{
		{
			name:         "queued",
			body:         `{"id":"task_1","status":"queued","progress":0}`,
			wantStatus:   string(model.TaskStatusQueued),
			wantProgress: taskcommon.ProgressQueued,
		},
		{
			name:         "in_progress",
			body:         `{"id":"task_1","status":"in_progress","progress":50}`,
			wantStatus:   string(model.TaskStatusInProgress),
			wantProgress: taskcommon.ProgressInProgress,
		},
		{
			name:         "completed",
			body:         `{"id":"task_1","status":"completed","url":"https://cdn.example.com/a.png"}`,
			wantStatus:   string(model.TaskStatusSuccess),
			wantURL:      "https://cdn.example.com/a.png",
			wantProgress: taskcommon.ProgressComplete,
		},
		{
			name:         "failed",
			body:         `{"id":"task_1","status":"failed","error":{"message":"boom"}}`,
			wantStatus:   string(model.TaskStatusFailure),
			wantReason:   "boom",
			wantProgress: taskcommon.ProgressComplete,
		},
		{
			name:         "未知状态保持进行中",
			body:         `{"id":"task_1","status":"something_new"}`,
			wantStatus:   string(model.TaskStatusInProgress),
			wantProgress: taskcommon.ProgressInProgress,
		},
	}

	adaptor := &TaskAdaptor{}
	for _, tc := range cases {
		info, err := adaptor.ParseTaskResult([]byte(tc.body))
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.name, err)
		}
		if info.Status != tc.wantStatus {
			t.Errorf("%s: Status = %q, want %q", tc.name, info.Status, tc.wantStatus)
		}
		if info.Url != tc.wantURL {
			t.Errorf("%s: Url = %q, want %q", tc.name, info.Url, tc.wantURL)
		}
		if info.Progress != tc.wantProgress {
			t.Errorf("%s: Progress = %q, want %q", tc.name, info.Progress, tc.wantProgress)
		}
		if tc.wantReason != "" && info.Reason != tc.wantReason {
			t.Errorf("%s: Reason = %q, want %q", tc.name, info.Reason, tc.wantReason)
		}
	}
}

// TestParseTaskResultFailedWithoutMessage 验证失败但无错误信息时给出兜底原因
func TestParseTaskResultFailedWithoutMessage(t *testing.T) {
	adaptor := &TaskAdaptor{}
	info, err := adaptor.ParseTaskResult([]byte(`{"id":"task_1","status":"failed"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Reason == "" {
		t.Error("Reason should not be empty for failed task")
	}
}

// TestGetModelListAndChannelName 验证模型列表与渠道名
func TestGetModelListAndChannelName(t *testing.T) {
	adaptor := &TaskAdaptor{}
	if got := adaptor.GetChannelName(); got != ChannelName {
		t.Errorf("GetChannelName() = %q, want %q", got, ChannelName)
	}
	models := adaptor.GetModelList()
	if len(models) != len(ModelList) {
		t.Errorf("GetModelList() len = %d, want %d", len(models), len(ModelList))
	}
}
