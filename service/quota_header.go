package service

import (
	"bytes"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// ConsumedAmountHeader 非流式响应中返回本次消耗金额的响应头（token 开启 return_consumed_quota 时生效）
const ConsumedAmountHeader = "X-New-Api-Consumed-Amount"

// FormatQuotaAmount 将 quota 换算为美元金额字符串（quota / QuotaPerUnit，6 位小数）
func FormatQuotaAmount(quota int) string {
	return decimal.NewFromInt(int64(quota)).Div(decimal.NewFromFloat(common.QuotaPerUnit)).StringFixed(6)
}

// bufferedResponseWriter 缓冲非流式响应的状态码与 body（header 共享原 writer 的 map），
// 以便在响应真正发给客户端之前注入计费响应头，再一次性冲刷。
type bufferedResponseWriter struct {
	gin.ResponseWriter
	buf     bytes.Buffer
	status  int
	size    int
	wrote   bool
	flushed bool
	orig    gin.ResponseWriter
}

const bufferedNoWritten = -1

func newBufferedResponseWriter(orig gin.ResponseWriter) *bufferedResponseWriter {
	return &bufferedResponseWriter{ResponseWriter: orig, orig: orig, status: 0, size: bufferedNoWritten}
}

// WriteHeader 仅记录状态码，不委托原 writer，避免提前提交 headers。
// 与 gin 一致：忽略非正数状态码（SSE 场景 Render(-1) 会传入 -1）。
func (w *bufferedResponseWriter) WriteHeader(code int) {
	if code > 0 && w.status != code {
		w.status = code
	}
}

// WriteHeaderNow 缓冲期间不提交（gin 会在首次 Write 时调用）
func (w *bufferedResponseWriter) WriteHeaderNow() {}

func (w *bufferedResponseWriter) Write(data []byte) (int, error) {
	w.wrote = true
	if w.status <= 0 {
		w.status = http.StatusOK
	}
	n, err := w.buf.Write(data)
	if w.size == bufferedNoWritten {
		w.size = 0
	}
	w.size += n
	return n, err
}

func (w *bufferedResponseWriter) WriteString(data string) (int, error) {
	w.wrote = true
	if w.status <= 0 {
		w.status = http.StatusOK
	}
	n, err := w.buf.WriteString(data)
	if w.size == bufferedNoWritten {
		w.size = 0
	}
	w.size += n
	return n, err
}

// Flush 缓冲期间不冲刷，防止向客户端提前提交空响应
func (w *bufferedResponseWriter) Flush() {}

func (w *bufferedResponseWriter) Written() bool {
	return w.wrote || w.size != bufferedNoWritten
}

func (w *bufferedResponseWriter) Status() int {
	if w.status <= 0 {
		return http.StatusOK
	}
	return w.status
}

func (w *bufferedResponseWriter) Size() int {
	return w.size
}

// FlushBufferToClient 将缓冲的响应写入原 writer 并恢复 c.Writer（幂等）。
// 未产生任何输出时不写，交由上层（如错误处理 defer）决定响应。
func (w *bufferedResponseWriter) FlushBufferToClient(c *gin.Context) {
	w.flushBufferToClient(c, false)
}

// flushBufferToClient force=true 时即使缓冲为空也写出（已注入计费响应头，需要发送）
func (w *bufferedResponseWriter) flushBufferToClient(c *gin.Context, force bool) {
	if w.flushed {
		return
	}
	w.flushed = true
	if c.Writer == gin.ResponseWriter(w) {
		c.Writer = w.orig
	}
	// 未产生任何输出时不写，交由上层（如错误处理 defer）决定响应
	if !force && !w.wrote && w.buf.Len() == 0 {
		return
	}
	status := w.status
	if status <= 0 {
		status = http.StatusOK
	}
	w.orig.WriteHeader(status)
	_, _ = w.orig.Write(w.buf.Bytes())
	w.orig.Flush()
}

// MaybeInstallQuotaHeaderBuffer 当 token 开启"响应返回消耗金额"且为非流式 HTTP 请求时，
// 用缓冲 writer 包装 c.Writer，延迟到结算金额确定后再注入响应头并冲刷。
func MaybeInstallQuotaHeaderBuffer(c *gin.Context, relayInfo *relaycommon.RelayInfo, relayFormat types.RelayFormat) {
	if relayFormat == types.RelayFormatOpenAIRealtime {
		return
	}
	if !common.GetContextKeyBool(c, constant.ContextKeyTokenReturnConsumedQuota) {
		return
	}
	if relayInfo.IsStream {
		return
	}
	c.Writer = newBufferedResponseWriter(c.Writer)
}

// FlushBufferedQuotaHeaderResponse 冲刷缓冲的响应（幂等；未安装缓冲时 no-op）。
// 由 controller 层 defer 兜底调用，正常路径已在结算函数内随金额响应头提前冲刷。
func FlushBufferedQuotaHeaderResponse(c *gin.Context) {
	if bw, ok := c.Writer.(*bufferedResponseWriter); ok {
		bw.FlushBufferToClient(c)
	}
}

// setConsumedAmountHeader 在缓冲响应上注入消耗金额响应头并立即冲刷给客户端。
// 仅当响应仍处于缓冲状态（token 开启 + 非流式且尚未冲刷）时生效，否则 no-op。
func setConsumedAmountHeader(ctx *gin.Context, amount string) {
	bw, ok := ctx.Writer.(*bufferedResponseWriter)
	if !ok || bw.flushed {
		return
	}
	bw.Header().Set(ConsumedAmountHeader, amount)
	bw.flushBufferToClient(ctx, true)
}
