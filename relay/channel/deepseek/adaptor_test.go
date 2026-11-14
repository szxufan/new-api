package deepseek

import (
	"bytes"
	"encoding/binary"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestContext(t *testing.T) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	return c
}

func newAudioRelayInfo(baseUrl string, relayMode int) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: baseUrl,
		},
		RelayMode: relayMode,
	}
}

func TestAdaptor_GetRequestURL_AudioSpeech(t *testing.T) {
	adaptor := &Adaptor{}
	url, err := adaptor.GetRequestURL(newAudioRelayInfo("https://api.deepseek.com", relayconstant.RelayModeAudioSpeech))
	require.NoError(t, err)
	assert.Equal(t, "https://api.deepseek.com/v1/tools/tts", url)
}

func TestAdaptor_GetRequestURL_AudioTranscription(t *testing.T) {
	adaptor := &Adaptor{}
	url, err := adaptor.GetRequestURL(newAudioRelayInfo("https://api.deepseek.com", relayconstant.RelayModeAudioTranscription))
	require.NoError(t, err)
	assert.Equal(t, "https://api.deepseek.com/v1/tools/asr", url)
}

func TestConvertTTSRequest_Defaults(t *testing.T) {
	c := newTestContext(t)
	reader, err := convertTTSRequest(c, dto.AudioRequest{Input: "你好，世界"})
	require.NoError(t, err)

	var ttsRequest deepSeekTTSRequest
	body, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, common.Unmarshal(body, &ttsRequest))

	assert.Equal(t, "你好，世界", ttsRequest.Text)
	assert.Equal(t, "stella", ttsRequest.VoiceID)
	assert.Equal(t, "wav", ttsRequest.Format)
	assert.NotNil(t, ttsRequest.FilterMarkdown)
	assert.True(t, *ttsRequest.FilterMarkdown)
	assert.Equal(t, "wav", c.GetString(contextKeyTTSFormat))
}

func TestConvertTTSRequest_CustomVoiceAndFormat(t *testing.T) {
	c := newTestContext(t)
	reader, err := convertTTSRequest(c, dto.AudioRequest{
		Input:          "hello",
		Voice:          "tide",
		ResponseFormat: "pcm",
	})
	require.NoError(t, err)

	var ttsRequest deepSeekTTSRequest
	body, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, common.Unmarshal(body, &ttsRequest))

	assert.Equal(t, "tide", ttsRequest.VoiceID)
	assert.Equal(t, "pcm", ttsRequest.Format)
	assert.Equal(t, "pcm", c.GetString(contextKeyTTSFormat))
}

func TestConvertTTSRequest_MetadataOverride(t *testing.T) {
	c := newTestContext(t)
	metadata, err := common.Marshal(map[string]any{"filter_markdown": false})
	require.NoError(t, err)
	reader, err := convertTTSRequest(c, dto.AudioRequest{
		Input:    "hello",
		Metadata: metadata,
	})
	require.NoError(t, err)

	var ttsRequest deepSeekTTSRequest
	body, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, common.Unmarshal(body, &ttsRequest))
	require.NotNil(t, ttsRequest.FilterMarkdown)
	assert.False(t, *ttsRequest.FilterMarkdown)
}

