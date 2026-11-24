package ali

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

func TestBuildAliInferenceURL(t *testing.T) {
	cases := []struct {
		name    string
		baseURL string
		want    string
		wantErr bool
	}{
		{"https default path", "https://dashscope.aliyuncs.com", "wss://dashscope.aliyuncs.com/api-ws/v1/inference", false},
		{"trailing slash", "https://dashscope.aliyuncs.com/", "wss://dashscope.aliyuncs.com/api-ws/v1/inference", false},
		{"workspace domain", "https://ws-id.cn-beijing.maas.aliyuncs.com", "wss://ws-id.cn-beijing.maas.aliyuncs.com/api-ws/v1/inference", false},
		{"custom path preserved", "https://dashscope.aliyuncs.com/custom-ws", "wss://dashscope.aliyuncs.com/custom-ws", false},
		{"wss scheme kept", "wss://dashscope.aliyuncs.com", "wss://dashscope.aliyuncs.com/api-ws/v1/inference", false},
		{"http to ws", "http://127.0.0.1:8080", "ws://127.0.0.1:8080/api-ws/v1/inference", false},
		{"empty", "", "", true},
		{"missing host", "https://", "", true},
		{"unsupported scheme", "ftp://dashscope.aliyuncs.com", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := buildAliInferenceURL(tc.baseURL)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestAdaptor_GetRequestURL_DashScopeInference(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "https://dashscope.aliyuncs.com",
			UpstreamModelName: "sambert-zhichu-v1",
		},
		RelayMode:   constant.RelayModeDashScopeInference,
		RelayFormat: types.RelayFormatDashScopeInference,
	}

	url, err := adaptor.GetRequestURL(info)
	assert.NoError(t, err)
	assert.Equal(t, "wss://dashscope.aliyuncs.com/api-ws/v1/inference", url)
}

func TestRewriteInferenceRunTaskModel(t *testing.T) {
	runTask := `{"header":{"action":"run-task","task_id":"t-1","streaming":"out"},"payload":{"model":"my-alias","task_group":"audio","task":"tts","function":"SpeechSynthesizer","input":{"text":"你好"},"parameters":{"format":"mp3","sample_rate":16000}}}`

	t.Run("rewrites model and preserves other fields", func(t *testing.T) {
		out := rewriteInferenceRunTaskModel([]byte(runTask), "sambert-zhichu-v1")
		var frame map[string]any
		assert.NoError(t, common.Unmarshal(out, &frame))
		payload := frame["payload"].(map[string]any)
		assert.Equal(t, "sambert-zhichu-v1", payload["model"])
		assert.Equal(t, "audio", payload["task_group"])
		assert.Equal(t, "SpeechSynthesizer", payload["function"])
		assert.Equal(t, "你好", payload["input"].(map[string]any)["text"])
		assert.Equal(t, "mp3", payload["parameters"].(map[string]any)["format"])
		header := frame["header"].(map[string]any)
		assert.Equal(t, "t-1", header["task_id"])
		assert.Equal(t, "out", header["streaming"])
	})

	t.Run("same model untouched", func(t *testing.T) {
		out := rewriteInferenceRunTaskModel([]byte(runTask), "my-alias")
		assert.Equal(t, runTask, string(out))
	})

	t.Run("preserves large integers and raw bytes outside payload.model", func(t *testing.T) {
		// 超过 2^53 的整数若经 map[string]any 往返会被 float64 静默截断
		msg := `{"header":{"action":"run-task","task_id":"t-1"},"payload":{"model":"my-alias","parameters":{"seed":9007199254740993,"rate":1e-7}}}`
		out := rewriteInferenceRunTaskModel([]byte(msg), "sambert-zhichu-v1")
		assert.Contains(t, string(out), `"seed":9007199254740993`)
		assert.Contains(t, string(out), `"rate":1e-7`)
		assert.Contains(t, string(out), `"model":"sambert-zhichu-v1"`)
		assert.NotContains(t, string(out), "my-alias")
	})

	t.Run("missing model untouched", func(t *testing.T) {
		msg := `{"header":{"action":"run-task","task_id":"t-1"},"payload":{"input":{"text":"你好"}}}`
		out := rewriteInferenceRunTaskModel([]byte(msg), "sambert-zhichu-v1")
		assert.Equal(t, msg, string(out))
	})

	t.Run("non run-task untouched", func(t *testing.T) {
		msg := `{"header":{"action":"continue-task","task_id":"t-1"},"payload":{"model":"my-alias","input":{"text":"你好"}}}`
		out := rewriteInferenceRunTaskModel([]byte(msg), "sambert-zhichu-v1")
		assert.Equal(t, msg, string(out))
	})

	t.Run("invalid json untouched", func(t *testing.T) {
		msg := `not-json`
		out := rewriteInferenceRunTaskModel([]byte(msg), "sambert-zhichu-v1")
		assert.Equal(t, msg, string(out))
	})
}

func TestInferenceRunTaskText(t *testing.T) {
	msg := `{"header":{"action":"run-task","task_id":"t-1"},"payload":{"model":"sambert-zhichu-v1","input":{"text":"你好世界abc"}}}`
	taskID, characters, ok := inferenceRunTaskText([]byte(msg))
	assert.True(t, ok)
	assert.Equal(t, "t-1", taskID)
	assert.Equal(t, 7, characters)

	_, _, ok = inferenceRunTaskText([]byte(`{"header":{"action":"finish-task","task_id":"t-1"}}`))
	assert.False(t, ok)
}

func TestParseInferenceServerFrame(t *testing.T) {
	finished := `{"header":{"task_id":"t-1","event":"task-finished","attributes":{"request_uuid":"u-1"}},"payload":{"usage":{"characters":13}}}`
	frame, ok := parseInferenceServerFrame([]byte(finished))
	assert.True(t, ok)
	assert.Equal(t, "task-finished", frame.Header.Event)
	assert.Equal(t, "t-1", frame.Header.TaskID)
	assert.NotNil(t, frame.Payload.Usage)
	assert.Equal(t, 13, frame.Payload.Usage.Characters)

	failed := `{"header":{"task_id":"t-1","event":"task-failed","error_code":"InvalidParameter","error_message":"bad"},"payload":{}}`
	frame, ok = parseInferenceServerFrame([]byte(failed))
	assert.True(t, ok)
	assert.Equal(t, "task-failed", frame.Header.Event)
	assert.Equal(t, "InvalidParameter", frame.Header.ErrorCode)
	assert.Equal(t, "bad", frame.Header.ErrorMessage)
	assert.Nil(t, frame.Payload.Usage)
}

func TestAliInferenceHandler_AppliesModelMapping(t *testing.T) {
	c := newAliSpeechContext()
	c.Set("model_mapping", `{"my-alias":"sambert-zhichu-v1"}`)
	info := &relaycommon.RelayInfo{
		ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: "my-alias"},
		OriginModelName: "my-alias",
	}

	// 连接为空时泵返回错误，但模型映射应已应用
	apiErr, _ := AliInferenceHandler(c, info)
	assert.NotNil(t, apiErr)
	assert.Equal(t, "sambert-zhichu-v1", info.UpstreamModelName)
	assert.True(t, info.IsModelMapped)
}

