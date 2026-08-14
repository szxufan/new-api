package relay

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

// TestApplyRetryAntiCacheOpenAI 覆盖 OpenAI 兼容格式的追加逻辑
func TestApplyRetryAntiCacheOpenAI(t *testing.T) {
	t.Run("未重试时（retryIndex<=1）不修改", func(t *testing.T) {
		messages := []dto.Message{{Role: "user", Content: "hello"}}
		modified := applyRetryAntiCacheOpenAI(messages, "快做！", 1)
		require.False(t, modified)
		require.Equal(t, "hello", messages[0].Content)

		modified = applyRetryAntiCacheOpenAI(messages, "快做！", 0)
		require.False(t, modified)
		require.Equal(t, "hello", messages[0].Content)
	})

	t.Run("内容为空时不修改", func(t *testing.T) {
		messages := []dto.Message{{Role: "user", Content: "hello"}}
		modified := applyRetryAntiCacheOpenAI(messages, "", 2)
		require.False(t, modified)
		require.Equal(t, "hello", messages[0].Content)
	})

	t.Run("第1次重试追加1个，第2次重试追加2个", func(t *testing.T) {
		first := []dto.Message{{Role: "user", Content: "hello"}}
		require.True(t, applyRetryAntiCacheOpenAI(first, "快做！", 2))
		require.Equal(t, "hello快做！", first[0].Content)

		second := []dto.Message{{Role: "user", Content: "hello"}}
		require.True(t, applyRetryAntiCacheOpenAI(second, "快做！", 3))
		require.Equal(t, "hello"+strings.Repeat("快做！", 2), second[0].Content)
	})

	t.Run("数组型内容在末尾追加text块", func(t *testing.T) {
		content := []any{
			map[string]any{"type": "text", "text": "hi"},
			map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:image/png;base64,xxx"}},
		}
		messages := []dto.Message{{Role: "user", Content: content}}
		require.True(t, applyRetryAntiCacheOpenAI(messages, "快做！", 2))
		updated := messages[0].Content.([]any)
		require.Len(t, updated, 3)
		require.Equal(t, "hi", updated[0].(map[string]any)["text"])
		require.Equal(t, "快做！", updated[2].(map[string]any)["text"])
		require.Equal(t, "text", updated[2].(map[string]any)["type"])
	})

	t.Run("最后一条非user消息时回退到上一条user", func(t *testing.T) {
		messages := []dto.Message{
			{Role: "user", Content: "first"},
			{Role: "assistant", Content: "answer"},
		}
		require.True(t, applyRetryAntiCacheOpenAI(messages, "快做！", 2))
		require.Equal(t, "first快做！", messages[0].Content)
		require.Equal(t, "answer", messages[1].Content)
	})

	t.Run("无user消息时不修改且不panic", func(t *testing.T) {
		messages := []dto.Message{{Role: "system", Content: "sys"}, {Role: "assistant", Content: "answer"}}
		require.False(t, applyRetryAntiCacheOpenAI(messages, "快做！", 2))
		require.Equal(t, "sys", messages[0].Content)
		require.Equal(t, "answer", messages[1].Content)
	})
}

// TestApplyRetryAntiCacheClaude 覆盖 Claude 格式的追加逻辑
func TestApplyRetryAntiCacheClaude(t *testing.T) {
	t.Run("字符串内容", func(t *testing.T) {
		messages := []dto.ClaudeMessage{{Role: "user", Content: "hello"}}
		require.True(t, applyRetryAntiCacheClaude(messages, "快做！", 2))
		require.Equal(t, "hello快做！", messages[0].Content)
	})

	t.Run("content blocks 在末尾追加text块", func(t *testing.T) {
		content := []any{
			map[string]any{"type": "text", "text": "hi"},
			map[string]any{"type": "image", "source": map[string]any{"type": "base64"}},
		}
		messages := []dto.ClaudeMessage{{Role: "user", Content: content}}
		require.True(t, applyRetryAntiCacheClaude(messages, "快做！", 3))
		updated := messages[0].Content.([]any)
		require.Len(t, updated, 3)
		require.Equal(t, strings.Repeat("快做！", 2), updated[2].(map[string]any)["text"])
	})

	t.Run("未重试时不修改", func(t *testing.T) {
		messages := []dto.ClaudeMessage{{Role: "user", Content: "hello"}}
		require.False(t, applyRetryAntiCacheClaude(messages, "快做！", 1))
		require.Equal(t, "hello", messages[0].Content)
	})
}

// TestApplyRetryAntiCacheResponses 覆盖 OpenAI Responses 格式的追加逻辑
func TestApplyRetryAntiCacheResponses(t *testing.T) {
	t.Run("字符串content", func(t *testing.T) {
		input := json.RawMessage(`[{"role":"system","content":"sys"},{"role":"user","content":"hello"}]`)
		out, err := applyRetryAntiCacheResponses(input, "快做！", 2)
		require.NoError(t, err)
		var msgs []map[string]interface{}
		require.NoError(t, json.Unmarshal(out, &msgs))
		require.Equal(t, "hello快做！", msgs[1]["content"])
		require.Equal(t, "sys", msgs[0]["content"])
	})

	t.Run("数组content在末尾追加input_text块", func(t *testing.T) {
		input := json.RawMessage(`[{"role":"user","content":[{"type":"input_text","text":"hi"}]}]`)
		out, err := applyRetryAntiCacheResponses(input, "快做！", 3)
		require.NoError(t, err)
		var msgs []map[string]interface{}
		require.NoError(t, json.Unmarshal(out, &msgs))
		content := msgs[0]["content"].([]interface{})
		require.Len(t, content, 2)
		require.Equal(t, "hi", content[0].(map[string]interface{})["text"])
		require.Equal(t, strings.Repeat("快做！", 2), content[1].(map[string]interface{})["text"])
	})

	t.Run("非法JSON返回原输入不报错", func(t *testing.T) {
		input := json.RawMessage(`not-json`)
		out, err := applyRetryAntiCacheResponses(input, "快做！", 2)
		require.NoError(t, err)
		require.Equal(t, string(input), string(out))
	})

	t.Run("未重试时不修改", func(t *testing.T) {
		input := json.RawMessage(`[{"role":"user","content":"hello"}]`)
		out, err := applyRetryAntiCacheResponses(input, "快做！", 1)
		require.NoError(t, err)
		require.Equal(t, string(input), string(out))
	})
}
