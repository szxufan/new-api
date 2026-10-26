package dto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsSystemLikeRole(t *testing.T) {
	assert.True(t, IsSystemLikeRole("system"))
	assert.True(t, IsSystemLikeRole("developer"))
	assert.False(t, IsSystemLikeRole("user"))
	assert.False(t, IsSystemLikeRole("assistant"))
	assert.False(t, IsSystemLikeRole("tool"))
}

func TestNormalizeDeveloperRole(t *testing.T) {
	request := &GeneralOpenAIRequest{
		Messages: []Message{
			{Role: "developer", Content: "system prompt"},
			{Role: "user", Content: "hi"},
			{Role: "assistant", Content: "hello"},
			{Role: "developer", Content: "extra instruction"},
		},
	}
	request.NormalizeDeveloperRole()
	assert.Equal(t, "system", request.Messages[0].Role)
	assert.Equal(t, "user", request.Messages[1].Role)
	assert.Equal(t, "assistant", request.Messages[2].Role)
	assert.Equal(t, "system", request.Messages[3].Role)
}

func TestNormalizeDeveloperRole_MixedSystemAndDeveloperMerged(t *testing.T) {
	// system 与 developer 混传：相邻的合并为一条 system，内容不丢失
	request := &GeneralOpenAIRequest{
		Messages: []Message{
			{Role: "system", Content: "base prompt"},
			{Role: "developer", Content: "extra instruction"},
			{Role: "user", Content: "hi"},
		},
	}
	request.NormalizeDeveloperRole()
	assert.Len(t, request.Messages, 2)
	assert.Equal(t, "system", request.Messages[0].Role)
	assert.Equal(t, "base prompt\nextra instruction", request.Messages[0].Content)
	assert.Equal(t, "user", request.Messages[1].Role)
}

func TestNormalizeDeveloperRole_DeveloperThenSystemMerged(t *testing.T) {
	request := &GeneralOpenAIRequest{
		Messages: []Message{
			{Role: "developer", Content: "first"},
			{Role: "system", Content: "second"},
			{Role: "user", Content: "hi"},
		},
	}
	request.NormalizeDeveloperRole()
	assert.Len(t, request.Messages, 2)
	assert.Equal(t, "system", request.Messages[0].Role)
	assert.Equal(t, "first\nsecond", request.Messages[0].Content)
}

func TestNormalizeDeveloperRole_ScatteredSystemKeptSeparate(t *testing.T) {
	// 非相邻的 system 消息保持独立，不做合并
	request := &GeneralOpenAIRequest{
		Messages: []Message{
			{Role: "system", Content: "first"},
			{Role: "user", Content: "hi"},
			{Role: "developer", Content: "mid instruction"},
			{Role: "user", Content: "again"},
		},
	}
	request.NormalizeDeveloperRole()
	assert.Len(t, request.Messages, 4)
	assert.Equal(t, "system", request.Messages[0].Role)
	assert.Equal(t, "system", request.Messages[2].Role)
	assert.Equal(t, "mid instruction", request.Messages[2].Content)
}

func TestNormalizeDeveloperRole_NonStringSystemNotMerged(t *testing.T) {
	// 多模态数组内容的 system 不参与合并，保持原样避免丢数据
	arrayContent := []any{map[string]any{"type": "text", "text": "structured"}}
	request := &GeneralOpenAIRequest{
		Messages: []Message{
			{Role: "system", Content: "plain"},
			{Role: "developer", Content: arrayContent},
			{Role: "user", Content: "hi"},
		},
	}
	request.NormalizeDeveloperRole()
	assert.Len(t, request.Messages, 3)
	assert.Equal(t, "system", request.Messages[0].Role)
	assert.Equal(t, "plain", request.Messages[0].Content)
	assert.Equal(t, "system", request.Messages[1].Role)
	assert.Equal(t, arrayContent, request.Messages[1].Content)
}
