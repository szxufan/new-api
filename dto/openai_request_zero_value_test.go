package dto

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestGeneralOpenAIRequestPreserveExplicitZeroValues(t *testing.T) {
	raw := []byte(`{
		"model":"gpt-4.1",
		"stream":false,
		"max_tokens":0,
		"max_completion_tokens":0,
		"top_p":0,
		"top_k":0,
		"n":0,
		"frequency_penalty":0,
		"presence_penalty":0,
		"seed":0,
		"logprobs":false,
		"top_logprobs":0,
		"dimensions":0,
		"return_images":false,
		"return_related_questions":false
	}`)

	var req GeneralOpenAIRequest
	err := common.Unmarshal(raw, &req)
	require.NoError(t, err)

	encoded, err := common.Marshal(req)
	require.NoError(t, err)

	require.True(t, gjson.GetBytes(encoded, "stream").Exists())
	require.True(t, gjson.GetBytes(encoded, "max_tokens").Exists())
	require.True(t, gjson.GetBytes(encoded, "max_completion_tokens").Exists())
	require.True(t, gjson.GetBytes(encoded, "top_p").Exists())
	require.True(t, gjson.GetBytes(encoded, "top_k").Exists())
	require.True(t, gjson.GetBytes(encoded, "n").Exists())
	require.True(t, gjson.GetBytes(encoded, "frequency_penalty").Exists())
	require.True(t, gjson.GetBytes(encoded, "presence_penalty").Exists())
	require.True(t, gjson.GetBytes(encoded, "seed").Exists())
	require.True(t, gjson.GetBytes(encoded, "logprobs").Exists())
	require.True(t, gjson.GetBytes(encoded, "top_logprobs").Exists())
	require.True(t, gjson.GetBytes(encoded, "dimensions").Exists())
	require.True(t, gjson.GetBytes(encoded, "return_images").Exists())
	require.True(t, gjson.GetBytes(encoded, "return_related_questions").Exists())
}

func TestOpenAIResponsesRequestPreserveExplicitZeroValues(t *testing.T) {
	raw := []byte(`{
		"model":"gpt-4.1",
		"max_output_tokens":0,
		"max_tool_calls":0,
		"stream":false,
		"top_p":0
	}`)

	var req OpenAIResponsesRequest
	err := common.Unmarshal(raw, &req)
	require.NoError(t, err)

	encoded, err := common.Marshal(req)
	require.NoError(t, err)

	require.True(t, gjson.GetBytes(encoded, "max_output_tokens").Exists())
	require.True(t, gjson.GetBytes(encoded, "max_tool_calls").Exists())
	require.True(t, gjson.GetBytes(encoded, "stream").Exists())
	require.True(t, gjson.GetBytes(encoded, "top_p").Exists())
}

func TestGeneralOpenAIRequestGetSystemRoleName(t *testing.T) {
	tests := []struct {
		name  string
		model string
		want  string
	}{
		{name: "o1 uses developer", model: "o1", want: "developer"},
		{name: "o3 family uses developer", model: "o3-mini-high", want: "developer"},
		{name: "o4 family uses developer", model: "o4-mini", want: "developer"},
		{name: "o1 mini stays system", model: "o1-mini", want: "system"},
		{name: "o1 preview stays system", model: "o1-preview", want: "system"},
		{name: "gpt 5 uses developer", model: "gpt-5", want: "developer"},
		{name: "omni is not o series", model: "omni-moderation-latest", want: "system"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := GeneralOpenAIRequest{Model: tt.model}

			require.Equal(t, tt.want, req.GetSystemRoleName())
		})
	}
}

func TestDeduplicateToolCallIDs(t *testing.T) {
	// 测试重复 tool_call_id 去重
	req := &GeneralOpenAIRequest{
		Messages: []Message{
			{Role: "user", Content: "What's the weather?"},
			{Role: "assistant", Content: "Let me check."},
			{Role: "tool", ToolCallId: "shell_command:12", Content: "sunny"},
			{Role: "tool", ToolCallId: "shell_command:12", Content: "rainy"}, // 重复
			{Role: "tool", ToolCallId: "shell_command:13", Content: "cloudy"},
			{Role: "assistant", Content: "The weather is sunny."},
		},
	}

	req.DeduplicateToolCallIDs()

	require.Len(t, req.Messages, 5)
	// 第一个 shell_command:12 保留
	require.Equal(t, "shell_command:12", req.Messages[2].ToolCallId)
	require.Equal(t, "sunny", req.Messages[2].Content)
	// shell_command:13 保留
	require.Equal(t, "shell_command:13", req.Messages[3].ToolCallId)
}

