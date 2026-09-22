package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
)

func newTestGinContext(userAgent string) *gin.Context {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	if userAgent != "" {
		ctx.Request.Header.Set("User-Agent", userAgent)
	}
	return ctx
}

func TestAppendUserAgentAdminInfo(t *testing.T) {
	t.Parallel()

	ctx := newTestGinContext("codex-cli/1.0")
	adminInfo := make(map[string]interface{})

	AppendUserAgentAdminInfo(ctx, adminInfo)

	require.Equal(t, "codex-cli/1.0", adminInfo["user_agent"])
}

func TestAppendUserAgentAdminInfoSkipsEmptyOrNil(t *testing.T) {
	t.Parallel()

	// UA 为空时不写入
	ctx := newTestGinContext("")
	adminInfo := make(map[string]interface{})
	AppendUserAgentAdminInfo(ctx, adminInfo)
	assert.NotContains(t, adminInfo, "user_agent")

	// nil ctx / nil adminInfo 不 panic 也不写入
	assert.NotPanics(t, func() {
		AppendUserAgentAdminInfo(nil, adminInfo)
		AppendUserAgentAdminInfo(ctx, nil)
	})

	// Request 为 nil 时不写入
	nilReqCtx := newTestGinContext("")
	nilReqCtx.Request = nil
	adminInfo2 := make(map[string]interface{})
	AppendUserAgentAdminInfo(nilReqCtx, adminInfo2)
	assert.NotContains(t, adminInfo2, "user_agent")
}

func TestGenerateTextOtherInfoIncludesUserAgent(t *testing.T) {
	t.Parallel()

	ctx := newTestGinContext("my-client/2.3")
	relayInfo := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}

	other := GenerateTextOtherInfo(ctx, relayInfo, 1.0, 1.0, 1.0, 0, 1.0, 0.0, 1.0)

	adminInfo, ok := other["admin_info"].(map[string]interface{})
	require.True(t, ok, "admin_info should exist")
	assert.Equal(t, "my-client/2.3", adminInfo["user_agent"])
}

func TestGenerateTextOtherInfoIncludesUpstreamRequestPath(t *testing.T) {
	t.Parallel()

	ctx := newTestGinContext("")
	common.SetContextKey(ctx, constant.ContextKeyUpstreamRequestPath, "/api/chat")
	relayInfo := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}

	other := GenerateTextOtherInfo(ctx, relayInfo, 1.0, 1.0, 1.0, 0, 1.0, 0.0, 1.0)
	assert.Equal(t, "/api/chat", other["upstream_request_path"])

	// 未记录出站路径时不写入该字段
	ctx2 := newTestGinContext("")
	other2 := GenerateTextOtherInfo(ctx2, relayInfo, 1.0, 1.0, 1.0, 0, 1.0, 0.0, 1.0)
	_, exists := other2["upstream_request_path"]
	assert.False(t, exists, "upstream_request_path should be omitted when not recorded")
}

func TestGenerateTextOtherInfoIncludesReasoningFillStats(t *testing.T) {
	t.Parallel()

	ctx := newTestGinContext("")
	RecordReasoningFillStats(ctx, ReasoningFillStats{Filled: 3, FromTag: 1, FromCache: 1, FromEmpty: 1})
	relayInfo := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}

	other := GenerateTextOtherInfo(ctx, relayInfo, 1.0, 1.0, 1.0, 0, 1.0, 0.0, 1.0)
	stats, ok := other["reasoning_fill"].(ReasoningFillStats)
	require.True(t, ok, "reasoning_fill should be a ReasoningFillStats")
	assert.Equal(t, 3, stats.Filled)
	assert.Equal(t, 1, stats.FromTag)
	assert.Equal(t, 1, stats.FromCache)
	assert.Equal(t, 1, stats.FromEmpty)

	// 未回填时不写入该字段
	ctx2 := newTestGinContext("")
	other2 := GenerateTextOtherInfo(ctx2, relayInfo, 1.0, 1.0, 1.0, 0, 1.0, 0.0, 1.0)
	_, exists := other2["reasoning_fill"]
	assert.False(t, exists, "reasoning_fill should be omitted when no fill happened")
}

func TestAppendRequestConversionChainFriendlyNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		chain []types.RelayFormat
		want  []string
	}{
		{
			name:  "openai only stays native",
			chain: []types.RelayFormat{types.RelayFormatOpenAI},
			want:  []string{"OpenAI Compatible"},
		},
		{
			name:  "openai to ollama",
			chain: []types.RelayFormat{types.RelayFormatOpenAI, types.RelayFormatOllama},
			want:  []string{"OpenAI Compatible", "Ollama"},
		},
		{
			name:  "claude to ollama",
			chain: []types.RelayFormat{types.RelayFormatClaude, types.RelayFormatOllama},
			want:  []string{"Claude Messages", "Ollama"},
		},
		{
			name:  "openai to dify",
			chain: []types.RelayFormat{types.RelayFormatOpenAI, types.RelayFormatDify},
			want:  []string{"OpenAI Compatible", "Dify"},
		},
		{
			name:  "openai to coze",
			chain: []types.RelayFormat{types.RelayFormatOpenAI, types.RelayFormatCoze},
			want:  []string{"OpenAI Compatible", "Coze"},
		},
		{
			name:  "unknown format falls back to raw value",
			chain: []types.RelayFormat{types.RelayFormatOpenAI, types.RelayFormat("custom")},
			want:  []string{"OpenAI Compatible", "custom"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			other := map[string]interface{}{}
			appendRequestConversionChain(&relaycommon.RelayInfo{RequestConversionChain: tt.chain}, other)
			chain, ok := other["request_conversion"].([]string)
			require.True(t, ok, "request_conversion should be a []string")
			assert.Equal(t, tt.want, chain)
		})
	}
}

func TestAppendRequestConversionChainEmptyChainOmitted(t *testing.T) {
	t.Parallel()

	other := map[string]interface{}{}
	appendRequestConversionChain(&relaycommon.RelayInfo{}, other)
	_, exists := other["request_conversion"]
	assert.False(t, exists, "empty chain should not write request_conversion")
}
