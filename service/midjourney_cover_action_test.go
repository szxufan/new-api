package service

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

// variation 分支由 if/else 链改写为 tagged switch（staticcheck QF1003）。
// 这里锁定 CoverPlusActionToNormalAction 对 customId 的解析契约：
// 三种 variation 动作映射到的 Action 与 Index，以及索引解析失败的错误路径。
func TestCoverPlusActionToNormalActionVariationBranches(t *testing.T) {
	tests := []struct {
		name        string
		customId    string
		wantAction  string
		wantIndex   int
		wantErrDesc string
	}{
		{
			name:       "plain variation takes index from customId",
			customId:   "MJ::JOB::variation::3::3dbbd469-36af-4a0f-8f02-df6c579e7011",
			wantAction: constant.MjActionVariation,
			wantIndex:  3,
		},
		{
			name:       "low_variation keeps default index",
			customId:   "MJ::JOB::low_variation::3::3dbbd469-36af-4a0f-8f02-df6c579e7011",
			wantAction: constant.MjActionLowVariation,
			wantIndex:  1,
		},
		{
			name:       "high_variation keeps default index",
			customId:   "MJ::JOB::high_variation::3::3dbbd469-36af-4a0f-8f02-df6c579e7011",
			wantAction: constant.MjActionHighVariation,
			wantIndex:  1,
		},
		{
			name:       "upsample still wins over variation",
			customId:   "MJ::JOB::upsample::2::3dbbd469-36af-4a0f-8f02-df6c579e7011",
			wantAction: constant.MjActionUpscale,
			wantIndex:  2,
		},
		{
			name:        "variation with non numeric index fails",
			customId:    "MJ::JOB::variation::nope::3dbbd469-36af-4a0f-8f02-df6c579e7011",
			wantErrDesc: "index_parse_failed",
		},
		{
			name:        "unknown action fails",
			customId:    "MJ::JOB::teleport::2::3dbbd469-36af-4a0f-8f02-df6c579e7011",
			wantErrDesc: "unknown_action:MJ::JOB::teleport::2::3dbbd469-36af-4a0f-8f02-df6c579e7011",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &dto.MidjourneyRequest{CustomId: tt.customId}
			resp := CoverPlusActionToNormalAction(req)

			if tt.wantErrDesc != "" {
				require.NotNil(t, resp)
				require.Equal(t, constant.MjRequestError, resp.Code)
				require.Equal(t, tt.wantErrDesc, resp.Description)
				return
			}

			require.Nil(t, resp, "expected no error response")
			require.Equal(t, tt.wantAction, req.Action)
			require.Equal(t, tt.wantIndex, req.Index)
		})
	}
}

func TestCoverPlusActionToNormalActionRequiresCustomId(t *testing.T) {
	resp := CoverPlusActionToNormalAction(&dto.MidjourneyRequest{})
	require.NotNil(t, resp)
	require.Equal(t, constant.MjRequestError, resp.Code)
	require.Equal(t, "custom_id_is_required", resp.Description)
}
