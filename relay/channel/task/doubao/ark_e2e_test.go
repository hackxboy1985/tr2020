package doubao

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
)

// 用真实的 ValidateBasicTaskRequest 走一遍文档里的 Ark 原生格式请求体
func TestArkNativeFullPipeline(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := `{
  "model": "doubao-seedance-2-0-260128",
  "content": [
    {"type": "text", "text": "人物自然介绍产品（使用音频中的声音）"},
    {"type": "image_url", "image_url": {"url": "asset://asset-xxxxxxxx"}},
    {"type": "audio_url", "audio_url": {"url": "https://example.com/voice.wav"}},
    {"type":"video_url","video_url":{"url":"https://example.com/reference.mp4"}}
  ],
  "resolution": "720p",
  "ratio": "16:9",
  "duration": 5,
  "generate_audio": true
}`

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/video/generations", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	if bs, err := common.CreateBodyStorage([]byte(body)); err == nil {
		c.Set(common.KeyBodyStorage, bs)
	}

	info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{Action: "generate"}}
	var taskErr *dto.TaskError
	taskErr = relaycommon.ValidateBasicTaskRequest(c, info, "generate")
	if taskErr != nil {
		t.Fatalf("ValidateBasicTaskRequest 失败: %v", taskErr.Message)
	}

	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		t.Fatalf("GetTaskRequest: %v", err)
	}

	t.Logf("req.Prompt=%q", req.Prompt)
	t.Logf("req.Content=%v", req.Content)
	t.Logf("req.Metadata=%v", req.Metadata)

	if !hasVideoInMetadata(req.Metadata) {
		t.Fatalf("hasVideoInMetadata 应识别出视频输入, metadata=%v", req.Metadata)
	}
	if got := ResolveVideoResolution(&req); got != Resolution720P {
		t.Fatalf("分辨率应解析为 720p, got %q", got)
	}
}

// 回归：content 的两种动态类型都必须被识别。
// ValidateBasicTaskRequest 写入 []map[string]interface{}；
// JSON 反序列化得到 []interface{}。漏掉任一种都会导致 Ark 原生格式漏判。
func TestHasVideoInMetadataBothContentTypes(t *testing.T) {
	native := map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "image_url", "image_url": map[string]interface{}{"url": "asset://asset-x"}},
			{"type": "video_url", "video_url": map[string]interface{}{"url": "https://x/ref.mp4"}},
		},
	}
	if !hasVideoInMetadata(native) {
		t.Error("[]map[string]interface{} 形式的 content 应识别出视频输入")
	}

	decoded := map[string]interface{}{
		"content": []interface{}{
			map[string]interface{}{"type": "image_url"},
			map[string]interface{}{"type": "video_url", "video_url": map[string]interface{}{"url": "https://x/ref.mp4"}},
		},
	}
	if !hasVideoInMetadata(decoded) {
		t.Error("[]interface{} 形式的 content 应识别出视频输入")
	}

	noVideo := map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "image_url", "image_url": map[string]interface{}{"url": "asset://asset-x"}},
		},
	}
	if hasVideoInMetadata(noVideo) {
		t.Error("仅含 image_url 不应识别为视频输入")
	}
}

