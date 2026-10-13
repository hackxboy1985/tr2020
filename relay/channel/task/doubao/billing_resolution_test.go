package doubao

import (
	"bytes"
	"math"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
)

const (
	model25   = "doubao-seedance-2-5-260628"
	model20   = "doubao-seedance-2-0-260128"
	modelFast = "doubao-seedance-2-0-fast-260128"
	modelMini = "doubao-seedance-2-0-mini-260615"
)

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

// ---------------------------------------------------------------------------
// 价格表：模型 → 分辨率 → (不含视频, 含视频)
// ---------------------------------------------------------------------------

func TestVideoResolutionPriceTable(t *testing.T) {
	cases := []struct {
		model                       string
		resolution                  string
		wantWithoutVideo, wantVideo float64
	}{
		// doubao-seedance-2.5
		{model25, "480p", 70.0, 42.0},
		{model25, "720p", 70.0, 42.0},
		{model25, "1080p", 77.0, 46.0},
		// doubao-seedance-2.0
		{model20, "480p", 46.0, 28.0},
		{model20, "720p", 46.0, 28.0},
		{model20, "1080p", 51.0, 31.0},
		{model20, "4k", 26.0, 16.0},
		// doubao-seedance-2.0-fast
		{modelFast, "480p", 37.0, 22.0},
		{modelFast, "720p", 37.0, 22.0},
		// doubao-seedance-2.0-mini
		{modelMini, "480p", 23.0, 14.0},
		{modelMini, "720p", 23.0, 14.0},
	}
	for _, tc := range cases {
		gotWithout, gotWith, ok := GetVideoResolutionPrice(tc.model, tc.resolution)
		if !ok {
			t.Fatalf("%s/%s: 期望配置存在", tc.model, tc.resolution)
		}
		if !almostEqual(gotWithout, tc.wantWithoutVideo) {
			t.Errorf("%s/%s 不含视频: got %v, want %v", tc.model, tc.resolution, gotWithout, tc.wantWithoutVideo)
		}
		if !almostEqual(gotWith, tc.wantVideo) {
			t.Errorf("%s/%s 含视频: got %v, want %v", tc.model, tc.resolution, gotWith, tc.wantVideo)
		}
	}
}

func TestVideoInputRatioValues(t *testing.T) {
	cases := []struct {
		model      string
		resolution string
		want       float64
	}{
		{model25, "480p", 42.0 / 70.0},
		{model25, "720p", 42.0 / 70.0},
		{model25, "1080p", 46.0 / 77.0},
		{model20, "480p", 28.0 / 46.0},
		{model20, "1080p", 31.0 / 51.0},
		{model20, "4k", 16.0 / 26.0},
		{modelFast, "720p", 22.0 / 37.0},
		{modelMini, "720p", 14.0 / 23.0},
	}
	for _, tc := range cases {
		without, with, ok := GetVideoResolutionPrice(tc.model, tc.resolution)
		if !ok {
			t.Fatalf("%s/%s: 期望配置存在", tc.model, tc.resolution)
		}
		ratio := with / without
		if !almostEqual(ratio, tc.want) {
			t.Errorf("%s/%s 视频折扣: got %v, want %v", tc.model, tc.resolution, ratio, tc.want)
		}
	}
}

func TestUnknownModelHasNoPrice(t *testing.T) {
	if _, _, ok := GetVideoResolutionPrice("doubao-seedance-1-5-pro-251215", "720p"); ok {
		t.Fatal("未配置价格表的模型不应返回价格")
	}
	if _, _, ok := GetVideoResolutionPrice("doubao-seedance-1-5-pro-251215", "720p"); ok {
		t.Fatal("未配置价格表的模型不应返回价格")
	}
}

