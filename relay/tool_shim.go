package relay

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
)

// 工具调用降级 shim：为原生 function calling 损坏的上游模型提供 prompt 注入式工具调用。
// 请求侧把工具定义渲染进 system prompt 并压平历史中的 tool_calls/tool 消息，
// 响应侧从文本中提取 <tool_call> 块（或调用解析模型提取），合成标准 tool_calls 返回。

const (
	toolCallOpenTag  = "<tool_call>"
	toolCallCloseTag = "</tool_call>"
	// toolCallVariantPrefix 无下划线的标签变体（<toolcall>、<toolcall/>、<toolcall_status> 等），
	// 模型格式漂移时的常见形态，出现即视为有调用意图，交由解析模型裁决
	toolCallVariantPrefix = "<toolcall"
	// parserCallTimeout 解析模型调用的独立超时，超时后降级为快速路径结果
	parserCallTimeout = 60 * time.Second
	// parserMaxTokens 解析模型输出上限（只需输出提取结果）
	parserMaxTokens = 512
	// parserMetaToolName 解析模型被强制调用的元函数名
	parserMetaToolName = "extract_tool_calls"
)

// toolShimState 一次请求内 shim 的运行态
type toolShimState struct {
	ParserModel string
	Tools       []dto.ToolCallRequest
}

// toolShimCall 提取出的工具调用（Arguments 为 JSON 对象字符串）
type toolShimCall struct {
	Name      string
	Arguments string
}

// applyToolCallShimRequest 判断是否启用工具调用降级并改写请求。
// 返回 nil 表示不启用（渠道未配置映射、模型未命中、或请求与工具无关）。
func applyToolCallShimRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) *toolShimState {
	// 虚拟模型分支（含解析器调用自身）不再叠加 shim，避免递归
	if common.GetContextKeyBool(c, constant.ContextKeyVirtualBranch) {
		return nil
	}
	mapping := info.ModelParserMapping
	if strings.TrimSpace(mapping) == "" {
		return nil
	}
	var parserMapping map[string]string
	if err := common.UnmarshalJsonStr(mapping, &parserMapping); err != nil {
		logger.LogWarn(c, fmt.Sprintf("tool shim: invalid model_parser_mapping on channel #%d: %s", info.ChannelId, err.Error()))
		return nil
	}
	parserModel, ok := parserMapping[info.OriginModelName]
	if !ok || strings.TrimSpace(parserModel) == "" {
		return nil
	}

	hasTools := len(request.Tools) > 0
	hasToolHistory := requestHasToolHistory(request)
	if !hasTools && !hasToolHistory {
		return nil
	}

	state := &toolShimState{ParserModel: parserModel, Tools: request.Tools}

	if hasTools {
		injectToolShimSystemPrompt(request)
		mergeToolShimStopSequence(request)
	}
	flattenToolShimHistory(request)

	request.Tools = nil
	request.ToolChoice = nil
	request.ParallelTooCalls = nil

	logger.LogInfo(c, fmt.Sprintf("tool shim: enabled for model %s, parser model %s", info.OriginModelName, parserModel))
	return state
}

// requestHasToolHistory 历史中是否含 tool_calls 或 tool 角色消息
// （客户端在工具轮后续回合可能不重发 tools，但历史仍需压平）
func requestHasToolHistory(request *dto.GeneralOpenAIRequest) bool {
	for i := range request.Messages {
		msg := &request.Messages[i]
		if msg.Role == "tool" && msg.ToolCallId != "" {
			return true
		}
		if msg.Role == "assistant" && len(msg.ParseToolCalls()) > 0 {
			return true
		}
	}
	return false
}

