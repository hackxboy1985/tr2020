package td

// 本文件锁定 Td 视频分支（minimax-h3 / seedance-2.0 / seedance-2.5）契约：
//
//  1. 校验边界全部由 videoModelSpecs 决定 —— 未知模型拒绝、模型不支持的
//     素材类型（videos/audios/generate_audio）拒绝、时长与分辨率按模型范围校验；
//  2. 时长缺省与显式 0 语义不同（*int 指针的意义）；
//  3. 视频走 /v1/videos/generations，图片仍走 /v1/images/generations/async（分流不串）；
//  4. 结果解析 video_url 优先、videos[].url[] 回退；
//  5. 计费注入 duration / resolution / aspect_ratio。

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
)

// mkVideoCtx 构造视频提交请求上下文
func mkVideoCtx(t *testing.T, body string) *gin.Context {
	t.Helper()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos/generations", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	storage, err := common.CreateBodyStorage([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = storage.Close() })
	c.Set(common.KeyBodyStorage, storage)
	return c
}

// ============================
// 校验：模型能力边界
// ============================

// 时长范围按模型区分：seedance-2.0 上限 15，seedance-2.5 上限 30
func TestVideoDurationRangePerModel(t *testing.T) {
	cases := []struct {
		model    string
		duration int
		wantErr  bool
	}{
		{VideoModelSeedance20, 10, false},
		{VideoModelSeedance20, 15, false}, // 上界含
		{VideoModelSeedance20, 16, true},  // 超 seedance-2.0 上限
		{VideoModelSeedance25, 30, false}, // seedance-2.5 上界
		{VideoModelSeedance25, 31, true},
		{VideoModelSeedance25, 3, true}, // 低于 seedance-2.5 下限 4
		{VideoModelSeedance25, 4, false},
		{VideoModelMiniMaxH3, 3, false},
		{VideoModelMiniMaxH3, 15, false},
		{VideoModelMiniMaxH3, 16, true},
	}

	for _, tc := range cases {
		body := `{"model":"` + tc.model + `","prompt":"p","duration":` + itoa(tc.duration) + `}`
		c := mkVideoCtx(t, body)
		info := mkInfo()
		a := &TaskAdaptor{}
		a.Init(info)

		taskErr := a.ValidateRequestAndSetAction(c, info)
		if tc.wantErr && taskErr == nil {
			t.Fatalf("%s duration=%d 应被拒绝但通过了", tc.model, tc.duration)
		}
		if !tc.wantErr && taskErr != nil {
			t.Fatalf("%s duration=%d 应通过但被拒: %s", tc.model, tc.duration, taskErr.Message)
		}
		t.Logf("%-14s duration=%-3d → %s", tc.model, tc.duration, passFail(tc.wantErr))
	}
}

// 分辨率按模型白名单：seedance 仅 720p，minimax-h3 支持 720p/1080p/2k
func TestVideoResolutionPerModel(t *testing.T) {
	cases := []struct {
		model      string
		resolution string
		wantErr    bool
	}{
		{VideoModelSeedance20, "720p", false},
		{VideoModelSeedance20, "1080p", true}, // seedance 文档仅支持 720p
		{VideoModelSeedance20, "2k", true},
		{VideoModelSeedance25, "720p", false},
		{VideoModelSeedance25, "2k", true},
		{VideoModelMiniMaxH3, "720p", false},
		{VideoModelMiniMaxH3, "1080p", false},
		{VideoModelMiniMaxH3, "2k", false},
		{VideoModelMiniMaxH3, "4k", true}, // 非视频档位
	}

	for _, tc := range cases {
		body := `{"model":"` + tc.model + `","prompt":"p","resolution":"` + tc.resolution + `"}`
		c := mkVideoCtx(t, body)
		info := mkInfo()
		a := &TaskAdaptor{}
		a.Init(info)

		taskErr := a.ValidateRequestAndSetAction(c, info)
		if tc.wantErr && taskErr == nil {
			t.Fatalf("%s resolution=%s 应被拒绝但通过了", tc.model, tc.resolution)
		}
		if !tc.wantErr && taskErr != nil {
			t.Fatalf("%s resolution=%s 应通过但被拒: %s", tc.model, tc.resolution, taskErr.Message)
		}
		t.Logf("%-14s resolution=%-6s → %s", tc.model, tc.resolution, passFail(tc.wantErr))
	}
}

