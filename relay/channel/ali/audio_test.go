package ali

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func newAliSpeechInfo(model string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "https://dashscope.aliyuncs.com",
			UpstreamModelName: model,
		},
		RelayMode:   constant.RelayModeAudioSpeech,
		RelayFormat: types.RelayFormatOpenAI,
	}
}

func newAliSpeechContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	return c
}

func decodeAliSpeechRequest(t *testing.T, reader io.Reader) map[string]any {
	t.Helper()
	data, err := io.ReadAll(reader)
	assert.NoError(t, err)
	var req map[string]any
	assert.NoError(t, common.Unmarshal(data, &req))
	return req
}

func TestIsAliSambertModel(t *testing.T) {
	assert.True(t, isAliSambertModel("sambert-zhichu-v1"))
	assert.True(t, isAliSambertModel("SAMBERT-zhimiao-emo-v1"))
	assert.False(t, isAliSambertModel("cosyvoice-v2"))
	assert.False(t, isAliSambertModel("not-sambert-v1"))
	assert.True(t, aliTTSSupportsModel("sambert-zhichu-v1"))
}

func TestAdaptor_GetRequestURL_AudioSpeech_Sambert(t *testing.T) {
	adaptor := &Adaptor{}

	url, err := adaptor.GetRequestURL(newAliSpeechInfo("sambert-zhichu-v1"))
	assert.NoError(t, err)
	assert.Equal(t, "https://dashscope.aliyuncs.com/api/v1/services/audio/tts/SpeechSynthesizer", url)
}

func TestConvertAliSpeechRequest_Sambert(t *testing.T) {
	c := newAliSpeechContext()
	speed := 1.5
	request := dto.AudioRequest{
		Model:          "sambert-zhichu-v1",
		Input:          "床前明月光，疑是地上霜。",
		Voice:          "alloy",
		ResponseFormat: "mp3",
		Speed:          &speed,
	}

	reader, err := convertAliSpeechRequest(c, newAliSpeechInfo("sambert-zhichu-v1"), request)
	assert.NoError(t, err)

	req := decodeAliSpeechRequest(t, reader)
	assert.Equal(t, "sambert-zhichu-v1", req["model"])

	input, ok := req["input"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "床前明月光，疑是地上霜。", input["text"])
	// Sambert 音色即模型，不应携带 voice 字段
	assert.NotContains(t, input, "voice")
	assert.NotContains(t, req, "voice")

	params, ok := req["parameters"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "PlainText", params["text_type"])
	assert.Equal(t, "mp3", params["format"])
	assert.Equal(t, float64(16000), params["sample_rate"])
	assert.Equal(t, 1.5, params["rate"])

	assert.Equal(t, 16000, c.GetInt(contextKeyAliAudioSampleRate))
	assert.Equal(t, "mp3", c.GetString(contextKeyAliAudioResponseFormat))
}

func TestConvertAliSpeechRequest_Sambert_SpeedClamp(t *testing.T) {
	cases := []struct {
		name     string
		speed    *float64
		wantRate any
	}{
		{"nil speed omits rate", nil, nil},
		{"fast clamped to 2.0", func() *float64 { v := 4.0; return &v }(), 2.0},
		{"slow clamped to 0.5", func() *float64 { v := 0.25; return &v }(), 0.5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newAliSpeechContext()
			request := dto.AudioRequest{Model: "sambert-zhichu-v1", Input: "你好", Speed: tc.speed}

			reader, err := convertAliSpeechRequest(c, newAliSpeechInfo("sambert-zhichu-v1"), request)
			assert.NoError(t, err)

			params := decodeAliSpeechRequest(t, reader)["parameters"].(map[string]any)
			if tc.wantRate == nil {
				assert.NotContains(t, params, "rate")
			} else {
				assert.Equal(t, tc.wantRate, params["rate"])
			}
		})
	}
}

func TestConvertAliSpeechRequest_Sambert_DefaultFormat(t *testing.T) {
	c := newAliSpeechContext()
	request := dto.AudioRequest{Model: "sambert-zhichu-v1", Input: "你好"}

	reader, err := convertAliSpeechRequest(c, newAliSpeechInfo("sambert-zhichu-v1"), request)
	assert.NoError(t, err)

	params := decodeAliSpeechRequest(t, reader)["parameters"].(map[string]any)
	assert.Equal(t, "wav", params["format"])
}

func TestConvertAliSpeechRequest_Sambert_EmptyInput(t *testing.T) {
	c := newAliSpeechContext()
	request := dto.AudioRequest{Model: "sambert-zhichu-v1"}

	_, err := convertAliSpeechRequest(c, newAliSpeechInfo("sambert-zhichu-v1"), request)
	assert.Error(t, err)
}

func TestConvertAliSpeechRequest_UnsupportedModel(t *testing.T) {
	c := newAliSpeechContext()
	request := dto.AudioRequest{Model: "gpt-4o-mini-tts", Input: "你好"}

	_, err := convertAliSpeechRequest(c, newAliSpeechInfo("gpt-4o-mini-tts"), request)
	assert.Error(t, err)
}

func TestConvertAliSpeechRequest_CosyVoice(t *testing.T) {
	c := newAliSpeechContext()
	request := dto.AudioRequest{Model: "cosyvoice-v2", Input: "你好", ResponseFormat: "pcm"}

	reader, err := convertAliSpeechRequest(c, newAliSpeechInfo("cosyvoice-v2"), request)
	assert.NoError(t, err)

	req := decodeAliSpeechRequest(t, reader)
	assert.Equal(t, "cosyvoice-v2", req["model"])
	assert.NotContains(t, req, "parameters")

	input := req["input"].(map[string]any)
	assert.Equal(t, "longanhuan_v3", input["voice"])
	assert.Equal(t, "pcm", input["format"])
	assert.Equal(t, float64(24000), input["sample_rate"])

	assert.Equal(t, 24000, c.GetInt(contextKeyAliAudioSampleRate))
}