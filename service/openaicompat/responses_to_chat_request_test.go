package openaicompat

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/assert"
)

func TestResponsesRequestToChatCompletionsRequest_Basic(t *testing.T) {
	// Test basic conversion with simple input
	inputJSON, _ := json.Marshal([]map[string]any{
		{"role": "user", "content": "Hello"},
	})

	req := &dto.OpenAIResponsesRequest{
		Model: "gpt-4o",
		Input: inputJSON,
	}

	result, err := ResponsesRequestToChatCompletionsRequest(req)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "gpt-4o", result.Model)
	assert.Len(t, result.Messages, 1)
	assert.Equal(t, "user", result.Messages[0].Role)
	assert.Equal(t, "Hello", result.Messages[0].Content)
}

func TestResponsesRequestToChatCompletionsRequest_WithInstructions(t *testing.T) {
	// Test conversion with instructions (should become system message)
	instructionsJSON, _ := json.Marshal("You are a helpful assistant.")
	inputJSON, _ := json.Marshal([]map[string]any{
		{"role": "user", "content": "Hello"},
	})

	req := &dto.OpenAIResponsesRequest{
		Model:        "gpt-4o",
		Input:        inputJSON,
		Instructions: instructionsJSON,
	}

	result, err := ResponsesRequestToChatCompletionsRequest(req)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Messages, 2)
	assert.Equal(t, "system", result.Messages[0].Role)
	assert.Equal(t, "You are a helpful assistant.", result.Messages[0].Content)
	assert.Equal(t, "user", result.Messages[1].Role)
}

func TestResponsesRequestToChatCompletionsRequest_WithFunctionCall(t *testing.T) {
	// Test conversion with function_call item
	inputJSON, _ := json.Marshal([]map[string]any{
		{"role": "user", "content": "What's the weather?"},
		{"type": "function_call", "call_id": "call_123", "name": "get_weather", "arguments": "{\"location\": \"Beijing\"}"},
	})

	req := &dto.OpenAIResponsesRequest{
		Model: "gpt-4o",
		Input: inputJSON,
	}

	result, err := ResponsesRequestToChatCompletionsRequest(req)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Messages, 2)
	assert.Equal(t, "user", result.Messages[0].Role)
	assert.Equal(t, "assistant", result.Messages[1].Role)

	// Check tool_calls in assistant message
	toolCalls := result.Messages[1].ParseToolCalls()
	assert.Len(t, toolCalls, 1)
	assert.Equal(t, "call_123", toolCalls[0].ID)
	assert.Equal(t, "get_weather", toolCalls[0].Function.Name)
	assert.Equal(t, "{\"location\": \"Beijing\"}", toolCalls[0].Function.Arguments)
}

func TestResponsesRequestToChatCompletionsRequest_WithFunctionCallOutput(t *testing.T) {
	// Test conversion with function_call_output item
	inputJSON, _ := json.Marshal([]map[string]any{
		{"role": "user", "content": "What's the weather?"},
		{"type": "function_call", "call_id": "call_123", "name": "get_weather", "arguments": "{\"location\": \"Beijing\"}"},
		{"type": "function_call_output", "call_id": "call_123", "output": "Sunny, 25°C"},
	})

	req := &dto.OpenAIResponsesRequest{
		Model: "gpt-4o",
		Input: inputJSON,
	}

	result, err := ResponsesRequestToChatCompletionsRequest(req)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Messages, 3)
	assert.Equal(t, "user", result.Messages[0].Role)
	assert.Equal(t, "assistant", result.Messages[1].Role)
	assert.Equal(t, "tool", result.Messages[2].Role)
	assert.Equal(t, "call_123", result.Messages[2].ToolCallId)
	assert.Equal(t, "Sunny, 25°C", result.Messages[2].Content)
}

func TestResponsesRequestToChatCompletionsRequest_WithTools(t *testing.T) {
	// Test conversion with tools
	toolsJSON, _ := json.Marshal([]map[string]any{
		{
			"type":        "function",
			"name":        "get_weather",
			"description": "Get weather info",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"location": map[string]any{"type": "string"},
				},
			},
		},
	})

	inputJSON, _ := json.Marshal([]map[string]any{
		{"role": "user", "content": "Hello"},
	})

	req := &dto.OpenAIResponsesRequest{
		Model: "gpt-4o",
		Input: inputJSON,
		Tools: toolsJSON,
	}

	result, err := ResponsesRequestToChatCompletionsRequest(req)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Tools, 1)
	assert.Equal(t, "function", result.Tools[0].Type)
	assert.Equal(t, "get_weather", result.Tools[0].Function.Name)
	assert.Equal(t, "Get weather info", result.Tools[0].Function.Description)
}

func TestResponsesRequestToChatCompletionsRequest_WithToolChoice(t *testing.T) {
	// Test conversion with tool_choice
	toolChoiceJSON, _ := json.Marshal(map[string]any{
		"type": "function",
		"name": "get_weather",
	})

	inputJSON, _ := json.Marshal([]map[string]any{
		{"role": "user", "content": "Hello"},
	})

	req := &dto.OpenAIResponsesRequest{
		Model:      "gpt-4o",
		Input:      inputJSON,
		ToolChoice: toolChoiceJSON,
	}

	result, err := ResponsesRequestToChatCompletionsRequest(req)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.ToolChoice)

	// Check tool_choice format conversion
	choiceMap, ok := result.ToolChoice.(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "function", choiceMap["type"])
	fnMap, ok := choiceMap["function"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "get_weather", fnMap["name"])
}

