package ollama

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"

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

func TestOllamaStreamHandler_CachesReasoningContent(t *testing.T) {
	info := newTestRelayInfo(true)
	info.TokenKey = "tok-stream-thinking"
	info.ChannelMeta.UpstreamModelName = "deepseek-reasoner"
	c, _ := newTestContext()

	chunks := []string{
		`{"model":"deepseek-reasoner","created_at":"2025-01-01T00:00:00Z","message":{"role":"assistant","content":"","thinking":"step one"},"done":false}`,
		`{"model":"deepseek-reasoner","created_at":"2025-01-01T00:00:01Z","message":{"role":"assistant","content":"The answer is 42"},"done":false}`,
		`{"model":"deepseek-reasoner","created_at":"2025-01-01T00:00:02Z","message":{"role":"assistant","content":""},"done":true,"done_reason":"stop","prompt_eval_count":10,"eval_count":5}`,
	}
	resp := buildOllamaStreamResp(chunks)

	_, apiErr := ollamaStreamHandler(c, info, resp)
	require.Nil(t, apiErr)

	rc, found := service.LookupReasoningContent(info.TokenKey, "The answer is 42", nil)
	require.True(t, found, "reasoning content should be cached after stream")
	assert.Equal(t, "step one", rc)
}

func TestOllamaStreamHandler_NoCacheForNonThinkingModel(t *testing.T) {
	info := newTestRelayInfo(true)
	info.TokenKey = "tok-stream-no-thinking"
	info.ChannelMeta.UpstreamModelName = "llama3"
	c, _ := newTestContext()

	chunks := []string{
		`{"model":"llama3","created_at":"2025-01-01T00:00:00Z","message":{"role":"assistant","content":"","thinking":"hm"},"done":false}`,
		`{"model":"llama3","created_at":"2025-01-01T00:00:01Z","message":{"role":"assistant","content":"answer"},"done":true,"done_reason":"stop","prompt_eval_count":1,"eval_count":1}`,
	}
	resp := buildOllamaStreamResp(chunks)

	_, apiErr := ollamaStreamHandler(c, info, resp)
	require.Nil(t, apiErr)

	_, found := service.LookupReasoningContent(info.TokenKey, "answer", nil)
	assert.False(t, found, "reasoning content should not be cached for non-thinking models")
}