func TestDeduplicateToolCallIDs_EmptyMessages(t *testing.T) {
	req := &GeneralOpenAIRequest{Messages: nil}
	req.DeduplicateToolCallIDs()
	require.Nil(t, req.Messages)
}

func TestDeduplicateToolCallIDs_EmptyToolCallId(t *testing.T) {
	// 空 tool_call_id 不做去重
	req := &GeneralOpenAIRequest{
		Messages: []Message{
			{Role: "tool", ToolCallId: "", Content: "a"},
			{Role: "tool", ToolCallId: "", Content: "b"},
		},
	}

	req.DeduplicateToolCallIDs()

	// 空 tool_call_id 的消息都保留
	require.Len(t, req.Messages, 2)
}

func TestDeduplicateToolCallIDs_AssistantToolCallsDedup(t *testing.T) {
	// 测试 assistant 消息中 tool_calls 重复 ID 去重
	toolCallsData, _ := json.Marshal([]ToolCallRequest{
		{ID: "shell_command:6", Type: "function", Function: FunctionRequest{Name: "run_shell", Arguments: `{"cmd":"ls"}`}},
		{ID: "shell_command:6", Type: "function", Function: FunctionRequest{Name: "run_shell", Arguments: `{"cmd":"pwd"}`}}, // 重复
		{ID: "shell_command:7", Type: "function", Function: FunctionRequest{Name: "run_shell", Arguments: `{"cmd":"whoami"}`}},
	})
	req := &GeneralOpenAIRequest{
		Messages: []Message{
			{Role: "user", Content: "Run commands"},
			{Role: "assistant", Content: "", ToolCalls: toolCallsData},
		},
	}

	req.DeduplicateToolCallIDs()

	require.Len(t, req.Messages, 2)
	assistantMsg := req.Messages[1]
	dedupedCalls := assistantMsg.ParseToolCalls()
	require.Len(t, dedupedCalls, 2)
	require.Equal(t, "shell_command:6", dedupedCalls[0].ID)
	require.Equal(t, "shell_command:7", dedupedCalls[1].ID)
}

func TestDeduplicateToolCallIDs_CrossRoleDedup(t *testing.T) {
	// 测试 assistant.tool_calls.id 和 tool.tool_call_id 交叉去重
	// 模拟 previous_response_id 合并场景：assistant 中有 tool_call，tool 消息引用了同一个 ID
	toolCallsData, _ := json.Marshal([]ToolCallRequest{
		{ID: "shell_command:12", Type: "function", Function: FunctionRequest{Name: "run_shell", Arguments: `{"cmd":"ls"}`}},
	})
	req := &GeneralOpenAIRequest{
		Messages: []Message{
			{Role: "user", Content: "Run command"},
			{Role: "assistant", Content: "", ToolCalls: toolCallsData},
			{Role: "tool", ToolCallId: "shell_command:12", Content: "file1.txt"},
			// 模拟 previous_response_id 合并导致的重复
			{Role: "tool", ToolCallId: "shell_command:12", Content: "file1.txt"},
		},
	}

	req.DeduplicateToolCallIDs()

	require.Len(t, req.Messages, 3)
	// 第一个 tool 消息保留
	require.Equal(t, "shell_command:12", req.Messages[2].ToolCallId)
}

func TestMergeConsecutiveMessages_UserMessages(t *testing.T) {
	req := &GeneralOpenAIRequest{
		Messages: []Message{
			{Role: "user", Content: "Hello"},
			{Role: "user", Content: "How are you?"},
			{Role: "assistant", Content: "Hi!"},
		},
	}
	req.MergeConsecutiveMessages()
	require.Len(t, req.Messages, 2)
	require.Equal(t, "user", req.Messages[0].Role)
	require.Equal(t, "Hello\nHow are you?", req.Messages[0].Content)
	require.Equal(t, "assistant", req.Messages[1].Role)
}

func TestMergeConsecutiveMessages_AssistantWithToolCalls(t *testing.T) {
	toolCalls1, _ := json.Marshal([]ToolCallRequest{
		{ID: "call_1", Type: "function", Function: FunctionRequest{Name: "search", Arguments: `{}`}},
	})
	toolCalls2, _ := json.Marshal([]ToolCallRequest{
		{ID: "call_2", Type: "function", Function: FunctionRequest{Name: "run", Arguments: `{}`}},
	})
	req := &GeneralOpenAIRequest{
		Messages: []Message{
			{Role: "user", Content: "Go"},
			{Role: "assistant", Content: "", ToolCalls: toolCalls1},
			{Role: "assistant", Content: "", ToolCalls: toolCalls2},
		},
	}
	req.MergeConsecutiveMessages()
	require.Len(t, req.Messages, 2)
	require.Equal(t, "user", req.Messages[0].Role)
	require.Equal(t, "assistant", req.Messages[1].Role)
	// 两个 assistant 消息的 tool_calls 应被合并
	mergedToolCalls := req.Messages[1].ParseToolCalls()
	require.Len(t, mergedToolCalls, 2)
	require.Equal(t, "call_1", mergedToolCalls[0].ID)
	require.Equal(t, "call_2", mergedToolCalls[1].ID)
}

