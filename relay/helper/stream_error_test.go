package helper

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newStreamTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	return c, recorder
}

func TestIsStreamResponseCommitted_NonStreamRequest(t *testing.T) {
	c, _ := newStreamTestContext()

	// 未设置 SSE 响应头标记：即使写入了数据也不算流式提交
	_, _ = c.Writer.Write([]byte("{}"))
	assert.False(t, IsStreamResponseCommitted(c))
}

func TestIsStreamResponseCommitted_HeadersSetButNotWritten(t *testing.T) {
	c, _ := newStreamTestContext()

	// 仅设置响应头（未 flush 任何字节）：HTTP 状态码尚未提交，仍可正常返回错误
	SetEventStreamHeaders(c)
	assert.False(t, IsStreamResponseCommitted(c))
}

func TestIsStreamResponseCommitted_AfterPingOrDataWritten(t *testing.T) {
	c, _ := newStreamTestContext()

	SetEventStreamHeaders(c)
	// 模拟保活数据写出（此时 HTTP 200 已提交）
	assert.NoError(t, PingData(c))
	assert.True(t, IsStreamResponseCommitted(c))
}

func TestWriteStreamError_OpenAIFormat(t *testing.T) {
	c, recorder := newStreamTestContext()

	SetEventStreamHeaders(c)
	assert.NoError(t, PingData(c))

	apiErr := types.NewError(errors.New("upstream exploded"), types.ErrorCodeDoRequestFailed)
	WriteStreamError(c, types.RelayFormatOpenAI, apiErr)

	body := recorder.Body.String()
	// 错误必须是合法 SSE 帧（data: 前缀），否则会被客户端解析器静默忽略
	assert.Contains(t, body, "data: {\"error\":", "error should be wrapped in an SSE data frame")
	assert.Contains(t, body, "upstream exploded")
	assert.True(t, strings.HasSuffix(body, "data: [DONE]\n\n"), "stream should be terminated with [DONE]")
}

func TestWriteStreamError_ClaudeFormat(t *testing.T) {
	c, recorder := newStreamTestContext()

	SetEventStreamHeaders(c)
	assert.NoError(t, PingData(c))

	apiErr := types.NewError(errors.New("upstream exploded"), types.ErrorCodeDoRequestFailed)
	WriteStreamError(c, types.RelayFormatClaude, apiErr)

	body := recorder.Body.String()
	assert.Contains(t, body, "event: error\n", "claude error should use an SSE error event")
	assert.Contains(t, body, "\"type\":\"error\"")
	assert.Contains(t, body, "upstream exploded")
}

func TestWriteStreamError_NilSafety(t *testing.T) {
	c, recorder := newStreamTestContext()

	// nil 错误不应写出任何内容，也不应 panic
	WriteStreamError(c, types.RelayFormatOpenAI, nil)
	assert.Empty(t, recorder.Body.String())
}
