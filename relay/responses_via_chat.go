package relay

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	appconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/claude"
	openaichannel "github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/openaicompat"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// maskSensitiveContent masks message content in a JSON request for privacy logging.
// It replaces the actual content with "[MASKED]" while preserving structure.
func maskSensitiveContent(data []byte) string {
	var req map[string]any
	if err := common.Unmarshal(data, &req); err != nil {
		return string(data) // fallback to original if parse fails
	}

	// Mask content in messages
	if messages, ok := req["messages"].([]any); ok {
		for _, msg := range messages {
			if m, ok := msg.(map[string]any); ok {
				if _, hasContent := m["content"]; hasContent {
					m["content"] = "[MASKED]"
				}
			}
		}
	}

	// Mask input (Responses API format)
	if _, hasInput := req["input"]; hasInput {
		req["input"] = "[MASKED]"
	}

	masked, err := common.Marshal(req)
	if err != nil {
		return string(data)
	}
	return string(masked)
}

// responsesViaChatCompletions handles a Responses API request by converting it
// to ChatCompletions format and sending to the upstream provider.
// This is used as a fallback when the /v1/responses endpoint returns 404.
func responsesViaChatCompletions(c *gin.Context, info *relaycommon.RelayInfo, adaptor channel.Adaptor, responsesReq *dto.OpenAIResponsesRequest) (*dto.Usage, *types.NewAPIError) {
	// Handle previous_response_id: merge context from cache
	if responsesReq.PreviousResponseID != "" {
		previousEntry, found := service.LookupResponsesContext(responsesReq.PreviousResponseID)
		if found {
			// Rebuild input by merging previous output with current input
			mergedInput, err := openaicompat.RebuildResponsesInput(previousEntry, responsesReq.Input)
			if err != nil {
				logger.LogError(c.Request.Context(), "failed to rebuild responses input: "+err.Error())
			} else {
				responsesReq.Input = mergedInput
				logger.LogDebug(c, "merged previous_response_id=%s into input", responsesReq.PreviousResponseID)
			}
		} else {
			logger.LogWarn(c.Request.Context(), "previous_response_id="+responsesReq.PreviousResponseID+" not found in cache")
		}
		// Clear previous_response_id since we've merged it into input
		responsesReq.PreviousResponseID = ""
	}

	// Convert Responses request to ChatCompletions request
	chatReq, err := openaicompat.ResponsesRequestToChatCompletionsRequest(responsesReq)
	if err != nil {
		return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	// Check cache: strip response_format if known to be incompatible with tools
	if service.ShouldStripResponseFormat(info.ChannelId, info.OriginModelName) {
		chatReq.ResponseFormat = nil
		logger.LogDebug(c, "responsesViaChatCompletions: stripped response_format (cached) for channel %d model %s", info.ChannelId, info.OriginModelName)
	}

	// Debug: log the converted request structure
	logger.LogDebug(c, "responsesViaChatCompletions: converted chatReq model=%s, messages_count=%d", chatReq.Model, len(chatReq.Messages))
	for i, msg := range chatReq.Messages {
		logger.LogDebug(c, "  message[%d]: role=%s, content_type=%T", i, msg.Role, msg.Content)
	}

	info.AppendRequestConversion(types.RelayFormatOpenAI)

	// Save and modify relay mode
	savedRelayMode := info.RelayMode
	savedRequestURLPath := info.RequestURLPath
	defer func() {
		info.RelayMode = savedRelayMode
		info.RequestURLPath = savedRequestURLPath
	}()

	info.RelayMode = relayconstant.RelayModeChatCompletions
	info.RequestURLPath = "/v1/chat/completions"
	statusCodeMappingStr := c.GetString("status_code_mapping")

	// Retry loop: max 2 attempts
	// First attempt sends with response_format (if present).
	// If upstream responds with "response format and function call" conflict error,
	// cache the decision and retry without response_format.
	var (
		lastCloser     io.Closer
		lastReqBodyJSON []byte
	)
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			logger.LogDebug(c, "responsesViaChatCompletions: retry attempt %d without response_format for channel %d model %s", attempt, info.ChannelId, info.OriginModelName)
		}

		chatJSON, err := common.Marshal(chatReq)
		if err != nil {
			if lastCloser != nil {
				lastCloser.Close()
			}
			return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}

		chatJSON, err = relaycommon.RemoveDisabledFields(chatJSON, info.ChannelOtherSettings, info.ChannelSetting.PassThroughBodyEnabled)
		if err != nil {
			if lastCloser != nil {
				lastCloser.Close()
			}
			return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}

		if len(info.ParamOverride) > 0 {
			chatJSON, err = relaycommon.ApplyParamOverrideWithRelayInfo(chatJSON, info)
			if err != nil {
				if lastCloser != nil {
					lastCloser.Close()
				}
				return nil, newAPIErrorFromParamOverride(err)
			}
		}

		var overriddenChatReq dto.GeneralOpenAIRequest
		if err := common.Unmarshal(chatJSON, &overriddenChatReq); err != nil {
			if lastCloser != nil {
				lastCloser.Close()
			}
			return nil, types.NewError(err, types.ErrorCodeChannelParamOverrideInvalid, types.ErrOptionWithSkipRetry())
		}

		// Apply system prompt if configured
		applySystemPromptIfNeeded(c, info, &overriddenChatReq)

		// Convert request using adaptor
		convertedRequest, err := adaptor.ConvertOpenAIRequest(c, info, &overriddenChatReq)
		if err != nil {
			if lastCloser != nil {
				lastCloser.Close()
			}
			return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
		relaycommon.AppendRequestConversionFromRequest(info, convertedRequest)

		jsonData, err := common.Marshal(convertedRequest)
		if err != nil {
			if lastCloser != nil {
				lastCloser.Close()
			}
			return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}

		jsonData, err = relaycommon.RemoveDisabledFields(jsonData, info.ChannelOtherSettings, info.ChannelSetting.PassThroughBodyEnabled)
		if err != nil {
			if lastCloser != nil {
				lastCloser.Close()
			}
			return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}

		logger.LogDebug(c, "responsesViaChatCompletions requestBody: %s", jsonData)
		body, size, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
		if err != nil {
			if lastCloser != nil {
				lastCloser.Close()
			}
			return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}

		// Close previous attempt's body if retrying
		if lastCloser != nil {
			lastCloser.Close()
		}
		lastCloser = closer
		lastReqBodyJSON = jsonData

		info.UpstreamRequestBodySize = size
		var requestBody io.Reader = body

		// Send request
		resp, err := adaptor.DoRequest(c, info, requestBody)
		if err != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("responsesViaChatCompletions: DoRequest error: %v, requestBody: %s", err, string(lastReqBodyJSON)))
			closer.Close()
			lastCloser = nil
			return nil, types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
		}
		if resp == nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("responsesViaChatCompletions: nil response, requestBody: %s", string(lastReqBodyJSON)))
			closer.Close()
			lastCloser = nil
			return nil, types.NewOpenAIError(nil, types.ErrorCodeBadResponse, http.StatusInternalServerError)
		}

		httpResp := resp.(*http.Response)
		info.IsStream = info.IsStream || strings.HasPrefix(httpResp.Header.Get("Content-Type"), "text/event-stream")

		if httpResp.StatusCode == http.StatusOK {
			closer.Close()
			lastCloser = nil

			// 如果上游是 Claude 适配器，需要先将 Claude 格式响应转换为 OpenAI ChatCompletions 格式
			if info.ApiType == appconstant.APITypeAnthropic {
				var convErr error
				if info.IsStream {
					httpResp, convErr = convertClaudeStreamToChatCompletions(httpResp, info.UpstreamModelName)
				} else {
					httpResp, convErr = convertClaudeResponseToChatCompletions(httpResp)
				}
				if convErr != nil {
					return nil, types.NewOpenAIError(convErr, types.ErrorCodeBadResponse, http.StatusInternalServerError)
				}
			}

			// Handle response - convert ChatCompletions response back to Responses format
			var usage *dto.Usage
			var newApiErr *types.NewAPIError

			if info.IsStream {
				usage, newApiErr = openaichannel.OaiChatToResponsesStreamHandler(c, info, httpResp, responsesReq)
			} else {
				usage, newApiErr = openaichannel.OaiChatToResponsesHandler(c, info, httpResp, responsesReq)
			}

			if newApiErr != nil {
				service.ResetStatusCode(newApiErr, statusCodeMappingStr)
				return nil, newApiErr
			}
			return usage, nil
		}

		// Error path: read error response body
		errorBody, readErr := io.ReadAll(httpResp.Body)
		httpResp.Body.Close()

		// Check if retryable: 400 error with response_format+tools conflict
		if attempt == 0 && chatReq.ResponseFormat != nil && len(chatReq.Tools) > 0 &&
			readErr == nil && strings.Contains(string(errorBody), "response format and function call") {
			service.MarkStripResponseFormat(info.ChannelId, info.OriginModelName)
			chatReq.ResponseFormat = nil
			logger.LogDebug(c, "responsesViaChatCompletions: detected response_format conflict, will retry without response_format")
			continue // Retry
		}

		// Non-retryable error
		if readErr != nil {
			logger.LogError(c.Request.Context(), "responsesViaChatCompletions: failed to read error response body: "+readErr.Error())
		} else {
			maskedRequest := maskSensitiveContent(lastReqBodyJSON)
			logger.LogError(c.Request.Context(), fmt.Sprintf("responsesViaChatCompletions: upstream error status=%d, body=%s, request=%s", httpResp.StatusCode, string(errorBody), maskedRequest))
		}
		closer.Close()
		lastCloser = nil
		newApiErr := service.RelayErrorHandler(c.Request.Context(), httpResp, false)
		service.ResetStatusCode(newApiErr, statusCodeMappingStr)
		return nil, newApiErr
	}

	// Cleanup if we somehow exit the loop without returning
	if lastCloser != nil {
		lastCloser.Close()
	}
	return nil, types.NewOpenAIError(nil, types.ErrorCodeBadResponse, http.StatusInternalServerError)
}

