package ali

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// 上下文键：保存客户端请求的原始 OpenAI response_format，供响应阶段使用
const contextKeyAliAudioResponseFormat = "ali_audio_response_format"

// ----- 模型路由（按模型家族选择上游端点与请求格式） -----

// Qwen-TTS 系列走 multimodal-generation 接口
func isAliQwenTTSModel(modelName string) bool {
	name := strings.ToLower(modelName)
	return strings.HasPrefix(name, "qwen3-tts") || strings.HasPrefix(name, "qwen-tts")
}

// CosyVoice / Qwen-Audio-TTS 走 SpeechSynthesizer 接口
func isAliSpeechSynthesizerModel(modelName string) bool {
	name := strings.ToLower(modelName)
	return strings.Contains(name, "cosyvoice") || strings.Contains(name, "qwen-audio-3.0-tts")
}

// Qwen3-ASR 支持 OpenAI 兼容模式
func isAliASRCompatibleModel(modelName string) bool {
	return strings.Contains(strings.ToLower(modelName), "qwen3-asr")
}

// 其余同步 ASR 模型（fun-asr、qwen-audio-3.0-asr）走 multimodal-generation 接口
func isAliASRMultimodalModel(modelName string) bool {
	name := strings.ToLower(modelName)
	return strings.Contains(name, "fun-asr") || strings.Contains(name, "qwen-audio-3.0-asr")
}

func aliTTSSupportsModel(modelName string) bool {
	return isAliQwenTTSModel(modelName) || isAliSpeechSynthesizerModel(modelName)
}

func aliASRSupportsModel(modelName string) bool {
	return isAliASRCompatibleModel(modelName) || isAliASRMultimodalModel(modelName)
}

// ----- 请求结构 -----

type aliTTSInput struct {
	Text         string `json:"text"`
	Voice        string `json:"voice,omitempty"`
	Format       string `json:"format,omitempty"`
	SampleRate   int    `json:"sample_rate,omitempty"`
	Instruction  string `json:"instruction,omitempty"`
	Instructions string `json:"instructions,omitempty"`
	LanguageType string `json:"language_type,omitempty"`
}

type aliTTSRequest struct {
	Model string      `json:"model"`
	Input aliTTSInput `json:"input"`
}

type aliASRInputAudio struct {
	Data string `json:"data"`
}

type aliASRContentPart struct {
	Type       string            `json:"type,omitempty"`
	InputAudio *aliASRInputAudio `json:"input_audio,omitempty"`
	Audio      string            `json:"audio,omitempty"`
}

type aliASRChatMessage struct {
	Role    string              `json:"role"`
	Content []aliASRContentPart `json:"content"`
}

type aliASROptions struct {
	Language string `json:"language,omitempty"`
}

// 兼容模式（qwen3-asr）请求
type aliASRChatRequest struct {
	Model      string              `json:"model"`
	Messages   []aliASRChatMessage `json:"messages"`
	AsrOptions *aliASROptions      `json:"asr_options,omitempty"`
}

// multimodal-generation（fun-asr / qwen-audio-3.0-asr）请求
type aliASRMultimodalRequest struct {
	Model    string              `json:"model"`
	Messages []aliASRChatMessage `json:"messages"`
}

func mimeByExtension(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".wav":
		return "audio/wav"
	case ".mp3":
		return "audio/mpeg"
	case ".m4a":
		return "audio/mp4"
	case ".opus":
		return "audio/opus"
	case ".flac":
		return "audio/flac"
	case ".webm":
		return "audio/webm"
	default:
		return "audio/wav"
	}
}

// mapAliTTSFormat 将 OpenAI response_format 映射为 DashScope 支持的音频格式
func mapAliTTSFormat(responseFormat string) (string, string) {
	switch strings.ToLower(responseFormat) {
	case "mp3", "opus":
		return "mp3", "audio/mpeg"
	case "pcm":
		return "pcm", "audio/pcm"
	default:
		return "wav", "audio/wav"
	}
}

// ----- 请求转换 -----

func convertAliSpeechRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	if request.Input == "" {
		return nil, errors.New("input is required")
	}
	if !aliTTSSupportsModel(info.UpstreamModelName) {
		return nil, fmt.Errorf("model %s does not support TTS on ali, supported: cosyvoice*/qwen-audio-3.0-tts*/qwen3-tts*/qwen-tts*", info.UpstreamModelName)
	}

	audioFormat, _ := mapAliTTSFormat(request.ResponseFormat)
	c.Set(contextKeyAliAudioResponseFormat, request.ResponseFormat)

	input := aliTTSInput{Text: request.Input}

	switch {
	case isAliQwenTTSModel(info.UpstreamModelName):
		input.Voice = request.Voice
		if input.Voice == "" {
			input.Voice = "Cherry"
		}
		input.Format = audioFormat
		if audioFormat == "wav" {
			input.SampleRate = 24000
		}
		input.LanguageType = "Auto"
		if request.Instructions != "" {
			input.Instructions = request.Instructions
		}
	default: // cosyvoice / qwen-audio-3.0-tts
		input.Voice = request.Voice
		if input.Voice == "" {
			input.Voice = "longanhuan_v3"
		}
		input.Format = audioFormat
		input.SampleRate = 24000
		if request.Instructions != "" {
			input.Instruction = request.Instructions
		}
	}

	aliReq := aliTTSRequest{Model: info.UpstreamModelName, Input: input}
	jsonData, err := common.Marshal(aliReq)
	if err != nil {
		return nil, fmt.Errorf("error marshalling ali tts request: %w", err)
	}
	return bytes.NewReader(jsonData), nil
}