func TestResponsesRequestToChatCompletionsRequest_WithTextFormat(t *testing.T) {
	// Test conversion with text.format -> response_format
	textJSON, _ := json.Marshal(map[string]any{
		"format": map[string]any{
			"type": "json_object",
		},
	})

	inputJSON, _ := json.Marshal([]map[string]any{
		{"role": "user", "content": "Hello"},
	})

	req := &dto.OpenAIResponsesRequest{
		Model: "gpt-4o",
		Input: inputJSON,
		Text:  textJSON,
	}

	result, err := ResponsesRequestToChatCompletionsRequest(req)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.ResponseFormat)
	assert.Equal(t, "json_object", result.ResponseFormat.Type)
}

func TestResponsesRequestToChatCompletionsRequest_WithReasoning(t *testing.T) {
	// Test conversion with reasoning.effort
	inputJSON, _ := json.Marshal([]map[string]any{
		{"role": "user", "content": "Hello"},
	})

	req := &dto.OpenAIResponsesRequest{
		Model: "o1-mini",
		Input: inputJSON,
		Reasoning: &dto.Reasoning{
			Effort: "high",
		},
	}

	result, err := ResponsesRequestToChatCompletionsRequest(req)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "high", result.ReasoningEffort)
}

func TestResponsesRequestToChatCompletionsRequest_WithMaxOutputTokens(t *testing.T) {
	// Test conversion with max_output_tokens -> max_tokens
	inputJSON, _ := json.Marshal([]map[string]any{
		{"role": "user", "content": "Hello"},
	})

	maxTokens := uint(1000)
	req := &dto.OpenAIResponsesRequest{
		Model:          "gpt-4o",
		Input:          inputJSON,
		MaxOutputTokens: &maxTokens,
	}

	result, err := ResponsesRequestToChatCompletionsRequest(req)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.MaxTokens)
	assert.Equal(t, uint(1000), *result.MaxTokens)
}

func TestResponsesRequestToChatCompletionsRequest_StringInput(t *testing.T) {
	// Test with string input (simple user message)
	inputJSON, _ := json.Marshal("Hello, how are you?")

	req := &dto.OpenAIResponsesRequest{
		Model: "gpt-4o",
		Input: inputJSON,
	}

	result, err := ResponsesRequestToChatCompletionsRequest(req)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Messages, 1)
	assert.Equal(t, "user", result.Messages[0].Role)
	assert.Equal(t, "Hello, how are you?", result.Messages[0].Content)
}

func TestResponsesRequestToChatCompletionsRequest_NilRequest(t *testing.T) {
	result, err := ResponsesRequestToChatCompletionsRequest(nil)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "request is nil")
}

func TestResponsesRequestToChatCompletionsRequest_EmptyModel(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model: "",
	}

	result, err := ResponsesRequestToChatCompletionsRequest(req)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "model is required")
}

func TestResponsesRequestToChatCompletionsRequest_MultiModalInput(t *testing.T) {
	// Test with multi-modal input (text + image)
	inputJSON, _ := json.Marshal([]map[string]any{
		{
			"role": "user",
			"content": []map[string]any{
				{"type": "input_text", "text": "What's in this image?"},
				{"type": "input_image", "image_url": "https://example.com/image.png"},
			},
		},
	})

	req := &dto.OpenAIResponsesRequest{
		Model: "gpt-4o",
		Input: inputJSON,
	}

	result, err := ResponsesRequestToChatCompletionsRequest(req)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Messages, 1)
	assert.Equal(t, "user", result.Messages[0].Role)

	// Check content is array
	contentArray, ok := result.Messages[0].Content.([]dto.MediaContent)
	assert.True(t, ok)
	assert.Len(t, contentArray, 2)
}

// Test round-trip conversion: ChatCompletions -> Responses -> ChatCompletions
func TestResponsesRequestToChatCompletionsRequest_RoundTrip(t *testing.T) {
	// Start with a ChatCompletions request
	originalReq := &dto.GeneralOpenAIRequest{
		Model: "gpt-4o",
		Messages: []dto.Message{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "Hello"},
		},
		Temperature: common.GetPointer(0.7),
		TopP:        common.GetPointer(0.9),
	}

	// Convert to Responses
	responsesReq, err := ChatCompletionsRequestToResponsesRequest(originalReq)
	assert.NoError(t, err)
	assert.NotNil(t, responsesReq)

	// Convert back to ChatCompletions
	chatReq, err := ResponsesRequestToChatCompletionsRequest(responsesReq)
	assert.NoError(t, err)
	assert.NotNil(t, chatReq)

	// Verify key fields match
	assert.Equal(t, originalReq.Model, chatReq.Model)
	assert.Equal(t, *originalReq.Temperature, *chatReq.Temperature)
	assert.Equal(t, *originalReq.TopP, *chatReq.TopP)
	// Note: messages structure may differ slightly due to instructions handling
}
