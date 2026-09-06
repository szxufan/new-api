package service

import (
	"crypto/hmac"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	// MCPUploadTicketTTL MCP 上传票据有效期（10 分钟，窗口内可多次上传）
	MCPUploadTicketTTL = 10 * time.Minute
	// mcpUploadTicketPurpose 票据用途前缀，防止与其他 HMAC 用途混淆
	mcpUploadTicketPurpose = "mcp-upload"
)

// GenerateMCPUploadTicket 为指定用户签发 MCP 临时图片上传票据。
// 供 MCP 工具 request_upload_ticket 调用：Agent 凭票据（无需 API 令牌）
// 调用 POST /v1/mcp-upload 上传临时图片。
// 格式: "mcp-upload:<userID>:<expUnix>.<hmac_hex>"，
// HMAC-SHA256 复用 common.GenerateHMAC（密钥 common.CryptoSecret）。
// 签发时间由 now 参数注入以便测试；有效期内票据可多次使用。
func GenerateMCPUploadTicket(userID int, now time.Time) (ticket string, expiresIn int, err error) {
	if userID <= 0 {
		return "", 0, fmt.Errorf("invalid user id: %d", userID)
	}
	exp := now.Add(MCPUploadTicketTTL).Unix()
	payload := fmt.Sprintf("%s:%d:%d", mcpUploadTicketPurpose, userID, exp)
	sig := common.GenerateHMAC(payload)
	return payload + "." + sig, int(MCPUploadTicketTTL.Seconds()), nil
}

// ValidateMCPUploadTicket 校验上传票据的签名与有效期，通过返回 userID。
// 签名使用 hmac.Equal 常量时间比较；格式、签名、过期错误分别返回明确 error。
func ValidateMCPUploadTicket(ticket string, now time.Time) (userID int, err error) {
	ticket = strings.TrimSpace(ticket)
	idx := strings.LastIndex(ticket, ".")
	if idx == -1 {
		return 0, fmt.Errorf("malformed upload ticket")
	}
	payload, sigHex := ticket[:idx], ticket[idx+1:]

	expectSig := common.GenerateHMAC(payload)
	if !hmac.Equal([]byte(sigHex), []byte(expectSig)) {
		return 0, fmt.Errorf("invalid upload ticket signature")
	}

	// payload: "<purpose>:<userID>:<expUnix>"
	segments := strings.Split(payload, ":")
	if len(segments) != 3 || segments[0] != mcpUploadTicketPurpose {
		return 0, fmt.Errorf("malformed upload ticket payload")
	}
	userID, err = strconv.Atoi(segments[1])
	if err != nil || userID <= 0 {
		return 0, fmt.Errorf("malformed upload ticket user id")
	}
	exp, err := strconv.ParseInt(segments[2], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("malformed upload ticket expiry")
	}
	if now.Unix() > exp {
		return 0, fmt.Errorf("upload ticket expired")
	}
	return userID, nil
}
