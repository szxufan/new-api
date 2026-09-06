package middleware

import (
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// MCPUploadAuth MCP 临时图片上传认证（POST /v1/mcp-upload），双模式：
//  1. 请求携带上传票据（query "ticket" 或 header "X-MCP-Upload-Ticket"，
//     由 MCP 工具 request_upload_ticket 签发）→ 校验票据，通过则注入
//     c.Set("id", userID) 放行；失败直接 401（不回退令牌认证，避免无效票据静默降级）。
//  2. 未携带票据 → 回退 TokenAuth（API 令牌路径，供人工 curl 调试）。
func MCPUploadAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		ticket := c.Query("ticket")
		if ticket == "" {
			ticket = c.GetHeader("X-MCP-Upload-Ticket")
		}
		if ticket == "" {
			// 无票据，回退 API 令牌认证
			TokenAuth()(c)
			return
		}

		userID, err := service.ValidateMCPUploadTicket(ticket, time.Now())
		if err != nil {
			abortWithOpenAiMessage(c, http.StatusUnauthorized, "invalid or expired upload ticket: "+err.Error())
			return
		}
		c.Set("id", userID)
		c.Next()
	}
}