func TestMergeConsecutiveMessages_ToolMessagesNotMerged(t *testing.T) {
	// tool 消息不应被合并，因为它们有独立的 tool_call_id
	req := &GeneralOpenAIRequest{
		Messages: []Message{
			{Role: "user", Content: "Run"},
			{Role: "tool", ToolCallId: "call_1", Content: "result1"},
			{Role: "tool", ToolCallId: "call_2", Content: "result2"},
		},
	}
	req.MergeConsecutiveMessages()
	require.Len(t, req.Messages, 3)
	require.Equal(t, "tool", req.Messages[1].Role)
	require.Equal(t, "call_1", req.Messages[1].ToolCallId)
	require.Equal(t, "tool", req.Messages[2].Role)
	require.Equal(t, "call_2", req.Messages[2].ToolCallId)
}

func TestMergeConsecutiveMessages_NoConsecutive(t *testing.T) {
	req := &GeneralOpenAIRequest{
		Messages: []Message{
			{Role: "user", Content: "Hi"},
			{Role: "assistant", Content: "Hello"},
		},
	}
	req.MergeConsecutiveMessages()
	require.Len(t, req.Messages, 2)
}

func TestMergeConsecutiveMessages_UserMultimodalArrays(t *testing.T) {
	req := &GeneralOpenAIRequest{
		Messages: []Message{
			{Role: "user", Content: []any{
				map[string]any{"type": "text", "text": "Look at this"},
				map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://example.com/a.png"}},
			}},
			{Role: "user", Content: []any{
				map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://example.com/b.png", "detail": "low"}},
				map[string]any{"type": "text", "text": "And this"},
			}},
		},
	}
	req.MergeConsecutiveMessages()
	require.Len(t, req.Messages, 1)
	parts := req.Messages[0].ParseContent()
	require.Len(t, parts, 4)
	require.Equal(t, ContentTypeText, parts[0].Type)
	require.Equal(t, "Look at this", parts[0].Text)
	require.Equal(t, ContentTypeImageURL, parts[1].Type)
	require.Equal(t, "https://example.com/a.png", parts[1].GetImageMedia().Url)
	require.Equal(t, ContentTypeImageURL, parts[2].Type)
	require.Equal(t, "https://example.com/b.png", parts[2].GetImageMedia().Url)
	require.Equal(t, ContentTypeText, parts[3].Type)
	require.Equal(t, "And this", parts[3].Text)

	// 序列化后字段与客户端原始内容一致：不注入 detail/MimeType 等多余字段
	data, err := common.Marshal(req.Messages[0].Content)
	require.NoError(t, err)
	require.Equal(t, "https://example.com/a.png", gjson.GetBytes(data, "1.image_url.url").String())
	require.False(t, gjson.GetBytes(data, "1.image_url.detail").Exists())
	require.False(t, gjson.GetBytes(data, "1.image_url.MimeType").Exists())
	require.Equal(t, "low", gjson.GetBytes(data, "2.image_url.detail").String())
}

func TestMergeConsecutiveMessages_UserStringAndArray(t *testing.T) {
	// string + 数组：文本转为 text part 前置，图片不丢失
	req := &GeneralOpenAIRequest{
		Messages: []Message{
			{Role: "user", Content: "Describe these"},
			{Role: "user", Content: []any{
				map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://example.com/a.png"}},
			}},
		},
	}
	req.MergeConsecutiveMessages()
	require.Len(t, req.Messages, 1)
	parts := req.Messages[0].ParseContent()
	require.Len(t, parts, 2)
	require.Equal(t, ContentTypeText, parts[0].Type)
	require.Equal(t, "Describe these", parts[0].Text)
	require.Equal(t, ContentTypeImageURL, parts[1].Type)

	// 数组 + string：文本转为 text part 后置
	req = &GeneralOpenAIRequest{
		Messages: []Message{
			{Role: "user", Content: []any{
				map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://example.com/a.png"}},
			}},
			{Role: "user", Content: "What is this?"},
		},
	}
	req.MergeConsecutiveMessages()
	require.Len(t, req.Messages, 1)
	parts = req.Messages[0].ParseContent()
	require.Len(t, parts, 2)
	require.Equal(t, ContentTypeImageURL, parts[0].Type)
	require.Equal(t, ContentTypeText, parts[1].Type)
	require.Equal(t, "What is this?", parts[1].Text)

	// 数组 + 空字符串：内容不丢失
	req = &GeneralOpenAIRequest{
		Messages: []Message{
			{Role: "user", Content: []any{
				map[string]any{"type": "text", "text": "keep me"},
			}},
			{Role: "user", Content: ""},
		},
	}
	req.MergeConsecutiveMessages()
	require.Len(t, req.Messages, 1)
	parts = req.Messages[0].ParseContent()
	require.Len(t, parts, 1)
	require.Equal(t, "keep me", parts[0].Text)
}

