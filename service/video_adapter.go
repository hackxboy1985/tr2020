package service

import (
	"context"
	"fmt"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/common"
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


// ApplyModelMapping 应用模型映射。如果 mapping 中配置了 userVideoModel 对应的上游值，则返回映射值；否则返回原值。
func ApplyModelMapping(mappingJSON string, userVideoModel string) string {
	if mappingJSON == "" {
		return userVideoModel
	}
	var mapping map[string]string
	if err := common.Unmarshal([]byte(mappingJSON), &mapping); err != nil {
		return userVideoModel
	}
	if mapped, ok := mapping[userVideoModel]; ok && mapped != "" {
		return mapped
	}
	return userVideoModel
}
