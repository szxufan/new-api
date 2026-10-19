package types

import "github.com/QuantumNous/new-api/constant"

// GetPreferredChannelTypesByRelayFormat 返回给定 RelayFormat 优先的渠道类型列表
// 返回列表按类型优先级排序，第一个元素为最优先类型。
// 如果 RelayFormat 不需要类型优先，返回 nil。
func GetPreferredChannelTypesByRelayFormat(relayFormat RelayFormat) []int {
	switch relayFormat {
	case RelayFormatClaude:
		// /v1/messages 优先 Anthropic 渠道
		return []int{constant.ChannelTypeAnthropic}
	case RelayFormatOpenAI:
		// /v1/chat/completions 及其兼容路径优先 OpenAI/Azure
		return []int{constant.ChannelTypeOpenAI, constant.ChannelTypeAzure}
	case RelayFormatOpenAIResponses, RelayFormatOpenAIResponsesCompaction:
		// /v1/responses 优先 OpenAI/Codex
		return []int{constant.ChannelTypeOpenAI, constant.ChannelTypeCodex}
	case RelayFormatGemini:
		// /v1beta/models/*path 和 /models/*path (Gemini 兼容)
		return []int{constant.ChannelTypeGemini}
	case RelayFormatOpenAIAudio, RelayFormatOpenAIImage:
		// 音频和图片相关
		return []int{constant.ChannelTypeOpenAI}
	case RelayFormatEmbedding:
		// 嵌入请求优先 OpenAI，回退 Jina
		return []int{constant.ChannelTypeOpenAI, constant.ChannelTypeJina}
	case RelayFormatRerank:
		// 重排序优先 Jina
		return []int{constant.ChannelTypeJina}
	case RelayFormatSystemOne:
		// System One 决策模型优先阿里云百炼 / Ollama
		return []int{constant.ChannelTypeAli, constant.ChannelTypeOllama}
	case RelayFormatOpenAIRealtime:
		// 实时通信
		return []int{constant.ChannelTypeOpenAI}
	case RelayFormatWebTools:
		// MCP 网页搜索/抓取工具，首期支持 DeepSeek 渠道
		return []int{constant.ChannelTypeDeepSeek}
	// 以下 RelayFormat 不参与类型优先选择：
	// - RelayFormatTask: 异步任务，通过 TaskPlatform 选择
	// - RelayFormatMjProxy: Midjourney 代理，专用渠道
	default:
		return nil
	}
}
