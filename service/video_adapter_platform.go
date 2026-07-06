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
	"github.com/QuantumNous/new-api/model"
)

// PlatformAdapter 三方平台适配器
type PlatformAdapter struct {
	ch     *model.VideoChannel
	client *http.Client
}

func NewPlatformAdapter(ch *model.VideoChannel) *PlatformAdapter {
	return &PlatformAdapter{
		ch: ch,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (a *PlatformAdapter) GetName() string {
	return "platform"
}

func (a *PlatformAdapter) CreateProject(ctx context.Context, req *dto.CreateVideoProjectRequest) (*dto.AdapterCreateResponse, error) {
	if a.ch.BaseURL == "" || a.ch.ApiKey == "" {
		return nil, errors.New("platform base url or api key not configured")
	}

	// 构建 mediaList：优先使用 req.MediaList，否则从旧字段自动转换
	mediaList := req.MediaList
	if len(mediaList) == 0 && req.ProductImgUrl != "" {
		mediaList = append(mediaList, dto.VideoMediaItem{
			MediaType: "PRODUCT",
			MediaUrl:  req.ProductImgUrl,
		})
		// 旧格式 roles 字段转换：[{name, url}] → ROLE 类型
		if req.Roles != "" {
			var roles []struct {
				Name string `json:"name"`
				URL  string `json:"url"`
			}
			if err := common.UnmarshalJsonStr(req.Roles, &roles); err == nil {
				for i, r := range roles {
					if r.URL != "" {
						mediaList = append(mediaList, dto.VideoMediaItem{
							MediaType: "ROLE",
							MediaUrl:  r.URL,
							RoleName:  r.Name,
							SortOrder: i,
						})
					}
				}
			}
		}
	}

	// 应用模型映射
	videoModel := req.VideoModel
	if mapped, _ := ApplyModelMapping(a.ch.ModelMapping, req.VideoModel); mapped != "" {
		videoModel = mapped
	}

	// 构建 camelCase 请求体以兼容 OpenAPI 服务端（Spring Boot Jackson 默认 camelCase）
	platformReq := map[string]interface{}{
		"productName":   req.ProductName,
		"brand":         req.Brand,
		"tagline":       req.Tagline,
		"sellingPoints": req.SellingPoints,
		"prompt":        req.Prompt,
		"vtype":         req.Vtype,
		"vtypeAdd":      req.VtypeAdd,
		"language":      req.Language,
		"platform":      req.Platform,
		"region":        req.Region,
		"duration":      req.Duration,
		"resolution":    req.Resolution,
		"videoModel":    videoModel,
		"whstr":         req.Whstr,
		"mediaList":     mediaList,
	}

	body, err := common.Marshal(platformReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	requestURL := a.ch.GetCreateURL()
	common.SysLog(fmt.Sprintf("video adapter calling upstream: POST %s body=%s", requestURL, string(body)))
	httpReq, err := http.NewRequestWithContext(ctx, "POST", requestURL, bytes.NewReader(body))
	if err != nil {
		return &dto.AdapterCreateResponse{RawRequest: body}, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+a.ch.ApiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return &dto.AdapterCreateResponse{RawRequest: body}, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return &dto.AdapterCreateResponse{RawRequest: body}, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return &dto.AdapterCreateResponse{RawRequest: body, RawResponse: respBody}, fmt.Errorf("platform api error: status=%d, body=%s", resp.StatusCode, string(respBody))
	}

	var platformResp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			// OpenAPI 返回 taskId（数字），兼容 project_id（字符串）两种格式
			TaskId      int64  `json:"taskId"`
			ProjectId   string `json:"project_id"`
			ProjectName string `json:"project_name"`
			Status      string `json:"status"`
		} `json:"data"`
	}

	if err := common.Unmarshal(respBody, &platformResp); err != nil {
		return &dto.AdapterCreateResponse{RawRequest: body, RawResponse: respBody}, fmt.Errorf("failed to parse response: %w", err)
	}

	if platformResp.Code != 200 {
		return &dto.AdapterCreateResponse{RawRequest: body, RawResponse: respBody}, fmt.Errorf("platform api error: code=%d, msg=%s", platformResp.Code, platformResp.Msg)
	}

	// taskId 优先（OpenAPI），fallback 到 project_id（旧格式）
	remoteId := platformResp.Data.ProjectId
	if platformResp.Data.TaskId > 0 {
		remoteId = fmt.Sprintf("%d", platformResp.Data.TaskId)
	}

	return &dto.AdapterCreateResponse{
		RemoteProjectId: remoteId,
		Status:          platformResp.Data.Status,
		Message:         platformResp.Msg,
		RawRequest:      body,
		RawResponse:     respBody,
	}, nil
}

func (a *PlatformAdapter) GetProjectStatus(ctx context.Context, remoteProjectId string) (*dto.AdapterStatusResponse, error) {
	if a.ch.BaseURL == "" || a.ch.ApiKey == "" {
		return nil, errors.New("platform base url or api key not configured")
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
		return nil, fmt.Errorf("platform api error: status=%d, body=%s", resp.StatusCode, string(respBody))
	}

	var platformResp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Status string `json:"status"`
			// camelCase（OpenAPI）
			VideoUrl string `json:"videoUrl"`
			ErrorMsg string `json:"errorMsg"`
			// snake_case（旧格式兼容）
			ErrorMsgSnake    string `json:"error_msg"`
			Progress         string `json:"progress"`
			MainImageUrl     string `json:"main_image_url"`
			MainImageAssetId string `json:"main_image_asset_id"`
			GeneratedResult  string `json:"generated_result"`
			FirstVideoUrl    string `json:"first_video_url"`
			// 积分结算字段
			CreditAmount int     `json:"creditAmount"`
			CreditRefund int     `json:"creditRefund"`
			CreditNet    int     `json:"creditNet"`
			// 金额结算字段
			MoneyAmount  float64 `json:"moneyAmount"`
			MoneyRefund  float64 `json:"moneyRefund"`
			MoneyNet     float64 `json:"moneyNet"`
		} `json:"data"`
	}

	if err := common.Unmarshal(respBody, &platformResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if platformResp.Code != 200 {
		return nil, fmt.Errorf("platform api error: code=%d, msg=%s", platformResp.Code, platformResp.Msg)
	}

	// camelCase 优先，fallback 到 snake_case
	errorMsg := platformResp.Data.ErrorMsg
	if errorMsg == "" {
		errorMsg = platformResp.Data.ErrorMsgSnake
	}
	firstVideoUrl := platformResp.Data.VideoUrl
	if firstVideoUrl == "" {
		firstVideoUrl = platformResp.Data.FirstVideoUrl
	}

	return &dto.AdapterStatusResponse{
		Status:           platformResp.Data.Status,
		ErrorMsg:         errorMsg,
		Progress:         platformResp.Data.Progress,
		MainImageUrl:     platformResp.Data.MainImageUrl,
		MainImageAssetId: platformResp.Data.MainImageAssetId,
		GeneratedResult:  platformResp.Data.GeneratedResult,
		FirstVideoUrl:    firstVideoUrl,
		CreditAmount:     platformResp.Data.CreditAmount,
		CreditRefund:     platformResp.Data.CreditRefund,
		CreditNet:        platformResp.Data.CreditNet,
		MoneyAmount:      platformResp.Data.MoneyAmount,
		MoneyRefund:      platformResp.Data.MoneyRefund,
		MoneyNet:         platformResp.Data.MoneyNet,
		RawResponse:      respBody,
	}, nil
}

func (a *PlatformAdapter) ValidateWebhook(ctx context.Context, signature string, body []byte) error {
	if a.ch.ApiSecret == "" {
		// 未配置 secret 时跳过签名验证
		return nil
	}
	// TODO: 根据实际平台文档实现签名验证
	return nil
}

func (a *PlatformAdapter) ParseWebhookPayload(body []byte) (*dto.WebhookPayload, error) {
	var payload dto.WebhookPayload
	if err := common.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse platform webhook: %w", err)
	}
	return &payload, nil
}
