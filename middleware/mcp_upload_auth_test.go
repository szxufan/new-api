package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// setupMCPUploadAuthTest 构造应用了 MCPUploadAuth 的测试路由
func setupMCPUploadAuthTest(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/v1/mcp-upload", MCPUploadAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true, "user_id": c.GetInt("id")})
	})
	return r
}

// TestMCPUploadAuth_ValidTicketQuery 测试 query 传递有效票据放行
func TestMCPUploadAuth_ValidTicketQuery(t *testing.T) {
	ticket, _, err := service.GenerateMCPUploadTicket(42, time.Now())
	if err != nil {
		t.Fatalf("generate ticket failed: %v", err)
	}

	r := setupMCPUploadAuthTest(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp-upload?ticket="+ticket, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
	if !contains(w.Body.String(), `"user_id":42`) {
		t.Errorf("expected user_id 42 in response, got %s", w.Body.String())
	}
}

// TestMCPUploadAuth_ValidTicketHeader 测试 header 传递有效票据放行
func TestMCPUploadAuth_ValidTicketHeader(t *testing.T) {
	ticket, _, err := service.GenerateMCPUploadTicket(7, time.Now())
	if err != nil {
		t.Fatalf("generate ticket failed: %v", err)
	}

	r := setupMCPUploadAuthTest(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp-upload", nil)
	req.Header.Set("X-MCP-Upload-Ticket", ticket)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
	if !contains(w.Body.String(), `"user_id":7`) {
		t.Errorf("expected user_id 7 in response, got %s", w.Body.String())
	}
}

// TestMCPUploadAuth_InvalidTicket 测试无效票据直接 401（不回退）
func TestMCPUploadAuth_InvalidTicket(t *testing.T) {
	ticket, _, err := service.GenerateMCPUploadTicket(42, time.Now())
	if err != nil {
		t.Fatalf("generate ticket failed: %v", err)
	}
	// 篡改签名
	badTicket := ticket[:len(ticket)-1] + "0"

	r := setupMCPUploadAuthTest(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp-upload?ticket="+badTicket, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d, body: %s", w.Code, w.Body.String())
	}
}

// TestMCPUploadAuth_ExpiredTicket 测试过期票据 401
func TestMCPUploadAuth_ExpiredTicket(t *testing.T) {
	// 以过去时间签发，使票据立即过期
	ticket, _, err := service.GenerateMCPUploadTicket(42, time.Now().Add(-service.MCPUploadTicketTTL-time.Minute))
	if err != nil {
		t.Fatalf("generate ticket failed: %v", err)
	}

	r := setupMCPUploadAuthTest(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp-upload?ticket="+ticket, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d, body: %s", w.Code, w.Body.String())
	}
}

// contains 检查字符串是否包含子串
func contains(s, substr string) bool {
	return len(substr) == 0 || (len(s) >= len(substr) && indexOf(s, substr) != -1)
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// 注：无票据回退 TokenAuth 的路径依赖数据库与 Redis，不在单测覆盖范围，
// 由接口手工验证覆盖（有效 sk-xxx 令牌直连上传 → 200）。
