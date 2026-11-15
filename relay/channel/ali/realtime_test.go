package ali

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gin-gonic/gin"
)

func newTestContext(t *testing.T) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	return c
}

func decodeEventJSON(t *testing.T, data string) map[string]any {
	t.Helper()
	var event map[string]any
	require.NoError(t, common.Unmarshal([]byte(data), &event))
	return event
}

func sessionOf(t *testing.T, event map[string]any) map[string]any {
	t.Helper()
	session, ok := event["session"].(map[string]any)
	require.True(t, ok, "session should be an object")
	return session
}

func TestAdaptClientSessionUpdate_Pcm16(t *testing.T) {
	message := []byte(`{"type":"session.update","event_id":"e1","session":{` +
		`"modalities":["text","audio"],"voice":"Tina","instructions":"hi",` +
		`"input_audio_format":"pcm16","output_audio_format":"pcm16",` +
		`"max_response_output_tokens":1024,` +
		`"input_audio_transcription":{"model":"whisper-1"},` +
		`"turn_detection":{"type":"server_vad","threshold":0.5}}}`)

	out, err := adaptClientSessionUpdate(message, decodeEventJSON(t, string(message)))
	require.NoError(t, err)
	require.Len(t, out, 1)

	event := decodeEventJSON(t, out[0])
	session := sessionOf(t, event)

	// 平铺格式字段被替换为嵌套 audio.input/output.format（OpenAI pcm16 = 24kHz）
	_, hasInput := session["input_audio_format"]
	_, hasOutput := session["output_audio_format"]
	assert.False(t, hasInput, "flat input_audio_format should be removed")
	assert.False(t, hasOutput, "flat output_audio_format should be removed")

	audio, ok := session["audio"].(map[string]any)
	require.True(t, ok, "audio should be an object")
	input := audio["input"].(map[string]any)
	inputFormat := input["format"].(map[string]any)
	assert.Equal(t, "pcm", inputFormat["type"])
	assert.Equal(t, float64(24000), inputFormat["sample_rate"])
	assert.Equal(t, "s16le", inputFormat["sample_format"])
	assert.Equal(t, float64(1), inputFormat["channels"])
	assert.Equal(t, "interleaved", inputFormat["packing"])
	assert.Equal(t, "mono", inputFormat["channel_layout"])

	output := audio["output"].(map[string]any)
	outputFormat := output["format"].(map[string]any)
	assert.Equal(t, "pcm", outputFormat["type"])
	assert.Equal(t, float64(24000), outputFormat["sample_rate"])

	// max_response_output_tokens → max_tokens
	assert.Equal(t, float64(1024), session["max_tokens"])
	_, hasMaxOutput := session["max_response_output_tokens"]
	assert.False(t, hasMaxOutput)

	// OpenAI 转写模型替换为百炼模型
	transcription := session["input_audio_transcription"].(map[string]any)
	assert.Equal(t, "qwen3-asr-flash-realtime", transcription["model"])

	// 其余字段原样保留
	assert.Equal(t, []any{"text", "audio"}, session["modalities"])
	assert.Equal(t, "Tina", session["voice"])
	assert.Equal(t, "hi", session["instructions"])
	turnDetection := session["turn_detection"].(map[string]any)
	assert.Equal(t, "server_vad", turnDetection["type"])
}

func TestAdaptClientSessionUpdate_NativeAudioUntouched(t *testing.T) {
	message := []byte(`{"type":"session.update","session":{` +
		`"audio":{"input":{"format":{"type":"pcm","sample_rate":16000}}},` +
		`"voice":"Tina"}}`)

	out, err := adaptClientSessionUpdate(message, decodeEventJSON(t, string(message)))
	require.NoError(t, err)
	assert.Equal(t, []string{string(message)}, out, "native DashScope session should pass through unchanged")
}

func TestAdaptClientSessionUpdate_NoChanges(t *testing.T) {
	message := []byte(`{"type":"session.update","session":{"modalities":["text"],"voice":"Tina"}}`)

	out, err := adaptClientSessionUpdate(message, decodeEventJSON(t, string(message)))
	require.NoError(t, err)
	assert.Equal(t, []string{string(message)}, out, "session without adaptable fields should pass through unchanged")
}

func TestAdaptClientSessionUpdate_NonPcm16FormatKept(t *testing.T) {
	message := []byte(`{"type":"session.update","session":{"input_audio_format":"g711_ulaw"}}`)

	out, err := adaptClientSessionUpdate(message, decodeEventJSON(t, string(message)))
	require.NoError(t, err)
	require.Len(t, out, 1)
	session := sessionOf(t, decodeEventJSON(t, out[0]))
	assert.Equal(t, "g711_ulaw", session["input_audio_format"], "non-pcm16 format should be kept for upstream error")
}

