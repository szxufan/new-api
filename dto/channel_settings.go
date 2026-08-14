package dto

type ChannelSettings struct {
	ForceFormat            bool               `json:"force_format,omitempty"`
	ThinkingToContent      bool               `json:"thinking_to_content,omitempty"`
	Proxy                  string             `json:"proxy"`
	PassThroughBodyEnabled bool               `json:"pass_through_body_enabled,omitempty"`
	SystemPrompt           string             `json:"system_prompt,omitempty"`
	SystemPromptOverride   bool               `json:"system_prompt_override,omitempty"`
	ResponseDetection      *ResponseDetection `json:"response_detection,omitempty"`
	AntiCacheTest          bool               `json:"anti_cache_test,omitempty"`          // 渠道测试时在提示词附加当前时间，防止上游缓存命中
	AntiCacheRetryEnabled  bool               `json:"anti_cache_retry_enabled,omitempty"` // 重试时在最后一条消息追加内容，避免命中上游错误缓存
	AntiCacheRetryContent  string             `json:"anti_cache_retry_content,omitempty"` // 重试追加的内容，第 N 次重试追加 N 个
}

// ResponseDetection 响应内容检测配置，检测到关键词后可自动重试
type ResponseDetection struct {
	Enabled         bool     `json:"enabled,omitempty"`            // 是否启用响应内容检测
	Keywords        []string `json:"keywords,omitempty"`           // 检测关键词列表（不区分大小写）
	MaxRetries      int      `json:"max_retries,omitempty"`        // 检测命中后最大重试次数（0=使用全局重试次数）
	OnHit           string   `json:"on_hit,omitempty"`             // 命中后行为: "retry"(默认) | "abort"
	TreatEmptyAsHit bool     `json:"treat_empty_as_hit,omitempty"` // 是否将空回复（trim 后内容为空且无工具调用）视为检测命中
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
}

func (s *ChannelOtherSettings) IsOpenRouterEnterprise() bool {
	if s == nil || s.OpenRouterEnterprise == nil {
		return false
	}
	return *s.OpenRouterEnterprise
}