func TestOllamaNonStreamHandler_CachesReasoningContent(t *testing.T) {
	info := newTestRelayInfo(false)
	info.TokenKey = "tok-nonstream-thinking"
	info.ChannelMeta.UpstreamModelName = "deepseek-reasoner"
	c, _ := newTestContext()

	body := `{"model":"deepseek-reasoner","created_at":"2025-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello world","thinking":"step one"},"done":true,"done_reason":"stop","prompt_eval_count":10,"eval_count":5}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       newReadCloser(body),
	}

	usage, apiErr := ollamaChatHandler(c, info, resp)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)

	rc, found := service.LookupReasoningContent(info.TokenKey, "Hello world", nil)
	require.True(t, found, "reasoning content should be cached for non-stream response")
	assert.Equal(t, "step one", rc)
}

func TestOpenAIChatToOllamaChat_MapsReasoningContentToThinking(t *testing.T) {
	c, _ := newTestContext()
	rc := "prior reasoning"
	req := &dto.GeneralOpenAIRequest{
		Model: "deepseek-reasoner",
		Messages: []dto.Message{
			{Role: "assistant", Content: "hi", ReasoningContent: &rc},
		},
	}

	chatReq, err := openAIChatToOllamaChat(c, req)
	require.Nil(t, err)
	require.Len(t, chatReq.Messages, 1)

	require.NotNil(t, chatReq.Messages[0].Thinking)
	var thinking string
	require.Nil(t, json.Unmarshal(chatReq.Messages[0].Thinking, &thinking))
	assert.Equal(t, "prior reasoning", thinking)

	// message without reasoning content should keep thinking unset
	req.Messages[0].ReasoningContent = nil
	chatReq, err = openAIChatToOllamaChat(c, req)
	require.Nil(t, err)
	assert.Nil(t, chatReq.Messages[0].Thinking)
}

func TestOllamaStreamHandler_CachedTokensExtracted(t *testing.T) {
	info := newTestRelayInfo(true)
	c, _ := newTestContext()

	chunks := []string{
		`{"model":"pro:latest","created_at":"2025-01-01T00:00:00Z","message":{"role":"assistant","content":"Hi"},"done":false}`,
		`{"model":"pro:latest","created_at":"2025-01-01T00:00:01Z","message":{"role":"assistant","content":""},"done":true,"done_reason":"stop","prompt_eval_count":10,"prompt_eval_cached_count":6,"eval_count":5}`,
	}
	resp := buildOllamaStreamResp(chunks)

	usage, apiErr := ollamaStreamHandler(c, info, resp)

	require.Nil(t, apiErr)
	assert.Equal(t, 10, usage.PromptTokens)
	assert.Equal(t, 6, usage.PromptTokensDetails.CachedTokens, "cached tokens should be surfaced in usage")
}

func TestOllamaStreamHandler_CachedTokensClamped(t *testing.T) {
	info := newTestRelayInfo(true)
	c, _ := newTestContext()

	chunks := []string{
		`{"model":"pro:latest","created_at":"2025-01-01T00:00:00Z","message":{"role":"assistant","content":"Hi"},"done":true,"done_reason":"stop","prompt_eval_count":10,"prompt_eval_cached_count":99,"eval_count":5}`,
	}
	resp := buildOllamaStreamResp(chunks)

	usage, apiErr := ollamaStreamHandler(c, info, resp)

	require.Nil(t, apiErr)
	assert.Equal(t, 10, usage.PromptTokensDetails.CachedTokens, "cached tokens should be clamped to prompt total")
}

func TestOllamaStreamHandler_CachedTokensAbsent(t *testing.T) {
	info := newTestRelayInfo(true)
	c, _ := newTestContext()

	chunks := []string{
		`{"model":"llama3","created_at":"2025-01-01T00:00:00Z","message":{"role":"assistant","content":"Hi"},"done":true,"done_reason":"stop","prompt_eval_count":10,"eval_count":5}`,
	}
	resp := buildOllamaStreamResp(chunks)

	usage, apiErr := ollamaStreamHandler(c, info, resp)

	require.Nil(t, apiErr)
	assert.Equal(t, 0, usage.PromptTokensDetails.CachedTokens, "old ollama without cached count should leave cached tokens zero")
}

func TestOllamaNonStreamHandler_CachedTokensExtracted(t *testing.T) {
	info := newTestRelayInfo(false)
	c, _ := newTestContext()

	body := `{"model":"pro:latest","created_at":"2025-01-01T00:00:00Z","message":{"role":"assistant","content":"Hello"},"done":true,"done_reason":"stop","prompt_eval_count":10,"prompt_eval_cached_count":4,"eval_count":5}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       newReadCloser(body),
	}

	usage, apiErr := ollamaChatHandler(c, info, resp)

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.Equal(t, 10, usage.PromptTokens)
	assert.Equal(t, 4, usage.PromptTokensDetails.CachedTokens)
}

func TestPromptEvalCachedTokens(t *testing.T) {
	six, zero, negative := 6, 0, -3
	assert.Equal(t, 0, promptEvalCachedTokens(nil, 10), "nil should yield 0")
	assert.Equal(t, 0, promptEvalCachedTokens(&zero, 10), "non-positive should yield 0")
	assert.Equal(t, 0, promptEvalCachedTokens(&negative, 10), "negative should yield 0")
	assert.Equal(t, 6, promptEvalCachedTokens(&six, 10), "in-range value should pass through")
	assert.Equal(t, 5, promptEvalCachedTokens(&six, 5), "value over prompt total should clamp")
	assert.Equal(t, 0, promptEvalCachedTokens(&six, 0), "zero prompt should yield 0")
}

