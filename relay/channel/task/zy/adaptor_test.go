package zy

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// newTestContext 构造带 JSON 请求体的 gin 上下文，并预置 body storage，
// 使 ValidateBasicTaskRequest 能像真实链路一样重新解析请求体。
func newTestContext(t *testing.T, body string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	storage, err := common.CreateBodyStorage([]byte(body))
	require.NoError(t, err)
	c.Set(common.KeyBodyStorage, storage)
	t.Cleanup(func() { _ = storage.Close() })

	return c
}

func newTestInfo() *relaycommon.RelayInfo {
	info := &relaycommon.RelayInfo{}
	info.ChannelMeta = &relaycommon.ChannelMeta{
		ChannelType:    constant.ChannelTypeZy,
		ChannelBaseUrl: "https://upstream.example.com",
		ApiKey:         "sk-test",
	}
	// 真实链路中 RelayFormatTask 会由框架初始化 TaskRelayInfo，
	// ValidateBasicTaskRequest 写入 info.Action 时依赖它非 nil。
	info.TaskRelayInfo = &relaycommon.TaskRelayInfo{}
	return info
}

// TestValidateKeepsAspectRatioFromImageRequest 复现真实入口链路：
// handleZyImageTask 先把 dto.ImageRequest 转成 TaskSubmitReq 塞进 context，
// 随后 ValidateBasicTaskRequest 会用请求体重新解析并覆盖。
// 这里验证 aspect_ratio / image_size 这类"非标准 OpenAI 字段"
// 在重解析后依然能被 adaptor 读到，否则尺寸控制会静默失效。
func TestValidateKeepsAspectRatioFromImageRequest(t *testing.T) {
	body := `{"model":"gpt-image-2.5-flare","prompt":"a cat","aspect_ratio":"16:9","image_size":"2K"}`
	c := newTestContext(t, body)
	info := newTestInfo()

	// 模拟 handleZyImageTask 的预置
	var extra map[string]json.RawMessage
	require.NoError(t, common.Unmarshal([]byte(body), &extra))
	c.Set("task_request", relaycommon.TaskSubmitReq{
		Prompt:      "a cat",
		AspectRatio: "16:9",
		ImageSize:   "2K",
	})

	adaptor := &TaskAdaptor{}
	adaptor.Init(info)
	require.Nil(t, adaptor.ValidateRequestAndSetAction(c, info))

	// 校验后 context 中的 task_request 是否仍带有 aspect_ratio / image_size
	got, err := relaycommon.GetTaskRequest(c)
	require.NoError(t, err)
	require.Equal(t, "16:9", got.AspectRatio, "aspect_ratio 在重解析后丢失")
	require.Equal(t, "2K", got.ImageSize, "image_size 在重解析后丢失")
}

// TestBuildRequestBodyEndToEnd 验证最终发给上游的报文。
func TestBuildRequestBodyEndToEnd(t *testing.T) {
	body := `{"model":"gpt-image-2.5-flare","prompt":"a cat","aspect_ratio":"16:9","image_size":"2K"}`
	c := newTestContext(t, body)
	info := newTestInfo()
	info.UpstreamModelName = "gpt-image-2.5-flare"
	c.Set("task_request", relaycommon.TaskSubmitReq{
		Model:       "gpt-image-2.5-flare",
		Prompt:      "a cat",
		AspectRatio: "16:9",
		ImageSize:   "2K",
	})

	adaptor := &TaskAdaptor{}
	adaptor.Init(info)
	require.Nil(t, adaptor.ValidateRequestAndSetAction(c, info))

	reader, err := adaptor.BuildRequestBody(c, info)
	require.NoError(t, err)
	data, err := io.ReadAll(reader)
	require.NoError(t, err)

	var sent map[string]any
	require.NoError(t, common.Unmarshal(data, &sent))
	require.Equal(t, "gpt-image-2.5-flare", sent["model"])
	require.Equal(t, "a cat", sent["prompt"])
	require.Equal(t, "16:9", sent["aspect_ratio"])
	require.Equal(t, "2K", sent["image_size"])
}

