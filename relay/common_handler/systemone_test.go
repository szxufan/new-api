package common_handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
)

func newSystemOneTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	return c, w
}

func systemOneHTTPResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{},
	}
}

func TestSystemOneHandlerPassthroughAndUsage(t *testing.T) {
	c, w := newSystemOneTestContext()
	info := &relaycommon.RelayInfo{}
	info.SetEstimatePromptTokens(7)
	body := `{"model":"nimble","answers":{"q":{"type":"noul","noul":0.87}},"usage":{"input_tokens":13,"output_tokens":4}}`

	usage, apiErr := SystemOneHandler(c, info, systemOneHTTPResponse(body))
	if apiErr != nil {
		t.Fatalf("unexpected error: %v", apiErr)
	}
	if usage.PromptTokens != 13 {
		t.Fatalf("PromptTokens = %d, want 13", usage.PromptTokens)
	}
	if usage.CompletionTokens != 4 {
		t.Fatalf("CompletionTokens = %d, want 4", usage.CompletionTokens)
	}
	if usage.TotalTokens != 17 {
		t.Fatalf("TotalTokens = %d, want 17", usage.TotalTokens)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if got := w.Body.String(); got != body {
		t.Fatalf("response body not passed through verbatim:\ngot:  %s\nwant: %s", got, body)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
}

func TestSystemOneHandlerInputTokensFallback(t *testing.T) {
	c, _ := newSystemOneTestContext()
	info := &relaycommon.RelayInfo{}
	info.SetEstimatePromptTokens(55)
	body := `{"model":"decision-model-preview","request_id":"req-1","answers":{},"usage":{"input_tokens":0},"latency_ms":52.9}`

	usage, apiErr := SystemOneHandler(c, info, systemOneHTTPResponse(body))
	if apiErr != nil {
		t.Fatalf("unexpected error: %v", apiErr)
	}
	if usage.PromptTokens != 55 {
		t.Fatalf("PromptTokens = %d, want fallback estimate 55", usage.PromptTokens)
	}
	if usage.CompletionTokens != 0 {
		t.Fatalf("CompletionTokens = %d, want 0", usage.CompletionTokens)
	}
	if usage.TotalTokens != 55 {
		t.Fatalf("TotalTokens = %d, want 55", usage.TotalTokens)
	}
}
