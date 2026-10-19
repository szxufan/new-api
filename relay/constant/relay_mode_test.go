package constant

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPath2RelayModeImagesGenerations(t *testing.T) {
	tests := []struct {
		name string
		path string
		want int
	}{
		{name: "v1 images generations", path: "/v1/images/generations", want: RelayModeImagesGenerations},
		{name: "v1 images generations with query", path: "/v1/images/generations?model=dall-e-3", want: RelayModeImagesGenerations},
		{name: "playground images generations", path: "/pg/images/generations", want: RelayModeImagesGenerations},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Path2RelayMode(tt.path))
		})
	}
}

func TestPath2RelayModeImagesEdits(t *testing.T) {
	tests := []struct {
		name string
		path string
		want int
	}{
		{name: "v1 images edits", path: "/v1/images/edits", want: RelayModeImagesEdits},
		{name: "playground images edits", path: "/pg/images/edits", want: RelayModeImagesEdits},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Path2RelayMode(tt.path))
		})
	}
}

func TestPath2RelayModeChatCompletionsRegression(t *testing.T) {
	assert.Equal(t, RelayModeChatCompletions, Path2RelayMode("/v1/chat/completions"))
	// 原有 /pg 聊天路径行为保持不变
	assert.Equal(t, RelayModeChatCompletions, Path2RelayMode("/pg/chat/completions"))
}

func TestPath2RelayModeUnknown(t *testing.T) {
	assert.Equal(t, RelayModeUnknown, Path2RelayMode("/pg/unknown/path"))
}
func TestPath2RelayModeWebTools(t *testing.T) {
	tests := []struct {
		name string
		path string
		want int
	}{
		{name: "v1 tools search", path: "/v1/tools/search", want: RelayModeWebSearch},
		{name: "v1 tools fetch", path: "/v1/tools/fetch", want: RelayModeWebFetch},
		{name: "v1 tools search with query", path: "/v1/tools/search?count=3", want: RelayModeWebSearch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Path2RelayMode(tt.path))
		})
	}
	// 未知的 tools 子路径不应误命中
	assert.Equal(t, RelayModeUnknown, Path2RelayMode("/v1/tools/other"))
}
