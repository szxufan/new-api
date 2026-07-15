package xunfei_maas

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// Adaptor 讯飞MaaS平台适配器
// 讯飞MaaS平台使用OpenAI兼容接口，但路径前缀为 /v2/ 而非 /v1/
type Adaptor struct {
}

func (a *Adaptor) ConvertGeminiRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, req *dto.ClaudeRequest) (any, error) {
	adaptor := openai.Adaptor{}
	return adaptor.ConvertClaudeRequest(c, info, req)
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
}

// GetRequestURL 构建上游请求URL
// 讯飞MaaS平台使用 /v2/ 前缀替代 OpenAI 标准的 /v1/ 前缀
func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	baseURL := info.ChannelBaseUrl

	switch info.RelayMode {
	case relayconstant.RelayModeChatCompletions:
		return fmt.Sprintf("%s/v2/chat/completions", baseURL), nil
	case relayconstant.RelayModeCompletions:
		return fmt.Sprintf("%s/v2/completions", baseURL), nil
	case relayconstant.RelayModeEmbeddings:
		return fmt.Sprintf("%s/v2/embeddings", baseURL), nil
	case relayconstant.RelayModeImagesGenerations:
		return fmt.Sprintf("%s/v2/images/generations", baseURL), nil
	case relayconstant.RelayModeRerank:
		return fmt.Sprintf("%s/v2/rerank", baseURL), nil
	case relayconstant.RelayModeResponses:
		return fmt.Sprintf("%s/v2/responses", baseURL), nil
	case relayconstant.RelayModeResponsesCompact:
		return fmt.Sprintf("%s/v2/responses/compact", baseURL), nil
	default:
		// 对于其他模式，将请求路径中的 /v1/ 替换为 /v2/
		requestURL := info.RequestURLPath
		if strings.HasPrefix(requestURL, "/v1/") {
			requestURL = "/v2/" + strings.TrimPrefix(requestURL, "/v1/")
		}
		return relaycommon.GetFullRequestURL(baseURL, requestURL, info.ChannelType), nil
	}
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	req.Set("Authorization", "Bearer "+info.ApiKey)
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	return request, nil
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	adaptor := openai.Adaptor{}
	usage, err = adaptor.DoResponse(c, resp, info)
	return
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