// ----- 泵集成测试（本地 WebSocket 对，无数据库依赖） -----

var testInferenceUpgrader = websocket.Upgrader{
	CheckOrigin: func(*http.Request) bool { return true },
}

func wsURL(httpURL string) string {
	return "ws" + strings.TrimPrefix(httpURL, "http")
}

func inferenceRunTaskMessage(taskID, model, text string) string {
	msg, _ := common.Marshal(map[string]any{
		"header": map[string]any{"action": "run-task", "task_id": taskID, "streaming": "out"},
		"payload": map[string]any{
			"model":      model,
			"task_group": "audio",
			"task":       "tts",
			"function":   "SpeechSynthesizer",
			"input":      map[string]any{"text": text},
			"parameters": map[string]any{"format": "mp3", "sample_rate": 16000},
		},
	})
	return string(msg)
}

// fakeInferenceUpstream 模拟 DashScope 上游：每个 run-task 回 task-started + 二进制音频帧 + task-finished。
// withUsage=false 时 task-finished 不携带 usage（验证本地字符数兜底）；
// failTask=true 时回 task-failed；holdTask=true 时只回 task-started（验证连接中断的未完成任务结算）。
func fakeInferenceUpstream(t *testing.T, withUsage, failTask, holdTask bool, characters int) (*httptest.Server, chan map[string]any) {
	t.Helper()
	received := make(chan map[string]any, 8)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := testInferenceUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			mt, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if mt != websocket.TextMessage {
				continue
			}
			var frame map[string]any
			if err := common.Unmarshal(msg, &frame); err != nil {
				continue
			}
			header, _ := frame["header"].(map[string]any)
			if header["action"] != "run-task" {
				continue
			}
			received <- frame
			taskID, _ := header["task_id"].(string)
			writeEvent := func(event string, payload map[string]any, extraHeader map[string]any) {
				h := map[string]any{"task_id": taskID, "event": event}
				for k, v := range extraHeader {
					h[k] = v
				}
				_ = conn.WriteJSON(map[string]any{"header": h, "payload": payload})
			}
			writeEvent("task-started", map[string]any{}, nil)
			if holdTask {
				continue
			}
			if failTask {
				writeEvent("task-failed", map[string]any{}, map[string]any{
					"error_code": "InvalidParameter", "error_message": "mock failure",
				})
				continue
			}
			_ = conn.WriteMessage(websocket.BinaryMessage, []byte{0x01, 0x02, 0x03, 0x04})
			payload := map[string]any{}
			if withUsage {
				payload["usage"] = map[string]any{"characters": characters}
			}
			writeEvent("task-finished", payload, nil)
		}
	}))
	t.Cleanup(server.Close)
	return server, received
}

