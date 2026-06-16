package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

func TestStreamResponseOpenAI2Claude_ToolCallIDDedup(t *testing.T) {
	// 测试 OpenAI 流式 -> Claude 转换中工具调用ID去重
	info := &relaycommon.RelayInfo{
		ClaudeConvertInfo: &relaycommon.ClaudeConvertInfo{
			LastMessagesType: relaycommon.LastMessageTypeNone,
		},
	}
	info.SendResponseCount = 1

	// 首个chunk：包含一个 tool_call
	firstChunk := &dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl-123",
		Object:  "chat.completion.chunk",
		Model:   "gpt-4",
		Created: 1234567890,
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					Role: "assistant",
					ToolCalls: []dto.ToolCallResponse{
						{
							ID:   "call_001",
							Type: "function",
							Function: dto.FunctionResponse{
								Name:      "get_weather",
								Arguments: "",
							},
						},
					},
				},
			},
		},
	}

	responses := StreamResponseOpenAI2Claude(firstChunk, info)
	require.NotEmpty(t, responses)
	// 首个chunk后 SeenToolIDs 应记录 call_001
	require.NotNil(t, info.ClaudeConvertInfo.SeenToolIDs)
	require.True(t, info.ClaudeConvertInfo.SeenToolIDs["call_001"])

	// 后续chunk：包含相同ID的 tool_call（重复）
	info.SendResponseCount = 2
	secondChunk := &dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl-123",
		Object:  "chat.completion.chunk",
		Model:   "gpt-4",
		Created: 1234567890,
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					ToolCalls: []dto.ToolCallResponse{
						{
							ID:   "call_001", // 重复ID
							Type: "function",
							Function: dto.FunctionResponse{
								Name:      "get_weather",
								Arguments: `{"city":"Shanghai"}`,
							},
						},
						{
							ID:   "call_002", // 新ID
							Type: "function",
							Function: dto.FunctionResponse{
								Name:      "get_time",
								Arguments: "",
							},
						},
					},
				},
			},
		},
	}

	responses = StreamResponseOpenAI2Claude(secondChunk, info)
	// call_001 重复应被跳过，只有 call_002 生成 content_block_start
	hasCall002 := false
	hasDuplicateCall001 := false
	for _, resp := range responses {
		if resp.Type == "content_block_start" && resp.ContentBlock != nil && resp.ContentBlock.Type == "tool_use" {
			if resp.ContentBlock.Id == "call_002" {
				hasCall002 = true
			}
			if resp.ContentBlock.Id == "call_001" {
				hasDuplicateCall001 = true
			}
		}
	}
	require.True(t, hasCall002, "call_002 should be present in content_block_start")
	require.False(t, hasDuplicateCall001, "duplicate call_001 should be skipped in content_block_start")
}

func TestStreamResponseOpenAI2Claude_FirstChunkToolCallRecordsID(t *testing.T) {
	// 测试首块chunk中的tool_call ID被正确记录到SeenToolIDs
	info := &relaycommon.RelayInfo{
		ClaudeConvertInfo: &relaycommon.ClaudeConvertInfo{
			LastMessagesType: relaycommon.LastMessageTypeNone,
		},
	}
	info.SendResponseCount = 1

	firstChunk := &dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl-456",
		Object:  "chat.completion.chunk",
		Model:   "gpt-4",
		Created: 1234567890,
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					Role: "assistant",
					ToolCalls: []dto.ToolCallResponse{
						{
							ID:   "call_abc",
							Type: "function",
							Function: dto.FunctionResponse{
								Name: "search",
							},
						},
					},
				},
			},
		},
	}

	StreamResponseOpenAI2Claude(firstChunk, info)
	require.NotNil(t, info.ClaudeConvertInfo.SeenToolIDs)
	require.True(t, info.ClaudeConvertInfo.SeenToolIDs["call_abc"])

	// 空ID的工具调用不应被记录
	info2 := &relaycommon.RelayInfo{
		ClaudeConvertInfo: &relaycommon.ClaudeConvertInfo{
			LastMessagesType: relaycommon.LastMessageTypeNone,
		},
	}
	info2.SendResponseCount = 1
	emptyIDChunk := &dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl-789",
		Object:  "chat.completion.chunk",
		Model:   "gpt-4",
		Created: 1234567890,
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					Role: "assistant",
					ToolCalls: []dto.ToolCallResponse{
						{
							ID:   "", // 空ID
							Type: "function",
							Function: dto.FunctionResponse{
								Name: "search",
							},
						},
					},
				},
			},
		},
	}

	StreamResponseOpenAI2Claude(emptyIDChunk, info2)
	// 空ID不应导致 panic，SeenToolIDs 应为 nil 或不包含空字符串key
	if info2.ClaudeConvertInfo.SeenToolIDs != nil {
		_, hasEmptyKey := info2.ClaudeConvertInfo.SeenToolIDs[""]
		require.False(t, hasEmptyKey, "empty tool ID should not be recorded in SeenToolIDs")
	}
}

// 辅助函数：创建带有 tool_calls 的 OpenAI 流式响应
func newToolCallChunk(id, name, arguments string) *dto.ChatCompletionsStreamResponse {
	return &dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl-test",
		Object:  "chat.completion.chunk",
		Model:   "gpt-4",
		Created: common.GetTimestamp(),
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					Role: "assistant",
					ToolCalls: []dto.ToolCallResponse{
						{
							ID:   id,
							Type: "function",
							Function: dto.FunctionResponse{
								Name:      name,
								Arguments: arguments,
							},
						},
					},
				},
			},
		},
	}
}
