package relay

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relay/channel"
	openaichannel "github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
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