func TestConvertTTSRequest_InvalidFormat(t *testing.T) {
	c := newTestContext(t)
	_, err := convertTTSRequest(c, dto.AudioRequest{Input: "hello", ResponseFormat: "mp3"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only wav and pcm")
}

func TestConvertTTSRequest_EmptyInput(t *testing.T) {
	c := newTestContext(t)
	_, err := convertTTSRequest(c, dto.AudioRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "input is required")
}

func buildMultipartRequest(t *testing.T, filename string, content []byte) *gin.Context {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, err = io.Copy(part, bytes.NewReader(content))
	require.NoError(t, err)
	_ = writer.WriteField("model", "deepseek-asr")
	_ = writer.WriteField("language", "zh")
	require.NoError(t, writer.Close())

	c := newTestContext(t)
	c.Request = httptest.NewRequest("POST", "/v1/audio/transcriptions", &body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	return c
}

func TestConvertASRRequest_WavFile(t *testing.T) {
	wavContent := []byte("RIFF....fake wav data")
	c := buildMultipartRequest(t, "audio.wav", wavContent)

	reader, err := convertASRRequest(c, dto.AudioRequest{Model: "deepseek-asr", ResponseFormat: "json"})
	require.NoError(t, err)

	// 重建后的请求体应只保留 file 字段，丢弃 model/language
	body, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Contains(t, string(body), `filename="audio.wav"`)
	assert.NotContains(t, string(body), `"model"`)
	assert.NotContains(t, string(body), `"language"`)

	contentType := c.Request.Header.Get("Content-Type")
	assert.Contains(t, contentType, "multipart/form-data")
}

func TestConvertASRRequest_OpusFile(t *testing.T) {
	c := buildMultipartRequest(t, "audio.opus", []byte("OggS fake opus data"))
	_, err := convertASRRequest(c, dto.AudioRequest{})
	require.NoError(t, err)
}

func TestConvertASRRequest_UnsupportedFileExtension(t *testing.T) {
	c := buildMultipartRequest(t, "audio.mp3", []byte("fake mp3 data"))
	_, err := convertASRRequest(c, dto.AudioRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "16bit PCM WAV and Ogg Opus")
}

func TestConvertASRRequest_UnsupportedResponseFormat(t *testing.T) {
	c := buildMultipartRequest(t, "audio.wav", []byte("RIFF"))
	_, err := convertASRRequest(c, dto.AudioRequest{ResponseFormat: "verbose_json"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only json")
}

func TestConvertASRRequest_MissingFile(t *testing.T) {
	c := newTestContext(t)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("model", "deepseek-asr")
	require.NoError(t, writer.Close())
	c.Request = httptest.NewRequest("POST", "/v1/audio/transcriptions", &body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())

	_, err := convertASRRequest(c, dto.AudioRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "file is required")
}

func TestAudioTokensFromDuration(t *testing.T) {
	assert.Equal(t, 0, audioTokensFromDuration(0))
	assert.Equal(t, 1000, audioTokensFromDuration(60))  // 整分钟 1000 tokens
	assert.Equal(t, 2000, audioTokensFromDuration(120)) // 两分钟 2000 tokens
	assert.Equal(t, 17, audioTokensFromDuration(1))     // 1 秒按秒折算（ceil(1)/60*1000）
	assert.Equal(t, 1017, audioTokensFromDuration(61))  // 61 秒向上取整为 61/60 分钟
}

func TestConvertAudioRequest_TranslationUnsupported(t *testing.T) {
	adaptor := &Adaptor{}
	c := newTestContext(t)
	_, err := adaptor.ConvertAudioRequest(c, newAudioRelayInfo("https://api.deepseek.com", relayconstant.RelayModeAudioTranslation), dto.AudioRequest{})
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "not supported"))
}

func TestConvertTTSRequest_MetadataFormatRejected(t *testing.T) {
	c := newTestContext(t)
	metadata, err := common.Marshal(map[string]any{"format": "mp3"})
	require.NoError(t, err)

	_, err = convertTTSRequest(c, dto.AudioRequest{Input: "hello", Metadata: metadata})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only wav and pcm")
}

func TestConvertTTSRequest_MetadataFormatNormalized(t *testing.T) {
	c := newTestContext(t)
	metadata, err := common.Marshal(map[string]any{"format": " PCM "})
	require.NoError(t, err)

	reader, err := convertTTSRequest(c, dto.AudioRequest{Input: "hello", Metadata: metadata})
	require.NoError(t, err)

	var ttsRequest deepSeekTTSRequest
	body, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, common.Unmarshal(body, &ttsRequest))

	assert.Equal(t, "pcm", ttsRequest.Format)
	assert.Equal(t, "pcm", c.GetString(contextKeyTTSFormat))
}

// newAudioResponseContext 构造带请求上下文的 gin Context，供响应处理函数测试使用
func newAudioResponseContext(t *testing.T, path string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, path, nil)
	return c, w
}

// buildTestWAV 生成指定位深参数的静音 WAV（16bit/单声道），用于时长换算测试
func buildTestWAV(sampleRate, seconds int) []byte {
	const (
		numChannels   = 1
		bitsPerSample = 16
	)
	dataSize := sampleRate * numChannels * bitsPerSample / 8 * seconds
	buf := make([]byte, 44+dataSize)
	copy(buf[0:4], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:8], uint32(36+dataSize))
	copy(buf[8:12], "WAVE")
	copy(buf[12:16], "fmt ")
	binary.LittleEndian.PutUint32(buf[16:20], 16)
	binary.LittleEndian.PutUint16(buf[20:22], 1) // PCM
	binary.LittleEndian.PutUint16(buf[22:24], numChannels)
	binary.LittleEndian.PutUint32(buf[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(buf[28:32], uint32(sampleRate*numChannels*bitsPerSample/8))
	binary.LittleEndian.PutUint16(buf[32:34], numChannels*bitsPerSample/8)
	binary.LittleEndian.PutUint16(buf[34:36], bitsPerSample)
	copy(buf[36:40], "data")
	binary.LittleEndian.PutUint32(buf[40:44], uint32(dataSize))
	return buf
}

func TestHandleDeepSeekTTSResponse_Wav(t *testing.T) {
	c, w := newAudioResponseContext(t, "/v1/audio/speech")
	c.Set(contextKeyTTSFormat, "wav")

	wavData := buildTestWAV(16000, 2) // 2 秒 → ceil(2)/60*1000 = 33.33 → 33 tokens
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"X-Test": []string{"1"}},
		Body:       io.NopCloser(bytes.NewReader(wavData)),
	}
	info := &relaycommon.RelayInfo{}
	info.SetEstimatePromptTokens(7)

	usageAny, apiErr := handleDeepSeekTTSResponse(c, resp, info)
	require.Nil(t, apiErr)
	usage, ok := usageAny.(*dto.Usage)
	require.True(t, ok)

	assert.Equal(t, "audio/wav", w.Header().Get("Content-Type"))
	assert.Equal(t, "1", w.Header().Get("X-Test"))
	assert.Equal(t, wavData, w.Body.Bytes())
	assert.Equal(t, 7, usage.PromptTokens)
	assert.Equal(t, 7, usage.PromptTokensDetails.TextTokens)
	assert.Equal(t, 33, usage.CompletionTokens)
	assert.Equal(t, 33, usage.CompletionTokenDetails.AudioTokens)
	assert.Equal(t, 40, usage.TotalTokens)
}

