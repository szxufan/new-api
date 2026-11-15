package middleware

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	relayconstant "github.com/QuantumNous/new-api/relay/constant"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newAudioJSONContext(t *testing.T, path, body string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c
}

func newAudioFormContext(t *testing.T, path string, fields map[string]string) *gin.Context {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "recording.webm")
	require.NoError(t, err)
	_, err = part.Write([]byte("fake-audio-bytes"))
	require.NoError(t, err)
	for key, value := range fields {
		require.NoError(t, writer.WriteField(key, value))
	}
	require.NoError(t, writer.Close())

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, path, &body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	return c
}

func TestNormalizeAudioRelayPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"playground speech", "/pg/audio/speech", "/v1/audio/speech"},
		{"playground transcriptions", "/pg/audio/transcriptions", "/v1/audio/transcriptions"},
		{"playground translations", "/pg/audio/translations", "/v1/audio/translations"},
		{"standard speech", "/v1/audio/speech", "/v1/audio/speech"},
		{"non audio path", "/pg/chat/completions", "/pg/chat/completions"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, normalizeAudioRelayPath(tt.path))
		})
	}
}

func TestGetModelRequestAudioJSON(t *testing.T) {
	c := newAudioJSONContext(t, "/pg/audio/speech", `{"model":"tts-custom","group":"vip"}`)
	req, shouldSelectChannel, err := getModelRequest(c)
	require.NoError(t, err)
	assert.True(t, shouldSelectChannel)
	assert.Equal(t, "tts-custom", req.Model)
	assert.Equal(t, "vip", req.Group, "playground 语音合成需要回填 group 供分组覆盖")
	assert.Equal(t, relayconstant.RelayModeAudioSpeech, c.GetInt("relay_mode"))
}

func TestGetModelRequestAudioSpeechDefaultModel(t *testing.T) {
	c := newAudioJSONContext(t, "/v1/audio/speech", `{}`)
	req, _, err := getModelRequest(c)
	require.NoError(t, err)
	assert.Equal(t, "tts-1", req.Model)
	assert.Equal(t, relayconstant.RelayModeAudioSpeech, c.GetInt("relay_mode"))
}

func TestGetModelRequestAudioMultipart(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantMode int
	}{
		{"playground transcriptions", "/pg/audio/transcriptions", relayconstant.RelayModeAudioTranscription},
		{"playground translations", "/pg/audio/translations", relayconstant.RelayModeAudioTranslation},
		{"standard transcriptions", "/v1/audio/transcriptions", relayconstant.RelayModeAudioTranscription},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newAudioFormContext(t, tt.path, map[string]string{
				"model": "whisper-1",
				"group": "vip",
			})
			req, _, err := getModelRequest(c)
			require.NoError(t, err)
			assert.Equal(t, "whisper-1", req.Model)
			assert.Equal(t, "vip", req.Group, "multipart 音频应一次解析同时得到 model 与 group")
			assert.Equal(t, tt.wantMode, c.GetInt("relay_mode"))
		})
	}
}