// injectToolShimSystemPrompt 把 shim 指令合并进首条 system 消息（无则插入）
func injectToolShimSystemPrompt(request *dto.GeneralOpenAIRequest) {
	shimPrompt := buildToolShimSystemPrompt(request.Tools)
	systemRole := request.GetSystemRoleName()
	for i := range request.Messages {
		msg := &request.Messages[i]
		if msg.Role == systemRole || msg.Role == "system" || msg.Role == "developer" {
			if msg.IsStringContent() {
				msg.SetStringContent(shimPrompt + "\n\n" + msg.StringContent())
			} else {
				// 非纯文本 system（含图片等）则拆开：shim 指令单独前置
				request.Messages = append(request.Messages[:i], append([]dto.Message{{Role: systemRole, Content: shimPrompt}}, request.Messages[i:]...)...)
			}
			return
		}
	}
	request.Messages = append([]dto.Message{{Role: systemRole, Content: shimPrompt}}, request.Messages...)
}

// buildToolShimSystemPrompt 渲染工具定义与调用格式指令
func buildToolShimSystemPrompt(tools []dto.ToolCallRequest) string {
	var b strings.Builder
	b.WriteString("You can call tools. When a task needs external data, real-time information, or an action, you MUST call a tool instead of guessing.\n\n")
	b.WriteString("# Available Tools\n")
	for _, tool := range tools {
		if tool.Type != "" && tool.Type != "function" {
			continue
		}
		params := "{}"
		if tool.Function.Parameters != nil {
			if bs, err := common.Marshal(tool.Function.Parameters); err == nil {
				params = string(bs)
			}
		}
		desc := tool.Function.Description
		if desc == "" {
			desc = "no description"
		}
		fmt.Fprintf(&b, "- %s(parameters: %s): %s\n", tool.Function.Name, params, desc)
	}
	b.WriteString("\n# How to call a tool\n")
	fmt.Fprintf(&b, "When you need to use a tool, respond with ONLY a tool call block in exactly this format and stop immediately:\n%s\n{\"name\": \"tool_name\", \"arguments\": {\"param\": \"value\"}}\n%s\n", toolCallOpenTag, toolCallCloseTag)
	b.WriteString("\nRules:\n- Do not write anything before or after the block. The tool result will be given to you in the next turn.\n- Never fabricate, imagine, or role-play a tool result.\n- If you can answer without any tool, answer directly without the block.")
	return b.String()
}

// flattenToolShimHistory 压平历史：assistant.tool_calls → 文本块，tool 消息 → user 消息
// 连续 tool 结果合并为一条 user 消息，避免部分上游拒绝连续同角色消息。
func flattenToolShimHistory(request *dto.GeneralOpenAIRequest) {
	out := make([]dto.Message, 0, len(request.Messages))
	pendingResults := make([]string, 0, 2)
	// tool_call_id → 函数名（用于渲染结果归属）
	toolCallNames := map[string]string{}

	flushPending := func() {
		if len(pendingResults) > 0 {
			out = append(out, dto.Message{Role: "user", Content: strings.Join(pendingResults, "\n\n")})
			pendingResults = pendingResults[:0]
		}
	}

	for i := range request.Messages {
		msg := &request.Messages[i]
		switch msg.Role {
		case "tool":
			owner := msg.ToolCallId
			if name, ok := toolCallNames[msg.ToolCallId]; ok {
				owner = name
			}
			pendingResults = append(pendingResults, fmt.Sprintf("Tool result for %s:\n%s", owner, msg.StringContent()))
		case "assistant":
			toolCalls := msg.ParseToolCalls()
			if len(toolCalls) == 0 {
				flushPending()
				out = append(out, *msg)
				continue
			}
			flushPending()
			var b strings.Builder
			b.WriteString(msg.StringContent())
			for _, tc := range toolCalls {
				if tc.ID != "" {
					toolCallNames[tc.ID] = tc.Function.Name
				}
				arguments := normalizeArgumentsJSON(tc.Function.Arguments)
				fmt.Fprintf(&b, "\n%s\n%s\n%s", toolCallOpenTag, marshalToolCallText(tc.Function.Name, arguments), toolCallCloseTag)
			}
			flattened := dto.Message{Role: "assistant", Content: b.String()}
			if msg.Name != nil {
				flattened.Name = msg.Name
			}
			out = append(out, flattened)
		default:
			flushPending()
			out = append(out, *msg)
		}
	}
	flushPending()
	request.Messages = out
}

