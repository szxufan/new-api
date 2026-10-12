package relay

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// VirtualModelTestTargetResult 虚拟模型测试中单个子模型（或聚合模型）的结果
type VirtualModelTestTargetResult struct {
	Index        int    `json:"index"`            // 子模型序号；聚合模型为 -1
	Model        string `json:"model"`            // 真实模型名
	ChannelId    int    `json:"channel_id"`       // 实际选中的渠道 ID
	Success      bool   `json:"success"`          // 分支是否成功
	FirstTokenMs int64  `json:"first_token_ms"`   // 首次写出耗时（非流式≈总耗时）
	ElapsedMs    int64  `json:"elapsed_ms"`       // 分支整体耗时
	Answer       string `json:"answer,omitempty"` // 提取的回答文本（流式模式下不提取）
	Error        string `json:"error,omitempty"`
}

// VirtualModelTestResult 虚拟模型测试结果
type VirtualModelTestResult struct {
	Mode       string                         `json:"mode"`     // speed | quality
	Question   string                         `json:"question"` // 实际使用的测试问题
	TotalMs    int64                          `json:"total_ms"` // 测试整体耗时
	Targets    []VirtualModelTestTargetResult `json:"targets"`
	Aggregator *VirtualModelTestTargetResult  `json:"aggregator,omitempty"` // 仅质量模式：聚合模型结果
}

// ---------------- testWriter：测试分支的计时缓冲 Writer ----------------

// testWriter 在 captureWriter 缓冲语义的基础上记录首次写出时间，
// 用于测量每个子模型的首响应时间与总耗时
type testWriter struct {
	header       http.Header
	status       int
	buf          bytes.Buffer
	start        time.Time
	mu           sync.Mutex
	firstWriteAt time.Time
}

func newTestWriter(start time.Time) *testWriter {
	return &testWriter{header: make(http.Header), status: http.StatusOK, start: start}
}

func (w *testWriter) Header() http.Header    { return w.header }
func (w *testWriter) WriteHeader(status int) { w.status = status }
func (w *testWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	if w.firstWriteAt.IsZero() {
		w.firstWriteAt = time.Now()
	}
	w.mu.Unlock()
	return w.buf.Write(p)
}
func (w *testWriter) Flush()                            {}
func (w *testWriter) WriteString(s string) (int, error) { return w.Write([]byte(s)) }
func (w *testWriter) Status() int                       { return w.status }
func (w *testWriter) Size() int                         { return w.buf.Len() }
func (w *testWriter) Written() bool                     { return w.buf.Len() > 0 }
func (w *testWriter) WriteHeaderNow()                   {}
func (w *testWriter) CloseNotify() <-chan bool          { return make(chan bool) }
func (w *testWriter) Pusher() http.Pusher               { return nil }
func (w *testWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, errors.New("hijack not supported in virtual model test branch")
}

// FirstWriteMs 距分支启动到首次写出的毫秒数；未写出返回 0
func (w *testWriter) FirstWriteMs() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.firstWriteAt.IsZero() {
		return 0
	}
	return w.firstWriteAt.Sub(w.start).Milliseconds()
}

// ---------------- 测试入口 ----------------

// RunVirtualModelTest 用给定测试请求完整执行一次虚拟模型测试：
// - 速度模式：所有子模型并行请求（不抢跑、不取消败者），返回每个子模型的响应时间
// - 质量模式：完整复刻生产质量模式流程（并行等待 + 触发数量/超时 + 聚合模型汇总）
// 测试不结算计费、不记录胜者缓存（分支上下文自身跳过计费）
func RunVirtualModelTest(c *gin.Context, vm *model.VirtualModel, testReq *dto.GeneralOpenAIRequest) *VirtualModelTestResult {
	start := time.Now()
	result := &VirtualModelTestResult{
		Mode:     vm.Mode,
		Question: extractTestQuestion(testReq),
	}

	info := relaycommon.GenRelayInfoOpenAI(c, testReq)
	info.Billing = nil

	targets, err := vm.GetTargets()
	if err != nil || len(targets) == 0 {
		result.Targets = []VirtualModelTestTargetResult{{
			Index:     0,
			Model:     vm.Name,
			Success:   false,
			ElapsedMs: time.Since(start).Milliseconds(),
			Error:     fmt.Sprintf("虚拟模型 %s 没有可用的子模型配置", vm.Name),
		}}
		result.TotalMs = time.Since(start).Milliseconds()
		return result
	}

	switch vm.Mode {
	case model.VirtualModelModeSpeed:
		runVirtualSpeedTest(c, info, targets, result)
	case model.VirtualModelModeQuality:
		runVirtualQualityTest(c, info, vm, targets, result)
	default:
		result.Targets = []VirtualModelTestTargetResult{{
			Index:     0,
			Model:     vm.Name,
			Success:   false,
			ElapsedMs: time.Since(start).Milliseconds(),
			Error:     fmt.Sprintf("未知虚拟模型模式: %s", vm.Mode),
		}}
	}
	result.TotalMs = time.Since(start).Milliseconds()
	return result
}

