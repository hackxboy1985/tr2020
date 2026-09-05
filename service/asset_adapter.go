package service

import (
	"net/url"
)

// AssetAdapter 素材管理适配器接口
// 不同的上游（Gateway、KWJM）实现各自的适配器
type AssetAdapter interface {
	// CreateAssetGroup 创建素材组
	CreateAssetGroup(req CreateAssetGroupRequest) (*AssetGroupResponse, error)

	// GetAssetGroup 查询素材组
	GetAssetGroup(groupID string) (*AssetGroupResponse, error)

	// UpdateAssetGroup 更新素材组
	UpdateAssetGroup(groupID string, req UpdateAssetGroupRequest) error

	// DeleteAssetGroup 删除素材组
	DeleteAssetGroup(groupID string) error

	// CreateAsset 创建素材
	CreateAsset(req CreateAssetRequest) (*AssetResponse, error)

	// GetAsset 查询素材
	GetAsset(assetID string, query url.Values) (*AssetResponse, error)

	// UpdateAsset 更新素材
	UpdateAsset(assetID string, req UpdateAssetRequest) error

	// DeleteAsset 删除素材
	DeleteAsset(assetID string) error
}

// ===== 请求结构体 =====

// CreateAssetGroupRequest 创建素材组请求
type CreateAssetGroupRequest struct {
	Name        string `json:"Name"`
	Description string `json:"Description"`
	GroupType   string `json:"GroupType"`
	ProjectName string `json:"ProjectName,omitempty"`
}

// UpdateAssetGroupRequest 更新素材组请求
type UpdateAssetGroupRequest struct {
	Name        string `json:"Name,omitempty"`
	Description string `json:"Description,omitempty"`
}

// CreateAssetRequest 创建素材请求
type CreateAssetRequest struct {
	GroupID     string `json:"GroupId"`
	URL         string `json:"URL"`
	AssetType   string `json:"AssetType"`
	Name        string `json:"Name"`
	ProjectName string `json:"ProjectName,omitempty"`
}

// UpdateAssetRequest 更新素材请求
type UpdateAssetRequest struct {
	Name string `json:"Name,omitempty"`
}

// ===== 响应结构体（统一格式）=====

// AssetGroupResponse 素材组响应（统一格式）
type AssetGroupResponse struct {
	ID          string `json:"Id"`
	Name        string `json:"Name"`
	Description string `json:"Description"`
	GroupType   string `json:"GroupType"`
	RawData     string `json:"-"` // 原始响应数据，用于保存到数据库
}

// AssetResponse 素材响应（统一格式）
type AssetResponse struct {
	ID          string `json:"Id"`
	GroupID     string `json:"GroupId"`
	Name        string `json:"Name"`
	AssetType   string `json:"AssetType"`
	Status      string `json:"Status"`
	SourceURL   string `json:"SourceUrl,omitempty"`
	RawData     string `json:"-"` // 原始响应数据
}