// marshalToolCallText 生成 <tool_call> 块内的一行 JSON 文本
func marshalToolCallText(name string, arguments string) string {
	obj := map[string]any{"name": name, "arguments": json.RawMessage(arguments)}
	bs, err := common.Marshal(obj)
	if err != nil {
		return fmt.Sprintf(`{"name": %q, "arguments": {}}`, name)
	}
	return string(bs)
}

// normalizeArgumentsJSON 把参数规整为合法 JSON 对象字符串；无法解析时回退为 {}
func normalizeArgumentsJSON(arguments string) string {
	arguments = strings.TrimSpace(arguments)
	if arguments == "" {
		return "{}"
	}
	var parsed any
	if err := common.UnmarshalJsonStr(arguments, &parsed); err != nil {
		return "{}"
	}
	if _, ok := parsed.(map[string]any); !ok {
		return "{}"
	}
	return arguments
}

// mergeToolShimStopSequence 在 stop 序列中追加闭合标签，用于掐断调用块之后的垃圾续写
func mergeToolShimStopSequence(request *dto.GeneralOpenAIRequest) {
	switch stop := request.Stop.(type) {
	case nil:
		request.Stop = []string{toolCallCloseTag}
	case string:
		if stop != toolCallCloseTag {
			request.Stop = []string{stop, toolCallCloseTag}
		}
	case []string:
		for _, v := range stop {
			if v == toolCallCloseTag {
				return
			}
		}
		request.Stop = append(stop, toolCallCloseTag)
	case []any:
		strs := make([]string, 0, len(stop)+1)
		hasTag := false
		for _, v := range stop {
			s, ok := v.(string)
			if !ok {
				continue
			}
			if s == toolCallCloseTag {
				hasTag = true
			}
			strs = append(strs, s)
		}
		if !hasTag {
			strs = append(strs, toolCallCloseTag)
		}
		request.Stop = strs
	}
}

// toolShimToolNames 收集工具名集合（仅 function 类型）
func toolShimToolNames(tools []dto.ToolCallRequest) map[string]bool {
	toolNames := map[string]bool{}
	for _, tool := range tools {
		if tool.Type == "" || tool.Type == "function" {
			toolNames[tool.Function.Name] = true
		}
	}
	return toolNames
}

// extractToolCallsFast 快速路径：从文本中提取工具调用。
// 返回（提取结果, 首个调用块之前的前导文本, 是否需要解析模型裁决）。
func extractToolCallsFast(content string, tools []dto.ToolCallRequest) (calls []toolShimCall, preamble string, needParser bool) {
	toolNames := toolShimToolNames(tools)

	// 1) 完整 <tool_call>...</tool_call> 块
	blocks, starts := findToolCallBlocks(content)
	if len(blocks) > 0 {
		calls = make([]toolShimCall, 0, len(blocks))
		for _, block := range blocks {
			if call, ok := parseToolCallText(block, toolNames); ok {
				calls = append(calls, call)
			}
		}
		if len(calls) > 0 {
			preamble = strings.TrimSpace(content[:starts[0]])
			return calls, preamble, false
		}
		// 有完整块但全部解析失败（如 name 不在工具表内）→ 交给解析器判断
		return nil, "", true
	}

	// 2) 出现过开标签或其变体（有调用意图但块不完整/格式漂移）→ 解析器裁决
	if strings.Contains(content, toolCallOpenTag) || strings.Contains(content, toolCallVariantPrefix) {
		return nil, "", true
	}

	// 3) 无标签：尝试裸 JSON 候选（模型格式漂移时会直接输出 JSON 对象）
	candidates := findTaglessToolCallCandidates(content, toolNames)
	switch len(candidates) {
	case 0:
		return nil, "", false
	case 1:
		return []toolShimCall{candidates[0].call}, candidates[0].preamble, false
	default:
		// 多个候选无法确定 → 解析器裁决
		return nil, "", true
	}
}

