package middleware

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestGetModelRequestWebToolsDefaults 测试网页工具端点的默认虚拟模型解析：
// body 未指定 model 时按端点回填 deepseek-search / deepseek-fetch，显式指定时优先
func TestGetModelRequestWebToolsDefaults(t *testing.T) {
	cases := []struct {
		name      string
		path      string
		body      string
		wantModel string
	}{
		{name: "search 无 model 默认", path: "/v1/tools/search", body: `{"query":"golang"}`, wantModel: "deepseek-search"},
		{name: "fetch 无 model 默认", path: "/v1/tools/fetch", body: `{"url":"https://example.com"}`, wantModel: "deepseek-fetch"},
		{name: "search 显式 model", path: "/v1/tools/search", body: `{"model":"tavily-search","query":"golang"}`, wantModel: "tavily-search"},
		{name: "fetch 显式 model", path: "/v1/tools/fetch", body: `{"model":"jina-fetch","url":"https://example.com"}`, wantModel: "jina-fetch"},
		{name: "playground search 默认", path: "/pg/tools/search", body: `{"query":"golang"}`, wantModel: "deepseek-search"},
		{name: "tts 无 model 默认", path: "/v1/tools/tts", body: `{"text":"你好"}`, wantModel: "deepseek-tts"},
		{name: "tts 显式 model", path: "/v1/tools/tts", body: `{"model":"my-tts","text":"你好"}`, wantModel: "my-tts"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			req := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			c.Request = req

			modelRequest, shouldSelectChannel, err := getModelRequest(c)
			require.NoError(t, err)
			require.True(t, shouldSelectChannel)
			require.Equal(t, tt.wantModel, modelRequest.Model)
		})
	}
}

// TestGetModelRequestToolsASRDefaults 测试原生语音识别端点（multipart）的默认虚拟模型解析：
// 表单未指定 model 时回填 deepseek-asr，显式指定时优先
func TestGetModelRequestToolsASRDefaults(t *testing.T) {
	cases := []struct {
		name      string
		model     string
		wantModel string
	}{
		{name: "asr 无 model 默认", model: "", wantModel: "deepseek-asr"},
		{name: "asr 显式 model", model: "my-asr", wantModel: "my-asr"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			if tt.model != "" {
				require.NoError(t, writer.WriteField("model", tt.model))
			}
			part, err := writer.CreateFormFile("file", "audio.wav")
			require.NoError(t, err)
			_, err = part.Write([]byte("RIFF fake wav"))
			require.NoError(t, err)
			require.NoError(t, writer.Close())

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			req := httptest.NewRequest(http.MethodPost, "/v1/tools/asr", &body)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			c.Request = req

			modelRequest, shouldSelectChannel, err := getModelRequest(c)
			require.NoError(t, err)
			require.True(t, shouldSelectChannel)
			require.Equal(t, tt.wantModel, modelRequest.Model)
		})
	}
}
