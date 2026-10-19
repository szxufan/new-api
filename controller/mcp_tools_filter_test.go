package controller

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/mcp_setting"
	"github.com/gin-gonic/gin"
)

// setMCPModelPoolsForTest 覆盖指定模型池配置（键：image/i2i/t2v/i2v/kf2v/r2v/search/fetch），
// 并返回还原全部原始配置的函数
func setMCPModelPoolsForTest(pools map[string]map[string]string) func() {
	s := mcp_setting.GetGroupImageModelSetting()
	set := func(field string, m map[string]string) {
		switch field {
		case "image":
			s.GroupImageModels = m
		case "i2i":
			s.GroupI2IModels = m
		case "t2v":
			s.GroupVideoT2VModels = m
		case "i2v":
			s.GroupVideoI2VModels = m
		case "kf2v":
			s.GroupVideoKF2VModels = m
		case "r2v":
			s.GroupVideoR2VModels = m
		case "search":
			s.GroupSearchModels = m
		case "fetch":
			s.GroupFetchModels = m
		}
	}
	originals := map[string]map[string]string{}
	for field := range pools {
		switch field {
		case "image":
			originals[field] = s.GroupImageModels
		case "i2i":
			originals[field] = s.GroupI2IModels
		case "t2v":
			originals[field] = s.GroupVideoT2VModels
		case "i2v":
			originals[field] = s.GroupVideoI2VModels
		case "kf2v":
			originals[field] = s.GroupVideoKF2VModels
		case "r2v":
			originals[field] = s.GroupVideoR2VModels
		case "search":
			originals[field] = s.GroupSearchModels
		case "fetch":
			originals[field] = s.GroupFetchModels
		}
	}
	for field, m := range pools {
		set(field, m)
	}
	return func() {
		for field, m := range originals {
			set(field, m)
		}
	}
}

// emptyMCPModelPools 返回"全部池清空 + 指定覆盖"的池配置，
// 避免全局默认值（dall-e-3 / deepseek-fetch 等）干扰"仅配置某池"的断言
func emptyMCPModelPools(overrides map[string]map[string]string) map[string]map[string]string {
	all := map[string]map[string]string{
		"image": {}, "i2i": {}, "t2v": {}, "i2v": {}, "kf2v": {}, "r2v": {},
		"search": {}, "fetch": {},
	}
	for k, v := range overrides {
		all[k] = v
	}
	return all
}

// TestMCPToolVisible 测试工具可见性规则：任一依赖场景配置了模型即可见，
// 不依赖模型池的工具始终可见
func TestMCPToolVisible(t *testing.T) {
	restore := setMCPModelPoolsForTest(emptyMCPModelPools(map[string]map[string]string{
		"search": {"default": "deepseek-search"},
	}))
	defer restore()

	visibleTools := []string{"web_search", "get_video_task", "request_upload_ticket", "unknown-future-tool"}
	for _, tool := range visibleTools {
		if !mcpToolVisible("default", tool) {
			t.Errorf("expected tool %q to be visible when search pool configured", tool)
		}
	}
	hiddenTools := []string{"web_fetch", "generate_image", "generate_video", "generate_video_from_frames", "generate_video_from_reference"}
	for _, tool := range hiddenTools {
		if mcpToolVisible("default", tool) {
			t.Errorf("expected tool %q to be hidden when only search pool configured", tool)
		}
	}

	// 任一依赖场景命中即可见：仅 image（无 i2i）→ generate_image 可见
	restore2 := setMCPModelPoolsForTest(emptyMCPModelPools(map[string]map[string]string{
		"image": {"default": "dall-e-3"},
	}))
	defer restore2()
	if !mcpToolVisible("default", "generate_image") {
		t.Error("expected generate_image visible when image pool configured")
	}
	if mcpToolVisible("default", "generate_video") {
		t.Error("expected generate_video hidden when t2v pool empty")
	}

	// 分组隔离：仅 vip 配置 search 池时，default 分组（无 default 回退）不可见
	restore3 := setMCPModelPoolsForTest(emptyMCPModelPools(map[string]map[string]string{
		"search": {"vip": "deepseek-search"},
	}))
	defer restore3()
	if !mcpToolVisible("vip", "web_search") {
		t.Error("expected web_search visible for group vip")
	}
	if mcpToolVisible("default", "web_search") {
		t.Error("expected web_search hidden for group default when only vip configured")
	}
}

