package openaicompat

import (
	"encoding/json"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

// RebuildResponsesInput 根据 previous_response_id 重建 input
// 将之前响应的 Output 转换为 input 项，并与当前 input 合并
// 仅在回退场景（上游不支持 /v1/responses）下使用
func RebuildResponsesInput(
	previousEntry *dto.ResponsesContextEntry,
	currentInput json.RawMessage,
) (json.RawMessage, error) {
	// 将之前的 Output 转换为 input 项
	previousItems := OutputsToInputItems(previousEntry.Output)

	// 解析当前 input
	var currentItems []map[string]any
	if len(currentInput) > 0 {
		// 先尝试解析为数组
		if err := common.Unmarshal(currentInput, &currentItems); err != nil {
			// 可能是字符串格式（简单用户消息）
			var strInput string
			if err := common.Unmarshal(currentInput, &strInput); err == nil {
				currentItems = []map[string]any{
					{"type": "message", "role": "user", "content": strInput},
				}
			}
		}
	}

	// 合并: 之前的内容 + 当前内容
	mergedItems := append(previousItems, currentItems...)

	return common.Marshal(mergedItems)
}

// OutputsToInputItems 将 Output 转换为 input 项
// 用于将缓存的响应内容转换为新请求的 input
func OutputsToInputItems(outputs []dto.ResponsesOutput) []map[string]any {
	items := make([]map[string]any, 0, len(outputs))

	for _, out := range outputs {
		switch out.Type {
		case "message":
			// 消息类型：保留 role 和 content
			item := map[string]any{
				"type": "message",
				"role": out.Role,
			}
			if len(out.Content) > 0 {
				// 将 Content 转换为合适格式
				// Responses API 的 type 如 "input_text", "output_text" 需要转换为 ChatCompletions 的 "text"
				contentItems := make([]map[string]any, 0, len(out.Content))
				for _, c := range out.Content {
					// 将 Responses API 的 content type 转换为通用 type
					contentType := c.Type
					if contentType == "input_text" || contentType == "output_text" {
						contentType = "text"
					}
					contentItem := map[string]any{
						"type": contentType,
					}
					if c.Text != "" {
						contentItem["text"] = c.Text
					}
					contentItems = append(contentItems, contentItem)
				}
				item["content"] = contentItems
			}
			items = append(items, item)

		case "function_call":
			// 函数调用类型：保留 call_id, name, arguments
			items = append(items, map[string]any{
				"type":      "function_call",
				"call_id":   out.CallId,
				"name":      out.Name,
				"arguments": string(out.Arguments),
			})

		case "function_call_output":
			// 函数调用输出类型：保留 call_id 和 output
			outputStr := ""
			if len(out.Content) > 0 {
				for _, c := range out.Content {
					if c.Text != "" {
						outputStr = c.Text
						break
					}
				}
			}
			items = append(items, map[string]any{
				"type":    "function_call_output",
				"call_id": out.CallId,
				"output":  outputStr,
			})
		}
	}

	return items
}

// OutputsToMessages 将 Output 转换为 ChatCompletions 的 Messages 格式
// 用于回退场景中，将之前的响应内容合并到 ChatCompletions 请求的 messages 中
func OutputsToMessages(outputs []dto.ResponsesOutput) []dto.Message {
	messages := make([]dto.Message, 0, len(outputs))

	for _, out := range outputs {
		switch out.Type {
		case "message":
			// 消息类型：转换为 dto.Message
			msg := dto.Message{
				Role: out.Role,
			}
			// 提取文本内容
			if len(out.Content) > 0 {
				textContent := ""
				for _, c := range out.Content {
					if c.Type == "output_text" || c.Type == "input_text" || c.Type == "text" {
						textContent += c.Text
					}
				}
				msg.Content = textContent
			}
			messages = append(messages, msg)

		case "function_call":
			// 函数调用：转换为 assistant message with tool_calls
			msg := dto.Message{
				Role:    "assistant",
				Content: "",
			}
			toolCall := dto.ToolCallRequest{
				ID:   out.CallId,
				Type: "function",
				Function: dto.FunctionRequest{
					Name:      out.Name,
					Arguments: string(out.Arguments),
				},
			}
			msg.SetToolCalls([]dto.ToolCallRequest{toolCall})
			messages = append(messages, msg)

		case "function_call_output":
			// 函数调用输出：转换为 tool message
			outputStr := ""
			if len(out.Content) > 0 {
				for _, c := range out.Content {
					if c.Text != "" {
						outputStr = c.Text
						break
					}
				}
			}
			messages = append(messages, dto.Message{
				Role:       "tool",
				Content:    outputStr,
				ToolCallId: out.CallId,
			})
		}
	}

	return messages
}