func TestMergeConsecutiveMessages_UserMediaContentSlices(t *testing.T) {
	// []MediaContent + []MediaContent（无缓存，模拟 DeepCopy 后的形态）
	req := &GeneralOpenAIRequest{
		Messages: []Message{
			{Role: "user", Content: []MediaContent{{Type: ContentTypeText, Text: "first"}}},
			{Role: "user", Content: []MediaContent{
				{Type: ContentTypeImageURL, ImageUrl: map[string]any{"url": "https://example.com/c.png"}},
			}},
		},
	}
	req.MergeConsecutiveMessages()
	require.Len(t, req.Messages, 1)
	parts := req.Messages[0].ParseContent()
	require.Len(t, parts, 2)
	require.Equal(t, "first", parts[0].Text)
	require.Equal(t, "https://example.com/c.png", parts[1].GetImageMedia().Url)

	// []any + []MediaContent 混合形态
	req = &GeneralOpenAIRequest{
		Messages: []Message{
			{Role: "user", Content: []any{map[string]any{"type": "text", "text": "a"}}},
			{Role: "user", Content: []MediaContent{{Type: ContentTypeText, Text: "b"}}},
		},
	}
	req.MergeConsecutiveMessages()
	require.Len(t, req.Messages, 1)
	parts = req.Messages[0].ParseContent()
	require.Len(t, parts, 2)
	require.Equal(t, "a", parts[0].Text)
	require.Equal(t, "b", parts[1].Text)
}

func TestMergeConsecutiveMessages_UserCacheControlPreserved(t *testing.T) {
	req := &GeneralOpenAIRequest{
		Messages: []Message{
			{Role: "user", Content: []any{
				map[string]any{"type": "text", "text": "hello", "cache_control": map[string]any{"type": "ephemeral"}},
			}},
			{Role: "user", Content: []any{
				map[string]any{"type": "text", "text": "world"},
			}},
		},
	}
	req.MergeConsecutiveMessages()
	require.Len(t, req.Messages, 1)
	data, err := common.Marshal(req.Messages[0].Content)
	require.NoError(t, err)
	require.Equal(t, "ephemeral", gjson.GetBytes(data, "0.cache_control.type").String())
	require.Equal(t, "world", gjson.GetBytes(data, "1.text").String())
}

func TestMergeConsecutiveMessages_AssistantMultimodalWithToolCalls(t *testing.T) {
	toolCalls1, _ := json.Marshal([]ToolCallRequest{
		{ID: "call_1", Type: "function", Function: FunctionRequest{Name: "search", Arguments: `{}`}},
	})
	toolCalls2, _ := json.Marshal([]ToolCallRequest{
		{ID: "call_2", Type: "function", Function: FunctionRequest{Name: "run", Arguments: `{}`}},
	})
	req := &GeneralOpenAIRequest{
		Messages: []Message{
			{Role: "user", Content: "Go"},
			{Role: "assistant", Content: []any{
				map[string]any{"type": "text", "text": "thinking"},
			}, ToolCalls: toolCalls1},
			{Role: "assistant", Content: "done", ToolCalls: toolCalls2},
		},
	}
	req.MergeConsecutiveMessages()
	require.Len(t, req.Messages, 2)
	parts := req.Messages[1].ParseContent()
	require.Len(t, parts, 2)
	require.Equal(t, "thinking", parts[0].Text)
	require.Equal(t, "done", parts[1].Text)
	mergedToolCalls := req.Messages[1].ParseToolCalls()
	require.Len(t, mergedToolCalls, 2)
	require.Equal(t, "call_1", mergedToolCalls[0].ID)
	require.Equal(t, "call_2", mergedToolCalls[1].ID)
}

