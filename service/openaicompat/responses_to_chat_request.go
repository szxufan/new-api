package openaicompat

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/samber/lo"
)

// ResponsesRequestToChatCompletionsRequest converts an OpenAI Responses API request
// to a ChatCompletions API request format.
// This is the reverse of ChatCompletionsRequestToResponsesRequest.
func ResponsesRequestToChatCompletionsRequest(req *dto.OpenAIResponsesRequest) (*dto.GeneralOpenAIRequest, error) {
	if req == nil {
		return nil, errors.New("request is nil")
	}
	if req.Model == "" {
		return nil, errors.New("model is required")
	}

	messages := make([]dto.Message, 0)

	// 1. Convert instructions to system message
	if len(req.Instructions) > 0 {
		var instructionsStr string
		if err := common.Unmarshal(req.Instructions, &instructionsStr); err == nil && strings.TrimSpace(instructionsStr) != "" {
			messages = append(messages, dto.Message{
				Role:    "system",
				Content: instructionsStr,
			})
		}
	}

	// 2. Parse and convert input array (preserves original order)
	inputMessages, err := parseResponsesInputToMessages(req.Input)
	if err != nil {
		return nil, fmt.Errorf("failed to parse input: %w", err)
	}

	// 3. Merge system messages and append input messages
	messages = mergeSystemMessages(messages, inputMessages)

	// 4. Convert tools format
	// Responses API tools: [{"type": "function", "name": "...", "description": "...", "parameters": {...}}]
	// ChatCompletions tools: [{"type": "function", "function": {"name": "...", "description": "...", "parameters": {...}}}]
	var tools []dto.ToolCallRequest
	if len(req.Tools) > 0 {
		var responsesTools []map[string]any
		if err := common.Unmarshal(req.Tools, &responsesTools); err == nil {
			tools = convertResponsesToolsToChatTools(responsesTools)
		}
	}

	// 5. Convert tool_choice format
	// Responses: {"type": "function", "name": "..."} or string "auto"/"required"/"none"
	// ChatCompletions: {"type": "function", "function": {"name": "..."}} or string
	var toolChoice any
	if len(req.ToolChoice) > 0 {
		toolChoice = convertResponsesToolChoiceToChatToolChoice(req.ToolChoice)
	}

	// 6. Convert parallel_tool_calls
	var parallelToolCalls *bool
	if len(req.ParallelToolCalls) > 0 {
		var ptc bool
		if err := common.Unmarshal(req.ParallelToolCalls, &ptc); err == nil {
			parallelToolCalls = &ptc
		}
	}

	// 7. Convert text.format to response_format
	var responseFormat *dto.ResponseFormat
	if len(req.Text) > 0 {
		responseFormat = convertResponsesTextToResponseFormat(req.Text)
	}

	// 8. Handle reasoning.effort -> reasoning_effort
	var reasoningEffort string
	if req.Reasoning != nil && req.Reasoning.Effort != "" {
		reasoningEffort = req.Reasoning.Effort
	}

	// 9. Build the ChatCompletions request
	out := &dto.GeneralOpenAIRequest{
		Model:            req.Model,
		Messages:         messages,
		Stream:           req.Stream,
		Temperature:      req.Temperature,
		TopP:             req.TopP,
		Tools:            tools,
		ToolChoice:       toolChoice,
		ParallelTooCalls: parallelToolCalls,
		ResponseFormat:   responseFormat,
		ReasoningEffort:  reasoningEffort,
		User:             req.User,
		Store:            req.Store,
		Metadata:         req.Metadata,
	}

	// ServiceTier: convert string to json.RawMessage
	if req.ServiceTier != "" {
		out.ServiceTier = json.RawMessage(`"` + req.ServiceTier + `"`)
	}

	// max_output_tokens -> max_tokens
	if req.MaxOutputTokens != nil {
		out.MaxTokens = req.MaxOutputTokens
	}

	// TopLogProbs
	if req.TopLogProbs != nil {
		out.TopLogProbs = req.TopLogProbs
		out.LogProbs = lo.ToPtr(true)
	}

	// 工具调用ID去重：previous_response_id 上下文合并可能导致重复的 tool_call_id
	out.DeduplicateToolCallIDs()

	// 合并连续相同 role 的消息：previous_response_id 合并后可能出现连续的 user/assistant 消息
	out.MergeConsecutiveMessages()

	return out, nil
}