func readAliASRFile(c *gin.Context) (string, error) {
	formData, err := common.ParseMultipartFormReusable(c)
	if err != nil {
		return "", fmt.Errorf("error parsing multipart form: %w", err)
	}

	fileHeaders := formData.File["file"]
	if len(fileHeaders) == 0 {
		return "", errors.New("file is required")
	}
	fileHeader := fileHeaders[0]

	file, err := fileHeader.Open()
	if err != nil {
		return "", fmt.Errorf("error opening audio file: %w", err)
	}
	defer file.Close()

	audioBytes, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("error reading audio file: %w", err)
	}

	mimeType := fileHeader.Header.Get("Content-Type")
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = mimeByExtension(fileHeader.Filename)
	}
	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(audioBytes)), nil
}

func convertAliTranscriptionRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	if !aliASRSupportsModel(info.UpstreamModelName) {
		return nil, fmt.Errorf("model %s does not support ASR on ali, supported: qwen3-asr-flash/fun-asr-flash/qwen-audio-3.0-asr-flash", info.UpstreamModelName)
	}

	dataURL, err := readAliASRFile(c)
	if err != nil {
		return nil, err
	}

	language := ""
	if len(request.Language) > 0 {
		var lang string
		if err := common.Unmarshal(request.Language, &lang); err == nil {
			language = lang
		}
	}

	message := aliASRChatMessage{Role: "user"}

	var aliReq any
	if isAliASRCompatibleModel(info.UpstreamModelName) {
		message.Content = []aliASRContentPart{{
			Type:       "input_audio",
			InputAudio: &aliASRInputAudio{Data: dataURL},
		}}
		req := aliASRChatRequest{
			Model:    info.UpstreamModelName,
			Messages: []aliASRChatMessage{message},
		}
		if language != "" {
			req.AsrOptions = &aliASROptions{Language: language}
		}
		aliReq = req
	} else {
		message.Content = []aliASRContentPart{{Audio: dataURL}}
		aliReq = aliASRMultimodalRequest{
			Model:    info.UpstreamModelName,
			Messages: []aliASRChatMessage{message},
		}
	}

	jsonData, err := common.Marshal(aliReq)
	if err != nil {
		return nil, fmt.Errorf("error marshalling ali asr request: %w", err)
	}
	return bytes.NewReader(jsonData), nil
}

// ----- 响应处理 -----

