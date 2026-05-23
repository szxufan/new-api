package ollama

import (
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
	"unsafe"

	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type readCloser struct {
	strings.Reader
}

func (rc *readCloser) Close() error { return nil }

func newReadCloser(s string) io.ReadCloser {
	return &readCloser{*strings.NewReader(s)}
}

func newTestRelayInfo(isStream bool) *relaycommon.RelayInfo {
	now := time.Now()
	info := &relaycommon.RelayInfo{
		StartTime:         now,
		FirstResponseTime: now.Add(-time.Second),
		IsStream:          isStream,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "llama3",
		},
	}
	setIsFirstResponse(info, true)
	return info
}

func setIsFirstResponse(info *relaycommon.RelayInfo, val bool) {
	field := reflectField(info, "isFirstResponse")
	if field.IsValid() {
		*(*bool)(unsafe.Pointer(field.UnsafeAddr())) = val
	}
}

func reflectField(obj interface{}, name string) reflectValue {
	v := reflect.ValueOf(obj).Elem()
	f := v.FieldByName(name)
	return f
}

type reflectValue = reflect.Value

func newTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Request.Header.Set("Accept", "text/event-stream")
	return c, w
}

func buildOllamaStreamResp(chunks []string) *http.Response {
	body := strings.Join(chunks, "\n")
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/x-ndjson"}},
		Body:       newReadCloser(body),
	}
}

func TestOllamaStreamHandler_SetsFirstResponseTime(t *testing.T) {
	info := newTestRelayInfo(true)
	c, _ := newTestContext()

	chunks := []string{
		`{"model":"llama3","created_at":"2025-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello"},"done":false}`,
		`{"model":"llama3","created_at":"2025-01-01T00:00:02Z","message":{"role":"assistant","content":""},"done":true,"done_reason":"stop","prompt_eval_count":10,"eval_count":5}`,
	}

	resp := buildOllamaStreamResp(chunks)

	usage, apiErr := ollamaStreamHandler(c, info, resp)

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.True(t, info.HasSendResponse(), "FirstResponseTime should be set after first stream chunk")
	assert.True(t, info.FirstResponseTime.After(info.StartTime) || info.FirstResponseTime.Equal(info.StartTime),
		"FirstResponseTime (%v) should be >= StartTime (%v)", info.FirstResponseTime, info.StartTime)

	frtMs := info.FirstResponseTime.Sub(info.StartTime).Milliseconds()
	assert.GreaterOrEqual(t, frtMs, int64(0), "frt should be non-negative, got %dms", frtMs)
}

func TestOllamaStreamHandler_FirstResponseTimeSetOnlyOnce(t *testing.T) {
	info := newTestRelayInfo(true)
	c, _ := newTestContext()

	chunks := []string{
		`{"model":"llama3","created_at":"2025-01-01T00:00:00Z","message":{"role":"assistant","content":"A"},"done":false}`,
		`{"model":"llama3","created_at":"2025-01-01T00:00:01Z","message":{"role":"assistant","content":"B"},"done":false}`,
		`{"model":"llama3","created_at":"2025-01-01T00:00:02Z","message":{"role":"assistant","content":""},"done":true,"done_reason":"stop","prompt_eval_count":5,"eval_count":2}`,
	}

	resp := buildOllamaStreamResp(chunks)

	_, _ = ollamaStreamHandler(c, info, resp)

	firstFRT := info.FirstResponseTime
	assert.True(t, info.HasSendResponse(), "FirstResponseTime should be set")
	assert.Equal(t, firstFRT, info.FirstResponseTime, "FirstResponseTime should not change after first set")
}

func TestOllamaStreamHandler_GenerateMode_SetsFirstResponseTime(t *testing.T) {
	info := newTestRelayInfo(true)
	c, _ := newTestContext()

	chunks := []string{
		`{"model":"llama3","created_at":"2025-01-01T00:00:00Z","response":"Hello","done":false}`,
		`{"model":"llama3","created_at":"2025-01-01T00:00:01Z","response":"","done":true,"done_reason":"stop","prompt_eval_count":10,"eval_count":5}`,
	}

	resp := buildOllamaStreamResp(chunks)

	usage, apiErr := ollamaStreamHandler(c, info, resp)

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.True(t, info.HasSendResponse(), "FirstResponseTime should be set for generate mode too")
}

func TestOllamaStreamHandler_UsageExtracted(t *testing.T) {
	info := newTestRelayInfo(true)
	c, _ := newTestContext()

	chunks := []string{
		`{"model":"llama3","created_at":"2025-01-01T00:00:00Z","message":{"role":"assistant","content":"Hi"},"done":false}`,
		`{"model":"llama3","created_at":"2025-01-01T00:00:01Z","message":{"role":"assistant","content":""},"done":true,"done_reason":"length","prompt_eval_count":20,"eval_count":10}`,
	}

	resp := buildOllamaStreamResp(chunks)

	usage, apiErr := ollamaStreamHandler(c, info, resp)

	require.Nil(t, apiErr)
	assert.Equal(t, 20, usage.PromptTokens)
	assert.Equal(t, 10, usage.CompletionTokens)
	assert.Equal(t, 30, usage.TotalTokens)
}