func TestAdaptClientSessionUpdate_MaxOutputTokensCompat(t *testing.T) {
	message := []byte(`{"type":"session.update","session":{"max_output_tokens":512}}`)

	out, err := adaptClientSessionUpdate(message, decodeEventJSON(t, string(message)))
	require.NoError(t, err)
	require.Len(t, out, 1)
	session := sessionOf(t, decodeEventJSON(t, out[0]))
	assert.Equal(t, float64(512), session["max_tokens"])
	_, hasOldField := session["max_output_tokens"]
	assert.False(t, hasOldField)
}

func TestAdaptClientSessionUpdate_InfMaxResponseTokensDropped(t *testing.T) {
	message := []byte(`{"type":"session.update","session":{"max_response_output_tokens":"inf"}}`)

	out, err := adaptClientSessionUpdate(message, decodeEventJSON(t, string(message)))
	require.NoError(t, err)
	require.Len(t, out, 1)
	session := sessionOf(t, decodeEventJSON(t, out[0]))
	_, hasMaxTokens := session["max_tokens"]
	assert.False(t, hasMaxTokens, "non-numeric limit should be dropped instead of forwarded")
	_, hasMaxResponse := session["max_response_output_tokens"]
	assert.False(t, hasMaxResponse)
}

func TestAdaptClientResponseCreate_StripsOverride(t *testing.T) {
	message := []byte(`{"type":"response.create","event_id":"e2",` +
		`"response":{"modalities":["text"],"instructions":"be brief"}}`)

	out, err := adaptClientResponseCreate(newTestContext(t), message, decodeEventJSON(t, string(message)))
	require.NoError(t, err)
	require.Len(t, out, 1)

	event := decodeEventJSON(t, out[0])
	assert.Equal(t, "response.create", event["type"])
	assert.Equal(t, "e2", event["event_id"])
	_, hasOverride := event["response"]
	assert.False(t, hasOverride, "DashScope response.create does not accept response overrides")
}

func TestAdaptClientResponseCreate_NoOverrideUnchanged(t *testing.T) {
	message := []byte(`{"type":"response.create","event_id":"e3"}`)

	out, err := adaptClientResponseCreate(newTestContext(t), message, decodeEventJSON(t, string(message)))
	require.NoError(t, err)
	assert.Equal(t, []string{string(message)}, out)
}

func TestAdaptUpstreamSessionEcho_FlattensAudioFormat(t *testing.T) {
	message := []byte(`{"type":"session.created","session":{"id":"s1",` +
		`"audio":{"input":{"format":{"type":"pcm","sample_rate":24000}},` +
		`"output":{"format":{"type":"pcm","sample_rate":24000}}}}}`)

	out, err := adaptUpstreamSessionEcho(message, decodeEventJSON(t, string(message)))
	require.NoError(t, err)
	require.Len(t, out, 1)

	session := sessionOf(t, decodeEventJSON(t, out[0]))
	assert.Equal(t, "pcm16", session["input_audio_format"])
	assert.Equal(t, "pcm16", session["output_audio_format"])
	// 嵌套字段保留
	audio := session["audio"].(map[string]any)
	inputFormat := audio["input"].(map[string]any)["format"].(map[string]any)
	assert.Equal(t, "pcm", inputFormat["type"])
	assert.Equal(t, "s1", session["id"])
}

func TestAdaptUpstreamSessionEcho_NoAudioUnchanged(t *testing.T) {
	message := []byte(`{"type":"session.updated","session":{"id":"s1","voice":"Tina"}}`)

	out, err := adaptUpstreamSessionEcho(message, decodeEventJSON(t, string(message)))
	require.NoError(t, err)
	assert.Equal(t, []string{string(message)}, out)
}

func TestAdaptClientConversationItemCreate_Base64ImageExtracted(t *testing.T) {
	message := []byte(`{"type":"conversation.item.create","item":{"type":"message","role":"user",` +
		`"content":[{"type":"input_text","text":"look at this"},` +
		`{"type":"input_image","image_url":"data:image/jpeg;base64,QUJD"}]}}`)

	out, err := adaptClientConversationItemCreate(newTestContext(t), message, decodeEventJSON(t, string(message)))
	require.NoError(t, err)
	require.Len(t, out, 2, "image append event first, then stripped item.create")

	appendEvent := decodeEventJSON(t, out[0])
	assert.Equal(t, "input_image_buffer.append", appendEvent["type"])
	assert.Equal(t, "QUJD", appendEvent["image"])

	itemEvent := decodeEventJSON(t, out[1])
	assert.Equal(t, "conversation.item.create", itemEvent["type"])
	content := itemEvent["item"].(map[string]any)["content"].([]any)
	require.Len(t, content, 1)
	assert.Equal(t, "input_text", content[0].(map[string]any)["type"])
}

