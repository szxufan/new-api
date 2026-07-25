package helper

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func FlushWriter(c *gin.Context) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("flush panic recovered: %v", r)
		}
	}()

	if c == nil || c.Writer == nil {
		return nil
	}

	if c.Request != nil && c.Request.Context().Err() != nil {
		return fmt.Errorf("request context done: %w", c.Request.Context().Err())
	}

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return errors.New("streaming error: flusher not found")
	}

	flusher.Flush()
	return nil
}

func SetEventStreamHeaders(c *gin.Context) {
	// 检查是否已经设置过头部
	if _, exists := c.Get("event_stream_headers_set"); exists {
		return
	}

	// 设置标志，表示头部已经设置过
	c.Set("event_stream_headers_set", true)

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
}

func ClaudeData(c *gin.Context, resp dto.ClaudeResponse) error {
	jsonData, err := common.Marshal(resp)
	if err != nil {
		common.SysError("error marshalling stream response: " + err.Error())
	} else {
		c.Render(-1, common.CustomEvent{Data: fmt.Sprintf("event: %s\n", resp.Type)})
		c.Render(-1, common.CustomEvent{Data: "data: " + string(jsonData)})
	}
	_ = FlushWriter(c)
	return nil
}

func ClaudeChunkData(c *gin.Context, resp dto.ClaudeResponse, data string) {
	c.Render(-1, common.CustomEvent{Data: fmt.Sprintf("event: %s\n", resp.Type)})
	c.Render(-1, common.CustomEvent{Data: fmt.Sprintf("data: %s\n", data)})
	_ = FlushWriter(c)
}

func ResponseChunkData(c *gin.Context, resp dto.ResponsesStreamResponse, data string) {
	c.Render(-1, common.CustomEvent{Data: fmt.Sprintf("event: %s\n", resp.Type)})
	c.Render(-1, common.CustomEvent{Data: fmt.Sprintf("data: %s", data)})
	_ = FlushWriter(c)
}

func StringData(c *gin.Context, str string) error {
	if c == nil || c.Writer == nil {
		return errors.New("context or writer is nil")
	}

	if c.Request != nil && c.Request.Context().Err() != nil {
		return fmt.Errorf("request context done: %w", c.Request.Context().Err())
	}

	c.Render(-1, common.CustomEvent{Data: "data: " + str})
	return FlushWriter(c)
}

func PingData(c *gin.Context) error {
	if c == nil || c.Writer == nil {
		return errors.New("context or writer is nil")
	}

	if c.Request != nil && c.Request.Context().Err() != nil {
		return fmt.Errorf("request context done: %w", c.Request.Context().Err())
	}

	if _, err := c.Writer.Write([]byte(": PING\n\n")); err != nil {
		return fmt.Errorf("write ping data failed: %w", err)
	}
	return FlushWriter(c)
}

// RetryKeepAlive 在重试等待期间向客户端发送 SSE 注释行保活信息。
// 仅在流式请求且 SSE 响应头已设置时有效，非流式请求直接返回 nil。
// 发送格式为 SSE 注释行 `: retrying\n\n`，客户端自动忽略。
func RetryKeepAlive(c *gin.Context) error {
	if c == nil || c.Writer == nil {
		return nil
	}
	// 仅在 SSE 响应头已设置时发送保活（即流式请求且已进入 doRequest）
	if _, set := c.Get("event_stream_headers_set"); !set {
		return nil
	}
	if c.Request != nil && c.Request.Context().Err() != nil {
		return fmt.Errorf("request context done: %w", c.Request.Context().Err())
	}
	if _, err := c.Writer.Write([]byte(": retrying\n\n")); err != nil {
		return fmt.Errorf("write retry keepalive failed: %w", err)
	}
	return FlushWriter(c)
}

// IsStreamResponseCommitted 判断 SSE 流式响应是否已向客户端提交。
// 判定条件：已设置 SSE 响应头（event_stream_headers_set）且已有字节实际写出
// （保活 ping、retrying 注释行或部分数据都会触发首个 Write）。
// 一旦提交，HTTP 状态码已锁定为 200，后续错误无法再依赖状态码表达。
func IsStreamResponseCommitted(c *gin.Context) bool {
	if c == nil || c.Writer == nil {
		return false
	}
	if _, set := c.Get("event_stream_headers_set"); !set {
		return false
	}
	return c.Writer.Written()
}

