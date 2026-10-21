package relay

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// handleToolShimResponse shim 激活时的响应处理：
// 全量缓冲上游输出（流式也内部缓冲），从文本提取工具调用并改写为标准 tool_calls。
// 提取优先走快速路径（标签块/裸 JSON），拿不准时调用解析模型裁决。
func handleToolShimResponse(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response, state *toolShimState) (*dto.Usage, *types.NewAPIError) {
	if info.IsStream {
		return toolShimStreamHandler(c, info, resp, state)
	}
	return toolShimNonStreamHandler(c, info, resp, state)
}

// toolShimNonStreamHandler 非流式：解析上游 JSON → 提取 → 改写 → 写回
func toolShimNonStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response, state *toolShimState) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}

	var full dto.OpenAITextResponse
	if unmarshalErr := common.Unmarshal(responseBody, &full); unmarshalErr != nil || len(full.Choices) == 0 {
		// 无法按标准结构解析：原样透传，按文本估算 usage
		service.IOCopyBytesGracefully(c, resp, responseBody)
		return service.ResponseText2Usage(c, string(responseBody), info.UpstreamModelName, info.GetEstimatePromptTokens()), nil
	}

	msg := &full.Choices[0].Message

	// 上游偶发返回原生 tool_calls：直接透传
	if len(msg.ParseToolCalls()) > 0 {
		service.IOCopyBytesGracefully(c, resp, responseBody)
		return &full.Usage, nil
	}

	calls, preamble, parserUsage := resolveToolShimCalls(c, info, state, msg.StringContent())

	if len(calls) == 0 {
		// 未提取到调用：原样透传
		logger.LogInfo(c, "tool shim: no tool call extracted, passing through")
		service.IOCopyBytesGracefully(c, resp, responseBody)
		return foldUsage(&full.Usage, parserUsage), nil
	}

	logger.LogInfo(c, fmt.Sprintf("tool shim: extracted %d tool call(s): %s", len(calls), callNames(calls)))
	msg.SetStringContent(preamble)
	msg.SetToolCalls(buildToolCallResponses(calls))
	full.Choices[0].FinishReason = "tool_calls"

	newBody, marshalErr := common.Marshal(full)
	if marshalErr != nil {
		// 改写序列化失败则退回原文
		service.IOCopyBytesGracefully(c, resp, responseBody)
		return foldUsage(&full.Usage, parserUsage), nil
	}
	service.IOCopyBytesGracefully(c, resp, newBody)
	return foldUsage(&full.Usage, parserUsage), nil
}

// toolShimStreamHandler 流式：只累积不转发，结束后合成 SSE 输出。
// 已知取舍：生成期间客户端无增量（判定工具调用必须全量缓冲）。
func toolShimStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response, state *toolShimState) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	var responseId string
	var modelName string
	var created int64
	var contentBuilder strings.Builder
	var reasoningBuilder strings.Builder
	usage := &dto.Usage{}
	containStreamUsage := false
	// 上游偶发原生 tool_calls 流式增量：按 index 累积
	type nativeCall struct {
		id   string
		name string
		args strings.Builder
	}
	nativeCalls := map[int]*nativeCall{}

	helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {
		if len(data) == 0 {
			return
		}
		parsed := gjson.Parse(data)
		if responseId == "" {
			responseId = parsed.Get("id").String()
			created = parsed.Get("created").Int()
			modelName = parsed.Get("model").String()
		}
		if usageRaw := parsed.Get("usage"); usageRaw.Exists() && usageRaw.IsObject() {
			var streamUsage dto.Usage
			if err := common.Unmarshal([]byte(usageRaw.Raw), &streamUsage); err == nil && service.ValidUsage(&streamUsage) {
				usage = &streamUsage
				containStreamUsage = true
			}
		}
		delta := parsed.Get("choices.0.delta")
		contentBuilder.WriteString(delta.Get("content").String())
		reasoningBuilder.WriteString(delta.Get("reasoning_content").String())
		reasoningBuilder.WriteString(delta.Get("reasoning").String())
		for _, tc := range delta.Get("tool_calls").Array() {
			idx := int(tc.Get("index").Int())
			nc, ok := nativeCalls[idx]
			if !ok {
				nc = &nativeCall{}
				nativeCalls[idx] = nc
			}
			if id := tc.Get("id").String(); id != "" {
				nc.id = id
			}
			if name := tc.Get("function.name").String(); name != "" {
				nc.name = name
			}
			nc.args.WriteString(tc.Get("function.arguments").String())
		}
	})

	if responseId == "" {
		responseId = "chatcmpl-" + common.GetRandomString(29)
	}
	if created == 0 {
		created = common.GetTimestamp()
	}
	if modelName == "" {
		modelName = info.UpstreamModelName
	}

	content := contentBuilder.String()
	var calls []toolShimCall
	var parserUsage *dto.Usage
	if len(nativeCalls) > 0 {
		// 原生 tool_calls 优先
		toolNames := toolShimToolNames(state.Tools)
		for i := 0; i < len(nativeCalls); i++ {
			nc := nativeCalls[i]
			if nc == nil {
				continue
			}
			if toolNames[nc.name] && nc.id != "" {
				calls = append(calls, toolShimCall{Name: nc.name, Arguments: normalizeArgumentsJSON(nc.args.String())})
			}
		}
	} else {
		calls, content, parserUsage = resolveToolShimCalls(c, info, state, content)
	}

	finishReason := "stop"
	if len(calls) > 0 {
		finishReason = "tool_calls"
	} else {
		logger.LogInfo(c, "tool shim: no tool call extracted from stream, passing through as content")
	}
	reasoning := reasoningBuilder.String()

	// 合成 SSE 序列
	helper.SetEventStreamHeaders(c)
	helper.ObjectData(c, helper.GenerateStartEmptyResponse(responseId, created, modelName, nil))
	if reasoning != "" {
		sendToolShimDeltaChunk(c, responseId, created, modelName, func(delta *dto.ChatCompletionsStreamResponseChoiceDelta) {
			delta.SetReasoningContent(reasoning)
		})
	}
	if content != "" {
		sendToolShimDeltaChunk(c, responseId, created, modelName, func(delta *dto.ChatCompletionsStreamResponseChoiceDelta) {
			delta.SetContentString(content)
		})
	}
	for i, call := range calls {
		index := i
		callCopy := call
		sendToolShimDeltaChunk(c, responseId, created, modelName, func(delta *dto.ChatCompletionsStreamResponseChoiceDelta) {
			delta.ToolCalls = []dto.ToolCallResponse{{
				Index:    &index,
				ID:       newToolCallId(),
				Type:     "function",
				Function: dto.FunctionResponse{Name: callCopy.Name, Arguments: callCopy.Arguments},
			}}
		})
	}
	helper.ObjectData(c, helper.GenerateStopResponse(responseId, created, modelName, finishReason))

	if !containStreamUsage {
		usage = service.ResponseText2Usage(c, content, modelName, info.GetEstimatePromptTokens())
	}
	usage = foldUsage(usage, parserUsage)
	if info.ShouldIncludeUsage {
		_ = helper.SendFinalUsageResponse(c, info, responseId, created, modelName, usage)
	}
	helper.Done(c)
	return usage, nil
}

