package service

import (
	"context"

	"github.com/QuantumNous/new-api/dto"
)

// VideoGenerationAdapter 视频生成渠道适配器接口
type VideoGenerationAdapter interface {
	// GetName 获取渠道名称
	GetName() string

	// CreateProject 创建视频项目并触发生成
	CreateProject(ctx context.Context, req *dto.CreateVideoProjectRequest) (*dto.AdapterCreateResponse, error)

	// GetProjectStatus 查询项目状态（某些渠道支持主动查询）
	GetProjectStatus(ctx context.Context, remoteProjectId string) (*dto.AdapterStatusResponse, error)

	// ValidateWebhook 验证 webhook 回调签名
	ValidateWebhook(ctx context.Context, signature string, body []byte) error

	// ParseWebhookPayload 解析 webhook 载荷
	ParseWebhookPayload(body []byte) (*dto.WebhookPayload, error)
}
