package ali

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/tidwall/sjson"
)

// inferenceAliInferencePath 是 DashScope 原生推理 WebSocket 的默认端点路径。
// 运营者在渠道 base_url 中自带路径时则尊重其配置（如业务空间绑定域名）。
const inferenceAliInferencePath = "/api-ws/v1/inference"

// buildAliInferenceURL 由渠道 base_url 构造 DashScope 原生推理 WebSocket 地址。
// 约定 base_url 形如 https://dashscope.aliyuncs.com 或
// https://{WorkspaceId}.cn-beijing.maas.aliyuncs.com，亦可直接填写 wss:// 地址；
// 路径为空（或仅有 "/"）时补默认推理端点，自带路径则保留。
// 模型不在 URL 上传递（原生协议在 run-task 帧的 payload.model 中携带）。
func buildAliInferenceURL(channelBaseURL string) (string, error) {
	trimmed := strings.TrimSpace(channelBaseURL)
	if trimmed == "" {
		return "", errors.New("channel base url is empty")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid channel base url %q: %w", channelBaseURL, err)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("channel base url %q is missing host", channelBaseURL)
	}
	switch parsed.Scheme {
	case "https":
		parsed.Scheme = "wss"
	case "http":
		parsed.Scheme = "ws"
	case "wss", "ws":
		// 已是 WebSocket scheme，保持不变
	default:
		return "", fmt.Errorf("unsupported scheme %q in channel base url %q", parsed.Scheme, channelBaseURL)
	}
	if parsed.Path == "" || parsed.Path == "/" {
		parsed.Path = inferenceAliInferencePath
	}
	return parsed.String(), nil
}

// ----- 原生协议帧的最小结构（仅解析路由/计费所需字段，其余内容透传） -----

type inferenceRunTaskFrame struct {
	Header struct {
		Action string `json:"action"`
		TaskID string `json:"task_id"`
	} `json:"header"`
	Payload struct {
		Model string `json:"model"`
		Input struct {
			Text string `json:"text"`
		} `json:"input"`
	} `json:"payload"`
}

type inferenceServerFrame struct {
	Header struct {
		Event        string `json:"event"`
		TaskID       string `json:"task_id"`
		ErrorCode    string `json:"error_code"`
		ErrorMessage string `json:"error_message"`
	} `json:"header"`
	Payload struct {
		Usage *struct {
			Characters int `json:"characters"`
		} `json:"usage"`
	} `json:"payload"`
}

type inferencePendingTask struct {
	characters int
	started    bool
}

type inferencePumpError struct {
	upstream bool
	err      error
}

// rewriteInferenceRunTaskModel 将 run-task 帧的 payload.model 改写为上游模型名（支持模型映射）。
// 非 run-task 帧、未携带 model 或模型已一致时原样返回，保证透传。
func rewriteInferenceRunTaskModel(message []byte, upstreamModel string) []byte {
	var frame inferenceRunTaskFrame
	if err := common.Unmarshal(message, &frame); err != nil {
		return message
	}
	if frame.Header.Action != "run-task" || frame.Payload.Model == "" || frame.Payload.Model == upstreamModel {
		return message
	}
	// 定点替换 payload.model，其余字节原样保留；若走 map 反序列化再序列化，
	// 数字会被转成 float64，超过 2^53 的整数（如 seed）会静默丢失精度。
	rewritten, err := sjson.SetBytes(message, "payload.model", upstreamModel)
	if err != nil {
		return message
	}
	return rewritten
}

// inferenceRunTaskText 提取 run-task 帧的 task_id 与输入文本字符数（上游 usage 缺失时的本地计费兜底）。
func inferenceRunTaskText(message []byte) (taskID string, characters int, ok bool) {
	var frame inferenceRunTaskFrame
	if err := common.Unmarshal(message, &frame); err != nil {
		return "", 0, false
	}
	if frame.Header.Action != "run-task" {
		return "", 0, false
	}
	return frame.Header.TaskID, utf8.RuneCountInString(frame.Payload.Input.Text), true
}

