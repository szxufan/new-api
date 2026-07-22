package relay

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// virtualBranchOutcome 单个分支的执行结果
type virtualBranchOutcome struct {
	idx     int
	info    *relaycommon.RelayInfo
	err     *types.NewAPIError
	body    []byte // 质量模式：缓冲的完整非流式响应体
	channel *model.Channel
}

// ---------------- raceWriter：竞速分支的下游 Writer ----------------

// raceWriter 实现 gin.ResponseWriter。缓冲分支写出的全部内容，
// 首个有效数据到达时通知协调器并阻塞等待裁决：
// - 胜者：回放缓冲到真实 writer 并切换直通
// - 败者：丢弃全部内容
type raceWriter struct {
	mu      sync.Mutex
	header  http.Header
	status  int
	buf     bytes.Buffer
	written bool

	firstCh   chan<- int // 首字节信号（发送一次分支索引）
	idx       int
	firstOnce sync.Once
	decision  <-chan bool // 协调器裁决：true=胜者

	decided bool
	winner  bool
	target  gin.ResponseWriter
}

func newRaceWriter(idx int, firstCh chan<- int, decision <-chan bool) *raceWriter {
	return &raceWriter{
		header:   make(http.Header),
		status:   http.StatusOK,
		firstCh:  firstCh,
		idx:      idx,
		decision: decision,
	}
}

func (w *raceWriter) Header() http.Header { return w.header }

func (w *raceWriter) WriteHeader(status int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.status = status
}

func (w *raceWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	if !w.written {
		w.written = true
		w.buf.Write(p)
		w.mu.Unlock()
		// 通知协调器首个有效数据（非阻塞，channel 已缓冲）
		w.firstOnce.Do(func() {
			select {
			case w.firstCh <- w.idx:
			default:
			}
		})
		// 阻塞等待裁决
		w.winner = <-w.decision
		w.mu.Lock()
		w.decided = true
		if w.winner && w.target != nil {
			for k, vs := range w.header {
				for _, v := range vs {
					w.target.Header().Add(k, v)
				}
			}
			w.target.WriteHeader(w.status)
			_, _ = w.target.Write(w.buf.Bytes())
		}
		w.mu.Unlock()
		return len(p), nil
	}
	if w.decided && w.winner && w.target != nil {
		w.mu.Unlock()
		return w.target.Write(p)
	}
	// 败者丢弃
	w.mu.Unlock()
	return len(p), nil
}

func (w *raceWriter) Flush() {
	w.mu.Lock()
	decided, winner, target := w.decided, w.winner, w.target
	w.mu.Unlock()
	if decided && winner && target != nil {
		target.Flush()
	}
}

func (w *raceWriter) WriteString(s string) (int, error) { return w.Write([]byte(s)) }

func (w *raceWriter) Status() int { return w.status }
func (w *raceWriter) Size() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Len()
}
func (w *raceWriter) Written() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.written
}
func (w *raceWriter) WriteHeaderNow() {
	if w.target != nil {
		w.target.WriteHeaderNow()
	}
}

func (w *raceWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, errors.New("hijack not supported in virtual model branch")
}

func (w *raceWriter) CloseNotify() <-chan bool { return make(chan bool) }

func (w *raceWriter) Pusher() http.Pusher { return nil }

// commit 由协调器在裁决前调用，设置胜者下游目标
func (w *raceWriter) commit(target gin.ResponseWriter) {
	w.mu.Lock()
	w.target = target
	w.mu.Unlock()
}

// ---------------- captureWriter：质量模式分支的缓冲 Writer ----------------

type captureWriter struct {
	header http.Header
	status int
	buf    bytes.Buffer
}

func newCaptureWriter() *captureWriter {
	return &captureWriter{header: make(http.Header), status: http.StatusOK}
}