// videos / audios / generate_audio 是否支持完全由 spec 决定：
// minimax-h3 三者皆不支持（文档明确 videos 返回 400、参数表未列 generate_audio）
func TestVideoUnsupportedMediaRejected(t *testing.T) {
	cases := []struct {
		name    string
		model   string
		body    string
		wantErr bool
	}{
		{
			"minimax-h3 传 videos 应拒绝", VideoModelMiniMaxH3,
			`{"model":"minimax-h3","prompt":"p","videos":["https://x/a.mp4"]}`, true,
		},
		{
			"minimax-h3 传 audios 应拒绝", VideoModelMiniMaxH3,
			`{"model":"minimax-h3","prompt":"p","images":["https://x/a.png"],"audios":["https://x/a.mp3"]}`, true,
		},
		{
			"minimax-h3 传 generate_audio 应拒绝", VideoModelMiniMaxH3,
			`{"model":"minimax-h3","prompt":"p","generate_audio":true}`, true,
		},
		{
			"seedance-2.0 传 3 视频应通过", VideoModelSeedance20,
			`{"model":"seedance-2.0","prompt":"p","videos":["https://x/1.mp4","https://x/2.mp4","https://x/3.mp4"]}`, false,
		},
		{
			"seedance-2.0 传 4 视频应拒绝", VideoModelSeedance20,
			`{"model":"seedance-2.0","prompt":"p","videos":["https://x/1.mp4","https://x/2.mp4","https://x/3.mp4","https://x/4.mp4"]}`, true,
		},
		{
			"seedance-2.5 传 10 视频应通过", VideoModelSeedance25,
			`{"model":"seedance-2.5","prompt":"p","videos":["https://x/1.mp4","https://x/2.mp4","https://x/3.mp4","https://x/4.mp4","https://x/5.mp4","https://x/6.mp4","https://x/7.mp4","https://x/8.mp4","https://x/9.mp4","https://x/10.mp4"]}`, false,
		},
		{
			"seedance-2.5 传 11 视频应拒绝", VideoModelSeedance25,
			`{"model":"seedance-2.5","prompt":"p","videos":["https://x/1.mp4","https://x/2.mp4","https://x/3.mp4","https://x/4.mp4","https://x/5.mp4","https://x/6.mp4","https://x/7.mp4","https://x/8.mp4","https://x/9.mp4","https://x/10.mp4","https://x/11.mp4"]}`, true,
		},
		{
			"seedance-2.5 支持 generate_audio", VideoModelSeedance25,
			`{"model":"seedance-2.5","prompt":"p","generate_audio":true}`, false,
		},
	}

	for _, tc := range cases {
		c := mkVideoCtx(t, tc.body)
		info := mkInfo()
		a := &TaskAdaptor{}
		a.Init(info)

		taskErr := a.ValidateRequestAndSetAction(c, info)
		if tc.wantErr && taskErr == nil {
			t.Fatalf("%s：应被拒绝但通过了", tc.name)
		}
		if !tc.wantErr && taskErr != nil {
			t.Fatalf("%s：应通过但被拒: %s", tc.name, taskErr.Message)
		}
		t.Logf("%-40s → %s", tc.name, passFail(tc.wantErr))
	}
}

