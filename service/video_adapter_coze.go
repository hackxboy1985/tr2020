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
	"github.com/QuantumNous/new-api/model"
)

// CozeAdapter Coze 渠道适配器
type CozeAdapter struct {
	ch     *model.VideoChannel
	client *http.Client
}

func NewCozeAdapter(ch *model.VideoChannel) *CozeAdapter {
	return &CozeAdapter{
		ch: ch,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (a *CozeAdapter) GetName() string {
	return "coze"
}

func (a *CozeAdapter) CreateProject(ctx context.Context, req *dto.CreateVideoProjectRequest) (*dto.AdapterCreateResponse, error) {
	if a.ch.ApiKey == "" || a.ch.WorkflowId == "" {
		return nil, errors.New("coze api key or workflow id not configured")
	}

	// 从 mediaList 拆分出 products / roles / others（Coze 工作流期望的格式）
	var products, roles, others []map[string]interface{}
	for _, m := range req.MediaList {
		switch m.MediaType {
		case "PRODUCT":
			products = append(products, map[string]interface{}{"url": m.MediaUrl})
		case "ROLE":
			item := map[string]interface{}{
				"url":   m.MediaUrl,
				"audio": "", // audio 可选，暂为空
			}
			if m.RoleName != "" {
				item["roleName"] = m.RoleName
			}
			roles = append(roles, item)
		default:
			others = append(others, map[string]interface{}{"url": m.MediaUrl})
		}
	}
	// 旧字段兜底
	if len(products) == 0 && req.ProductImgUrl != "" {
		products = append(products, map[string]interface{}{"url": req.ProductImgUrl})
	}

	// selectAudios：从旧字段解析（roles 的 audio 字段）
	var selectAudios []map[string]interface{}
	if req.SelectAudios != "" {
		var parsed []struct {
			URL    string `json:"url"`
			Remark string `json:"remark"`
		}
		if err := common.UnmarshalJsonStr(req.SelectAudios, &parsed); err == nil {
			for _, a := range parsed {
				selectAudios = append(selectAudios, map[string]interface{}{
					"url":    a.URL,
					"remark": a.Remark,
				})
			}
		}
	}

	cozeReq := map[string]interface{}{
		"workflow_id": a.ch.WorkflowId,
		"parameters": map[string]interface{}{
			"products":     products,
			"roles":        roles,
			"others":       others,
			"selectAudios": selectAudios,
			"brand":        req.Brand,
			"product":      req.ProductName,   // Coze 用 product 而非 productName
			"slogan":       req.Tagline,        // Coze 用 slogan 而非 tagline
			"points":     req.SellingPoints,
			"propmt":     req.Prompt,
			"vtype":      req.Vtype,
			"vtypeAdd":   req.VtypeAdd,
			"language":   req.Language,
			"platform":   req.Platform,
			"region":     req.Region,
			"time":       req.Duration, // Coze 工作流里叫 time，对应视频时长
			"resolution": req.Resolution,
			"model":      ApplyModelMapping(a.ch.ModelMapping, req.VideoModel),
			"whstr":      req.Whstr,
			"system":     "",
		},
	}

	body, err := common.Marshal(cozeReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

		requestURL := a.ch.GetCreateURL()
	common.SysLog("video adapter calling upstream: POST " + requestURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", requestURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+a.ch.ApiKey)
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
		Status:          model.VideoProjectStatusCozeRunning,
		Message:         "coze workflow started",
	}, nil
}

func (a *CozeAdapter) GetProjectStatus(ctx context.Context, remoteProjectId string) (*dto.AdapterStatusResponse, error) {
	if a.ch.ApiKey == "" {
		return nil, errors.New("coze api key not configured")
	}

	httpReq, err := http.NewRequestWithContext(ctx, "GET", a.ch.GetStatusQueryURL(remoteProjectId), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+a.ch.ApiKey)

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

	return &dto.AdapterStatusResponse{
		Status:   mapCozeStatus(cozeResp.Data.Status),
		Progress: cozeResp.Data.Status,
	}, nil
}

func (a *CozeAdapter) ValidateWebhook(ctx context.Context, signature string, body []byte) error {
	if a.ch.ApiSecret == "" {
		return errors.New("coze webhook secret not configured")
	}

	mac := hmac.New(sha256.New, []byte(a.ch.ApiSecret))
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

	return &dto.WebhookPayload{
		RemoteProjectId:  cozePayload.ExecuteID,
		Status:           mapCozeStatus(cozePayload.Status),
		ErrorMsg:         cozePayload.Error.Message,
		MainImageUrl:     cozePayload.Output.MainImageUrl,
		MainImageAssetId: cozePayload.Output.MainImageAssetId,
		GeneratedResult:  cozePayload.Output.GeneratedResult,
	}, nil
}

func mapCozeStatus(cozeStatus string) string {
	switch cozeStatus {
	case "running":
		return model.VideoProjectStatusCozeRunning
	case "succeeded":
		return model.VideoProjectStatusVideoProcessing
	case "failed":
		return model.VideoProjectStatusFailed
	default:
		return model.VideoProjectStatusCozeRunning
	}
}
