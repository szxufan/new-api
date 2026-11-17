package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

// dialRealtimeTestServer 起一个本地 WebSocket 上游并返回客户端连接，
// handler 在服务端连接上模拟上游事件行为。
func dialRealtimeTestServer(t *testing.T, handler func(conn *websocket.Conn)) *websocket.Conn {
	t.Helper()
	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		handler(conn)
	}))
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// writeEvent 在模拟上游的 handler goroutine 中写事件。
// 这里不能用 require（FailNow 只能在测试 goroutine 调用），写失败仅记录，交由客户端读取超时/失败裁决。
func writeEvent(t *testing.T, conn *websocket.Conn, payload string) {
	t.Helper()
	if err := conn.WriteMessage(websocket.TextMessage, []byte(payload)); err != nil {
		t.Logf("write realtime test event failed: %v", err)
	}
}

// TestRunRealtimeTestExchange_Success 上游正常完成 session.created → response.done 往返
func TestRunRealtimeTestExchange_Success(t *testing.T) {
	conn := dialRealtimeTestServer(t, func(server *websocket.Conn) {
		writeEvent(t, server, `{"type":"session.created","session":{"id":"s1"}}`)
		for {
			_, msg, err := server.ReadMessage()
			if err != nil {
				return
			}
			var event struct {
				Type string `json:"type"`
			}
			if err := common.Unmarshal(msg, &event); err != nil {
				continue
			}
			if event.Type == "response.create" {
				writeEvent(t, server, `{"type":"response.created","response":{"id":"r1"}}`)
				writeEvent(t, server, `{"type":"response.text.delta","delta":"hello"}`)
				writeEvent(t, server, `{"type":"response.done","response":{"status":"completed"}}`)
				return
			}
		}
	})

	require.NoError(t, runRealtimeTestExchange(conn))
}

// TestRunRealtimeTestExchange_UpstreamError 上游返回 error 事件时测试失败并携带错误信息
func TestRunRealtimeTestExchange_UpstreamError(t *testing.T) {
	conn := dialRealtimeTestServer(t, func(server *websocket.Conn) {
		writeEvent(t, server, `{"type":"session.created","session":{"id":"s1"}}`)
		writeEvent(t, server, `{"type":"error","error":{"message":"invalid model","type":"invalid_request_error"}}`)
		// 保持连接打开并排空客户端事件，确保客户端能读到上面的 error 事件
		for {
			if _, _, err := server.ReadMessage(); err != nil {
				return
			}
		}
	})

	err := runRealtimeTestExchange(conn)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid model")
}

// TestRunRealtimeTestExchange_ResponseFailed 上游未发送 error 事件，
// 但以 response.done(status=failed) 结束响应时，测试应失败并透出 status_details 中的错误信息
func TestRunRealtimeTestExchange_ResponseFailed(t *testing.T) {
	conn := dialRealtimeTestServer(t, func(server *websocket.Conn) {
		writeEvent(t, server, `{"type":"session.created","session":{"id":"s1"}}`)
		for {
			_, msg, err := server.ReadMessage()
			if err != nil {
				return
			}
			var event struct {
				Type string `json:"type"`
			}
			if err := common.Unmarshal(msg, &event); err != nil {
				continue
			}
			if event.Type == "response.create" {
				writeEvent(t, server, `{"type":"response.done","response":{"status":"failed","status_details":{"type":"failed","error":{"message":"modalities text is not supported","type":"invalid_request_error"}}}}`)
				return
			}
		}
	})

	err := runRealtimeTestExchange(conn)
	require.Error(t, err)
	require.Contains(t, err.Error(), "modalities text is not supported")
}

// TestRunRealtimeTestExchange_SessionNotEstablished 上游未发送 session.created 即关闭连接
func TestRunRealtimeTestExchange_SessionNotEstablished(t *testing.T) {
	conn := dialRealtimeTestServer(t, func(server *websocket.Conn) {
		// 不发送任何事件，直接关闭
	})

	err := runRealtimeTestExchange(conn)
	require.Error(t, err)
	require.Contains(t, err.Error(), "session not established")
}
