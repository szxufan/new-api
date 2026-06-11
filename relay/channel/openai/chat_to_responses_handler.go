package openai

import (
	"bufio"
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
	"github.com/QuantumNous/new-api/service/openaicompat"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// OaiChatToResponsesHandler handles non-streaming ChatCompletions response
// and converts it to Responses API format.
// originalReq is the original Responses API request (used for context storage).
func OaiChatToResponsesHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response, originalReq *dto.OpenAIResponsesRequest) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}

	defer service.CloseResponseBodyGracefully(resp)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}

	var chatResp dto.OpenAITextResponse
	if err := common.Unmarshal(body, &chatResp); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}

	// Check for error in response
	if chatResp.Error != nil {
		openAIError := chatResp.GetOpenAIError()
		return nil, types.NewError(fmt.Errorf(openAIError.Message), types.ErrorCodeBadResponse, types.ErrOptionWithStatusCode(http.StatusBadRequest))
	}

	// Convert ChatCompletions response to Responses format
	responsesResp, err := openaicompat.ChatCompletionsResponseToResponsesResponse(&chatResp)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}

	// Generate response ID if not present
	if responsesResp.ID == "" {
		responsesResp.ID = helper.GetResponsesID(c)
	}

	// Override model with original client-requested model name
	// (chatResp.Model from upstream may differ due to model mapping)
	responsesResp.Model = info.OriginModelName

	// Store context for previous_response_id support (only if store=true)
	if originalReq != nil && service.ParseStoreField(originalReq.Store) {
		// Build context entry
		entry := &dto.ResponsesContextEntry{
			Model:        responsesResp.Model,
			Instructions: originalReq.Instructions,
			Input:        originalReq.Input,
			Output:       responsesResp.Output,
			Tools:        originalReq.Tools,
		}
		service.StoreResponsesContext(responsesResp.ID, entry)
		logger.LogDebug(c, "stored responses context for id=%s", responsesResp.ID)
	}

	// Marshal and send response
	respJSON, err := common.Marshal(responsesResp)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(http.StatusOK)
	_, _ = c.Writer.Write(respJSON)

	usage := &dto.Usage{}
	if responsesResp.Usage != nil {
		usage.PromptTokens = responsesResp.Usage.InputTokens
		usage.CompletionTokens = responsesResp.Usage.OutputTokens
		usage.TotalTokens = responsesResp.Usage.TotalTokens
		usage.InputTokens = responsesResp.Usage.InputTokens
		usage.OutputTokens = responsesResp.Usage.OutputTokens
	}

	return usage, nil
}

