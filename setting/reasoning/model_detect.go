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

// IsMimoThinkingModel checks if the model is a mimo chat model that supports
// reasoning content. Matching is version-agnostic (any mimo-v* chat model);
// the ASR/TTS audio models are excluded because they never emit reasoning.
func IsMimoThinkingModel(modelName string) bool {
	if !strings.HasPrefix(modelName, "mimo-v") {
		return false
	}
	return !strings.Contains(modelName, "-asr") && !strings.Contains(modelName, "-tts")
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
