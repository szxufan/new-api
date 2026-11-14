package deepseek

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

// contextKeyTTSFormat 保存本次 TTS 请求的目标音频格式，供 DoResponse 设置 Content-Type
const contextKeyTTSFormat = "deepseek_tts_format"

// deepSeekTTSRequest 对应 DeepSeek POST /v1/tools/tts 的请求体
type deepSeekTTSRequest struct {
	Text           string `json:"text"`
	VoiceID        string `json:"voice_id,omitempty"`
	Format         string `json:"format,omitempty"`
	FilterMarkdown *bool  `json:"filter_markdown,omitempty"`
}

// deepSeekASRResponse 对应 DeepSeek POST /v1/tools/asr 的响应体
type deepSeekASRResponse struct {
	RequestId  string `json:"request_id"`
	Text       string `json:"text"`
	DurationMs int64  `json:"duration_ms"`
}

// normalizeTTSFormat 归一化并校验 DeepSeek TTS 支持的音频格式（仅 wav/pcm），
// 空值按 wav 处理
func normalizeTTSFormat(format string) (string, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = "wav"
	}
	if format != "wav" && format != "pcm" {
		return "", fmt.Errorf("response_format %q is not supported by DeepSeek TTS, only wav and pcm are available", format)
	}
	return format, nil
}

func convertTTSRequest(c *gin.Context, request dto.AudioRequest) (io.Reader, error) {
	if strings.TrimSpace(request.Input) == "" {
		return nil, errors.New("input is required for DeepSeek TTS")
	}

	format, err := normalizeTTSFormat(request.ResponseFormat)
	if err != nil {
		return nil, err
	}

	voiceID := request.Voice
	if voiceID == "" {
		voiceID = "stella"
	}

	// filter_markdown 默认开启，可通过 request.Metadata 覆盖（逃生舱模式）
	filterMarkdown := true
	ttsRequest := deepSeekTTSRequest{
		Text:           request.Input,
		VoiceID:        voiceID,
		Format:         format,
		FilterMarkdown: &filterMarkdown,
	}
	if len(request.Metadata) > 0 {
		if err := common.Unmarshal(request.Metadata, &ttsRequest); err != nil {
			return nil, fmt.Errorf("error unmarshalling metadata to deepseek tts request: %w", err)
		}
	}

	// metadata 可整体覆盖请求结构体，合并后必须重新归一化并校验 format，
	// 否则非法格式会透传上游，响应侧也会把非 pcm 值一律错标为 audio/wav
	ttsRequest.Format, err = normalizeTTSFormat(ttsRequest.Format)
	if err != nil {
		return nil, err
	}

	c.Set(contextKeyTTSFormat, ttsRequest.Format)

	jsonData, err := common.Marshal(ttsRequest)
	if err != nil {
		return nil, fmt.Errorf("error marshalling deepseek tts request: %w", err)
	}
	return bytes.NewReader(jsonData), nil
}

// allowedASRExtensions DeepSeek ASR 仅支持 16bit PCM WAV 和 Ogg Opus
var allowedASRExtensions = map[string]bool{
	".wav":  true,
	".ogg":  true,
	".opus": true,
}

func convertASRRequest(c *gin.Context, request dto.AudioRequest) (io.Reader, error) {
	if request.ResponseFormat != "" && request.ResponseFormat != "json" {
		return nil, fmt.Errorf("response_format %q is not supported by DeepSeek ASR, only json is available", request.ResponseFormat)
	}

	formData, err := common.ParseMultipartFormReusable(c)
	if err != nil {
		return nil, fmt.Errorf("error parsing multipart form: %w", err)
	}

	fileHeaders := formData.File["file"]
	if len(fileHeaders) == 0 {
		return nil, errors.New("file is required")
	}
	fileHeader := fileHeaders[0]

	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if !allowedASRExtensions[ext] {
		return nil, fmt.Errorf("audio format %q is not supported by DeepSeek ASR, only 16bit PCM WAV and Ogg Opus are available", ext)
	}

	file, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("error opening audio file: %v", err)
	}
	defer file.Close()

	// DeepSeek /v1/tools/asr 只接受 file 字段，丢弃 model/language 等其它表单字段
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)
	part, err := writer.CreateFormFile("file", fileHeader.Filename)
	if err != nil {
		return nil, errors.New("create form file failed")
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, errors.New("copy file failed")
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer failed: %w", err)
	}

	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	return &requestBody, nil
}

