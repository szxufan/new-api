package ali

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

const (
	// realtimeAliRealtimePath 是业务空间绑定域名的默认实时端点路径，
	// 运营者在渠道 base_url 中自带路径时则尊重其配置（兼容历史 inference 端点）。
	realtimeAliRealtimePath = "/api-ws/v1/realtime"
	// realtimeOpenAIPcm16 是 OpenAI Realtime 协议的 16-bit PCM 音频格式标识（24kHz）。
	realtimeOpenAIPcm16 = "pcm16"
	// realtimeAliTranscriptionModel 用于替换 OpenAI 协议客户端默认的输入转写模型。
	realtimeAliTranscriptionModel = "qwen3-asr-flash-realtime"
)

// buildAliRealtimeURL 由渠道 base_url 构造百炼 Realtime WebSocket 地址。
// 约定 base_url 形如 https://{WorkspaceId}.cn-beijing.maas.aliyuncs.com（北京/新加坡地域），
// 亦可直接填写 wss:// 地址；路径为空（或仅有 "/"）时补默认实时端点，自带路径则保留。
func buildAliRealtimeURL(channelBaseURL string, model string) (string, error) {
	trimmed := strings.TrimSpace(channelBaseURL)
	if trimmed == "" {
		return "", errors.New("channel base url is empty")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid channel base url %q: %w", channelBaseURL, err)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("channel base url %q is missing host", channelBaseURL)
	}
	switch parsed.Scheme {
	case "https":
		parsed.Scheme = "wss"
	case "http":
		parsed.Scheme = "ws"
	case "wss", "ws":
		// 已是 WebSocket scheme，保持不变
	default:
		return "", fmt.Errorf("unsupported scheme %q in channel base url %q", parsed.Scheme, channelBaseURL)
	}
	// 运营者在 base_url 末尾带 "/" 时视为未配置路径，避免拼出 wss://host/?model=...
	if parsed.Path == "" || parsed.Path == "/" {
		parsed.Path = realtimeAliRealtimePath
	}
	query := parsed.Query()
	query.Set("model", model)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

// AliRealtimeHandler 中继百炼 Qwen-Omni-Realtime 的双向事件流：
// 计数与计费沿用 RealtimePump 的通用逻辑（response.done 中的上游 usage 优先），
// 仅在转发时按 schema 差异做字段适配。
func AliRealtimeHandler(c *gin.Context, info *relaycommon.RelayInfo) (*types.NewAPIError, *dto.RealtimeUsage) {
	return openai.RealtimePump(c, info, transformClientEvent, transformUpstreamEvent)
}

// ---- 客户端 → 上游（OpenAI → 百炼）----

// transformClientEvent 仅改写需要适配的事件；其余事件原样转发。
func transformClientEvent(c *gin.Context, info *relaycommon.RelayInfo, message []byte) ([]string, error) {
	event, ok := decodeEventMap(message)
	if !ok {
		return []string{string(message)}, nil
	}
	switch eventType(event) {
	case dto.RealtimeEventTypeSessionUpdate:
		return adaptClientSessionUpdate(message, event)
	case dto.RealtimeEventTypeConversationCreate:
		return adaptClientConversationItemCreate(c, message, event)
	case dto.RealtimeEventTypeResponseCreate:
		return adaptClientResponseCreate(c, message, event)
	default:
		return []string{string(message)}, nil
	}
}

// adaptClientSessionUpdate 将 OpenAI 会话配置字段转换为百炼 schema：
//   - input_audio_format/output_audio_format 的 "pcm16" 转换为嵌套 audio.input/output.format
//     （OpenAI pcm16 为 24kHz，百炼 qwen3.5 realtime 支持 8k/16k/24k/48k，qwen3.8 单声道默认 16k）；
//   - max_response_output_tokens（兼容 max_output_tokens）重命名为 max_tokens；
//   - OpenAI 侧的输入转写模型替换为 qwen3-asr-flash-realtime。
//
// 客户端直接发送百炼原生嵌套 audio 字段时不做音频格式转换（双 schema 兼容）。
func adaptClientSessionUpdate(message []byte, event map[string]any) ([]string, error) {
	session, _ := event["session"].(map[string]any)
	if session == nil {
		return []string{string(message)}, nil
	}
	adapted := false

	if _, hasNativeAudio := session["audio"]; !hasNativeAudio {
		audio := map[string]any{}
		if format, ok := session["input_audio_format"].(string); ok && format == realtimeOpenAIPcm16 {
			delete(session, "input_audio_format")
			audio["input"] = map[string]any{"format": map[string]any{
				"type": "pcm", "sample_rate": 24000, "sample_format": "s16le",
				"channels": 1, "packing": "interleaved", "channel_layout": "mono",
			}}
			adapted = true
		}
		if format, ok := session["output_audio_format"].(string); ok && format == realtimeOpenAIPcm16 {
			delete(session, "output_audio_format")
			audio["output"] = map[string]any{"format": map[string]any{
				"type": "pcm", "sample_rate": 24000,
			}}
			adapted = true
		}
		if len(audio) > 0 {
			session["audio"] = audio
		}
	}

	// OpenAI Realtime 的会话字段为 max_response_output_tokens（数值或 "inf"），
	// 兼容早期按 Responses API 习惯误用的 max_output_tokens；百炼对应字段为整数 max_tokens。
	// "inf" 等非数值表示不限制输出，直接丢弃（沿用百炼默认）。
	for _, field := range []string{"max_response_output_tokens", "max_output_tokens"} {
		value, exists := session[field]
		if !exists {
			continue
		}
		delete(session, field)
		if _, isNumber := value.(float64); isNumber {
			if _, hasMaxTokens := session["max_tokens"]; !hasMaxTokens {
				session["max_tokens"] = value
			}
		}
		adapted = true
	}

	if transcription, ok := session["input_audio_transcription"].(map[string]any); ok {
		if model, ok := transcription["model"].(string); ok && isOpenAIClientTranscriptionModel(model) {
			transcription["model"] = realtimeAliTranscriptionModel
			adapted = true
		}
	}

	if !adapted {
		return []string{string(message)}, nil
	}
	out, err := common.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("marshal adapted session.update: %w", err)
	}
	return []string{string(out)}, nil
}