// TestBuildRequestURLEndToEnd 校验上游地址拼接。
func TestBuildRequestURLEndToEnd(t *testing.T) {
	info := newTestInfo()
	adaptor := &TaskAdaptor{}
	adaptor.Init(info)

	url, err := adaptor.BuildRequestURL(info)
	require.NoError(t, err)
	require.Equal(t, "https://upstream.example.com/v1/videos", url)
}

// TestSubmitAndFetchAgainstFakeUpstream 用真实 HTTP 服务模拟上游，
// 覆盖「提交 → 解析 task_id → 查询 → 解析结果 URL」的完整闭环。
func TestSubmitAndFetchAgainstFakeUpstream(t *testing.T) {
	// FetchTask 使用 service 的包级 http 客户端，测试环境需显式初始化
	service.InitHttpClient()

	var submitBody []byte
	var gotAuth string

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/videos":
			var err error
			submitBody, err = io.ReadAll(r.Body)
			require.NoError(t, err)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"task_abc123","status":"queued","progress":0}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/videos/task_abc123":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"task_abc123","status":"completed","progress":100,` +
				`"url":"https://cdn.zy.com/a.png","metadata":{"image_url":"https://cdn.zy.com/a.png"}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer upstream.Close()

	info := newTestInfo()
	info.ChannelBaseUrl = upstream.URL
	adaptor := &TaskAdaptor{}
	adaptor.Init(info)

	// —— 提交 ——
	body := `{"model":"gpt-image-2","prompt":"a dog","size":"1280x720"}`
	c := newTestContext(t, body)
	c.Set("task_request", relaycommon.TaskSubmitReq{
		Model:  "gpt-image-2",
		Prompt: "a dog",
		Size:   "1280x720",
	})
	require.Nil(t, adaptor.ValidateRequestAndSetAction(c, info))

	reader, err := adaptor.BuildRequestBody(c, info)
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, upstream.URL+EndpointSubmit, reader)
	require.NoError(t, err)
	require.NoError(t, adaptor.BuildRequestHeader(c, req, info))

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	taskID, taskData, taskErr := adaptor.DoResponse(c, resp, info)
	require.Nil(t, taskErr)
	require.Equal(t, "task_abc123", taskID)
	require.NotEmpty(t, taskData)
	require.Equal(t, "Bearer sk-test", gotAuth)

	// 1280x720 命中 16:9 的 1K 档；gpt-image-2 档位在模型名里，不下发档位
	var sent map[string]any
	require.NoError(t, common.Unmarshal(submitBody, &sent))
	require.Equal(t, "gpt-image-2", sent["model"])
	require.Equal(t, "16:9", sent["aspect_ratio"])
	_, hasImageSize := sent["image_size"]
	require.False(t, hasImageSize, "档位在模型名中的模型不应下发 image_size")

	// —— 查询 ——
	fetchResp, err := adaptor.FetchTask(upstream.URL, "sk-test", map[string]any{"task_id": "task_abc123"}, "")
	require.NoError(t, err)
	defer fetchResp.Body.Close()
	require.Equal(t, http.StatusOK, fetchResp.StatusCode)

	fetchBody, err := io.ReadAll(fetchResp.Body)
	require.NoError(t, err)

	taskInfo, err := adaptor.ParseTaskResult(fetchBody)
	require.NoError(t, err)
	require.Equal(t, "SUCCESS", taskInfo.Status)
	require.Equal(t, "https://cdn.zy.com/a.png", taskInfo.Url)
}

// TestValidateRejectsBadInput 覆盖关键拒绝路径。
func TestValidateRejectsBadInput(t *testing.T) {
	cases := []struct {
		name string
		body string
		req  relaycommon.TaskSubmitReq
	}{
		{
			name: "aspect_ratio 非法",
			body: `{"model":"gpt-image-2","prompt":"x","aspect_ratio":"7:3"}`,
			req:  relaycommon.TaskSubmitReq{Prompt: "x", AspectRatio: "7:3"},
		},
		{
			name: "档位非法",
			body: `{"model":"gpt-image-2","prompt":"x","size":"1280x720","image_size":"8K"}`,
			req:  relaycommon.TaskSubmitReq{Prompt: "x", Size: "1280x720", ImageSize: "8K"},
		},
		{
			name: "gpt-image-2.5 不支持 2K",
			body: `{"model":"gpt-image-2.5","prompt":"x","image_size":"2K"}`,
			req:  relaycommon.TaskSubmitReq{Prompt: "x", ImageSize: "2K"},
		},
		{
			name: "image_size 与 resolution 冲突",
			body: `{"model":"gpt-image-2","prompt":"x","image_size":"2K","resolution":"4K"}`,
			req:  relaycommon.TaskSubmitReq{Prompt: "x", ImageSize: "2K", Resolution: "4K"},
		},
		{
			name: "prompt 缺失",
			body: `{"model":"gpt-image-2","prompt":""}`,
			req:  relaycommon.TaskSubmitReq{Model: "gpt-image-2"},
		},
	}

	adaptor := &TaskAdaptor{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestContext(t, tc.body)
			info := newTestInfo()
			c.Set("task_request", tc.req)

			adaptor.Init(info)
			taskErr := adaptor.ValidateRequestAndSetAction(c, info)
			require.NotNil(t, taskErr, "应当拒绝该请求")
		})
	}
}

// TestDoResponseRejectsEmptyTaskID 上游未返回任务 ID 时必须报错，避免落库空 ID。
func TestDoResponseRejectsEmptyTaskID(t *testing.T) {
	adaptor := &TaskAdaptor{}
	c := newTestContext(t, `{}`)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"status":"queued"}`)),
	}
	_, _, taskErr := adaptor.DoResponse(c, resp, newTestInfo())
	require.NotNil(t, taskErr)
}

