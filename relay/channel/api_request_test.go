package channel

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestProcessHeaderOverride_ChannelTestSkipsPassthroughRules(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Request.Header.Set("X-Trace-Id", "trace-123")

	info := &relaycommon.RelayInfo{
		IsChannelTest: true,
		ChannelMeta: &relaycommon.ChannelMeta{
			HeadersOverride: map[string]any{
				"*": "",
			},
		},
	}

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	require.Empty(t, headers)
}

func TestProcessHeaderOverride_ChannelTestSkipsClientHeaderPlaceholder(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Request.Header.Set("X-Trace-Id", "trace-123")

	info := &relaycommon.RelayInfo{
		IsChannelTest: true,
		ChannelMeta: &relaycommon.ChannelMeta{
			HeadersOverride: map[string]any{
				"X-Upstream-Trace": "{client_header:X-Trace-Id}",
			},
		},
	}

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	_, ok := headers["x-upstream-trace"]
	require.False(t, ok)
}

func TestProcessHeaderOverride_NonTestKeepsClientHeaderPlaceholder(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Request.Header.Set("X-Trace-Id", "trace-123")

	info := &relaycommon.RelayInfo{
		IsChannelTest: false,
		ChannelMeta: &relaycommon.ChannelMeta{
			HeadersOverride: map[string]any{
				"X-Upstream-Trace": "{client_header:X-Trace-Id}",
			},
		},
	}

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	require.Equal(t, "trace-123", headers["x-upstream-trace"])
}

func TestProcessHeaderOverride_RuntimeOverrideIsFinalHeaderMap(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	info := &relaycommon.RelayInfo{
		IsChannelTest:             false,
		UseRuntimeHeadersOverride: true,
		RuntimeHeadersOverride: map[string]any{
			"x-static":  "runtime-value",
			"x-runtime": "runtime-only",
		},
		ChannelMeta: &relaycommon.ChannelMeta{
			HeadersOverride: map[string]any{
				"X-Static": "legacy-value",
				"X-Legacy": "legacy-only",
			},
		},
	}

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	require.Equal(t, "runtime-value", headers["x-static"])
	require.Equal(t, "runtime-only", headers["x-runtime"])
	_, exists := headers["x-legacy"]
	require.False(t, exists)
}

func TestProcessHeaderOverride_PassthroughSkipsAcceptEncoding(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Request.Header.Set("X-Trace-Id", "trace-123")
	ctx.Request.Header.Set("Accept-Encoding", "gzip")

	info := &relaycommon.RelayInfo{
		IsChannelTest: false,
		ChannelMeta: &relaycommon.ChannelMeta{
			HeadersOverride: map[string]any{
				"*": "",
			},
		},
	}

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	require.Equal(t, "trace-123", headers["x-trace-id"])

	_, hasAcceptEncoding := headers["accept-encoding"]
	require.False(t, hasAcceptEncoding)
}

func TestProcessHeaderOverride_PassHeadersTemplateSetsRuntimeHeaders(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	ctx.Request.Header.Set("Originator", "Codex CLI")
	ctx.Request.Header.Set("Session_id", "sess-123")

	info := &relaycommon.RelayInfo{
		IsChannelTest: false,
		RequestHeaders: map[string]string{
			"Originator": "Codex CLI",
			"Session_id": "sess-123",
		},
		ChannelMeta: &relaycommon.ChannelMeta{
			ParamOverride: map[string]any{
				"operations": []any{
					map[string]any{
						"mode":  "pass_headers",
						"value": []any{"Originator", "Session_id", "X-Codex-Beta-Features"},
					},
				},
			},
			HeadersOverride: map[string]any{
				"X-Static": "legacy-value",
			},
		},
	}

	_, err := relaycommon.ApplyParamOverrideWithRelayInfo([]byte(`{"model":"gpt-4.1"}`), info)
	require.NoError(t, err)
	require.True(t, info.UseRuntimeHeadersOverride)
	require.Equal(t, "Codex CLI", info.RuntimeHeadersOverride["originator"])
	require.Equal(t, "sess-123", info.RuntimeHeadersOverride["session_id"])
	_, exists := info.RuntimeHeadersOverride["x-codex-beta-features"]
	require.False(t, exists)
	require.Equal(t, "legacy-value", info.RuntimeHeadersOverride["x-static"])

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	require.Equal(t, "Codex CLI", headers["originator"])
	require.Equal(t, "sess-123", headers["session_id"])
	_, exists = headers["x-codex-beta-features"]
	require.False(t, exists)

	upstreamReq := httptest.NewRequest(http.MethodPost, "https://example.com/v1/responses", nil)
	applyHeaderOverrideToRequest(upstreamReq, headers)
	require.Equal(t, "Codex CLI", upstreamReq.Header.Get("Originator"))
	require.Equal(t, "sess-123", upstreamReq.Header.Get("Session_id"))
	require.Empty(t, upstreamReq.Header.Get("X-Codex-Beta-Features"))
}

