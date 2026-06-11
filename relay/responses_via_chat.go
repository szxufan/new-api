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

	// Debug: log the converted request structure
	logger.LogDebug(c, "responsesViaChatCompletions: converted chatReq model=%s, messages_count=%d", chatReq.Model, len(chatReq.Messages))
	for i, msg := range chatReq.Messages {
		logger.LogDebug(c, "  message[%d]: role=%s, content_type=%T", i, msg.Role, msg.Content)
	}

	chatJSON, err := common.Marshal(chatReq)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}

	chatJSON, err = relaycommon.RemoveDisabledFields(chatJSON, info.ChannelOtherSettings, info.ChannelSetting.PassThroughBodyEnabled)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}

	if len(info.ParamOverride) > 0 {
		chatJSON, err = relaycommon.ApplyParamOverrideWithRelayInfo(chatJSON, info)
		if err != nil {
			return nil, newAPIErrorFromParamOverride(err)
		}
	}

	var overriddenChatReq dto.GeneralOpenAIRequest
	if err := common.Unmarshal(chatJSON, &overriddenChatReq); err != nil {
		return nil, types.NewError(err, types.ErrorCodeChannelParamOverrideInvalid, types.ErrOptionWithSkipRetry())
	}

	// Apply system prompt if configured
	applySystemPromptIfNeeded(c, info, &overriddenChatReq)

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

	// Convert request using adaptor
	convertedRequest, err := adaptor.ConvertOpenAIRequest(c, info, &overriddenChatReq)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	relaycommon.AppendRequestConversionFromRequest(info, convertedRequest)

	jsonData, err := common.Marshal(convertedRequest)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}

	jsonData, err = relaycommon.RemoveDisabledFields(jsonData, info.ChannelOtherSettings, info.ChannelSetting.PassThroughBodyEnabled)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}

	logger.LogDebug(c, "responsesViaChatCompletions requestBody: %s", jsonData)
	body, size, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	defer closer.Close()
	requestBodyJSON := jsonData // Save for error logging
	jsonData = nil
	info.UpstreamRequestBodySize = size
	var requestBody io.Reader = body

	// Send request
	var httpResp *http.Response
	resp, err := adaptor.DoRequest(c, info, requestBody)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("responsesViaChatCompletions: DoRequest error: %v, requestBody: %s", err, string(requestBodyJSON)))
		return nil, types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}
	if resp == nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("responsesViaChatCompletions: nil response, requestBody: %s", string(requestBodyJSON)))
		return nil, types.NewOpenAIError(nil, types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}

	statusCodeMappingStr := c.GetString("status_code_mapping")

	httpResp = resp.(*http.Response)
	info.IsStream = info.IsStream || strings.HasPrefix(httpResp.Header.Get("Content-Type"), "text/event-stream")
	if httpResp.StatusCode != http.StatusOK {
		// Read error response body for debugging
		errorBody, readErr := io.ReadAll(httpResp.Body)
		if readErr != nil {
			logger.LogError(c.Request.Context(), "responsesViaChatCompletions: failed to read error response body: "+readErr.Error())
		} else {
			// Log request with content masked for privacy
			maskedRequest := maskSensitiveContent(requestBodyJSON)
			logger.LogError(c.Request.Context(), fmt.Sprintf("responsesViaChatCompletions: upstream error status=%d, body=%s, request=%s", httpResp.StatusCode, string(errorBody), maskedRequest))
		}
		newApiErr := service.RelayErrorHandler(c.Request.Context(), httpResp, false)
		service.ResetStatusCode(newApiErr, statusCodeMappingStr)
		return nil, newApiErr
	}

	// Handle response - convert ChatCompletions response back to Responses format
	// Use OpenAI-specific handlers since we're using OpenAI-compatible format
	// Pass the original Responses request for context storage
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