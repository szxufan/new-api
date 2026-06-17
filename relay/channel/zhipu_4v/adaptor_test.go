package zhipu_4v

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/assert"
)

func TestBuildZhipuURL(t *testing.T) {
	tests := []struct {
		name        string
		baseURL     string
		relayMode   int
		relayFormat types.RelayFormat
		want        string
	}{
		// 默认 base URL（无路径后缀）
		{
			name:        "default base URL - chat",
			baseURL:     "https://open.bigmodel.cn",
			relayMode:   relayconstant.RelayModeChatCompletions,
			relayFormat: types.RelayFormatOpenAI,
			want:        "https://open.bigmodel.cn/api/paas/v4/chat/completions",
		},
		{
			name:        "default base URL - embeddings",
			baseURL:     "https://open.bigmodel.cn",
			relayMode:   relayconstant.RelayModeEmbeddings,
			relayFormat: types.RelayFormatOpenAI,
			want:        "https://open.bigmodel.cn/api/paas/v4/embeddings",
		},
		{
			name:        "default base URL - images",
			baseURL:     "https://open.bigmodel.cn",
			relayMode:   relayconstant.RelayModeImagesGenerations,
			relayFormat: types.RelayFormatOpenAI,
			want:        "https://open.bigmodel.cn/api/paas/v4/images/generations",
		},
		{
			name:        "default base URL - claude",
			baseURL:     "https://open.bigmodel.cn",
			relayMode:   relayconstant.RelayModeChatCompletions,
			relayFormat: types.RelayFormatClaude,
			want:        "https://open.bigmodel.cn/api/anthropic/v1/messages",
		},
		// 用户设置 /api/paas 后缀
		{
			name:        "/api/paas suffix - chat",
			baseURL:     "https://open.bigmodel.cn/api/paas",
			relayMode:   relayconstant.RelayModeChatCompletions,
			relayFormat: types.RelayFormatOpenAI,
			want:        "https://open.bigmodel.cn/api/paas/v4/chat/completions",
		},
		{
			name:        "/api/paas suffix - embeddings",
			baseURL:     "https://open.bigmodel.cn/api/paas",
			relayMode:   relayconstant.RelayModeEmbeddings,
			relayFormat: types.RelayFormatOpenAI,
			want:        "https://open.bigmodel.cn/api/paas/v4/embeddings",
		},
		// 用户设置 /api/paas/v4 后缀
		{
			name:        "/api/paas/v4 suffix - chat",
			baseURL:     "https://open.bigmodel.cn/api/paas/v4",
			relayMode:   relayconstant.RelayModeChatCompletions,
			relayFormat: types.RelayFormatOpenAI,
			want:        "https://open.bigmodel.cn/api/paas/v4/chat/completions",
		},
		{
			name:        "/api/paas/v4 suffix - embeddings",
			baseURL:     "https://open.bigmodel.cn/api/paas/v4",
			relayMode:   relayconstant.RelayModeEmbeddings,
			relayFormat: types.RelayFormatOpenAI,
			want:        "https://open.bigmodel.cn/api/paas/v4/embeddings",
		},
		// 用户设置 /api/coding/paas 后缀
		{
			name:        "/api/coding/paas suffix - chat",
			baseURL:     "https://open.bigmodel.cn/api/coding/paas",
			relayMode:   relayconstant.RelayModeChatCompletions,
			relayFormat: types.RelayFormatOpenAI,
			want:        "https://open.bigmodel.cn/api/coding/paas/v4/chat/completions",
		},
		// 用户设置 /api/coding/paas/v4 后缀
		{
			name:        "/api/coding/paas/v4 suffix - chat",
			baseURL:     "https://open.bigmodel.cn/api/coding/paas/v4",
			relayMode:   relayconstant.RelayModeChatCompletions,
			relayFormat: types.RelayFormatOpenAI,
			want:        "https://open.bigmodel.cn/api/coding/paas/v4/chat/completions",
		},
		// 用户设置完整路径（包含 /chat/completions）
		{
			name:        "full path with /chat/completions suffix",
			baseURL:     "https://open.bigmodel.cn/api/paas/v4/chat/completions",
			relayMode:   relayconstant.RelayModeChatCompletions,
			relayFormat: types.RelayFormatOpenAI,
			want:        "https://open.bigmodel.cn/api/paas/v4/chat/completions",
		},
		// 国际域名
		{
			name:        "international domain - default",
			baseURL:     "https://api.z.ai",
			relayMode:   relayconstant.RelayModeChatCompletions,
			relayFormat: types.RelayFormatOpenAI,
			want:        "https://api.z.ai/api/paas/v4/chat/completions",
		},
		{
			name:        "international domain - /api/paas suffix",
			baseURL:     "https://api.z.ai/api/paas",
			relayMode:   relayconstant.RelayModeChatCompletions,
			relayFormat: types.RelayFormatOpenAI,
			want:        "https://api.z.ai/api/paas/v4/chat/completions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildZhipuURL(tt.baseURL, tt.relayMode, tt.relayFormat)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAdaptor_GetRequestURL(t *testing.T) {
	adaptor := &Adaptor{}

	tests := []struct {
		name      string
		baseURL   string
		relayMode int
		want      string
	}{
		{
			name:      "default base URL",
			baseURL:   "https://open.bigmodel.cn",
			relayMode: relayconstant.RelayModeChatCompletions,
			want:      "https://open.bigmodel.cn/api/paas/v4/chat/completions",
		},
		{
			name:      "custom base URL with /api/paas",
			baseURL:   "https://open.bigmodel.cn/api/paas",
			relayMode: relayconstant.RelayModeChatCompletions,
			want:      "https://open.bigmodel.cn/api/paas/v4/chat/completions",
		},
		{
			name:      "custom base URL with /api/paas/v4",
			baseURL:   "https://open.bigmodel.cn/api/paas/v4",
			relayMode: relayconstant.RelayModeChatCompletions,
			want:      "https://open.bigmodel.cn/api/paas/v4/chat/completions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &relaycommon.RelayInfo{
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelBaseUrl: tt.baseURL,
				},
				RelayMode:   tt.relayMode,
				RelayFormat: types.RelayFormatOpenAI,
			}
			got, err := adaptor.GetRequestURL(info)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
