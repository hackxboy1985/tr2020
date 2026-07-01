package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

// PlatformAdapter 三方平台适配器（已封装 Coze 的平台）
type PlatformAdapter struct {
	baseURL   string
	apiKey    string
	apiSecret string
	client    *http.Client
}

// NewPlatformAdapter 创建三方平台适配器
func NewPlatformAdapter() *PlatformAdapter {
	return &PlatformAdapter{
		baseURL:   common.GetEnvOrDefault("PLATFORM_BASE_URL", ""),
		apiKey:    common.GetEnvOrDefault("PLATFORM_API_KEY", ""),
		apiSecret: common.GetEnvOrDefault("PLATFORM_API_SECRET", ""),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (a *PlatformAdapter) GetName() string {
	return "platform"
}

func (a *PlatformAdapter) CreateProject(ctx context.Context, req *dto.CreateVideoProjectRequest) (*dto.AdapterCreateResponse, error) {
	if a.baseURL == "" || a.apiKey == "" {
		return nil, errors.New("platform base url or api key not configured")
	}

	// 三方平台的接口参数格式与 Coze 一致，直接转发
	platformReq := map[string]interface{}{
		"product_img_url": req.ProductImgUrl,
		"brand":           req.Brand,
		"product_name":    req.ProductName,
		"tagline":         req.Tagline,
		"selling_points":  req.SellingPoints,
		"prompt":          req.Prompt,
		"vtype":           req.Vtype,
		"vtype_add":       req.VtypeAdd,
		"language":        req.Language,
		"platform":        req.Platform,
		"region":          req.Region,
		"roles":           req.Roles,
		"select_audios":   req.SelectAudios,
		"duration":        req.Duration,
		"resolution":      req.Resolution,
		"video_model":     req.VideoModel,
		"whstr":           req.Whstr,
	}

	// 序列化请求
	body, err := common.Marshal(platformReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 发送请求到三方平台
	url := a.baseURL + "/api/video/create"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+a.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("platform api error: status=%d, body=%s", resp.StatusCode, string(respBody))
	}

	// 解析响应
	var platformResp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			ProjectId   string `json:"project_id"`
			ProjectName string `json:"project_name"`
			Status      string `json:"status"`
		} `json:"data"`
	}

	if err := common.Unmarshal(respBody, &platformResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if platformResp.Code != 200 {
		return nil, fmt.Errorf("platform api error: code=%d, msg=%s", platformResp.Code, platformResp.Msg)
	}

	return &dto.AdapterCreateResponse{
		RemoteProjectId: platformResp.Data.ProjectId,
		Status:          platformResp.Data.Status,
		Message:         platformResp.Msg,
	}, nil
}

func (a *PlatformAdapter) GetProjectStatus(ctx context.Context, remoteProjectId string) (*dto.AdapterStatusResponse, error) {
	if a.baseURL == "" || a.apiKey == "" {
		return nil, errors.New("platform base url or api key not configured")
	}

	// 查询三方平台项目状态
	url := fmt.Sprintf("%s/api/video/projects/%s", a.baseURL, remoteProjectId)
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+a.apiKey)

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("platform api error: status=%d, body=%s", resp.StatusCode, string(respBody))
	}

	// 解析响应
	var platformResp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Status           string `json:"status"`
			ErrorMsg         string `json:"error_msg"`
			Progress         string `json:"progress"`
			MainImageUrl     string `json:"main_image_url"`
			MainImageAssetId string `json:"main_image_asset_id"`
			GeneratedResult  string `json:"generated_result"`
			FirstVideoUrl    string `json:"first_video_url"`
		} `json:"data"`
	}

	if err := common.Unmarshal(respBody, &platformResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if platformResp.Code != 200 {
		return nil, fmt.Errorf("platform api error: code=%d, msg=%s", platformResp.Code, platformResp.Msg)
	}

	return &dto.AdapterStatusResponse{
		Status:           platformResp.Data.Status,
		ErrorMsg:         platformResp.Data.ErrorMsg,
		Progress:         platformResp.Data.Progress,
		MainImageUrl:     platformResp.Data.MainImageUrl,
		MainImageAssetId: platformResp.Data.MainImageAssetId,
		GeneratedResult:  platformResp.Data.GeneratedResult,
		FirstVideoUrl:    platformResp.Data.FirstVideoUrl,
	}, nil
}

func (a *PlatformAdapter) ValidateWebhook(ctx context.Context, signature string, body []byte) error {
	if a.apiSecret == "" {
		return errors.New("platform api secret not configured")
	}

	// 三方平台可能有自己的签名验证方式
	// 这里假设与 Coze 类似，使用 HMAC-SHA256
	// 实际需要根据平台文档调整
	// 暂时简化处理：如果配置了secret就验证，否则跳过
	if signature == "" {
		return nil // 允许无签名（开发阶段）
	}

	// TODO: 实现具体的签名验证逻辑
	return nil
}

func (a *PlatformAdapter) ParseWebhookPayload(body []byte) (*dto.WebhookPayload, error) {
	// 三方平台的 webhook 格式可能与文档中定义的一致
	var payload dto.WebhookPayload

	if err := common.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse platform webhook: %w", err)
	}

	return &payload, nil
}