// TestMCPRequestIsToolsList 测试 tools/list 请求识别（单条与批量）
func TestMCPRequestIsToolsList(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"单条 tools/list", `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, true},
		{"批量含 tools/list", `[{"jsonrpc":"2.0","id":1,"method":"tools/list"},{"jsonrpc":"2.0","method":"notifications/initialized"}]`, true},
		{"tools/call", `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"web_search"}}`, false},
		{"initialize", `{"jsonrpc":"2.0","id":0,"method":"initialize"}`, false},
		{"批量不含", `[{"jsonrpc":"2.0","id":1,"method":"tools/call"}]`, false},
		{"非法 JSON", `not-json`, false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := mcpRequestIsToolsList([]byte(tt.body)); got != tt.want {
				t.Errorf("mcpRequestIsToolsList(%s) = %v, want %v", tt.body, got, tt.want)
			}
		})
	}
}

// buildMCPToolsListBody 构造包含全部 8 个工具的 tools/list JSON-RPC 响应体
func buildMCPToolsListBody() []byte {
	names := []string{
		"generate_image", "generate_video", "generate_video_from_frames",
		"generate_video_from_reference", "get_video_task", "request_upload_ticket",
		"web_search", "web_fetch",
	}
	var b strings.Builder
	b.WriteString(`{"jsonrpc":"2.0","id":1,"result":{"tools":[`)
	for i, name := range names {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `{"name":%q,"description":"tool %s"}`, name, name)
	}
	b.WriteString(`],"nextCursor":""}}`)
	return []byte(b.String())
}

// toolNamesFromBody 解析响应体中 tools/list 结果的工具名列表
func toolNamesFromBody(t *testing.T, body []byte) []string {
	t.Helper()
	var msg struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := common.Unmarshal(body, &msg); err != nil {
		t.Fatalf("failed to unmarshal filtered body: %v\nbody: %s", err, body)
	}
	names := make([]string, 0, len(msg.Result.Tools))
	for _, tool := range msg.Result.Tools {
		names = append(names, tool.Name)
	}
	return names
}

// TestFilterMCPToolsListBody_JSON 测试 JSON 形态响应的过滤与透传
func TestFilterMCPToolsListBody_JSON(t *testing.T) {
	restore := setMCPModelPoolsForTest(emptyMCPModelPools(map[string]map[string]string{
		"search": {"default": "deepseek-search"},
	}))
	defer restore()

	body := buildMCPToolsListBody()
	filtered, changed := filterMCPToolsListBody(body, "application/json", "default")
	if !changed {
		t.Fatal("expected body to be changed")
	}
	names := toolNamesFromBody(t, filtered)
	want := []string{"get_video_task", "request_upload_ticket", "web_search"}
	if len(names) != len(want) {
		t.Fatalf("expected %v, got %v", want, names)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, names)
		}
	}

	// 全部场景配置后无工具被移除：返回原始字节、changed=false
	restoreAll := setMCPModelPoolsForTest(map[string]map[string]string{
		"image": {"default": "dall-e-3"}, "i2i": {"default": "qwen-image-edit"},
		"t2v": {"default": "wan-t2v"}, "i2v": {"default": "wan-i2v"},
		"kf2v": {"default": "wan-kf2v"}, "r2v": {"default": "wan-r2v"},
		"search": {"default": "deepseek-search"}, "fetch": {"default": "deepseek-fetch"},
	})
	defer restoreAll()
	same, changed := filterMCPToolsListBody(body, "application/json", "default")
	if changed {
		t.Error("expected no change when all pools configured")
	}
	if string(same) != string(body) {
		t.Error("expected original bytes returned when nothing filtered")
	}

	// 非工具列表消息不受影响
	callBody := []byte(`{"jsonrpc":"2.0","id":2,"result":{"content":[{"type":"text","text":"ok"}]}}`)
	out, changed := filterMCPToolsListBody(callBody, "application/json", "default")
	if changed || string(out) != string(callBody) {
		t.Error("expected tools/call response to pass through untouched")
	}

	// 错误响应不受影响
	errBody := []byte(`{"jsonrpc":"2.0","id":3,"error":{"code":-32601,"message":"method not found"}}`)
	out, changed = filterMCPToolsListBody(errBody, "application/json", "default")
	if changed || string(out) != string(errBody) {
		t.Error("expected error response to pass through untouched")
	}
}

// TestFilterMCPToolsListBody_JSONBatch 测试批量 JSON 响应只过滤其中的 tools/list 消息
func TestFilterMCPToolsListBody_JSONBatch(t *testing.T) {
	restore := setMCPModelPoolsForTest(emptyMCPModelPools(map[string]map[string]string{
		"search": {"default": "deepseek-search"},
	}))
	defer restore()

	callMsg := `{"jsonrpc":"2.0","id":2,"result":{"content":[{"type":"text","text":"ok"}]}}`
	body := []byte("[" + string(buildMCPToolsListBody()) + "," + callMsg + "]")
	filtered, changed := filterMCPToolsListBody(body, "application/json", "default")
	if !changed {
		t.Fatal("expected batch body to be changed")
	}

	var msgs []map[string]any
	if err := common.Unmarshal(filtered, &msgs); err != nil {
		t.Fatalf("failed to unmarshal batch: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	tools, ok := msgs[0]["result"].(map[string]any)["tools"].([]any)
	if !ok || len(tools) != 3 {
		t.Errorf("expected 3 tools in first message, got %v", tools)
	}
	if _, ok := msgs[1]["result"].(map[string]any)["content"]; !ok {
		t.Error("expected second message untouched")
	}
}

// TestFilterMCPToolsListBody_SSE 测试 SSE 形态响应的过滤：data: 行改写，其余行保留
func TestFilterMCPToolsListBody_SSE(t *testing.T) {
	restore := setMCPModelPoolsForTest(emptyMCPModelPools(map[string]map[string]string{
		"search": {"default": "deepseek-search"},
	}))
	defer restore()

	body := []byte("event: message\r\n" +
		"data: " + string(buildMCPToolsListBody()) + "\r\n\r\n")
	filtered, changed := filterMCPToolsListBody(body, "text/event-stream", "default")
	if !changed {
		t.Fatal("expected SSE body to be changed")
	}
	text := string(filtered)
	if !strings.HasPrefix(text, "event: message") {
		t.Errorf("expected event line preserved, got: %s", text[:40])
	}
	dataLine := ""
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "data: ") {
			dataLine = strings.TrimSpace(strings.TrimPrefix(line, "data: "))
		}
	}
	if dataLine == "" {
		t.Fatal("expected data line in filtered SSE body")
	}
	names := toolNamesFromBody(t, []byte(dataLine))
	if len(names) != 3 || names[2] != "web_search" {
		t.Errorf("expected only search/get_video_task/request_upload_ticket, got %v", names)
	}
}

// newMCPFilterTestRequest 构造带 gin context（含指定分组）的 MCP 请求
func newMCPFilterTestRequest(method, path, body string, group string) *http.Request {
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	ginW := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(ginW)
	ginCtx.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	if group != "" {
		common.SetContextKey(ginCtx, constant.ContextKeyUsingGroup, group)
	}
	return req.WithContext(context.WithValue(req.Context(), mcpGinContextKey, ginCtx))
}

// TestWrapMCPToolsListFilter 测试 wrapper：tools/list 响应被过滤，
// 其余请求（tools/call、GET）原样透传且请求体完整恢复
func TestWrapMCPToolsListFilter(t *testing.T) {
	restore := setMCPModelPoolsForTest(emptyMCPModelPools(map[string]map[string]string{
		"search": {"default": "deepseek-search"},
	}))
	defer restore()

	toolsListBody := buildMCPToolsListBody()
	var receivedBody string
	base := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(toolsListBody)
	})
	wrapped := wrapMCPToolsListFilter(base)

	// tools/list POST：响应被过滤
	w := httptest.NewRecorder()
	reqBody := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	wrapped.ServeHTTP(w, newMCPFilterTestRequest(http.MethodPost, "/v1/mcp", reqBody, "default"))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if receivedBody != reqBody {
		t.Errorf("expected base handler to receive restored request body, got %q", receivedBody)
	}
	if names := toolNamesFromBody(t, w.Body.Bytes()); len(names) != 3 {
		t.Errorf("expected filtered tools list, got %v", names)
	}

	// tools/call POST：响应原样透传
	callReqBody := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"web_search","arguments":{"query":"go"}}}`
	w2 := httptest.NewRecorder()
	wrapped.ServeHTTP(w2, newMCPFilterTestRequest(http.MethodPost, "/v1/mcp", callReqBody, "default"))
	if w2.Body.String() != string(toolsListBody) {
		t.Error("expected tools/call response to pass through verbatim")
	}

	// GET：直接透传，不读取请求体
	passedThrough := false
	getBase := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		passedThrough = true
	})
	wrappedGet := wrapMCPToolsListFilter(getBase)
	wrappedGet.ServeHTTP(httptest.NewRecorder(), newMCPFilterTestRequest(http.MethodGet, "/v1/mcp", "", "default"))
	if !passedThrough {
		t.Error("expected GET request to pass through to base handler")
	}
}

