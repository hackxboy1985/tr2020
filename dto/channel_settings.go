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
	// Poster 渠道路径覆盖
	// PosterApiVersion: 替换默认路径中的版本号，如 "v2" 将 /openapi/v1/... 改为 /openapi/v2/...
	// PosterEndpoints: 按模型精确覆盖完整路径，优先级高于 PosterApiVersion
	//   示例: {"poster-matting": "/openapi/v2/ai/matting_pro"}
	PosterApiVersion string            `json:"poster_api_version,omitempty"`
	PosterEndpoints  map[string]string `json:"poster_endpoints,omitempty"`
	// RR 渠道配置
	// RREndpoints: 按模型配置完整上游路径，key 为模型名，value 为完整路径
	//   示例: {"rhart-image-g-2-official": "/openapi/v2/rhart-image-g-2-official/text-to-image"}
	// RRUrlTTLHours: 上游图片 URL 有效期（小时），默认 24
	// RRUrlProxyBaseURL: 对外图片代理基础地址，如 https://img.example.com/oss。配置后替换上游 URL 的 scheme/host，并将配置路径作为前缀，原路径和 query 保持不变。
	RREndpoints       map[string]string `json:"rr_endpoints,omitempty"`
	RRUrlTTLHours     int               `json:"rr_url_ttl_hours,omitempty"`
	RRUrlProxyBaseURL string            `json:"rr_url_proxy_base_url,omitempty"`
	// Td 渠道配置
	// TdUrlTTLHours: 上游图片 URL 有效期（小时），默认 24
	// TdUrlProxyBaseURL: 对外图片代理基础地址，配置后替换上游 URL 的 scheme/host，原路径和 query 保持不变。
	TdUrlTTLHours     int    `json:"td_url_ttl_hours,omitempty"`
	TdUrlProxyBaseURL string `json:"td_url_proxy_base_url,omitempty"`
}

func (s *ChannelOtherSettings) IsOpenRouterEnterprise() bool {
	if s == nil || s.OpenRouterEnterprise == nil {
		return false
	}
	return *s.OpenRouterEnterprise
}