func parseInferenceServerFrame(message []byte) (*inferenceServerFrame, bool) {
	var frame inferenceServerFrame
	if err := common.Unmarshal(message, &frame); err != nil {
		return nil, false
	}
	return &frame, true
}

func inferenceCharacterUsage(characters int) *dto.RealtimeUsage {
	usage := &dto.RealtimeUsage{
		TotalTokens: characters,
		InputTokens: characters,
	}
	usage.InputTokenDetails.TextTokens = characters
	return usage
}

func addInferenceCharacters(usage *dto.RealtimeUsage, characters int) {
	usage.TotalTokens += characters
	usage.InputTokens += characters
	usage.InputTokenDetails.TextTokens += characters
}

// AliInferenceHandler 中继 DashScope 原生推理 WebSocket 协议（Sambert 等）：
// 文本帧与二进制音频帧均原样透传，仅改写 run-task 的 payload.model 以支持模型映射。
// 每个任务转发前按连接累计用量追加预扣；task-finished 按 payload.usage.characters
// 累计实际用量（1 字符 = 1 输入 token），缺失时回退本地字符数。连接结束时由
// WssHelper 统一结算，预留与实际用量的差额多退少补，避免重复扣费。
func AliInferenceHandler(c *gin.Context, info *relaycommon.RelayInfo) (*types.NewAPIError, *dto.RealtimeUsage) {
	// 应用渠道模型映射，泵据此改写 run-task 的 payload.model
	if err := helper.ModelMappedHelper(c, info, nil); err != nil {
		return types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry()), nil
	}
	return aliInferencePumpWithReserve(c, info, func(usage *dto.RealtimeUsage) error {
		return service.ReserveWssQuota(c, info, usage)
	})
}

func aliInferencePump(c *gin.Context, info *relaycommon.RelayInfo) (*types.NewAPIError, *dto.RealtimeUsage) {
	return aliInferencePumpWithReserve(c, info, nil)
}

