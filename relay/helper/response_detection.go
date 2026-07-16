package helper

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/tidwall/gjson"
)

// CheckNonStreamResponse 检查非流式响应文本是否命中检测关键词
// 命中时返回 NewAPIError（skipRetry=false，允许重试）
func CheckNonStreamResponse(text string, info *common.RelayInfo) *types.NewAPIError {
	if info.ChannelMeta == nil || info.ChannelMeta.ChannelSetting.ResponseDetection == nil {
		return nil
	}
	detection := info.ChannelMeta.ChannelSetting.ResponseDetection
	hit, keywords := service.CheckResponseDetection(text, detection)
	if !hit {
		return nil
	}
	return types.NewError(
		fmt.Errorf("response detection hit: keywords=%v", keywords),
		types.ErrorCodeResponseDetectionHit,
	)
}

// IsDetectionHitError 判断错误是否由响应检测命中触发
func IsDetectionHitError(err *types.NewAPIError) bool {
	return err != nil && err.GetErrorCode() == types.ErrorCodeResponseDetectionHit
}

// StreamDetectionWrapper 返回一个包装后的 dataHandler，在原始 dataHandler 之后插入检测逻辑
// 检测命中时仅标记 info.DetectionHit，不截流，流继续正常完成
func StreamDetectionWrapper(
	originalHandler func(data string, sr *StreamResult),
	info *common.RelayInfo,
) func(data string, sr *StreamResult) {
	if info.ChannelMeta == nil {
		return originalHandler
	}
	detection := info.ChannelMeta.ChannelSetting.ResponseDetection
	if detection == nil || !detection.Enabled || len(detection.Keywords) == 0 {
		return originalHandler
	}

	var textBuilder strings.Builder

	return func(data string, sr *StreamResult) {
		// 先正常转发给客户端（不截流）
		originalHandler(data, sr)

		// 如果已经命中过，不再重复检测
		if info.DetectionHit {
			return
		}

		// 提取当前 chunk 的文本内容并累积
		chunkText := extractStreamText(data, info)
		textBuilder.WriteString(chunkText)

		// 对累积文本做关键词检测
		hit, keywords := service.CheckResponseDetection(textBuilder.String(), detection)
		if hit {
			info.SetDetectionHit(keywords)
			// 不调用 sr.Stop()，流继续正常完成
		}
	}
}

// extractStreamText 从 SSE data JSON 中提取文本内容，按 RelayFormat 分发
func extractStreamText(data string, info *common.RelayInfo) string {
	switch info.GetFinalRequestRelayFormat() {
	case types.RelayFormatOpenAI, types.RelayFormatOpenAIResponses:
		return extractOpenAIStreamText(data)
	case types.RelayFormatClaude:
		return extractClaudeStreamText(data)
	case types.RelayFormatGemini:
		return extractGeminiStreamText(data)
	default:
		return extractOpenAIStreamText(data)
	}
}

func extractOpenAIStreamText(data string) string {
	result := gjson.Get(data, "choices.0.delta.content")
	if result.Exists() && result.Type == gjson.String {
		return result.String()
	}
	result = gjson.Get(data, "choices.0.delta.reasoning_content")
	if result.Exists() && result.Type == gjson.String {
		return result.String()
	}
	return ""
}

func extractClaudeStreamText(data string) string {
	result := gjson.Get(data, "delta.text")
	if result.Exists() && result.Type == gjson.String {
		return result.String()
	}
	result = gjson.Get(data, "delta.thinking")
	if result.Exists() && result.Type == gjson.String {
		return result.String()
	}
	return ""
}

func extractGeminiStreamText(data string) string {
	var parts []string
	for _, candidate := range gjson.Get(data, "candidates").Array() {
		for _, part := range candidate.Get("content.parts").Array() {
			if text := part.Get("text").String(); text != "" {
				parts = append(parts, text)
			}
		}
	}
	return strings.Join(parts, "")
}

// ExtractFullTextFromResponse 从各格式非流式响应中提取完整文本用于检测
func ExtractFullTextFromResponse(info *common.RelayInfo, responseBody []byte) string {
	switch info.RelayFormat {
	case types.RelayFormatOpenAI, types.RelayFormatOpenAIResponses:
		var parts []string
		for _, choice := range gjson.GetBytes(responseBody, "choices").Array() {
			if content := choice.Get("message.content").String(); content != "" {
				parts = append(parts, content)
			}
			if reasoning := choice.Get("message.reasoning_content").String(); reasoning != "" {
				parts = append(parts, reasoning)
			}
		}
		return strings.Join(parts, " ")
	case types.RelayFormatClaude:
		var parts []string
		for _, block := range gjson.GetBytes(responseBody, "content").Array() {
			if text := block.Get("text").String(); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, " ")
	case types.RelayFormatGemini:
		var parts []string
		for _, candidate := range gjson.GetBytes(responseBody, "candidates").Array() {
			for _, part := range candidate.Get("content.parts").Array() {
				if text := part.Get("text").String(); text != "" {
					parts = append(parts, text)
				}
			}
		}
		return strings.Join(parts, " ")
	default:
		return string(responseBody)
	}
}