// startInferencePump 建立 客户端↔网关↔上游 链路并启动泵，返回客户端连接与结果通道。
func startInferencePump(t *testing.T, upstreamURL, upstreamModel string) (*websocket.Conn, chan struct {
	apiErr *types.NewAPIError
	usage  *dto.RealtimeUsage
}) {
	return startInferencePumpWithOptions(t, upstreamURL, upstreamModel, inferencePumpTestOptions{})
}

type inferencePumpTestOptions struct {
	reserve          func(*dto.RealtimeUsage) error
	firstMessageType int
	firstMessage     []byte
}

func startInferencePumpWithOptions(t *testing.T, upstreamURL, upstreamModel string, options inferencePumpTestOptions) (*websocket.Conn, chan struct {
	apiErr *types.NewAPIError
	usage  *dto.RealtimeUsage
}) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	targetConn, _, err := websocket.DefaultDialer.Dial(wsURL(upstreamURL), nil)
	assert.NoError(t, err)

	clientConnCh := make(chan *websocket.Conn, 1)
	front := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := testInferenceUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		clientConnCh <- conn
	}))
	t.Cleanup(front.Close)

	clientConn, _, err := websocket.DefaultDialer.Dial(wsURL(front.URL), nil)
	assert.NoError(t, err)
	assert.NoError(t, clientConn.SetReadDeadline(time.Now().Add(10*time.Second)))

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/api-ws/v1/inference?model="+upstreamModel, nil)

	info := &relaycommon.RelayInfo{
		ChannelMeta:        &relaycommon.ChannelMeta{UpstreamModelName: upstreamModel},
		ClientWs:           <-clientConnCh,
		TargetWs:           targetConn,
		FirstWsMessageType: options.firstMessageType,
		FirstWsMessage:     options.firstMessage,
	}

	done := make(chan struct {
		apiErr *types.NewAPIError
		usage  *dto.RealtimeUsage
	}, 1)
	go func() {
		apiErr, usage := aliInferencePumpWithReserve(c, info, options.reserve)
		done <- struct {
			apiErr *types.NewAPIError
			usage  *dto.RealtimeUsage
		}{apiErr, usage}
	}()
	return clientConn, done
}

