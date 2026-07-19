package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