func TestAdaptClientConversationItemCreate_NonBase64ImageDropped(t *testing.T) {
	message := []byte(`{"type":"conversation.item.create","item":{"type":"message","role":"user",` +
		`"content":[{"type":"input_image","image_url":"https://example.com/a.jpg"}]}}`)

	out, err := adaptClientConversationItemCreate(newTestContext(t), message, decodeEventJSON(t, string(message)))
	require.NoError(t, err)
	assert.Empty(t, out, "item with only non-base64 image should be dropped")
}

func TestAdaptClientConversationItemCreate_NoImageUnchanged(t *testing.T) {
	message := []byte(`{"type":"conversation.item.create","item":{"type":"message","role":"user",` +
		`"content":[{"type":"input_text","text":"hello"}]}}`)

	out, err := adaptClientConversationItemCreate(newTestContext(t), message, decodeEventJSON(t, string(message)))
	require.NoError(t, err)
	assert.Equal(t, []string{string(message)}, out)
}

func TestAdaptClientConversationItemCreate_ImageUrlObjectForm(t *testing.T) {
	message := []byte(`{"type":"conversation.item.create","item":{"type":"message","role":"user",` +
		`"content":[{"type":"input_image","image_url":{"url":"data:image/png;base64,WFla"}}]}}`)

	out, err := adaptClientConversationItemCreate(newTestContext(t), message, decodeEventJSON(t, string(message)))
	require.NoError(t, err)
	require.Len(t, out, 1, "image extracted, no remaining content so item.create is dropped")
	assert.Equal(t, "input_image_buffer.append", decodeEventJSON(t, out[0])["type"])
	assert.Equal(t, "WFla", decodeEventJSON(t, out[0])["image"])
}

func TestExtractAliImagePayload(t *testing.T) {
	payload, ok := extractAliImagePayload("data:image/jpeg;base64,QUJD")
	assert.True(t, ok)
	assert.Equal(t, "QUJD", payload)

	payload, ok = extractAliImagePayload(map[string]any{"url": "data:image/png;base64,WFla"})
	assert.True(t, ok)
	assert.Equal(t, "WFla", payload)

	_, ok = extractAliImagePayload("https://example.com/a.jpg")
	assert.False(t, ok, "http URL is not supported")

	_, ok = extractAliImagePayload("data:image/jpeg")
	assert.False(t, ok, "data URI without base64 marker is not supported")

	_, ok = extractAliImagePayload(nil)
	assert.False(t, ok)
}

func TestIsOpenAIClientTranscriptionModel(t *testing.T) {
	assert.True(t, isOpenAIClientTranscriptionModel("whisper-1"))
	assert.True(t, isOpenAIClientTranscriptionModel("gpt-4o-transcribe"))
	assert.False(t, isOpenAIClientTranscriptionModel("qwen3-asr-flash-realtime"))
	assert.False(t, isOpenAIClientTranscriptionModel(""))
}

func TestBuildAliRealtimeURL(t *testing.T) {
	url, err := buildAliRealtimeURL("https://wsid.cn-beijing.maas.aliyuncs.com", "qwen3.8-omni-flash-realtime")
	require.NoError(t, err)
	assert.Equal(t, "wss://wsid.cn-beijing.maas.aliyuncs.com/api-ws/v1/realtime?model=qwen3.8-omni-flash-realtime", url)

	// base_url 末尾带 "/" 时仍应补默认实时端点，而不是拼成 wss://host/?model=
	url, err = buildAliRealtimeURL("https://wsid.cn-beijing.maas.aliyuncs.com/", "qwen3.8-omni-flash-realtime")
	require.NoError(t, err)
	assert.Equal(t, "wss://wsid.cn-beijing.maas.aliyuncs.com/api-ws/v1/realtime?model=qwen3.8-omni-flash-realtime", url)

	_, err = buildAliRealtimeURL("", "qwen3.8-omni-flash-realtime")
	assert.Error(t, err)

	_, err = buildAliRealtimeURL("https://", "qwen3.8-omni-flash-realtime")
	assert.Error(t, err, "base url without host should be rejected")

	_, err = buildAliRealtimeURL("https://wsid.cn-beijing.maas.aliyuncs.com/%zz", "m")
	assert.Error(t, err, "invalid percent-encoding should make the base URL unparseable")
}