// WriteStreamError 在流式响应已提交（状态码锁定为 200）后，将错误以 SSE 帧
// 的形式追加到流中。裸 JSON 不是合法 SSE 帧，会被客户端解析器按未知字段静默
// 忽略，表现为流被静默截断；封装成 SSE 帧后客户端可以正常感知错误。
// OpenAI 系格式：data: {"error": {...}} + [DONE]；Claude 格式：event: error。
func WriteStreamError(c *gin.Context, relayFormat types.RelayFormat, newAPIError *types.NewAPIError) {
	if c == nil || c.Writer == nil || newAPIError == nil {
		return
	}
	var frame string
	if relayFormat == types.RelayFormatClaude {
		errJSON, err := common.Marshal(gin.H{
			"type":  "error",
			"error": newAPIError.ToClaudeError(),
		})
		if err != nil {
			logger.LogError(c, "marshal claude stream error failed: "+err.Error())
			return
		}
		frame = "event: error\ndata: " + string(errJSON) + "\n\n"
	} else {
		errJSON, err := common.Marshal(gin.H{
			"error": newAPIError.ToOpenAIError(),
		})
		if err != nil {
			logger.LogError(c, "marshal openai stream error failed: "+err.Error())
			return
		}
		frame = "data: " + string(errJSON) + "\n\ndata: [DONE]\n\n"
	}
	if _, err := c.Writer.Write([]byte(frame)); err != nil {
		logger.LogError(c, "write stream error failed: "+err.Error())
		return
	}
	_ = FlushWriter(c)
}

func ObjectData(c *gin.Context, object interface{}) error {
	if object == nil {
		return errors.New("object is nil")
	}
	jsonData, err := common.Marshal(object)
	if err != nil {
		return fmt.Errorf("error marshalling object: %w", err)
	}
	return StringData(c, string(jsonData))
}

func Done(c *gin.Context) {
	_ = StringData(c, "[DONE]")
}

func WssString(c *gin.Context, ws *websocket.Conn, str string) error {
	if ws == nil {
		logger.LogError(c, "websocket connection is nil")
		return errors.New("websocket connection is nil")
	}
	//common.LogInfo(c, fmt.Sprintf("sending message: %s", str))
	return ws.WriteMessage(1, []byte(str))
}

func WssObject(c *gin.Context, ws *websocket.Conn, object interface{}) error {
	jsonData, err := common.Marshal(object)
	if err != nil {
		return fmt.Errorf("error marshalling object: %w", err)
	}
	if ws == nil {
		logger.LogError(c, "websocket connection is nil")
		return errors.New("websocket connection is nil")
	}
	//common.LogInfo(c, fmt.Sprintf("sending message: %s", jsonData))
	return ws.WriteMessage(1, jsonData)
}

func WssError(c *gin.Context, ws *websocket.Conn, openaiError types.OpenAIError) {
	if ws == nil {
		return
	}
	errorObj := &dto.RealtimeEvent{
		Type:    "error",
		EventId: GetLocalRealtimeID(c),
		Error:   &openaiError,
	}
	_ = WssObject(c, ws, errorObj)
}

func GetResponseID(c *gin.Context) string {
	logID := c.GetString(common.RequestIdKey)
	return fmt.Sprintf("chatcmpl-%s", logID)
}

func GetLocalRealtimeID(c *gin.Context) string {
	logID := c.GetString(common.RequestIdKey)
	return fmt.Sprintf("evt_%s", logID)
}

// GetResponsesID 生成 Responses API 格式的响应 ID
// 格式: resp_{request_id}
// 客户端可在后续请求中将此 ID 作为 previous_response_id 使用
func GetResponsesID(c *gin.Context) string {
	logID := c.GetString(common.RequestIdKey)
	return fmt.Sprintf("resp_%s", logID)
}

func GenerateStartEmptyResponse(id string, createAt int64, model string, systemFingerprint *string) *dto.ChatCompletionsStreamResponse {
	return &dto.ChatCompletionsStreamResponse{
		Id:                id,
		Object:            "chat.completion.chunk",
		Created:           createAt,
		Model:             model,
		SystemFingerprint: systemFingerprint,
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					Role:    "assistant",
					Content: common.GetPointer(""),
				},
			},
		},
	}
}

func GenerateStopResponse(id string, createAt int64, model string, finishReason string) *dto.ChatCompletionsStreamResponse {
	return &dto.ChatCompletionsStreamResponse{
		Id:                id,
		Object:            "chat.completion.chunk",
		Created:           createAt,
		Model:             model,
		SystemFingerprint: nil,
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				FinishReason: &finishReason,
			},
		},
	}
}

func GenerateFinalUsageResponse(id string, createAt int64, model string, usage dto.Usage) *dto.ChatCompletionsStreamResponse {
	return &dto.ChatCompletionsStreamResponse{
		Id:                id,
		Object:            "chat.completion.chunk",
		Created:           createAt,
		Model:             model,
		SystemFingerprint: nil,
		Choices:           make([]dto.ChatCompletionsStreamResponseChoice, 0),
		Usage:             &usage,
	}
}