func TestOpenAIChatToOllamaChat_ToolCallIdMapsToToolName(t *testing.T) {
	c, _ := newTestContext()
	req := &dto.GeneralOpenAIRequest{
		Model: "pro:latest",
		Messages: []dto.Message{
			{Role: "user", Content: "what is the weather in Toronto?"},
			{Role: "assistant", Content: "", ToolCalls: json.RawMessage(`[{"id":"call_0","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"Toronto\"}"}},{"id":"call_1","type":"function","function":{"name":"get_time","arguments":"{}"}}]`)},
			{Role: "tool", ToolCallId: "call_0", Content: "11 degrees celsius"},
			{Role: "tool", ToolCallId: "call_1", Content: "10:00"},
		},
	}

	chatReq, err := openAIChatToOllamaChat(c, req)
	require.Nil(t, err)
	require.Len(t, chatReq.Messages, 4)

	// assistant tool_calls 的 arguments 应转为对象
	require.Len(t, chatReq.Messages[1].ToolCalls, 2)
	assert.Equal(t, "get_weather", chatReq.Messages[1].ToolCalls[0].Function.Name)
	assert.Equal(t, map[string]any{"city": "Toronto"}, chatReq.Messages[1].ToolCalls[0].Function.Arguments)

	// tool 消息按 tool_call_id 反查 tool_name（OpenAI 约定，无 name 字段）
	assert.Equal(t, "get_weather", chatReq.Messages[2].ToolName)
	assert.Equal(t, "get_time", chatReq.Messages[3].ToolName)
}

func TestOpenAIChatToOllamaChat_ToolNameFallback(t *testing.T) {
	c, _ := newTestContext()
	name := "legacy_tool"
	req := &dto.GeneralOpenAIRequest{
		Model: "pro:latest",
		Messages: []dto.Message{
			{Role: "tool", ToolCallId: "call_unknown", Name: &name, Content: "result"},
		},
	}

	chatReq, err := openAIChatToOllamaChat(c, req)
	require.Nil(t, err)
	assert.Equal(t, "legacy_tool", chatReq.Messages[0].ToolName, "Name should be used when tool_call_id cannot be resolved")
}

func TestOllamaStreamHandler_DoneFrameToolCalls(t *testing.T) {
	info := newTestRelayInfo(true)
	c, w := newTestContext()

	// Ollama 一次性下发：done:true 帧直接携带 tool_calls
	chunks := []string{
		`{"model":"pro:latest","created_at":"2025-01-01T00:00:00Z","message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"get_weather","arguments":{"city":"Toronto"}}}]},"done":true,"done_reason":"stop","prompt_eval_count":15,"eval_count":8}`,
	}
	resp := buildOllamaStreamResp(chunks)

	usage, apiErr := ollamaStreamHandler(c, info, resp)

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	body := w.Body.String()
	assert.Contains(t, body, `"name":"get_weather"`, "tool call delta should be emitted for done-frame tool calls")
	assert.Contains(t, body, `"finish_reason":"tool_calls"`, "finish reason should be tool_calls when tool calls present")
}

func TestOllamaStreamHandler_ToolCallsFinishReason(t *testing.T) {
	info := newTestRelayInfo(true)
	c, w := newTestContext()

	chunks := []string{
		`{"model":"llama3","created_at":"2025-01-01T00:00:00Z","message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"get_weather","arguments":{"city":"Beijing"}}}]},"done":false}`,
		`{"model":"llama3","created_at":"2025-01-01T00:00:01Z","message":{"role":"assistant","content":""},"done":true,"done_reason":"stop","prompt_eval_count":15,"eval_count":8}`,
	}
	resp := buildOllamaStreamResp(chunks)

	_, apiErr := ollamaStreamHandler(c, info, resp)

	require.Nil(t, apiErr)
	assert.Contains(t, w.Body.String(), `"finish_reason":"tool_calls"`)
}

