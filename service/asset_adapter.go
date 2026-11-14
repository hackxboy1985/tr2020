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
	ProjectName string `json:"ProjectName,omitempty"`
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
	Name        string `json:"Name,omitempty"`
	ProjectName string `json:"ProjectName,omitempty"`
}

// ===== 响应结构体（统一格式）=====

// AssetGroupResponse 素材组响应（统一格式）
type AssetGroupResponse struct {
	ID          string `json:"Id"`
	Name        string `json:"Name"`
	Description string `json:"Description"`
	GroupType   string `json:"GroupType"`
	ProjectName string `json:"ProjectName,omitempty"`
	CreateTime  string `json:"CreateTime,omitempty"`
	UpdateTime  string `json:"UpdateTime,omitempty"`
	RawData     string `json:"-"` // 原始响应数据，用于保存到数据库
}

// AssetResponse 素材响应（统一格式）
type AssetResponse struct {
	ID          string      `json:"Id"`
	GroupID     string      `json:"GroupId"`
	Name        string      `json:"Name"`
	AssetType   string      `json:"AssetType"`
	Status      string      `json:"Status"`
	SourceURL   string      `json:"SourceUrl,omitempty"`
	URL         string      `json:"URL,omitempty"`
	CreateTime  string      `json:"CreateTime,omitempty"`
	UpdateTime  string      `json:"UpdateTime,omitempty"`
	Moderation  interface{} `json:"Moderation,omitempty"`
	RawData     string      `json:"-"` // 原始响应数据
}

// ===== 真人核验（可选能力）=====

// FaceVerificationAdapter 是素材上游的「可选能力」接口。
//
// 支持真人核验会话的上游（当前为 n-axis）实现本接口；未实现的上游（老咪咕
// Gateway / KWJM / sdos2）保持 controller 里原有的网关透传逻辑，行为完全不变。
// controller 通过类型断言判断是否支持，避免改动老链路。
type FaceVerificationAdapter interface {
	// CreateFaceVerification 创建真人核验会话，callbackURL 可为空。
	CreateFaceVerification(callbackURL string) (*FaceVerificationSession, error)

	// GetFaceVerification 按会话凭证查询核验结果。
	GetFaceVerification(token string) (*FaceVerificationResult, error)
}

// FaceVerificationSession 核验会话（统一格式）
type FaceVerificationSession struct {
	Token     string // 会话凭证（n-axis 的 byted_token / BytedToken）
	H5URL     string // 火山官方 H5 活体认证链接
	ExpiresAt int64  // 过期时间（unix 秒），上游未给或解析失败为 0
	RawData   string // 原始响应，落库用于排查
}

// FaceVerificationResult 核验结果（统一格式）
type FaceVerificationResult struct {
	Status     string // waiting_user / callback_received / verified / failed / expired
	GroupID    string // verified 后的真人人像组 ID
	Passed     bool
	AssetID    string
	Reason     string
	ExpireTime string // 上游原始 ISO 时间（官方 TOP 响应字段，透传给下游）
	RawData    string
}

