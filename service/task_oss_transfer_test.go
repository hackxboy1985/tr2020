package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

// setOssConfigured 临时把对象存储配置设为「完整」或「不完整」。
func setOssConfigured(t *testing.T, configured bool) {
	t.Helper()
	origEndpoint := system_setting.ObjectStorageEndpoint
	origBucket := system_setting.ObjectStorageBucket
	origAK := system_setting.ObjectStorageAccessKeyId
	origSK := system_setting.ObjectStorageAccessKeySecret
	t.Cleanup(func() {
		system_setting.ObjectStorageEndpoint = origEndpoint
		system_setting.ObjectStorageBucket = origBucket
		system_setting.ObjectStorageAccessKeyId = origAK
		system_setting.ObjectStorageAccessKeySecret = origSK
	})

	if configured {
		system_setting.ObjectStorageEndpoint = "http://127.0.0.1:9000"
		system_setting.ObjectStorageBucket = "td-results"
		system_setting.ObjectStorageAccessKeyId = "ak"
		system_setting.ObjectStorageAccessKeySecret = "sk"
		return
	}
	system_setting.ObjectStorageEndpoint = ""
	system_setting.ObjectStorageBucket = ""
	system_setting.ObjectStorageAccessKeyId = ""
	system_setting.ObjectStorageAccessKeySecret = ""
}

// mkChannel 构造一个带指定开关设置的渠道。
func mkChannel(channelType int, settings dto.ChannelOtherSettings) *model.Channel {
	ch := &model.Channel{Id: 1}
	ch.Type = channelType
	ch.SetOtherSettings(settings)
	return ch
}

func TestShouldTransferToOss(t *testing.T) {
	const srcURL = "https://upstream.example.com/video.mp4"

	tests := []struct {
		name         string
		configured   bool
		channelType  int
		settings     dto.ChannelOtherSettings
		task         *model.Task
		url          string
		isVideo      bool
		wantTransfer bool
	}{
		{
			name: "视频开关开_应转存", configured: true,
			channelType: constant.ChannelTypeTudou,
			settings:    dto.ChannelOtherSettings{TdOssVideoEnabled: true},
			task:        &model.Task{TaskID: "task_1"},
			url:         srcURL, isVideo: true,
			wantTransfer: true,
		},
		{
			name: "图片开关开_视频不转存", configured: true,
			channelType: constant.ChannelTypeTudou,
			settings:    dto.ChannelOtherSettings{TdOssImageEnabled: true},
			task:        &model.Task{TaskID: "task_1"},
			url:         srcURL, isVideo: true,
			wantTransfer: false,
		},
		{
			name: "视频开关开_图片不转存", configured: true,
			channelType: constant.ChannelTypeTudou,
			settings:    dto.ChannelOtherSettings{TdOssVideoEnabled: true},
			task:        &model.Task{TaskID: "task_1"},
			url:         srcURL, isVideo: false,
			wantTransfer: false,
		},
		{
			name: "两个开关都开_视频转存", configured: true,
			channelType: constant.ChannelTypeTudou,
			settings: dto.ChannelOtherSettings{
				TdOssImageEnabled: true, TdOssVideoEnabled: true,
			},
			task: &model.Task{TaskID: "task_1"},
			url:  srcURL, isVideo: true,
			wantTransfer: true,
		},
		{
			name: "开关全关_不转存", configured: true,
			channelType: constant.ChannelTypeTudou,
			settings:    dto.ChannelOtherSettings{},
			task:        &model.Task{TaskID: "task_1"},
			url:         srcURL, isVideo: true,
			wantTransfer: false,
		},
		{
			name: "对象存储未配置_不转存", configured: false,
			channelType: constant.ChannelTypeTudou,
			settings:    dto.ChannelOtherSettings{TdOssVideoEnabled: true},
			task:        &model.Task{TaskID: "task_1"},
			url:         srcURL, isVideo: true,
			wantTransfer: false,
		},
		{
			name: "非Td渠道_不转存", configured: true,
			channelType: constant.ChannelTypeRR,
			settings:    dto.ChannelOtherSettings{TdOssVideoEnabled: true},
			task:        &model.Task{TaskID: "task_1"},
			url:         srcURL, isVideo: true,
			wantTransfer: false,
		},
		{
			name: "已转存过_幂等不重复", configured: true,
			channelType: constant.ChannelTypeTudou,
			settings:    dto.ChannelOtherSettings{TdOssVideoEnabled: true},
			task: &model.Task{TaskID: "task_1", PrivateData: model.TaskPrivateData{
				OssURL: "https://oss.example.com/already.mp4",
			}},
			url: srcURL, isVideo: true,
			wantTransfer: false,
		},
		{
			name: "已达重试上限_不转存", configured: true,
			channelType: constant.ChannelTypeTudou,
			settings:    dto.ChannelOtherSettings{TdOssVideoEnabled: true},
			task: &model.Task{TaskID: "task_1", PrivateData: model.TaskPrivateData{
				OssTransferAttempts: ossTransferMaxAttempts,
			}},
			url: srcURL, isVideo: true,
			wantTransfer: false,
		},
		{
			name: "上游URL已过期_不转存", configured: true,
			channelType: constant.ChannelTypeTudou,
			settings:    dto.ChannelOtherSettings{TdOssVideoEnabled: true},
			task: &model.Task{TaskID: "task_1", PrivateData: model.TaskPrivateData{
				ExpireAt: time.Now().Add(-1 * time.Hour).Unix(),
			}},
			url: srcURL, isVideo: true,
			wantTransfer: false,
		},
		{
			name: "上游URL未过期_可转存", configured: true,
			channelType: constant.ChannelTypeTudou,
			settings:    dto.ChannelOtherSettings{TdOssVideoEnabled: true},
			task: &model.Task{TaskID: "task_1", PrivateData: model.TaskPrivateData{
				ExpireAt: time.Now().Add(1 * time.Hour).Unix(),
			}},
			url: srcURL, isVideo: true,
			wantTransfer: true,
		},
		{
			name: "空URL_不转存", configured: true,
			channelType: constant.ChannelTypeTudou,
			settings:    dto.ChannelOtherSettings{TdOssVideoEnabled: true},
			task:        &model.Task{TaskID: "task_1"},
			url:         "", isVideo: true,
			wantTransfer: false,
		},
		{
			name: "data前缀_不转存", configured: true,
			channelType: constant.ChannelTypeTudou,
			settings:    dto.ChannelOtherSettings{TdOssVideoEnabled: true},
			task:        &model.Task{TaskID: "task_1"},
			url:         "data:video/mp4;base64,AAAA", isVideo: true,
			wantTransfer: false,
		},
		{
			name: "非http协议_不转存", configured: true,
			channelType: constant.ChannelTypeTudou,
			settings:    dto.ChannelOtherSettings{TdOssVideoEnabled: true},
			task:        &model.Task{TaskID: "task_1"},
			url:         "ftp://example.com/a.mp4", isVideo: true,
			wantTransfer: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setOssConfigured(t, tt.configured)
			ch := mkChannel(tt.channelType, tt.settings)

			got := ShouldTransferToOss(ch, tt.task, tt.url, tt.isVideo)
			if got != tt.wantTransfer {
				t.Errorf("ShouldTransferToOss() = %v, want %v", got, tt.wantTransfer)
			}
		})
	}
}

