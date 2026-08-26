package relay

import (
	"testing"
)

func TestIsOpenAIVideoAPIPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{"v1 videos fetch", "/v1/videos/task-123", true},
		{"v1 videos fetch with query", "/v1/videos/task-123?foo=bar", true},
		{"pg videos fetch", "/pg/videos/task-123", true},
		{"pg videos fetch with query", "/pg/videos/task-123?foo=bar", true},
		{"v1 video generations legacy", "/v1/video/generations/task-123", false},
		{"pg images generations", "/pg/images/generations", false},
		{"v1 videos submit", "/v1/videos", false},
		{"pg videos submit", "/pg/videos", false},
		{"chat completions", "/v1/chat/completions", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isOpenAIVideoAPIPath(tt.path); got != tt.want {
				t.Errorf("isOpenAIVideoAPIPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}