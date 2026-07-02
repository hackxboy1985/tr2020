package service

import (
	"context"
	"fmt"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

// VideoGenerationAdapter 视频生成渠道适配器接口
type VideoGenerationAdapter interface {
	GetName() string
	CreateProject(ctx context.Context, req *dto.CreateVideoProjectRequest) (*dto.AdapterCreateResponse, error)
	GetProjectStatus(ctx context.Context, remoteProjectId string) (*dto.AdapterStatusResponse, error)
	ValidateWebhook(ctx context.Context, signature string, body []byte) error
	ParseWebhookPayload(body []byte) (*dto.WebhookPayload, error)
}

// NewAdapterFromChannel 根据 VideoChannel 创建对应适配器
func NewAdapterFromChannel(ch *model.VideoChannel) (VideoGenerationAdapter, error) {
	switch ch.ChannelType {
	case "coze":
		return NewCozeAdapter(ch), nil
	case "platform":
		return NewPlatformAdapter(ch), nil
	default:
		return nil, fmt.Errorf("unsupported channel type: %s", ch.ChannelType)
	}
}
