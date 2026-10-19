package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCP 网页工具（web_search / web_fetch）：
// 与图片/视频工具一致，按「分组模型池（mcp_setting.group_search/fetch_models）→
// 分组+模型 选渠道 → 按次计费（模型固定价格）→ 上游调用 → 结算」的链路执行，
// 首期支持 DeepSeek 渠道（POST /v1/tools/search、POST /v1/tools/fetch）。

// webSearchInputSchema 返回 web_search 工具的 JSON Schema
func webSearchInputSchema() json.RawMessage {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "The search query, 1-128 characters. Supports operators such as site:, -site: and quoted phrases",
			},
			"count": map[string]any{
				"type":        "integer",
				"minimum":     dto.MinWebSearchCount,
				"maximum":     dto.MaxWebSearchCount,
				"description": "Optional. Number of results to return (1-50, default 10)",
			},
			"ori_question": map[string]any{
				"type":        "string",
				"description": "Optional. The original user question that triggered this search; helps the engine understand intent",
			},
			"need_content": map[string]any{
				"type":        "boolean",
				"description": "Optional. Whether to include the full page content of each result (default false)",
			},
		},
		"required": []string{"query"},
	}
	data, _ := common.Marshal(schema)
	return json.RawMessage(data)
}

// webFetchInputSchema 返回 web_fetch 工具的 JSON Schema
func webFetchInputSchema() json.RawMessage {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{
				"type":        "string",
				"description": "The http(s) URL of the web page to fetch (max 8192 characters, no credentials). Returns the page title and markdown content",
			},
		},
		"required": []string{"url"},
	}
	data, _ := common.Marshal(schema)
	return json.RawMessage(data)
}

// invokeMCPWebTool 执行网页工具调用的完整链路（对齐 handleMCPGenerateImage）：
// 解析模型 → 预扣费 → 选渠道（优先 DeepSeek 类型）→ 上游请求 → 按次结算。
//
// scene 为场景常量（mcpSceneSearch/mcpSceneFetch）；relayMode/requestURLPath 对应上游端点；
// request 为上游请求体（同时实现 dto.Request 供计费与模型映射使用）；
// result 为上游响应解析目标（成功时作为工具结果返回）；
// isBillable 在响应解析成功后调用，返回 false 时退款不结算（对齐上游"无结果不计费"策略）。
func invokeMCPWebTool(ginCtx *gin.Context, scene string, relayMode int, requestURLPath string, request dto.Request, result any, isBillable func() bool) *mcp.CallToolResult {
	// 解析本次使用的模型：优先场景请求头指定（权限校验通过后生效），否则使用模型池分组配置
	model, requestedModel := resolveMCPModel(ginCtx, scene)
	if model == "" {
		group := common.GetContextKeyString(ginCtx, constant.ContextKeyUsingGroup)
		if group == "" {
			group = "default"
		}
		if requestedModel != "" {
			return newMCPErrorResult(fmt.Sprintf("requested model %q is not available for your groups and no %s model is configured for group: %s",
				requestedModel, scene, group))
		}
		return newMCPErrorResult(fmt.Sprintf("no %s model configured for group: %s", scene, group))
	}

	// MCP 路由未走 Distribute 中间件，需要手动设置此键
	common.SetContextKey(ginCtx, constant.ContextKeyOriginalModel, model)

	relayInfo, err := relaycommon.GenRelayInfo(ginCtx, types.RelayFormatWebTools, nil, nil)
	if err != nil {
		logger.LogError(ginCtx, fmt.Sprintf("MCP: GenRelayInfo failed: %s", err.Error()))
		return newMCPErrorResult(fmt.Sprintf("failed to build relay info: %s", err.Error()))
	}
	relayInfo.RequestURLPath = requestURLPath
	relayInfo.RelayMode = relayMode

	// 价格计算与预扣费（按次计费：为虚拟模型配置固定价格）
	priceData, err := helper.ModelPriceHelper(ginCtx, relayInfo, 0, request.GetTokenCountMeta())
	if err != nil {
		logger.LogError(ginCtx, fmt.Sprintf("MCP: ModelPriceHelper failed: %s", err.Error()))
		return newMCPErrorResult(fmt.Sprintf("pricing error: %s", err.Error()))
	}

	if !priceData.FreeModel {
		if newAPIErr := service.PreConsumeBilling(ginCtx, priceData.QuotaToPreConsume, relayInfo); newAPIErr != nil {
			logger.LogError(ginCtx, fmt.Sprintf("MCP: PreConsumeBilling failed: %s", newAPIErr.Error()))
			return newMCPErrorResult(fmt.Sprintf("insufficient quota: %s", newAPIErr.Error()))
		}
	}

	// 失败退款
	var newAPIError *types.NewAPIError
	defer func() {
		if newAPIError != nil {
			if relayInfo.Billing != nil {
				relayInfo.Billing.Refund(ginCtx)
			}
		}
	}()

	// 选择渠道（优先 DeepSeek 类型）
	channel, _, err := service.CacheGetRandomSatisfiedChannel(&service.RetryParam{
		Ctx:                   ginCtx,
		TokenGroup:            relayInfo.TokenGroup,
		ModelName:             relayInfo.OriginModelName,
		Retry:                 common.GetPointer(0),
		PreferredChannelTypes: types.GetPreferredChannelTypesByRelayFormat(types.RelayFormatWebTools),
	})
	if err != nil {
		newAPIError = types.NewError(fmt.Errorf("no available channel: %w", err), types.ErrorCodeGetChannelFailed)
		return newMCPErrorResult(fmt.Sprintf("no available channel: %s", err.Error()))
	}

	// 注入渠道元数据到 gin context
	if newAPIErr := middleware.SetupContextForSelectedChannel(ginCtx, channel, relayInfo.OriginModelName); newAPIErr != nil {
		newAPIError = newAPIErr
		return newMCPErrorResult(fmt.Sprintf("channel setup failed: %s", newAPIErr.Error()))
	}
	relayInfo.InitChannelMeta(ginCtx)

	// 获取适配器（DeepSeek 适配器按 RelayMode 返回 /v1/tools/* 端点）
	adaptor := relay.GetAdaptor(relayInfo.ApiType)
	if adaptor == nil {
		newAPIError = types.NewError(fmt.Errorf("unsupported channel type: %d", relayInfo.ChannelType), types.ErrorCodeInvalidApiType)
		return newMCPErrorResult(fmt.Sprintf("unsupported channel type: %d", relayInfo.ChannelType))
	}
	adaptor.Init(relayInfo)

	// 模型映射
	if err := helper.ModelMappedHelper(ginCtx, relayInfo, request); err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeChannelModelMappedError)
		return newMCPErrorResult(fmt.Sprintf("model mapping failed: %s", err.Error()))
	}

	jsonData, err := common.Marshal(request)
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeConvertRequestFailed)
		return newMCPErrorResult(fmt.Sprintf("request serialization failed: %s", err.Error()))
	}

	logger.LogDebug(ginCtx, "MCP web tool request body: %s", jsonData)

	resp, err := adaptor.DoRequest(ginCtx, relayInfo, bytes.NewReader(jsonData))
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeDoRequestFailed)
		return newMCPErrorResult(fmt.Sprintf("upstream request failed: %s", err.Error()))
	}

	httpResp, ok := resp.(*http.Response)
	if !ok || httpResp == nil {
		newAPIError = types.NewError(fmt.Errorf("unexpected response type"), types.ErrorCodeDoRequestFailed)
		return newMCPErrorResult("unexpected response from upstream")
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeDoRequestFailed)
		return newMCPErrorResult(fmt.Sprintf("failed to read upstream response: %s", err.Error()))
	}

	if httpResp.StatusCode != http.StatusOK {
		newAPIError = service.RelayErrorHandler(ginCtx.Request.Context(), httpResp, false)
		return newMCPErrorResult(fmt.Sprintf("upstream returned status %d: %s", httpResp.StatusCode, string(respBody)))
	}

	if err := common.Unmarshal(respBody, result); err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeBadResponseBody)
		return newMCPErrorResult(fmt.Sprintf("failed to parse response: %s", err.Error()))
	}

	// 结算计费（按次）。DeepSeek 仅对"有实际结果"的请求计费（搜索 results 非空、
	// 抓取返回正文），对齐该策略：无结果时退还预扣费
	if isBillable() {
		usage := &dto.Usage{
			TotalTokens:      1,
			PromptTokens:     1,
			CompletionTokens: 0,
		}
		service.PostTextConsumeQuota(ginCtx, relayInfo, usage, nil)
	} else {
		if relayInfo.Billing != nil {
			relayInfo.Billing.Refund(ginCtx)
		}
	}

	resultData, err := common.Marshal(result)
	if err != nil {
		return newMCPErrorResult(fmt.Sprintf("failed to marshal result: %s", err.Error()))
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(resultData)},
		},
	}
}

