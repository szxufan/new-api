package operation_setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetUserAgentPassthroughList(t *testing.T) {
	t.Parallel()

	// 空串 → 空列表
	g := &GeneralSetting{UserAgentPassthrough: ""}
	require.Empty(t, g.GetUserAgentPassthroughList())

	// 纯空白行 → 空列表
	g = &GeneralSetting{UserAgentPassthrough: "  \n\t\n"}
	require.Empty(t, g.GetUserAgentPassthroughList())

	// 多行 + 缩进 + 空行 → 正确拆分与 trim
	g = &GeneralSetting{UserAgentPassthrough: " codex\n  claude-cli\n\nMozilla/5.0\n"}
	require.Equal(t, []string{"codex", "claude-cli", "Mozilla/5.0"}, g.GetUserAgentPassthroughList())
}

func TestShouldPassthroughUserAgent(t *testing.T) {
	t.Parallel()

	// 名单为空恒为 false
	empty := &GeneralSetting{}
	require.False(t, empty.ShouldPassthroughUserAgent("codex-cli/1.0"))

	g := &GeneralSetting{UserAgentPassthrough: "codex-cli\nClaude CLI"}

	tests := []struct {
		name string
		ua   string
		want bool
	}{
		{name: "命中-子串", ua: "codex-cli/1.0 (linux)", want: true},
		{name: "命中-忽略大小写", ua: "MyClient CODEX-CLI/2.0", want: true},
		{name: "命中-完整等于", ua: "claude cli", want: true},
		{name: "未命中", ua: "Mozilla/5.0 (Windows NT 10.0)", want: false},
		{name: "部分片段不构成子串", ua: "codex", want: false},
		{name: "空 UA", ua: "", want: false},
		{name: "空白 UA", ua: "   ", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, g.ShouldPassthroughUserAgent(tt.ua))
		})
	}
}

func TestGetDefaultUserAgent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		configured string
		want       string
	}{
		{name: "留空-回退内置", configured: "", want: BuiltinDefaultUserAgent},
		{name: "纯空白-回退内置", configured: "   \n ", want: BuiltinDefaultUserAgent},
		{name: "配置值-去除首尾空白", configured: " my-agent/2.1 ", want: "my-agent/2.1"},
		{name: "配置值-原样返回", configured: "Go-http-client/2.0", want: "Go-http-client/2.0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := &GeneralSetting{DefaultUserAgent: tt.configured}
			require.Equal(t, tt.want, g.GetDefaultUserAgent())
		})
	}
}
