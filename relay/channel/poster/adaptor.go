package poster

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

const ChannelName = "poster"

var ModelList = []string{
	"poster-extension",
	"poster-translate",
	"poster-partial-redraw",
	"poster-enlarge",
	"poster-matting",
	"poster-scene-replace",
	"poster-product-replace",
	"poster-color-change",
	"poster-enhance",
	"poster-assisted",
}

// model → upstream endpoint
var modelEndpoints = map[string]string{
	"poster-extension":       "/openapi/v1/ai/extension",
	"poster-translate":       "/openapi/v1/ai/translate",
	"poster-partial-redraw":  "/openapi/v1/ai/partialRedrawing",
	"poster-enlarge":         "/openapi/v1/ai/enlarge",
	"poster-matting":         "/openapi/v1/ai/matting",
	"poster-scene-replace":   "/openapi/v1/ai/sceneReplace",
	"poster-product-replace": "/openapi/v1/ai/productReplace",
	"poster-color-change":    "/openapi/v1/ai/colorChange",
	"poster-enhance":         "/openapi/v1/ai/enhance",
	"poster-assisted":        "/openapi/v1/ai/assisted",
}

type Adaptor struct{}

func (a *Adaptor) Init(_ *relaycommon.RelayInfo) {}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	endpoint, ok := modelEndpoints[info.UpstreamModelName]
	if !ok {
		return "", fmt.Errorf("unsupported poster model: %s", info.UpstreamModelName)
	}
	return fmt.Sprintf("%s%s", info.ChannelBaseUrl, endpoint), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	req.Set("Content-Type", "application/json")
	req.Set("Accept", "application/json")
	req.Set("Authorization", "Bearer "+info.ApiKey)
	return nil
}

// ConvertImageRequest 根据模型名构建上游请求体
func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	meta := extractMetadata(request)
	model := info.UpstreamModelName

	switch model {
	case "poster-extension":
		req := &extensionRequest{}
		if err := unmarshalMeta(meta, req); err != nil {
			return nil, err
		}
		return req, nil

	case "poster-translate":
		req := &translateRequest{}
		if err := unmarshalMeta(meta, req); err != nil {
			return nil, err
		}
		return req, nil

	case "poster-partial-redraw":
		req := &partialRedrawingRequest{}
		if err := unmarshalMeta(meta, req); err != nil {
			return nil, err
		}
		// textPrompt 兼容外层 prompt
		if req.TextPrompt == "" {
			req.TextPrompt = request.Prompt
		}
		return req, nil

	case "poster-enlarge":
		req := &enlargeRequest{}
		if err := unmarshalMeta(meta, req); err != nil {
			return nil, err
		}
		return req, nil

	case "poster-matting":
		req := &mattingRequest{}
		if err := unmarshalMeta(meta, req); err != nil {
			return nil, err
		}
		return req, nil

	case "poster-scene-replace":
		req := &sceneReplaceRequest{}
		if err := unmarshalMeta(meta, req); err != nil {
			return nil, err
		}
		if req.TextPrompt == "" {
			req.TextPrompt = request.Prompt
		}
		return req, nil

	case "poster-product-replace":
		req := &productReplaceRequest{}
		if err := unmarshalMeta(meta, req); err != nil {
			return nil, err
		}
		if req.TextPrompt == "" {
			req.TextPrompt = request.Prompt
		}
		return req, nil

	case "poster-color-change":
		req := &colorChangeRequest{}
		if err := unmarshalMeta(meta, req); err != nil {
			return nil, err
		}
		if req.TextPrompt == "" {
			req.TextPrompt = request.Prompt
		}
		return req, nil

	case "poster-enhance":
		req := &enhanceRequest{}
		if err := unmarshalMeta(meta, req); err != nil {
			return nil, err
		}
		return req, nil

	case "poster-assisted":
		req := &assistedRequest{}
		if err := unmarshalMeta(meta, req); err != nil {
			return nil, err
		}
		// query 兼容外层 prompt
		if req.Query == "" {
			req.Query = request.Prompt
		}
		return req, nil
	}

	return nil, fmt.Errorf("unsupported poster model: %s", model)
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	switch info.UpstreamModelName {
	case "poster-assisted":
		return posterAssistedHandler(c, resp, info)
	case "poster-extension":
		return posterExtensionHandler(c, resp, info)
	default:
		return posterImageHandler(c, resp, info)
	}
}

func (a *Adaptor) GetModelList() []string { return ModelList }
func (a *Adaptor) GetChannelName() string { return ChannelName }

// ── 未使用接口的 stub 实现 ───────────────────────────────────────────────

