package common_handler

import (
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// SystemOneHandler 处理 System One 决策模型响应。
// 上游 body 原样回写客户端（保留 request_id / latency_ms 等提供商差异字段），
// 仅解析 usage 用于计费：input_tokens 计为 PromptTokens，output_tokens（Ollama
// 返回的内部评分 token）计为 CompletionTokens；input_tokens 缺失时回退到预扣费估算值。
func SystemOneHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	service.CloseResponseBodyGracefully(resp)
	logger.LogDebug(c, "systemone response body: %s", responseBody)

	var systemOneResp dto.SystemOneResponse
	if err := common.Unmarshal(responseBody, &systemOneResp); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	promptTokens := systemOneResp.Usage.InputTokens
	if promptTokens == 0 {
		promptTokens = info.GetEstimatePromptTokens()
	}
	usage := &dto.Usage{
		PromptTokens:     promptTokens,
		CompletionTokens: systemOneResp.Usage.OutputTokens,
		TotalTokens:      promptTokens + systemOneResp.Usage.OutputTokens,
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(http.StatusOK)
	_, _ = c.Writer.Write(responseBody)
	return usage, nil
}