// convertClaudeResponseToChatCompletions 将 Claude 非流式响应转换为 OpenAI ChatCompletions 格式的 http.Response。
// 如果 Claude 响应包含错误，返回 error 而非伪装成正常的 http.Response。
func convertClaudeResponseToChatCompletions(httpResp *http.Response) (*http.Response, error) {
	body, err := io.ReadAll(httpResp.Body)
	httpResp.Body.Close()
	if err != nil {
		return nil, err
	}

	var claudeResp dto.ClaudeResponse
	if err := common.Unmarshal(body, &claudeResp); err != nil {
		return nil, err
	}

	// 检查 Claude 错误 — 直接返回 error，避免 Claude 格式错误体被传给 OpenAI handler 导致二次解析失败
	if claudeErr := claudeResp.GetClaudeError(); claudeErr != nil && claudeErr.Type != "" {
		return nil, fmt.Errorf("claude upstream error: %s - %s", claudeErr.Type, claudeErr.Message)
	}

	// 提取 usage
	usage := &dto.Usage{}
	if claudeResp.Usage != nil {
		usage.PromptTokens = claudeResp.Usage.InputTokens
		usage.CompletionTokens = claudeResp.Usage.OutputTokens
		usage.TotalTokens = claudeResp.Usage.InputTokens + claudeResp.Usage.OutputTokens
		usage.UsageSemantic = "anthropic"
		usage.PromptTokensDetails.CachedTokens = claudeResp.Usage.CacheReadInputTokens
		usage.PromptTokensDetails.CachedCreationTokens = claudeResp.Usage.CacheCreationInputTokens
		usage.ClaudeCacheCreation5mTokens = claudeResp.Usage.GetCacheCreation5mTokens()
		usage.ClaudeCacheCreation1hTokens = claudeResp.Usage.GetCacheCreation1hTokens()
	}

	// usage 校验：如果 TotalTokens 为零，记录 warning
	if usage.TotalTokens == 0 {
		common.SysLog("convertClaudeResponseToChatCompletions: claude response has zero total tokens, usage may be incorrect")
	}

	// 转换为 OpenAI 格式
	openaiResp := claude.ResponseClaude2OpenAI(&claudeResp)
	openaiResp.Usage = claude.BuildOpenAIStyleUsageFromClaudeUsage(usage)

	openaiJSON, err := common.Marshal(openaiResp)
	if err != nil {
		return nil, err
	}

	return &http.Response{
		StatusCode: httpResp.StatusCode,
		Header:     httpResp.Header,
		Body:       io.NopCloser(bytes.NewReader(openaiJSON)),
	}, nil
}