func (a *Adaptor) ConvertOpenAIRequest(_ *gin.Context, _ *relaycommon.RelayInfo, _ *dto.GeneralOpenAIRequest) (any, error) {
	return nil, errors.New("not implemented")
}
func (a *Adaptor) ConvertRerankRequest(_ *gin.Context, _ int, _ dto.RerankRequest) (any, error) {
	return nil, errors.New("not implemented")
}
func (a *Adaptor) ConvertEmbeddingRequest(_ *gin.Context, _ *relaycommon.RelayInfo, _ dto.EmbeddingRequest) (any, error) {
	return nil, errors.New("not implemented")
}
func (a *Adaptor) ConvertAudioRequest(_ *gin.Context, _ *relaycommon.RelayInfo, _ dto.AudioRequest) (io.Reader, error) {
	return nil, errors.New("not implemented")
}
func (a *Adaptor) ConvertClaudeRequest(_ *gin.Context, _ *relaycommon.RelayInfo, _ *dto.ClaudeRequest) (any, error) {
	return nil, errors.New("not implemented")
}
func (a *Adaptor) ConvertGeminiRequest(_ *gin.Context, _ *relaycommon.RelayInfo, _ *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("not implemented")
}
func (a *Adaptor) ConvertOpenAIResponsesRequest(_ *gin.Context, _ *relaycommon.RelayInfo, _ dto.OpenAIResponsesRequest) (any, error) {
	return nil, errors.New("not implemented")
}

// ── 响应处理 ────────────────────────────────────────────────────────────

// posterImageHandler 处理返回图片 URL 的同步接口
func posterImageHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	service.CloseResponseBodyGracefully(resp)

	var sResp syncResponse
	if err := common.Unmarshal(responseBody, &sResp); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if sResp.Code != 200 {
		return nil, types.WithOpenAIError(types.OpenAIError{
			Message: sResp.Msg,
			Type:    "upstream_error",
			Code:    fmt.Sprintf("%d", sResp.Code),
		}, resp.StatusCode)
	}

	imageResp := &dto.ImageResponse{
		Created: time.Now().Unix(),
	}
	for _, rawURL := range strings.Split(sResp.Data, ",") {
		u := strings.TrimSpace(rawURL)
		if u != "" {
			imageResp.Data = append(imageResp.Data, dto.ImageData{Url: u})
		}
	}

	jsonData, err := common.Marshal(imageResp)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(http.StatusOK)
	_, _ = c.Writer.Write(jsonData)

	return &dto.Usage{PromptTokens: 1, TotalTokens: 1}, nil
}

// posterExtensionHandler 处理 /ai/extension，data 字段是字符串数组
func posterExtensionHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	service.CloseResponseBodyGracefully(resp)

	var eResp extensionSyncResponse
	if err := common.Unmarshal(responseBody, &eResp); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if eResp.Code != 200 {
		return nil, types.WithOpenAIError(types.OpenAIError{
			Message: eResp.Msg,
			Type:    "upstream_error",
			Code:    fmt.Sprintf("%d", eResp.Code),
		}, resp.StatusCode)
	}

	imageResp := &dto.ImageResponse{Created: time.Now().Unix()}
	for _, u := range eResp.Data {
		if strings.TrimSpace(u) != "" {
			imageResp.Data = append(imageResp.Data, dto.ImageData{Url: u})
		}
	}

	jsonData, err := common.Marshal(imageResp)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(http.StatusOK)
	_, _ = c.Writer.Write(jsonData)
	return &dto.Usage{PromptTokens: 1, TotalTokens: 1}, nil
}

// posterAssistedHandler 处理 /ai/assisted 文案接口，返回文本选项
func posterAssistedHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	service.CloseResponseBodyGracefully(resp)

	var aResp assistedResponse
	if err := common.Unmarshal(responseBody, &aResp); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if aResp.Code != 200 {
		return nil, types.WithOpenAIError(types.OpenAIError{
			Message: aResp.Msg,
			Type:    "upstream_error",
			Code:    fmt.Sprintf("%d", aResp.Code),
		}, resp.StatusCode)
	}

	imageResp := &dto.ImageResponse{
		Created: time.Now().Unix(),
	}
	for _, option := range aResp.Data.Options {
		imageResp.Data = append(imageResp.Data, dto.ImageData{
			Url:           "",
			RevisedPrompt: option,
		})
	}

	jsonData, err := common.Marshal(imageResp)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(http.StatusOK)
	_, _ = c.Writer.Write(jsonData)

	return &dto.Usage{PromptTokens: 1, TotalTokens: 1}, nil
}

// ── 工具函数 ─────────────────────────────────────────────────────────────

// extractMetadata 从 ImageRequest.Extra 中取出 metadata 字段
func extractMetadata(request dto.ImageRequest) map[string]any {
	if request.Extra == nil {
		return nil
	}
	raw, ok := request.Extra["metadata"]
	if !ok {
		return nil
	}
	var meta map[string]any
	if err := common.Unmarshal(raw, &meta); err != nil {
		return nil
	}
	return meta
}

// unmarshalMeta 将 metadata map 反序列化到目标结构体
func unmarshalMeta(meta map[string]any, target any) error {
	if meta == nil {
		return nil
	}
	b, err := common.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal metadata failed: %w", err)
	}
	return common.Unmarshal(b, target)
}