// OaiChatToResponsesStreamHandler handles streaming ChatCompletions response
// and converts it to Responses API stream format.
// Emits all required lifecycle events per the OpenAI Responses API spec:
//   response.created → response.in_progress → response.output_item.added →
//   response.content_part.added → response.output_text.delta* →
//   response.output_text.done → response.content_part.done →
//   response.output_item.done → response.completed
func OaiChatToResponsesStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response, originalReq *dto.OpenAIResponsesRequest) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}

	defer service.CloseResponseBodyGracefully(resp)

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.WriteHeaderNow()

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return nil, types.NewOpenAIError(fmt.Errorf("streaming not supported"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}

	// Generate IDs
	respID := helper.GetResponsesID(c)
	msgID := fmt.Sprintf("msg_%d", time.Now().UnixNano())
	model := info.OriginModelName
	createdAt := time.Now().Unix()

	// State tracking
	seqNum := 0
	hasSentTextItemEvents := false
	nextOutputIndex := 0
	var textBuilder strings.Builder
	var usage *dto.Usage
	var collectedOutput []dto.ResponsesOutput

	// Tool call state tracking
	var toolCallBuilder strings.Builder
	var toolCallName string
	var toolCallID string
	var hasSentToolCallItem bool
	toolCallMsgID := fmt.Sprintf("msg_%d_1", time.Now().UnixNano())

	// Helper: send an SSE event with sequence_number
	sendEvent := func(eventType string, data map[string]any) {
		if data == nil {
			data = make(map[string]any)
		}
		data["sequence_number"] = seqNum
		seqNum++
		eventJSON, err := json.Marshal(data)
		if err != nil {
			logger.LogDebug(c, "failed to marshal event: %v", err)
			return
		}
		_, _ = c.Writer.Write([]byte("event: " + eventType + "\ndata: " + string(eventJSON) + "\n\n"))
		flusher.Flush()
	}

	// Helper: build the base response object for lifecycle events
	buildBaseResponse := func(status string, usageData map[string]any) map[string]any {
		respObj := map[string]any{
			"id":         respID,
			"object":     "response",
			"status":     status,
			"model":      model,
			"created_at": createdAt,
			"output":     []any{},
		}
		if usageData != nil {
			respObj["usage"] = usageData
		}
		return respObj
	}

	// --- Send initial lifecycle events ---
	initResponse := buildBaseResponse("in_progress", nil)
	sendEvent("response.created", map[string]any{
		"type":     "response.created",
		"response": initResponse,
	})
	sendEvent("response.in_progress", map[string]any{
		"type":     "response.in_progress",
		"response": initResponse,
	})

	// --- Process stream chunks ---
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk dto.ChatCompletionsStreamResponse
		if err := common.Unmarshal([]byte(data), &chunk); err != nil {
			logger.LogDebug(c, "failed to unmarshal stream chunk: %v", err)
			continue
		}

		// Capture usage from final chunk (must be done before continue on nil streamEvent)
		if chunk.Usage != nil && chunk.Usage.TotalTokens > 0 {
			usage = &dto.Usage{
				PromptTokens:     chunk.Usage.PromptTokens,
				CompletionTokens: chunk.Usage.CompletionTokens,
				TotalTokens:      chunk.Usage.TotalTokens,
			}
		}

		for _, choice := range chunk.Choices {
			// --- Handle content delta ---
			if choice.Delta.Content != nil && *choice.Delta.Content != "" {
				textBuilder.WriteString(*choice.Delta.Content)

				// Send output_item.added and content_part.added on first content
				if !hasSentTextItemEvents {
					hasSentTextItemEvents = true
					textOutputIndex := nextOutputIndex
					nextOutputIndex++

					sendEvent("response.output_item.added", map[string]any{
						"type": "response.output_item.added",
						"item": map[string]any{
							"id":     msgID,
							"type":   "message",
							"role":   "assistant",
							"phase":  "final_answer",
							"status": "in_progress",
							"content": []any{},
						},
						"output_index": textOutputIndex,
					})

					sendEvent("response.content_part.added", map[string]any{
						"type":          "response.content_part.added",
						"part": map[string]any{
							"type":        "output_text",
							"text":        "",
							"annotations": []any{},
							"logprobs":    []any{},
						},
						"item_id":       msgID,
						"output_index":  textOutputIndex,
						"content_index": 0,
					})
				}

				// Send delta event
				sendEvent("response.output_text.delta", map[string]any{
					"type":          "response.output_text.delta",
					"delta":         *choice.Delta.Content,
					"item_id":       msgID,
					"output_index":  0,
					"content_index": 0,
				})
			}

			// --- Handle tool calls delta ---
			if len(choice.Delta.ToolCalls) > 0 {
				for _, tc := range choice.Delta.ToolCalls {
					// Capture name and id from first chunk of this tool call
					if tc.ID != "" {
						toolCallID = tc.ID
					}
					if tc.Function.Name != "" {
						toolCallName = tc.Function.Name
					}

					// Send output_item.added only once for this function call
					if !hasSentToolCallItem {
						hasSentToolCallItem = true
						tcOutputIndex := nextOutputIndex
						nextOutputIndex++

						sendEvent("response.output_item.added", map[string]any{
							"type": "response.output_item.added",
							"item": map[string]any{
								"id":        toolCallMsgID,
								"type":      "function_call",
								"call_id":   toolCallID,
								"name":      toolCallName,
								"arguments": "",
								"status":    "in_progress",
							},
							"output_index": tcOutputIndex,
						})
					}

					// Accumulate arguments
					if tc.Function.Arguments != "" {
						toolCallBuilder.WriteString(tc.Function.Arguments)
					}

					// Send arguments delta
					sendEvent("response.function_call_arguments.delta", map[string]any{
						"type":         "response.function_call_arguments.delta",
						"delta":        tc.Function.Arguments,
						"item_id":      toolCallMsgID,
						"output_index": 0,
					})
				}
			}

			// --- Handle finish reason ---
			if choice.FinishReason != nil {
				fullText := textBuilder.String()

				// Only send output_text.done if there was actual text content
				if fullText != "" {
					sendEvent("response.output_text.done", map[string]any{
						"type":          "response.output_text.done",
						"text":          fullText,
						"item_id":       msgID,
						"output_index":  0,
						"content_index": 0,
						"logprobs":      []any{},
					})
				}

				// Send content_part.done and output_item.done for text
				if hasSentTextItemEvents {
					sendEvent("response.content_part.done", map[string]any{
						"type": "response.content_part.done",
						"part": map[string]any{
							"type":        "output_text",
							"text":        fullText,
							"annotations": []any{},
							"logprobs":    []any{},
						},
						"item_id":       msgID,
						"output_index":  0,
						"content_index": 0,
					})

					// Build output content array
					contentItems := []any{}
					if fullText != "" {
						contentItems = append(contentItems, map[string]any{
							"type":        "output_text",
							"text":        fullText,
							"annotations": []any{},
						})
					}

					sendEvent("response.output_item.done", map[string]any{
						"type": "response.output_item.done",
						"item": map[string]any{
							"id":      msgID,
							"type":    "message",
							"role":    "assistant",
							"phase":   "final_answer",
							"status":  "completed",
							"content": contentItems,
						},
						"output_index": 0,
					})

					// Collect for response.completed output
					msgOutput := dto.ResponsesOutput{
						Type:   "message",
						ID:     msgID,
						Status: "completed",
						Role:   "assistant",
					}
					if fullText != "" {
						msgOutput.Content = []dto.ResponsesOutputContent{
							{Type: "output_text", Text: fullText},
						}
					}
					collectedOutput = append(collectedOutput, msgOutput)
				}

				// Send function_call_arguments.done and output_item.done for tool calls
				if hasSentToolCallItem {
					fullArgs := toolCallBuilder.String()

					sendEvent("response.function_call_arguments.done", map[string]any{
						"type":         "response.function_call_arguments.done",
						"name":         toolCallName,
						"arguments":    fullArgs,
						"item_id":      toolCallMsgID,
						"output_index": 0,
					})

					sendEvent("response.output_item.done", map[string]any{
						"type": "response.output_item.done",
						"item": map[string]any{
							"id":        toolCallMsgID,
							"type":      "function_call",
							"call_id":   toolCallID,
							"name":      toolCallName,
							"arguments": fullArgs,
							"status":    "completed",
						},
						"output_index": 0,
					})

					// Collect for response.completed output
					fcOutput := dto.ResponsesOutput{
						Type: "function_call",
						ID:   toolCallMsgID,
						Name: toolCallName,
					}
					collectedOutput = append(collectedOutput, fcOutput)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		logger.LogError(c.Request.Context(), "stream scanner error: "+err.Error())
	}

	// --- Send response.completed ---
	usageMap := map[string]any{
		"input_tokens":  0,
		"output_tokens": 0,
		"total_tokens":  0,
	}
	if usage != nil {
		usageMap = map[string]any{
			"input_tokens":  usage.PromptTokens,
			"output_tokens": usage.CompletionTokens,
			"total_tokens":  usage.TotalTokens,
		}
	}

	// Build output array from collected output for the completed event
	var completedOutput []any
	if len(collectedOutput) > 0 {
		outputJSON, _ := json.Marshal(collectedOutput)
		json.Unmarshal(outputJSON, &completedOutput)
	}

	completedResponse := buildBaseResponse("completed", usageMap)
	completedResponse["output"] = completedOutput
	sendEvent("response.completed", map[string]any{
		"type":     "response.completed",
		"response": completedResponse,
	})

	// Store context for previous_response_id support (only if store=true)
	if originalReq != nil && service.ParseStoreField(originalReq.Store) && len(collectedOutput) > 0 {
		entry := &dto.ResponsesContextEntry{
			Model:        model,
			Instructions: originalReq.Instructions,
			Input:        originalReq.Input,
			Output:       collectedOutput,
			Tools:        originalReq.Tools,
		}
		service.StoreResponsesContext(respID, entry)
		logger.LogDebug(c, "stored responses context for id=%s", respID)
	}

	if usage == nil {
		usage = &dto.Usage{}
	}

	return usage, nil
}