// convertClaudeStreamToChatCompletions 将 Claude 流式响应转换为 OpenAI ChatCompletions 流式格式的 http.Response。
//
// 已知局限：如果 Claude 流异常截断（没有 message_delta 事件），usage 将只有 promptTokens 而无 completionTokens。
// 正常情况下 Claude API 总会发送 message_delta 包含 output_tokens。
// 此函数无法做 ResponseText2Usage 兜底估算（需要 gin.Context 和累积文本），如需更健壮的兜底，
// 需在 responsesViaChatCompletions 调用方层面处理。
func convertClaudeStreamToChatCompletions(httpResp *http.Response, model string) (*http.Response, error) {
	pr, pw := io.Pipe()

	go func() {
		defer httpResp.Body.Close()
		defer func() {
			if r := recover(); r != nil {
				common.SysError(fmt.Sprintf("panic in convertClaudeStreamToChatCompletions: %v", r))
			}
			pw.Close()
		}()

		scanner := bufio.NewScanner(httpResp.Body)
		scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)

		var (
			promptTokens      int
			completionTokens  int
			cacheReadTokens   int
			cacheCreateTokens int
			cacheCreate5m     int
			cacheCreate1h     int
			actualModel       = model
		)

		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				pw.Write([]byte("data: [DONE]\n\n"))
				break
			}

			var claudeResp dto.ClaudeResponse
			if err := common.UnmarshalJsonStr(data, &claudeResp); err != nil {
				continue
			}

			// 检查错误 — 写入错误信息后终止
			if claudeErr := claudeResp.GetClaudeError(); claudeErr != nil && claudeErr.Type != "" {
				// 构造 OpenAI 格式的错误响应
				errChunk := &dto.ChatCompletionsStreamResponse{
					Id:      fmt.Sprintf("chatcmpl-error-%d", time.Now().UnixNano()),
					Object:  "chat.completion.chunk",
					Created: time.Now().Unix(),
					Model:   actualModel,
					Choices: []dto.ChatCompletionsStreamResponseChoice{
						{
							Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
								Content: common.GetPointer(claudeErr.Message),
							},
							FinishReason: common.GetPointer("stop"),
						},
					},
				}
				errJSON, _ := common.Marshal(errChunk)
				pw.Write([]byte("data: " + string(errJSON) + "\n\n"))
				pw.Write([]byte("data: [DONE]\n\n"))
				break
			}

			// 提取 usage
			// message_start 事件的 usage 在 message.usage 中（嵌套），message_delta 的 usage 在顶层
			var usagePtr *dto.ClaudeUsage
			if claudeResp.Type == "message_start" && claudeResp.Message != nil && claudeResp.Message.Usage != nil {
				usagePtr = claudeResp.Message.Usage
				// 从 message_start 提取实际 model 名
				if claudeResp.Message.Model != "" {
					actualModel = claudeResp.Message.Model
				}
			} else if claudeResp.Usage != nil {
				usagePtr = claudeResp.Usage
			}

			if usagePtr != nil {
				if claudeResp.Type == "message_start" {
					promptTokens = usagePtr.InputTokens
					cacheReadTokens = usagePtr.CacheReadInputTokens
					cacheCreateTokens = usagePtr.CacheCreationInputTokens
					cacheCreate5m = usagePtr.GetCacheCreation5mTokens()
					cacheCreate1h = usagePtr.GetCacheCreation1hTokens()
				} else if claudeResp.Type == "message_delta" {
					completionTokens = usagePtr.OutputTokens
					// message_delta 中也可能包含 cache 字段
					if c5m := usagePtr.GetCacheCreation5mTokens(); c5m > 0 {
						cacheCreate5m = c5m
					}
					if c1h := usagePtr.GetCacheCreation1hTokens(); c1h > 0 {
						cacheCreate1h = c1h
					}
				}
			}

			// 转换为 OpenAI 流式格式
			openaiChunk := claude.StreamResponseClaude2OpenAI(&claudeResp)
			if openaiChunk == nil {
				continue
			}

			chunkJSON, err := common.Marshal(openaiChunk)
			if err != nil {
				continue
			}
			pw.Write([]byte("data: " + string(chunkJSON) + "\n\n"))
		}

		// 发送 final usage chunk
		if promptTokens > 0 || completionTokens > 0 {
			usage := &dto.Usage{
				PromptTokens:     promptTokens,
				CompletionTokens: completionTokens,
				TotalTokens:      promptTokens + completionTokens,
				UsageSemantic:    "anthropic",
			}
			usage.PromptTokensDetails.CachedTokens = cacheReadTokens
			usage.PromptTokensDetails.CachedCreationTokens = cacheCreateTokens
			usage.ClaudeCacheCreation5mTokens = cacheCreate5m
			usage.ClaudeCacheCreation1hTokens = cacheCreate1h

			openaiUsage := claude.BuildOpenAIStyleUsageFromClaudeUsage(usage)
			finalChunk := helper.GenerateFinalUsageResponse("", common.GetTimestamp(), actualModel, openaiUsage)
			finalJSON, err := common.Marshal(finalChunk)
			if err == nil {
				pw.Write([]byte("data: " + string(finalJSON) + "\n\n"))
			}
		}

		// 始终发送 [DONE] 标记结束流（Claude SSE 不包含 [DONE]，需要我们自行添加）
		pw.Write([]byte("data: [DONE]\n\n"))
	}()

	return &http.Response{
		StatusCode: httpResp.StatusCode,
		Header:     httpResp.Header,
		Body:       pr,
	}, nil
}