func readInferenceEvent(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()
	mt, msg, err := conn.ReadMessage()
	assert.NoError(t, err)
	assert.Equal(t, websocket.TextMessage, mt)
	var frame map[string]any
	assert.NoError(t, common.Unmarshal(msg, &frame))
	return frame
}

func waitInferencePump(t *testing.T, done chan struct {
	apiErr *types.NewAPIError
	usage  *dto.RealtimeUsage
}) *dto.RealtimeUsage {
	t.Helper()
	select {
	case res := <-done:
		assert.Nil(t, res.apiErr)
		return res.usage
	case <-time.After(10 * time.Second):
		t.Fatal("inference pump did not exit in time")
		return nil
	}
}

func TestAliInferencePump_PassthroughAndUsage(t *testing.T) {
	upstream, received := fakeInferenceUpstream(t, true, false, false, 13)
	clientConn, done := startInferencePump(t, upstream.URL, "sambert-zhichu-v1")

	// 任务 1：客户端使用映射前的模型名
	assert.NoError(t, clientConn.WriteMessage(websocket.TextMessage, []byte(inferenceRunTaskMessage("task-1", "my-tts-alias", "你好世界"))))

	frame := readInferenceEvent(t, clientConn)
	assert.Equal(t, "task-started", frame["header"].(map[string]any)["event"])

	mt, audio, err := clientConn.ReadMessage()
	assert.NoError(t, err)
	assert.Equal(t, websocket.BinaryMessage, mt)
	assert.Equal(t, []byte{0x01, 0x02, 0x03, 0x04}, audio)

	frame = readInferenceEvent(t, clientConn)
	assert.Equal(t, "task-finished", frame["header"].(map[string]any)["event"])

	// 上游收到的 run-task 模型已被改写，其余字段保留
	got := <-received
	payload := got["payload"].(map[string]any)
	assert.Equal(t, "sambert-zhichu-v1", payload["model"])
	assert.Equal(t, "你好世界", payload["input"].(map[string]any)["text"])
	assert.Equal(t, "mp3", payload["parameters"].(map[string]any)["format"])

	// 任务 2：复用连接
	assert.NoError(t, clientConn.WriteMessage(websocket.TextMessage, []byte(inferenceRunTaskMessage("task-2", "sambert-zhichu-v1", "床前明月光"))))
	readInferenceEvent(t, clientConn) // task-started
	_, _, err = clientConn.ReadMessage()
	assert.NoError(t, err)            // binary
	readInferenceEvent(t, clientConn) // task-finished
	<-received

	// 客户端正常关闭，泵退出并按上游 usage 累计计费
	assert.NoError(t, clientConn.WriteControl(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second)))
	assert.NoError(t, clientConn.Close())

	usage := waitInferencePump(t, done)
	assert.Equal(t, 26, usage.TotalTokens)
	assert.Equal(t, 26, usage.InputTokens)
	assert.Equal(t, 26, usage.InputTokenDetails.TextTokens)
	assert.Equal(t, 0, usage.OutputTokens)
}

