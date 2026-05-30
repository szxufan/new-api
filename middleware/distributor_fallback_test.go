package middleware

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHasMultimodalContentInBytes_PureText(t *testing.T) {
	body := []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hello"}]}`)
	assert.False(t, hasMultimodalContentInBytes(body))
}

func TestHasMultimodalContentInBytes_OpenAIImageUrl(t *testing.T) {
	body := []byte(`{
		"model":"gpt-4",
		"messages":[{
			"role":"user",
			"content":[
				{"type":"text","text":"what is this"},
				{"type":"image_url","image_url":{"url":"https://example.com/img.png"}}
			]
		}]
	}`)
	assert.True(t, hasMultimodalContentInBytes(body))
}

func TestHasMultimodalContentInBytes_ClaudeImage(t *testing.T) {
	body := []byte(`{
		"model":"claude-sonnet",
		"messages":[{
			"role":"user",
			"content":[
				{"type":"text","text":"describe"},
				{"type":"image","source":{"type":"base64","media_type":"image/png","data":"abc"}}
			]
		}]
	}`)
	assert.True(t, hasMultimodalContentInBytes(body))
}

func TestHasMultimodalContentInBytes_VideoUrl(t *testing.T) {
	body := []byte(`{
		"model":"gpt-4",
		"messages":[{
			"role":"user",
			"content":[{"type":"video_url","video_url":{"url":"https://example.com/v.mp4"}}]
		}]
	}`)
	assert.True(t, hasMultimodalContentInBytes(body))
}

func TestHasMultimodalContentInBytes_InputAudio(t *testing.T) {
	body := []byte(`{
		"model":"gpt-4",
		"messages":[{
			"role":"user",
			"content":[{"type":"input_audio","input_audio":{"data":"base64..."}}]
		}]
	}`)
	assert.True(t, hasMultimodalContentInBytes(body))
}

func TestHasMultimodalContentInBytes_File(t *testing.T) {
	body := []byte(`{
		"model":"gpt-4",
		"messages":[{
			"role":"user",
			"content":[{"type":"file","file":{"filename":"doc.pdf"}}]
		}]
	}`)
	assert.True(t, hasMultimodalContentInBytes(body))
}

func TestHasMultimodalContentInBytes_ResponsesInputImage(t *testing.T) {
	body := []byte(`{"model":"gpt-4","input":[{"type":"input_image","image_url":"..."}]}`)
	assert.True(t, hasMultimodalContentInBytes(body))
}

func TestHasMultimodalContentInBytes_ResponsesInputVideo(t *testing.T) {
	body := []byte(`{"model":"gpt-4","input":[{"type":"input_video","video_url":"..."}]}`)
	assert.True(t, hasMultimodalContentInBytes(body))
}

func TestHasMultimodalContentInBytes_ResponsesInputFile(t *testing.T) {
	body := []byte(`{"model":"gpt-4","input":[{"type":"input_file","filename":"a.pdf"}]}`)
	assert.True(t, hasMultimodalContentInBytes(body))
}

func TestHasMultimodalContentInBytes_NestedResponsesInput(t *testing.T) {
	body := []byte(`{
		"model":"gpt-4",
		"input":[{
			"type":"message",
			"content":[{"type":"input_image","image_url":"..."}]
		}]
	}`)
	assert.True(t, hasMultimodalContentInBytes(body))
}

func TestHasMultimodalContentInBytes_InvalidJSON(t *testing.T) {
	body := []byte(`not json`)
	assert.False(t, hasMultimodalContentInBytes(body))
}

func TestHasMultimodalContentInBytes_EmptyBody(t *testing.T) {
	body := []byte(`{}`)
	assert.False(t, hasMultimodalContentInBytes(body))
}

func TestHasMultimodalContentInBytes_NoMessages(t *testing.T) {
	body := []byte(`{"model":"gpt-4","prompt":"hello"}`)
	assert.False(t, hasMultimodalContentInBytes(body))
}

func TestHasMultimodalContentInBytes_NoFallbackModel(t *testing.T) {
	body := []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hello"}]}`)
	assert.False(t, hasMultimodalContentInBytes(body))
}