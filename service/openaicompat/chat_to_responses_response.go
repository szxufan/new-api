package openaicompat

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/dto"
	"github.com/samber/lo"
)

// ChatCompletionsResponseToResponsesResponse converts a ChatCompletions API response
// to a Responses API response format.
// This is the reverse of ResponsesResponseToChatCompletionsResponse.
func ChatCompletionsResponseToResponsesResponse(chatResp *dto.OpenAITextResponse) (*dto.OpenAIResponsesResponse, error) {
	if chatResp == nil {
		return nil, fmt.Errorf("response is nil")
	}

	// Generate a Responses-style ID if not present (resp_ prefix)
	respID := chatResp.Id
	if respID == "" || !isResponsesID(respID) {
		respID = fmt.Sprintf("resp_%d", time.Now().UnixNano())
	}

	// Convert created timestamp
	createdAt := 0
	switch v := chatResp.Created.(type) {
	case int:
		createdAt = v
	case int64:
		createdAt = int(v)
	case float64:
		createdAt = int(v)
	}

	// Build output array from choices
	output := make([]dto.ResponsesOutput, 0)
	for _, choice := range chatResp.Choices {
		// Build content array from message
		content := make([]dto.ResponsesOutputContent, 0)

		// Add text content if present
		if choice.Message.IsStringContent() {
			text := choice.Message.StringContent()
			if text != "" {
				content = append(content, dto.ResponsesOutputContent{
					Type: "output_text",
					Text: text,
				})
			}
		} else {
			// Handle multi-part content
			mediaContent := choice.Message.ParseContent()
			for _, mc := range mediaContent {
				if mc.Type == "text" || mc.Type == "output_text" {
					content = append(content, dto.ResponsesOutputContent{
						Type: "output_text",
						Text: mc.Text,
					})
				}
			}
		}

		// Create message output
		msgOutput := dto.ResponsesOutput{
			Type:    "message",
			ID:      fmt.Sprintf("msg_%d", time.Now().UnixNano()),
			Status:  "completed",
			Role:    "assistant",
			Content: content,
		}
		output = append(output, msgOutput)

		// Handle tool calls
		toolCalls := choice.Message.ParseToolCalls()
		for _, tc := range toolCalls {
			toolOutput := dto.ResponsesOutput{
				Type:      "function_call",
				CallId:    tc.ID,
				Name:      tc.Function.Name,
				Arguments: json.RawMessage(tc.Function.Arguments),
			}
			output = append(output, toolOutput)
		}
	}

	// Convert usage to ResponsesUsage format
	var usage *dto.ResponsesUsage
	if chatResp.Usage.TotalTokens > 0 {
		usage = &dto.ResponsesUsage{
			InputTokens:  chatResp.Usage.PromptTokens,
			OutputTokens: chatResp.Usage.CompletionTokens,
			TotalTokens:  chatResp.Usage.TotalTokens,
		}

		// Add input_tokens_details if there are any details
		if chatResp.Usage.PromptTokensDetails.CachedTokens > 0 ||
			chatResp.Usage.PromptTokensDetails.ImageTokens > 0 ||
			chatResp.Usage.PromptTokensDetails.AudioTokens > 0 ||
			chatResp.Usage.PromptTokensDetails.TextTokens > 0 {
			usage.InputTokensDetails = &dto.InputTokenDetails{
				CachedTokens: chatResp.Usage.PromptTokensDetails.CachedTokens,
				TextTokens:   chatResp.Usage.PromptTokensDetails.TextTokens,
				AudioTokens:  chatResp.Usage.PromptTokensDetails.AudioTokens,
				ImageTokens:  chatResp.Usage.PromptTokensDetails.ImageTokens,
			}
		}

		// Add output_tokens_details if there are any details
		if chatResp.Usage.CompletionTokenDetails.ReasoningTokens > 0 ||
			chatResp.Usage.CompletionTokenDetails.TextTokens > 0 ||
			chatResp.Usage.CompletionTokenDetails.AudioTokens > 0 ||
			chatResp.Usage.CompletionTokenDetails.ImageTokens > 0 {
			usage.OutputTokensDetails = &dto.OutputTokenDetails{
				TextTokens:      chatResp.Usage.CompletionTokenDetails.TextTokens,
				AudioTokens:     chatResp.Usage.CompletionTokenDetails.AudioTokens,
				ImageTokens:     chatResp.Usage.CompletionTokenDetails.ImageTokens,
				ReasoningTokens: chatResp.Usage.CompletionTokenDetails.ReasoningTokens,
			}
		}
	}

	// Build Responses response
	resp := &dto.OpenAIResponsesResponse{
		ID:               respID,
		Object:           "response",
		CreatedAt:        createdAt,
		Status:           []byte(`"completed"`),
		Model:            chatResp.Model,
		Output:           output,
		Usage:            usage,
		ParallelToolCalls: lo.ToPtr(true),
	}

	return resp, nil
}

// isResponsesID checks if the ID follows Responses API format (resp_ prefix)
func isResponsesID(id string) bool {
	return len(id) > 5 && id[:5] == "resp_"
}

// ChatCompletionsStreamResponseToResponsesStreamResponse converts a ChatCompletions stream chunk
// to a Responses API stream event format.
func ChatCompletionsStreamResponseToResponsesStreamResponse(chunk *dto.ChatCompletionsStreamResponse) (map[string]any, error) {
	if chunk == nil {
		return nil, fmt.Errorf("chunk is nil")
	}

	// Responses stream events have different structure
	// See: https://platform.openai.com/docs/api-reference/responses/streaming

	for _, choice := range chunk.Choices {
		// Handle content delta
		if choice.Delta.Content != nil && *choice.Delta.Content != "" {
			return map[string]any{
				"type":  "response.output_text.delta",
				"index": choice.Index,
				"delta": *choice.Delta.Content,
			}, nil
		}

		// Handle tool calls delta
		if len(choice.Delta.ToolCalls) > 0 {
			for _, tc := range choice.Delta.ToolCalls {
				return map[string]any{
					"type":  "response.function_call.delta",
					"index": choice.Index,
					"delta": map[string]any{
						"call_id":   tc.ID,
						"name":      tc.Function.Name,
						"arguments": tc.Function.Arguments,
					},
				}, nil
			}
		}

		// Handle finish reason
		if choice.FinishReason != nil {
			switch *choice.FinishReason {
			case "stop":
				return map[string]any{
					"type":  "response.output_text.done",
					"index": choice.Index,
				}, nil
			case "tool_calls":
				return map[string]any{
					"type": "response.function_call.done",
				}, nil
			}
		}
	}

	// A chunk with only usage (no choices) is handled at the handler level
	return nil, nil
}