// 严格查表：未配置的组合一律 ok=false，不做任何回退
func TestUnlistedResolutionHasNoPrice(t *testing.T) {
	// 空分辨率不是合法档位
	if _, _, ok := GetVideoResolutionPrice(model25, ""); ok {
		t.Fatal("空分辨率不应返回价格")
	}
	// fast 未配置 1080p：应暴露为未配置，而不是悄悄按 720p 计价
	if _, _, ok := GetVideoResolutionPrice(modelFast, "1080p"); ok {
		t.Fatal("fast 未配置 1080p，不应回退返回价格")
	}
	// mini 未配置 1080p
	if _, _, ok := GetVideoResolutionPrice(modelMini, "1080p"); ok {
		t.Fatal("mini 未配置 1080p，不应回退返回价格")
	}
	// 2.5 未配置 4k
	if _, _, ok := GetVideoResolutionPrice(model25, "4k"); ok {
		t.Fatal("2.5 未配置 4k，不应回退返回价格")
	}
}

// ---------------------------------------------------------------------------
// 分辨率归一化
// ---------------------------------------------------------------------------

func TestNormalizeResolution(t *testing.T) {
	cases := map[string]string{
		"480p":   Resolution480P,
		"480P":   Resolution480P,
		" 720p ": Resolution720P,
		"720P":   Resolution720P,
		"1080p":  Resolution1080P,
		"1080P":  Resolution1080P,
		"4k":     Resolution4K,
		"4K":     Resolution4K,
		"2160p":  Resolution4K,
		"1080":   Resolution1080P,
		"720":    Resolution720P,
		// 尺寸不是 Doubao 的分辨率入参，不转换
		"1920x1080": "",
		"1280*720":  "",
		"3840x2160": "",
		// 2K(DCI 2048x1080) 与 1080p 不是同一档，不收录
		"2k":      "",
		"":        "",
		"unknown": "",
	}
	for input, want := range cases {
		if got := NormalizeResolution(input); got != want {
			t.Errorf("NormalizeResolution(%q) = %q, want %q", input, got, want)
		}
	}
}

// ---------------------------------------------------------------------------
// 请求解析：路径一（OpenAI 兼容 /v1/videos，size 字段）
// ---------------------------------------------------------------------------