// audioTokensFromDuration 项目音频计费惯例：每分钟 1000 tokens
func audioTokensFromDuration(duration float64) int {
	if duration <= 0 {
		return 0
	}
	return int(math.Round(math.Ceil(duration) / 60.0 * 1000))
}

// handleDeepSeekTTSResponse 处理 /v1/tools/tts 响应：响应体即原始音频字节流（wav/pcm），
// 直接透传给客户端，与 OpenAI /v1/audio/speech 的二进制响应语义一致
func handleDeepSeekTTSResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (any, *types.NewAPIError) {
	// 响应头已写出后的失败不可恢复，不对外重试（与 OpenaiTTSHandler 一致）
	defer service.CloseResponseBodyGracefully(resp)
	usage := &dto.Usage{}
	usage.PromptTokens = info.GetEstimatePromptTokens()
	usage.TotalTokens = usage.PromptTokens

	for k, v := range resp.Header {
		if !service.ShouldCopyUpstreamHeader(c, k, v) {
			continue
		}
		c.Writer.Header().Set(k, v[0])
	}

	common.SetContextKey(c, constant.ContextKeyLocalCountTokens, true)
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.LogError(c, fmt.Sprintf("failed to read DeepSeek TTS response body: %v", err))
		c.Writer.WriteHeaderNow()
		return usage, nil
	}

	format := c.GetString(contextKeyTTSFormat)
	if format == "" {
		format = "wav"
	}
	if format == "pcm" {
		c.Writer.Header().Set("Content-Type", "audio/pcm")
	} else {
		c.Writer.Header().Set("Content-Type", "audio/wav")
	}

	c.Writer.WriteHeaderNow()
	if _, err := c.Writer.Write(bodyBytes); err != nil {
		logger.LogError(c, fmt.Sprintf("failed to write DeepSeek TTS response: %v", err))
	}

	// 计算音频时长并换算为音频 tokens
	var duration float64
	var durationErr error
	if format == "pcm" {
		// DeepSeek PCM 参数：采样率 24000 Hz，16-bit（2 bytes），单声道
		const sampleRate = 24000
		const bytesPerSample = 2
		const channels = 1
		duration = float64(len(bodyBytes)) / float64(sampleRate*bytesPerSample*channels)
	} else {
		duration, durationErr = common.GetAudioDuration(c.Request.Context(), bytes.NewReader(bodyBytes), ".wav")
	}

	usage.PromptTokensDetails.TextTokens = usage.PromptTokens
	if durationErr != nil {
		logger.LogWarn(c, fmt.Sprintf("failed to get audio duration: %v", durationErr))
		// 无法获取时长时按 body 大小粗略估算，每 KB 约 1 token
		estimatedTokens := int(math.Ceil(float64(len(bodyBytes)) / 1000.0))
		usage.CompletionTokens = estimatedTokens
		usage.CompletionTokenDetails.AudioTokens = estimatedTokens
	} else {
		usage.CompletionTokens = audioTokensFromDuration(duration)
		usage.CompletionTokenDetails.AudioTokens = usage.CompletionTokens
	}
	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens

	return usage, nil
}

// handleDeepSeekASRResponse 处理 /v1/tools/asr 响应：将 {request_id, text, duration_ms}
// 归一化为 OpenAI /v1/audio/transcriptions 的 json 格式 {"text": ...}
func handleDeepSeekASRResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (any, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}

	var asrResponse deepSeekASRResponse
	if err := common.Unmarshal(responseBody, &asrResponse); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	c.Header("Content-Type", "application/json")
	c.JSON(http.StatusOK, gin.H{"text": asrResponse.Text})

	// DeepSeek ASR 按音频时长计费：优先使用上游返回的 duration_ms；
	// 缺失时退化为本地音频时长估算（与预扣费口径一致），
	// 避免把音频时长按文本倍率计费、并与识别文本 token 叠加计费
	audioTokens := 0
	if asrResponse.DurationMs > 0 {
		audioTokens = audioTokensFromDuration(float64(asrResponse.DurationMs) / 1000.0)
	} else {
		audioTokens = info.GetEstimatePromptTokens()
	}

	usage := &dto.Usage{}
	if audioTokens > 0 {
		usage.PromptTokens = audioTokens
		usage.PromptTokensDetails.AudioTokens = audioTokens
		usage.TotalTokens = audioTokens
	}

	return usage, nil
}
