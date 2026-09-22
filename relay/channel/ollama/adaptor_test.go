package ollama

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newConversionTestInfo(urlPath string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		RelayFormat:            types.RelayFormatOpenAI,
		RequestURLPath:         urlPath,
		RequestConversionChain: []types.RelayFormat{types.RelayFormatOpenAI},
	}
}

func newConversionTestContext(urlPath string) *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, urlPath, nil)
	return c
}

func TestConvertOpenAIRequest_AppendsOllamaConversion(t *testing.T) {
	c := newConversionTestContext("/v1/chat/completions")
	info := newConversionTestInfo("/v1/chat/completions")
	request := &dto.GeneralOpenAIRequest{
		Model:    "pro:latest",
		Messages: []dto.Message{{Role: "user", Content: "hi"}},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(c, info, request)
	require.Nil(t, err)
	require.NotNil(t, converted)
	_, isChat := converted.(*OllamaChatRequest)
	assert.True(t, isChat, "chat endpoint should convert to OllamaChatRequest")
	assert.Equal(t, []types.RelayFormat{types.RelayFormatOpenAI, types.RelayFormatOllama}, info.RequestConversionChain)
}

func TestConvertOpenAIRequest_GenerateMode_AppendsOllamaConversion(t *testing.T) {
	c := newConversionTestContext("/v1/completions")
	info := newConversionTestInfo("/v1/completions")
	request := &dto.GeneralOpenAIRequest{Model: "pro:latest", Prompt: "hi"}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(c, info, request)
	require.Nil(t, err)
	require.NotNil(t, converted)
	_, isGenerate := converted.(*OllamaGenerateRequest)
	assert.True(t, isGenerate, "completions endpoint should convert to OllamaGenerateRequest")
	assert.Equal(t, []types.RelayFormat{types.RelayFormatOpenAI, types.RelayFormatOllama}, info.RequestConversionChain)
}

func TestConvertOpenAIRequest_NoAppendOnNil(t *testing.T) {
	c := newConversionTestContext("/v1/chat/completions")
	info := newConversionTestInfo("/v1/chat/completions")

	_, err := (&Adaptor{}).ConvertOpenAIRequest(c, info, nil)
	assert.NotNil(t, err)
	assert.Equal(t, []types.RelayFormat{types.RelayFormatOpenAI}, info.RequestConversionChain)
}

func TestConvertEmbeddingRequest_AppendsOllamaConversion(t *testing.T) {
	c := newConversionTestContext("/v1/embeddings")
	info := &relaycommon.RelayInfo{
		RelayFormat:            types.RelayFormatEmbedding,
		RequestURLPath:         "/v1/embeddings",
		RequestConversionChain: []types.RelayFormat{types.RelayFormatEmbedding},
	}
	request := dto.EmbeddingRequest{Model: "nomic-embed-text", Input: "hello"}

	_, err := (&Adaptor{}).ConvertEmbeddingRequest(c, info, request)
	require.Nil(t, err)
	assert.Equal(t, []types.RelayFormat{types.RelayFormatEmbedding, types.RelayFormatOllama}, info.RequestConversionChain)
}
