package service

import (
	"strings"

	"github.com/QuantumNous/new-api/dto"
)

// CheckResponseDetection 检查文本是否命中渠道的响应检测关键词
// 返回: 是否命中, 命中的关键词列表
//
// 兼容包装：不感知工具调用，hasToolCalls 视为 false。
// 空回复命中（AllowEmpty）需要调用 CheckResponseDetectionWithEmpty。
func CheckResponseDetection(text string, detection *dto.ResponseDetection) (bool, []string) {
	return CheckResponseDetectionWithEmpty(text, false, detection)
}

// CheckResponseDetectionWithEmpty 检查文本是否命中渠道的响应检测关键词，支持空回复命中。
//   - hasToolCalls 表示响应中是否包含工具调用（有工具调用时不视为空回复）
//   - 当 detection.AllowEmpty 为 true 且文本 trim 后为空且无工具调用时，返回 (true, nil)
//     （命中关键词列表为 nil 表示"空回复命中"，与关键词命中区分）
//   - 关键词检测逻辑与原 CheckResponseDetection 一致：不区分大小写的子串匹配
func CheckResponseDetectionWithEmpty(text string, hasToolCalls bool, detection *dto.ResponseDetection) (bool, []string) {
	if detection == nil || !detection.Enabled {
		return false, nil
	}

	// 空回复命中判定：trim 后内容为空且无工具调用
	if detection.AllowEmpty && !hasToolCalls && len(strings.TrimSpace(text)) == 0 {
		return true, nil
	}

	// 关键词检测：无关键词时直接未命中（空回复命中已在上方处理）
	if len(detection.Keywords) == 0 {
		return false, nil
	}
	if len(text) == 0 {
		return false, nil
	}
	lowerText := strings.ToLower(text)
	var hitKeywords []string
	for _, kw := range detection.Keywords {
		// 跳过空字符串关键词，避免 strings.Contains 对空字符串返回 true 导致误判
		if kw == "" {
			continue
		}
		if strings.Contains(lowerText, strings.ToLower(kw)) {
			hitKeywords = append(hitKeywords, kw)
		}
	}
	return len(hitKeywords) > 0, hitKeywords
}