// findToolCallBlocks 返回所有完整标签块内容及其起始下标
func findToolCallBlocks(content string) (blocks []string, starts []int) {
	searchFrom := 0
	for {
		open := strings.Index(content[searchFrom:], toolCallOpenTag)
		if open < 0 {
			return blocks, starts
		}
		open += searchFrom
		closeIdx := strings.Index(content[open:], toolCallCloseTag)
		if closeIdx < 0 {
			return blocks, starts
		}
		closeIdx += open
		inner := strings.TrimSpace(content[open+len(toolCallOpenTag) : closeIdx])
		blocks = append(blocks, inner)
		starts = append(starts, open)
		searchFrom = closeIdx + len(toolCallCloseTag)
	}
}

type taglessCandidate struct {
	call     toolShimCall
	preamble string
}

// findTaglessToolCallCandidates 扫描裸 JSON 对象候选：含 name+arguments 键且 name 在工具表内。
// 依赖 JSON 解码器按值边界切分，天然跳过字符串内的花括号。
func findTaglessToolCallCandidates(content string, toolNames map[string]bool) []taglessCandidate {
	candidates := make([]taglessCandidate, 0, 2)
	for i := 0; i < len(content); i++ {
		if content[i] != '{' {
			continue
		}
		var raw json.RawMessage
		if err := common.DecodeJson(bytes.NewReader([]byte(content[i:])), &raw); err != nil {
			continue
		}
		var probe struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := common.Unmarshal(raw, &probe); err != nil || probe.Name == "" || probe.Arguments == nil {
			i += max(len(raw)-1, 0)
			continue
		}
		if call, ok := buildToolShimCall(probe.Name, probe.Arguments, toolNames); ok {
			candidates = append(candidates, taglessCandidate{
				call:     call,
				preamble: strings.TrimSpace(content[:i]),
			})
		}
		i += max(len(raw)-1, 0)
	}
	return candidates
}

// parseToolCallText 解析一个标签块内的 JSON 文本
func parseToolCallText(block string, toolNames map[string]bool) (toolShimCall, bool) {
	var probe struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := common.UnmarshalJsonStr(block, &probe); err != nil || probe.Name == "" {
		return toolShimCall{}, false
	}
	if probe.Arguments == nil {
		probe.Arguments = json.RawMessage("{}")
	}
	return buildToolShimCall(probe.Name, probe.Arguments, toolNames)
}

// buildToolShimCall 校验 name 并规整 arguments（对象直用；字符串则二次解析其内层 JSON）
func buildToolShimCall(name string, arguments json.RawMessage, toolNames map[string]bool) (toolShimCall, bool) {
	if !toolNames[name] {
		return toolShimCall{}, false
	}
	trimmed := bytes.TrimSpace(arguments)
	if len(trimmed) > 0 && trimmed[0] == '"' {
		// arguments 是 JSON 字符串，内层才是参数对象
		var inner string
		if err := common.Unmarshal(trimmed, &inner); err != nil {
			return toolShimCall{}, false
		}
		return toolShimCall{Name: name, Arguments: normalizeArgumentsJSON(inner)}, true
	}
	var obj map[string]any
	if err := common.Unmarshal(trimmed, &obj); err != nil {
		return toolShimCall{}, false
	}
	return toolShimCall{Name: name, Arguments: string(trimmed)}, true
}

