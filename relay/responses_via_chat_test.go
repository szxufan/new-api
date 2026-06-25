package relay

import (
	"bufio"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

// TestConvertClaudeResponseToChatCompletions 测试 Claude 非流式响应转换为 OpenAI ChatCompletions 格式
func TestConvertClaudeResponseToChatCompletions(t *testing.T) {
	claudeJSON := `{
		"id": "msg_12345",
		"type": "message",
		"role": "assistant",
		"model": "claude-sonnet-4-20250514",
		"content": [
			{"type": "text", "text": "Hello! How can I help you?"}
		],
		"stop_reason": "end_turn",
		"usage": {
			"input_tokens": 10,
			"output_tokens": 20
		}
	}`

	httpResp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(claudeJSON)),
	}

	convertedResp, err := convertClaudeResponseToChatCompletions(httpResp)
	require.NoError(t, err)
	require.NotNil(t, convertedResp)

	body, err := io.ReadAll(convertedResp.Body)
	require.NoError(t, err)

	var openaiResp dto.OpenAITextResponse
	err = common.Unmarshal(body, &openaiResp)
	require.NoError(t, err)

	// 验证基本字段
	require.Equal(t, "chat.completion", openaiResp.Object)
	require.Equal(t, "msg_12345", openaiResp.Id)
	require.Len(t, openaiResp.Choices, 1)
	require.Equal(t, "assistant", openaiResp.Choices[0].Message.Role)

	// 验证 usage 转换（BuildOpenAIStyleUsageFromClaudeUsage 会将 input_tokens 映射为 prompt_tokens）
	require.Greater(t, openaiResp.Usage.PromptTokens, 0)
	require.Equal(t, 20, openaiResp.Usage.CompletionTokens)
	require.Greater(t, openaiResp.Usage.TotalTokens, 0)
}

// TestConvertClaudeResponseToChatCompletionsWithError 测试 Claude 错误响应返回 error
func TestConvertClaudeResponseToChatCompletionsWithError(t *testing.T) {
	claudeJSON := `{
		"type": "error",
		"error": {
			"type": "overloaded_error",
			"message": "Overloaded"
		}
	}`

	httpResp := &http.Response{
		StatusCode: http.StatusServiceUnavailable,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(claudeJSON)),
	}

	_, err := convertClaudeResponseToChatCompletions(httpResp)
	require.Error(t, err)
	require.Contains(t, err.Error(), "claude upstream error")
	require.Contains(t, err.Error(), "overloaded_error")
}

// TestConvertClaudeResponseToChatCompletionsWithToolUse 测试包含 tool_use 的 Claude 响应
func TestConvertClaudeResponseToChatCompletionsWithToolUse(t *testing.T) {
	claudeJSON := `{
		"id": "msg_12345",
		"type": "message",
		"role": "assistant",
		"model": "claude-sonnet-4-20250514",
		"content": [
			{"type": "text", "text": "I'll use the tool."},
			{"type": "tool_use", "id": "toolu_123", "name": "get_weather", "input": {"city": "SF"}}
		],
		"stop_reason": "tool_use",
		"usage": {
			"input_tokens": 15,
			"output_tokens": 25
		}
	}`

	httpResp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(claudeJSON)),
	}

	convertedResp, err := convertClaudeResponseToChatCompletions(httpResp)
	require.NoError(t, err)

	body, err := io.ReadAll(convertedResp.Body)
	require.NoError(t, err)

	var openaiResp dto.OpenAITextResponse
	err = common.Unmarshal(body, &openaiResp)
	require.NoError(t, err)

	// 验证 tool_calls 转换（直接检查 JSON，因为 ParseToolCalls 返回 ToolCallRequest 而非 ToolCallResponse）
	toolCallsJSON := string(openaiResp.Choices[0].Message.ToolCalls)
	require.NotEmpty(t, toolCallsJSON)
	require.Contains(t, toolCallsJSON, "toolu_123")
	require.Contains(t, toolCallsJSON, "get_weather")
}

// TestConvertClaudeStreamToChatCompletions 测试 Claude 流式响应转换为 OpenAI ChatCompletions 流式格式
func TestConvertClaudeStreamToChatCompletions(t *testing.T) {
	// 构造 Claude SSE 流
	claudeSSE := strings.Join([]string{
		`event: message_start`,
		`data: {"type":"message_start","message":{"id":"msg_123","model":"claude-sonnet-4-20250514","usage":{"input_tokens":10,"output_tokens":1}}}`,
		``,
		`event: content_block_start`,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" world"}}`,
		``,
		`event: content_block_stop`,
		`data: {"type":"content_block_stop","index":0}`,
		``,
		`event: message_delta`,
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}`,
		``,
		`event: message_stop`,
		`data: {"type":"message_stop"}`,
		``,
	}, "\n")

	httpResp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(claudeSSE)),
	}

	convertedResp, err := convertClaudeStreamToChatCompletions(httpResp, "claude-sonnet-4-20250514")
	require.NoError(t, err)
	require.NotNil(t, convertedResp)

	// 读取转换后的流
	scanner := bufio.NewScanner(convertedResp.Body)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)

	var chunks []dto.ChatCompletionsStreamResponse
	var sawDone bool
	var finalUsageChunk *dto.ChatCompletionsStreamResponse

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			sawDone = true
			break
		}

		var chunk dto.ChatCompletionsStreamResponse
		err := common.Unmarshal([]byte(data), &chunk)
		require.NoError(t, err, "failed to parse chunk: %s", data)

		// 检查是否是 final usage chunk（有 usage 且 choices 为空或 finish_reason 为 stop）
		if chunk.Usage != nil && chunk.Usage.TotalTokens > 0 && len(chunk.Choices) == 0 {
			finalUsageChunk = &chunk
		}
		chunks = append(chunks, chunk)
	}

	require.True(t, sawDone, "stream should end with [DONE]")
	require.NotEmpty(t, chunks, "should have at least one chunk")

	// 验证从 message_start 提取的 model 名
	for _, chunk := range chunks {
		if chunk.Model != "" {
			require.Equal(t, "claude-sonnet-4-20250514", chunk.Model)
			break
		}
	}

	// 验证 final usage chunk
	require.NotNil(t, finalUsageChunk, "should have a final usage chunk")
	require.Equal(t, 10, finalUsageChunk.Usage.PromptTokens)
	require.Equal(t, 5, finalUsageChunk.Usage.CompletionTokens)
}

// TestConvertClaudeStreamToChatCompletionsWithError 测试 Claude 流式错误事件
func TestConvertClaudeStreamToChatCompletionsWithError(t *testing.T) {
	claudeSSE := strings.Join([]string{
		`event: error`,
		`data: {"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`,
		``,
	}, "\n")

	httpResp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(claudeSSE)),
	}

	convertedResp, err := convertClaudeStreamToChatCompletions(httpResp, "claude-sonnet-4-20250514")
	require.NoError(t, err)

	scanner := bufio.NewScanner(convertedResp.Body)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)

	var sawDone bool
	var errorChunk *dto.ChatCompletionsStreamResponse

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			sawDone = true
			break
		}

		var chunk dto.ChatCompletionsStreamResponse
		_ = common.Unmarshal([]byte(data), &chunk)
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != nil {
			errorChunk = &chunk
		}
	}

	require.True(t, sawDone, "stream should end with [DONE] even on error")
	require.NotNil(t, errorChunk, "should have an error chunk with the error message")
	require.Contains(t, *errorChunk.Choices[0].Delta.Content, "Overloaded")
}