func TestOllamaStreamHandler_EmptyStreamReturnsNoError(t *testing.T) {
	info := newTestRelayInfo(true)
	c, _ := newTestContext()

	resp := buildOllamaStreamResp([]string{""})

	usage, apiErr := ollamaStreamHandler(c, info, resp)

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
}

func TestToUnix(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
	}{
		{"empty string uses now", "", time.Now().Unix()},
		{"RFC3339 UTC", "2025-01-15T02:30:00Z", 1736908200},
		{"invalid format uses now", "not-a-date", time.Now().Unix()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toUnix(tt.input)
			if tt.name == "empty string uses now" || tt.name == "invalid format uses now" {
				now := time.Now().Unix()
				assert.InDelta(t, now, result, 2)
			} else {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestOllamaStreamHandler_NilResponse(t *testing.T) {
	info := newTestRelayInfo(true)
	c, _ := newTestContext()

	usage, apiErr := ollamaStreamHandler(c, info, nil)

	assert.Nil(t, usage)
	assert.NotNil(t, apiErr)
}

func TestOllamaStreamHandler_InvalidJSON(t *testing.T) {
	info := newTestRelayInfo(true)
	c, _ := newTestContext()

	chunks := []string{
		`{invalid json}`,
	}

	resp := buildOllamaStreamResp(chunks)

	usage, apiErr := ollamaStreamHandler(c, info, resp)

	assert.NotNil(t, apiErr)
	assert.NotNil(t, usage)
}

func TestContentPtr(t *testing.T) {
	assert.Nil(t, contentPtr(""))
	assert.NotNil(t, contentPtr("hello"))
	assert.Equal(t, "hello", *contentPtr("hello"))
}

func TestOllamaNonStreamHandler(t *testing.T) {
	info := newTestRelayInfo(false)
	c, _ := newTestContext()

	body := `{"model":"llama3","created_at":"2025-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello world"},"done":true,"done_reason":"stop","prompt_eval_count":10,"eval_count":5}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       newReadCloser(body),
	}

	usage, apiErr := ollamaChatHandler(c, info, resp)

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.Equal(t, 10, usage.PromptTokens)
	assert.Equal(t, 5, usage.CompletionTokens)
}

func TestOllamaStreamHandler_ToolCalls(t *testing.T) {
	info := newTestRelayInfo(true)
	c, _ := newTestContext()

	chunks := []string{
		`{"model":"llama3","created_at":"2025-01-01T00:00:00Z","message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"get_weather","arguments":{"city":"Beijing"}}}]},"done":false}`,
		`{"model":"llama3","created_at":"2025-01-01T00:00:01Z","message":{"role":"assistant","content":""},"done":true,"done_reason":"tool_calls","prompt_eval_count":15,"eval_count":8}`,
	}

	resp := buildOllamaStreamResp(chunks)

	usage, apiErr := ollamaStreamHandler(c, info, resp)

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.True(t, info.HasSendResponse())
	assert.Equal(t, 15, usage.PromptTokens)
	assert.Equal(t, 8, usage.CompletionTokens)
}

func TestOllamaStreamHandler_ThinkingContent(t *testing.T) {
	info := newTestRelayInfo(true)
	c, _ := newTestContext()

	chunks := []string{
		`{"model":"llama3","created_at":"2025-01-01T00:00:00Z","message":{"role":"assistant","content":"","thinking":"\"Let me think about this...\""},"done":false}`,
		`{"model":"llama3","created_at":"2025-01-01T00:00:01Z","message":{"role":"assistant","content":"The answer is 42"},"done":false}`,
		`{"model":"llama3","created_at":"2025-01-01T00:00:02Z","message":{"role":"assistant","content":""},"done":true,"done_reason":"stop","prompt_eval_count":10,"eval_count":5}`,
	}

	resp := buildOllamaStreamResp(chunks)

	usage, apiErr := ollamaStreamHandler(c, info, resp)

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.True(t, info.HasSendResponse())
}

func TestFrtInLogIsNoLongerNegative(t *testing.T) {
	info := newTestRelayInfo(true)

	beforeStream := info.FirstResponseTime.Sub(info.StartTime).Milliseconds()
	assert.Equal(t, int64(-1000), beforeStream, "before stream, frt should be -1000ms (i.e. -1.0s)")

	c, _ := newTestContext()
	chunks := []string{
		`{"model":"llama3","created_at":"2025-01-01T00:00:00Z","message":{"role":"assistant","content":"Hi"},"done":false}`,
		`{"model":"llama3","created_at":"2025-01-01T00:00:01Z","message":{"role":"assistant","content":""},"done":true,"done_reason":"stop","prompt_eval_count":5,"eval_count":2}`,
	}
	resp := buildOllamaStreamResp(chunks)
	_, _ = ollamaStreamHandler(c, info, resp)

	afterStream := info.FirstResponseTime.Sub(info.StartTime).Milliseconds()
	assert.GreaterOrEqual(t, afterStream, int64(0), "after stream, frt should be non-negative, got %dms", afterStream)
}