func TestOllamaNonStreamHandler_ToolCallsExtracted(t *testing.T) {
	info := newTestRelayInfo(false)
	c, w := newTestContext()

	body := `{"model":"llama3.2","created_at":"2025-01-01T00:00:00Z","message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"get_weather","arguments":{"city":"Toronto"}}}]},"done":true,"done_reason":"stop","prompt_eval_count":15,"eval_count":8}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       newReadCloser(body),
	}

	usage, apiErr := ollamaChatHandler(c, info, resp)

	require.Nil(t, apiErr)
	require.NotNil(t, usage)

	var out struct {
		Choices []struct {
			Message struct {
				ToolCalls []struct {
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	require.Nil(t, json.Unmarshal(w.Body.Bytes(), &out))
	require.Len(t, out.Choices, 1)
	require.Len(t, out.Choices[0].Message.ToolCalls, 1)
	assert.Equal(t, "call_0", out.Choices[0].Message.ToolCalls[0].ID)
	assert.Equal(t, "get_weather", out.Choices[0].Message.ToolCalls[0].Function.Name)
	assert.JSONEq(t, `{"city":"Toronto"}`, out.Choices[0].Message.ToolCalls[0].Function.Arguments)
	assert.Equal(t, "tool_calls", out.Choices[0].FinishReason)
}

func TestOpenAIChatToOllamaChat_MultiRoundToolCallIdScoping(t *testing.T) {
	c, _ := newTestContext()
	// 多轮工具调用：网关生成的 call_N 每轮从 0 重新编号，历史中存在重复 id。
	// 每条 tool 消息必须解析到它紧邻前一条 assistant 的工具名，而不是全局最后注册的。
	req := &dto.GeneralOpenAIRequest{
		Model: "pro:latest",
		Messages: []dto.Message{
			{Role: "user", Content: "generate an image then tell the time"},
			{Role: "assistant", Content: "", ToolCalls: json.RawMessage(`[{"id":"call_0","type":"function","function":{"name":"generate_image","arguments":"{\"prompt\":\"cat\"}"}},{"id":"call_1","type":"function","function":{"name":"get_time","arguments":"{}"}}]`)},
			{Role: "tool", ToolCallId: "call_0", Content: "img.png"},
			{Role: "tool", ToolCallId: "call_1", Content: "10:00"},
			{Role: "assistant", Content: "", ToolCalls: json.RawMessage(`[{"id":"call_0","type":"function","function":{"name":"search","arguments":"{}"}}]`)},
			{Role: "tool", ToolCallId: "call_0", Content: "results"},
		},
	}

	chatReq, err := openAIChatToOllamaChat(c, req)
	require.Nil(t, err)
	require.Len(t, chatReq.Messages, 6)

	assert.Equal(t, "generate_image", chatReq.Messages[2].ToolName, "round-1 tool result must resolve to round-1 tool name")
	assert.Equal(t, "get_time", chatReq.Messages[3].ToolName)
	assert.Equal(t, "search", chatReq.Messages[5].ToolName)
}

func TestOpenAIChatToOllamaChat_ObjectArgumentsTolerated(t *testing.T) {
	c, _ := newTestContext()
	// 部分 Agent 回传 assistant tool_calls 时 arguments 是对象而非字符串
	req := &dto.GeneralOpenAIRequest{
		Model: "pro:latest",
		Messages: []dto.Message{
			{Role: "assistant", Content: "", ToolCalls: json.RawMessage(`[{"id":"call_0","type":"function","function":{"name":"get_weather","arguments":{"city":"Toronto"}}}]`)},
			{Role: "tool", ToolCallId: "call_0", Content: "11 degrees"},
		},
	}

	chatReq, err := openAIChatToOllamaChat(c, req)
	require.Nil(t, err)
	require.Len(t, chatReq.Messages, 2)

	require.Len(t, chatReq.Messages[0].ToolCalls, 1)
	assert.Equal(t, "get_weather", chatReq.Messages[0].ToolCalls[0].Function.Name)
	assert.Equal(t, map[string]any{"city": "Toronto"}, chatReq.Messages[0].ToolCalls[0].Function.Arguments)
	assert.Equal(t, "get_weather", chatReq.Messages[1].ToolName, "tool_name must still resolve when arguments are object-form")
}