func TestAliInferencePump_LocalCharactersFallback(t *testing.T) {
	upstream, _ := fakeInferenceUpstream(t, false, false, false, 0)
	clientConn, done := startInferencePump(t, upstream.URL, "sambert-zhichu-v1")

	assert.NoError(t, clientConn.WriteMessage(websocket.TextMessage, []byte(inferenceRunTaskMessage("task-1", "sambert-zhichu-v1", "你好世界"))))
	readInferenceEvent(t, clientConn) // task-started
	_, _, err := clientConn.ReadMessage()
	assert.NoError(t, err)            // binary
	readInferenceEvent(t, clientConn) // task-finished（无 usage）

	assert.NoError(t, clientConn.Close())
	usage := waitInferencePump(t, done)
	// 回退到 run-task 输入文本的本地字符数
	assert.Equal(t, 4, usage.TotalTokens)
	assert.Equal(t, 4, usage.InputTokenDetails.TextTokens)
}

func TestAliInferencePump_TaskFailedNotBilled(t *testing.T) {
	upstream, _ := fakeInferenceUpstream(t, true, true, false, 13)
	clientConn, done := startInferencePump(t, upstream.URL, "sambert-zhichu-v1")

	assert.NoError(t, clientConn.WriteMessage(websocket.TextMessage, []byte(inferenceRunTaskMessage("task-1", "sambert-zhichu-v1", "你好世界"))))
	readInferenceEvent(t, clientConn) // task-started
	frame := readInferenceEvent(t, clientConn)
	header := frame["header"].(map[string]any)
	assert.Equal(t, "task-failed", header["event"])
	assert.Equal(t, "InvalidParameter", header["error_code"])
	assert.Equal(t, "mock failure", header["error_message"])

	assert.NoError(t, clientConn.Close())
	usage := waitInferencePump(t, done)
	assert.Equal(t, 0, usage.TotalTokens)
}

func TestAliInferencePump_PendingTaskBilledOnDisconnect(t *testing.T) {
	upstream, _ := fakeInferenceUpstream(t, true, false, true, 13)
	clientConn, done := startInferencePump(t, upstream.URL, "sambert-zhichu-v1")

	assert.NoError(t, clientConn.WriteMessage(websocket.TextMessage, []byte(inferenceRunTaskMessage("task-1", "sambert-zhichu-v1", "你好世界"))))
	readInferenceEvent(t, clientConn) // task-started，上游挂起任务不结束

	// 客户端中断连接，未完成任务按本地字符数计入用量
	assert.NoError(t, clientConn.Close())
	usage := waitInferencePump(t, done)
	assert.Equal(t, 4, usage.TotalTokens)
	assert.Equal(t, 4, usage.InputTokenDetails.TextTokens)
}

func TestAliInferencePump_FirstMessageForwarded(t *testing.T) {
	upstream, received := fakeInferenceUpstream(t, true, false, false, 4)
	firstMessage := []byte(inferenceRunTaskMessage("task-1", "my-tts-alias", "你好世界"))
	clientConn, done := startInferencePumpWithOptions(t, upstream.URL, "sambert-zhichu-v1", inferencePumpTestOptions{
		firstMessageType: websocket.TextMessage,
		firstMessage:     firstMessage,
	})

	readInferenceEvent(t, clientConn) // task-started
	_, _, err := clientConn.ReadMessage()
	assert.NoError(t, err)            // binary
	readInferenceEvent(t, clientConn) // task-finished

	got := <-received
	assert.Equal(t, "sambert-zhichu-v1", got["payload"].(map[string]any)["model"])
	assert.NoError(t, clientConn.Close())
	usage := waitInferencePump(t, done)
	assert.Equal(t, 4, usage.TotalTokens)
}

