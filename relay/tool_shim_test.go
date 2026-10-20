package relay

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
)

func shimTestTools() []dto.ToolCallRequest {
	return []dto.ToolCallRequest{
		{
			Type: "function",
			Function: dto.FunctionRequest{
				Name:        "get_weather",
				Description: "Get current weather for a city",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"city": map[string]any{"type": "string"},
					},
					"required": []string{"city"},
				},
			},
		},
		{
			Type: "function",
			Function: dto.FunctionRequest{
				Name:        "read_file",
				Description: "Read a file",
				Parameters: map[string]any{
					"type":       "object",
					"properties": map[string]any{"path": map[string]any{"type": "string"}},
				},
			},
		},
	}
}

func newShimTestContext(t *testing.T) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	c.Request = req
	return c
}

func newShimTestInfo(mapping string) *relaycommon.RelayInfo {
	info := &relaycommon.RelayInfo{OriginModelName: "or-free"}
	info.ChannelMeta = &relaycommon.ChannelMeta{
		ChannelId:          1,
		ModelParserMapping: mapping,
	}
	return info
}

func TestBuildToolShimSystemPrompt(t *testing.T) {
	prompt := buildToolShimSystemPrompt(shimTestTools())
	for _, want := range []string{"get_weather", "read_file", toolCallOpenTag, toolCallCloseTag, "Never fabricate"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("system prompt missing %q", want)
		}
	}
}

func TestInjectToolShimSystemPrompt(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Messages: []dto.Message{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "hi"},
		},
	}
	req.Tools = shimTestTools()
	injectToolShimSystemPrompt(req)
	if len(req.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(req.Messages))
	}
	if !strings.HasPrefix(req.Messages[0].StringContent(), "You can call tools.") {
		t.Errorf("shim prompt not prepended to existing system message")
	}
	if !strings.Contains(req.Messages[0].StringContent(), "You are helpful.") {
		t.Errorf("original system prompt lost")
	}

	req2 := &dto.GeneralOpenAIRequest{Messages: []dto.Message{{Role: "user", Content: "hi"}}}
	req2.Tools = shimTestTools()
	injectToolShimSystemPrompt(req2)
	if len(req2.Messages) != 2 || req2.Messages[0].Role != "system" {
		t.Fatalf("expected system message inserted, got %+v", req2.Messages)
	}
}

func TestFlattenToolShimHistory(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Messages: []dto.Message{
			{Role: "system", Content: "sys"},
			{Role: "user", Content: "weather?"},
			{
				Role:    "assistant",
				Content: "Let me check.",
				ToolCalls: marshalForTest(t, []dto.ToolCallRequest{
					{ID: "call_1", Type: "function", Function: dto.FunctionRequest{Name: "get_weather", Arguments: `{"city":"Beijing"}`}},
					{ID: "call_2", Type: "function", Function: dto.FunctionRequest{Name: "read_file", Arguments: `{"path":"/tmp/a"}`}},
				}),
			},
			{Role: "tool", ToolCallId: "call_1", Content: `{"temp":3.2}`},
			{Role: "tool", ToolCallId: "call_2", Content: "file content"},
			{Role: "user", Content: "thanks"},
		},
	}

	flattenToolShimHistory(req)

	if len(req.Messages) != 5 {
		t.Fatalf("expected 5 messages after flatten, got %d: %+v", len(req.Messages), req.Messages)
	}
	assistant := req.Messages[2]
	if assistant.Role != "assistant" {
		t.Fatalf("expected assistant at index 2, got %s", assistant.Role)
	}
	if assistant.ToolCalls != nil {
		t.Errorf("assistant ToolCalls should be cleared")
	}
	if strings.Count(assistant.StringContent(), toolCallOpenTag) != 2 {
		t.Errorf("expected 2 tool_call blocks in flattened assistant, got content: %s", assistant.StringContent())
	}
	// 两个连续 tool 结果应合并为一条 user 消息
	merged := req.Messages[3]
	if merged.Role != "user" {
		t.Fatalf("expected merged tool results as user message at index 3, got %s", merged.Role)
	}
	content := merged.StringContent()
	if !strings.Contains(content, "Tool result for get_weather") || !strings.Contains(content, `{"temp":3.2}`) {
		t.Errorf("merged tool result missing first result: %s", content)
	}
	if !strings.Contains(content, "Tool result for read_file") || !strings.Contains(content, "file content") {
		t.Errorf("merged tool result missing second result: %s", content)
	}
	if req.Messages[4].StringContent() != "thanks" {
		t.Errorf("trailing user message altered")
	}
}

func marshalForTest(t *testing.T, v any) []byte {
	t.Helper()
	bs, err := common.Marshal(v)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	return bs
}

