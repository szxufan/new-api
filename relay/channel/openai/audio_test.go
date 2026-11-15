package openai

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newAudioMultipartContext 构造携带 multipart 音频请求体的 gin 上下文
func newAudioMultipartContext(t *testing.T, path string, fields map[string]string) *gin.Context {
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

// TestConvertAudioRequestStripsInternalGroup 校验网关内部字段 group 不会随 ASR 请求透传给上游
func TestConvertAudioRequestStripsInternalGroup(t *testing.T) {
	c := newAudioMultipartContext(t, "/pg/audio/transcriptions", map[string]string{
		"model":    "whisper-1",
		"language": "zh",
		"prompt":   "专业术语",
		"group":    "vip",
	})

	adaptor := &Adaptor{}
	reader, err := adaptor.ConvertAudioRequest(c, &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeAudioTranscription,
	}, dto.AudioRequest{Model: "whisper-1"})
	require.NoError(t, err)

	raw, err := io.ReadAll(reader)
	require.NoError(t, err)
	body := string(raw)

	assert.NotContains(t, body, `name="group"`, "内部字段 group 不应透传上游")
	assert.Contains(t, body, `name="language"`)
	assert.Contains(t, body, `name="prompt"`)
	assert.Contains(t, body, `name="file"`)
	assert.Contains(t, c.Request.Header.Get("Content-Type"), "multipart/form-data")
}
