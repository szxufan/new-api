package relay

import (
	"fmt"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// WebToolsHelper 处理网页工具中继请求（/v1/tools/search、/v1/tools/fetch）。
// 计费为按次模型固定价格：预扣费由上层 Relay 完成，本函数在成功且"有实际结果"
// 时结算（对齐 DeepSeek 仅对 results 非空 / 返回正文的请求计费），无结果时退还
// 预扣费；失败时上层 Relay 的 defer 统一退款。上游响应体原样透传给客户端。
func WebToolsHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *types.NewAPIError) {
	info.InitChannelMeta(c)

	var upstreamReq dto.Request
	switch info.RelayMode {
	case relayconstant.RelayModeWebSearch:
		request, ok := info.Request.(*dto.WebSearchRequest)
		if !ok {
			return types.NewErrorWithStatusCode(fmt.Errorf("invalid request type, expected dto.WebSearchRequest, got %T", info.Request), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		}
		request, err := common.DeepCopy(request)
		if err != nil {
			return types.NewError(fmt.Errorf("failed to copy web search request: %w", err), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
		}
		upstreamReq = request
	case relayconstant.RelayModeWebFetch:
		request, ok := info.Request.(*dto.WebFetchRequest)
		if !ok {
			return types.NewErrorWithStatusCode(fmt.Errorf("invalid request type, expected dto.WebFetchRequest, got %T", info.Request), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		}
		request, err := common.DeepCopy(request)
		if err != nil {
			return types.NewError(fmt.Errorf("failed to copy web fetch request: %w", err), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
		}
		upstreamReq = request
	default:
		return types.NewErrorWithStatusCode(fmt.Errorf("unsupported web tools relay mode: %d", info.RelayMode), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	if err := helper.ModelMappedHelper(c, info, upstreamReq); err != nil {
		return types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
	}

	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)

	jsonData, err := common.Marshal(upstreamReq)
	if err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	body, size, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
	if err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	defer closer.Close()
	info.UpstreamRequestBodySize = size

	logger.LogDebug(c, "web tools request body: %s", jsonData)

	statusCodeMappingStr := c.GetString("status_code_mapping")

	resp, err := adaptor.DoRequest(c, info, body)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}
	httpResp, ok := resp.(*http.Response)
	if !ok || httpResp == nil {
		return types.NewError(fmt.Errorf("unexpected response type from upstream"), types.ErrorCodeDoRequestFailed, types.ErrOptionWithSkipRetry())
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		newAPIError = service.RelayErrorHandler(c.Request.Context(), httpResp, false)
		service.ResetStatusCode(newAPIError, statusCodeMappingStr)
		return newAPIError
	}

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}

	// 解析并按"有实际结果"判定是否计费（与 MCP 工具一致）
	isBillable := false
	switch info.RelayMode {
	case relayconstant.RelayModeWebSearch:
		var parsed dto.WebSearchResponse
		if err := common.Unmarshal(respBody, &parsed); err != nil {
			return types.NewOpenAIError(fmt.Errorf("invalid web search response: %w", err), types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
		isBillable = len(parsed.Results) > 0
	default:
		var parsed dto.WebFetchResponse
		if err := common.Unmarshal(respBody, &parsed); err != nil {
			return types.NewOpenAIError(fmt.Errorf("invalid web fetch response: %w", err), types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
		isBillable = parsed.Content != ""
	}

	usage := &dto.Usage{
		TotalTokens:      1,
		PromptTokens:     1,
		CompletionTokens: 0,
	}
	if isBillable {
		service.PostTextConsumeQuota(c, info, usage, nil)
	} else {
		// 上游对无结果请求不计费，退还预扣费
		if info.Billing != nil {
			info.Billing.Refund(c)
		}
	}

	c.Data(http.StatusOK, "application/json", respBody)
	return nil
}
