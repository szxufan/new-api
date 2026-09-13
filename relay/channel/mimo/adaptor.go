package mimo

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	openai "github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// 上下文键：保存客户端请求的原始 OpenAI response_format，供响应阶段使用
const contextKeyResponseFormat = "mimo_response_format"

type Adaptor struct {
	ResponseFormat string
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info.RelayMode == relayconstant.RelayModeAudioSpeech || info.RelayMode == relayconstant.RelayModeAudioTranscription {
		// MiMo 的音频能力通过 chat/completions 承载
		return fmt.Sprintf("%s/v1/chat/completions", info.ChannelBaseUrl), nil
	}
	// 其余模式 MiMo 与 OpenAI 兼容，按原始路径透传
	return relaycommon.GetFullRequestURL(info.ChannelBaseUrl, info.RequestURLPath, info.ChannelType), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	req.Set("Authorization", "Bearer "+info.ApiKey)
	if info.RelayMode == relayconstant.RelayModeAudioSpeech || info.RelayMode == relayconstant.RelayModeAudioTranscription {
		// 音频请求已被转换为 JSON，覆盖客户端的 multipart Content-Type
		req.Set("Content-Type", "application/json")
	}
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

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	a.ResponseFormat = request.ResponseFormat
	c.Set(contextKeyResponseFormat, request.ResponseFormat)

	switch info.RelayMode {
	case relayconstant.RelayModeAudioSpeech:
		return convertSpeechRequest(info, request)
	case relayconstant.RelayModeAudioTranscription:
		return convertTranscriptionRequest(c, request)
	default:
		return nil, fmt.Errorf("unsupported relay mode: %d", info.RelayMode)
	}
}

type mimoChatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type mimoAudioParam struct {
	Format string `json:"format"`
	Voice  string `json:"voice,omitempty"`
}

type mimoTTSRequest struct {
	Model    string            `json:"model"`
	Messages []mimoChatMessage `json:"messages"`
	Audio    mimoAudioParam    `json:"audio"`
	Stream   bool              `json:"stream,omitempty"`
}

func convertSpeechRequest(info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	if request.Input == "" {
		return nil, errors.New("input is required")
	}

	voice := request.Voice
	if voice == "" {
		voice = mimoDefaultVoice
	}

	// 非流式返回完整 wav；流式返回可拼接的 pcm16(24kHz/16bit/mono)
	format := "wav"
	stream := false
	if info.IsStream {
		format = "pcm16"
		stream = true
	}

	messages := make([]mimoChatMessage, 0, 2)
	if request.Instructions != "" {
		messages = append(messages, mimoChatMessage{Role: "user", Content: request.Instructions})
	}
	messages = append(messages, mimoChatMessage{Role: "assistant", Content: request.Input})

	mimoReq := mimoTTSRequest{
		Model:    info.UpstreamModelName,
		Messages: messages,
		Audio:    mimoAudioParam{Format: format, Voice: voice},
		Stream:   stream,
	}

	jsonData, err := common.Marshal(mimoReq)
	if err != nil {
		return nil, fmt.Errorf("error marshalling mimo tts request: %w", err)
	}
	return bytes.NewReader(jsonData), nil
}

type mimoInputAudio struct {
	Data string `json:"data"`
}

type mimoContentPart struct {
	Type       string         `json:"type"`
	InputAudio mimoInputAudio `json:"input_audio"`
}

type mimoAsrOptions struct {
	Language string `json:"language,omitempty"`
}

type mimoASRRequest struct {
	Model      string            `json:"model"`
	Messages   []mimoChatMessage `json:"messages"`
	AsrOptions *mimoAsrOptions   `json:"asr_options,omitempty"`
	Stream     bool              `json:"stream,omitempty"`
}

func convertTranscriptionRequest(c *gin.Context, request dto.AudioRequest) (io.Reader, error) {
	formData, err := common.ParseMultipartFormReusable(c)
	if err != nil {
		return nil, fmt.Errorf("error parsing multipart form: %w", err)
	}

	fileHeaders := formData.File["file"]
	if len(fileHeaders) == 0 {
		return nil, errors.New("file is required")
	}
	fileHeader := fileHeaders[0]

	file, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("error opening audio file: %w", err)
	}
	defer file.Close()

	audioBytes, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("error reading audio file: %w", err)
	}
	if len(audioBytes) > 10*1024*1024 {
		return nil, errors.New("audio file too large, base64 encoded size must be under 10MB")
	}

	mimeType := fileHeader.Header.Get("Content-Type")
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = mimeByExtension(fileHeader.Filename)
	}

	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(audioBytes))

	language := "auto"
	if len(request.Language) > 0 {
		var lang string
		if err := common.Unmarshal(request.Language, &lang); err == nil && lang != "" {
			language = lang
		}
	}

	mimoReq := mimoASRRequest{
		Model: request.Model,
		Messages: []mimoChatMessage{{
			Role:    "user",
			Content: []mimoContentPart{{Type: "input_audio", InputAudio: mimoInputAudio{Data: dataURL}}},
		}},
		AsrOptions: &mimoAsrOptions{Language: language},
	}

	jsonData, err := common.Marshal(mimoReq)
	if err != nil {
		return nil, fmt.Errorf("error marshalling mimo asr request: %w", err)
	}
	return bytes.NewReader(jsonData), nil
}

func mimeByExtension(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".wav":
		return "audio/wav"
	case ".mp3":
		return "audio/mpeg"
	default:
		return "audio/wav"
	}
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	switch info.RelayMode {
	case relayconstant.RelayModeAudioSpeech:
		return speechResponseHandler(c, resp, info)
	case relayconstant.RelayModeAudioTranscription:
		sttErr, sttUsage := transcriptionResponseHandler(c, resp, info, a.ResponseFormat)
		return sttUsage, sttErr
	default:
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
