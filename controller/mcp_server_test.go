package controller

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
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

// TestGenerateImageInputSchema_ImageIds 测试 generate_image schema 包含 image_ids 属性
func TestGenerateImageInputSchema_ImageIds(t *testing.T) {
	var parsed map[string]any
	if err := common.Unmarshal(generateImageInputSchema(), &parsed); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}
	props, ok := parsed["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected properties to be a map")
	}
	imageIDs, ok := props["image_ids"].(map[string]any)
	if !ok {
		t.Fatal("expected 'image_ids' property in schema")
	}
	if imageIDs["maxItems"] != float64(3) {
		t.Errorf("expected image_ids maxItems 3, got %v", imageIDs["maxItems"])
	}
	if imageIDs["minItems"] != float64(1) {
		t.Errorf("expected image_ids minItems 1, got %v", imageIDs["minItems"])
	}
}

// TestGenerateVideoSchemas 测试视频工具 schema 的必填项与取值范围
func TestGenerateVideoSchemas(t *testing.T) {
	cases := []struct {
		name          string
		schema        json.RawMessage
		wantRequired  []string
		wantImageMaxN float64 // -1 表示无 image_ids 属性
	}{
		{
			name:          "generate_video",
			schema:        generateVideoInputSchema(),
			wantRequired:  []string{"prompt"},
			wantImageMaxN: -1,
		},
		{
			name:          "generate_video_from_frames",
			schema:        generateVideoFromFramesInputSchema(),
			wantRequired:  []string{"prompt", "first_frame_id"},
			wantImageMaxN: -1,
		},
		{
			name:          "generate_video_from_reference",
			schema:        generateVideoFromReferenceInputSchema(),
			wantRequired:  []string{"prompt", "image_ids"},
			wantImageMaxN: 3,
		},
		{
			name:          "get_video_task",
			schema:        getVideoTaskInputSchema(),
			wantRequired:  []string{"task_id"},
			wantImageMaxN: -1,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			var parsed map[string]any
			if err := common.Unmarshal(tt.schema, &parsed); err != nil {
				t.Fatalf("schema is not valid JSON: %v", err)
			}
			required, ok := parsed["required"].([]any)
			if !ok {
				t.Fatal("expected required to be an array")
			}
			if len(required) != len(tt.wantRequired) {
				t.Fatalf("expected %d required fields, got %d", len(tt.wantRequired), len(required))
			}
			for i, want := range tt.wantRequired {
				if required[i] != want {
					t.Errorf("required[%d] = %v, want %v", i, required[i], want)
				}
			}
			props := parsed["properties"].(map[string]any)
			if tt.wantImageMaxN >= 0 {
				imageIDs := props["image_ids"].(map[string]any)
				if imageIDs["maxItems"] != tt.wantImageMaxN {
					t.Errorf("expected image_ids maxItems %v, got %v", tt.wantImageMaxN, imageIDs["maxItems"])
				}
			}
		})
	}
}

// TestResolveMCPImageIDs_NotFound 测试解析不存在的临时图片 ID
func TestResolveMCPImageIDs_NotFound(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	c.Request.Host = "localhost:3000"

	_, err := resolveMCPImageIDs(c, []string{"nonexistent123"})
	if err == nil {
		t.Fatal("expected error for nonexistent image id, got nil")
	}
	if !contains(err.Error(), "not found or expired") {
		t.Errorf("expected 'not found or expired' in error, got: %s", err.Error())
	}
}

