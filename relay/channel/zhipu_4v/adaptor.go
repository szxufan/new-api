package zhipu_4v

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	channelconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/claude"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/samber/lo"

	"github.com/gin-gonic/gin"
)

// zhipuURLSuffixes 定义智谱 API 的路径后缀层级
// 根据用户设置的 base URL 后缀，智能拼接剩余路径，避免路径重复
var zhipuURLSuffixes = []struct {
	suffix      string
	chatPath    string // chat completions 的剩余路径
	embeddingPath string
	imagePath   string
	claudePath  string
}{
	{suffix: "/chat/completions", chatPath: "", embeddingPath: "", imagePath: "", claudePath: ""},           // 已包含完整路径
	{suffix: "/v4", chatPath: "/chat/completions", embeddingPath: "/embeddings", imagePath: "/images/generations", claudePath: ""},
	{suffix: "/paas", chatPath: "/v4/chat/completions", embeddingPath: "/v4/embeddings", imagePath: "/v4/images/generations", claudePath: ""},
	{suffix: "/coding/paas", chatPath: "/v4/chat/completions", embeddingPath: "/v4/embeddings", imagePath: "/v4/images/generations", claudePath: ""},
}

// buildZhipuURL 根据用户设置的 base URL 后缀智能拼接完整请求 URL
func buildZhipuURL(baseURL string, relayMode int, relayFormat types.RelayFormat) string {
	for _, s := range zhipuURLSuffixes {
		if strings.HasSuffix(baseURL, s.suffix) {
			switch relayFormat {
			case types.RelayFormatClaude:
				if s.claudePath != "" {
					return fmt.Sprintf("%s%s", baseURL, s.claudePath)
				}
				// 未匹配到 claude 路径，回退到默认
				return fmt.Sprintf("%s/api/anthropic/v1/messages", baseURL)
			default:
				switch relayMode {
				case relayconstant.RelayModeEmbeddings:
					return fmt.Sprintf("%s%s", baseURL, s.embeddingPath)
				case relayconstant.RelayModeImagesGenerations:
					return fmt.Sprintf("%s%s", baseURL, s.imagePath)
				default:
					return fmt.Sprintf("%s%s", baseURL, s.chatPath)
				}
			}
		}
	}
	// 默认拼接完整路径
	switch relayFormat {
	case types.RelayFormatClaude:
		return fmt.Sprintf("%s/api/anthropic/v1/messages", baseURL)
	default:
		switch relayMode {
		case relayconstant.RelayModeEmbeddings:
			return fmt.Sprintf("%s/api/paas/v4/embeddings", baseURL)
		case relayconstant.RelayModeImagesGenerations:
			return fmt.Sprintf("%s/api/paas/v4/images/generations", baseURL)
		default:
			return fmt.Sprintf("%s/api/paas/v4/chat/completions", baseURL)
		}
	}
}

type Adaptor struct {
}

func (a *Adaptor) ConvertGeminiRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeminiChatRequest) (any, error) {
	//TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, req *dto.ClaudeRequest) (any, error) {
	return req, nil
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	//TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	baseURL := info.ChannelBaseUrl
	if baseURL == "" {
		baseURL = channelconstant.ChannelBaseURLs[channelconstant.ChannelTypeZhipu_v4]
	}
	specialPlan, hasSpecialPlan := channelconstant.ChannelSpecialBases[baseURL]

	switch info.RelayFormat {
	case types.RelayFormatClaude:
		if hasSpecialPlan && specialPlan.ClaudeBaseURL != "" {
			return fmt.Sprintf("%s/v1/messages", specialPlan.ClaudeBaseURL), nil
		}
	default:
		if hasSpecialPlan && specialPlan.OpenAIBaseURL != "" {
			switch info.RelayMode {
			case relayconstant.RelayModeEmbeddings:
				return fmt.Sprintf("%s/embeddings", specialPlan.OpenAIBaseURL), nil
			case relayconstant.RelayModeImagesGenerations:
				return fmt.Sprintf("%s/images/generations", specialPlan.OpenAIBaseURL), nil
			default:
				return fmt.Sprintf("%s/chat/completions", specialPlan.OpenAIBaseURL), nil
			}
		}
	}

	return buildZhipuURL(baseURL, info.RelayMode, info.RelayFormat), nil
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
	if lo.FromPtrOr(request.TopP, 0) >= 1 {
		request.TopP = lo.ToPtr(0.99)
	}
	return requestOpenAI2Zhipu(*request), nil
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, nil
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	// TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	switch info.RelayFormat {
	case types.RelayFormatClaude:
		adaptor := claude.Adaptor{}
		return adaptor.DoResponse(c, resp, info)
	default:
		if info.RelayMode == relayconstant.RelayModeImagesGenerations {
			return zhipu4vImageHandler(c, resp, info)
		}
		adaptor := openai.Adaptor{}
		return adaptor.DoResponse(c, resp, info)
	}
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
