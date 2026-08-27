package operation_setting

import (
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

// 额度展示类型
const (
	QuotaDisplayTypeUSD    = "USD"
	QuotaDisplayTypeCNY    = "CNY"
	QuotaDisplayTypeTokens = "TOKENS"
	QuotaDisplayTypeCustom = "CUSTOM"
)

// BuiltinDefaultUserAgent 是未配置 default_user_agent 时使用的内置上游默认 User-Agent。
const BuiltinDefaultUserAgent = "hertz"

type GeneralSetting struct {
	DocsLink            string `json:"docs_link"`
	PingIntervalEnabled bool   `json:"ping_interval_enabled"`
	PingIntervalSeconds int    `json:"ping_interval_seconds"`
	// 当前站点额度展示类型：USD / CNY / TOKENS
	QuotaDisplayType string `json:"quota_display_type"`
	// 自定义货币符号，用于 CUSTOM 展示类型
	CustomCurrencySymbol string `json:"custom_currency_symbol"`
	// 自定义货币与美元汇率（1 USD = X Custom）
	CustomCurrencyExchangeRate float64 `json:"custom_currency_exchange_rate"`
	// 允许透传给上游的客户端 User-Agent 片段名单，多行文本，每行一个；
	// 入口请求 UA 含任一片段（子串匹配、忽略大小写）时，将原始 UA 透传给上游；
	// 留空表示全部不透传，沿用默认 User-Agent。
	UserAgentPassthrough string `json:"user_agent_passthrough"`
	// 向上游发送请求时的默认 User-Agent；留空表示使用内置默认值（hertz）。
	// 仅在渠道适配器未显式设置 UA 且入口 UA 未命中透传名单时生效。
	DefaultUserAgent string `json:"default_user_agent"`
}

// 默认配置
var generalSetting = GeneralSetting{
	DocsLink:                   "https://docs.newapi.pro",
	PingIntervalEnabled:        false,
	PingIntervalSeconds:        60,
	QuotaDisplayType:           QuotaDisplayTypeUSD,
	CustomCurrencySymbol:       "¤",
	CustomCurrencyExchangeRate: 1.0,
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("general_setting", &generalSetting)
}

func GetGeneralSetting() *GeneralSetting {
	return &generalSetting
}

// IsCurrencyDisplay 是否以货币形式展示（美元或人民币）
func IsCurrencyDisplay() bool {
	return generalSetting.QuotaDisplayType != QuotaDisplayTypeTokens
}

// IsCNYDisplay 是否以人民币展示
func IsCNYDisplay() bool {
	return generalSetting.QuotaDisplayType == QuotaDisplayTypeCNY
}

// GetQuotaDisplayType 返回额度展示类型
func GetQuotaDisplayType() string {
	return generalSetting.QuotaDisplayType
}

// GetCurrencySymbol 返回当前展示类型对应符号
func GetCurrencySymbol() string {
	switch generalSetting.QuotaDisplayType {
	case QuotaDisplayTypeUSD:
		return "$"
	case QuotaDisplayTypeCNY:
		return "¥"
	case QuotaDisplayTypeCustom:
		if generalSetting.CustomCurrencySymbol != "" {
			return generalSetting.CustomCurrencySymbol
		}
		return "¤"
	default:
		return ""
	}
}

// GetUsdToCurrencyRate 返回 1 USD = X <currency> 的 X（TOKENS 不适用）
func GetUsdToCurrencyRate(usdToCny float64) float64 {
	switch generalSetting.QuotaDisplayType {
	case QuotaDisplayTypeUSD:
		return 1
	case QuotaDisplayTypeCNY:
		return usdToCny
	case QuotaDisplayTypeCustom:
		if generalSetting.CustomCurrencyExchangeRate > 0 {
			return generalSetting.CustomCurrencyExchangeRate
		}
		return 1
	default:
		return 1
	}
}

// GetUserAgentPassthroughList 返回 UA 透传名单（按行拆分、去空白、过滤空行）。
func (g *GeneralSetting) GetUserAgentPassthroughList() []string {
	raw := strings.Split(g.UserAgentPassthrough, "\n")
	list := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		list = append(list, line)
	}
	return list
}

// ShouldPassthroughUserAgent 判断入口请求的 User-Agent 是否命中透传名单：
// 任一片段作为子串出现（忽略大小写）即命中；名单为空或 ua 为空时恒为 false。
func (g *GeneralSetting) ShouldPassthroughUserAgent(ua string) bool {
	ua = strings.TrimSpace(ua)
	if ua == "" {
		return false
	}
	uaLower := strings.ToLower(ua)
	for _, pattern := range g.GetUserAgentPassthroughList() {
		if strings.Contains(uaLower, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

// GetDefaultUserAgent 返回向上游发送的默认 User-Agent：
// 配置值去除首尾空白后非空则使用配置值，否则回退内置 BuiltinDefaultUserAgent。
func (g *GeneralSetting) GetDefaultUserAgent() string {
	ua := strings.TrimSpace(g.DefaultUserAgent)
	if ua == "" {
		return BuiltinDefaultUserAgent
	}
	return ua
}
