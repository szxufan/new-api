package relay

import (
	"encoding/json"
	"strings"

	"github.com/QuantumNous/new-api/dto"
)

// applyRetryAntiCacheOpenAI 在 OpenAI 兼容格式（chat/completions）的消息列表中，
// 给最后一条 user 消息末尾追加内容，使重试请求体与首次请求不同，避免命中上游错误缓存。
// retryIndex 为当前尝试序号（首次尝试为 1）；第 N 次重试（retryIndex=N+1）追加 N 个 content。
// 仅在 retryIndex > 1 且 content 非空时生效；返回是否发生了修改。
func applyRetryAntiCacheOpenAI(messages []dto.Message, content string, retryIndex int) bool {
	if retryIndex <= 1 || content == "" {
		return false
	}
	repeated := strings.Repeat(content, retryIndex-1)
	// 倒序查找最后一条 user 消息，避免改动 system 消息
	for i := len(messages) - 1; i >= 0; i-- {
		msg := &messages[i]
		if msg.Role != "user" {
			continue
		}
		switch c := msg.Content.(type) {
		case string:
			msg.Content = c + repeated
		case []any:
			// 数组型（多模态）内容：末尾追加一个 text 块，不破坏已有结构
			msg.Content = append(c, map[string]any{"type": dto.ContentTypeText, "text": repeated})
		default:
			return false
		}
		return true
	}
	return false
}

// applyRetryAntiCacheClaude 在 Claude 格式（/v1/messages）的消息列表中执行同样的追加逻辑。
// Claude 的 content 同样支持字符串与 content blocks 数组两种形态。
func applyRetryAntiCacheClaude(messages []dto.ClaudeMessage, content string, retryIndex int) bool {
	if retryIndex <= 1 || content == "" {
		return false
	}
	repeated := strings.Repeat(content, retryIndex-1)
	for i := len(messages) - 1; i >= 0; i-- {
		msg := &messages[i]
		if msg.Role != "user" {
			continue
		}
		switch c := msg.Content.(type) {
		case string:
			msg.Content = c + repeated
		case []any:
			msg.Content = append(c, map[string]any{"type": dto.ContentTypeText, "text": repeated})
		default:
			return false
		}
		return true
	}
	return false
}

// applyRetryAntiCacheResponses 在 OpenAI Responses 格式（/v1/responses）的 Input
// （json.RawMessage）中执行同样的追加逻辑。解析失败时返回原输入且不报错。
func applyRetryAntiCacheResponses(input json.RawMessage, content string, retryIndex int) (json.RawMessage, error) {
	if retryIndex <= 1 || content == "" || len(input) == 0 {
		return input, nil
	}
	repeated := strings.Repeat(content, retryIndex-1)
	var msgs []map[string]interface{}
	if err := json.Unmarshal(input, &msgs); err != nil {
		return input, nil
	}
	for i := len(msgs) - 1; i >= 0; i-- {
		if role, _ := msgs[i]["role"].(string); role != "user" {
			continue
		}
		switch c := msgs[i]["content"].(type) {
		case string:
			msgs[i]["content"] = c + repeated
		case []any:
			// 数组型内容：末尾追加 input_text 块
			msgs[i]["content"] = append(c, map[string]any{"type": "input_text", "text": repeated})
		default:
			return input, nil
		}
		break
	}
	b, err := json.Marshal(msgs)
	if err != nil {
		return input, nil
	}
	return json.RawMessage(b), nil
}
