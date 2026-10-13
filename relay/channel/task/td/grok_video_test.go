package td

// 本文件锁定 grok-imagine-video-1.5 接入契约，并专项回归存量三模型。
//
// grok 与前三者的**路径相同但参数协议不同**（详见 docs/channel/td-grok-video-design.md）：
//  1. duration 是枚举（6/10/15）而非范围，且必填；
//  2. aspect_ratio 支持 1:1（前三者不支持）；
//  3. 参考图走 input_reference（string 或 object）+ extra.reference_images，
//     无 images 字段；单图误用 reference_images 会被上游拒绝；
//  4. 提交响应任务 ID 可能在 data.task_id 或顶层 id；
//  5. 查询状态含 queued；
//  6. 上游返回 videos[].expires_at。
//
// 「存量三模型零回归」由本文件末尾的专项用例守住。

import (
	"io"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
)

// ============================
// 1. duration 枚举与必填
// ============================

func TestGrokDurationEnum(t *testing.T) {
	cases := []struct {
		duration int
		wantErr  bool
	}{
		{6, false},
		{10, false},
		{15, false},
		{7, true},  // 区间内但不在枚举集合 —— 这是枚举模式存在的意义
		{14, true}, // 同上
		{5, true},  // 低于最小档
		{16, true}, // 高于最大档
		{0, true},
	}
	for _, tc := range cases {
		body := `{"model":"` + VideoModelGrokImagineVideo15 +
			`","prompt":"p","duration":` + itoa(tc.duration) + `}`
		c := mkVideoCtx(t, body)
		err := (&TaskAdaptor{}).ValidateRequestAndSetAction(c, mkInfo())
		if tc.wantErr && err == nil {
			t.Errorf("duration=%d 应被拒绝但通过", tc.duration)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("duration=%d 应通过但被拒: %s", tc.duration, err.Message)
		}
	}
}

// duration 必填：不传应被拒绝（与前三者可缺省不同）
func TestGrokDurationRequired(t *testing.T) {
	body := `{"model":"` + VideoModelGrokImagineVideo15 + `","prompt":"p"}`
	c := mkVideoCtx(t, body)
	err := (&TaskAdaptor{}).ValidateRequestAndSetAction(c, mkInfo())
	if err == nil {
		t.Fatal("grok 未传 duration 应被拒绝")
	}
	if !strings.Contains(err.Message, "duration is required") {
		t.Errorf("应提示 duration is required，实际: %s", err.Message)
	}
	if !strings.Contains(err.Message, "6, 10, 15") {
		t.Errorf("错误信息应列出枚举集合，实际: %s", err.Message)
	}
}

// durationHint 枚举模式输出 "6, 10, 15"，范围模式输出 "3-15"
func TestGrokDurationHintFormat(t *testing.T) {
	grok, ok := videoModelSpecs[VideoModelGrokImagineVideo15]
	if !ok {
		t.Fatal("grok spec 未登记")
	}
	if got := grok.durationHint(); got != "6, 10, 15" {
		t.Errorf("grok durationHint 应为 \"6, 10, 15\"，实际 %q", got)
	}
	h3 := videoModelSpecs[VideoModelMiniMaxH3]
	if got := h3.durationHint(); got != "3-15" {
		t.Errorf("minimax-h3 durationHint 应为 \"3-15\"，实际 %q", got)
	}
}

// ============================
// 2. aspect_ratio：grok 支持 1:1
// ============================

func TestGrokAspectRatioIncludes1x1(t *testing.T) {
	for _, ratio := range []string{"16:9", "9:16", "1:1"} {
		body := `{"model":"` + VideoModelGrokImagineVideo15 +
			`","prompt":"p","duration":6,"aspect_ratio":"` + ratio + `"}`
		c := mkVideoCtx(t, body)
		if err := (&TaskAdaptor{}).ValidateRequestAndSetAction(c, mkInfo()); err != nil {
			t.Errorf("grok aspect_ratio=%s 应通过但被拒: %s", ratio, err.Message)
		}
	}
	// 非法值仍应拒绝
	body := `{"model":"` + VideoModelGrokImagineVideo15 +
		`","prompt":"p","duration":6,"aspect_ratio":"4:3"}`
	c := mkVideoCtx(t, body)
	if err := (&TaskAdaptor{}).ValidateRequestAndSetAction(c, mkInfo()); err == nil {
		t.Error("grok aspect_ratio=4:3 应被拒绝")
	}
}