func TestResolveVideoResolutionOpenAIPath(t *testing.T) {
	// 标准格式：分辨率在 metadata.resolution
	req := &relaycommon.TaskSubmitReq{Metadata: map[string]interface{}{"resolution": "1080p"}}
	if got := ResolveVideoResolution(req); got != Resolution1080P {
		t.Fatalf("metadata.resolution=1080p 应解析为 1080p, got %q", got)
	}

	// size 字段不属于 Doubao 入参，不应被解析
	req = &relaycommon.TaskSubmitReq{Size: "1920x1080"}
	if got := ResolveVideoResolution(req); got != "" {
		t.Fatalf("size 不应被解析为分辨率, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// 请求解析：路径二（官方 Ark 原生，根级 resolution → metadata）
// ---------------------------------------------------------------------------

func TestResolveVideoResolutionArkPath(t *testing.T) {
	req := &relaycommon.TaskSubmitReq{
		Metadata: map[string]interface{}{
			"resolution": "1080p",
			"ratio":      "16:9",
		},
	}
	if got := ResolveVideoResolution(req); got != Resolution1080P {
		t.Fatalf("metadata.resolution=1080p 应解析为 1080p, got %q", got)
	}

	// 数值型分辨率
	req = &relaycommon.TaskSubmitReq{
		Metadata: map[string]interface{}{"resolution": float64(1080)},
	}
	if got := ResolveVideoResolution(req); got != Resolution1080P {
		t.Fatalf("数值型 resolution 应解析为 1080p, got %q", got)
	}
}

// 两种格式的请求体，经真实解析链路后都应得到正确分辨率
func TestResolveVideoResolutionFromRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, tc := range []struct {
		body string
		want string
	}{
		// Ark 原生格式：顶层 resolution
		{`{"model":"doubao-seedance-2-5-260628","resolution":"1080p","content":[{"type":"text","text":"hi"}]}`, Resolution1080P},

		// 标准格式：metadata.resolution
		{`{"model":"doubao-seedance-2-5-260628","prompt":"p","metadata":{"resolution":"480p"}}`, Resolution480P},
	} {
		c, _ := newTestContext(tc.body)
		// 校验阶段解析并缓存；计费复用该值
		if got := c.GetString(contextKeyResolvedResolution); got != tc.want {
			t.Errorf("body=%s: 校验缓存 got %q, want %q", tc.body, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// 视频输入识别：两条路径
// ---------------------------------------------------------------------------

func TestHasVideoInputBothPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 路径二：Ark content[] 含 video_url
	arkBody := `{"model":"doubao-seedance-2-0-260128","resolution":"720p","content":[` +
		`{"type":"video_url","video_url":{"url":"https://x/v.mp4"}},` +
		`{"type":"text","text":"prompt"}]}`
	c, _ := newTestContext(arkBody)
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		t.Fatalf("GetTaskRequest: %v", err)
	}
	if !hasVideoInMetadata(req.Metadata) {
		t.Fatal("Ark content[] 含 video_url 应识别为带视频输入")
	}

	// metadata.content 含 video_url
	withVideo := &relaycommon.TaskSubmitReq{
		Metadata: map[string]interface{}{
			"content": []interface{}{
				map[string]interface{}{"type": "video_url", "video_url": map[string]interface{}{"url": "https://x/v.mp4"}},
			},
		},
	}
	if !hasVideoInMetadata(withVideo.Metadata) {
		t.Fatal("metadata.content 含 video_url 应识别为带视频输入")
	}

	// metadata.video_url 扁平字段
	flat := &relaycommon.TaskSubmitReq{
		Metadata: map[string]interface{}{"video_url": "https://x/v.mp4"},
	}
	if !hasVideoInMetadata(flat.Metadata) {
		t.Fatal("metadata.video_url 应识别为带视频输入")
	}

	// 无视频输入：仅含图片
	noVideo := &relaycommon.TaskSubmitReq{
		Metadata: map[string]interface{}{
			"content": []interface{}{
				map[string]interface{}{"type": "image_url", "image_url": map[string]interface{}{"url": "https://x/i.png"}},
			},
		},
	}
	if hasVideoInMetadata(noVideo.Metadata) {
		t.Fatal("仅含图片不应识别为带视频输入")
	}
}

// ---------------------------------------------------------------------------
// EstimateBilling 端到端：两条路径 × 分辨率 × 是否带视频
// ---------------------------------------------------------------------------

func TestEstimateBillingDoubaoPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name      string
		body      string
		model     string
		wantVideo float64 // 唯一倍率 = 该档目标价 / 基准档(720p)不含视频价
	}{
		{
			name:      "ark-720p-无视频",
			body:      `{"model":"doubao-seedance-2-5-260628","resolution":"720p","content":[{"type":"text","text":"p"}]}`,
			model:     model25,
			wantVideo: 70.0 / 70.0,
		},
		{
			name:      "ark-1080p-无视频",
			body:      `{"model":"doubao-seedance-2-5-260628","resolution":"1080p","content":[{"type":"text","text":"p"}]}`,
			model:     model25,
			wantVideo: 77.0 / 70.0,
		},
		{
			name: "ark-720p-带视频",
			body: `{"model":"doubao-seedance-2-5-260628","resolution":"720p","content":[` +
				`{"type":"video_url","video_url":{"url":"https://x/v.mp4"}},{"type":"text","text":"p"}]}`,
			model:     model25,
			wantVideo: 42.0 / 70.0,
		},
		{
			name: "ark-1080p-带视频",
			body: `{"model":"doubao-seedance-2-5-260628","resolution":"1080p","content":[` +
				`{"type":"video_url","video_url":{"url":"https://x/v.mp4"}},{"type":"text","text":"p"}]}`,
			model:     model25,
			wantVideo: 46.0 / 70.0,
		},
		{
			name:      "openai-1080p-无视频",
			body:      `{"model":"doubao-seedance-2-0-260128","prompt":"p","metadata":{"resolution":"1080p"}}`,
			model:     model20,
			wantVideo: 51.0 / 46.0,
		},
		{
			name: "openai-1080p-带视频",
			body: `{"model":"doubao-seedance-2-0-260128","prompt":"p","metadata":{` +
				`"resolution":"1080p",` +
				`"content":[{"type":"video_url","video_url":{"url":"https://x/v.mp4"}}]}}`,
			model:     model20,
			wantVideo: 31.0 / 46.0,
		},
		{
			name:      "openai-4k-无视频",
			body:      `{"model":"doubao-seedance-2-0-260128","prompt":"p","metadata":{"resolution":"4k"}}`,
			model:     model20,
			wantVideo: 26.0 / 46.0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := newTestContext(tc.body)
			req, err := relaycommon.GetTaskRequest(c)
			if err != nil {
				t.Fatalf("GetTaskRequest: %v", err)
			}
			_ = req
			info := &relaycommon.RelayInfo{
				OriginModelName: tc.model,
				ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: tc.model},
			}
			got := (&TaskAdaptor{}).EstimateBilling(c, info)

			// 只返回 1 个倍率：video_input 键恒存在，值 = 该档目标价 / 基准档不含视频价。
			if len(got) != 1 {
				t.Fatalf("应只返回 1 个倍率, got=%v", got)
			}
			videoRatio, hasVideo := got["video_input"]
			if !hasVideo {
				t.Fatalf("缺少 video_input 倍率, got=%v", got)
			}
			if !almostEqual(videoRatio, tc.wantVideo) {
				t.Errorf("video_input 倍率 got %v, want %v", videoRatio, tc.wantVideo)
			}
		})
	}
}

