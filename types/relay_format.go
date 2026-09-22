package types

type RelayFormat string

const (
	RelayFormatOpenAI                    RelayFormat = "openai"
	RelayFormatClaude                                = "claude"
	RelayFormatGemini                                = "gemini"
	RelayFormatOpenAIResponses                       = "openai_responses"
	RelayFormatOpenAIResponsesCompaction             = "openai_responses_compaction"
	RelayFormatOpenAIAudio                           = "openai_audio"
	RelayFormatOpenAIImage                           = "openai_image"
	RelayFormatOpenAIRealtime                        = "openai_realtime"
	RelayFormatRerank                                = "rerank"
	RelayFormatEmbedding                             = "embedding"

	RelayFormatTask    = "task"
	RelayFormatMjProxy = "mj_proxy"

	// 渠道私有协议格式：客户端中继格式转换到渠道自有线上协议时登记，
	// 用于日志展示请求转换链（other.request_conversion）。
	RelayFormatOllama RelayFormat = "ollama"
	RelayFormatDify   RelayFormat = "dify"
	RelayFormatCoze   RelayFormat = "coze"
)