func TestAliInferencePump_ReserveUsesCumulativeUsage(t *testing.T) {
	upstream, _ := fakeInferenceUpstream(t, false, false, false, 0)
	var reserved []int
	clientConn, done := startInferencePumpWithOptions(t, upstream.URL, "sambert-zhichu-v1", inferencePumpTestOptions{
		reserve: func(usage *dto.RealtimeUsage) error {
			reserved = append(reserved, usage.TotalTokens)
			return nil
		},
	})

	assert.NoError(t, clientConn.WriteMessage(websocket.TextMessage, []byte(inferenceRunTaskMessage("task-1", "sambert-zhichu-v1", "你好世界"))))
	readInferenceEvent(t, clientConn)
	_, _, err := clientConn.ReadMessage()
	assert.NoError(t, err)
	readInferenceEvent(t, clientConn)

	assert.NoError(t, clientConn.WriteMessage(websocket.TextMessage, []byte(inferenceRunTaskMessage("task-2", "sambert-zhichu-v1", "床前明月光"))))
	readInferenceEvent(t, clientConn)
	_, _, err = clientConn.ReadMessage()
	assert.NoError(t, err)
	readInferenceEvent(t, clientConn)

	assert.NoError(t, clientConn.Close())
	usage := waitInferencePump(t, done)
	assert.Equal(t, []int{4, 9}, reserved)
	assert.Equal(t, 9, usage.TotalTokens)
}

func TestAliInferencePump_ReserveFailureRejectsTask(t *testing.T) {
	upstream, received := fakeInferenceUpstream(t, false, false, false, 0)
	clientConn, done := startInferencePumpWithOptions(t, upstream.URL, "sambert-zhichu-v1", inferencePumpTestOptions{
		reserve: func(usage *dto.RealtimeUsage) error {
			if usage.TotalTokens > 4 {
				return errors.New("quota exhausted")
			}
			return nil
		},
	})

	assert.NoError(t, clientConn.WriteMessage(websocket.TextMessage, []byte(inferenceRunTaskMessage("task-1", "sambert-zhichu-v1", "你好世界"))))
	readInferenceEvent(t, clientConn)
	_, _, err := clientConn.ReadMessage()
	assert.NoError(t, err)
	readInferenceEvent(t, clientConn)
	<-received

	assert.NoError(t, clientConn.WriteMessage(websocket.TextMessage, []byte(inferenceRunTaskMessage("task-2", "sambert-zhichu-v1", "床前明月光"))))
	frame := readInferenceEvent(t, clientConn)
	header := frame["header"].(map[string]any)
	assert.Equal(t, "task-2", header["task_id"])
	assert.Equal(t, "task-failed", header["event"])
	assert.Equal(t, "InsufficientQuota", header["error_code"])
	assert.Contains(t, header["error_message"], "quota exhausted")

	select {
	case <-received:
		t.Fatal("rejected task must not be forwarded upstream")
	default:
	}

	assert.NoError(t, clientConn.Close())
	usage := waitInferencePump(t, done)
	assert.Equal(t, 4, usage.TotalTokens)
}

// fakeInferenceUpstreamClose 模拟上游在不同阶段关闭连接，验证错误帧和关闭码透传。
func fakeInferenceUpstreamClose(t *testing.T, mode string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := testInferenceUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		closeWith := func(code int, reason string) {
			_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason), time.Now().Add(time.Second))
		}
		if mode == "immediate" {
			closeWith(websocket.CloseInternalServerErr, "upstream unavailable")
			return
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var frame map[string]any
		assert.NoError(t, common.Unmarshal(message, &frame))
		taskID := frame["header"].(map[string]any)["task_id"].(string)
		writeEvent := func(event string, characters int) {
			payload := map[string]any{}
			if characters > 0 {
				payload["usage"] = map[string]any{"characters": characters}
			}
			_ = conn.WriteJSON(map[string]any{
				"header":  map[string]any{"task_id": taskID, "event": event},
				"payload": payload,
			})
		}
		writeEvent("task-started", 0)
		if mode == "after-start" {
			closeWith(websocket.CloseInternalServerErr, "task interrupted")
			return
		}
		writeEvent("task-finished", 4)
		closeWith(websocket.CloseNormalClosure, "done")
	}))
	t.Cleanup(server.Close)
	return server
}