// 存量三模型仍不接受 1:1（确认 per-model 集合没有误放开）
func TestExistingModelsStillReject1x1(t *testing.T) {
	for _, model := range []string{
		VideoModelMiniMaxH3, VideoModelSeedance20, VideoModelSeedance25,
	} {
		body := `{"model":"` + model + `","prompt":"p","aspect_ratio":"1:1"}`
		c := mkVideoCtx(t, body)
		err := (&TaskAdaptor{}).ValidateRequestAndSetAction(c, mkInfo())
		if err == nil {
			t.Errorf("%s 传 1:1 应被拒绝（回归！）", model)
		}
	}
}

// 错误提示应使用该模型自己的比例集合，与实际校验一致
func TestAspectRatioHintPerModel(t *testing.T) {
	grok := videoModelSpecs[VideoModelGrokImagineVideo15]
	if !strings.Contains(grok.aspectRatioHint(), "1:1") {
		t.Errorf("grok 提示应含 1:1，实际 %q", grok.aspectRatioHint())
	}
	h3 := videoModelSpecs[VideoModelMiniMaxH3]
	if strings.Contains(h3.aspectRatioHint(), "1:1") {
		t.Errorf("minimax-h3 提示不应含 1:1，实际 %q", h3.aspectRatioHint())
	}
}

// ============================
// 3. extra 参考图通道
// ============================

// images 字段对 grok 不支持（上游文档无该字段）
func TestGrokRejectsImagesField(t *testing.T) {
	body := `{"model":"` + VideoModelGrokImagineVideo15 +
		`","prompt":"p","duration":6,"images":["https://x/1.png"]}`
	c := mkVideoCtx(t, body)
	err := (&TaskAdaptor{}).ValidateRequestAndSetAction(c, mkInfo())
	if err == nil {
		t.Fatal("grok 传 images 应被拒绝（上游不认该字段）")
	}
	if !strings.Contains(err.Message, "does not support images") {
		t.Errorf("应提示不支持 images，实际: %s", err.Message)
	}
}

// 单图误用 extra.reference_images 应被拦截（上游会 400）
func TestGrokSingleRefImageMustUseInputReference(t *testing.T) {
	body := `{"model":"` + VideoModelGrokImagineVideo15 + `","prompt":"p","duration":6,
		"extra":{"aspect_ratio":"16:9","resolution":"720p",
		"reference_images":[{"url":"https://x/1.png","role":"first_frame"}]}}`
	c := mkVideoCtx(t, body)
	err := (&TaskAdaptor{}).ValidateRequestAndSetAction(c, mkInfo())
	if err == nil {
		t.Fatal("单图走 reference_images 应被拒绝")
	}
	if !strings.Contains(err.Message, "input_reference") {
		t.Errorf("应引导改用 input_reference，实际: %s", err.Message)
	}
}

// extra.reference_images 2~7 张通过，8 张拒绝
func TestGrokExtraReferenceImagesLimit(t *testing.T) {
	mk := func(n int) string {
		items := make([]string, 0, n)
		for i := 0; i < n; i++ {
			items = append(items,
				`{"url":"https://x/`+itoa(i)+`.png","role":"reference_image"}`)
		}
		return `{"model":"` + VideoModelGrokImagineVideo15 + `","prompt":"p","duration":6,
			"extra":{"aspect_ratio":"16:9","resolution":"720p","reference_images":[` +
			strings.Join(items, ",") + `]}}`
	}
	for _, n := range []int{2, 7} {
		c := mkVideoCtx(t, mk(n))
		if err := (&TaskAdaptor{}).ValidateRequestAndSetAction(c, mkInfo()); err != nil {
			t.Errorf("%d 张参考图应通过但被拒: %s", n, err.Message)
		}
	}
	c := mkVideoCtx(t, mk(8))
	err := (&TaskAdaptor{}).ValidateRequestAndSetAction(c, mkInfo())
	if err == nil {
		t.Fatal("8 张参考图应被拒绝")
	}
	if !strings.Contains(err.Message, "too many reference images") {
		t.Errorf("应提示超限，实际: %s", err.Message)
	}
}

