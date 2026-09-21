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

// IsMimoThinkingModel checks if the model is a mimo thinking model that supports reasoning content.
func IsMimoThinkingModel(modelName string) bool {
	switch modelName {
	case "mimo-v2.5-pro", "mimo-v2.5", "mimo-v2-pro", "mimo-v2-omni", "mimo-v2-flash":
		return true
	default:
		return false
	}
}

// IsOllamaThinkingModel checks locally deployed (ollama-style) models that emit
// thinking content, addressed by name with an optional tag ("pro", "pro:latest").
func IsOllamaThinkingModel(modelName string) bool {
	base := modelName
	if idx := strings.LastIndex(base, ":"); idx >= 0 {
		base = base[:idx]
	}
	return base == "pro"
}

// IsThinkingModel checks if the model supports reasoning content caching.
// It includes DeepSeek thinking models, mimo thinking models and ollama thinking models.
func IsThinkingModel(modelName string) bool {
	return IsDeepSeekThinkingModel(modelName) || IsMimoThinkingModel(modelName) || IsOllamaThinkingModel(modelName)
}

func trimDeepSeekSuffix(modelName string) string {
	for _, s := range DeepSeekV4EffortSuffixes {
		if strings.HasSuffix(modelName, s) {
			return strings.TrimSuffix(modelName, s)
		}
	}
	return modelName
}