// callParserModel 调用解析模型提取工具调用：
// 强制 tool_choice 到 extract_tool_calls 元函数，空数组表示无调用。
// 返回解析器分支的 usage（由调用方并入主请求结算）。
func callParserModel(c *gin.Context, info *relaycommon.RelayInfo, state *toolShimState, rawText string) ([]toolShimCall, *dto.Usage, error) {
	parserReq := &dto.GeneralOpenAIRequest{
		Model: state.ParserModel,
		Messages: []dto.Message{
			{Role: "system", Content: "You are a tool-call extraction engine. You receive a list of available tools and a raw assistant output that may contain a tool call attempt in an unstable format. Extract the intended tool call(s). If the assistant output does not contain a real intended tool call, return an empty calls array. Never invent a tool call that is not in the text."},
			{Role: "user", Content: "Available tools:\n" + renderToolsForParser(state.Tools) + "\n\nRaw assistant output:\n" + rawText},
		},
		Tools: []dto.ToolCallRequest{{
			Type: "function",
			Function: dto.FunctionRequest{
				Name:        parserMetaToolName,
				Description: "Report the tool call(s) intended by the raw assistant output",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"calls": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"name":      map[string]any{"type": "string"},
									"arguments": map[string]any{"type": "object"},
								},
								"required": []string{"name", "arguments"},
							},
						},
					},
					"required": []string{"calls"},
				},
			},
		}},
		ToolChoice:  map[string]any{"type": "function", "function": map[string]any{"name": parserMetaToolName}},
		Stream:      common.GetPointer(false),
		Temperature: common.GetPointer(0.0),
		MaxTokens:   common.GetPointer(uint(parserMaxTokens)),
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), parserCallTimeout)
	defer cancel()

	writer := newCaptureWriter()
	outcome := runVirtualBranch(c, info, model.VirtualModelTarget{Model: state.ParserModel}, parserReq, writer, ctx, 0)

	var parserUsage *dto.Usage
	if outcome.info != nil {
		parserUsage = outcome.info.VirtualBranchUsage
	}
	if outcome.err != nil {
		return nil, parserUsage, fmt.Errorf("parser branch failed: %s", outcome.err.Error())
	}

	var parsed dto.OpenAITextResponse
	if err := common.Unmarshal(writer.buf.Bytes(), &parsed); err != nil {
		return nil, parserUsage, fmt.Errorf("parser response unmarshal failed: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return nil, parserUsage, fmt.Errorf("parser response has no choices")
	}
	toolCalls := parsed.Choices[0].Message.ParseToolCalls()
	if len(toolCalls) == 0 || toolCalls[0].Function.Name != parserMetaToolName {
		return nil, parserUsage, fmt.Errorf("parser did not call meta function %s", parserMetaToolName)
	}
	var report struct {
		Calls []struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		} `json:"calls"`
	}
	if err := common.UnmarshalJsonStr(toolCalls[0].Function.Arguments, &report); err != nil {
		return nil, parserUsage, fmt.Errorf("parser report unmarshal failed: %w", err)
	}

	toolNames := toolShimToolNames(state.Tools)
	calls := make([]toolShimCall, 0, len(report.Calls))
	for _, rc := range report.Calls {
		if call, ok := buildToolShimCall(rc.Name, rc.Arguments, toolNames); ok {
			calls = append(calls, call)
		}
	}
	return calls, parserUsage, nil
}

// renderToolsForParser 为解析模型渲染紧凑的工具清单
func renderToolsForParser(tools []dto.ToolCallRequest) string {
	var b strings.Builder
	idx := 0
	for _, tool := range tools {
		if tool.Type != "" && tool.Type != "function" {
			continue
		}
		idx++
		params := "{}"
		if tool.Function.Parameters != nil {
			if bs, err := common.Marshal(tool.Function.Parameters); err == nil {
				params = string(bs)
			}
		}
		fmt.Fprintf(&b, "%d. %s(parameters: %s): %s\n", idx, tool.Function.Name, params, tool.Function.Description)
	}
	return b.String()
}