func TestMergeToolShimStopSequence(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{}
	mergeToolShimStopSequence(req)
	if !isEqualStringSlice(req.Stop, []string{toolCallCloseTag}) {
		t.Errorf("nil stop: got %v", req.Stop)
	}

	req = &dto.GeneralOpenAIRequest{Stop: "END"}
	mergeToolShimStopSequence(req)
	if !isEqualStringSlice(req.Stop, []string{"END", toolCallCloseTag}) {
		t.Errorf("string stop: got %v", req.Stop)
	}

	req = &dto.GeneralOpenAIRequest{Stop: []string{"END", toolCallCloseTag}}
	mergeToolShimStopSequence(req)
	if !isEqualStringSlice(req.Stop, []string{"END", toolCallCloseTag}) {
		t.Errorf("existing tag stop: got %v", req.Stop)
	}

	req = &dto.GeneralOpenAIRequest{Stop: []any{"END"}}
	mergeToolShimStopSequence(req)
	if !isEqualStringSlice(req.Stop, []string{"END", toolCallCloseTag}) {
		t.Errorf("[]any stop: got %v", req.Stop)
	}
}

func isEqualStringSlice(v any, want []string) bool {
	switch s := v.(type) {
	case []string:
		if len(s) != len(want) {
			return false
		}
		for i := range s {
			if s[i] != want[i] {
				return false
			}
		}
		return true
	}
	return false
}

func TestExtractToolCallsFastTaggedBlockWithJunk(t *testing.T) {
	content := "Let me check.\n" + toolCallOpenTag + "\n" + `{"name": "get_weather", "arguments": {"city": "Shanghai", "unit": "celsius"}}` + "\n" + toolCallCloseTag + "Question: unrelated training junk..."
	calls, preamble, needParser := extractToolCallsFast(content, shimTestTools())
	if needParser {
		t.Fatalf("should not need parser")
	}
	if len(calls) != 1 || calls[0].Name != "get_weather" {
		t.Fatalf("unexpected calls: %+v", calls)
	}
	if !strings.Contains(calls[0].Arguments, `"city"`) {
		t.Errorf("arguments lost: %s", calls[0].Arguments)
	}
	if preamble != "Let me check." {
		t.Errorf("preamble = %q", preamble)
	}
}

func TestExtractToolCallsFastTagless(t *testing.T) {
	content := "\n\n" + `{"name": "get_weather", "arguments": {"city": "Shanghai", "unit": "celsius"}}`
	calls, _, needParser := extractToolCallsFast(content, shimTestTools())
	if needParser || len(calls) != 1 || calls[0].Name != "get_weather" {
		t.Fatalf("tagless extraction failed: calls=%+v needParser=%v", calls, needParser)
	}
}

func TestExtractToolCallsFastArgumentsAsString(t *testing.T) {
	block := toolCallOpenTag + "\n" + `{"name": "get_weather", "arguments": "{\"city\": \"Beijing\"}"}` + "\n" + toolCallCloseTag
	calls, _, needParser := extractToolCallsFast(block, shimTestTools())
	if needParser || len(calls) != 1 {
		t.Fatalf("string-arguments extraction failed: %+v", calls)
	}
	if !strings.Contains(calls[0].Arguments, `"city"`) {
		t.Errorf("inner arguments not parsed: %s", calls[0].Arguments)
	}
}

func TestExtractToolCallsFastNoCall(t *testing.T) {
	calls, _, needParser := extractToolCallsFast("The weather in Beijing is cold today.", shimTestTools())
	if needParser || len(calls) != 0 {
		t.Fatalf("plain text should yield no call: %+v needParser=%v", calls, needParser)
	}
}

func TestExtractToolCallsFastPartialTagNeedsParser(t *testing.T) {
	calls, _, needParser := extractToolCallsFast(toolCallOpenTag+` {"name": "get_weather"`, shimTestTools())
	if !needParser || len(calls) != 0 {
		t.Fatalf("partial tag should trigger parser: calls=%+v needParser=%v", calls, needParser)
	}
}

func TestExtractToolCallsFastVariantTagNeedsParser(t *testing.T) {
	// 无下划线变体标签 + 可提取的 JSON（混合漂移）
	content := `<toolcall>` + "\n" + `{"name": "get_weather", "arguments": {"city": "Shanghai"}}` + "\n" + `</toolcall>`
	calls, _, needParser := extractToolCallsFast(content, shimTestTools())
	if !needParser || len(calls) != 0 {
		t.Fatalf("variant tag with JSON should trigger parser: calls=%+v needParser=%v", calls, needParser)
	}

	// 自创状态协议：无函数名无参数
	content = "<toolcall_status>waiting</toolcall_status>\n<toolcall/>"
	calls, _, needParser = extractToolCallsFast(content, shimTestTools())
	if !needParser || len(calls) != 0 {
		t.Fatalf("status-protocol variant should trigger parser: calls=%+v needParser=%v", calls, needParser)
	}

	// 正文里不带尖括号的 toolcall 字样不应误触发
	calls, _, needParser = extractToolCallsFast("I would use the toolcall mechanism to check.", shimTestTools())
	if needParser || len(calls) != 0 {
		t.Fatalf("prose 'toolcall' should not trigger parser: calls=%+v needParser=%v", calls, needParser)
	}
}

