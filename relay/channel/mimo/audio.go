package mimo

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// MiMo chat/completions 响应中承载音频与文本的结构
type mimoAudioMessage struct {
	Content string `json:"content"`
	Audio   struct {
		Data string `json:"data"`
	} `json:"audio"`
}

type mimoChatChoice struct {
	Message *mimoAudioMessage `json:"message"`
	Delta   *mimoAudioMessage `json:"delta"`
}

type mimoChatResponse struct {
	Choices []mimoChatChoice `json:"choices"`
	Usage   *dto.Usage       `json:"usage"`
	Error   *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

type mimoChatStreamChunk struct {
	Choices []mimoChatChoice `json:"choices"`
	Usage   *dto.Usage       `json:"usage"`
}

// speechResponseHandler 处理 TTS 响应：
// 非流式：choices[0].message.audio.data 为 base64 的 wav，解码后以二进制返回。
// 流式：SSE 中每个 chunk 的 choices[0].delta.audio.data 为 base64 的 pcm16 片段，
// 解码后以 chunked 二进制流返回（24kHz/16bit/mono，可由客户端直接拼接）。
func speechResponseHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (any, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, service.RelayErrorHandler(c.Request.Context(), resp, false)
	}

	if info.IsStream {
		return streamSpeechResponse(c, resp, info)
	}
	return nonStreamSpeechResponse(c, resp, info)
}

func nonStreamSpeechResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (any, *types.NewAPIError) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}

	var chatResp mimoChatResponse
	if err := common.Unmarshal(body, &chatResp); err != nil {
		return nil, types.NewOpenAIError(fmt.Errorf("failed to parse mimo tts response: %w", err), types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if chatResp.Error != nil && chatResp.Error.Message != "" {
		return nil, types.NewErrorWithStatusCode(fmt.Errorf("mimo tts error: %s", chatResp.Error.Message), types.ErrorCodeBadResponse, http.StatusBadRequest)
	}
	if len(chatResp.Choices) == 0 || chatResp.Choices[0].Message == nil || chatResp.Choices[0].Message.Audio.Data == "" {
		return nil, types.NewOpenAIError(fmt.Errorf("mimo tts response missing audio data"), types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	audioData, err := base64.StdEncoding.DecodeString(chatResp.Choices[0].Message.Audio.Data)
	if err != nil {
		return nil, types.NewOpenAIError(fmt.Errorf("failed to decode mimo tts audio: %w", err), types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	c.Header("Content-Type", "audio/wav")
	c.Data(http.StatusOK, "audio/wav", audioData)

	return buildTTSUsage(c, info, audioData, 0, "wav"), nil
}

func streamSpeechResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (any, *types.NewAPIError) {
	c.Header("Content-Type", "audio/pcm")
	c.Header("Transfer-Encoding", "chunked")
	c.Writer.WriteHeader(http.StatusOK)

	usage := &dto.Usage{}
	usage.PromptTokens = info.GetEstimatePromptTokens()
	usage.PromptTokensDetails.TextTokens = usage.PromptTokens

	var pcmLen int

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}

		var chunk mimoChatStreamChunk
		if err := common.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		if chunk.Usage != nil && chunk.Usage.TotalTokens > 0 {
			usage.PromptTokens = chunk.Usage.PromptTokens
			usage.CompletionTokens = chunk.Usage.CompletionTokens
			usage.TotalTokens = chunk.Usage.TotalTokens
		}
		if len(chunk.Choices) == 0 || chunk.Choices[0].Delta == nil || chunk.Choices[0].Delta.Audio.Data == "" {
			continue
		}

		pcmBytes, err := base64.StdEncoding.DecodeString(chunk.Choices[0].Delta.Audio.Data)
		if err != nil {
			continue
		}
		pcmLen += len(pcmBytes)
		if _, err := c.Writer.Write(pcmBytes); err != nil {
			return nil, types.NewErrorWithStatusCode(fmt.Errorf("failed to write audio data: %w", err), types.ErrorCodeBadResponse, http.StatusInternalServerError)
		}
		c.Writer.Flush()
	}
	if err := scanner.Err(); err != nil {
		return nil, types.NewOpenAIError(fmt.Errorf("failed to read mimo tts stream: %w", err), types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	if usage.CompletionTokens == 0 && pcmLen > 0 {
		usage = buildTTSUsage(c, info, []byte{}, pcmLen, "pcm")
	}
	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	return usage, nil
}

// buildTTSUsage 按音频时长估算 tokens（每分钟 1000），与 OpenAI TTS 计费口径一致。
// format 为 pcm 时使用 pcmLen（24kHz/16bit/mono 每秒 48000 字节），否则按容器格式解析时长。
func buildTTSUsage(c *gin.Context, info *relaycommon.RelayInfo, audioData []byte, pcmLen int, format string) *dto.Usage {
	usage := &dto.Usage{}
	usage.PromptTokens = info.GetEstimatePromptTokens()
	usage.PromptTokensDetails.TextTokens = usage.PromptTokens

	var duration float64
	if format == "pcm" {
		duration = float64(pcmLen) / 48000.0
	} else {
		if d, err := common.GetAudioDuration(c.Request.Context(), bytes.NewReader(audioData), "."+format); err == nil {
			duration = d
		} else {
			duration = float64(len(audioData)) / 48000.0
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

// transcriptionResponseHandler 处理 ASR 响应：
// MiMo 返回 chat completion，识别文本在 choices[0].message.content。
// 默认转换为 OpenAI 的 {"text": ...}；response_format 为 text 时返回纯文本。
func transcriptionResponseHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo, responseFormat string) (*types.NewAPIError, any) {
	defer service.CloseResponseBodyGracefully(resp)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError), nil
	}

	var chatResp mimoChatResponse
	if err := common.Unmarshal(body, &chatResp); err != nil {
		return types.NewOpenAIError(fmt.Errorf("failed to parse mimo asr response: %w", err), types.ErrorCodeBadResponseBody, http.StatusInternalServerError), nil
	}
	if chatResp.Error != nil && chatResp.Error.Message != "" {
		return types.NewErrorWithStatusCode(fmt.Errorf("mimo asr error: %s", chatResp.Error.Message), types.ErrorCodeBadResponse, http.StatusBadRequest), nil
	}
	if len(chatResp.Choices) == 0 || chatResp.Choices[0].Message == nil {
		return types.NewOpenAIError(fmt.Errorf("mimo asr response missing choices"), types.ErrorCodeBadResponseBody, http.StatusInternalServerError), nil
	}

	text := chatResp.Choices[0].Message.Content

	usage := &dto.Usage{}
	if chatResp.Usage != nil && chatResp.Usage.TotalTokens > 0 {
		usage.PromptTokens = chatResp.Usage.PromptTokens
		usage.CompletionTokens = chatResp.Usage.CompletionTokens
		usage.TotalTokens = chatResp.Usage.TotalTokens
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