// extractTestQuestion 从测试请求中取出首条 user 消息作为测试问题展示
func extractTestQuestion(testReq *dto.GeneralOpenAIRequest) string {
	for _, m := range testReq.Messages {
		if m.Role == "user" {
			if content, ok := m.Content.(string); ok {
				return content
			}
			break
		}
	}
	return ""
}

// ---------------- 速度模式测试：测每个子模型的响应时间 ----------------

// runVirtualSpeedTest 并行请求全部子模型（不取消败者，等待全部完成），
// 返回每个子模型的首响应时间与总耗时
func runVirtualSpeedTest(c *gin.Context, info *relaycommon.RelayInfo, targets []model.VirtualModelTarget, result *VirtualModelTestResult) {
	n := len(targets)
	results := make([]VirtualModelTestTargetResult, n)
	var wg sync.WaitGroup
	for i, target := range targets {
		wg.Add(1)
		go func(idx int, t model.VirtualModelTarget) {
			defer wg.Done()
			// 不强制非流式：stream 请求下 FirstTokenMs 即为首 chunk 时间
			results[idx] = runVirtualTestBranch(c, info, t, idx, false)
		}(i, target)
	}
	wg.Wait()
	result.Targets = results
}

// ---------------- 质量模式测试：完整走一遍生产聚合流程 ----------------