// aliInferencePumpWithReserve 是双向透传泵。reserve 接收包含新任务后的累计预估用量，
// 只负责追加预扣；实际费用仍在连接结束时统一结算。测试传入 nil 可跳过外部计费依赖。
func aliInferencePumpWithReserve(c *gin.Context, info *relaycommon.RelayInfo, reserve func(*dto.RealtimeUsage) error) (*types.NewAPIError, *dto.RealtimeUsage) {
	if info == nil || info.ClientWs == nil || info.TargetWs == nil {
		return types.NewError(fmt.Errorf("invalid websocket connection"), types.ErrorCodeBadResponse), nil
	}

	info.IsStream = true
	clientConn := info.ClientWs
	targetConn := info.TargetWs

	var mu sync.Mutex
	// task_id → 本地字符数与启动状态。只有收到 task-started 的未完成任务才在断连时计费，
	// 避免把上游未确认接收的请求计入费用。
	pendingTasks := make(map[string]inferencePendingTask)
	lastTaskID := ""
	sumUsage := &dto.RealtimeUsage{}

	// Gorilla 要求每个连接同一时刻最多一个 writer；错误帧和关闭帧也需要共享写锁。
	var clientWriteMu sync.Mutex
	var targetWriteMu sync.Mutex
	var reserveMu sync.Mutex
	reservedCharacters := 0
	var reserveFailed error

	clientClosed := make(chan struct{})
	targetClosed := make(chan struct{})
	errChan := make(chan inferencePumpError, 2)

	writeMessage := func(conn *websocket.Conn, writeMu *sync.Mutex, messageType int, message []byte) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return conn.WriteMessage(messageType, message)
	}

	writeTaskFailed := func(taskID, errorCode, errorMessage string) error {
		frame, err := common.Marshal(map[string]any{
			"header": map[string]any{
				"task_id":       taskID,
				"event":         "task-failed",
				"error_code":    errorCode,
				"error_message": errorMessage,
			},
			"payload": map[string]any{},
		})
		if err != nil {
			return err
		}
		return writeMessage(clientConn, &clientWriteMu, websocket.TextMessage, frame)
	}

	// forwardClose 仅透传正常关闭。异常关闭先由主协程发送 task-failed，
	// 避免客户端只收到 1006/1011 而看不到失败原因。
	forwardClose := func(conn *websocket.Conn, writeMu *sync.Mutex, readErr error) {
		var closeErr *websocket.CloseError
		if !errors.As(readErr, &closeErr) {
			return
		}
		if closeErr.Code != websocket.CloseNormalClosure && closeErr.Code != websocket.CloseGoingAway {
			return
		}
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(closeErr.Code, closeErr.Text), time.Now().Add(time.Second))
	}

	// reserveTaskLocked 计算并追加累计预留。调用方必须持有 reserveMu，
	// 并将任务登记/权威用量更新放在同一临界区，避免漏算并发任务。
	reserveTaskLocked := func(characters int) error {
		if reserve == nil {
			return nil
		}
		mu.Lock()
		totalCharacters := characters + sumUsage.InputTokenDetails.TextTokens
		for _, task := range pendingTasks {
			totalCharacters += task.characters
		}
		mu.Unlock()
		if totalCharacters <= reservedCharacters {
			return nil
		}
		if err := reserve(inferenceCharacterUsage(totalCharacters)); err != nil {
			return err
		}
		reservedCharacters = totalCharacters
		return nil
	}

	// forwardClientMessage 返回是否属于上游错误。额度不足时在本地失败该任务，不转发上游。
	forwardClientMessage := func(messageType int, message []byte) (bool, error) {
		taskID := ""
		characters := 0
		isRunTask := false
		if messageType == websocket.TextMessage {
			taskID, characters, isRunTask = inferenceRunTaskText(message)
			message = rewriteInferenceRunTaskModel(message, info.UpstreamModelName)
		}

		if isRunTask {
			// 预留、任务登记和上游写入是一个串行边界：权威用量补预留失败后，
			// 已经完成旧预留但尚未写入的新任务也不能继续提交。
			reserveMu.Lock()
			if reserveFailed != nil {
				reserveMu.Unlock()
				if writeErr := writeTaskFailed(taskID, "InsufficientQuota", reserveFailed.Error()); writeErr != nil {
					return false, writeErr
				}
				return false, nil
			}
			if err := reserveTaskLocked(characters); err != nil {
				reserveMu.Unlock()
				if writeErr := writeTaskFailed(taskID, "InsufficientQuota", err.Error()); writeErr != nil {
					return false, writeErr
				}
				return false, nil
			}
			mu.Lock()
			pendingTasks[taskID] = inferencePendingTask{characters: characters}
			lastTaskID = taskID
			mu.Unlock()

			if err := writeMessage(targetConn, &targetWriteMu, messageType, message); err != nil {
				mu.Lock()
				delete(pendingTasks, taskID)
				mu.Unlock()
				reserveMu.Unlock()
				return true, err
			}
			reserveMu.Unlock()
			return false, nil
		}

		if err := writeMessage(targetConn, &targetWriteMu, messageType, message); err != nil {
			return true, err
		}
		return false, nil
	}

	// 客户端 → 上游：run-task 预留额度并记录任务，其余帧（含二进制）原样转发。
	gopool.Go(func() {
		defer close(clientClosed)
		defer func() {
			if r := recover(); r != nil {
				errChan <- inferencePumpError{upstream: true, err: fmt.Errorf("panic in inference client reader: %v", r)}
			}
		}()

		if len(info.FirstWsMessage) > 0 {
			messageType := info.FirstWsMessageType
			if messageType == 0 {
				messageType = websocket.TextMessage
			}
			if upstream, err := forwardClientMessage(messageType, info.FirstWsMessage); err != nil {
				errChan <- inferencePumpError{upstream: upstream, err: err}
				return
			}
		}

		for {
			select {
			case <-c.Done():
				return
			default:
			}
			messageType, message, err := clientConn.ReadMessage()
			if err != nil {
				forwardClose(targetConn, &targetWriteMu, err)
				if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					errChan <- inferencePumpError{upstream: false, err: fmt.Errorf("error reading from client: %v", err)}
				}
				return
			}
			if upstream, err := forwardClientMessage(messageType, message); err != nil {
				errChan <- inferencePumpError{upstream: upstream, err: err}
				return
			}
		}
	})

	// 上游 → 客户端：二进制音频帧原样转发；维护任务状态并累计实际字符用量。
	gopool.Go(func() {
		defer close(targetClosed)
		defer func() {
			if r := recover(); r != nil {
				errChan <- inferencePumpError{upstream: true, err: fmt.Errorf("panic in inference target reader: %v", r)}
			}
		}()
		for {
			select {
			case <-c.Done():
				return
			default:
			}
			messageType, message, err := targetConn.ReadMessage()
			if err != nil {
				forwardClose(clientConn, &clientWriteMu, err)
				if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					errChan <- inferencePumpError{upstream: true, err: fmt.Errorf("error reading from target: %v", err)}
				}
				return
			}
			info.SetFirstResponseTime()
			var actualReserveErr error
			if messageType == websocket.TextMessage {
				if frame, ok := parseInferenceServerFrame(message); ok {
					switch frame.Header.Event {
					case "task-started":
						mu.Lock()
						if task, exists := pendingTasks[frame.Header.TaskID]; exists {
							task.started = true
							pendingTasks[frame.Header.TaskID] = task
						}
						mu.Unlock()
					case "task-finished":
						characters := 0
						if frame.Payload.Usage != nil {
							characters = frame.Payload.Usage.Characters
						}
						// 与客户端新任务的预留/登记/写入串行，保证补预留能看到完整任务集合。
						reserveMu.Lock()
						mu.Lock()
						localCharacters := pendingTasks[frame.Header.TaskID].characters
						delete(pendingTasks, frame.Header.TaskID)
						if characters <= 0 {
							characters = localCharacters
						}
						addInferenceCharacters(sumUsage, characters)
						if lastTaskID == frame.Header.TaskID {
							lastTaskID = ""
						}
						mu.Unlock()
						// 上游权威用量可能大于本地估算，立即按修正后的累计用量补充预留；
						// 若额度不足则停止接受后续任务，避免继续超额消费。
						actualReserveErr = reserveTaskLocked(0)
						if actualReserveErr != nil {
							reserveFailed = actualReserveErr
						}
						reserveMu.Unlock()
					case "task-failed":
						logger.LogError(c, fmt.Sprintf("dashscope inference task failed: task_id=%s, error_code=%s, error_message=%s",
							frame.Header.TaskID, frame.Header.ErrorCode, frame.Header.ErrorMessage))
						mu.Lock()
						delete(pendingTasks, frame.Header.TaskID)
						if lastTaskID == frame.Header.TaskID {
							lastTaskID = ""
						}
						mu.Unlock()
					}
				}
			}
			if err := writeMessage(clientConn, &clientWriteMu, messageType, message); err != nil {
				errChan <- inferencePumpError{upstream: false, err: fmt.Errorf("error writing to client: %v", err)}
				return
			}
			if actualReserveErr != nil {
				errChan <- inferencePumpError{upstream: true, err: fmt.Errorf("error reserving actual usage: %w", actualReserveErr)}
				return
			}
		}
	})

	var pumpErr *inferencePumpError
	select {
	case <-clientClosed:
	case <-targetClosed:
	case err := <-errChan:
		pumpErr = &err
		logger.LogError(c, "dashscope inference relay error: "+err.err.Error())
	case <-c.Done():
	}

	// 上游异常时先补一个原生错误帧；随后关闭两侧连接并等待协程退出。
	// 不在协议中途重试，避免同一 run-task 被重复提交和重复合成。
	if pumpErr != nil && pumpErr.upstream {
		mu.Lock()
		failureTaskID := lastTaskID
		mu.Unlock()
		_ = writeTaskFailed(failureTaskID, "UpstreamError", pumpErr.err.Error())
	}
	_ = clientConn.Close()
	_ = targetConn.Close()
	<-clientClosed
	<-targetClosed

	// 连接中断时，已启动但仍未收到结束事件的任务按本地字符数计入用量。
	mu.Lock()
	for _, task := range pendingTasks {
		if task.started {
			addInferenceCharacters(sumUsage, task.characters)
		}
	}
	mu.Unlock()

	return nil, sumUsage
}
