package service

import (
	"strings"
	"testing"
	"time"
)

// TestGenerateAndValidateTicket 测试签发→校验往返
func TestGenerateAndValidateTicket(t *testing.T) {
	now := time.Now()
	ticket, expiresIn, err := GenerateMCPUploadTicket(42, now)
	if err != nil {
		t.Fatalf("generate ticket failed: %v", err)
	}
	if expiresIn != 600 {
		t.Errorf("expected expires_in 600 (10min), got %d", expiresIn)
	}
	if !strings.HasPrefix(ticket, "mcp-upload:42:") {
		t.Errorf("unexpected ticket format: %s", ticket)
	}

	gotUserID, err := ValidateMCPUploadTicket(ticket, now)
	if err != nil {
		t.Fatalf("validate ticket failed: %v", err)
	}
	if gotUserID != 42 {
		t.Errorf("expected user id 42, got %d", gotUserID)
	}
}

// TestValidateTicket_TamperedPayload 测试篡改 payload（改 userID）被拒绝
func TestValidateTicket_TamperedPayload(t *testing.T) {
	ticket, _, err := GenerateMCPUploadTicket(42, time.Now())
	if err != nil {
		t.Fatalf("generate ticket failed: %v", err)
	}

	// 把 userID 从 42 改成 43（同长度替换，签名必然不匹配）
	tampered := strings.Replace(ticket, "mcp-upload:42:", "mcp-upload:43:", 1)
	if tampered == ticket {
		t.Fatal("expected tampered ticket to differ")
	}
	if _, err := ValidateMCPUploadTicket(tampered, time.Now()); err == nil {
		t.Error("expected error for tampered payload, got nil")
	}
}

// TestValidateTicket_TamperedSignature 测试篡改签名被拒绝
func TestValidateTicket_TamperedSignature(t *testing.T) {
	ticket, _, err := GenerateMCPUploadTicket(42, time.Now())
	if err != nil {
		t.Fatalf("generate ticket failed: %v", err)
	}

	tampered := ticket[:len(ticket)-1]
	if tampered[len(tampered)-1] == '0' {
		tampered += "1"
	} else {
		tampered += "0"
	}
	if _, err := ValidateMCPUploadTicket(tampered, time.Now()); err == nil {
		t.Error("expected error for tampered signature, got nil")
	}
}

// TestValidateTicket_Expired 测试过期票据被拒绝
func TestValidateTicket_Expired(t *testing.T) {
	now := time.Now()
	ticket, _, err := GenerateMCPUploadTicket(42, now)
	if err != nil {
		t.Fatalf("generate ticket failed: %v", err)
	}

	// 有效期内最后时刻应通过
	if _, err := ValidateMCPUploadTicket(ticket, now.Add(MCPUploadTicketTTL)); err != nil {
		t.Errorf("expected ticket valid at expiry boundary, got %v", err)
	}
	// 过期后应拒绝
	if _, err := ValidateMCPUploadTicket(ticket, now.Add(MCPUploadTicketTTL+time.Second)); err == nil {
		t.Error("expected error for expired ticket, got nil")
	}
}

// TestValidateTicket_Malformed 测试各种格式错误被拒绝
func TestValidateTicket_Malformed(t *testing.T) {
	cases := []string{
		"",                     // 空串
		"no-dot-here",          // 无 "." 分隔
		"a.b",                  // payload 格式错误
		"mcp-upload:42:abc.x",  // exp 非数字（且签名错）
		"other:42:123." + strings.Repeat("0", 64), // 错误前缀
	}
	for _, ticket := range cases {
		if _, err := ValidateMCPUploadTicket(ticket, time.Now()); err == nil {
			t.Errorf("expected error for malformed ticket %q, got nil", ticket)
		}
	}
}

// TestGenerateTicket_InvalidUser 测试非法 userID 签发被拒绝
func TestGenerateTicket_InvalidUser(t *testing.T) {
	for _, uid := range []int{0, -1} {
		if _, _, err := GenerateMCPUploadTicket(uid, time.Now()); err == nil {
			t.Errorf("expected error for invalid user id %d, got nil", uid)
		}
	}
}
