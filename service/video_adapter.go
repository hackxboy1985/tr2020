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


// ApplyModelMapping 应用模型映射。
// 返回值：
//   - mapped: 映射后的模型名
//   - hasMapping: 渠道是否配置了 ModelMapping
//   - matched: 传入的 userVideoModel 是否在 mapping 的 key 中
func ApplyModelMapping(mappingJSON string, userVideoModel string) (mapped string, hasMapping bool, matched bool) {
	if mappingJSON == "" {
		return userVideoModel, false, false
	}
	var mapping map[string]string
	if err := common.Unmarshal([]byte(mappingJSON), &mapping); err != nil {
		return userVideoModel, false, false
	}
	if len(mapping) == 0 {
		return userVideoModel, false, false
	}
	if v, ok := mapping[userVideoModel]; ok && v != "" {
		return v, true, true
	}
	return userVideoModel, true, false // 有映射配置但未匹配到
}