func TestHandleDeepSeekTTSResponse_PCM(t *testing.T) {
	c, w := newAudioResponseContext(t, "/v1/audio/speech")
	c.Set(contextKeyTTSFormat, "pcm")

	pcmData := make([]byte, 48000) // 24kHz/16bit/单声道 1 秒
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{},
		Body:       io.NopCloser(bytes.NewReader(pcmData)),
	}
	info := &relaycommon.RelayInfo{}
	info.SetEstimatePromptTokens(3)

	usageAny, apiErr := handleDeepSeekTTSResponse(c, resp, info)
	require.Nil(t, apiErr)
	usage := usageAny.(*dto.Usage)

	assert.Equal(t, "audio/pcm", w.Header().Get("Content-Type"))
	assert.Equal(t, 17, usage.CompletionTokens)
	assert.Equal(t, 20, usage.TotalTokens)
}

func TestHandleDeepSeekTTSResponse_DurationFallback(t *testing.T) {
	c, w := newAudioResponseContext(t, "/v1/audio/speech")
	c.Set(contextKeyTTSFormat, "wav")

	invalidWav := []byte("not a wav file") // 14 字节，无法解析时长
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{},
		Body:       io.NopCloser(bytes.NewReader(invalidWav)),
	}
	info := &relaycommon.RelayInfo{}
	info.SetEstimatePromptTokens(5)

	usageAny, apiErr := handleDeepSeekTTSResponse(c, resp, info)
	require.Nil(t, apiErr)
	usage := usageAny.(*dto.Usage)

	assert.Equal(t, "audio/wav", w.Header().Get("Content-Type"))
	// 无法解析时长时按 body 大小粗略估算：每 KB 约 1 token（向上取整）
	assert.Equal(t, 1, usage.CompletionTokens)
	assert.Equal(t, 1, usage.CompletionTokenDetails.AudioTokens)
	assert.Equal(t, 6, usage.TotalTokens)
}

func TestHandleDeepSeekASRResponse_WithDuration(t *testing.T) {
	c, w := newAudioResponseContext(t, "/v1/audio/transcriptions")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader(`{"request_id":"req-1","text":"你好","duration_ms":60000}`)),
	}

	usageAny, apiErr := handleDeepSeekASRResponse(c, resp, &relaycommon.RelayInfo{})
	require.Nil(t, apiErr)
	usage := usageAny.(*dto.Usage)

	assert.JSONEq(t, `{"text":"你好"}`, w.Body.String())
	assert.Equal(t, 1000, usage.PromptTokens)
	assert.Equal(t, 1000, usage.PromptTokensDetails.AudioTokens)
	assert.Equal(t, 0, usage.CompletionTokens)
	assert.Equal(t, 1000, usage.TotalTokens)
}

func TestHandleDeepSeekASRResponse_DurationFallback(t *testing.T) {
	c, _ := newAudioResponseContext(t, "/v1/audio/transcriptions")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader(`{"request_id":"req-2","text":"hello"}`)),
	}
	info := &relaycommon.RelayInfo{}
	info.SetEstimatePromptTokens(500)

	usageAny, apiErr := handleDeepSeekASRResponse(c, resp, info)
	require.Nil(t, apiErr)
	usage := usageAny.(*dto.Usage)

	// 上游未返回时长时沿用本地音频时长估算，仍按音频 token 计费
	assert.Equal(t, 500, usage.PromptTokens)
	assert.Equal(t, 500, usage.PromptTokensDetails.AudioTokens)
	assert.Equal(t, 0, usage.CompletionTokens)
	assert.Equal(t, 500, usage.TotalTokens)
}

func TestHandleDeepSeekASRResponse_InvalidJSON(t *testing.T) {
	c, _ := newAudioResponseContext(t, "/v1/audio/transcriptions")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader("not json")),
	}

	_, apiErr := handleDeepSeekASRResponse(c, resp, &relaycommon.RelayInfo{})
	require.NotNil(t, apiErr)
}