// TestWrapMCPToolsListFilter_NoGinContext 请求无 gin context 时不过滤（fail-open）
func TestWrapMCPToolsListFilter_NoGinContext(t *testing.T) {
	base := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(buildMCPToolsListBody())
	})
	wrapped := wrapMCPToolsListFilter(base)

	req := httptest.NewRequest(http.MethodPost, "/v1/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)
	if names := toolNamesFromBody(t, w.Body.Bytes()); len(names) != 8 {
		t.Errorf("expected all 8 tools listed when gin context missing, got %v", names)
	}
}

// TestMCPServerHandler_ToolsListFiltered 端到端：真实 SDK 链路的 tools/list
// 按调用方分组过滤工具（stateless + JSONResponse，无需先 initialize）
func TestMCPServerHandler_ToolsListFiltered(t *testing.T) {
	restore := setMCPModelPoolsForTest(emptyMCPModelPools(map[string]map[string]string{
		"search": {"default": "deepseek-search"},
	}))
	defer restore()

	handler := MCPServerHandler()
	req := newMCPFilterTestRequest(http.MethodPost, "/v1/mcp", `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, "default")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("expected application/json response, got %q", ct)
	}
	names := toolNamesFromBody(t, w.Body.Bytes())
	want := []string{"get_video_task", "request_upload_ticket", "web_search"}
	if len(names) != len(want) {
		t.Fatalf("expected tools %v, got %v", want, names)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("expected tools %v, got %v", want, names)
		}
	}
}