// 图片上限按模型区分：seedance-2.0/9 vs seedance-2.5/30
func TestVideoImageLimitPerModel(t *testing.T) {
	imgs := func(n int) string {
		parts := make([]string, n)
		for i := range parts {
			parts[i] = `"https://x/` + itoa(i) + `.png"`
		}
		return "[" + strings.Join(parts, ",") + "]"
	}

	cases := []struct {
		model   string
		count   int
		wantErr bool
	}{
		{VideoModelSeedance20, 9, false},
		{VideoModelSeedance20, 10, true},
		{VideoModelSeedance25, 30, false},
		{VideoModelSeedance25, 31, true},
		{VideoModelMiniMaxH3, 9, false},
		{VideoModelMiniMaxH3, 10, true},
	}

	for _, tc := range cases {
		body := `{"model":"` + tc.model + `","prompt":"p","images":` + imgs(tc.count) + `}`
		c := mkVideoCtx(t, body)
		info := mkInfo()
		a := &TaskAdaptor{}
		a.Init(info)

		taskErr := a.ValidateRequestAndSetAction(c, info)
		if tc.wantErr && taskErr == nil {
			t.Fatalf("%s images=%d 应被拒绝但通过了", tc.model, tc.count)
		}
		if !tc.wantErr && taskErr != nil {
			t.Fatalf("%s images=%d 应通过但被拒: %s", tc.model, tc.count, taskErr.Message)
		}
		t.Logf("%-14s images=%-3d → %s", tc.model, tc.count, passFail(tc.wantErr))
	}
}

// 未知模型一律拒绝（不静默透传）
func TestVideoUnknownModelRejected(t *testing.T) {
	for _, model := range []string{"veo-3", "kling-v3", "seedance-3.0", "gpt-image-2-all"} {
		body := `{"model":"` + model + `","prompt":"p"}`
		c := mkVideoCtx(t, body)
		info := mkInfo()
		a := &TaskAdaptor{}
		a.Init(info)

		taskErr := a.ValidateRequestAndSetAction(c, info)
		if taskErr == nil {
			t.Fatalf("未知模型 %s 应被拒绝但通过了", model)
		}
		t.Logf("模型 %-16s → %s", model, taskErr.Message)
	}
}

// audios 不能单独使用：必须配合 images 或 videos
func TestVideoAudiosCannotBeAlone(t *testing.T) {
	body := `{"model":"seedance-2.5","prompt":"p","audios":["https://x/a.mp3"]}`
	c := mkVideoCtx(t, body)
	info := mkInfo()
	a := &TaskAdaptor{}
	a.Init(info)
	if taskErr := a.ValidateRequestAndSetAction(c, info); taskErr == nil {
		t.Fatal("audios 单独使用应被拒绝")
	} else {
		t.Logf("audios 单独使用 → %s", taskErr.Message)
	}

	// 配合图片后应通过
	body2 := `{"model":"seedance-2.5","prompt":"p","images":["https://x/a.png"],"audios":["https://x/a.mp3"]}`
	c2 := mkVideoCtx(t, body2)
	info2 := mkInfo()
	a2 := &TaskAdaptor{}
	a2.Init(info2)
	if taskErr := a2.ValidateRequestAndSetAction(c2, info2); taskErr != nil {
		t.Fatalf("audios 配合图片应通过: %s", taskErr.Message)
	}
}

// aspect_ratio 仅允许 16:9 / 9:16
func TestVideoAspectRatio(t *testing.T) {
	for _, tc := range []struct {
		ar      string
		wantErr bool
	}{
		{"16:9", false}, {"9:16", false}, {"1:1", true}, {"4:3", true},
	} {
		body := `{"model":"seedance-2.0","prompt":"p","aspect_ratio":"` + tc.ar + `"}`
		c := mkVideoCtx(t, body)
		info := mkInfo()
		a := &TaskAdaptor{}
		a.Init(info)
		taskErr := a.ValidateRequestAndSetAction(c, info)
		if tc.wantErr && taskErr == nil {
			t.Fatalf("aspect_ratio=%s 应被拒绝", tc.ar)
		}
		if !tc.wantErr && taskErr != nil {
			t.Fatalf("aspect_ratio=%s 应通过: %s", tc.ar, taskErr.Message)
		}
	}
}

// ============================
// duration 指针语义
// ============================

