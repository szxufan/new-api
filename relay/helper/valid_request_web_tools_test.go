package helper

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newWebToolsTestContext(path, body string) *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	return c
}

// TestGetAndValidateWebToolsRequest_Search 测试搜索端点请求校验
func TestGetAndValidateWebToolsRequest_Search(t *testing.T) {
	request, err := GetAndValidateWebToolsRequest(
		newWebToolsTestContext("/v1/tools/search", `{"query":"golang","count":3,"need_content":true}`),
		relayconstant.RelayModeWebSearch)
	require.NoError(t, err)
	searchReq, ok := request.(*dto.WebSearchRequest)
	require.True(t, ok)
	require.Equal(t, "golang", searchReq.Query)
	require.NotNil(t, searchReq.Count)
	require.Equal(t, 3, *searchReq.Count)
	require.NotNil(t, searchReq.NeedContent)
	require.True(t, *searchReq.NeedContent)
}

// TestGetAndValidateWebToolsRequest_Fetch 测试抓取端点请求校验
func TestGetAndValidateWebToolsRequest_Fetch(t *testing.T) {
	request, err := GetAndValidateWebToolsRequest(
		newWebToolsTestContext("/v1/tools/fetch", `{"url":"https://example.com/page"}`),
		relayconstant.RelayModeWebFetch)
	require.NoError(t, err)
	fetchReq, ok := request.(*dto.WebFetchRequest)
	require.True(t, ok)
	require.Equal(t, "https://example.com/page", fetchReq.Url)
}

// TestGetAndValidateWebToolsRequest_ToolsTTS 测试原生语音合成端点请求校验
func TestGetAndValidateWebToolsRequest_ToolsTTS(t *testing.T) {
	request, err := GetAndValidateWebToolsRequest(
		newWebToolsTestContext("/v1/tools/tts", `{"text":"你好","voice_id":"tide","format":"pcm","filter_markdown":false}`),
		relayconstant.RelayModeToolsTTS)
	require.NoError(t, err)
	ttsReq, ok := request.(*dto.ToolsTTSRequest)
	require.True(t, ok)
	require.Equal(t, "你好", ttsReq.Text)
	require.Equal(t, "tide", ttsReq.VoiceID)
	require.Equal(t, "pcm", ttsReq.Format)
	require.NotNil(t, ttsReq.FilterMarkdown)
	require.False(t, *ttsReq.FilterMarkdown)

	// 空 text 被拒绝
	_, err = GetAndValidateWebToolsRequest(
		newWebToolsTestContext("/v1/tools/tts", `{"text":"  "}`),
		relayconstant.RelayModeToolsTTS)
	require.Error(t, err)
}

// newToolsASRTestContext 构造原生 ASR 的 multipart 测试上下文
func newToolsASRTestContext(t *testing.T, withFile bool) *gin.Context {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if withFile {
		part, err := writer.CreateFormFile("file", "audio.wav")
		require.NoError(t, err)
		_, err = part.Write([]byte("RIFF fake wav"))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/v1/tools/asr", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	c.Request = req
	return c
}

// TestGetAndValidateWebToolsRequest_ToolsASR 测试原生语音识别端点请求校验
func TestGetAndValidateWebToolsRequest_ToolsASR(t *testing.T) {
	request, err := GetAndValidateWebToolsRequest(newToolsASRTestContext(t, true), relayconstant.RelayModeToolsASR)
	require.NoError(t, err)
	_, ok := request.(*dto.ToolsASRRequest)
	require.True(t, ok)

	// 缺 file 被拒绝
	_, err = GetAndValidateWebToolsRequest(newToolsASRTestContext(t, false), relayconstant.RelayModeToolsASR)
	require.Error(t, err)
}

// TestGetAndValidateWebToolsRequest_Invalid 测试非法请求被拒绝
func TestGetAndValidateWebToolsRequest_Invalid(t *testing.T) {
	cases := []struct {
		name      string
		relayMode int
		body      string
	}{
		{"search 缺 query", relayconstant.RelayModeWebSearch, `{"count":3}`},
		{"search query 超长", relayconstant.RelayModeWebSearch, `{"query":"` + strings.Repeat("a", 129) + `"}`},
		{"search count 越界", relayconstant.RelayModeWebSearch, `{"query":"go","count":51}`},
		{"fetch 缺 url", relayconstant.RelayModeWebFetch, `{}`},
		{"fetch url 非法", relayconstant.RelayModeWebFetch, `{"url":"ftp://example.com"}`},
		{"未知 relay mode", relayconstant.RelayModeUnknown, `{}`},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GetAndValidateWebToolsRequest(newWebToolsTestContext("/v1/tools", tt.body), tt.relayMode)
			require.Error(t, err)
		})
	}
}