func TestParseContent_MediaContentSliceWithoutCache(t *testing.T) {
	// 内部构造路径直接赋值 []MediaContent，不设缓存
	msg := Message{Role: "user", Content: []MediaContent{
		{Type: ContentTypeText, Text: "hello"},
		{Type: ContentTypeImageURL, ImageUrl: map[string]any{"url": "https://example.com/x.png"}},
	}}
	parts := msg.ParseContent()
	require.Len(t, parts, 2)
	require.Equal(t, "hello", parts[0].Text)
	require.Equal(t, "https://example.com/x.png", parts[1].GetImageMedia().Url)
	// 缓存已同步，二次调用结果一致
	require.Equal(t, parts, msg.ParseContent())

	// DeepCopy 丢失私有缓存后，ParseContent 仍应识别 []MediaContent
	src := &GeneralOpenAIRequest{Messages: []Message{{Role: "user"}}}
	src.Messages[0].SetMediaContent([]MediaContent{{Type: ContentTypeText, Text: "cached"}})
	copied, err := common.DeepCopy(src)
	require.NoError(t, err)
	copiedParts := copied.Messages[0].ParseContent()
	require.Len(t, copiedParts, 1)
	require.Equal(t, "cached", copiedParts[0].Text)
}

func TestMergeConsecutiveMessages_EmptyContentKeepsOriginalForm(t *testing.T) {
	// 前一条 assistant 内容为 nil（仅工具调用），后一条为字符串：
	// 合并结果应保持字符串形态，而不是升级为内容数组。
	req := &GeneralOpenAIRequest{
		Messages: []Message{
			{Role: "assistant", Content: nil},
			{Role: "assistant", Content: "done"},
		},
	}
	req.MergeConsecutiveMessages()
	require.Len(t, req.Messages, 1)
	require.Equal(t, "done", req.Messages[0].Content)

	// 空字符串 + 多模态数组：保留数组原始形态，且不丢图片
	parts := []any{
		map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://example.com/a.png"}},
	}
	req = &GeneralOpenAIRequest{
		Messages: []Message{
			{Role: "user", Content: ""},
			{Role: "user", Content: parts},
		},
	}
	req.MergeConsecutiveMessages()
	require.Len(t, req.Messages, 1)
	require.Equal(t, parts, req.Messages[0].Content)

	// 多模态数组 + 空数组：保留非空一侧
	req = &GeneralOpenAIRequest{
		Messages: []Message{
			{Role: "user", Content: parts},
			{Role: "user", Content: []any{}},
		},
	}
	req.MergeConsecutiveMessages()
	require.Len(t, req.Messages, 1)
	require.Equal(t, parts, req.Messages[0].Content)
}

func TestMessageStringContent_MediaContentSlice(t *testing.T) {
	// 合并后内容为 []MediaContent 时，纯文本渠道仍应能提取文本
	msg := Message{Role: "user", Content: []MediaContent{
		{Type: ContentTypeText, Text: "hello"},
		{Type: ContentTypeImageURL, ImageUrl: map[string]any{"url": "https://example.com/x.png"}},
		{Type: ContentTypeText, Text: " world"},
	}}
	require.Equal(t, "hello world", msg.StringContent())
	require.False(t, msg.IsStringContent())
}

func TestMediaContentGetImageMediaForms(t *testing.T) {
	// 值类型（responses 转换内部构造）与字符串形态都应能取出图片
	valueForm := &MediaContent{Type: ContentTypeImageURL, ImageUrl: MessageImageUrl{Url: "https://example.com/v.png", Detail: "low"}}
	img := valueForm.GetImageMedia()
	require.NotNil(t, img)
	require.Equal(t, "https://example.com/v.png", img.Url)
	require.Equal(t, "low", img.Detail)

	stringForm := &MediaContent{Type: ContentTypeImageURL, ImageUrl: "https://example.com/s.png"}
	img = stringForm.GetImageMedia()
	require.NotNil(t, img)
	require.Equal(t, "https://example.com/s.png", img.Url)

	// 指针与 map 形态保持原有行为
	ptrForm := &MediaContent{Type: ContentTypeImageURL, ImageUrl: &MessageImageUrl{Url: "https://example.com/p.png"}}
	require.Equal(t, "https://example.com/p.png", ptrForm.GetImageMedia().Url)
	mapForm := &MediaContent{Type: ContentTypeImageURL, ImageUrl: map[string]any{"url": "https://example.com/m.png", "mime_type": "image/png"}}
	require.Equal(t, "image/png", mapForm.GetImageMedia().MimeType)

	require.Nil(t, (&MediaContent{Type: ContentTypeText, Text: "x"}).GetImageMedia())
}
