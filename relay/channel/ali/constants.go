package ali

var ModelList = []string{
	"qwen-turbo",
	"qwen-plus",
	"qwen-max",
	"qwen-max-longcontext",
	"qwq-32b",
	"qwen3-235b-a22b",
	// 全模态（非实时，Chat Completions / Responses）
	"qwen3.8-omni-flash",
	"qwen3.5-omni-plus",
	"qwen3.5-omni-flash",
	// 全模态实时（WebSocket /v1/realtime）
	"qwen3.8-omni-flash-realtime",
	"qwen3.5-omni-plus-realtime",
	"qwen3.5-omni-flash-realtime",
	"text-embedding-v1",
	"gte-rerank-v2",
	"qwen3-tts-flash",
	"qwen3-tts-instruct-flash",
	"qwen-tts",
	"qwen-audio-3.0-tts-flash",
	"cosyvoice-v3-flash",
	"cosyvoice-v2",
	// Sambert（音色即模型，其余 sambert-* 音色可手动添加）
	"sambert-zhichu-v1",
	"sambert-zhiying-v1",
	"sambert-zhimiao-emo-v1",
	"qwen3-asr-flash",
	"fun-asr-flash",
	"qwen-audio-3.0-asr-flash",
	"decision-model-preview",
}

var ChannelName = "ali"