// TestParseTaskResultFailureKeepsReason 失败任务要带出上游原因。
func TestParseTaskResultFailureKeepsReason(t *testing.T) {
	adaptor := &TaskAdaptor{}
	info, err := adaptor.ParseTaskResult([]byte(`{"id":"task_x","status":"failed","error":{"message":"boom","code":"E1"}}`))
	require.NoError(t, err)
	require.Equal(t, "FAILURE", info.Status)
	require.Equal(t, "boom", info.Reason)
}

// TestDTOStatusConstantsMatchDoc 防止常量与文档漂移。
func TestDTOStatusConstantsMatchDoc(t *testing.T) {
	require.Equal(t, "queued", StatusQueued)
	require.Equal(t, "in_progress", StatusInProgress)
	require.Equal(t, "completed", StatusCompleted)
	require.Equal(t, "failed", StatusFailed)
	require.Equal(t, "/v1/videos", EndpointSubmit)
	require.Equal(t, "/v1/videos/", EndpointQueryTask)
	require.Equal(t, 8, MaxReferenceImages)

	// taskResponse 关键字段可解析
	var r taskResponse
	require.NoError(t, common.Unmarshal([]byte(`{"id":"task_1","url":"u","metadata":{"image":"m"}}`), &r))
	require.Equal(t, "task_1", r.ID)
	require.Equal(t, "u", r.resultURL())
}

// TestChannelTypeRegistered 确认渠道常量与适配器注册一致。
func TestChannelTypeRegistered(t *testing.T) {
	require.Equal(t, 61, constant.ChannelTypeZy)
	require.Equal(t, "Zy", ChannelName)
	require.Contains(t, ModelList, "gpt-image-2.5-flare")
}

var _ = dto.TaskError{}
