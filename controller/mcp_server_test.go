package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/mcp_setting"
	"github.com/gin-gonic/gin"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestExtractImageSources_URLFormat 测试从标准 OpenAI 格式响应中提取图片 URL
func TestExtractImageSources_URLFormat(t *testing.T) {
	respBody := []byte(`{
		"created": 1234567890,
		"data": [
			{"url": "https://example.com/image1.png"},
			{"url": "https://example.com/image2.png"}
		]
	}`)

	sources, err := extractImageSources(respBody)
	if err != nil {
		t.Fatalf("extractImageSources failed: %v", err)
	}
	if len(sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(sources))
	}
	if sources[0].url != "https://example.com/image1.png" {
		t.Errorf("expected first URL to be image1.png, got %s", sources[0].url)
	}
	if sources[1].url != "https://example.com/image2.png" {
		t.Errorf("expected second URL to be image2.png, got %s", sources[1].url)
	}
}

// TestExtractImageSources_B64Format 测试从 b64_json 格式响应中提取图片
func TestExtractImageSources_B64Format(t *testing.T) {
	respBody := []byte(`{
		"created": 1234567890,
		"data": [
			{"b64_json": "iVBORw0KGgoAAAANSUhEUg=="}
		]
	}`)

	sources, err := extractImageSources(respBody)
	if err != nil {
		t.Fatalf("extractImageSources failed: %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}
	if sources[0].b64 != "iVBORw0KGgoAAAANSUhEUg==" {
		t.Errorf("expected b64 data, got %s", sources[0].b64)
	}
}

// TestExtractImageSources_MixedFormat 测试混合格式响应
func TestExtractImageSources_MixedFormat(t *testing.T) {
	respBody := []byte(`{
		"created": 1234567890,
		"data": [
			{"url": "https://example.com/image1.png"},
			{"b64_json": "iVBORw0KGgoAAAANSUhEUg=="}
		]
	}`)

	sources, err := extractImageSources(respBody)
	if err != nil {
		t.Fatalf("extractImageSources failed: %v", err)
	}
	if len(sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(sources))
	}
}

// TestExtractImageSources_EmptyResponse 测试空响应
func TestExtractImageSources_EmptyResponse(t *testing.T) {
	_, err := extractImageSources([]byte(`{}`))
	if err == nil {
		t.Error("expected error for empty response, got nil")
	}
}

// TestExtractImageSources_InvalidJSON 测试无效 JSON
func TestExtractImageSources_InvalidJSON(t *testing.T) {
	_, err := extractImageSources([]byte(`not json at all`))
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

// TestNewMCPErrorResult 测试错误结果构造
func TestNewMCPErrorResult(t *testing.T) {
	result := newMCPErrorResult("test error message")
	if !result.IsError {
		t.Error("expected IsError to be true")
	}
	if len(result.Content) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(result.Content))
	}
	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatal("expected content to be *mcp.TextContent")
	}
	if textContent.Text != "test error message" {
		t.Errorf("expected text 'test error message', got %s", textContent.Text)
	}
}

// TestMCPServerHandler_NotNil 测试 MCPServerHandler 返回非 nil handler
func TestMCPServerHandler_NotNil(t *testing.T) {
	handler := MCPServerHandler()
	if handler == nil {
		t.Fatal("MCPServerHandler() returned nil")
	}
}

// TestMCPServerHandler_ServeHTTP 测试 MCP handler 的 HTTP 连通性
// 发送一个 MCP initialize 请求，验证能收到有效响应
func TestMCPServerHandler_ServeHTTP(t *testing.T) {
	handler := MCPServerHandler()

	// 构造 MCP initialize 请求
	initReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}
	body, _ := common.Marshal(initReq)

	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
	}

	// 验证响应体包含 server info
	respBody := w.Body.String()
	if !contains(respBody, "new-api-mcp") {
		t.Errorf("expected response to contain 'new-api-mcp', got: %s", respBody)
	}
}

// TestHandleMCPGenerateImage_NoGinContext 测试缺少 gin context 时返回错误
func TestHandleMCPGenerateImage_NoGinContext(t *testing.T) {
	ctx := context.Background()
	req := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Arguments: json.RawMessage(`{"prompt":"test"}`),
		},
	}

	result, err := handleMCPGenerateImage(ctx, req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError to be true when gin context is missing")
	}
}

// TestHandleMCPGenerateImage_NoPrompt 测试缺少 prompt 参数时返回错误
func TestHandleMCPGenerateImage_NoPrompt(t *testing.T) {
	// 设置带 gin context 的 MCP context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	ctx := context.WithValue(context.Background(), mcpGinContextKey, c)

	req := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Arguments: json.RawMessage(`{}`),
		},
	}

	result, err := handleMCPGenerateImage(ctx, req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError to be true when prompt is missing")
	}
}

// TestHandleMCPGenerateImage_NoModelConfigured 测试分组未配置文生图模型时返回错误
func TestHandleMCPGenerateImage_NoModelConfigured(t *testing.T) {
	// 保存原始配置
	original := mcp_setting.GetGroupImageModelSetting().GroupImageModels
	defer func() {
		mcp_setting.GetGroupImageModelSetting().GroupImageModels = original
	}()
	// 清空配置
	mcp_setting.GetGroupImageModelSetting().GroupImageModels = map[string]string{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	ctx := context.WithValue(context.Background(), mcpGinContextKey, c)

	req := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Arguments: json.RawMessage(`{"prompt":"a cat"}`),
		},
	}

	result, err := handleMCPGenerateImage(ctx, req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError to be true when no model configured")
	}
}

// TestGenerateImageInputSchema 测试输入 schema 生成
func TestGenerateImageInputSchema(t *testing.T) {
	schema := generateImageInputSchema()
	if len(schema) == 0 {
		t.Fatal("expected non-empty schema")
	}

	var parsed map[string]any
	if err := common.Unmarshal(schema, &parsed); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}

	if parsed["type"] != "object" {
		t.Errorf("expected type 'object', got %v", parsed["type"])
	}

	props, ok := parsed["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected properties to be a map")
	}
	if _, ok := props["prompt"]; !ok {
		t.Error("expected 'prompt' property in schema")
	}

	required, ok := parsed["required"].([]any)
	if !ok {
		t.Fatal("expected required to be an array")
	}
	if len(required) == 0 || required[0] != "prompt" {
		t.Error("expected 'prompt' to be required")
	}
}

// contains 检查字符串是否包含子串
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
