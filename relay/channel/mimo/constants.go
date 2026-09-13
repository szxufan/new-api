package mimo

var ModelList = []string{
	"mimo-v2.5-asr",
	"mimo-v2.5-tts",
	"mimo-v2.5-tts-voicedesign",
	"mimo-v2.5-tts-voiceclone",
}

var ChannelName = "Xiaomi MiMo"

// MiMo 的 ASR/TTS 通过 OpenAI 兼容的 chat/completions 接口提供，
// 音频以 base64 形式嵌入请求与响应。
const mimoDefaultVoice = "mimo_default"
