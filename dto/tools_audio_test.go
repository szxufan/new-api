package dto

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestToolsTTSRequestValidate(t *testing.T) {
	require.NoError(t, (&ToolsTTSRequest{Text: "你好"}).Validate())
	require.Error(t, (&ToolsTTSRequest{}).Validate())
	require.Error(t, (&ToolsTTSRequest{Text: "   "}).Validate())
}

func TestToolsTTSRequestTokenCountMeta(t *testing.T) {
	req := &ToolsTTSRequest{Text: "你好世界"}
	meta := req.GetTokenCountMeta()
	require.Equal(t, "你好世界", meta.CombineText)
	require.Equal(t, types.TokenTypeTextNumber, meta.TokenType)
}

func TestToolsAudioRequestSetModelName(t *testing.T) {
	tts := &ToolsTTSRequest{}
	tts.SetModelName("deepseek-tts")
	require.Equal(t, "deepseek-tts", tts.Model)

	asr := &ToolsASRRequest{}
	asr.SetModelName("deepseek-asr")
	require.Equal(t, "deepseek-asr", asr.Model)
}

// TestToolsAudioRequestModelNotSerialized 虚拟模型名仅用于路由/计费，不得序列化到上游请求体
func TestToolsAudioRequestModelNotSerialized(t *testing.T) {
	tts := &ToolsTTSRequest{Text: "你好"}
	tts.SetModelName("deepseek-tts")
	data, err := common.Marshal(tts)
	require.NoError(t, err)
	require.NotContains(t, string(data), "deepseek-tts")
	require.NotContains(t, string(data), `"model"`)
	require.Contains(t, string(data), `"text":"你好"`)
}