// TestResolveMCPImageIDs_PassthroughURL 测试直接传代理 URL 时原样透传
func TestResolveMCPImageIDs_PassthroughURL(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	urls, err := resolveMCPImageIDs(c, []string{"https://example.com/a.png"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(urls) != 1 || urls[0] != "https://example.com/a.png" {
		t.Errorf("expected passthrough URL, got %v", urls)
	}
}

// TestResolveMCPImageIDs_FromCache 测试从缓存解析已上传的临时图片 ID
func TestResolveMCPImageIDs_FromCache(t *testing.T) {
	pngBytes := []byte("\x89PNG\r\n\x1a\n0000")
	imageID, _, err := cacheBase64Image("data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes))
	if err != nil {
		t.Fatalf("failed to cache test image: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	c.Request.Host = "example.com"
	c.Request.Header.Set("X-Forwarded-Proto", "https")

	urls, err := resolveMCPImageIDs(c, []string{imageID, imageID + ".png"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(urls) != 2 {
		t.Fatalf("expected 2 urls, got %d", len(urls))
	}
	wantURL := "https://example.com/v1/mcp-image/" + imageID + ".png"
	if urls[0] != wantURL || urls[1] != wantURL {
		t.Errorf("expected proxy url %s (both with and without extension), got %v", wantURL, urls)
	}
}

// TestResolveMCPImageIDs_EmptyID 测试空 ID 返回错误
func TestResolveMCPImageIDs_EmptyID(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	if _, err := resolveMCPImageIDs(c, []string{"  "}); err == nil {
		t.Error("expected error for empty image id, got nil")
	}
}

// TestMCPUploadImage_MissingFile 测试上传缺少 file 字段返回 400
func TestMCPUploadImage_MissingFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/mcp-upload", nil)

	MCPUploadImage(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d, body: %s", w.Code, w.Body.String())
	}
}

// TestMCPUploadImage_Success 测试正常上传 PNG 文件
func TestMCPUploadImage_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 最小 PNG 头（DetectContentType 需要真实魔数）
	pngData := []byte("\x89PNG\r\n\x1a\n" + string(make([]byte, 64)))

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "test.png")
	if err != nil {
		t.Fatalf("create form file failed: %v", err)
	}
	if _, err := part.Write(pngData); err != nil {
		t.Fatalf("write form file failed: %v", err)
	}
	writer.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/mcp-upload", body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	c.Request.Host = "example.com"

	MCPUploadImage(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			ID        string `json:"id"`
			URL       string `json:"url"`
			MimeType  string `json:"mime_type"`
			Size      int    `json:"size"`
			ExpiresIn int    `json:"expires_in"`
		} `json:"data"`
	}
	if err := common.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if !resp.Success {
		t.Error("expected success to be true")
	}
	if resp.Data.ID == "" {
		t.Error("expected non-empty id")
	}
	if resp.Data.MimeType != "image/png" {
		t.Errorf("expected mime_type image/png, got %s", resp.Data.MimeType)
	}
	if resp.Data.ExpiresIn != 7200 {
		t.Errorf("expected expires_in 7200 (2h), got %d", resp.Data.ExpiresIn)
	}
	if !contains(resp.Data.URL, "/v1/mcp-image/"+resp.Data.ID) {
		t.Errorf("expected url to contain /v1/mcp-image/%s, got %s", resp.Data.ID, resp.Data.URL)
	}

	// 验证图片已入缓存且可通过 ServeMCPImage 读取
	entry, found := service.GetCachedImage(resp.Data.ID)
	if !found {
		t.Fatal("expected uploaded image to be cached")
	}
	if entry.MimeType != "image/png" {
		t.Errorf("expected cached mime image/png, got %s", entry.MimeType)
	}
}

// TestMCPUploadImage_RejectNonImage 测试上传非图片内容被拒绝
func TestMCPUploadImage_RejectNonImage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.txt")
	part.Write([]byte("this is plain text, not an image"))
	writer.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/mcp-upload", body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())

	MCPUploadImage(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for non-image upload, got %d, body: %s", w.Code, w.Body.String())
	}
}

// newVideoToolTestContext 构造带 gin context 的 MCP 工具调用环境
func newVideoToolTestContext(t *testing.T) context.Context {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	return context.WithValue(context.Background(), mcpGinContextKey, c)
}

// newVideoToolRequest 构造 MCP 工具调用请求
func newVideoToolRequest(arguments string) *mcp.CallToolRequest {
	return &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Arguments: json.RawMessage(arguments),
		},
	}
}

// TestHandleMCPGenerateVideo_NoPrompt 测试文生视频缺少 prompt
func TestHandleMCPGenerateVideo_NoPrompt(t *testing.T) {
	ctx := newVideoToolTestContext(t)
	result, err := handleMCPGenerateVideo(ctx, newVideoToolRequest(`{}`))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError when prompt missing")
	}
}

// TestHandleMCPGenerateVideo_NoModelConfigured 测试 t2v 模型池未配置时返回错误
func TestHandleMCPGenerateVideo_NoModelConfigured(t *testing.T) {
	setting := mcp_setting.GetGroupImageModelSetting()
	original := setting.GroupVideoT2VModels
	defer func() { setting.GroupVideoT2VModels = original }()
	setting.GroupVideoT2VModels = map[string]string{}

	ctx := newVideoToolTestContext(t)
	result, err := handleMCPGenerateVideo(ctx, newVideoToolRequest(`{"prompt":"a cat running"}`))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError when no t2v model configured")
	}
}