// EstimateBilling 必须同时解析「分辨率」与「有无视频」两个输入，并只返回 1 个倍率。
// 覆盖两种格式 × 三种承载位置。
func TestEstimateBillingParsesBothInputs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name      string
		body      string
		wantVideo float64 // 唯一倍率 = 该档目标价 / 基准档(720p)不含视频价
	}{
		{
			name: "标准格式-metadata.resolution-720p-含视频",
			body: `{"model":"doubao-seedance-2-0-260128","prompt":"p","metadata":{
				"resolution":"720p",
				"content":[{"type":"video_url","video_url":{"url":"https://x/ref.mp4"}}]}}`,
			wantVideo: 28.0 / 46.0,
		},
		{
			name:      "标准格式-metadata.resolution-1080p-不含视频",
			body:      `{"model":"doubao-seedance-2-0-260128","prompt":"p","metadata":{"resolution":"1080p"}}`,
			wantVideo: 51.0 / 46.0,
		},
		{
			name: "标准格式-metadata.resolution-1080p-含视频",
			body: `{"model":"doubao-seedance-2-0-260128","prompt":"p","metadata":{
				"resolution":"1080p",
				"content":[{"type":"video_url","video_url":{"url":"https://x/ref.mp4"}}]}}`,
			wantVideo: 31.0 / 46.0,
		},
		{
			name: "Ark原生-顶层resolution-720p-含视频",
			body: `{"model":"doubao-seedance-2-0-260128","content":[
				{"type":"text","text":"p"},
				{"type":"video_url","video_url":{"url":"https://x/ref.mp4"}}],
				"resolution":"720p"}`,
			wantVideo: 28.0 / 46.0,
		},
		{
			name: "Ark原生-顶层resolution-4k-不含视频",
			body: `{"model":"doubao-seedance-2-0-260128","content":[
				{"type":"text","text":"p"}],
				"resolution":"4k"}`,
			wantVideo: 26.0 / 46.0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := newTestContext(tc.body)
			got := (&TaskAdaptor{}).EstimateBilling(c, &relaycommon.RelayInfo{
				OriginModelName: model20,
				ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: model20},
			})

			// 只返回 1 个倍率，键名固定为 video_input
			if len(got) != 1 {
				t.Fatalf("应只返回 1 个倍率, got=%v", got)
			}
			if v, ok := got["video_input"]; !ok || !almostEqual(v, tc.wantVideo) {
				t.Errorf("video_input 倍率 got %v, want %v (all=%v)", v, tc.wantVideo, got)
			}
		})
	}
}

// 分辨率是计价的必要输入：缺失或无法识别时，校验阶段必须直接报错。
func TestValidateRejectsMissingResolution(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rejected := []struct {
		name string
		body string
	}{
		{"标准格式-无resolution", `{"model":"doubao-seedance-2-0-260128","prompt":"p","metadata":{"ratio":"16:9"}}`},
		{"标准格式-resol无metadata", `{"model":"doubao-seedance-2-0-260128","prompt":"p"}`},
		{"Ark原生-无resolution", `{"model":"doubao-seedance-2-0-260128","content":[{"type":"text","text":"p"}]}`},
		{"无法识别的值-2k", `{"model":"doubao-seedance-2-0-260128","prompt":"p","metadata":{"resolution":"2k"}}`},
		{"无法识别的值-尺寸", `{"model":"doubao-seedance-2-0-260128","prompt":"p","metadata":{"resolution":"1920x1080"}}`},
		{"无法识别的值-空串", `{"model":"doubao-seedance-2-0-260128","prompt":"p","metadata":{"resolution":""}}`},
	}
	for _, tc := range rejected {
		t.Run(tc.name, func(t *testing.T) {
			if err := newTestContextExpectErr(tc.body); err == nil {
				t.Fatalf("应拒绝缺少/非法分辨率的请求: %s", tc.body)
			} else if err.Code != "invalid_resolution" {
				t.Errorf("错误码 got %q, want invalid_resolution", err.Code)
			}
		})
	}

	accepted := []struct {
		name string
		body string
		want string
	}{
		{"标准格式-480p", `{"model":"doubao-seedance-2-0-260128","prompt":"p","metadata":{"resolution":"480p"}}`, Resolution480P},
		{"标准格式-1080p", `{"model":"doubao-seedance-2-0-260128","prompt":"p","metadata":{"resolution":"1080p"}}`, Resolution1080P},
		{"Ark原生-720p", `{"model":"doubao-seedance-2-0-260128","content":[{"type":"text","text":"p"}],"resolution":"720p"}`, Resolution720P},
	}
	for _, tc := range accepted {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := newTestContext(tc.body)
			if err := newTestContextExpectErr(tc.body); err != nil {
				t.Fatalf("合法请求不应被拒绝: %v", err.Message)
			}
			if got := c.GetString(contextKeyResolvedResolution); got != tc.want {
				t.Errorf("缓存分辨率 got %q, want %q", got, tc.want)
			}
		})
	}
}