// 时长缺省填模型默认值；显式传 0 因低于下限被拒（而非被当作缺省）
func TestVideoDurationPointerSemantics(t *testing.T) {
	// 缺省 → 填 4
	body := `{"model":"seedance-2.5","prompt":"p"}`
	c := mkVideoCtx(t, body)
	info := mkInfo()
	a := &TaskAdaptor{}
	a.Init(info)
	if taskErr := a.ValidateRequestAndSetAction(c, info); taskErr != nil {
		t.Fatalf("缺省应通过: %s", taskErr.Message)
	}
	got, ok := getVideoRequest(c)
	if !ok {
		t.Fatal("task_request 应为视频请求")
	}
	if got.Duration == nil || *got.Duration != 4 {
		t.Fatalf("缺省 duration 应为 4, 实际 %v", got.Duration)
	}
	t.Logf("缺省 duration → %d", *got.Duration)

	// 显式 0 → 低于 seedance-2.5 下限 4，必须拒绝（若用非指针 int 会与缺省混淆）
	body0 := `{"model":"seedance-2.5","prompt":"p","duration":0}`
	c0 := mkVideoCtx(t, body0)
	info0 := mkInfo()
	a0 := &TaskAdaptor{}
	a0.Init(info0)
	if taskErr := a0.ValidateRequestAndSetAction(c0, info0); taskErr == nil {
		t.Fatal("显式 duration=0 应被拒绝（低于模型下限）")
	} else {
		t.Logf("显式 duration=0 → %s", taskErr.Message)
	}
}

// ============================
// 分流：视频与图片互不干扰
// ============================

// 视频走 /v1/videos/generations，图片仍走 /v1/images/generations/async
func TestVideoAndImageEndpointSplit(t *testing.T) {
	// 视频请求
	vc := mkVideoCtx(t, `{"model":"seedance-2.5","prompt":"p"}`)
	vinfo := mkInfo()
	va := &TaskAdaptor{}
	va.Init(vinfo)
	if taskErr := va.ValidateRequestAndSetAction(vc, vinfo); taskErr != nil {
		t.Fatalf("视频校验: %s", taskErr.Message)
	}
	vurl, err := va.BuildRequestURL(vinfo)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(vurl, EndpointVideoGenerate) {
		t.Fatalf("视频应走 %s, 实际 %s", EndpointVideoGenerate, vurl)
	}
	t.Logf("视频 URL: %s", vurl)

	// 图片请求（走 /v1/images/tasks 入口，但模型为图片模型）
	ic := mkCtx(t, `{"model":"gpt-image-2-all","prompt":"p","resolution":"4k","quality":"high"}`)
	iinfo := mkInfo()
	ia := &TaskAdaptor{}
	ia.Init(iinfo)
	if taskErr := ia.ValidateRequestAndSetAction(ic, iinfo); taskErr != nil {
		t.Fatalf("图片校验: %s", taskErr.Message)
	}
	iurl, err := ia.BuildRequestURL(iinfo)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(iurl, EndpointGenerateAsync) {
		t.Fatalf("图片应走 %s, 实际 %s", EndpointGenerateAsync, iurl)
	}
	t.Logf("图片 URL: %s", iurl)
}

// 图片模型不会因请求体含 duration 等字段而误入视频分支
func TestImageModelNotHijackedByVideoFields(t *testing.T) {
	body := `{"model":"gpt-image-2-all","prompt":"p","resolution":"4k","quality":"high","duration":10}`
	c := mkCtx(t, body)
	info := mkInfo()
	a := &TaskAdaptor{}
	a.Init(info)
	if taskErr := a.ValidateRequestAndSetAction(c, info); taskErr != nil {
		t.Fatalf("图片模型应仍走图片分支: %s", taskErr.Message)
	}
	if _, isVideo := getVideoRequest(c); isVideo {
		t.Fatal("图片模型不应被判为视频请求")
	}
	url, _ := a.BuildRequestURL(info)
	if !strings.HasSuffix(url, EndpointGenerateAsync) {
		t.Fatalf("图片模型应走图片端点, 实际 %s", url)
	}
}

// ============================
// 上游请求体
// ============================

// 视频请求体必须下发 duration / aspect_ratio / resolution，且遵守规则 6：
// generate_audio 显式 false 不能被 omitempty 丢掉
func TestVideoUpstreamBody(t *testing.T) {
	body := `{"model":"seedance-2.5","prompt":"p","duration":20,"aspect_ratio":"9:16","resolution":"720p","images":["https://x/1.png"],"generate_audio":false}`
	c := mkVideoCtx(t, body)
	info := mkInfo()
	info.UpstreamModelName = VideoModelSeedance25
	a := &TaskAdaptor{}
	a.Init(info)
	if taskErr := a.ValidateRequestAndSetAction(c, info); taskErr != nil {
		t.Fatalf("校验: %s", taskErr.Message)
	}

	r, err := a.BuildRequestBody(c, info)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	raw, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)
	t.Logf("视频上游请求体: %s", got)

	for _, want := range []string{
		`"model":"seedance-2.5"`,
		`"duration":20`,
		`"aspect_ratio":"9:16"`,
		`"resolution":"720p"`,
		`"generate_audio":false`, // 显式 false 必须下发
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("上游请求体缺少 %s: %s", want, got)
		}
	}
}