// resolveToolShimCalls 提取决策：快速路径 → 需要时解析模型裁决。
// 返回（调用列表, 前导文本/透传正文, 解析器分支 usage）。
func resolveToolShimCalls(c *gin.Context, info *relaycommon.RelayInfo, state *toolShimState, content string) ([]toolShimCall, string, *dto.Usage) {
	calls, preamble, needParser := extractToolCallsFast(content, state.Tools)
	if !needParser {
		return calls, preamble, nil
	}
	logger.LogInfo(c, "tool shim: fast path inconclusive, invoking parser model")
	parserCalls, parserUsage, parserErr := callParserModel(c, info, state, content)
	if parserErr != nil {
		logger.LogWarn(c, "tool shim: parser model failed, falling back to fast path result: "+parserErr.Error())
		return calls, preamble, parserUsage
	}
	if len(parserCalls) == 0 {
		logger.LogInfo(c, "tool shim: parser model reported no tool call")
		return nil, "", parserUsage
	}
	return parserCalls, "", parserUsage
}

// buildToolCallResponses 把提取结果转为标准响应结构
func buildToolCallResponses(calls []toolShimCall) []dto.ToolCallResponse {
	out := make([]dto.ToolCallResponse, 0, len(calls))
	for _, call := range calls {
		out = append(out, dto.ToolCallResponse{
			ID:       newToolCallId(),
			Type:     "function",
			Function: dto.FunctionResponse{Name: call.Name, Arguments: call.Arguments},
		})
	}
	return out
}

func newToolCallId() string {
	return fmt.Sprintf("call_%s", common.GetRandomString(24))
}

func callNames(calls []toolShimCall) string {
	names := make([]string, 0, len(calls))
	for _, call := range calls {
		names = append(names, call.Name)
	}
	return strings.Join(names, ", ")
}

// foldUsage 把解析器分支 usage 并入主 usage（随主请求统一结算，不二次计费）
func foldUsage(main *dto.Usage, branch *dto.Usage) *dto.Usage {
	if main == nil {
		return branch
	}
	if branch == nil {
		return main
	}
	main.PromptTokens += branch.PromptTokens
	main.CompletionTokens += branch.CompletionTokens
	main.TotalTokens += branch.TotalTokens
	return main
}

// sendToolShimDeltaChunk 发送一个自定义 delta 的流式块
func sendToolShimDeltaChunk(c *gin.Context, id string, created int64, model string, mutate func(delta *dto.ChatCompletionsStreamResponseChoiceDelta)) {
	delta := dto.ChatCompletionsStreamResponseChoiceDelta{}
	mutate(&delta)
	helper.ObjectData(c, &dto.ChatCompletionsStreamResponse{
		Id:      id,
		Object:  "chat.completion.chunk",
		Created: created,
		Model:   model,
		Choices: []dto.ChatCompletionsStreamResponseChoice{{Delta: delta}},
	})
}