type aliAudioUsage struct {
	InputTokens      int `json:"input_tokens"`
	OutputTokens     int `json:"output_tokens"`
	TotalTokens      int `json:"total_tokens"`
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

func (u aliAudioUsage) toUsage() *dto.Usage {
	usage := &dto.Usage{}
	if u.PromptTokens > 0 {
		usage.PromptTokens = u.PromptTokens
	} else {
		usage.PromptTokens = u.InputTokens
	}
	if u.CompletionTokens > 0 {
		usage.CompletionTokens = u.CompletionTokens
	} else {
		usage.CompletionTokens = u.OutputTokens
	}
	usage.TotalTokens = u.TotalTokens
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	return usage
}

// aliTTSHandler 处理 TTS 响应：上游返回音频 URL（24 小时有效），服务端下载后以二进制返回。
func aliTTSHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, service.RelayErrorHandler(c.Request.Context(), resp, false)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}

	var ttsResp struct {
		Output struct {
			Audio struct {
				URL  string `json:"url"`
				Data string `json:"data"`
			} `json:"audio"`
			URL string `json:"url"`
		} `json:"output"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := common.Unmarshal(body, &ttsResp); err != nil {
		return nil, types.NewOpenAIError(fmt.Errorf("failed to parse ali tts response: %w", err), types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if ttsResp.Code != "" && ttsResp.Message != "" {
		return nil, types.NewErrorWithStatusCode(fmt.Errorf("ali tts error: %s", ttsResp.Message), types.ErrorCodeBadResponse, http.StatusBadRequest)
	}

	audioURL := ttsResp.Output.Audio.URL
	if audioURL == "" {
		audioURL = ttsResp.Output.URL
	}

	var audioData []byte
	if audioURL != "" {
		download, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, audioURL, nil)
		if err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
		}
		audioResp, err := http.DefaultClient.Do(download)
		if err != nil {
			return nil, types.NewOpenAIError(fmt.Errorf("failed to download ali tts audio: %w", err), types.ErrorCodeDoRequestFailed, http.StatusBadGateway)
		}
		defer audioResp.Body.Close()
		if audioResp.StatusCode != http.StatusOK {
			return nil, types.NewErrorWithStatusCode(fmt.Errorf("failed to download ali tts audio, status: %d", audioResp.StatusCode), types.ErrorCodeBadResponseStatusCode, http.StatusBadGateway)
		}
		audioData, err = io.ReadAll(audioResp.Body)
		if err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
		}
	} else if ttsResp.Output.Audio.Data != "" {
		decoded, err := base64.StdEncoding.DecodeString(ttsResp.Output.Audio.Data)
		if err != nil {
			return nil, types.NewOpenAIError(fmt.Errorf("failed to decode ali tts audio: %w", err), types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
		audioData = decoded
	} else {
		return nil, types.NewOpenAIError(errors.New("ali tts response missing audio"), types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	_, contentType := mapAliTTSFormat(c.GetString(contextKeyAliAudioResponseFormat))
	c.Header("Content-Type", contentType)
	c.Data(http.StatusOK, contentType, audioData)

	return buildAliTTSUsage(c, info, audioData, contentType), nil
}

// buildAliTTSUsage 按音频时长估算 tokens（每分钟 1000），与 OpenAI TTS 计费口径一致
func buildAliTTSUsage(c *gin.Context, info *relaycommon.RelayInfo, audioData []byte, contentType string) *dto.Usage {
	usage := &dto.Usage{}
	usage.PromptTokens = info.GetEstimatePromptTokens()
	usage.PromptTokensDetails.TextTokens = usage.PromptTokens

	var duration float64
	if contentType == "audio/pcm" {
		// DashScope PCM 默认 24kHz/16bit/mono
		duration = float64(len(audioData)) / 48000.0
	} else {
		ext := ".wav"
		if contentType == "audio/mpeg" {
			ext = ".mp3"
		}
		if d, err := common.GetAudioDuration(c.Request.Context(), bytes.NewReader(audioData), ext); err == nil {
			duration = d
		}
	}

	if duration > 0 {
		completionTokens := int(math.Round(math.Ceil(duration) / 60.0 * 1000))
		usage.CompletionTokens = completionTokens
		usage.CompletionTokenDetails.AudioTokens = completionTokens
	}
	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	return usage
}

// aliSTTHandler 处理 ASR 响应：
// qwen3-asr（兼容模式）返回标准 chat completion，文本在 choices[0].message.content；
// fun-asr / qwen-audio-3.0-asr（multimodal-generation）返回 output.text / output.output.sentence.text。
// 统一转换为 OpenAI 的 {"text": ...}；response_format 为 text 时返回纯文本。
func aliSTTHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo, responseFormat string) (*types.NewAPIError, *dto.Usage) {
	defer service.CloseResponseBodyGracefully(resp)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError), nil
	}

	var sttResp struct {
		Choices []struct {
			Message struct {
				Content any `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Output struct {
			Text     string `json:"text"`
			Sentence struct {
				Text string `json:"text"`
			} `json:"sentence"`
		} `json:"output"`
		Code    string         `json:"code"`
		Message string         `json:"message"`
		Usage   *aliAudioUsage `json:"usage"`
	}
	if err := common.Unmarshal(body, &sttResp); err != nil {
		return types.NewOpenAIError(fmt.Errorf("failed to parse ali asr response: %w", err), types.ErrorCodeBadResponseBody, http.StatusInternalServerError), nil
	}
	if sttResp.Code != "" && sttResp.Message != "" {
		return types.NewErrorWithStatusCode(fmt.Errorf("ali asr error: %s", sttResp.Message), types.ErrorCodeBadResponse, http.StatusBadRequest), nil
	}

	text := ""
	if len(sttResp.Choices) > 0 {
		// 兼容模式下 content 可能为字符串或分段数组
		switch v := sttResp.Choices[0].Message.Content.(type) {
		case string:
			text = v
		case []any:
			var parts []string
			for _, item := range v {
				if m, ok := item.(map[string]any); ok {
					if t, ok := m["text"].(string); ok {
						parts = append(parts, t)
					}
				}
			}
			text = strings.Join(parts, "")
		}
	}
	if text == "" {
		text = sttResp.Output.Text
	}
	if text == "" {
		text = sttResp.Output.Sentence.Text
	}
	if text == "" {
		return types.NewOpenAIError(errors.New("ali asr response missing text"), types.ErrorCodeBadResponseBody, http.StatusInternalServerError), nil
	}

	usage := &dto.Usage{}
	if sttResp.Usage != nil && sttResp.Usage.TotalTokens > 0 {
		usage = sttResp.Usage.toUsage()
	} else {
		usage.PromptTokens = info.GetEstimatePromptTokens()
		usage.TotalTokens = usage.PromptTokens
	}

	if responseFormat == "text" {
		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(text))
		return nil, usage
	}

	respBody, _ := common.Marshal(dto.AudioResponse{Text: text})
	c.Header("Content-Type", "application/json")
	c.Data(http.StatusOK, "application/json", respBody)
	return nil, usage
}
