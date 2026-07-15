package xunfei_maas

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/stretchr/testify/assert"
)

func TestAdaptor_GetRequestURL_ChatCompletions(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://maas-api.cn-huabei-1.xf-yun.com",
			ChannelType:    constant.ChannelTypeXunfeiMaaS,
		},
		RelayMode: relayconstant.RelayModeChatCompletions,
	}
	url, err := adaptor.GetRequestURL(info)
	assert.NoError(t, err)
	assert.Equal(t, "https://maas-api.cn-huabei-1.xf-yun.com/v2/chat/completions", url)
}

func TestAdaptor_GetRequestURL_ChatCompletions_CustomBaseUrl(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://custom-maas.example.com",
			ChannelType:    constant.ChannelTypeXunfeiMaaS,
		},
		RelayMode: relayconstant.RelayModeChatCompletions,
	}
	url, err := adaptor.GetRequestURL(info)
	assert.NoError(t, err)
	assert.Equal(t, "https://custom-maas.example.com/v2/chat/completions", url)
}

func TestAdaptor_GetRequestURL_Embeddings(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://maas-api.cn-huabei-1.xf-yun.com",
			ChannelType:    constant.ChannelTypeXunfeiMaaS,
		},
		RelayMode: relayconstant.RelayModeEmbeddings,
	}
	url, err := adaptor.GetRequestURL(info)
	assert.NoError(t, err)
	assert.Equal(t, "https://maas-api.cn-huabei-1.xf-yun.com/v2/embeddings", url)
}

func TestAdaptor_GetRequestURL_ImagesGenerations(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://maas-api.cn-huabei-1.xf-yun.com",
			ChannelType:    constant.ChannelTypeXunfeiMaaS,
		},
		RelayMode: relayconstant.RelayModeImagesGenerations,
	}
	url, err := adaptor.GetRequestURL(info)
	assert.NoError(t, err)
	assert.Equal(t, "https://maas-api.cn-huabei-1.xf-yun.com/v2/images/generations", url)
}

func TestAdaptor_GetRequestURL_Rerank(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://maas-api.cn-huabei-1.xf-yun.com",
			ChannelType:    constant.ChannelTypeXunfeiMaaS,
		},
		RelayMode: relayconstant.RelayModeRerank,
	}
	url, err := adaptor.GetRequestURL(info)
	assert.NoError(t, err)
	assert.Equal(t, "https://maas-api.cn-huabei-1.xf-yun.com/v2/rerank", url)
}

func TestAdaptor_GetRequestURL_Completions(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://maas-api.cn-huabei-1.xf-yun.com",
			ChannelType:    constant.ChannelTypeXunfeiMaaS,
		},
		RelayMode: relayconstant.RelayModeCompletions,
	}
	url, err := adaptor.GetRequestURL(info)
	assert.NoError(t, err)
	assert.Equal(t, "https://maas-api.cn-huabei-1.xf-yun.com/v2/completions", url)
}

func TestAdaptor_GetRequestURL_DefaultFallback(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://maas-api.cn-huabei-1.xf-yun.com",
			ChannelType:    constant.ChannelTypeXunfeiMaaS,
		},
		RelayMode:      9999, // unknown mode
		RequestURLPath: "/v1/audio/speech",
	}
	url, err := adaptor.GetRequestURL(info)
	assert.NoError(t, err)
	// 未知模式应将 /v1/ 替换为 /v2/
	assert.Equal(t, "https://maas-api.cn-huabei-1.xf-yun.com/v2/audio/speech", url)
}

func TestAdaptor_GetModelList(t *testing.T) {
	adaptor := &Adaptor{}
	models := adaptor.GetModelList()
	assert.NotEmpty(t, models)
	assert.Contains(t, models, "spark-4.0-ultra")
}

func TestAdaptor_GetChannelName(t *testing.T) {
	adaptor := &Adaptor{}
	name := adaptor.GetChannelName()
	assert.Equal(t, "xunfei_maas", name)
}
