package helper

import (
	"testing"

	relayconstant "github.com/QuantumNous/new-api/relay/constant"

	"github.com/stretchr/testify/require"
)

// TestGetAndValidateTextRequest_DeveloperRoleMappedToSystem 验证 OpenAI 兼容接口
// 中客户端传入的 developer 角色在入口处被统一映射为 system
func TestGetAndValidateTextRequest_DeveloperRoleMappedToSystem(t *testing.T) {
	body := `{"model":"deepseek-chat","messages":[{"role":"developer","content":"You are a helpful assistant"},{"role":"user","content":"hi"}]}`
	request, err := GetAndValidateTextRequest(
		newWebToolsTestContext("/v1/chat/completions", body),
		relayconstant.RelayModeChatCompletions)
	require.NoError(t, err)
	require.Len(t, request.Messages, 2)
	require.Equal(t, "system", request.Messages[0].Role)
	require.Equal(t, "user", request.Messages[1].Role)
}

// TestGetAndValidateTextRequest_MixedSystemAndDeveloper 验证同时传 system 和 developer 时
// 相邻系统消息被合并为一条，内容不丢失
func TestGetAndValidateTextRequest_MixedSystemAndDeveloper(t *testing.T) {
	body := `{"model":"deepseek-chat","messages":[{"role":"system","content":"base"},{"role":"developer","content":"extra"},{"role":"user","content":"hi"}]}`
	request, err := GetAndValidateTextRequest(
		newWebToolsTestContext("/v1/chat/completions", body),
		relayconstant.RelayModeChatCompletions)
	require.NoError(t, err)
	require.Len(t, request.Messages, 2)
	require.Equal(t, "system", request.Messages[0].Role)
	require.Equal(t, "base\nextra", request.Messages[0].Content)
	require.Equal(t, "user", request.Messages[1].Role)
}