// adaptClientConversationItemCreate 将 item 中的 base64 input_image 部分抽取为
// 独立的 input_image_buffer.append 事件先行发送（百炼图片输入走缓冲区模型），
// 再转发剥离图片后的事件；剥离后无剩余内容时丢弃该 item 事件。
// http(s) URL 图片无法由服务端取图，丢弃该部分并记录告警。
func adaptClientConversationItemCreate(c *gin.Context, message []byte, event map[string]any) ([]string, error) {
	item, _ := event["item"].(map[string]any)
	if item == nil {
		return []string{string(message)}, nil
	}
	content, ok := item["content"].([]any)
	if !ok || len(content) == 0 {
		return []string{string(message)}, nil
	}
	adapted := false
	kept := make([]any, 0, len(content))
	imageEvents := make([]string, 0, len(content))
	for _, part := range content {
		partMap, isMap := part.(map[string]any)
		if !isMap {
			kept = append(kept, part)
			continue
		}
		if partType, _ := partMap["type"].(string); partType != "input_image" {
			kept = append(kept, part)
			continue
		}
		adapted = true
		payload, isBase64 := extractAliImagePayload(partMap["image_url"])
		if !isBase64 {
			logger.LogWarn(c, "realtime: dropped non-base64 input_image in conversation.item.create (DashScope only accepts base64 via input_image_buffer.append)")
			continue
		}
		data, err := common.Marshal(map[string]any{
			"type":  "input_image_buffer.append",
			"image": payload,
		})
		if err != nil {
			return nil, fmt.Errorf("marshal input_image_buffer.append: %w", err)
		}
		imageEvents = append(imageEvents, string(data))
	}

	if !adapted {
		return []string{string(message)}, nil
	}

	item["content"] = kept
	var itemEvents []string
	if len(kept) > 0 {
		out, err := common.Marshal(event)
		if err != nil {
			return nil, fmt.Errorf("marshal stripped conversation.item.create: %w", err)
		}
		itemEvents = append(itemEvents, string(out))
	}
	return append(imageEvents, itemEvents...), nil
}

