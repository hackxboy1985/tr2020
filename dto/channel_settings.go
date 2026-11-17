package dto

type ChannelSettings struct {
	ForceFormat            bool   `json:"force_format,omitempty"`
	ThinkingToContent      bool   `json:"thinking_to_content,omitempty"`
	Proxy                  string `json:"proxy"`
	PassThroughBodyEnabled bool   `json:"pass_through_body_enabled,omitempty"`
	SystemPrompt           string `json:"system_prompt,omitempty"`
	SystemPromptOverride   bool   `json:"system_prompt_override,omitempty"`
}

type VertexKeyType string

const (
	VertexKeyTypeJSON   VertexKeyType = "json"
	VertexKeyTypeAPIKey VertexKeyType = "api_key"
)

type AwsKeyType string

const (
	AwsKeyTypeAKSK   AwsKeyType = "ak_sk" // 默认
	AwsKeyTypeApiKey AwsKeyType = "api_key"
)

type ChannelOtherSettings struct {
	AzureResponsesVersion                 string        `json:"azure_responses_version,omitempty"`
	VertexKeyType                         VertexKeyType `json:"vertex_key_type,omitempty"` // "json" or "api_key"
	OpenRouterEnterprise                  *bool         `json:"openrouter_enterprise,omitempty"`
	ClaudeBetaQuery                       bool          `json:"claude_beta_query,omitempty"`         // Claude 渠道是否强制追加 ?beta=true
	AllowServiceTier                      bool          `json:"allow_service_tier,omitempty"`        // 是否允许 service_tier 透传（默认过滤以避免额外计费）
	AllowInferenceGeo                     bool          `json:"allow_inference_geo,omitempty"`       // 是否允许 inference_geo 透传（仅 Claude，默认过滤以满足数据驻留合规
	AllowSpeed                            bool          `json:"allow_speed,omitempty"`               // 是否允许 speed 透传（仅 Claude，默认过滤以避免意外切换推理速度模式）
	AllowSafetyIdentifier                 bool          `json:"allow_safety_identifier,omitempty"`   // 是否允许 safety_identifier 透传（默认过滤以保护用户隐私）
	DisableStore                          bool          `json:"disable_store,omitempty"`             // 是否禁用 store 透传（默认允许透传，禁用后可能导致 Codex 无法使用）
	AllowIncludeObfuscation               bool          `json:"allow_include_obfuscation,omitempty"` // 是否允许 stream_options.include_obfuscation 透传（默认过滤以避免关闭流混淆保护）
	AwsKeyType                            AwsKeyType    `json:"aws_key_type,omitempty"`
	UpstreamModelUpdateCheckEnabled       bool          `json:"upstream_model_update_check_enabled,omitempty"`        // 是否检测上游模型更新
	UpstreamModelUpdateAutoSyncEnabled    bool          `json:"upstream_model_update_auto_sync_enabled,omitempty"`    // 是否自动同步上游模型更新
	UpstreamModelUpdateLastCheckTime      int64         `json:"upstream_model_update_last_check_time,omitempty"`      // 上次检测时间
	UpstreamModelUpdateLastDetectedModels []string      `json:"upstream_model_update_last_detected_models,omitempty"` // 上次检测到的可加入模型
	UpstreamModelUpdateLastRemovedModels  []string      `json:"upstream_model_update_last_removed_models,omitempty"`  // 上次检测到的可删除模型
	UpstreamModelUpdateIgnoredModels      []string      `json:"upstream_model_update_ignored_models,omitempty"`       // 手动忽略的模型
	// DoubaoVideo 自定义上游路径，对接 new-api 中转站时填写，留空使用默认 Ark 路径
	// 生成接口: 默认 /api/v3/contents/generations/tasks，中转填 /v1/video/generations
	// 查询接口: 默认 /api/v3/contents/generations/tasks，中转填 /v1/videos （后面自动拼 /{task_id}）
	DoubaoVideoGeneratePath string `json:"doubao_video_generate_path,omitempty"`
	DoubaoVideoFetchPath    string `json:"doubao_video_fetch_path,omitempty"`
	// Seedance Gateway 素材库/人脸认证服务地址，例如 https://sd.dawnloadai.com:9444
	SeedanceAssetBaseUrl string `json:"seedance_asset_base_url,omitempty"`
	// SeedanceRelayMode 为 true 时，素材接口调用下游 new-api 路径（/api/seedance/assets 等）
	// 为 false（默认）时，调用 Seedance Gateway 原生路径（/api/seedance/proxy/assets 等）
	SeedanceRelayMode bool `json:"seedance_relay_mode,omitempty"`
	// SeedanceAssetAPIFormat 指定素材接口的上游 API 格式
	// "gatewayMg": 咪咕 Gateway RESTful 格式（默认）- /api/seedance/proxy/assets/groups
	// "official": 火山官方 Action 格式 - /?Action=CreateAssetGroup&Version=2024-01-01
	SeedanceAssetAPIFormat string `json:"seedance_asset_api_format,omitempty"`
	// KWJM 素材库配置（新增）
	KwjmAssetBaseUrl string `json:"kwjm_asset_base_url,omitempty"` // KWJM 基础URL，例如 https://kwjm.com
	KwjmAssetModel   string `json:"kwjm_asset_model,omitempty"`    // KWJM 默认模型，例如 sd-video-v2
	// AssetUpstreamVersion 上游版本选择
	//   "gateway"（默认）：咪咕 Gateway RESTful 契约 /api/seedance/proxy/assets*
	//   "kwjm"：KWJM Action 契约
	//   "sdos2"：seedance-gateway 原生契约 /seedance/media*（小写蛇形字段）
	//   "naxis"：n-axis（咪咕 Gateway 重构版）火山方舟原生 Action 入口
	//            POST /?Action=…&Version=2024-01-01，与官方 SDK 同构
	//   "naxis_rest"：同一上游的平台 REST 入口 /v1/asset-groups、/v1/assets
	//            需要素材原始 source_url 时使用（Action 入口返回的是平台签名代理地址）
	//            两者地址均复用 seedance_asset_base_url（如 https://platform.n-axis.com）
	AssetUpstreamVersion string `json:"asset_upstream_version,omitempty"`
	// Sdos2VideoContentPublic 控制 sdos2 成片下载接口是否免鉴权
	// false（默认）：GET /v1/videos/{task_id}/content 需要 API Key 授权
	// true：该地址可直接访问，无需 Authorization 头
	Sdos2VideoContentPublic bool `json:"sdos2_video_content_public,omitempty"`
	// AssetAutoSync 素材库跨渠道自动补登记（默认 true）
	// 换渠道后旧素材在当前渠道没有绑定时，是否按 source_url 自动重新登记（保留组结构）
	// 未配置（nil）时视为开启；只有显式 false 才关闭
	AssetAutoSync *bool `json:"asset_auto_sync,omitempty"`
	// AllowNonOfficialVideoURL 是否允许上游返回非官链成片地址（默认 false）
	// false：空值 / 相对路径拼不出绝对地址 / 非 http(s) → 记 error 日志 + 任务置失败 + fail_code=INVALID_VIDEO_URL
	// true ：接受该地址（相对路径用渠道 Base URL 拼），下游看到的 URL 换成本服务的
	//        /v1/videos/{task_id}/content，由服务端带上游 Key 代理拉流
	AllowNonOfficialVideoURL bool `json:"allow_non_official_video_url,omitempty"`
	// Sdos2RelayBaseUrl 本服务对外基址，用于拼接返回给下游的成片下载地址
	// 例如 https://api.example.com；留空时回退到系统设置里的 ServerAddress
	Sdos2RelayBaseUrl string `json:"sdos2_relay_base_url,omitempty"`
	// DoubaoVideoAlwaysFetchUpstream 是否强制实时查询上游（ARK 格式）
	// true: 每次查询都调用上游（实时获取最新状态）
	// false（默认）: 仅当 task.Data 不完整时才调用上游
	DoubaoVideoAlwaysFetchUpstream bool `json:"doubao_video_always_fetch_upstream,omitempty"`
	// OpenAIVideoAlwaysFetchUpstream 是否强制实时查询上游（OpenAI Video API 格式）
	// true: 每次查询都调用上游（实时获取最新状态）
	// false（默认）: 永不调用上游，总是读取数据库
	OpenAIVideoAlwaysFetchUpstream bool `json:"openai_video_always_fetch_upstream,omitempty"`
	// Poster 渠道路径覆盖
	// PosterApiVersion: 替换默认路径中的版本号，如 "v2" 将 /openapi/v1/... 改为 /openapi/v2/...
	// PosterEndpoints: 按模型精确覆盖完整路径，优先级高于 PosterApiVersion
	//   示例: {"poster-matting": "/openapi/v2/ai/matting_pro"}
	PosterApiVersion string            `json:"poster_api_version,omitempty"`
	PosterEndpoints  map[string]string `json:"poster_endpoints,omitempty"`
}

// AssetAutoSyncEnabled 素材跨渠道自动补登记是否开启。
// 未配置（nil）时默认开启；显式配置 false 时才关闭。
func (s ChannelOtherSettings) AssetAutoSyncEnabled() bool {
	if s.AssetAutoSync == nil {
		return true
	}
	return *s.AssetAutoSync
}

func (s *ChannelOtherSettings) IsOpenRouterEnterprise() bool {
	if s == nil || s.OpenRouterEnterprise == nil {
		return false
	}
	return *s.OpenRouterEnterprise
}