// mergeSystemMessages merges consecutive system messages into a single message.
// This is needed because some providers don't support multiple system messages.
func mergeSystemMessages(existing []dto.Message, new []dto.Message) []dto.Message {
	result := make([]dto.Message, 0, len(existing)+len(new))

	// Collect all system content
	var systemContents []string

	// Helper to extract string content from a message
	extractContent := func(msg dto.Message) string {
		switch c := msg.Content.(type) {
		case string:
			return c
		default:
			if b, err := common.Marshal(c); err == nil {
				return string(b)
			}
			return ""
		}
	}

	// Process existing messages
	for _, msg := range existing {
		if msg.Role == "system" {
			content := extractContent(msg)
			if content != "" {
				systemContents = append(systemContents, content)
			}
		} else {
			result = append(result, msg)
		}
	}

	// Process new messages
	for _, msg := range new {
		if msg.Role == "system" {
			content := extractContent(msg)
			if content != "" {
				systemContents = append(systemContents, content)
			}
		} else {
			result = append(result, msg)
		}
	}

	// Prepend merged system message if any
	if len(systemContents) > 0 {
		mergedContent := strings.Join(systemContents, "\n\n")
		systemMsg := dto.Message{
			Role:    "system",
			Content: mergedContent,
		}
		result = append([]dto.Message{systemMsg}, result...)
	}

	return result
}

// parseResponsesInputToMessages parses the Responses API input field and converts it to messages
// in the original input order. function_call items are merged into the preceding assistant
// message (or a new one is created), and function_call_output items become tool messages.
func parseResponsesInputToMessages(input json.RawMessage) (
	messages []dto.Message,
	err error,
) {
	if input == nil {
		return nil, nil
	}

	// Input can be a string (simple user message)
	if common.GetJsonType(input) == "string" {
		var str string
		if err := common.Unmarshal(input, &str); err != nil {
			return nil, err
		}
		return []dto.Message{{Role: "user", Content: str}}, nil
	}

	// Input can be an array of items
	if common.GetJsonType(input) != "array" {
		return nil, fmt.Errorf("input must be string or array, got %s", common.GetJsonType(input))
	}

	var items []map[string]any
	if err := common.Unmarshal(input, &items); err != nil {
		return nil, err
	}

	messages = make([]dto.Message, 0)

	for _, item := range items {
		itemType, _ := item["type"].(string)
		itemRole, _ := item["role"].(string)

		switch itemType {
		case "function_call":
			// Convert to assistant message with tool_calls
			callID, _ := item["call_id"].(string)
			name, _ := item["name"].(string)
			arguments, _ := item["arguments"].(string)

			if callID == "" {
				callID, _ = item["id"].(string) // fallback to id
			}

			if name != "" && callID != "" {
				toolCall := dto.ToolCallRequest{
					ID:   callID,
					Type: "function",
					Function: dto.FunctionRequest{
						Name:      name,
						Arguments: arguments,
					},
				}
				// Try to append to the last assistant message if it's empty (no text content)
				if len(messages) > 0 {
					lastMsg := &messages[len(messages)-1]
					if lastMsg.Role == "assistant" {
						contentStr, _ := lastMsg.Content.(string)
						if contentStr == "" {
							existingToolCalls := lastMsg.ParseToolCalls()
							existingToolCalls = append(existingToolCalls, toolCall)
							lastMsg.SetToolCalls(existingToolCalls)
							continue
						}
					}
				}
				// Otherwise create a new assistant message
				msg := dto.Message{
					Role:    "assistant",
					Content: "",
				}
				msg.SetToolCalls([]dto.ToolCallRequest{toolCall})
				messages = append(messages, msg)
			}

		case "function_call_output":
			// Convert to tool message
			callID, _ := item["call_id"].(string)

			if callID == "" {
				callID, _ = item["id"].(string)
			}

			if callID != "" {
				output := item["output"]
				var contentStr string
				switch v := output.(type) {
				case string:
					contentStr = v
				default:
					if b, err := common.Marshal(output); err == nil {
						contentStr = string(b)
					}
				}
				messages = append(messages, dto.Message{
					Role:       "tool",
					Content:    contentStr,
					ToolCallId: callID,
				})
			}

		default:
			// Regular message (user/assistant/developer)
			role := itemRole
			if role == "" {
				// Infer role from type
				switch itemType {
				case "input_text", "input_image", "input_file", "input_audio", "input_video":
					role = "user"
				case "output_text":
					role = "assistant"
				default:
					role = "user" // default
				}
			}

			// Convert "developer" role to "system" (not all providers support developer)
			if role == "developer" {
				role = "system"
			}

			// Parse content
			content := parseItemContent(item, itemType, role)
			msg := dto.Message{
				Role:    role,
				Content: content,
			}

			// Check for tool_calls in assistant message (some providers embed them)
			if role == "assistant" {
				if tcRaw, ok := item["tool_calls"]; ok {
					if tcList, ok := tcRaw.([]any); ok {
						var toolCalls []dto.ToolCallRequest
						for _, tcAny := range tcList {
							if tcMap, ok := tcAny.(map[string]any); ok {
								tcID, _ := tcMap["id"].(string)
								tcType, _ := tcMap["type"].(string)
								if tcType == "" {
									tcType = "function"
								}
								if fnMap, ok := tcMap["function"].(map[string]any); ok {
									fnName, _ := fnMap["name"].(string)
									fnArgs, _ := fnMap["arguments"].(string)
									if tcID != "" && fnName != "" {
										toolCalls = append(toolCalls, dto.ToolCallRequest{
											ID:   tcID,
											Type: tcType,
											Function: dto.FunctionRequest{
												Name:      fnName,
												Arguments: fnArgs,
											},
										})
									}
								}
							}
						}
						if len(toolCalls) > 0 {
							msg.SetToolCalls(toolCalls)
						}
					}
				}
			}

			messages = append(messages, msg)
		}
	}

	return messages, nil
}

