package controller

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	realtimeTestDialTimeout     = 10 * time.Second
	realtimeTestExchangeTimeout = 30 * time.Second
	// realtimeResponseStatusFailed 是 response.done 中表示本次响应在上游侧执行失败的状态值
	realtimeResponseStatusFailed = "failed"
)

// testRealtimeChannel 对 realtime 端点做纯文本往返测试：
// 拨号上游 → 等 session.created → 发送文本 "hi" → 等 response.done。
// 复用 testChannel 已设置好的 gin 上下文（渠道密钥/base_url/分组等）。
// 与 HTTP 测试不同，realtime 探针不扣配额、不写消费日志。
func testRealtimeChannel(c *gin.Context, channel *model.Channel, testModel string) testResult {
	c.Request.Method = http.MethodGet
	c.Request.URL.Path = "/v1/realtime"
	c.Request.URL.RawQuery = url.Values{"model": []string{testModel}}.Encode()

	info := relaycommon.GenRelayInfoWs(c, nil)
	info.IsChannelTest = true
	info.InitChannelMeta(c)
	if err := helper.ModelMappedHelper(c, info, nil); err != nil {
		return testResult{context: c, localErr: err, newAPIError: types.NewError(err, types.ErrorCodeChannelModelMappedError)}
	}
	testModel = info.UpstreamModelName
	// 模型映射后同步 query 与 RequestURLPath（OpenAI 默认 realtime URL 透传 RequestURLPath）
	c.Request.URL.RawQuery = url.Values{"model": []string{testModel}}.Encode()
	info.RequestURLPath = c.Request.URL.String()

	apiType, _ := common.ChannelType2APIType(channel.Type)
	adaptor := relay.GetAdaptor(apiType)
	if adaptor == nil {
		err := fmt.Errorf("invalid api type: %d, adaptor is nil", apiType)
		return testResult{context: c, localErr: err, newAPIError: types.NewError(err, types.ErrorCodeInvalidApiType)}
	}
	adaptor.Init(info)

	requestURL, err := adaptor.GetRequestURL(info)
	if err != nil {
		return testResult{context: c, localErr: err, newAPIError: types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)}
	}
	// 通用守卫：realtime URL 必须为 ws/wss，否则说明该渠道类型（或 base_url 配置）不支持 realtime 测试
	if parsed, parseErr := url.Parse(requestURL); parseErr != nil || (parsed.Scheme != "ws" && parsed.Scheme != "wss") {
		err := fmt.Errorf("%s channel does not support realtime test (resolved URL %q is not ws/wss)", constant.GetChannelTypeName(channel.Type), requestURL)
		return testResult{context: c, localErr: err, newAPIError: types.NewError(err, types.ErrorCodeInvalidApiType)}
	}

	header := http.Header{}
	if err := adaptor.SetupRequestHeader(c, &header, info); err != nil {
		return testResult{context: c, localErr: err, newAPIError: types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)}
	}

	dialer := websocket.Dialer{HandshakeTimeout: realtimeTestDialTimeout}
	conn, handshakeResp, err := dialer.Dial(requestURL, header)
	if err != nil {
		detail := err.Error()
		if handshakeResp != nil {
			body, _ := io.ReadAll(io.LimitReader(handshakeResp.Body, 4096))
			_ = handshakeResp.Body.Close()
			if len(body) > 0 {
				detail = fmt.Sprintf("%s (status %d: %s)", err.Error(), handshakeResp.StatusCode, strings.TrimSpace(string(body)))
			}
		}
		err = fmt.Errorf("realtime dial failed: %s", detail)
		return testResult{context: c, localErr: err, newAPIError: types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusBadGateway)}
	}
	defer conn.Close()

	if err := runRealtimeTestExchange(conn); err != nil {
		return testResult{context: c, localErr: err, newAPIError: types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusBadGateway)}
	}
	return testResult{context: c}
}

// runRealtimeTestExchange 在已建立的上游连接上执行最小文本往返。
// 使用 OpenAI 与百炼共有的事件子集（纯文本、手动模式），无需字段适配。
func runRealtimeTestExchange(conn *websocket.Conn) error {
	deadline := time.Now().Add(realtimeTestExchangeTimeout)
	// 写入同样受整体超时约束，避免上游不读导致发送永久阻塞
	_ = conn.SetWriteDeadline(deadline)

	// 1. 等待会话建立
	if err := waitRealtimeEvent(conn, deadline, dto.RealtimeEventTypeSessionCreated, dto.RealtimeEventTypeSessionUpdated); err != nil {
		return fmt.Errorf("realtime session not established: %w", err)
	}

	// 2. 发送纯文本测试序列
	events := []map[string]any{
		{
			"type": "session.update",
			"session": map[string]any{
				"modalities":     []string{"text"},
				"turn_detection": nil,
				"instructions":   "You are a helpful assistant.",
			},
		},
		{
			"type": "conversation.item.create",
			"item": map[string]any{
				"type": "message",
				"role": "user",
				"content": []any{
					map[string]any{"type": "input_text", "text": "hi"},
				},
			},
		},
		{"type": "response.create"},
	}
	for _, event := range events {
		data, err := common.Marshal(event)
		if err != nil {
			return fmt.Errorf("marshal realtime test event: %w", err)
		}
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			return fmt.Errorf("send realtime test event: %w", err)
		}
	}

	// 3. 等待响应完成
	if err := waitRealtimeEvent(conn, deadline, dto.RealtimeEventTypeResponseDone); err != nil {
		return fmt.Errorf("realtime response not completed: %w", err)
	}
	return nil
}

// waitRealtimeEvent 读取上游事件直到命中目标类型；遇到 error 事件立即返回错误。
func waitRealtimeEvent(conn *websocket.Conn, deadline time.Time, targets ...string) error {
	targetSet := make(map[string]bool, len(targets))
	for _, target := range targets {
		targetSet[target] = true
	}
	for {
		_ = conn.SetReadDeadline(deadline)
		_, message, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		event := &dto.RealtimeEvent{}
		if err := common.Unmarshal(message, event); err != nil {
			// 无法解析的事件跳过，交由目标事件/超时裁决
			continue
		}
		if event.Type == dto.RealtimeEventTypeError {
			msg := "upstream realtime error"
			if event.Error != nil && event.Error.Message != "" {
				msg = event.Error.Message
			}
			return errors.New(msg)
		}
		if targetSet[event.Type] {
			// 上游可能不发送独立 error 事件，而是用 response.done(status=failed) 结束本次响应
			if event.Type == dto.RealtimeEventTypeResponseDone {
				if err := realtimeResponseDoneError(event.Response); err != nil {
					return err
				}
			}
			return nil
		}
	}
}

// realtimeResponseDoneError 检查 response.done 内嵌的失败状态。
// 若只看事件类型而不看 response.status，上游失败（如模型拒绝该模态）会被误判为测试通过。
func realtimeResponseDoneError(response *dto.RealtimeResponse) error {
	if response == nil || response.Status != realtimeResponseStatusFailed {
		return nil
	}
	if response.StatusDetails != nil && response.StatusDetails.Error != nil && response.StatusDetails.Error.Message != "" {
		return fmt.Errorf("upstream realtime response failed: %s", response.StatusDetails.Error.Message)
	}
	return errors.New("upstream realtime response failed")
}
