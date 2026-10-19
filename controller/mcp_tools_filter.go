package controller

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/mcp_setting"

	"github.com/gin-gonic/gin"
)

// MCP tools/list 按分组模型池 + 渠道可用性过滤：
// 调用方分组配置了场景模型、且该分组下存在声明此模型的可用渠道时，工具才出现在
// 工具列表中；其余请求原样透传。tools/call 直接点名调用未列出的工具时，
// 仍由各工具 handler 的"未配置模型 / 无可用渠道"校验拦截，过滤仅影响工具发现体验。
//
// 可见性与调用时的模型解析保持一致（mcpSceneConfiguredModel 的
// 分组 → default 回退），但不含 X-MCP-*-Model 请求头跨分组指定的能力：
// 工具被隐藏时仍可能通过请求头指定其他分组模型成功调用。
//
// 渠道可用性不含后备渠道机制（CacheGetFallbackChannel / 被禁用渠道后备链）：
// 常规候选为空但存在后备渠道时工具会被隐藏，实际调用可能仍成功。

// mcpToolScenes 工具 → 依赖的模型池场景列表（任一场景配置了模型即可见）。
// 不在表中的工具（get_video_task / request_upload_ticket）不依赖模型池，始终可见。
var mcpToolScenes = map[string][]string{
	"generate_image":                {mcpSceneImage, mcpSceneI2I},
	"generate_video":                {mcp_setting.VideoModelKindT2V},
	"generate_video_from_frames":    {mcp_setting.VideoModelKindI2V, mcp_setting.VideoModelKindKF2V},
	"generate_video_from_reference": {mcp_setting.VideoModelKindR2V},
	"web_search":                    {mcpSceneSearch},
	"web_fetch":                     {mcpSceneFetch},
}

// mcpGroupFromRequest 从注入的 gin context 读取调用方分组（与 resolveMCPModel 一致）。
// 返回 ok=false 表示请求未携带 gin context（异常装配），调用方应跳过过滤。
func mcpGroupFromRequest(r *http.Request) (string, bool) {
	ginCtx, ok := r.Context().Value(mcpGinContextKey).(*gin.Context)
	if !ok || ginCtx == nil {
		return "", false
	}
	group := common.GetContextKeyString(ginCtx, constant.ContextKeyUsingGroup)
	if group == "" {
		group = "default"
	}
	return group, true
}

// mcpModelHasChannel 判断分组下是否存在声明该模型的可用渠道。
// 与调用时 CacheGetRandomSatisfiedChannel 的常规候选来源一致
// （内存缓存 group2model2channels 或 abilities 查询，自动兼容 MemoryCacheEnabled 开关）。
func mcpModelHasChannel(group, modelName string) bool {
	return len(model.GetSortedChannelsByPriorityAndAffinity(group, modelName, nil, nil)) > 0
}

// mcpToolVisible 返回工具对指定分组是否可见：
// 依赖场景中任一场景在分组模型池中配置了模型（含 default 回退），
// 且该分组下存在此模型的可用渠道，才可见
func mcpToolVisible(group, tool string) bool {
	scenes, ok := mcpToolScenes[tool]
	if !ok || len(scenes) == 0 {
		return true
	}
	for _, scene := range scenes {
		modelName := mcpSceneConfiguredModel(scene, group)
		if modelName != "" && mcpModelHasChannel(group, modelName) {
			return true
		}
	}
	return false
}

// maxMCPFilterRequestPeek 预读请求体的上限：tools/list 请求体很小，
// 超限（异常超大请求）时按非 tools/list 处理，直接透传不过滤
const maxMCPFilterRequestPeek = 1 << 20

// mcpRequestIsToolsList 判断 JSON-RPC 请求体（单条或批量数组）是否包含 tools/list
func mcpRequestIsToolsList(body []byte) bool {
	var single struct {
		Method string `json:"method"`
	}
	if err := common.Unmarshal(body, &single); err == nil {
		return single.Method == "tools/list"
	}
	var batch []struct {
		Method string `json:"method"`
	}
	if err := common.Unmarshal(body, &batch); err == nil {
		for _, m := range batch {
			if m.Method == "tools/list" {
				return true
			}
		}
	}
	return false
}

// mcpFilterableContentType 响应是否为可过滤的形态（stateless 模式下
// tools/list 的 POST 响应为 application/json；批量请求等场景可能回退 SSE）
func mcpFilterableContentType(contentType string) bool {
	return strings.Contains(contentType, "application/json") ||
		strings.Contains(contentType, "text/event-stream")
}