func TestAliInferencePump_UpstreamFailureBeforeTaskReturnsNativeError(t *testing.T) {
	upstream := fakeInferenceUpstreamClose(t, "immediate")
	clientConn, done := startInferencePump(t, upstream.URL, "sambert-zhichu-v1")

	frame := readInferenceEvent(t, clientConn)
	header := frame["header"].(map[string]any)
	assert.Equal(t, "task-failed", header["event"])
	assert.Equal(t, "UpstreamError", header["error_code"])
	assert.Contains(t, header["error_message"], "error reading from target")

	usage := waitInferencePump(t, done)
	assert.Equal(t, 0, usage.TotalTokens)
}

func TestAliInferencePump_UpstreamFailureAfterStartBillsOnlyStartedTask(t *testing.T) {
	upstream := fakeInferenceUpstreamClose(t, "after-start")
	clientConn, done := startInferencePump(t, upstream.URL, "sambert-zhichu-v1")

	assert.NoError(t, clientConn.WriteMessage(websocket.TextMessage, []byte(inferenceRunTaskMessage("task-1", "sambert-zhichu-v1", "你好世界"))))
	readInferenceEvent(t, clientConn) // task-started
	frame := readInferenceEvent(t, clientConn)
	header := frame["header"].(map[string]any)
	assert.Equal(t, "task-failed", header["event"])
	assert.Equal(t, "task-1", header["task_id"])

	usage := waitInferencePump(t, done)
	assert.Equal(t, 4, usage.TotalTokens)
}

func TestAliInferencePump_ForwardsNormalCloseCode(t *testing.T) {
	upstream := fakeInferenceUpstreamClose(t, "after-finish")
	clientConn, done := startInferencePump(t, upstream.URL, "sambert-zhichu-v1")

	assert.NoError(t, clientConn.WriteMessage(websocket.TextMessage, []byte(inferenceRunTaskMessage("task-1", "sambert-zhichu-v1", "你好世界"))))
	readInferenceEvent(t, clientConn) // task-started
	readInferenceEvent(t, clientConn) // task-finished

	_, _, err := clientConn.ReadMessage()
	var closeErr *websocket.CloseError
	if assert.ErrorAs(t, err, &closeErr) {
		assert.Equal(t, websocket.CloseNormalClosure, closeErr.Code)
		assert.Equal(t, "done", closeErr.Text)
	}

	usage := waitInferencePump(t, done)
	assert.Equal(t, 4, usage.TotalTokens)
}

func TestAliInferencePump_ReconcilesReserveWithUpstreamUsage(t *testing.T) {
	upstream, _ := fakeInferenceUpstream(t, true, false, false, 10000)
	var reserved []int
	clientConn, done := startInferencePumpWithOptions(t, upstream.URL, "sambert-zhichu-v1", inferencePumpTestOptions{
		reserve: func(usage *dto.RealtimeUsage) error {
			reserved = append(reserved, usage.TotalTokens)
			if usage.TotalTokens > 4 {
				return errors.New("actual usage exceeds available quota")
			}
			return nil
		},
	})

	assert.NoError(t, clientConn.WriteMessage(websocket.TextMessage, []byte(inferenceRunTaskMessage("task-1", "sambert-zhichu-v1", "你好世界"))))
	readInferenceEvent(t, clientConn) // task-started
	_, _, err := clientConn.ReadMessage()
	assert.NoError(t, err)            // binary
	readInferenceEvent(t, clientConn) // task-finished 先透传上游完成事件
	frame := readInferenceEvent(t, clientConn)
	header := frame["header"].(map[string]any)
	assert.Equal(t, "task-failed", header["event"])
	assert.Equal(t, "UpstreamError", header["error_code"])
	assert.Contains(t, header["error_message"], "actual usage exceeds available quota")

	usage := waitInferencePump(t, done)
	assert.Equal(t, []int{4, 10000}, reserved, "authoritative upstream usage must trigger another reservation")
	assert.Equal(t, 10000, usage.TotalTokens)
}