// adaptClientResponseCreate 移除百炼不支持的 response 覆盖对象。
// 百炼 response.create 仅接受 type/event_id，而 OpenAI 客户端常附带
// response 覆盖（instructions、modalities、tools 等），原样转发会被上游拒绝。
// 覆盖内容被丢弃并记录告警，本次响应沿用会话级配置。
func adaptClientResponseCreate(c *gin.Context, message []byte, event map[string]any) ([]string, error) {
	override, exists := event["response"]
	if !exists {
		return []string{string(message)}, nil
	}
	if overrideMap, ok := override.(map[string]any); ok && len(overrideMap) > 0 {
		logger.LogWarn(c, "realtime: dropped response overrides in response.create (DashScope only accepts type/event_id)")
	}
	delete(event, "response")
	out, err := common.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("marshal adapted response.create: %w", err)
	}
	return []string{string(out)}, nil
}

// ---- 上游 → 客户端（百炼 → OpenAI）----

// transformUpstreamEvent 仅改写 session.created/updated 的音频格式回显；其余事件原样转发。
func transformUpstreamEvent(c *gin.Context, info *relaycommon.RelayInfo, message []byte) ([]string, error) {
	event, ok := decodeEventMap(message)
	if !ok {
		return []string{string(message)}, nil
	}
	switch eventType(event) {
	case dto.RealtimeEventTypeSessionCreated, dto.RealtimeEventTypeSessionUpdated:
		return adaptUpstreamSessionEcho(message, event)
	default:
		return []string{string(message)}, nil
	}
}

// adaptUpstreamSessionEcho 将百炼嵌套的 audio.input/output.format 平铺为
// OpenAI 客户端可识别的 input_audio_format/output_audio_format 字段（保留嵌套字段）。
func adaptUpstreamSessionEcho(message []byte, event map[string]any) ([]string, error) {
	session, _ := event["session"].(map[string]any)
	if session == nil {
		return []string{string(message)}, nil
	}
	audio, ok := session["audio"].(map[string]any)
	if !ok {
		return []string{string(message)}, nil
	}
	adapted := false
	if nestedGetString(audio, "input", "format", "type") == "pcm" {
		if _, exists := session["input_audio_format"]; !exists {
			session["input_audio_format"] = realtimeOpenAIPcm16
			adapted = true
		}
	}
	if nestedGetString(audio, "output", "format", "type") == "pcm" {
		if _, exists := session["output_audio_format"]; !exists {
			session["output_audio_format"] = realtimeOpenAIPcm16
			adapted = true
		}
	}
	if !adapted {
		return []string{string(message)}, nil
	}
	out, err := common.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("marshal adapted session echo: %w", err)
	}
	return []string{string(out)}, nil
}

// ---- 工具函数 ----

func decodeEventMap(message []byte) (map[string]any, bool) {
	var event map[string]any
	if err := common.Unmarshal(message, &event); err != nil {
		return nil, false
	}
	return event, true
}

func eventType(event map[string]any) string {
	eventType, _ := event["type"].(string)
	return eventType
}

func nestedGetString(obj map[string]any, path ...string) string {
	if len(path) == 0 {
		return ""
	}
	node := obj
	for _, key := range path[:len(path)-1] {
		next, ok := node[key].(map[string]any)
		if !ok {
			return ""
		}
		node = next
	}
	value, _ := node[path[len(path)-1]].(string)
	return value
}

// extractAliImagePayload 从 OpenAI input_image 的 image_url 中提取 base64 数据，
// 仅支持 data URI（服务端不代取公网图片）。兼容 image_url 为字符串或 {url} 对象两种写法。
func extractAliImagePayload(imageURL any) (string, bool) {
	raw := ""
	switch value := imageURL.(type) {
	case string:
		raw = value
	case map[string]any:
		raw, _ = value["url"].(string)
	}
	raw = strings.TrimSpace(raw)
	const base64Marker = ";base64,"
	if !strings.HasPrefix(raw, "data:") {
		return "", false
	}
	idx := strings.Index(raw, base64Marker)
	if idx < 0 {
		return "", false
	}
	payload := raw[idx+len(base64Marker):]
	if payload == "" {
		return "", false
	}
	return payload, true
}

// isOpenAIClientTranscriptionModel 判断是否为 OpenAI 侧的输入转写模型，
// 百炼侧统一替换为 qwen3-asr-flash-realtime。
func isOpenAIClientTranscriptionModel(model string) bool {
	switch model {
	case "whisper-1", "whisper-large-v3", "gpt-4o-transcribe", "gpt-4o-mini-transcribe":
		return true
	}
	return false
}
