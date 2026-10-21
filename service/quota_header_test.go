package service

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestFormatQuotaAmount(t *testing.T) {
	// 默认 QuotaPerUnit = 500000：1230 quota = 0.002460 USD
	if got := FormatQuotaAmount(1230); got != "0.002460" {
		t.Fatalf("FormatQuotaAmount(1230) = %q, want %q", got, "0.002460")
	}
	if got := FormatQuotaAmount(0); got != "0.000000" {
		t.Fatalf("FormatQuotaAmount(0) = %q, want %q", got, "0.000000")
	}
}

func TestBufferedResponseWriterInjectsHeaderOnFlush(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	orig := c.Writer
	c.Writer = newBufferedResponseWriter(orig)

	// 模拟 handler 写响应：header 设置在共享 map 上，body 进缓冲
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(200)
	if _, err := c.Writer.Write([]byte(`{"ok":true}`)); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// 结算确定金额后注入响应头并冲刷
	setConsumedAmountHeader(c, "0.002460")

	if c.Writer != orig {
		t.Fatal("c.Writer not restored after flush")
	}
	if got := rec.Header().Get(ConsumedAmountHeader); got != "0.002460" {
		t.Fatalf("missing %s header: %q", ConsumedAmountHeader, got)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("upstream Content-Type header lost: %q", got)
	}
	body := strings.TrimSpace(rec.Body.String())
	if body != `{"ok":true}` {
		t.Fatalf("body = %q, want %q", body, `{"ok":true}`)
	}
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestBufferedResponseWriterSkipsEmptyAndKeepsErrorPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	orig := c.Writer
	c.Writer = newBufferedResponseWriter(orig)

	// 失败路径：handler 未写任何内容，兜底冲刷应跳过，
	// 随后错误处理 defer 仍能以正确状态码写错误响应
	FlushBufferedQuotaHeaderResponse(c)
	if c.Writer != orig {
		t.Fatal("c.Writer not restored")
	}
	if rec.Body.Len() != 0 || rec.Code != 200 {
		t.Fatalf("unexpected early write: code=%d body=%q", rec.Code, rec.Body.String())
	}
	c.JSON(502, gin.H{"error": "bad gateway"})
	if rec.Code != 502 {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
}

func TestBufferedResponseWriterFlushIsIdempotent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Writer = newBufferedResponseWriter(c.Writer)
	_, _ = c.Writer.Write([]byte("hello"))
	setConsumedAmountHeader(c, "0.000002")
	FlushBufferedQuotaHeaderResponse(c) // 二次冲刷应为 no-op
	body := rec.Body.String()
	if body != "hello" {
		t.Fatalf("body = %q, want %q", body, "hello")
	}
}