// TestHandleMCPGenerateVideoFromFrames_NoFirstFrame 测试缺少 first_frame_id
func TestHandleMCPGenerateVideoFromFrames_NoFirstFrame(t *testing.T) {
	ctx := newVideoToolTestContext(t)
	result, err := handleMCPGenerateVideoFromFrames(ctx, newVideoToolRequest(`{"prompt":"fly"}`))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError when first_frame_id missing")
	}
}

// TestHandleMCPGenerateVideoFromReference_TooManyImages 测试参考图超过 3 张
func TestHandleMCPGenerateVideoFromReference_TooManyImages(t *testing.T) {
	ctx := newVideoToolTestContext(t)
	result, err := handleMCPGenerateVideoFromReference(ctx, newVideoToolRequest(
		`{"prompt":"dance","image_ids":["a","b","c","d"]}`))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError when more than 3 reference images")
	}
}

// TestHandleMCPGenerateVideoFromReference_NoImages 测试缺少参考图
func TestHandleMCPGenerateVideoFromReference_NoImages(t *testing.T) {
	ctx := newVideoToolTestContext(t)
	result, err := handleMCPGenerateVideoFromReference(ctx, newVideoToolRequest(`{"prompt":"dance"}`))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError when image_ids missing")
	}
}

// TestHandleMCPGetVideoTask_EmptyTaskID 测试缺少 task_id
func TestHandleMCPGetVideoTask_EmptyTaskID(t *testing.T) {
	ctx := newVideoToolTestContext(t)
	result, err := handleMCPGetVideoTask(ctx, newVideoToolRequest(`{}`))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError when task_id missing")
	}
}

// TestBuildVideoToolCommonSchema_Properties 测试视频工具公共属性齐全
func TestBuildVideoToolCommonSchema_Properties(t *testing.T) {
	props := videoToolCommonSchema("desc")
	for _, key := range []string{"prompt", "duration", "size"} {
		if _, ok := props[key]; !ok {
			t.Errorf("expected %q property in common video schema", key)
		}
	}
}

// TestHandleMCPRequestUploadTicket 测试上传票据签发工具正常返回
func TestHandleMCPRequestUploadTicket(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	c.Request.Host = "example.com"
	c.Set("id", 42)
	ctx := context.WithValue(context.Background(), mcpGinContextKey, c)

	result, err := handleMCPRequestUploadTicket(ctx, newVideoToolRequest(`{}`))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.IsError {
		t.Fatalf("expected non-error result, got: %v", result.Content)
	}
	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}

	var data struct {
		UploadURL string `json:"upload_url"`
		Ticket    string `json:"ticket"`
		ExpiresIn int    `json:"expires_in"`
		Example   string `json:"example"`
	}
	if err := common.Unmarshal([]byte(textContent.Text), &data); err != nil {
		t.Fatalf("unmarshal result failed: %v", err)
	}
	if !strings.Contains(data.UploadURL, "/v1/mcp-upload") {
		t.Errorf("expected upload_url to contain /v1/mcp-upload, got %s", data.UploadURL)
	}
	if data.ExpiresIn != 600 {
		t.Errorf("expected expires_in 600 (10min), got %d", data.ExpiresIn)
	}
	if !strings.Contains(data.Example, "curl") {
		t.Errorf("expected example to contain curl, got %s", data.Example)
	}
	// 票据可校验通过且用户 ID 一致
	userID, err := service.ValidateMCPUploadTicket(data.Ticket, time.Now())
	if err != nil {
		t.Fatalf("validate returned ticket failed: %v", err)
	}
	if userID != 42 {
		t.Errorf("expected ticket user id 42, got %d", userID)
	}
}

// TestHandleMCPRequestUploadTicket_NoUser 测试认证信息缺失时返回错误
func TestHandleMCPRequestUploadTicket_NoUser(t *testing.T) {
	ctx := newVideoToolTestContext(t)
	result, err := handleMCPRequestUploadTicket(ctx, newVideoToolRequest(`{}`))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError when user id missing")
	}
}

// TestRequestUploadTicketInputSchema 测试 schema 无必填参数
func TestRequestUploadTicketInputSchema(t *testing.T) {
	var schema struct {
		Type       string   `json:"type"`
		Required   []string `json:"required"`
		Properties map[string]any
	}
	if err := common.Unmarshal(requestUploadTicketInputSchema(), &schema); err != nil {
		t.Fatalf("unmarshal schema failed: %v", err)
	}
	if schema.Type != "object" {
		t.Errorf("expected type object, got %s", schema.Type)
	}
	if len(schema.Required) != 0 {
		t.Errorf("expected no required fields, got %v", schema.Required)
	}
}