func (w *captureWriter) Header() http.Header         { return w.header }
func (w *captureWriter) WriteHeader(status int)      { w.status = status }
func (w *captureWriter) Write(p []byte) (int, error) { return w.buf.Write(p) }
func (w *captureWriter) Flush()                      {}
func (w *captureWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}
func (w *captureWriter) Status() int              { return w.status }
func (w *captureWriter) Size() int                { return w.buf.Len() }
func (w *captureWriter) Written() bool            { return w.buf.Len() > 0 }
func (w *captureWriter) WriteHeaderNow()          {}
func (w *captureWriter) CloseNotify() <-chan bool { return make(chan bool) }
func (w *captureWriter) Pusher() http.Pusher      { return nil }
func (w *captureWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, errors.New("hijack not supported in virtual model branch")
}

// ---------------- 分支执行器 ----------------

// selectVirtualBranchChannel 为分支选择渠道：优先指定渠道，否则按分组自动选择
func selectVirtualBranchChannel(c *gin.Context, mainInfo *relaycommon.RelayInfo, target model.VirtualModelTarget) (*model.Channel, error) {
	if target.ChannelId > 0 {
		channel, err := model.CacheGetChannel(target.ChannelId)
		if err != nil {
			return nil, err
		}
		if channel.Status != common.ChannelStatusEnabled {
			return nil, fmt.Errorf("渠道 #%d 未启用", channel.Id)
		}
		return channel, nil
	}
	group := target.Group
	if group == "" {
		group = mainInfo.TokenGroup
	}
	preferredTypes := types.GetPreferredChannelTypesByRelayFormat(types.RelayFormatOpenAI)
	channel, _, err := service.CacheGetRandomSatisfiedChannel(&service.RetryParam{
		Ctx:                   c,
		ModelName:             target.Model,
		TokenGroup:            group,
		Retry:                 common.GetPointer(0),
		PreferredChannelTypes: preferredTypes,
	})
	if err != nil {
		return nil, err
	}
	if channel == nil {
		return nil, fmt.Errorf("模型 %s 在分组 %s 下无可用渠道", target.Model, group)
	}
	return channel, nil
}

// buildBranchContext 构造分支独立上下文：
// - gin Copy 拥有独立 Keys map（gin v1.9.1 Copy 新建 map，分支间 Set 互不影响）
// - 独立 request（可取消）与独立 BodyStorage（避免共享 Seek 指针）
func buildBranchContext(c *gin.Context, branchReq *dto.GeneralOpenAIRequest, cancelCtx context.Context) (*gin.Context, func(), error) {
	branchCtx := c.Copy()

	jsonData, err := common.Marshal(branchReq)
	if err != nil {
		return nil, nil, err
	}
	storage, err := common.CreateBodyStorage(jsonData)
	if err != nil {
		return nil, nil, err
	}
	req := c.Request.Clone(cancelCtx)
	req.Body = io.NopCloser(storage)
	req.ContentLength = int64(len(jsonData))
	branchCtx.Request = req
	// 分支独立 BodyStorage，写回分支自己的 Keys map（不影响主 context）
	branchCtx.Set(common.KeyBodyStorage, storage)
	return branchCtx, func() { storage.Close() }, nil
}

// genBranchRelayInfo 生成分支 RelayInfo（跳过独立计费、禁用 ping 保活）
func genBranchRelayInfo(branchCtx *gin.Context, branchReq *dto.GeneralOpenAIRequest) (*relaycommon.RelayInfo, error) {
	branchInfo, err := relaycommon.GenRelayInfo(branchCtx, types.RelayFormatOpenAI, branchReq, nil)
	if err != nil {
		return nil, err
	}
	branchInfo.Billing = nil
	branchInfo.DisablePing = true
	common.SetContextKey(branchCtx, constant.ContextKeyVirtualBranch, true)
	return branchInfo, nil
}