func TestShouldTransferToOss_NilSafety(t *testing.T) {
	setOssConfigured(t, true)

	if ShouldTransferToOss(nil, &model.Task{}, "https://a.com/v.mp4", true) {
		t.Error("nil channel 应返回 false")
	}
	if ShouldTransferToOss(mkChannel(constant.ChannelTypeTudou, dto.ChannelOtherSettings{}), nil, "https://a.com/v.mp4", true) {
		t.Error("nil task 应返回 false")
	}
}

func TestClassifyOssFailStage(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"nil按上传处理", nil, model.OssFailStageUpload},
		{"下载失败", errStr("download source failed: status=404"), model.OssFailStageDownload},
		{"SSRF拒绝", errStr("request reject: blocked"), model.OssFailStageDownload},
		{"构造请求失败", errStr("build request failed: bad"), model.OssFailStageDownload},
		{"上传失败", errStr("put object failed: access denied"), model.OssFailStageUpload},
		{"其他错误归上传", errStr("some unknown error"), model.OssFailStageUpload},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyOssFailStage(tt.err); got != tt.want {
				t.Errorf("classifyOssFailStage(%v) = %q, want %q", tt.err, got, tt.want)
			}
		})
	}
}

func TestLooksLikeVideoURL(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"https://a.com/v.mp4", true},
		{"https://a.com/v.MP4", true},
		{"https://a.com/v.mp4?auth=xxx", true},
		{"https://a.com/v.mov", true},
		{"https://a.com/v.webm", true},
		{"https://a.com/v.png", false},
		{"https://a.com/v.jpg", false},
		{"https://a.com/download", false},
		{"https://a.com/x?a=.mp4", false},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			if got := looksLikeVideoURL(tt.url); got != tt.want {
				t.Errorf("looksLikeVideoURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestMediaTypeIsVideo(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{
			name: "video_url 字段",
			body: `{"code":200,"data":{"status":"completed","result":{"video_url":"https://a.com/v.mp4"}}}`,
			want: true,
		},
		{
			name: "videos 数组",
			body: `{"code":200,"data":{"status":"completed","result":{"videos":[{"url":["https://a.com/v.mp4"]}]}}}`,
			want: true,
		},
		{
			name: "图片结果",
			body: `{"code":200,"data":{"status":"completed","result":{"images":[{"url":["https://a.com/i.png"]}]}}}`,
			want: false,
		},
		{name: "非法JSON", body: `not json`, want: false},
		{name: "空对象", body: `{}`, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MediaTypeIsVideo([]byte(tt.body)); got != tt.want {
				t.Errorf("MediaTypeIsVideo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAcquireOssTransfer(t *testing.T) {
	release, ok := acquireOssTransfer("task_concurrent")
	if !ok {
		t.Fatal("first attempt should acquire")
	}
	defer release()
	// 模拟补扫在实时上传尚未完成时调用，必须跳过而非启动相同尝试。
	done := make(chan bool, 1)
	go func() {
		otherRelease, acquired := acquireOssTransfer("task_concurrent")
		if acquired {
			otherRelease()
		}
		done <- acquired
	}()
	if <-done {
		t.Fatal("same task acquired concurrently")
	}
	otherRelease, ok := acquireOssTransfer("task_independent")
	if !ok {
		t.Fatal("different task should acquire independently")
	}
	otherRelease()
	release()
	retryRelease, ok := acquireOssTransfer("task_concurrent")
	if !ok {
		t.Fatal("next retry should acquire after previous attempt finishes")
	}
	retryRelease()
}

func TestZyOssChannelIsolation(t *testing.T) {
	setOssConfigured(t, true)
	for _, tt := range []struct {
		name        string
		settings    dto.ChannelOtherSettings
		video, want bool
	}{
		{"default_off", dto.ChannelOtherSettings{}, false, false},
		{"image_on", dto.ChannelOtherSettings{ZyOssImageEnabled: true}, false, true},
		{"image_does_not_enable_video", dto.ChannelOtherSettings{ZyOssImageEnabled: true}, true, false},
		{"video_on", dto.ChannelOtherSettings{ZyOssVideoEnabled: true}, true, true},
		{"video_does_not_enable_image", dto.ChannelOtherSettings{ZyOssVideoEnabled: true}, false, false},
		{"td_switches_ignored", dto.ChannelOtherSettings{TdOssImageEnabled: true, TdOssVideoEnabled: true}, false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ch := mkChannel(constant.ChannelTypeZy, tt.settings)
			if got := ShouldTransferToOss(ch, &model.Task{}, "https://cdn/result", tt.video); got != tt.want {
				t.Fatalf("transfer = %v, want %v", got, tt.want)
			}
		})
	}
	// 同类型的另一个渠道仍然关闭，不能因前一个渠道开启而受影响。
	ch := mkChannel(constant.ChannelTypeZy, dto.ChannelOtherSettings{})
	ch.Id = 2
	if ShouldTransferToOss(ch, &model.Task{}, "https://cdn/result.png", false) {
		t.Fatal("switch leaked across channel IDs")
	}
	td := mkChannel(constant.ChannelTypeTudou, dto.ChannelOtherSettings{ZyOssImageEnabled: true})
	if ShouldTransferToOss(td, &model.Task{}, "https://cdn/result.png", false) {
		t.Fatal("Zy switch enabled Td")
	}
}

func TestZyOssMediaType(t *testing.T) {
	for _, tt := range []struct {
		body, url string
		video     bool
	}{
		{`{"object":"video","image_url":"https://cdn/result.png","video_url":"https://cdn/result.png"}`, "https://cdn/result.png", false},
		{`{"object":"video","model":"gpt-image-2-4K","url":"https://cdn/result","video_url":"https://cdn/result"}`, "https://cdn/result", false},
		{`{"url":"https://cdn/result.mp4","video_url":"https://cdn/result.mp4"}`, "https://cdn/result.mp4", true},
		{"", "https://cdn/result.mp4", true},
	} {
		if got := taskResultIsVideo(constant.ChannelTypeZy, []byte(tt.body), tt.url); got != tt.video {
			t.Fatalf("body=%s: video=%v, want=%v", tt.body, got, tt.video)
		}
	}
}

// errStr 构造一个简单错误，避免引入额外依赖。
type errStr string

func (e errStr) Error() string { return string(e) }