// handleMCPWebSearch 是 web_search 工具的 handler
func handleMCPWebSearch(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ginCtx, ok := ctx.Value(mcpGinContextKey).(*gin.Context)
	if !ok || ginCtx == nil {
		return newMCPErrorResult("internal error: failed to get request context"), nil
	}

	var args struct {
		Query       string  `json:"query"`
		Count       *int    `json:"count"`
		OriQuestion *string `json:"ori_question"`
		NeedContent *bool   `json:"need_content"`
	}
	if err := common.Unmarshal(req.Params.Arguments, &args); err != nil {
		return newMCPErrorResult(fmt.Sprintf("invalid arguments: %s", err.Error())), nil
	}
	var oriQuestion *string
	if args.OriQuestion != nil && strings.TrimSpace(*args.OriQuestion) != "" {
		oriQuestion = args.OriQuestion
	}

	searchReq := &dto.WebSearchRequest{
		Query:       strings.TrimSpace(args.Query),
		Count:       args.Count,
		OriQuestion: oriQuestion,
		NeedContent: args.NeedContent,
	}
	if err := searchReq.Validate(); err != nil {
		return newMCPErrorResult(err.Error()), nil
	}
	var searchResp dto.WebSearchResponse
	return invokeMCPWebTool(ginCtx, mcpSceneSearch, relayconstant.RelayModeWebSearch, "/v1/tools/search",
		searchReq, &searchResp, func() bool { return len(searchResp.Results) > 0 }), nil
}

// handleMCPWebFetch 是 web_fetch 工具的 handler
func handleMCPWebFetch(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ginCtx, ok := ctx.Value(mcpGinContextKey).(*gin.Context)
	if !ok || ginCtx == nil {
		return newMCPErrorResult("internal error: failed to get request context"), nil
	}

	var args struct {
		URL string `json:"url"`
	}
	if err := common.Unmarshal(req.Params.Arguments, &args); err != nil {
		return newMCPErrorResult(fmt.Sprintf("invalid arguments: %s", err.Error())), nil
	}

	fetchReq := &dto.WebFetchRequest{Url: args.URL}
	if err := fetchReq.Validate(); err != nil {
		return newMCPErrorResult(err.Error()), nil
	}
	var fetchResp dto.WebFetchResponse
	return invokeMCPWebTool(ginCtx, mcpSceneFetch, relayconstant.RelayModeWebFetch, "/v1/tools/fetch",
		fetchReq, &fetchResp, func() bool { return fetchResp.Content != "" }), nil
}
