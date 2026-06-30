package types

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
)

func TestGetPreferredChannelTypesByRelayFormat_Claude(t *testing.T) {
	result := GetPreferredChannelTypesByRelayFormat(RelayFormatClaude)
	if len(result) != 1 || result[0] != constant.ChannelTypeAnthropic {
		t.Errorf("RelayFormatClaude: expected [ChannelTypeAnthropic], got %v", result)
	}
}

func TestGetPreferredChannelTypesByRelayFormat_OpenAI(t *testing.T) {
	result := GetPreferredChannelTypesByRelayFormat(RelayFormatOpenAI)
	if len(result) != 2 || result[0] != constant.ChannelTypeOpenAI || result[1] != constant.ChannelTypeAzure {
		t.Errorf("RelayFormatOpenAI: expected [ChannelTypeOpenAI, ChannelTypeAzure], got %v", result)
	}
}

func TestGetPreferredChannelTypesByRelayFormat_OpenAIResponses(t *testing.T) {
	result := GetPreferredChannelTypesByRelayFormat(RelayFormatOpenAIResponses)
	if len(result) != 2 || result[0] != constant.ChannelTypeOpenAI || result[1] != constant.ChannelTypeCodex {
		t.Errorf("RelayFormatOpenAIResponses: expected [ChannelTypeOpenAI, ChannelTypeCodex], got %v", result)
	}
}

func TestGetPreferredChannelTypesByRelayFormat_OpenAIResponsesCompaction(t *testing.T) {
	result := GetPreferredChannelTypesByRelayFormat(RelayFormatOpenAIResponsesCompaction)
	if len(result) != 2 || result[0] != constant.ChannelTypeOpenAI || result[1] != constant.ChannelTypeCodex {
		t.Errorf("RelayFormatOpenAIResponsesCompaction: expected [ChannelTypeOpenAI, ChannelTypeCodex], got %v", result)
	}
}

func TestGetPreferredChannelTypesByRelayFormat_Gemini(t *testing.T) {
	result := GetPreferredChannelTypesByRelayFormat(RelayFormatGemini)
	if len(result) != 1 || result[0] != constant.ChannelTypeGemini {
		t.Errorf("RelayFormatGemini: expected [ChannelTypeGemini], got %v", result)
	}
}

func TestGetPreferredChannelTypesByRelayFormat_OpenAIAudio(t *testing.T) {
	result := GetPreferredChannelTypesByRelayFormat(RelayFormatOpenAIAudio)
	if len(result) != 1 || result[0] != constant.ChannelTypeOpenAI {
		t.Errorf("RelayFormatOpenAIAudio: expected [ChannelTypeOpenAI], got %v", result)
	}
}

func TestGetPreferredChannelTypesByRelayFormat_OpenAIImage(t *testing.T) {
	result := GetPreferredChannelTypesByRelayFormat(RelayFormatOpenAIImage)
	if len(result) != 1 || result[0] != constant.ChannelTypeOpenAI {
		t.Errorf("RelayFormatOpenAIImage: expected [ChannelTypeOpenAI], got %v", result)
	}
}

func TestGetPreferredChannelTypesByRelayFormat_Embedding(t *testing.T) {
	result := GetPreferredChannelTypesByRelayFormat(RelayFormatEmbedding)
	if len(result) != 2 || result[0] != constant.ChannelTypeOpenAI || result[1] != constant.ChannelTypeJina {
		t.Errorf("RelayFormatEmbedding: expected [ChannelTypeOpenAI, ChannelTypeJina], got %v", result)
	}
}

func TestGetPreferredChannelTypesByRelayFormat_Rerank(t *testing.T) {
	result := GetPreferredChannelTypesByRelayFormat(RelayFormatRerank)
	if len(result) != 1 || result[0] != constant.ChannelTypeJina {
		t.Errorf("RelayFormatRerank: expected [ChannelTypeJina], got %v", result)
	}
}

func TestGetPreferredChannelTypesByRelayFormat_OpenAIRealtime(t *testing.T) {
	result := GetPreferredChannelTypesByRelayFormat(RelayFormatOpenAIRealtime)
	if len(result) != 1 || result[0] != constant.ChannelTypeOpenAI {
		t.Errorf("RelayFormatOpenAIRealtime: expected [ChannelTypeOpenAI], got %v", result)
	}
}

func TestGetPreferredChannelTypesByRelayFormat_Task_ReturnsNil(t *testing.T) {
	result := GetPreferredChannelTypesByRelayFormat(RelayFormatTask)
	if result != nil {
		t.Errorf("RelayFormatTask: expected nil, got %v", result)
	}
}

func TestGetPreferredChannelTypesByRelayFormat_MjProxy_ReturnsNil(t *testing.T) {
	result := GetPreferredChannelTypesByRelayFormat(RelayFormatMjProxy)
	if result != nil {
		t.Errorf("RelayFormatMjProxy: expected nil, got %v", result)
	}
}

func TestGetPreferredChannelTypesByRelayFormat_Unknown_ReturnsNil(t *testing.T) {
	result := GetPreferredChannelTypesByRelayFormat(RelayFormat("unknown"))
	if result != nil {
		t.Errorf("unknown RelayFormat: expected nil, got %v", result)
	}
}
