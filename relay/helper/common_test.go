package helper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestRetryKeepAlive_NonStreamRequest(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	// 未设置 event_stream_headers_set，应不发送任何数据
	err := RetryKeepAlive(c)
	assert.NoError(t, err)
	assert.Empty(t, recorder.Body.String())
}

func TestRetryKeepAlive_StreamRequest(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	// 模拟 SSE 响应头已设置
	c.Set("event_stream_headers_set", true)

	err := RetryKeepAlive(c)
	assert.NoError(t, err)
	assert.Equal(t, ": retrying\n\n", recorder.Body.String())
}

func TestRetryKeepAlive_NilContext(t *testing.T) {
	err := RetryKeepAlive(nil)
	assert.NoError(t, err)
}

func TestRetryKeepAlive_ClientDisconnected(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	ctx, cancel := context.WithCancel(context.Background())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(ctx)
	c.Set("event_stream_headers_set", true)

	// 取消 context 模拟客户端断开
	cancel()

	err := RetryKeepAlive(c)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "request context done")
}

func TestRetryKeepAlive_MultipleCalls(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Set("event_stream_headers_set", true)

	// 多次调用应累积发送
	err := RetryKeepAlive(c)
	assert.NoError(t, err)
	err = RetryKeepAlive(c)
	assert.NoError(t, err)
	assert.Equal(t, ": retrying\n\n: retrying\n\n", recorder.Body.String())
}

func TestPingData_StillWorks(t *testing.T) {
	// 确保 PingData 行为未受影响
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	err := PingData(c)
	assert.NoError(t, err)
	assert.Equal(t, ": PING\n\n", recorder.Body.String())
}

func TestRetryKeepAlive_DistinctFromPing(t *testing.T) {
	// 确认 RetryKeepAlive 和 PingData 发送不同的内容
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Set("event_stream_headers_set", true)

	err := PingData(c)
	assert.NoError(t, err)
	err = RetryKeepAlive(c)
	assert.NoError(t, err)

	body := recorder.Body.String()
	assert.True(t, strings.Contains(body, ": PING\n\n"))
	assert.True(t, strings.Contains(body, ": retrying\n\n"))
}
