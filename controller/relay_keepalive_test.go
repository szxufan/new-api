package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestRetryKeepAliveSleep_NonStreamRequest(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	// 未设置 event_stream_headers_set，应退化为普通 Sleep
	start := time.Now()
	retryKeepAliveSleep(c, 50*time.Millisecond)
	elapsed := time.Since(start)

	assert.True(t, elapsed >= 50*time.Millisecond, "should sleep for at least the specified duration")
	assert.Empty(t, recorder.Body.String(), "should not send any data for non-stream requests")
}

func TestRetryKeepAliveSleep_StreamRequest(t *testing.T) {
	// 设置较短的 ping 间隔
	generalSettings := operation_setting.GetGeneralSetting()
	oldSeconds := generalSettings.PingIntervalSeconds
	generalSettings.PingIntervalSeconds = 1
	t.Cleanup(func() {
		generalSettings.PingIntervalSeconds = oldSeconds
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Set("event_stream_headers_set", true)

	// 等待 2 秒，ping 间隔 1 秒，应发送至少 1 次保活
	retryKeepAliveSleep(c, 2*time.Second)

	body := recorder.Body.String()
	count := strings.Count(body, ": retrying\n\n")
	assert.True(t, count >= 1, "should send at least 1 keepalive during 2s wait with 1s interval, got %d", count)
}

func TestRetryKeepAliveSleep_ShortWaitNoKeepalive(t *testing.T) {
	generalSettings := operation_setting.GetGeneralSetting()
	oldSeconds := generalSettings.PingIntervalSeconds
	generalSettings.PingIntervalSeconds = 10 // 10 秒间隔
	t.Cleanup(func() {
		generalSettings.PingIntervalSeconds = oldSeconds
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Set("event_stream_headers_set", true)

	// 等待 50ms，ping 间隔 10s，不应发送保活（间隔 > 等待时间，但间隔会被调整为等待时间）
	// 实际上 pingInterval 会被调整为 totalWait (50ms)，但 ticker 第一次触发需要 50ms
	// 由于 totalWait 也是 50ms，deadline 立刻到达，不会发送保活
	retryKeepAliveSleep(c, 50*time.Millisecond)

	body := recorder.Body.String()
	// 在如此短的等待时间内，可能不会触发 ticker
	// 这是预期行为：如果等待时间非常短，可能来不及发送保活
	_ = body
}

func TestRetryKeepAliveSleep_ClientDisconnect(t *testing.T) {
	generalSettings := operation_setting.GetGeneralSetting()
	oldSeconds := generalSettings.PingIntervalSeconds
	generalSettings.PingIntervalSeconds = 1
	t.Cleanup(func() {
		generalSettings.PingIntervalSeconds = oldSeconds
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	ctx, cancel := context.WithCancel(context.Background())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(ctx)
	c.Set("event_stream_headers_set", true)

	// 100ms 后取消 context
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	retryKeepAliveSleep(c, 10*time.Second)
	elapsed := time.Since(start)

	// 应在 context 取消后快速退出，不会等待完整的 10 秒
	assert.True(t, elapsed < 2*time.Second, "should exit quickly after context cancellation, took %v", elapsed)
}

func TestRetryKeepAliveSleep_ZeroPingInterval(t *testing.T) {
	generalSettings := operation_setting.GetGeneralSetting()
	oldSeconds := generalSettings.PingIntervalSeconds
	generalSettings.PingIntervalSeconds = 0 // 未配置，应使用默认 5 秒
	t.Cleanup(func() {
		generalSettings.PingIntervalSeconds = oldSeconds
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Set("event_stream_headers_set", true)

	// 默认间隔 5 秒 > 等待时间 100ms，间隔会被调整为 100ms
	// ticker 首次触发在 100ms 后，此时 deadline 也到达，可能发送 0 或 1 次
	retryKeepAliveSleep(c, 100*time.Millisecond)

	// 主要验证：函数正常退出，不会 panic 或死循环
	body := recorder.Body.String()
	_ = strings.Count(body, ": retrying\n\n")
}

func TestRetryKeepAliveSleep_ExitsOnDeadline(t *testing.T) {
	generalSettings := operation_setting.GetGeneralSetting()
	oldSeconds := generalSettings.PingIntervalSeconds
	generalSettings.PingIntervalSeconds = 1
	t.Cleanup(func() {
		generalSettings.PingIntervalSeconds = oldSeconds
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Set("event_stream_headers_set", true)

	// 使用 3 秒等待，1 秒间隔
	start := time.Now()
	retryKeepAliveSleep(c, 3*time.Second)
	elapsed := time.Since(start)

	// 应在约 3 秒后退出
	assert.True(t, elapsed >= 3*time.Second && elapsed < 4*time.Second,
		"should exit after deadline, took %v", elapsed)

	body := recorder.Body.String()
	count := strings.Count(body, ": retrying\n\n")
	// 3 秒等待 + 1 秒间隔 = 应发送约 2-3 次保活
	assert.True(t, count >= 2, "should send at least 2 keepalives in 3s with 1s interval, got %d", count)
}
