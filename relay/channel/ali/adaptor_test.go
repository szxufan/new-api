package ali

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/assert"
)

func TestAdaptor_GetRequestURL_Rerank(t *testing.T) {
	adaptor := &Adaptor{}

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://dashscope.aliyuncs.com",
		},
		RelayMode:   constant.RelayModeRerank,
		RelayFormat: types.RelayFormatRerank,
	}

	url, err := adaptor.GetRequestURL(info)
	assert.NoError(t, err)
	assert.Equal(t, "https://dashscope.aliyuncs.com/compatible-api/v1/reranks", url)
}

func TestAdaptor_GetRequestURL_Rerank_CustomBaseUrl(t *testing.T) {
	adaptor := &Adaptor{}

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://dashscope-intl.aliyuncs.com",
		},
		RelayMode:   constant.RelayModeRerank,
		RelayFormat: types.RelayFormatRerank,
	}

	url, err := adaptor.GetRequestURL(info)
	assert.NoError(t, err)
	assert.Equal(t, "https://dashscope-intl.aliyuncs.com/compatible-api/v1/reranks", url)
}

func TestAdaptor_ConvertRerankRequest(t *testing.T) {
	adaptor := &Adaptor{}

	topN := 5
	returnDocs := true
	request := dto.RerankRequest{
		Model:           "qwen3-rerank",
		Query:           "什么是重排序模型",
		Documents:       []any{"文档1", "文档2", "文档3"},
		TopN:            &topN,
		ReturnDocuments: &returnDocs,
	}

	converted, err := adaptor.ConvertRerankRequest(nil, constant.RelayModeRerank, request)
	assert.NoError(t, err)
	assert.NotNil(t, converted)

	convertedReq, ok := converted.(dto.RerankRequest)
	assert.True(t, ok, "converted request should be dto.RerankRequest")
	assert.Equal(t, "qwen3-rerank", convertedReq.Model)
	assert.Equal(t, "什么是重排序模型", convertedReq.Query)
	assert.Len(t, convertedReq.Documents, 3)
	assert.Equal(t, &topN, convertedReq.TopN)
	assert.Equal(t, &returnDocs, convertedReq.ReturnDocuments)
}

func TestAdaptor_ConvertRerankRequest_NilOptionalFields(t *testing.T) {
	adaptor := &Adaptor{}

	request := dto.RerankRequest{
		Model:     "qwen3-rerank",
		Query:     "test query",
		Documents: []any{"doc1", "doc2"},
	}

	converted, err := adaptor.ConvertRerankRequest(nil, constant.RelayModeRerank, request)
	assert.NoError(t, err)
	assert.NotNil(t, converted)

	convertedReq, ok := converted.(dto.RerankRequest)
	assert.True(t, ok)
	assert.Nil(t, convertedReq.TopN)
	assert.Nil(t, convertedReq.ReturnDocuments)
}
