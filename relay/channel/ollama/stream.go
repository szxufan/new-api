package ollama

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/reasoning"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

type ollamaChatStreamToolCall struct {
	Function struct {
		Name      string      `json:"name"`
		Arguments interface{} `json:"arguments"`
	} `json:"function"`
}

type ollamaChatStreamMessage struct {
	Role      string                     `json:"role"`
	Content   string                     `json:"content"`
	Thinking  json.RawMessage            `json:"thinking"`
	ToolCalls []ollamaChatStreamToolCall `json:"tool_calls"`
}

type ollamaChatStreamChunk struct {
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
	// chat
	Message *ollamaChatStreamMessage `json:"message"`
	// generate
	Response           string `json:"response"`
	Done               bool   `json:"done"`
	DoneReason         string `json:"done_reason"`
	TotalDuration      int64  `json:"total_duration"`
	LoadDuration       int64  `json:"load_duration"`
	PromptEvalCount    int    `json:"prompt_eval_count"`
	PromptEvalCached   *int   `json:"prompt_eval_cached_count"`
	EvalCount          int    `json:"eval_count"`
	PromptEvalDuration int64  `json:"prompt_eval_duration"`
	EvalDuration       int64  `json:"eval_duration"`
}

// promptEvalCachedTokens 将 Ollama 的 prompt_eval_cached_count 归一化为可安全
// 计费的缓存命中 token 数：旧版 Ollama 不上报该字段（nil）或数值非正时不采信，
// 越界时钳制到 prompt 总数。
func promptEvalCachedTokens(cachedCount *int, promptTokens int) int {
	if cachedCount == nil || *cachedCount <= 0 || promptTokens <= 0 {
		return 0
	}
	if *cachedCount > promptTokens {
		return promptTokens
	}
	return *cachedCount
}

// toolCallResponses 将 Ollama 的 tool_calls（arguments 为已解析对象）转换为
// OpenAI 格式（arguments 为 JSON 字符串），id 按 startIndex 起顺序生成 call_N。
func toolCallResponses(calls []ollamaChatStreamToolCall, startIndex int) []dto.ToolCallResponse {
	trs := make([]dto.ToolCallResponse, 0, len(calls))
	for i, tc := range calls {
		argBytes, _ := json.Marshal(tc.Function.Arguments)
		tr := dto.ToolCallResponse{
			ID:       fmt.Sprintf("call_%d", startIndex+i),
			Type:     "function",
			Function: dto.FunctionResponse{Name: tc.Function.Name, Arguments: string(argBytes)},
		}
		trs = append(trs, tr)
	}
	return trs
}

func toUnix(ts string) int64 {
	if ts == "" {
		return time.Now().Unix()
	}
	// try time.RFC3339 or with nanoseconds
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		t2, err2 := time.Parse(time.RFC3339, ts)
		if err2 == nil {
			return t2.Unix()
		}
		return time.Now().Unix()
	}
	return t.Unix()
}

func ollamaStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("empty response"), types.ErrorCodeBadResponse, http.StatusBadRequest)
	}
	defer service.CloseResponseBodyGracefully(resp)

	helper.SetEventStreamHeaders(c)
	scanner := helper.NewStreamScanner(resp.Body)
	usage := &dto.Usage{}
	var model = info.UpstreamModelName
	var responseId = common.GetUUID()
	var created = time.Now().Unix()
	var toolCallIndex int
	var (
		contentBuilder     strings.Builder
		reasoningBuilder   strings.Builder
		collectedToolCalls []dto.ToolCallResponse
	)
	start := helper.GenerateStartEmptyResponse(responseId, created, model, nil)
	if data, err := common.Marshal(start); err == nil {
		_ = helper.StringData(c, string(data))
	}

	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var chunk ollamaChatStreamChunk
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			logger.LogError(c, "ollama stream json decode error: "+err.Error()+" line="+line)
			return usage, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
		if chunk.Model != "" {
			model = chunk.Model
		}
		created = toUnix(chunk.CreatedAt)

		if !chunk.Done {
			info.SetFirstResponseTime()
			var content string
			if chunk.Message != nil {
				content = chunk.Message.Content
			} else {
				content = chunk.Response
			}
			delta := dto.ChatCompletionsStreamResponse{
				Id:      responseId,
				Object:  "chat.completion.chunk",
				Created: created,
				Model:   model,
				Choices: []dto.ChatCompletionsStreamResponseChoice{{
					Index: 0,
					Delta: dto.ChatCompletionsStreamResponseChoiceDelta{Role: "assistant"},
				}},
			}
			if content != "" {
				delta.Choices[0].Delta.SetContentString(content)
				contentBuilder.WriteString(content)
			}
			if chunk.Message != nil && len(chunk.Message.Thinking) > 0 {
				raw := strings.TrimSpace(string(chunk.Message.Thinking))
				if raw != "" && raw != "null" {
					// Unmarshal the JSON string to get the actual content without quotes
					var thinkingContent string
					if err := json.Unmarshal(chunk.Message.Thinking, &thinkingContent); err == nil {
						delta.Choices[0].Delta.SetReasoningContent(thinkingContent)
						reasoningBuilder.WriteString(thinkingContent)
					} else {
						// Fallback to raw string if it's not a JSON string
						delta.Choices[0].Delta.SetReasoningContent(raw)
						reasoningBuilder.WriteString(raw)
					}
				}
			}
			// tool calls
			if chunk.Message != nil && len(chunk.Message.ToolCalls) > 0 {
				trs := toolCallResponses(chunk.Message.ToolCalls, toolCallIndex)
				for i := range trs {
					trs[i].SetIndex(toolCallIndex + i)
				}
				toolCallIndex += len(trs)
				delta.Choices[0].Delta.ToolCalls = trs
				collectedToolCalls = append(collectedToolCalls, trs...)
			}
			if data, err := common.Marshal(delta); err == nil {
				_ = helper.StringData(c, string(data))
			}
			continue
		}
		// done frame
		// finalize once and break loop
		usage.PromptTokens = chunk.PromptEvalCount
		usage.CompletionTokens = chunk.EvalCount
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
		usage.PromptTokensDetails.CachedTokens = promptEvalCachedTokens(chunk.PromptEvalCached, usage.PromptTokens)
		// done 帧本身也可能直接携带 tool_calls（Ollama 一次性下发时不分片）
		if chunk.Message != nil && len(chunk.Message.ToolCalls) > 0 {
			trs := toolCallResponses(chunk.Message.ToolCalls, toolCallIndex)
			for i := range trs {
				trs[i].SetIndex(toolCallIndex + i)
			}
			toolCallIndex += len(trs)
			collectedToolCalls = append(collectedToolCalls, trs...)
			delta := dto.ChatCompletionsStreamResponse{
				Id:      responseId,
				Object:  "chat.completion.chunk",
				Created: created,
				Model:   model,
				Choices: []dto.ChatCompletionsStreamResponseChoice{{
					Index: 0,
					Delta: dto.ChatCompletionsStreamResponseChoiceDelta{Role: "assistant", ToolCalls: trs},
				}},
			}
			if data, err := common.Marshal(delta); err == nil {
				_ = helper.StringData(c, string(data))
			}
		}
		finishReason := chunk.DoneReason
		if finishReason == "" {
			finishReason = "stop"
		}
		// OpenAI 语义：响应含工具调用时 finish_reason 应为 tool_calls
		if len(collectedToolCalls) > 0 && finishReason == "stop" {
			finishReason = "tool_calls"
		}
		// emit stop delta
		if stop := helper.GenerateStopResponse(responseId, created, model, finishReason); stop != nil {
			if data, err := common.Marshal(stop); err == nil {
				_ = helper.StringData(c, string(data))
			}
		}
		// emit usage frame
		if final := helper.GenerateFinalUsageResponse(responseId, created, model, *usage); final != nil {
			if data, err := common.Marshal(final); err == nil {
				_ = helper.StringData(c, string(data))
			}
		}
		// send [DONE]
		helper.Done(c)
		break
	}
	cacheReasoningContentForOllama(c, info, contentBuilder.String(), reasoningBuilder.String(), collectedToolCalls)
	if err := scanner.Err(); err != nil && err != io.EOF {
		logger.LogError(c, "ollama stream scan error: "+err.Error())
	}
	return usage, nil
}

// cacheReasoningContentForOllama stores reasoning content so multi-turn
// requests through fillReasoningContentForDeepSeekThinking can restore it.
func cacheReasoningContentForOllama(c *gin.Context, info *relaycommon.RelayInfo, content string, reasoningContent string, toolCalls []dto.ToolCallResponse) {
	if !reasoning.IsThinkingModel(info.UpstreamModelName) && !reasoning.IsThinkingModel(info.OriginModelName) {
		return
	}
	if reasoningContent == "" && content == "" {
		return
	}
	var toolCallsJSON json.RawMessage
	if len(toolCalls) > 0 {
		if b, err := common.Marshal(toolCalls); err == nil {
			toolCallsJSON = b
		}
	}
	service.StoreReasoningContent(info.TokenKey, content, toolCallsJSON, reasoningContent)
	logger.LogDebug(c, "ollama thinking: cached reasoning_content, content_len=%d reasoning_len=%d", len(content), len(reasoningContent))
}

