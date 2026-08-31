package vidu

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
)

func newViduTestContext(t *testing.T, body string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c
}

func newViduRelayInfo() *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeVidu,
			UpstreamModelName: "viduq2",
		},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{
			PublicTaskID: "task_public_0001",
		},
	}
}

func TestIsViduAction(t *testing.T) {
	tests := []struct {
		action string
		want   bool
	}{
		{constant.TaskActionTextGenerate, true},
		{constant.TaskActionGenerate, true},
		{constant.TaskActionFirstTailGenerate, true},
		{constant.TaskActionReferenceGenerate, true},
		{"", false},
		{"bogus", false},
		{constant.TaskActionRemix, false},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q", tt.action), func(t *testing.T) {
			if got := isViduAction(tt.action); got != tt.want {
				t.Errorf("isViduAction(%q) = %v, want %v", tt.action, got, tt.want)
			}
		})
	}
}

// TestValidateRequestAndSetAction 锚定 vidu 模式判定：
// 显式 metadata.action（D2 白名单）优先，缺省时按图片数量隐式推导。
func TestValidateRequestAndSetAction(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantAction string
		wantErr    bool
	}{
		{
			name:       "no images -> text2video",
			body:       `{"model":"viduq2","prompt":"p"}`,
			wantAction: constant.TaskActionTextGenerate,
		},
		{
			name:       "one image -> first frame",
			body:       `{"model":"viduq2","prompt":"p","images":["https://x/a.png"]}`,
			wantAction: constant.TaskActionGenerate,
		},
		{
			name:       "two images -> first-last frame",
			body:       `{"model":"viduq2","prompt":"p","images":["https://x/a.png","https://x/b.png"]}`,
			wantAction: constant.TaskActionFirstTailGenerate,
		},
		{
			name:       "three images -> reference",
			body:       `{"model":"viduq2","prompt":"p","images":["https://x/a.png","https://x/b.png","https://x/c.png"]}`,
			wantAction: constant.TaskActionReferenceGenerate,
		},
		{
			name:       "explicit metadata.action wins",
			body:       `{"model":"viduq2","prompt":"p","images":["https://x/a.png"],"metadata":{"action":"referenceGenerate"}}`,
			wantAction: constant.TaskActionReferenceGenerate,
		},
		{
			name:    "invalid metadata.action rejected (D2)",
			body:    `{"model":"viduq2","prompt":"p","metadata":{"action":"bogus"}}`,
			wantErr: true,
		},
		{
			name:    "non-string metadata.action rejected (D2)",
			body:    `{"model":"viduq2","prompt":"p","metadata":{"action":123}}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newViduTestContext(t, tt.body)
			info := newViduRelayInfo()
			a := &TaskAdaptor{ChannelType: constant.ChannelTypeVidu}

			taskErr := a.ValidateRequestAndSetAction(c, info)
			if tt.wantErr {
				if taskErr == nil {
					t.Fatalf("ValidateRequestAndSetAction() expected error, got nil (action=%q)", info.Action)
				}
				return
			}
			if taskErr != nil {
				t.Fatalf("ValidateRequestAndSetAction() error = %v", taskErr)
			}
			if info.Action != tt.wantAction {
				t.Errorf("info.Action = %q, want %q", info.Action, tt.wantAction)
			}
		})
	}
}

// TestValidateRequestAndSetActionNamedMetadataKeys 阶段 1 增量：
// 无 images 时，统一 metadata 具名键也能推导模式。
func TestValidateRequestAndSetActionNamedMetadataKeys(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantAction string
	}{
		{
			name:       "first_frame_image only -> image2video",
			body:       `{"model":"viduq2","prompt":"p","metadata":{"first_frame_image":"https://x/a.png"}}`,
			wantAction: constant.TaskActionGenerate,
		},
		{
			name:       "first+last frame keys -> first-last",
			body:       `{"model":"viduq2","prompt":"p","metadata":{"first_frame_image":"https://x/a.png","last_frame_image":"https://x/b.png"}}`,
			wantAction: constant.TaskActionFirstTailGenerate,
		},
		{
			name:       "reference_images key -> reference",
			body:       `{"model":"viduq2","prompt":"p","metadata":{"reference_images":["https://x/a.png","https://x/b.png"]}}`,
			wantAction: constant.TaskActionReferenceGenerate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newViduTestContext(t, tt.body)
			info := newViduRelayInfo()
			a := &TaskAdaptor{ChannelType: constant.ChannelTypeVidu}

			if taskErr := a.ValidateRequestAndSetAction(c, info); taskErr != nil {
				t.Fatalf("ValidateRequestAndSetAction() error = %v", taskErr)
			}
			if info.Action != tt.wantAction {
				t.Errorf("info.Action = %q, want %q", info.Action, tt.wantAction)
			}
		})
	}
}

// TestConvertToRequestPayloadNamedKeysFlatten 阶段 1 增量：
// 使用具名键时，images 按「首帧→尾帧→参考图」展平；未使用时保持原样。
func TestConvertToRequestPayloadNamedKeysFlatten(t *testing.T) {
	a := &TaskAdaptor{ChannelType: constant.ChannelTypeVidu}
	info := newViduRelayInfo()

	req := relaycommon.TaskSubmitReq{
		Prompt: "p",
		Model:  "viduq2",
		Metadata: map[string]interface{}{
			relaycommon.MetadataKeyFirstFrame:      "https://x/first.png",
			relaycommon.MetadataKeyLastFrame:       "https://x/last.png",
			relaycommon.MetadataKeyReferenceImages: []string{"https://x/r1.png"},
		},
	}
	payload, err := a.convertToRequestPayload(&req, info)
	if err != nil {
		t.Fatalf("convertToRequestPayload() error = %v", err)
	}
	want := []string{"https://x/first.png", "https://x/last.png", "https://x/r1.png"}
	if len(payload.Images) != len(want) {
		t.Fatalf("payload.Images = %v, want %v", payload.Images, want)
	}
	for i := range want {
		if payload.Images[i] != want[i] {
			t.Errorf("payload.Images[%d] = %q, want %q", i, payload.Images[i], want[i])
		}
	}

	// 未使用具名键：req.Images 原样透传（现状）
	req2 := relaycommon.TaskSubmitReq{Prompt: "p", Model: "viduq2", Images: []string{"https://x/a.png", "https://x/b.png"}}
	payload2, err := a.convertToRequestPayload(&req2, info)
	if err != nil {
		t.Fatalf("convertToRequestPayload() error = %v", err)
	}
	if len(payload2.Images) != 2 || payload2.Images[0] != "https://x/a.png" || payload2.Images[1] != "https://x/b.png" {
		t.Errorf("payload2.Images = %v, want req.Images unchanged", payload2.Images)
	}
}
