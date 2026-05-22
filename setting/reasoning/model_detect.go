package reasoning

import (
	"strings"
)

func IsDeepSeekThinkingModel(modelName string) bool {
	if modelName == "deepseek-reasoner" {
		return true
	}
	baseName := trimDeepSeekSuffix(modelName)
	if baseName == "deepseek-reasoner" {
		return true
	}
	return strings.HasPrefix(baseName, "deepseek-v4-")
}

func trimDeepSeekSuffix(modelName string) string {
	for _, s := range DeepSeekV4EffortSuffixes {
		if strings.HasSuffix(modelName, s) {
			return strings.TrimSuffix(modelName, s)
		}
	}
	return modelName
}