// parseItemContent parses the content of an input item
func parseItemContent(item map[string]any, itemType string, role string) any {
	// Check if content is directly in the item
	if contentRaw, ok := item["content"]; ok {
		// Content can be string or array
		switch c := contentRaw.(type) {
		case string:
			return c
		case []any:
			// Parse as MediaContent array
			mediaContents := make([]dto.MediaContent, 0)
			for _, partAny := range c {
				if part, ok := partAny.(map[string]any); ok {
					mc := parseMapToMediaContent(part)
					mediaContents = append(mediaContents, mc)
				}
			}
			return mediaContents
		case map[string]any:
			// Single content object
			mc := parseMapToMediaContent(c)
			return []dto.MediaContent{mc}
		}
	}

	// Otherwise, construct content from type-specific fields
	switch itemType {
	case "input_text":
		text, _ := item["text"].(string)
		return text

	case "input_image":
		imageUrl := parseImageUrl(item["image_url"])
		return []dto.MediaContent{
			{Type: "image_url", ImageUrl: imageUrl},
		}

	case "input_file":
		fileData, _ := item["file_data"].(string)
		filename, _ := item["filename"].(string)
		return []dto.MediaContent{
			{Type: "file", File: map[string]any{
				"file_data": fileData,
				"filename":  filename,
			}},
		}

	case "input_audio":
		audioData, _ := item["audio_data"].(string)
		format, _ := item["format"].(string)
		return []dto.MediaContent{
			{Type: "input_audio", InputAudio: map[string]any{
				"data":   audioData,
				"format": format,
			}},
		}

	case "output_text":
		text, _ := item["text"].(string)
		return text
	}

	// Fallback: return empty string
	return ""
}

// parseMapToMediaContent converts a map to MediaContent
func parseMapToMediaContent(part map[string]any) dto.MediaContent {
	mc := dto.MediaContent{}
	contentType, _ := part["type"].(string)

	// Convert Responses API content types to ChatCompletions types
	// input_text, output_text -> text
	// input_image -> image_url
	// input_audio -> input_audio (same)
	// input_video, video_url -> video_url
	switch contentType {
	case "input_text", "output_text":
		mc.Type = "text"
	case "input_image":
		mc.Type = "image_url"
	case "input_video":
		mc.Type = "video_url"
	default:
		mc.Type = contentType
	}

	switch contentType {
	case "text", "input_text", "output_text":
		mc.Text, _ = part["text"].(string)
	case "image_url", "input_image":
		mc.ImageUrl = parseImageUrl(part["image_url"])
	case "input_audio":
		mc.InputAudio = part["input_audio"]
	case "file", "input_file":
		mc.File = part["file"]
	case "video_url", "input_video":
		mc.VideoUrl = part["video_url"]
	}

	return mc
}

