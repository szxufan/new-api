package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func RelayPanicRecover() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				common.SysLog(fmt.Sprintf("panic detected: %v", err))
				common.SysLog(fmt.Sprintf("stacktrace from panic: %s", string(debug.Stack())))
				// 流式响应已提交（HTTP 200 已发出）时无法返回 500 状态码，
				// 裸 JSON 会被 SSE 客户端静默忽略，改为向流内写入 SSE 错误帧
				if helper.IsStreamResponseCommitted(c) {
					helper.WriteStreamError(c, types.RelayFormatOpenAI,
						types.NewError(fmt.Errorf("panic detected, error: %v", err), types.ErrorCodeDoRequestFailed))
					c.Abort()
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": gin.H{
						"message": fmt.Sprintf("Panic detected, error: %v. Please submit a issue here: https://github.com/Calcium-Ion/new-api", err),
						"type":    "new_api_panic",
					},
				})
				c.Abort()
			}
		}()
		c.Next()
	}
}