func TestApplyDefaultUserAgent(t *testing.T) {
	// 不使用 t.Parallel：本组用例会读写全局 generalSetting，需顺序执行避免互相污染。
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	info := &relaycommon.RelayInfo{IsChannelTest: false}

	setting := operation_setting.GetGeneralSetting()
	prevDefault := setting.DefaultUserAgent
	setting.DefaultUserAgent = ""
	t.Cleanup(func() { setting.DefaultUserAgent = prevDefault })

	// 未设置 UA 时写入默认值
	headers := http.Header{}
	applyDefaultUserAgent(ctx, info, &headers)
	require.Equal(t, operation_setting.BuiltinDefaultUserAgent, headers.Get("User-Agent"))

	// 适配器已显式设置 UA 时保留原值
	headers = http.Header{}
	headers.Set("User-Agent", "kling-sdk/1.0")
	applyDefaultUserAgent(ctx, info, &headers)
	require.Equal(t, "kling-sdk/1.0", headers.Get("User-Agent"))
}

func TestApplyDefaultUserAgentPassthrough(t *testing.T) {
	gin.SetMode(gin.TestMode)

	setGeneralSetting := func(list string, defaultUA string) (restore func()) {
		setting := operation_setting.GetGeneralSetting()
		prevList := setting.UserAgentPassthrough
		prevDefault := setting.DefaultUserAgent
		setting.UserAgentPassthrough = list
		setting.DefaultUserAgent = defaultUA
		return func() {
			setting.UserAgentPassthrough = prevList
			setting.DefaultUserAgent = prevDefault
		}
	}

	newCtx := func(clientUA string) *gin.Context {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		if clientUA != "" {
			ctx.Request.Header.Set("User-Agent", clientUA)
		}
		return ctx
	}

	const clientUA = "codex-cli/1.0 (linux x86_64)"

	// 命中名单 → 上游 UA 等于客户端原始 UA（默认 UA 配置不生效）
	restore := setGeneralSetting("codex\nclaude-cli", "custom-upstream/1.0")
	headers := http.Header{}
	applyDefaultUserAgent(newCtx(clientUA), &relaycommon.RelayInfo{IsChannelTest: false}, &headers)
	require.Equal(t, clientUA, headers.Get("User-Agent"))
	restore()

	// 名单未命中 → 使用配置的默认 UA
	restore = setGeneralSetting("claude-cli", "custom-upstream/1.0")
	headers = http.Header{}
	applyDefaultUserAgent(newCtx(clientUA), &relaycommon.RelayInfo{IsChannelTest: false}, &headers)
	require.Equal(t, "custom-upstream/1.0", headers.Get("User-Agent"))
	restore()

	// 命中但为渠道测试请求 → 不透传，使用配置的默认 UA
	restore = setGeneralSetting("codex", "custom-upstream/1.0")
	headers = http.Header{}
	applyDefaultUserAgent(newCtx(clientUA), &relaycommon.RelayInfo{IsChannelTest: true}, &headers)
	require.Equal(t, "custom-upstream/1.0", headers.Get("User-Agent"))
	restore()

	// 客户端无 UA → 配置的默认 UA（即使名单宽松）
	restore = setGeneralSetting("codex", "custom-upstream/1.0")
	headers = http.Header{}
	applyDefaultUserAgent(newCtx(""), &relaycommon.RelayInfo{IsChannelTest: false}, &headers)
	require.Equal(t, "custom-upstream/1.0", headers.Get("User-Agent"))
	restore()

	// 请求头已有 UA（模拟适配器显式设置）→ 保持原值，配置的默认 UA 不覆盖
	restore = setGeneralSetting("codex", "custom-upstream/1.0")
	headers = http.Header{}
	headers.Set("User-Agent", "kling-sdk/1.0")
	applyDefaultUserAgent(newCtx(clientUA), &relaycommon.RelayInfo{IsChannelTest: false}, &headers)
	require.Equal(t, "kling-sdk/1.0", headers.Get("User-Agent"))
	restore()

	// 默认 UA 留空 → 回退内置默认值
	restore = setGeneralSetting("", "")
	headers = http.Header{}
	applyDefaultUserAgent(newCtx(clientUA), &relaycommon.RelayInfo{IsChannelTest: false}, &headers)
	require.Equal(t, operation_setting.BuiltinDefaultUserAgent, headers.Get("User-Agent"))
	restore()

	// 默认 UA 仅空白 → 同样回退内置默认值
	restore = setGeneralSetting("", "   ")
	headers = http.Header{}
	applyDefaultUserAgent(newCtx(clientUA), &relaycommon.RelayInfo{IsChannelTest: false}, &headers)
	require.Equal(t, operation_setting.BuiltinDefaultUserAgent, headers.Get("User-Agent"))
	restore()
}

// TestStartPingKeepAlive_StopWaitsForGoroutineExit 验证停止函数返回后
// ping goroutine 已完全退出，不会再有任何写入（避免与后续数据写并发污染 SSE 流）。
func TestStartPingKeepAlive_StopWaitsForGoroutineExit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	stop := startPingKeepAlive(ctx, 20*time.Millisecond)
	// 等待若干次 ping 发出
	time.Sleep(150 * time.Millisecond)
	stop()

	// stop 返回后 goroutine 必须已退出：记录当前字节数，再等待多个 ping 周期，
	// 字节数不应再增长
	bodyLenAfterStop := recorder.Body.Len()
	require.Greater(t, bodyLenAfterStop, 0, "ping data should have been written before stop")
	time.Sleep(150 * time.Millisecond)
	require.Equal(t, bodyLenAfterStop, recorder.Body.Len(),
		"no ping data should be written after stop returns")
}