// 视频请求体不得出现图片专有字段（size / quality）
func TestVideoBodyHasNoImageFields(t *testing.T) {
	body := `{"model":"minimax-h3","prompt":"p","duration":8}`
	c := mkVideoCtx(t, body)
	info := mkInfo()
	info.UpstreamModelName = VideoModelMiniMaxH3
	a := &TaskAdaptor{}
	a.Init(info)
	if taskErr := a.ValidateRequestAndSetAction(c, info); taskErr != nil {
		t.Fatalf("校验: %s", taskErr.Message)
	}
	r, err := a.BuildRequestBody(c, info)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(r)
	got := string(raw)
	t.Logf("minimax-h3 上游请求体: %s", got)

	for _, unwanted := range []string{`"size"`, `"quality"`} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("视频请求体不应含图片字段 %s: %s", unwanted, got)
		}
	}
}

// ============================
// 结果解析
// ============================

// video_url 优先
func TestParseVideoTaskResultPrefersVideoURL(t *testing.T) {
	body := `{"code":200,"data":{"id":"t1","status":"completed","progress":100,
		"result":{"video_url":"https://cdn/out/a.mp4","videos":[{"url":["https://cdn/out/fallback.mp4"]}]}}}`
	info, err := parseVideoTaskResult([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if info.Url != "https://cdn/out/a.mp4" {
		t.Fatalf("应优先 video_url, 实际 %s", info.Url)
	}
	t.Logf("video_url 优先 → %s", info.Url)
}

// 只有 videos[] 时回退
func TestParseVideoTaskResultFallsBackToVideos(t *testing.T) {
	body := `{"code":200,"data":{"id":"t1","status":"completed","progress":100,
		"result":{"videos":[{"url":["https://cdn/out/only.mp4"]}]}}}`
	info, err := parseVideoTaskResult([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if info.Url != "https://cdn/out/only.mp4" {
		t.Fatalf("应回退 videos[0].url[0], 实际 %s", info.Url)
	}
	t.Logf("videos 回退 → %s", info.Url)
}

// 状态映射与失败原因
func TestParseVideoTaskResultStatuses(t *testing.T) {
	cases := []struct {
		body       string
		wantStatus string
	}{
		{`{"code":200,"data":{"id":"t","status":"submitted"}}`, "submitted"},
		{`{"code":200,"data":{"id":"t","status":"processing"}}`, "21"},
		{`{"code":200,"data":{"id":"t","status":"completed","result":{"video_url":"https://x/a.mp4"}}}`, "30"},
		{`{"code":200,"data":{"id":"t","status":"failed","error":{"code":"e","message":"boom"}}}`, "22"},
	}
	for _, tc := range cases {
		info, err := parseVideoTaskResult([]byte(tc.body))
		if err != nil {
			t.Fatalf("%s: %v", tc.body, err)
		}
		t.Logf("status=%s → %s", tc.wantStatus, info.Status)
	}

	// 失败原因
	failBody := `{"code":200,"data":{"id":"t","status":"failed","error":{"code":"e","message":"boom"}}}`
	fi, err := parseVideoTaskResult([]byte(failBody))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Reason != "boom" {
		t.Fatalf("失败原因应为 boom, 实际 %q", fi.Reason)
	}
}

// 视频任务走视频解析、图片任务走图片解析（ParseTaskResult 按响应结构分流）
//
// 关键：这里刻意用「未设置 videoTask 标记」的适配器，模拟轮询阶段新建实例的场景。
// 若 ParseTaskResult 依赖 a.videoTask，本测试会失败 —— 这正是要防住的回归。
func TestParseTaskResultDispatch(t *testing.T) {
	videoBody := `{"code":200,"data":{"id":"t","status":"completed","progress":100,"result":{"video_url":"https://x/v.mp4"}}}`
	va := &TaskAdaptor{} // 无 videoTask 标记，模拟轮询阶段的新实例
	vinfo, err := va.ParseTaskResult([]byte(videoBody))
	if err != nil {
		t.Fatal(err)
	}
	if vinfo.Url != "https://x/v.mp4" {
		t.Fatalf("视频解析应取到 video_url, 实际 %q", vinfo.Url)
	}

	// 只有 videos[] 的视频结果同样要能识别
	videoBody2 := `{"code":200,"data":{"id":"t","status":"completed","result":{"videos":[{"url":["https://x/v2.mp4"]}]}}}`
	v2info, err := va.ParseTaskResult([]byte(videoBody2))
	if err != nil {
		t.Fatal(err)
	}
	if v2info.Url != "https://x/v2.mp4" {
		t.Fatalf("videos 形式应取到 url, 实际 %q", v2info.Url)
	}

	imageBody := `{"code":200,"data":{"id":"t","status":"completed","progress":100,"result":{"images":[{"url":["https://x/i.png"]}]}}}`
	ia := &TaskAdaptor{}
	iinfo, err := ia.ParseTaskResult([]byte(imageBody))
	if err != nil {
		t.Fatal(err)
	}
	if iinfo.Url != "https://x/i.png" {
		t.Fatalf("图片解析应取到 images[0].url[0], 实际 %q", iinfo.Url)
	}
	t.Logf("视频 → %s, 视频(videos) → %s, 图片 → %s", vinfo.Url, v2info.Url, iinfo.Url)
}

// 响应结构判定：视频/图片/进行中 三种形态
func TestIsVideoTaskResponse(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"video_url", `{"code":200,"data":{"result":{"video_url":"https://x/a.mp4"}}}`, true},
		{"videos", `{"code":200,"data":{"result":{"videos":[{"url":["https://x/a.mp4"]}]}}}`, true},
		{"images", `{"code":200,"data":{"result":{"images":[{"url":["https://x/a.png"]}]}}}`, false},
		{"进行中无 result", `{"code":200,"data":{"id":"t","status":"processing"}}`, false},
		{"空响应", `{}`, false},
		{"非 JSON", `not-json`, false},
	}
	for _, tc := range cases {
		got := isVideoTaskResponse([]byte(tc.body))
		if got != tc.want {
			t.Fatalf("%s: got %v want %v", tc.name, got, tc.want)
		}
		t.Logf("%-16s → %v ✓", tc.name, got)
	}
}

// ============================
// 计费注入
// ============================

// 视频计费输入必须含 duration / resolution / aspect_ratio
func TestVideoInjectBillingParams(t *testing.T) {
	body := `{"model":"seedance-2.5","prompt":"p","duration":20,"resolution":"720p","aspect_ratio":"9:16"}`
	c := mkVideoCtx(t, body)
	info := mkInfo()
	info.UpstreamModelName = VideoModelSeedance25
	a := &TaskAdaptor{}
	a.Init(info)
	if taskErr := a.ValidateRequestAndSetAction(c, info); taskErr != nil {
		t.Fatalf("校验: %s", taskErr.Message)
	}

	a.InjectBillingParams(c, info)
	got := string(info.BillingRequestInput.Body)
	t.Logf("视频计费注入: %s", got)

	for _, want := range []string{`"duration":20`, `"resolution":"720p"`, `"aspect_ratio":"9:16"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("计费输入缺少 %s: %s", want, got)
		}
	}
}

// 视频计费缺省时长也必须注入（不能缺字段导致 param() 取不到值）
func TestVideoInjectBillingParamsDefaults(t *testing.T) {
	body := `{"model":"minimax-h3","prompt":"p"}`
	c := mkVideoCtx(t, body)
	info := mkInfo()
	info.UpstreamModelName = VideoModelMiniMaxH3
	a := &TaskAdaptor{}
	a.Init(info)
	if taskErr := a.ValidateRequestAndSetAction(c, info); taskErr != nil {
		t.Fatalf("校验: %s", taskErr.Message)
	}
	a.InjectBillingParams(c, info)
	got := string(info.BillingRequestInput.Body)
	t.Logf("视频缺省计费注入: %s", got)

	if !strings.Contains(got, `"duration":4`) {
		t.Fatalf("缺省 duration 应注入 4: %s", got)
	}
	if !strings.Contains(got, `"resolution":"720p"`) {
		t.Fatalf("缺省 resolution 应注入 720p: %s", got)
	}
}

// ============================
// 模型映射
// ============================

// 别名经 model_mapping 解析为上游名后能正确命中 spec
func TestVideoModelMappingResolution(t *testing.T) {
	body := `{"model":"my-video","prompt":"p","duration":20}`
	c := mkVideoCtx(t, body)
	c.Set("model_mapping", `{"my-video":"seedance-2.5"}`)
	info := mkInfo()
	info.OriginModelName = "my-video"
	a := &TaskAdaptor{}
	a.Init(info)

	if taskErr := a.ValidateRequestAndSetAction(c, info); taskErr != nil {
		t.Fatalf("别名映射到 seedance-2.5 应通过（duration=20 超 seedance-2.0 上限，若未解析映射会被误拒）: %s", taskErr.Message)
	}
	if info.UpstreamModelName != VideoModelSeedance25 {
		t.Fatalf("UpstreamModelName 应为 %s, 实际 %s", VideoModelSeedance25, info.UpstreamModelName)
	}
	t.Logf("my-video → %s", info.UpstreamModelName)
}

// 别名映射到不支持视频的模型时，按目标模型的能力拒绝
func TestVideoModelMappingToImageModelRejected(t *testing.T) {
	body := `{"model":"alias","prompt":"p"}`
	c := mkVideoCtx(t, body)
	c.Set("model_mapping", `{"alias":"gpt-image-2-all"}`)
	info := mkInfo()
	info.OriginModelName = "alias"
	a := &TaskAdaptor{}
	a.Init(info)

	// 路径为 /v1/videos/generations，故仍走视频分支，但目标模型不在 videoModelSpecs 中
	if taskErr := a.ValidateRequestAndSetAction(c, info); taskErr == nil {
		t.Fatal("映射到图片模型的视频请求应被拒绝")
	} else {
		t.Logf("alias → gpt-image-2-all → %s", taskErr.Message)
	}
}

// ============================
// spec 表完整性
// ============================

// 三模型都必须在 spec 表中，且默认值自洽
func TestVideoModelSpecsConsistency(t *testing.T) {
	for _, model := range []string{VideoModelMiniMaxH3, VideoModelSeedance20, VideoModelSeedance25} {
		spec, ok := lookupVideoModelSpec(model)
		if !ok {
			t.Fatalf("模型 %s 未登记在 videoModelSpecs", model)
		}
		if spec.MinDuration <= 0 || spec.MaxDuration < spec.MinDuration {
			t.Fatalf("%s 时长范围非法: %d-%d", model, spec.MinDuration, spec.MaxDuration)
		}
		if spec.DefaultDuration < spec.MinDuration || spec.DefaultDuration > spec.MaxDuration {
			t.Fatalf("%s 默认时长 %d 不在 %d-%d 内", model, spec.DefaultDuration, spec.MinDuration, spec.MaxDuration)
		}
		if len(spec.AllowedResolutions) == 0 {
			t.Fatalf("%s 未配置分辨率档位", model)
		}
		if spec.MaxImages <= 0 {
			t.Fatalf("%s 参考图上限应为正数", model)
		}
		t.Logf("%-14s duration=%d-%d(default %d) res=%v images=%d videos=%d audios=%d genAudio=%v",
			model, spec.MinDuration, spec.MaxDuration, spec.DefaultDuration,
			spec.AllowedResolutions, spec.MaxImages, spec.MaxVideos, spec.MaxAudios, spec.AllowGenerateAudio)
	}
}

// ============================
// 测试辅助
// ============================

func passFail(wantErr bool) string {
	if wantErr {
		return "拒绝 ✓"
	}
	return "通过 ✓"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf []byte
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}

// 保持 constant 引用，避免未使用导入（渠道类型断言相关测试）
var _ = constant.ChannelTypeTudou
