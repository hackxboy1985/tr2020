package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

// CozeAdapter Coze 渠道适配器
type CozeAdapter struct {
	apiKey        string
	workflowID    string
	webhookSecret string
	baseURL       string
	client        *http.Client
}

// NewCozeAdapter 创建 Coze 适配器
func NewCozeAdapter() *CozeAdapter {
	return &CozeAdapter{
		apiKey:        common.GetEnvOrDefaultString("COZE_API_KEY", ""),
		workflowID:    common.GetEnvOrDefaultString("COZE_WORKFLOW_ID", ""),
		webhookSecret: common.GetEnvOrDefaultString("COZE_WEBHOOK_SECRET", ""),
		baseURL:       common.GetEnvOrDefaultString("COZE_BASE_URL", "https://api.coze.cn"),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (a *CozeAdapter) GetName() string {
	return "coze"
}

func (a *CozeAdapter) CreateProject(ctx context.Context, req *dto.CreateVideoProjectRequest) (*dto.AdapterCreateResponse, error) {
	if a.apiKey == "" || a.workflowID == "" {
		return nil, errors.New("coze api key or workflow id not configured")
	}

	// 构建 Coze 工作流请求参数
	cozeReq := map[string]interface{}{
		"workflow_id": a.workflowID,
		"parameters": map[string]interface{}{
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
		},
	}

	// 序列化请求
	body, err := common.Marshal(cozeReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 发送请求到 Coze
	httpReq, err := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/v1/workflow/run", bytes.NewReader(body))
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
		return nil, fmt.Errorf("coze api error: status=%d, body=%s", resp.StatusCode, string(respBody))
	}

	// 解析响应
	var cozeResp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			ExecuteID string `json:"execute_id"`
			Status    string `json:"status"`
		} `json:"data"`
	}

	if err := common.Unmarshal(respBody, &cozeResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if cozeResp.Code != 0 {
		return nil, fmt.Errorf("coze api error: code=%d, msg=%s", cozeResp.Code, cozeResp.Msg)
	}

	return &dto.AdapterCreateResponse{
		RemoteProjectId: cozeResp.Data.ExecuteID,
		Status:          "COZE_RUNNING",
		Message:         "coze workflow started",
	}, nil
}

func (a *CozeAdapter) GetProjectStatus(ctx context.Context, remoteProjectId string) (*dto.AdapterStatusResponse, error) {
	if a.apiKey == "" {
		return nil, errors.New("coze api key not configured")
	}

	// 查询 Coze 工作流执行状态
	url := fmt.Sprintf("%s/v1/workflow/run/%s", a.baseURL, remoteProjectId)
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
		return nil, fmt.Errorf("coze api error: status=%d, body=%s", resp.StatusCode, string(respBody))
	}

	// 解析响应
	var cozeResp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Status string                 `json:"status"`
			Output map[string]interface{} `json:"output"`
		} `json:"data"`
	}

	if err := common.Unmarshal(respBody, &cozeResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if cozeResp.Code != 0 {
		return nil, fmt.Errorf("coze api error: code=%d, msg=%s", cozeResp.Code, cozeResp.Msg)
	}

	// 映射状态
	status := mapCozeStatus(cozeResp.Data.Status)

	return &dto.AdapterStatusResponse{
		Status:   status,
		Progress: cozeResp.Data.Status,
	}, nil
}

func (a *CozeAdapter) ValidateWebhook(ctx context.Context, signature string, body []byte) error {
	if a.webhookSecret == "" {
		return errors.New("coze webhook secret not configured")
	}

	// 计算 HMAC-SHA256 签名
	mac := hmac.New(sha256.New, []byte(a.webhookSecret))
	mac.Write(body)
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	if signature != expectedSignature {
		return errors.New("invalid webhook signature")
	}

	return nil
}

func (a *CozeAdapter) ParseWebhookPayload(body []byte) (*dto.WebhookPayload, error) {
	var cozePayload struct {
		ExecuteID string `json:"execute_id"`
		Status    string `json:"status"`
		Output    struct {
			MainImageUrl     string `json:"main_image_url"`
			MainImageAssetId string `json:"main_image_asset_id"`
			GeneratedResult  string `json:"generated_result"`
		} `json:"output"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := common.Unmarshal(body, &cozePayload); err != nil {
		return nil, fmt.Errorf("failed to parse coze webhook: %w", err)
	}

	// 映射到通用格式
	status := mapCozeStatus(cozePayload.Status)

	return &dto.WebhookPayload{
		RemoteProjectId:  cozePayload.ExecuteID,
		Status:           status,
		ErrorMsg:         cozePayload.Error.Message,
		MainImageUrl:     cozePayload.Output.MainImageUrl,
		MainImageAssetId: cozePayload.Output.MainImageAssetId,
		GeneratedResult:  cozePayload.Output.GeneratedResult,
	}, nil
}

// mapCozeStatus 映射 Coze 状态到内部状态
func mapCozeStatus(cozeStatus string) string {
	switch cozeStatus {
	case "running":
		return "COZE_RUNNING"
	case "succeeded":
		return "VIDEO_PROCESSING"
	case "failed":
		return "FAILED"
	default:
		return "COZE_RUNNING"
	}
}
