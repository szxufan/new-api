package service

import (
	"strings"

	"github.com/QuantumNous/new-api/dto"
)

// CheckResponseDetection 检查文本是否命中渠道的响应检测关键词
// 返回: 是否命中, 命中的关键词列表
func CheckResponseDetection(text string, detection *dto.ResponseDetection) (bool, []string) {
	if detection == nil || !detection.Enabled || len(detection.Keywords) == 0 {
		return false, nil
	}
	if len(text) == 0 {
		return false, nil
	}
	lowerText := strings.ToLower(text)
	var hitKeywords []string
	for _, kw := range detection.Keywords {
		if strings.Contains(lowerText, strings.ToLower(kw)) {
			hitKeywords = append(hitKeywords, kw)
		}
	}
	return len(hitKeywords) > 0, hitKeywords
}