// filterMCPToolsListBody 过滤响应体中的 tools/list 结果。
// 支持 application/json（单条或批量数组）与 text/event-stream（data: 行）两种形态，
// 非工具列表消息不受影响。返回 (新响应体, 是否发生变更)。
func filterMCPToolsListBody(body []byte, contentType string, group string) ([]byte, bool) {
	if !mcpFilterableContentType(contentType) {
		return body, false
	}
	if strings.Contains(contentType, "text/event-stream") {
		return filterMCPToolsListSSE(body, group)
	}
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return body, false
	}
	if trimmed[0] == '[' {
		var msgs []json.RawMessage
		if err := common.Unmarshal(trimmed, &msgs); err != nil {
			return body, false
		}
		changed := false
		for i, raw := range msgs {
			filtered, c := filterMCPToolsListMessage(raw, group)
			if c {
				msgs[i] = filtered
				changed = true
			}
		}
		if !changed {
			return body, false
		}
		out, err := common.Marshal(msgs)
		if err != nil {
			return body, false
		}
		return out, true
	}
	filtered, changed := filterMCPToolsListMessage(json.RawMessage(trimmed), group)
	if !changed {
		return body, false
	}
	return filtered, true
}

// filterMCPToolsListMessage 过滤单条 JSON-RPC 消息中 result.tools 的工具列表。
// 仅当有工具被移除时重新序列化整个消息，否则返回原始字节。
func filterMCPToolsListMessage(raw json.RawMessage, group string) (json.RawMessage, bool) {
	var msg map[string]any
	if err := common.Unmarshal(raw, &msg); err != nil {
		return raw, false
	}
	result, ok := msg["result"].(map[string]any)
	if !ok {
		return raw, false
	}
	tools, ok := result["tools"].([]any)
	if !ok {
		// 错误响应等非工具列表消息
		return raw, false
	}
	kept := make([]any, 0, len(tools))
	for _, tool := range tools {
		toolName := ""
		if toolMap, ok := tool.(map[string]any); ok {
			if name, ok := toolMap["name"].(string); ok {
				toolName = name
			}
		}
		if mcpToolVisible(group, toolName) {
			kept = append(kept, tool)
		}
	}
	if len(kept) == len(tools) {
		return raw, false
	}
	result["tools"] = kept
	out, err := common.Marshal(msg)
	if err != nil {
		return raw, false
	}
	return out, true
}

// filterMCPToolsListSSE 过滤 SSE 响应中 data: 行携带的 tools/list 结果，
// 其余行（event:、id:、注释、空行）原样保留
func filterMCPToolsListSSE(body []byte, group string) ([]byte, bool) {
	changed := false
	lines := strings.Split(string(body), "\n")
	for i, line := range lines {
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(line[len("data:"):])
		if payload == "" {
			continue
		}
		filtered, c := filterMCPToolsListMessage(json.RawMessage(payload), group)
		if c {
			lines[i] = "data: " + string(filtered)
			changed = true
		}
	}
	if !changed {
		return body, false
	}
	return []byte(strings.Join(lines, "\n")), true
}

// mcpBufferedResponseWriter 缓冲下游 handler 的完整响应，便于事后改写。
// Flush 为空实现：SDK 对 SSE 逐事件 flush，过滤场景下单条 tools/list
// 响应在 handler 返回后一次性写出，无需流式转发。
type mcpBufferedResponseWriter struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (w *mcpBufferedResponseWriter) Header() http.Header { return w.header }

func (w *mcpBufferedResponseWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}

func (w *mcpBufferedResponseWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(p)
}

func (w *mcpBufferedResponseWriter) Flush() {}

// wrapMCPToolsListFilter 包装 MCP StreamableHTTP handler：
// tools/list 请求的响应按调用方分组过滤工具列表，其余请求与方法直接透传。
func wrapMCPToolsListFilter(base http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r == nil || r.Body == nil || r.Method != http.MethodPost {
			base.ServeHTTP(w, r)
			return
		}

		reqBody, err := io.ReadAll(io.LimitReader(r.Body, maxMCPFilterRequestPeek))
		r.Body = io.NopCloser(bytes.NewReader(reqBody))
		if err != nil || !mcpRequestIsToolsList(reqBody) {
			base.ServeHTTP(w, r)
			return
		}

		group, ok := mcpGroupFromRequest(r)
		if !ok {
			base.ServeHTTP(w, r)
			return
		}

		rec := &mcpBufferedResponseWriter{header: make(http.Header)}
		base.ServeHTTP(rec, r)
		if rec.status == 0 {
			rec.status = http.StatusOK
		}

		out := rec.body.Bytes()
		if rec.status == http.StatusOK {
			if filtered, changed := filterMCPToolsListBody(out, rec.header.Get("Content-Type"), group); changed {
				out = filtered
				rec.header.Del("Content-Length")
			}
		}

		for key, values := range rec.header {
			for _, v := range values {
				w.Header().Add(key, v)
			}
		}
		w.WriteHeader(rec.status)
		_, _ = w.Write(out)
	})
}
