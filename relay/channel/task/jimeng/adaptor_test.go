package jimeng

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func newJimengRelayInfo(upstreamModel string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		OriginModelName: upstreamModel,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: upstreamModel,
		},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{
			PublicTaskID: "task_public_0001",
			// ValidateBasicTaskRequest 的默认 action
			Action: constant.TaskActionGenerate,
		},
	}
}

// TestConvertToRequestPayloadV30ReqKeyAndAction 锚定即梦 v30 的 req_key 切换
// 与 D3 修复后的 info.Action 回填：两者必须始终一致。
func TestConvertToRequestPayloadV30ReqKeyAndAction(t *testing.T) {
	tests := []struct {
		name        string
		model       string
		images      []string
		wantReqKey  string
		wantAction  string
	}{
		{
			name:       "v30 no images -> t2v",
			model:      "jimeng_v30",
			wantReqKey: "jimeng_t2v_v30",
			wantAction: constant.TaskActionTextGenerate,
		},
		{
			name:       "v30 one image -> first frame",
			model:      "jimeng_v30",
			images:     []string{"https://x/a.png"},
			wantReqKey: "jimeng_i2v_first_v30",
			wantAction: constant.TaskActionGenerate,
		},
		{
			name:       "v30 two images -> first-last frame",
			model:      "jimeng_v30",
			images:     []string{"https://x/a.png", "https://x/b.png"},
			wantReqKey: "jimeng_i2v_first_tail_v30",
			wantAction: constant.TaskActionFirstTailGenerate,
		},
		{
			// §1.2 现状语义：即梦 ≥3 张图仍走首尾帧 req_key
			name:       "v30 three images -> still first-last frame",
			model:      "jimeng_v30",
			images:     []string{"https://x/a.png", "https://x/b.png", "https://x/c.png"},
			wantReqKey: "jimeng_i2v_first_tail_v30",
			wantAction: constant.TaskActionFirstTailGenerate,
		},
		{
			name:       "v30 pro fixed req_key, action follows images",
			model:      "jimeng_v30_pro",
			images:     []string{"https://x/a.png"},
			wantReqKey: "jimeng_ti2v_v30_pro",
			wantAction: constant.TaskActionGenerate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &TaskAdaptor{}
			info := newJimengRelayInfo(tt.model)
			req := relaycommon.TaskSubmitReq{
				Prompt: "p",
				Model:  tt.model,
				Images: tt.images,
			}

			payload, err := a.convertToRequestPayload(&req, info)
			if err != nil {
				t.Fatalf("convertToRequestPayload() error = %v", err)
			}
			if payload.ReqKey != tt.wantReqKey {
				t.Errorf("payload.ReqKey = %q, want %q", payload.ReqKey, tt.wantReqKey)
			}
			if info.Action != tt.wantAction {
				t.Errorf("info.Action = %q, want %q", info.Action, tt.wantAction)
			}
		})
	}
}

// TestConvertToRequestPayloadNonV30KeepsAction 锚定现状：
// 非 v30 模型不做 req_key 转换，也不回填 action。
func TestConvertToRequestPayloadNonV30KeepsAction(t *testing.T) {
	a := &TaskAdaptor{}
	info := newJimengRelayInfo("jimeng_v21")
	req := relaycommon.TaskSubmitReq{
		Prompt: "p",
		Model:  "jimeng_v21",
		Images: []string{"https://x/a.png", "https://x/b.png", "https://x/c.png"},
	}

	payload, err := a.convertToRequestPayload(&req, info)
	if err != nil {
		t.Fatalf("convertToRequestPayload() error = %v", err)
	}
	if payload.ReqKey != "jimeng_v21" {
		t.Errorf("payload.ReqKey = %q, want jimeng_v21 (unchanged)", payload.ReqKey)
	}
	if info.Action != constant.TaskActionGenerate {
		t.Errorf("info.Action = %q, want generate (default, unchanged)", info.Action)
	}
}