func TestExtractToolCallsFastAmbiguousTaglessNeedsParser(t *testing.T) {
	content := `{"name": "get_weather", "arguments": {"city": "A"}} and also {"name": "read_file", "arguments": {"path": "B"}}`
	calls, _, needParser := extractToolCallsFast(content, shimTestTools())
	if !needParser || len(calls) != 0 {
		t.Fatalf("ambiguous candidates should trigger parser: calls=%+v needParser=%v", calls, needParser)
	}
}

func TestExtractToolCallsFastUnknownNameNeedsParser(t *testing.T) {
	block := toolCallOpenTag + "\n" + `{"name": "nonexistent_tool", "arguments": {}}` + "\n" + toolCallCloseTag
	calls, _, needParser := extractToolCallsFast(block, shimTestTools())
	if !needParser || len(calls) != 0 {
		t.Fatalf("unknown tool name should trigger parser: calls=%+v needParser=%v", calls, needParser)
	}
}

func TestApplyToolCallShimRequestGate(t *testing.T) {
	tools := shimTestTools()

	// 无映射 → 不启用
	c := newShimTestContext(t)
	req := &dto.GeneralOpenAIRequest{Model: "or-free", Tools: tools, Messages: []dto.Message{{Role: "user", Content: "hi"}}}
	if state := applyToolCallShimRequest(c, newShimTestInfo(""), req); state != nil {
		t.Fatalf("empty mapping should disable shim")
	}

	// 模型未命中 → 不启用
	c = newShimTestContext(t)
	info := newShimTestInfo(`{"other-model": "parser"}`)
	if state := applyToolCallShimRequest(c, info, req); state != nil {
		t.Fatalf("model miss should disable shim")
	}

	// 非法 JSON → 不启用
	c = newShimTestContext(t)
	info = newShimTestInfo(`{invalid`)
	if state := applyToolCallShimRequest(c, info, req); state != nil {
		t.Fatalf("invalid mapping should disable shim")
	}

	// 虚拟分支守卫 → 不启用
	c = newShimTestContext(t)
	common.SetContextKey(c, constant.ContextKeyVirtualBranch, true)
	info = newShimTestInfo(`{"or-free": "parser-model"}`)
	if state := applyToolCallShimRequest(c, info, req); state != nil {
		t.Fatalf("virtual branch should disable shim")
	}

	// 命中：映射 + tools → 启用并改写
	c = newShimTestContext(t)
	info = newShimTestInfo(`{"or-free": "parser-model"}`)
	req = &dto.GeneralOpenAIRequest{
		Model:  "or-free",
		Tools:  tools,
		Stop:   "END",
		Messages: []dto.Message{
			{Role: "system", Content: "be nice"},
			{Role: "user", Content: "weather?"},
		},
	}
	state := applyToolCallShimRequest(c, info, req)
	if state == nil {
		t.Fatalf("shim should be enabled")
	}
	if state.ParserModel != "parser-model" {
		t.Errorf("parser model = %s", state.ParserModel)
	}
	if req.Tools != nil || req.ToolChoice != nil || req.ParallelTooCalls != nil {
		t.Errorf("tools fields not stripped: %+v", req)
	}
	if !strings.Contains(req.Messages[0].StringContent(), "You can call tools.") {
		t.Errorf("shim prompt not injected")
	}
	if !isEqualStringSlice(req.Stop, []string{"END", toolCallCloseTag}) {
		t.Errorf("stop not merged: %v", req.Stop)
	}

	// 命中：仅历史含 tool 消息（无 tools）→ 启用并压平，但不注入 prompt
	c = newShimTestContext(t)
	info = newShimTestInfo(`{"or-free": "parser-model"}`)
	req = &dto.GeneralOpenAIRequest{
		Model: "or-free",
		Messages: []dto.Message{
			{
				Role:    "assistant",
				ToolCalls: marshalForTest(t, []dto.ToolCallRequest{
					{ID: "call_1", Type: "function", Function: dto.FunctionRequest{Name: "get_weather", Arguments: `{"city":"Beijing"}`}},
				}),
			},
			{Role: "tool", ToolCallId: "call_1", Content: `{"temp":3.2}`},
		},
	}
	state = applyToolCallShimRequest(c, info, req)
	if state == nil {
		t.Fatalf("shim should be enabled for tool history")
	}
	if req.Messages[0].ToolCalls != nil {
		t.Errorf("history not flattened")
	}
	firstRole := req.Messages[0].StringContent()
	if !strings.Contains(firstRole, toolCallOpenTag) {
		t.Errorf("flattened assistant missing tool_call block: %s", firstRole)
	}
}

func TestNormalizeArgumentsJSON(t *testing.T) {
	if got := normalizeArgumentsJSON(`{"city":"Beijing"}`); got != `{"city":"Beijing"}` {
		t.Errorf("valid object altered: %s", got)
	}
	if got := normalizeArgumentsJSON("not json"); got != "{}" {
		t.Errorf("invalid json should fall back to {}: %s", got)
	}
	if got := normalizeArgumentsJSON(`[1,2]`); got != "{}" {
		t.Errorf("non-object should fall back to {}: %s", got)
	}
	if got := normalizeArgumentsJSON(""); got != "{}" {
		t.Errorf("empty should fall back to {}: %s", got)
	}
}
