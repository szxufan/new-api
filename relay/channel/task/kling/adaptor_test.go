package kling

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func newKlingRelayInfo(upstreamModel string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		OriginModelName: upstreamModel,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: upstreamModel,
		},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{
			PublicTaskID: "task_public_0001",
			Action:       constant.TaskActionGenerate,
		},
	}
}

// TestConvertToRequestPayloadImageFallback 锚定 D1 修复：
// 客户端只传 images 数组（不传 image）时，首张图片必须进入上游 image 字段，
// 而不是被丢弃（此前会退化为无图的 image2video 请求）。
func TestConvertToRequestPayloadImageFallback(t *testing.T) {
	a := &TaskAdaptor{}
	info := newKlingRelayInfo("kling-v1")

	tests := []struct {
		name      string
		req       relaycommon.TaskSubmitReq
		wantImage string
	}{
		{
			name:      "image field wins",
			req:       relaycommon.TaskSubmitReq{Prompt: "p", Image: "https://x/a.png"},
			wantImage: "https://x/a.png",
		},
		{
			name:      "images only falls back to first",
			req:       relaycommon.TaskSubmitReq{Prompt: "p", Images: []string{"https://x/a.png", "https://x/b.png"}},
			wantImage: "https://x/a.png",
		},
		{
			name:      "image preferred over images",
			req:       relaycommon.TaskSubmitReq{Prompt: "p", Image: "https://x/a.png", Images: []string{"https://x/b.png"}},
			wantImage: "https://x/a.png",
		},
		{
			name:      "no image at all",
			req:       relaycommon.TaskSubmitReq{Prompt: "p"},
			wantImage: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, err := a.convertToRequestPayload(&tt.req, info)
			if err != nil {
				t.Fatalf("convertToRequestPayload() error = %v", err)
			}
			if payload.Image != tt.wantImage {
				t.Errorf("payload.Image = %q, want %q", payload.Image, tt.wantImage)
			}
		})
	}
}

// TestConvertToRequestPayloadModeDefault 锚定现状：
// req.Mode 映射为可灵清晰度档位，缺省为 std。
func TestConvertToRequestPayloadModeDefault(t *testing.T) {
	a := &TaskAdaptor{}
	info := newKlingRelayInfo("kling-v1")

	payload, err := a.convertToRequestPayload(&relaycommon.TaskSubmitReq{Prompt: "p"}, info)
	if err != nil {
		t.Fatalf("convertToRequestPayload() error = %v", err)
	}
	if payload.Mode != "std" {
		t.Errorf("payload.Mode = %q, want std", payload.Mode)
	}

	payload, err = a.convertToRequestPayload(&relaycommon.TaskSubmitReq{Prompt: "p", Mode: "pro"}, info)
	if err != nil {
		t.Fatalf("convertToRequestPayload() error = %v", err)
	}
	if payload.Mode != "pro" {
		t.Errorf("payload.Mode = %q, want pro", payload.Mode)
	}
}

// TestConvertToRequestPayloadNamedMetadataKeys 阶段 1 增量：
// metadata 具名键显式指定首/尾帧；尾帧不从图片数量推导（现状语义）。
func TestConvertToRequestPayloadNamedMetadataKeys(t *testing.T) {
	a := &TaskAdaptor{}
	info := newKlingRelayInfo("kling-v1")

	// 显式首尾帧
	payload, err := a.convertToRequestPayload(&relaycommon.TaskSubmitReq{
		Prompt: "p",
		Metadata: map[string]interface{}{
			relaycommon.MetadataKeyFirstFrame: "https://x/first.png",
			relaycommon.MetadataKeyLastFrame:  "https://x/last.png",
		},
	}, info)
	if err != nil {
		t.Fatalf("convertToRequestPayload() error = %v", err)
	}
	if payload.Image != "https://x/first.png" || payload.ImageTail != "https://x/last.png" {
		t.Errorf("image/image_tail = (%q, %q), want metadata first/last", payload.Image, payload.ImageTail)
	}

	// 2 张图隐式路径：只用首张，尾帧保持为空（零回归）
	payload, err = a.convertToRequestPayload(&relaycommon.TaskSubmitReq{
		Prompt: "p",
		Images: []string{"https://x/a.png", "https://x/b.png"},
	}, info)
	if err != nil {
		t.Fatalf("convertToRequestPayload() error = %v", err)
	}
	if payload.Image != "https://x/a.png" {
		t.Errorf("payload.Image = %q, want https://x/a.png", payload.Image)
	}
	if payload.ImageTail != "" {
		t.Errorf("payload.ImageTail = %q, want empty (implicit path uses single image)", payload.ImageTail)
	}

	// req.Image 优先于 metadata 具名键（现状字段优先）
	payload, err = a.convertToRequestPayload(&relaycommon.TaskSubmitReq{
		Prompt: "p",
		Image:  "https://x/plain.png",
		Metadata: map[string]interface{}{
			relaycommon.MetadataKeyFirstFrame: "https://x/first.png",
		},
	}, info)
	if err != nil {
		t.Fatalf("convertToRequestPayload() error = %v", err)
	}
	if payload.Image != "https://x/plain.png" {
		t.Errorf("payload.Image = %q, want req.Image to win", payload.Image)
	}
}