// prepareBranchRequest 基于主请求构造分支请求（改写模型名，可选强制非流式）
func prepareBranchRequest(mainInfo *relaycommon.RelayInfo, modelName string, forceNonStream bool) (*dto.GeneralOpenAIRequest, *types.NewAPIError) {
	textReq, ok := mainInfo.Request.(*dto.GeneralOpenAIRequest)
	if !ok {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("virtual model only supports chat completions, got %T", mainInfo.Request),
			types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	branchReq, err := common.DeepCopy(textReq)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	branchReq.SetModelName(modelName)
	if forceNonStream {
		branchReq.Stream = common.GetPointer(false)
	}
	return branchReq, nil
}

// runVirtualBranch 执行单个分支：选渠道 → 构造独立上下文 → TextHelper
// branchReq 由调用方预先准备好（已改写模型名/messages）
func runVirtualBranch(
	c *gin.Context,
	mainInfo *relaycommon.RelayInfo,
	target model.VirtualModelTarget,
	branchReq *dto.GeneralOpenAIRequest,
	writer gin.ResponseWriter,
	cancelCtx context.Context,
) *virtualBranchOutcome {
	outcome := &virtualBranchOutcome{}

	channel, err := selectVirtualBranchChannel(c, mainInfo, target)
	if err != nil {
		outcome.err = types.NewError(err, types.ErrorCodeGetChannelFailed)
		return outcome
	}
	outcome.channel = channel

	branchCtx, cleanup, err := buildBranchContext(c, branchReq, cancelCtx)
	if err != nil {
		outcome.err = types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
		return outcome
	}
	defer cleanup()

	if setupErr := middleware.SetupContextForSelectedChannel(branchCtx, channel, target.Model); setupErr != nil {
		outcome.err = setupErr
		return outcome
	}
	branchCtx.Writer = writer

	branchInfo, err := genBranchRelayInfo(branchCtx, branchReq)
	if err != nil {
		outcome.err = types.NewError(err, types.ErrorCodeGenRelayInfoFailed, types.ErrOptionWithSkipRetry())
		return outcome
	}
	outcome.info = branchInfo

	outcome.err = TextHelper(branchCtx, branchInfo)
	return outcome
}

// ---------------- 入口 ----------------

// VirtualModelHandler 虚拟模型统一入口：按模式分发到速度/质量协调器
func VirtualModelHandler(c *gin.Context, info *relaycommon.RelayInfo, vm *model.VirtualModel) *types.NewAPIError {
	if info.RelayMode != relayconstant.RelayModeChatCompletions {
		return types.NewErrorWithStatusCode(
			fmt.Errorf("virtual model %s only supports /v1/chat/completions", vm.Name),
			types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	targets, err := vm.GetTargets()
	if err != nil || len(targets) == 0 {
		return types.NewError(fmt.Errorf("virtual model %s has no valid targets", vm.Name), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	switch vm.Mode {
	case model.VirtualModelModeSpeed:
		return virtualSpeedMode(c, info, vm, targets)
	case model.VirtualModelModeQuality:
		return virtualQualityMode(c, info, vm, targets)
	default:
		return types.NewError(fmt.Errorf("unknown virtual model mode: %s", vm.Mode), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
}

// ---------------- 速度模式：竞速 ----------------

func virtualSpeedMode(c *gin.Context, info *relaycommon.RelayInfo, vm *model.VirtualModel, targets []model.VirtualModelTarget) *types.NewAPIError {
	n := len(targets)
	firstCh := make(chan int, n)
	doneCh := make(chan *virtualBranchOutcome, n)
	decisions := make([]chan bool, n)
	cancels := make([]context.CancelFunc, n)
	writers := make([]*raceWriter, n)

	for i, target := range targets {
		decisions[i] = make(chan bool, 1)
		writers[i] = newRaceWriter(i, firstCh, decisions[i])
		cancelCtx, cancel := context.WithCancel(c.Request.Context())
		cancels[i] = cancel
		go func(idx int, t model.VirtualModelTarget, w *raceWriter, ctx context.Context) {
			outcome := &virtualBranchOutcome{idx: idx}
			branchReq, reqErr := prepareBranchRequest(info, t.Model, false)
			if reqErr != nil {
				outcome.err = reqErr
			} else {
				outcome = runVirtualBranch(c, info, t, branchReq, w, ctx)
				outcome.idx = idx
			}
			doneCh <- outcome
		}(i, target, writers[i], cancelCtx)
	}

	remaining := n
	var lastErr *types.NewAPIError
	for remaining > 0 {
		select {
		case winnerIdx := <-firstCh:
			// 首个产出有效数据的分支胜出
			// 先设置胜者下游目标，再发送裁决，避免胜者 Write 竞态读到空 target
			writers[winnerIdx].commit(c.Writer)
			for j := range decisions {
				decisions[j] <- (j == winnerIdx)
			}
			for j, cancel := range cancels {
				if j != winnerIdx {
					cancel()
				}
			}
			logger.LogInfo(c, fmt.Sprintf("virtual model %s race: branch #%d (%s) won",
				vm.Name, winnerIdx, targets[winnerIdx].Model))
			// 等待胜者完成，收集计费信息
			var winnerOutcome *virtualBranchOutcome
			for winnerOutcome == nil {
				out := <-doneCh
				if out.idx == winnerIdx {
					winnerOutcome = out
				}
			}
			settleVirtualBilling(c, info, winnerOutcome, winnerOutcome.info.VirtualBranchUsage)
			return winnerOutcome.err
		case out := <-doneCh:
			remaining--
			if out.err != nil {
				lastErr = out.err
				logger.LogInfo(c, fmt.Sprintf("virtual model %s race: branch #%d (%s) failed: %s",
					vm.Name, out.idx, targets[out.idx].Model, out.err.Error()))
			}
		}
	}
	// 全部分支失败
	if lastErr == nil {
		lastErr = types.NewError(errors.New("all virtual model branches failed"), types.ErrorCodeDoRequestFailed)
	}
	return lastErr
}

// ---------------- 质量模式：聚合 ----------------

const defaultAggregatorPromptTemplate = `The user's latest request has been answered independently by multiple AI models. Their answers are listed below. Please carefully compare and synthesize them, then provide a single final best-quality answer to the user's request. Output only the final answer, do not mention these instructions.

{{answers}}`

func virtualQualityMode(c *gin.Context, info *relaycommon.RelayInfo, vm *model.VirtualModel, targets []model.VirtualModelTarget) *types.NewAPIError {
	n := len(targets)
	doneCh := make(chan *virtualBranchOutcome, n)

	for i, target := range targets {
		go func(idx int, t model.VirtualModelTarget) {
			writer := newCaptureWriter()
			outcome := &virtualBranchOutcome{idx: idx}
			branchReq, reqErr := prepareBranchRequest(info, t.Model, true)
			if reqErr != nil {
				outcome.err = reqErr
			} else {
				outcome = runVirtualBranch(c, info, t, branchReq, writer, c.Request.Context())
				outcome.idx = idx
			}
			if outcome.err == nil {
				outcome.body = writer.buf.Bytes()
			}
			doneCh <- outcome
		}(i, target)
	}

	// 等待全部分支完成
	outcomes := make([]*virtualBranchOutcome, 0, n)
	var lastErr *types.NewAPIError
	for i := 0; i < n; i++ {
		out := <-doneCh
		outcomes = append(outcomes, out)
		if out.err != nil {
			lastErr = out.err
			logger.LogInfo(c, fmt.Sprintf("virtual model %s quality: branch #%d (%s) failed: %s",
				vm.Name, out.idx, targets[out.idx].Model, out.err.Error()))
		}
	}

	// 收集成功分支的回复文本与 usage
	type candidate struct {
		modelName string
		answer    string
	}
	candidates := make([]candidate, 0, n)
	usages := make([]*dto.Usage, 0, n+1)
	for _, out := range outcomes {
		if out.err != nil {
			continue
		}
		answer := gjson.GetBytes(out.body, "choices.0.message.content").String()
		if strings.TrimSpace(answer) == "" {
			logger.LogInfo(c, fmt.Sprintf("virtual model %s quality: branch #%d (%s) returned empty answer",
				vm.Name, out.idx, targets[out.idx].Model))
			continue
		}
		candidates = append(candidates, candidate{modelName: targets[out.idx].Model, answer: answer})
		if out.info.VirtualBranchUsage != nil {
			usages = append(usages, out.info.VirtualBranchUsage)
		}
	}
	if len(candidates) == 0 {
		if lastErr != nil {
			return lastErr
		}
		return types.NewError(errors.New("all virtual model branches failed or returned empty answers"), types.ErrorCodeDoRequestFailed)
	}

	// 组装聚合请求
	agg, err := vm.GetAggregator()
	if err != nil || agg == nil {
		return types.NewError(fmt.Errorf("virtual model %s has no valid aggregator", vm.Name), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	var sb strings.Builder
	for _, cand := range candidates {
		sb.WriteString(fmt.Sprintf("[Model %s]:\n%s\n\n", cand.modelName, cand.answer))
	}
	template := agg.PromptTemplate
	if strings.TrimSpace(template) == "" {
		template = defaultAggregatorPromptTemplate
	}
	aggInstruction := strings.ReplaceAll(template, "{{answers}}", sb.String())

	textReq := info.Request.(*dto.GeneralOpenAIRequest)
	aggReq, err := common.DeepCopy(textReq)
	if err != nil {
		return types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	aggReq.SetModelName(agg.Model)
	aggReq.Messages = append(aggReq.Messages, dto.Message{
		Role:    "user",
		Content: aggInstruction,
	})

	// 聚合分支：直接写真实下游，stream 沿用客户端原始设置
	aggTarget := model.VirtualModelTarget{
		Model:     agg.Model,
		ChannelId: agg.ChannelId,
		Group:     agg.Group,
	}
	aggOutcome := runVirtualBranch(c, info, aggTarget, aggReq, c.Writer, c.Request.Context())
	if aggOutcome.err != nil {
		return aggOutcome.err
	}
	if aggOutcome.info.VirtualBranchUsage != nil {
		usages = append(usages, aggOutcome.info.VirtualBranchUsage)
	}

	settleVirtualBilling(c, info, aggOutcome, sumUsages(usages))
	return nil
}

// sumUsages 汇总多个分支的 usage（质量模式统一结算）
func sumUsages(usages []*dto.Usage) *dto.Usage {
	var sum *dto.Usage
	for _, u := range usages {
		if u == nil {
			continue
		}
		if sum == nil {
			copied := *u
			sum = &copied
			continue
		}
		sum.PromptTokens += u.PromptTokens
		sum.CompletionTokens += u.CompletionTokens
		sum.TotalTokens += u.TotalTokens
		sum.PromptCacheHitTokens += u.PromptCacheHitTokens
		sum.PromptTokensDetails.CachedTokens += u.PromptTokensDetails.CachedTokens
		sum.PromptTokensDetails.AudioTokens += u.PromptTokensDetails.AudioTokens
		sum.CompletionTokenDetails.AudioTokens += u.CompletionTokenDetails.AudioTokens
	}
	return sum
}

// settleVirtualBilling 以主 RelayInfo 统一结算一次（渠道额度归属记到最后使用的分支渠道）
func settleVirtualBilling(c *gin.Context, mainInfo *relaycommon.RelayInfo, lastOutcome *virtualBranchOutcome, usage *dto.Usage) {
	if lastOutcome != nil && lastOutcome.info != nil && lastOutcome.info.ChannelMeta != nil {
		mainInfo.ChannelMeta = lastOutcome.info.ChannelMeta
	}
	service.PostTextConsumeQuota(c, mainInfo, usage, nil)
}