// runVirtualQualityTest 复刻 virtualQualityMode 的完整语义：
// 并行分支（强制非流式）→ 触发数量 + 等待超时 → 收集非空回答 → 聚合模型汇总，
// 额外返回每个子模型与聚合模型的输出
func runVirtualQualityTest(c *gin.Context, info *relaycommon.RelayInfo, vm *model.VirtualModel, targets []model.VirtualModelTarget, result *VirtualModelTestResult) {
	n := len(targets)
	doneCh := make(chan *virtualBranchOutcome, n)
	cancels := make([]context.CancelFunc, n)
	branchDone := make([]bool, n)
	writers := make([]*testWriter, n)
	elapsedMs := make([]int64, n)

	for i, target := range targets {
		cancelCtx, cancel := context.WithCancel(c.Request.Context())
		cancels[i] = cancel
		go func(idx int, t model.VirtualModelTarget, bCtx context.Context) {
			writer := newTestWriter(time.Now())
			outcome := &virtualBranchOutcome{idx: idx}
			branchReq, reqErr := prepareBranchRequest(info, t.Model, true)
			if reqErr != nil {
				outcome.err = reqErr
			} else {
				outcome = runVirtualBranch(c, info, t, branchReq, writer, bCtx, 0)
				outcome.idx = idx
			}
			writers[idx] = writer
			elapsedMs[idx] = time.Since(writer.start).Milliseconds()
			if outcome.err == nil {
				outcome.body = writer.buf.Bytes()
			}
			doneCh <- outcome
		}(i, target, cancelCtx)
	}

	// 等待逻辑与生产一致：达到触发数量后启动等待计时，超时取消未完成分支
	triggerCount := vm.QualityTriggerCount
	if triggerCount < 1 {
		triggerCount = 1
	}
	var timer *time.Timer
	var timerCh <-chan time.Time
	fired := false
	successCount := 0
	outcomes := make([]*virtualBranchOutcome, 0, n)

	for completed := 0; completed < n; {
		select {
		case out := <-doneCh:
			completed++
			branchDone[out.idx] = true
			outcomes = append(outcomes, out)
			if out.err == nil {
				successCount++
				if vm.QualityWaitMs > 0 && !fired && successCount >= triggerCount {
					fired = true
					timer = time.NewTimer(time.Duration(vm.QualityWaitMs) * time.Millisecond)
					timerCh = timer.C
				}
			}
		case <-timerCh:
			timerCh = nil
			for idx, cancel := range cancels {
				if cancel != nil && !branchDone[idx] {
					cancel()
				}
			}
		}
	}
	if timer != nil {
		timer.Stop()
	}

	// 组装每个子模型的结果并收集非空回答
	targetResults := make([]VirtualModelTestTargetResult, n)
	candidates := make([]int, 0, n)
	for i := range targetResults {
		targetResults[i] = VirtualModelTestTargetResult{
			Index:        i,
			Model:        targets[i].Model,
			FirstTokenMs: 0,
			ElapsedMs:    elapsedMs[i],
		}
	}
	for _, out := range outcomes {
		tr := &targetResults[out.idx]
		if w := writers[out.idx]; w != nil {
			tr.FirstTokenMs = w.FirstWriteMs()
		}
		if out.channel != nil {
			tr.ChannelId = out.channel.Id
		}
		if out.err != nil {
			tr.Error = out.err.Error()
			continue
		}
		answer := gjson.GetBytes(out.body, "choices.0.message.content").String()
		if strings.TrimSpace(answer) == "" {
			tr.Error = "分支返回了空回答，未参与聚合"
			continue
		}
		tr.Success = true
		tr.Answer = answer
		candidates = append(candidates, out.idx)
	}
	result.Targets = targetResults

	agg, aggErr := vm.GetAggregator()
	if aggErr != nil || agg == nil {
		result.Aggregator = &VirtualModelTestTargetResult{
			Index:   -1,
			Success: false,
			Error:   fmt.Sprintf("虚拟模型 %s 没有有效的聚合模型配置", vm.Name),
		}
		return
	}
	if len(candidates) == 0 {
		// 无有效回答：跳过聚合（由前端提示），Aggregator 保持 nil
		return
	}

	// 组装聚合指令（与生产一致的模板替换逻辑）
	var sb strings.Builder
	for _, idx := range candidates {
		sb.WriteString(fmt.Sprintf("[Model %s]:\n%s\n\n", targets[idx].Model, targetResults[idx].Answer))
	}
	template := agg.PromptTemplate
	if strings.TrimSpace(template) == "" {
		template = defaultAggregatorPromptTemplate
	}
	aggInstruction := strings.ReplaceAll(template, "{{answers}}", sb.String())

	textReq, ok := info.Request.(*dto.GeneralOpenAIRequest)
	if !ok {
		result.Aggregator = &VirtualModelTestTargetResult{
			Index:   -1,
			Success: false,
			Error:   fmt.Sprintf("invalid request type: %T", info.Request),
		}
		return
	}
	aggReq, err := common.DeepCopy(textReq)
	if err != nil {
		result.Aggregator = &VirtualModelTestTargetResult{
			Index:   -1,
			Success: false,
			Error:   err.Error(),
		}
		return
	}
	aggReq.SetModelName(agg.Model)
	aggReq.Stream = common.GetPointer(false)
	aggReq.Messages = append(aggReq.Messages, dto.Message{
		Role:    "user",
		Content: aggInstruction,
	})

	aggTarget := model.VirtualModelTarget{
		Model:     agg.Model,
		ChannelId: agg.ChannelId,
		Group:     agg.Group,
	}
	aggResult := runVirtualTestBranchWithRequest(c, info, aggTarget, aggReq, -1)
	result.Aggregator = &aggResult
}

// ---------------- 分支执行与结果组装 ----------------

// runVirtualTestBranch 构造分支请求并执行，返回计时结果
func runVirtualTestBranch(c *gin.Context, info *relaycommon.RelayInfo, target model.VirtualModelTarget, idx int, forceNonStream bool) VirtualModelTestTargetResult {
	branchReq, reqErr := prepareBranchRequest(info, target.Model, forceNonStream)
	if reqErr != nil {
		return VirtualModelTestTargetResult{
			Index: idx,
			Model: target.Model,
			Error: reqErr.Error(),
		}
	}
	return runVirtualTestBranchWithRequest(c, info, target, branchReq, idx)
}

// runVirtualTestBranchWithRequest 执行已构造好的分支请求，测量首响应与总耗时并提取回答
func runVirtualTestBranchWithRequest(c *gin.Context, info *relaycommon.RelayInfo, target model.VirtualModelTarget, branchReq *dto.GeneralOpenAIRequest, idx int) VirtualModelTestTargetResult {
	start := time.Now()
	writer := newTestWriter(start)
	res := VirtualModelTestTargetResult{Index: idx, Model: target.Model}

	outcome := runVirtualBranch(c, info, target, branchReq, writer, c.Request.Context(), 0)
	res.FirstTokenMs = writer.FirstWriteMs()
	res.ElapsedMs = time.Since(start).Milliseconds()
	if outcome.channel != nil {
		res.ChannelId = outcome.channel.Id
	}
	if outcome.err != nil {
		res.Error = outcome.err.Error()
		return res
	}
	res.Success = true
	res.Answer = strings.TrimSpace(gjson.GetBytes(writer.buf.Bytes(), "choices.0.message.content").String())
	return res
}
