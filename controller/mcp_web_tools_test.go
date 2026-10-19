package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting/mcp_setting"
	"github.com/gin-gonic/gin"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// newMCPWebToolTestContext 构造带空请求 gin context 的 MCP 工具调用 context
func newMCPWebToolTestContext() context.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	return context.WithValue(context.Background(), mcpGinContextKey, c)
}

func newWebToolCallRequest(args string) *mcp.CallToolRequest {
	return &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Arguments: json.RawMessage(args),
		},
	}
}

// TestHandleMCPWebSearch_NoGinContext 测试缺少 gin context 时返回错误
func TestHandleMCPWebSearch_NoGinContext(t *testing.T) {
	result, err := handleMCPWebSearch(context.Background(), newWebToolCallRequest(`{"query":"test"}`))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError to be true when gin context is missing")
	}
}

// TestHandleMCPWebSearch_ArgumentValidation 测试 query/count 参数校验
func TestHandleMCPWebSearch_ArgumentValidation(t *testing.T) {
	cases := []struct {
		name string
		args string
	}{
		{"缺少 query", `{}`},
		{"query 全空白", `{"query":"   "}`},
		{"query 超长", `{"query":"` + strings.Repeat("字", 129) + `"}`},
		{"count 超上限", `{"query":"test","count":51}`},
		{"count 低于下限", `{"query":"test","count":0}`},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			result, err := handleMCPWebSearch(newMCPWebToolTestContext(), newWebToolCallRequest(tt.args))
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if !result.IsError {
				t.Errorf("expected IsError to be true for case %q", tt.name)
			}
		})
	}
}

// TestHandleMCPWebSearch_NoModelConfigured 测试分组未配置搜索模型时返回错误
func TestHandleMCPWebSearch_NoModelConfigured(t *testing.T) {
	original := mcp_setting.GetGroupImageModelSetting().GroupSearchModels
	defer func() {
		mcp_setting.GetGroupImageModelSetting().GroupSearchModels = original
	}()
	mcp_setting.GetGroupImageModelSetting().GroupSearchModels = map[string]string{}

	result, err := handleMCPWebSearch(newMCPWebToolTestContext(), newWebToolCallRequest(`{"query":"golang"}`))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError to be true when no search model configured")
	}
}

// TestHandleMCPWebFetch_NoGinContext 测试缺少 gin context 时返回错误
func TestHandleMCPWebFetch_NoGinContext(t *testing.T) {
	result, err := handleMCPWebFetch(context.Background(), newWebToolCallRequest(`{"url":"https://example.com"}`))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError to be true when gin context is missing")
	}
}

// TestHandleMCPWebFetch_ArgumentValidation 测试 url 参数校验
func TestHandleMCPWebFetch_ArgumentValidation(t *testing.T) {
	cases := []struct {
		name string
		args string
	}{
		{"缺少 url", `{}`},
		{"url 为空白", `{"url":"   "}`},
		{"非 http(s) 协议", `{"url":"ftp://example.com/page"}`},
		{"缺少 host", `{"url":"http:///path"}`},
		{"携带凭据", `{"url":"http://user:pass@example.com"}`},
		{"url 超长", `{"url":"https://example.com/` + strings.Repeat("a", 8192) + `"}`},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			result, err := handleMCPWebFetch(newMCPWebToolTestContext(), newWebToolCallRequest(tt.args))
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if !result.IsError {
				t.Errorf("expected IsError to be true for case %q", tt.name)
			}
		})
	}
}

// TestHandleMCPWebFetch_NoModelConfigured 测试分组未配置抓取模型时返回错误
func TestHandleMCPWebFetch_NoModelConfigured(t *testing.T) {
	original := mcp_setting.GetGroupImageModelSetting().GroupFetchModels
	defer func() {
		mcp_setting.GetGroupImageModelSetting().GroupFetchModels = original
	}()
	mcp_setting.GetGroupImageModelSetting().GroupFetchModels = map[string]string{}

	result, err := handleMCPWebFetch(newMCPWebToolTestContext(), newWebToolCallRequest(`{"url":"https://example.com"}`))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError to be true when no fetch model configured")
	}
}

// TestWebSearchInputSchema 测试 web_search schema 结构
func TestWebSearchInputSchema(t *testing.T) {
	var parsed map[string]any
	if err := common.Unmarshal(webSearchInputSchema(), &parsed); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}
	props, ok := parsed["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected properties to be a map")
	}
	for _, name := range []string{"query", "count", "ori_question", "need_content"} {
		if _, ok := props[name].(map[string]any); !ok {
			t.Errorf("expected %q property in web_search schema", name)
		}
	}
	required, ok := parsed["required"].([]any)
	if !ok || len(required) != 1 || required[0] != "query" {
		t.Errorf("expected required to be [query], got %v", parsed["required"])
	}
}

// TestWebFetchInputSchema 测试 web_fetch schema 结构
func TestWebFetchInputSchema(t *testing.T) {
	var parsed map[string]any
	if err := common.Unmarshal(webFetchInputSchema(), &parsed); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}
	props, ok := parsed["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected properties to be a map")
	}
	if _, ok := props["url"].(map[string]any); !ok {
		t.Error("expected 'url' property in web_fetch schema")
	}
	required, ok := parsed["required"].([]any)
	if !ok || len(required) != 1 || required[0] != "url" {
		t.Errorf("expected required to be [url], got %v", parsed["required"])
	}
}

// TestWebSearchRequestMarshalOptionalFields 测试可选标量的显式零值仍被序列化（规则 6）
func TestWebSearchRequestMarshalOptionalFields(t *testing.T) {
	zero := 0
	flagFalse := false
	req := dto.WebSearchRequest{
		Query:       "golang",
		Count:       &zero,
		NeedContent: &flagFalse,
	}
	data, err := common.Marshal(req)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var raw map[string]any
	if err := common.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if v, ok := raw["count"]; !ok || v.(float64) != 0 {
		t.Errorf("expected count=0 to be serialized, got %v", raw)
	}
	if v, ok := raw["need_content"]; !ok || v.(bool) != false {
		t.Errorf("expected need_content=false to be serialized, got %v", raw)
	}
	if _, ok := raw["model"]; ok {
		t.Error("model field must not be serialized to upstream")
	}

	// 未设置的可选字段应被省略
	minimal, err := common.Marshal(dto.WebSearchRequest{Query: "golang"})
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if strings.Contains(string(minimal), "count") || strings.Contains(string(minimal), "need_content") {
		t.Errorf("expected optional fields omitted, got %s", minimal)
	}
}
