package middleware

import (
	"net/http"
	"testing"

	relayconstant "github.com/QuantumNous/new-api/relay/constant"
)

func TestResolveVideoRelayMode(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		want   int
	}{
		// OpenAI 兼容视频任务 API
		{"v1 videos submit", http.MethodPost, "/v1/videos", relayconstant.RelayModeVideoSubmit},
		{"v1 videos fetch", http.MethodGet, "/v1/videos/task-123", relayconstant.RelayModeVideoFetchByID},
		// playground 视频调试（登录会话）
		{"pg videos submit", http.MethodPost, "/pg/videos", relayconstant.RelayModeVideoSubmit},
		{"pg videos fetch", http.MethodGet, "/pg/videos/task-123", relayconstant.RelayModeVideoFetchByID},
		// Sora 风格端点
		{"v1 video generations submit", http.MethodPost, "/v1/video/generations", relayconstant.RelayModeVideoSubmit},
		{"v1 video generations fetch", http.MethodGet, "/v1/video/generations/task-123", relayconstant.RelayModeVideoFetchByID},
		// remix：也属于提交动作
		{"remix submit", http.MethodPost, "/v1/videos/video-456/remix", relayconstant.RelayModeVideoSubmit},
		// 非视频路径
		{"chat completions", http.MethodPost, "/v1/chat/completions", relayconstant.RelayModeUnknown},
		{"images generations", http.MethodPost, "/pg/images/generations", relayconstant.RelayModeUnknown},
		{"images edits", http.MethodPost, "/pg/images/edits", relayconstant.RelayModeUnknown},
		// 不支持的 HTTP 方法
		{"videos put", http.MethodPut, "/v1/videos", relayconstant.RelayModeUnknown},
		{"videos delete", http.MethodDelete, "/v1/videos/task-123", relayconstant.RelayModeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveVideoRelayMode(tt.method, tt.path); got != tt.want {
				t.Errorf("resolveVideoRelayMode(%s, %s) = %d, want %d", tt.method, tt.path, got, tt.want)
			}
		})
	}
}