// 未识别分辨率时不应附加任何倍率（返回 nil，走原有基础费率）
func TestEstimateBillingUnknownResolution(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := newTestContext(`{"model":"doubao-seedance-2-5-260628","prompt":"p"}`)
	info := &relaycommon.RelayInfo{OriginModelName: model25, ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: model25}}
	got := (&TaskAdaptor{}).EstimateBilling(c, info)
	if len(got) != 0 {
		t.Fatalf("未识别分辨率不应返回倍率, got=%v", got)
	}
}

// 未配置价格表的模型不应产生任何倍率
func TestEstimateBillingUnconfiguredModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := newTestContext(`{"model":"doubao-seedance-1-5-pro-251215","resolution":"1080p","prompt":"p"}`)
	info := &relaycommon.RelayInfo{
		OriginModelName: "doubao-seedance-1-5-pro-251215",
		ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: "doubao-seedance-1-5-pro-251215"},
	}
	if got := (&TaskAdaptor{}).EstimateBilling(c, info); len(got) != 0 {
		t.Fatalf("未配置模型不应产生倍率, got=%v", got)
	}
}

// 模型映射：应以上游模型名查表
func TestEstimateBillingUsesUpstreamModelName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := newTestContext(`{"model":"my-alias","content":[{"type":"text","text":"p"}],"resolution":"1080p"}`)
	info := &relaycommon.RelayInfo{
		OriginModelName: "my-alias",
		ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: model20, IsModelMapped: true},
	}
	got := (&TaskAdaptor{}).EstimateBilling(c, info)
	// 1080p 不含视频：51/46（上游 model20 的基准档不含视频价为 46）
	videoRatio, hasVideo := got["video_input"]
	if !hasVideo || !almostEqual(videoRatio, 51.0/46.0) {
		t.Fatalf("模型映射后应按上游模型名查表, got=%v", got)
	}
	if len(got) != 1 {
		t.Fatalf("应只返回 1 个倍率, got=%v", got)
	}
}

// ---------------------------------------------------------------------------
// 基准档（720p）不含视频：倍率为 1.0；1080p 不含视频：倍率含分辨率溢价
// ---------------------------------------------------------------------------

func TestEstimateBillingNoVideoFallsBackToOne(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := newTestContext(`{"model":"doubao-seedance-2-5-260628","content":[{"type":"text","text":"p"}],"resolution":"720p"}`)
	info := &relaycommon.RelayInfo{OriginModelName: model25, ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: model25}}
	got := (&TaskAdaptor{}).EstimateBilling(c, info)

	if len(got) != 1 {
		t.Fatalf("应只返回 1 个倍率, got=%v", got)
	}
	v, ok := got["video_input"]
	if !ok {
		t.Fatalf("无视频输入时仍须返回倍率（分辨率档位也要计费）, got=%v", got)
	}
	if !almostEqual(v, 1.0) {
		t.Fatalf("基准档不含视频时倍率应为 1.0, got %v", v)
	}
}

