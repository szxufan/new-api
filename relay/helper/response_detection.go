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
//   - hasToolCalls 表示响应中是否包含工具调用（有工具调用时不视为空回复）
func CheckNonStreamResponse(text string, hasToolCalls bool, info *common.RelayInfo) *types.NewAPIError {
	if info.ChannelMeta == nil || info.ChannelMeta.ChannelSetting.ResponseDetection == nil {
		return nil
	}
	detection := info.ChannelMeta.ChannelSetting.ResponseDetection
	hit, keywords := service.CheckResponseDetectionWithEmpty(text, hasToolCalls, detection)
	if !hit {
		return nil
	}
	return newDetectionHitError(keywords)
}

// newDetectionHitError 根据命中关键词构造检测命中错误
// 当 keywords 为 nil 时表示空回复命中，错误描述为 "empty response"
func newDetectionHitError(keywords []string) *types.NewAPIError {
	if keywords == nil {
		return types.NewError(
			fmt.Errorf("response detection hit: empty response"),
			types.ErrorCodeResponseDetectionHit,
		)
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
//
// 返回值：
//   - handler: 包装后的 dataHandler，需在流式 data 循环中调用
//   - finalizer: 流结束（dataChan 关闭）后调用，用于判定空回复命中（TreatEmptyAsHit 场景）
//
// 当检测配置不存在、未启用、且未开启 TreatEmptyAsHit 时，finalizer 为 nil 调用方应跳过。
func StreamDetectionWrapper(
	originalHandler func(data string, sr *StreamResult),
	info *common.RelayInfo,
) (func(data string, sr *StreamResult), func()) {
	if info.ChannelMeta == nil {
		return originalHandler, nil
	}
	detection := info.ChannelMeta.ChannelSetting.ResponseDetection
	// 仅当检测启用时才需要包装；TreatEmptyAsHit 场景下即使无关键词也需要包装以追踪工具调用与累积文本
	if detection == nil || !detection.Enabled {
		return originalHandler, nil
	}
	// 无关键词且未开启 TreatEmptyAsHit，无需检测
	if len(detection.Keywords) == 0 && !detection.TreatEmptyAsHit {
		return originalHandler, nil
	}

	var textBuilder strings.Builder
	var toolCallSeen bool

	wrapped := func(data string, sr *StreamResult) {
		// 先正常转发给客户端（不截流）
		originalHandler(data, sr)

		// 如果已经命中过，不再重复检测
		if info.DetectionHit {
			return
		}

		// 追踪工具调用（用于流结束时空回复命中判定）
		if !toolCallSeen {
			if extractStreamHasToolCalls(data, info) {
				toolCallSeen = true
			}
		}

		// 提取当前 chunk 的文本内容并累积
		chunkText := extractStreamText(data, info)
		textBuilder.WriteString(chunkText)

		// 对累积文本做关键词检测（空回复判定延迟到流结束）
		if len(detection.Keywords) > 0 {
			hit, keywords := service.CheckResponseDetectionWithEmpty(textBuilder.String(), toolCallSeen, detection)
			if hit && keywords != nil {
				info.SetDetectionHit(keywords)
				// 不调用 sr.Stop()，流继续正常完成
			}
		}
	}

	finalizer := func() {
		// 流结束时空回复命中判定：仅当 TreatEmptyAsHit 且尚未命中且累积文本 trim 后为空且无工具调用
		if info.DetectionHit {
			return
		}
		if !detection.TreatEmptyAsHit {
			return
		}
		if toolCallSeen {
			return
		}
		if len(strings.TrimSpace(textBuilder.String())) != 0 {
			return
		}
		info.SetDetectionHit(nil)
	}

	return wrapped, finalizer
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

// extractStreamHasToolCalls 从 SSE data JSON 中检测是否包含工具调用，按 RelayFormat 分发
func extractStreamHasToolCalls(data string, info *common.RelayInfo) bool {
	switch info.GetFinalRequestRelayFormat() {
	case types.RelayFormatOpenAI, types.RelayFormatOpenAIResponses:
		return extractOpenAIStreamHasToolCalls(data)
	case types.RelayFormatClaude:
		return extractClaudeStreamHasToolCalls(data)
	case types.RelayFormatGemini:
		return extractGeminiStreamHasToolCalls(data)
	default:
		return extractOpenAIStreamHasToolCalls(data)
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

// extractOpenAIStreamHasToolCalls 检测 OpenAI 流式 chunk 是否包含工具调用
func extractOpenAIStreamHasToolCalls(data string) bool {
	toolCalls := gjson.Get(data, "choices.0.delta.tool_calls")
	return toolCalls.Exists() && toolCalls.IsArray() && len(toolCalls.Array()) > 0
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

// extractClaudeStreamHasToolCalls 检测 Claude 流式 chunk 是否包含工具调用
// Claude 流式事件中工具调用相关的事件：
//   - content_block_start 且 content_block.type == "tool_use"
//   - content_block_delta 且 delta.type == "input_json_delta"
func extractClaudeStreamHasToolCalls(data string) bool {
	if blockType := gjson.Get(data, "content_block.type"); blockType.Exists() && blockType.String() == "tool_use" {
		return true
	}
	if deltaType := gjson.Get(data, "delta.type"); deltaType.Exists() && deltaType.String() == "input_json_delta" {
		return true
	}
	return false
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

// extractGeminiStreamHasToolCalls 检测 Gemini 流式 chunk 是否包含工具调用
// Gemini 的函数调用在 candidates[].content.parts[].functionCall 中
func extractGeminiStreamHasToolCalls(data string) bool {
	for _, candidate := range gjson.Get(data, "candidates").Array() {
		for _, part := range candidate.Get("content.parts").Array() {
			if part.Get("functionCall").Exists() {
				return true
			}
		}
	}
	return false
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