// non-stream handler for chat/generate
func ollamaChatHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	service.CloseResponseBodyGracefully(resp)
	raw := string(body)
	if common.DebugEnabled {
		println("ollama non-stream raw resp:", raw)
	}

	lines := strings.Split(raw, "\n")
	var (
		aggContent         strings.Builder
		reasoningBuilder   strings.Builder
		collectedToolCalls []dto.ToolCallResponse
		lastChunk          ollamaChatStreamChunk
		parsedAny          bool
	)
	appendToolCalls := func(calls []ollamaChatStreamToolCall) {
		trs := toolCallResponses(calls, len(collectedToolCalls))
		collectedToolCalls = append(collectedToolCalls, trs...)
	}
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		var ck ollamaChatStreamChunk
		if err := json.Unmarshal([]byte(ln), &ck); err != nil {
			if len(lines) == 1 {
				return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
			}
			continue
		}
		parsedAny = true
		lastChunk = ck
		if ck.Message != nil {
			if len(ck.Message.Thinking) > 0 {
				raw := strings.TrimSpace(string(ck.Message.Thinking))
				if raw != "" && raw != "null" {
					// Unmarshal the JSON string to get the actual content without quotes
					var thinkingContent string
					if err := json.Unmarshal(ck.Message.Thinking, &thinkingContent); err == nil {
						reasoningBuilder.WriteString(thinkingContent)
					} else {
						// Fallback to raw string if it's not a JSON string
						reasoningBuilder.WriteString(raw)
					}
				}
			}
			if ck.Message.Content != "" {
				aggContent.WriteString(ck.Message.Content)
			}
			if len(ck.Message.ToolCalls) > 0 {
				appendToolCalls(ck.Message.ToolCalls)
			}
		} else if ck.Response != "" {
			aggContent.WriteString(ck.Response)
		}
	}

	if !parsedAny {
		var single ollamaChatStreamChunk
		if err := json.Unmarshal(body, &single); err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
		lastChunk = single
		if single.Message != nil {
			if len(single.Message.Thinking) > 0 {
				raw := strings.TrimSpace(string(single.Message.Thinking))
				if raw != "" && raw != "null" {
					// Unmarshal the JSON string to get the actual content without quotes
					var thinkingContent string
					if err := json.Unmarshal(single.Message.Thinking, &thinkingContent); err == nil {
						reasoningBuilder.WriteString(thinkingContent)
					} else {
						// Fallback to raw string if it's not a JSON string
						reasoningBuilder.WriteString(raw)
					}
				}
			}
			aggContent.WriteString(single.Message.Content)
			if len(single.Message.ToolCalls) > 0 {
				appendToolCalls(single.Message.ToolCalls)
			}
		} else {
			aggContent.WriteString(single.Response)
		}
	}

	model := lastChunk.Model
	if model == "" {
		model = info.UpstreamModelName
	}
	created := toUnix(lastChunk.CreatedAt)
	usage := &dto.Usage{PromptTokens: lastChunk.PromptEvalCount, CompletionTokens: lastChunk.EvalCount, TotalTokens: lastChunk.PromptEvalCount + lastChunk.EvalCount}
	usage.PromptTokensDetails.CachedTokens = promptEvalCachedTokens(lastChunk.PromptEvalCached, usage.PromptTokens)
	content := aggContent.String()
	finishReason := lastChunk.DoneReason
	if finishReason == "" {
		finishReason = "stop"
	}
	// OpenAI 语义：响应含工具调用时 finish_reason 应为 tool_calls
	if len(collectedToolCalls) > 0 && finishReason == "stop" {
		finishReason = "tool_calls"
	}

	msg := dto.Message{Role: "assistant", Content: contentPtr(content)}
	if rc := reasoningBuilder.String(); rc != "" {
		msg.ReasoningContent = &rc
	}
	if len(collectedToolCalls) > 0 {
		if b, err := common.Marshal(collectedToolCalls); err == nil {
			msg.ToolCalls = b
		}
	}
	full := dto.OpenAITextResponse{
		Id:      common.GetUUID(),
		Model:   model,
		Object:  "chat.completion",
		Created: created,
		Choices: []dto.OpenAITextResponseChoice{{
			Index:        0,
			Message:      msg,
			FinishReason: finishReason,
		}},
		Usage: *usage,
	}
	out, _ := common.Marshal(full)
	service.IOCopyBytesGracefully(c, resp, out)
	cacheReasoningContentForOllama(c, info, content, reasoningBuilder.String(), collectedToolCalls)
	return usage, nil
}

func contentPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