// 非基准档 + 不含视频：倍率必须体现分辨率溢价（避免 1080p 按 720p 价计费）
func TestEstimateBillingNoVideoNonBaselineKeepsResolutionPremium(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := newTestContext(`{"model":"doubao-seedance-2-5-260628","content":[{"type":"text","text":"p"}],"resolution":"1080p"}`)
	info := &relaycommon.RelayInfo{OriginModelName: model25, ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: model25}}
	got := (&TaskAdaptor{}).EstimateBilling(c, info)

	v, ok := got["video_input"]
	if !ok {
		t.Fatalf("无视频输入时 1080p 也必须返回倍率, got=%v", got)
	}
	if !almostEqual(v, 77.0/70.0) {
		t.Fatalf("1080p 不含视频倍率应为 77/70, got %v", v)
	}
}

// ---------------------------------------------------------------------------
// 上游请求体：分辨率透传
// ---------------------------------------------------------------------------

func TestConvertToRequestPayloadKeepsResolution(t *testing.T) {
	adaptor := &TaskAdaptor{}

	// Ark 原生路径：metadata 中已有 resolution，保持原值
	payload, err := adaptor.convertToRequestPayload(&relaycommon.TaskSubmitReq{
		Model:    model20,
		Prompt:   "p",
		Metadata: map[string]interface{}{"resolution": "1080p"},
	})
	if err != nil {
		t.Fatalf("convertToRequestPayload: %v", err)
	}
	if payload.Resolution != "1080p" {
		t.Fatalf("Ark 路径分辨率应保留 1080p, got %q", payload.Resolution)
	}

	// 标准格式：metadata.resolution 已在 UnmarshalMetadata 中落到 payload
	payload, err = adaptor.convertToRequestPayload(&relaycommon.TaskSubmitReq{
		Model:    model20,
		Prompt:   "p",
		Metadata: map[string]interface{}{"resolution": "720p"},
	})
	if err != nil {
		t.Fatalf("convertToRequestPayload: %v", err)
	}
	if payload.Resolution != "720p" {
		t.Fatalf("标准格式分辨率应为 720p, got %q", payload.Resolution)
	}
}

// ---------------------------------------------------------------------------
// 辅助
// ---------------------------------------------------------------------------

// newTestContext 构造 gin 测试上下文，并走**真实的**适配器校验链路
// （ValidateRequestAndSetAction），与生产一致。
//
// 注意：
//  1. 不要手工模拟解析结果 —— ValidateBasicTaskRequest 会把 Ark 原生 content[]
//     写成 []map[string]interface{}，与 JSON 反序列化的 []interface{} 类型不同，
//     手工模拟会掩盖真实行为差异。
//  2. 必须走适配器校验（而非只调 ValidateBasicTaskRequest），因为分辨率校验与
//     结果缓存发生在适配器层，EstimateBilling 依赖该缓存。
func newTestContext(body string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/video/generations", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")

	bs, err := common.CreateBodyStorage([]byte(body))
	if err == nil {
		c.Set(common.KeyBodyStorage, bs)
	}

	info := &relaycommon.RelayInfo{
		TaskRelayInfo: &relaycommon.TaskRelayInfo{Action: "generate"},
	}
	_ = (&TaskAdaptor{}).ValidateRequestAndSetAction(c, info)
	return c, w
}

// newTestContextExpectErr 同上，但返回校验错误（用于断言非法请求被拒）。
func newTestContextExpectErr(body string) *dto.TaskError {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/video/generations", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")

	bs, err := common.CreateBodyStorage([]byte(body))
	if err == nil {
		c.Set(common.KeyBodyStorage, bs)
	}

	info := &relaycommon.RelayInfo{
		TaskRelayInfo: &relaycommon.TaskRelayInfo{Action: "generate"},
	}
	return (&TaskAdaptor{}).ValidateRequestAndSetAction(c, info)
}