// extra 校验只对声明了 MaxExtraReferenceImages 的模型生效
func TestExtraReferenceImagesValidationScoped(t *testing.T) {
	for _, model := range []string{
		VideoModelMiniMaxH3, VideoModelSeedance20, VideoModelSeedance25,
	} {
		spec := videoModelSpecs[model]
		if spec.MaxExtraReferenceImages != 0 {
			t.Errorf("%s 不应启用 extra 参考图校验，实际上限 %d",
				model, spec.MaxExtraReferenceImages)
		}
	}
	// 存量模型传 extra.reference_images=1 不应触发 grok 专属报错
	body := `{"model":"` + VideoModelSeedance25 + `","prompt":"p","duration":6,
		"extra":{"reference_images":[{"url":"https://x/1.png"}]}}`
	c := mkVideoCtx(t, body)
	err := (&TaskAdaptor{}).ValidateRequestAndSetAction(c, mkInfo())
	if err != nil && strings.Contains(err.Message, "input_reference") {
		t.Errorf("存量模型不应触发 grok 专属的 input_reference 引导，实际: %s", err.Message)
	}
}

// ============================
// 4. extra 透传
// ============================

func TestGrokExtraPassThrough(t *testing.T) {
	body := `{"model":"` + VideoModelGrokImagineVideo15 + `","prompt":"p","duration":10,
		"aspect_ratio":"16:9","resolution":"720p",
		"extra":{"aspect_ratio":"16:9","resolution":"720p",
		"reference_images":[{"url":"https://x/a.png","role":"first_frame"},
		{"url":"https://x/b.png","role":"last_frame"}]}}`
	c := mkVideoCtx(t, body)
	info := mkInfo()
	a := &TaskAdaptor{}
	if err := a.ValidateRequestAndSetAction(c, info); err != nil {
		t.Fatalf("校验失败: %s", err.Message)
	}
	reader, err := a.BuildRequestBody(c, info)
	if err != nil {
		t.Fatalf("建体失败: %s", err)
	}
	raw, _ := io.ReadAll(reader)
	got := string(raw)
	for _, want := range []string{
		`"extra"`, `"reference_images"`, `"first_frame"`, `"last_frame"`,
		`"duration":10`, `"aspect_ratio":"16:9"`, `"resolution":"720p"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("上游请求体缺少 %s\n实际: %s", want, got)
		}
	}
}

// ============================
// 5. input_reference 双写法
// ============================

func TestInputReferenceBothForms(t *testing.T) {
	// 字符串写法
	var s InputReference
	if err := common.UnmarshalJsonStr(`"https://x/a.jpg"`, &s); err != nil {
		t.Fatal(err)
	}
	if s.URL != "https://x/a.jpg" || s.WasObject {
		t.Errorf("字符串写法解析错误: %+v", s)
	}
	out, _ := common.Marshal(s)
	if string(out) != `"https://x/a.jpg"` {
		t.Errorf("字符串写法回传应保持字符串，实际 %s", string(out))
	}

	// 对象写法
	var o InputReference
	if err := common.UnmarshalJsonStr(`{"url":"https://x/b.jpg"}`, &o); err != nil {
		t.Fatal(err)
	}
	if o.URL != "https://x/b.jpg" || !o.WasObject {
		t.Errorf("对象写法解析错误: %+v", o)
	}
	out2, _ := common.Marshal(o)
	if string(out2) != `{"url":"https://x/b.jpg"}` {
		t.Errorf("对象写法回传应保持对象，实际 %s", string(out2))
	}
}

// input_reference 经完整请求链路解析并下发上游
func TestGrokInputReferenceEndToEnd(t *testing.T) {
	for _, form := range []string{
		`"https://x/first.jpg"`,
		`{"url":"https://x/first.jpg"}`,
	} {
		body := `{"model":"` + VideoModelGrokImagineVideo15 + `","prompt":"p","duration":10,
			"input_reference":` + form + `}`
		c := mkVideoCtx(t, body)
		info := mkInfo()
		a := &TaskAdaptor{}
		if err := a.ValidateRequestAndSetAction(c, info); err != nil {
			t.Fatalf("form=%s 校验失败: %s", form, err.Message)
		}
		reader, err := a.BuildRequestBody(c, info)
		if err != nil {
			t.Fatalf("form=%s 建体失败: %s", form, err)
		}
		raw, _ := io.ReadAll(reader)
		if !strings.Contains(string(raw), "input_reference") ||
			!strings.Contains(string(raw), "https://x/first.jpg") {
			t.Errorf("form=%s 未正确下发: %s", form, string(raw))
		}
	}
}

// ============================
// 6. 提交响应任务 ID 三级兜底
// ============================

func TestSubmitTaskIDFallback(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"data.id", `{"code":200,"data":{"id":"t_a"}}`, "t_a"},
		{"data.task_id", `{"code":200,"data":{"task_id":"t_b"}}`, "t_b"},
		{"顶层 id", `{"code":200,"id":"t_c","data":{}}`, "t_c"},
		{"data.id 优先于 task_id", `{"code":200,"data":{"id":"t_d","task_id":"t_x"}}`, "t_d"},
		{"data.task_id 优先于顶层", `{"code":200,"id":"t_x","data":{"task_id":"t_e"}}`, "t_e"},
		{"全空", `{"code":200,"data":{}}`, ""},
	}
	for _, tc := range cases {
		var resp SubmitResponse
		if err := common.UnmarshalJsonStr(tc.body, &resp); err != nil {
			t.Fatalf("%s 解析失败: %v", tc.name, err)
		}
		if got := resp.resolveTaskID(); got != tc.want {
			t.Errorf("%s: resolveTaskID=%q，期望 %q", tc.name, got, tc.want)
		}
	}
}

// ============================
// 7. queued 状态
// ============================

func TestQueuedStatusMapping(t *testing.T) {
	got := convertTudouStatus("queued")
	if got == "UNKNOWN" || got == "" {
		t.Errorf("queued 不应映射为 %q（会卡到超时）", got)
	}
	if got != "QUEUED" {
		t.Errorf("queued 应映射为 QUEUED，实际 %q", got)
	}
	// 既有映射不受影响
	existing := map[string]string{
		"submitted":  "SUBMITTED",
		"processing": "IN_PROGRESS",
		"completed":  "SUCCESS",
		"failed":     "FAILURE",
	}
	for in, want := range existing {
		if got := convertTudouStatus(in); got != want {
			t.Errorf("%s 应映射为 %s，实际 %s（回归！）", in, want, got)
		}
	}
}

// ============================
// 8. expires_at 解析
// ============================

func TestTaskVideoExpiresAt(t *testing.T) {
	body := `{"code":200,"data":{"id":"t1","status":"completed","result":{
		"video_url":"https://cdn/x.mp4",
		"videos":[{"url":["https://cdn/x.mp4"],"expires_at":1787677893}]}}}`
	var resp VideoQueryTaskResponse
	if err := common.UnmarshalJsonStr(body, &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Data.Result.Videos) != 1 {
		t.Fatalf("videos 数量错误: %d", len(resp.Data.Result.Videos))
	}
	if got := resp.Data.Result.Videos[0].ExpiresAt; got != 1787677893 {
		t.Errorf("expires_at 应为 1787677893，实际 %d", got)
	}
}

// ============================
// 9. grok 不支持的能力
// ============================

func TestGrokUnsupportedFeatures(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"参考视频", `"videos":["https://x/a.mp4"]`, "does not support videos"},
		{"参考音频", `"audios":["https://x/a.mp3"]`, "does not support audios"},
		{"音效开关", `"generate_audio":true`, "does not support generate_audio"},
		{"1080p", `"resolution":"1080p"`, "invalid resolution"},
	}
	for _, tc := range cases {
		body := `{"model":"` + VideoModelGrokImagineVideo15 +
			`","prompt":"p","duration":6,` + tc.body + `}`
		c := mkVideoCtx(t, body)
		err := (&TaskAdaptor{}).ValidateRequestAndSetAction(c, mkInfo())
		if err == nil {
			t.Errorf("%s 应被拒绝", tc.name)
			continue
		}
		if !strings.Contains(err.Message, tc.want) {
			t.Errorf("%s 应提示 %q，实际: %s", tc.name, tc.want, err.Message)
		}
	}
}

// ============================
// 10. 存量三模型专项回归
// ============================

// grok 接入不得改变前三者的任何能力边界
func TestExistingVideoModelsUnaffected(t *testing.T) {
	type expect struct {
		minD, maxD, defD int
		res              []string
		imgs, vids, auds int
		genAudio         bool
	}
	want := map[string]expect{
		VideoModelMiniMaxH3:  {3, 15, 4, []string{"720p", "1080p", "2k"}, 9, 0, 0, false},
		VideoModelSeedance20: {3, 15, 4, []string{"720p"}, 9, 3, 3, true},
		VideoModelSeedance25: {4, 30, 4, []string{"720p"}, 30, 10, 10, true},
	}
	for model, w := range want {
		spec, ok := videoModelSpecs[model]
		if !ok {
			t.Fatalf("%s 从能力矩阵中消失（回归！）", model)
		}
		if spec.MinDuration != w.minD || spec.MaxDuration != w.maxD ||
			spec.DefaultDuration != w.defD {
			t.Errorf("%s 时长范围被改动: %d-%d(def %d)，期望 %d-%d(def %d)",
				model, spec.MinDuration, spec.MaxDuration, spec.DefaultDuration,
				w.minD, w.maxD, w.defD)
		}
		if len(spec.AllowedResolutions) != len(w.res) {
			t.Errorf("%s 分辨率档位被改动: %v，期望 %v",
				model, spec.AllowedResolutions, w.res)
		}
		if spec.MaxImages != w.imgs || spec.MaxVideos != w.vids || spec.MaxAudios != w.auds {
			t.Errorf("%s 素材上限被改动: %d/%d/%d，期望 %d/%d/%d",
				model, spec.MaxImages, spec.MaxVideos, spec.MaxAudios,
				w.imgs, w.vids, w.auds)
		}
		if spec.AllowGenerateAudio != w.genAudio {
			t.Errorf("%s generate_audio 被改动: %v，期望 %v",
				model, spec.AllowGenerateAudio, w.genAudio)
		}
		// 新增字段对存量模型必须为空，否则会改变校验走向
		if len(spec.AllowedDurations) != 0 {
			t.Errorf("%s AllowedDurations 应为空（回归！）", model)
		}
		if spec.RequireDuration {
			t.Errorf("%s RequireDuration 应为 false（回归！）", model)
		}
		if len(spec.AllowedAspectRatios) != 0 {
			t.Errorf("%s AllowedAspectRatios 应为空（回归！）", model)
		}
		if spec.MaxExtraReferenceImages != 0 {
			t.Errorf("%s MaxExtraReferenceImages 应为 0（回归！）", model)
		}
	}
}

// 存量模型的 duration 仍可缺省（grok 之外的模型不受 RequireDuration 影响）
func TestExistingModelsDurationOptional(t *testing.T) {
	for _, model := range []string{
		VideoModelMiniMaxH3, VideoModelSeedance20, VideoModelSeedance25,
	} {
		body := `{"model":"` + model + `","prompt":"p"}`
		c := mkVideoCtx(t, body)
		info := mkInfo()
		a := &TaskAdaptor{}
		if err := a.ValidateRequestAndSetAction(c, info); err != nil {
			t.Errorf("%s 缺省 duration 应通过但被拒: %s", model, err.Message)
			continue
		}
		// 应填充默认值 4
		reader, err := a.BuildRequestBody(c, info)
		if err != nil {
			t.Fatalf("%s 建体失败: %s", model, err)
		}
		raw, _ := io.ReadAll(reader)
		if !strings.Contains(string(raw), `"duration":4`) {
			t.Errorf("%s 缺省 duration 应填 4，实际请求体: %s", model, string(raw))
		}
	}
}

// 存量模型的错误文案未因本次改造而改变
func TestExistingModelErrorMessagesUnchanged(t *testing.T) {
	cases := []struct {
		name  string
		body  string
		want  string
	}{
		{"seedance-2.0 时长越界",
			`{"model":"seedance-2.0","prompt":"p","duration":16}`,
			"invalid duration: 16, model seedance-2.0 supports 3-15 seconds"},
		{"seedance-2.0 分辨率",
			`{"model":"seedance-2.0","prompt":"p","resolution":"1080p"}`,
			"invalid resolution: 1080p, model seedance-2.0 supports: 720p"},
		{"minimax-h3 不支持视频",
			`{"model":"minimax-h3","prompt":"p","videos":["https://x/a.mp4"]}`,
			"model minimax-h3 does not support videos"},
		{"比例非法（存量集合）",
			`{"model":"seedance-2.5","prompt":"p","aspect_ratio":"4:3"}`,
			"invalid aspect_ratio: 4:3, must be one of: 16:9, 9:16"},
	}
	for _, tc := range cases {
		c := mkVideoCtx(t, tc.body)
		err := (&TaskAdaptor{}).ValidateRequestAndSetAction(c, mkInfo())
		if err == nil {
			t.Errorf("%s 应被拒绝", tc.name)
			continue
		}
		if err.Message != tc.want {
			t.Errorf("%s 文案改变（回归！）\n 期望: %s\n 实际: %s",
				tc.name, tc.want, err.Message)
		}
	}
}

// 图片分支：新增字段不影响图片模型判定与校验
func TestImageBranchUnaffectedByGrok(t *testing.T) {
	body := `{"model":"gpt-image-2-all","prompt":"p","resolution":"4k","quality":"high"}`
	c := mkCtx(t, body)
	a := &TaskAdaptor{}
	if err := a.ValidateRequestAndSetAction(c, mkInfo()); err != nil {
		t.Fatalf("图片校验失败: %s", err.Message)
	}
	if a.videoTask {
		t.Error("图片请求不应被判为视频分支（回归！）")
	}
}