// parseImageUrl handles image_url which can be string or object
func parseImageUrl(v any) any {
	switch u := v.(type) {
	case string:
		return dto.MessageImageUrl{Url: u}
	case map[string]any:
		url, _ := u["url"].(string)
		detail, _ := u["detail"].(string)
		return dto.MessageImageUrl{Url: url, Detail: detail}
	default:
		return v
	}
}

// convertResponsesToolsToChatTools converts Responses tools format to ChatCompletions format
// Only keeps tools that are supported by ChatCompletions API: function, web_search_preview, code_interpreter
func convertResponsesToolsToChatTools(responsesTools []map[string]any) []dto.ToolCallRequest {
	// ChatCompletions supported tool types
	supportedTypes := map[string]bool{
		"function":            true,
		"web_search_preview":  true,
		"code_interpreter":    true,
	}

	tools := make([]dto.ToolCallRequest, 0, len(responsesTools))
	for _, tool := range responsesTools {
		toolType, _ := tool["type"].(string)
		if toolType == "" {
			toolType = "function"
		}

		// Skip unsupported tool types (custom, namespace, tool_search, web_search, etc.)
		if !supportedTypes[toolType] {
			continue
		}

		if toolType == "function" {
			name, _ := tool["name"].(string)
			description, _ := tool["description"].(string)
			parameters := tool["parameters"]

			tools = append(tools, dto.ToolCallRequest{
				Type: toolType,
				Function: dto.FunctionRequest{
					Name:        name,
					Description: description,
					Parameters:  parameters, // any type, pass directly
				},
			})
		} else {
			// web_search_preview, code_interpreter: preserve as-is
			customRaw, _ := common.Marshal(tool)
			tools = append(tools, dto.ToolCallRequest{
				Type:   toolType,
				Custom: customRaw,
			})
		}
	}
	return tools
}

// convertResponsesToolChoiceToChatToolChoice converts Responses tool_choice to ChatCompletions format
func convertResponsesToolChoiceToChatToolChoice(toolChoiceRaw json.RawMessage) any {
	if toolChoiceRaw == nil {
		return nil
	}

	// Try string first
	var strChoice string
	if err := common.Unmarshal(toolChoiceRaw, &strChoice); err == nil {
		return strChoice // "auto", "required", "none" pass through
	}

	// Try object
	var choiceMap map[string]any
	if err := common.Unmarshal(toolChoiceRaw, &choiceMap); err == nil {
		choiceType, _ := choiceMap["type"].(string)
		if choiceType == "function" {
			// Responses: {"type": "function", "name": "..."}
			// ChatCompletions: {"type": "function", "function": {"name": "..."}}
			name, _ := choiceMap["name"].(string)
			if name != "" {
				return map[string]any{
					"type": "function",
					"function": map[string]any{
						"name": name,
					},
				}
			}
		}
		// Return as-is for unknown types
		return choiceMap
	}

	return nil
}

// convertResponsesTextToResponseFormat converts Responses text.format to ChatCompletions response_format
func convertResponsesTextToResponseFormat(textRaw json.RawMessage) *dto.ResponseFormat {
	if textRaw == nil {
		return nil
	}

	var textMap map[string]any
	if err := common.Unmarshal(textRaw, &textMap); err != nil {
		return nil
	}

	formatRaw, ok := textMap["format"]
	if !ok {
		return nil
	}

	format, ok := formatRaw.(map[string]any)
	if !ok {
		return nil
	}

	formatType, _ := format["type"].(string)
	if formatType == "" {
		return nil
	}

	responseFormat := &dto.ResponseFormat{
		Type: formatType,
	}

	// Handle json_schema
	if formatType == "json_schema" {
		// Collect all schema-related fields
		schemaMap := make(map[string]any)
		for k, v := range format {
			if k != "type" {
				schemaMap[k] = v
			}
		}
		if len(schemaMap) > 0 {
			schemaRaw, _ := common.Marshal(schemaMap)
			responseFormat.JsonSchema = schemaRaw
		}
	}

	return responseFormat
}
