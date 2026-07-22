package relay

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestGinWriter() (gin.ResponseWriter, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	return c.Writer, recorder
}

// 胜者：首字节信号 → 裁决 → 回放缓冲并直通
func TestRaceWriterWinner(t *testing.T) {
	target, recorder := newTestGinWriter()
	firstCh := make(chan int, 1)
	decision := make(chan bool, 1)

	w := newRaceWriter(0, firstCh, decision)
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = w.Write([]byte("data: first\n\n"))
		_, _ = w.Write([]byte("data: second\n\n"))
		w.Flush()
	}()

	// 收到首字节信号
	select {
	case idx := <-firstCh:
		assert.Equal(t, 0, idx)
	case <-time.After(2 * time.Second):
		t.Fatal("未收到首字节信号")
	}
	// 裁决前不应有内容写出
	assert.Empty(t, recorder.Body.String())

	// 裁决为胜者
	w.commit(target)
	decision <- true

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("胜者 Write 未在裁决后放行")
	}

	body := recorder.Body.String()
	assert.Contains(t, body, "data: first")
	assert.Contains(t, body, "data: second")
	assert.Equal(t, "text/event-stream", recorder.Header().Get("Content-Type"))
}

// 败者：裁决后内容被丢弃，不写入真实下游
func TestRaceWriterLoser(t *testing.T) {
	_, recorder := newTestGinWriter()
	firstCh := make(chan int, 1)
	decision := make(chan bool, 1)

	w := newRaceWriter(1, firstCh, decision)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = w.Write([]byte("data: loser\n\n"))
		_, _ = w.Write([]byte("data: loser2\n\n"))
	}()

	select {
	case <-firstCh:
	case <-time.After(2 * time.Second):
		t.Fatal("未收到首字节信号")
	}
	decision <- false

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("败者 Write 未在裁决后放行")
	}
	assert.Empty(t, recorder.Body.String(), "败者内容不得写入真实下游")
}

// 并发分支同时写入：只有一个胜者回放（go test -race 下验证互斥安全）
func TestRaceWriterConcurrent(t *testing.T) {
	target, recorder := newTestGinWriter()
	firstCh := make(chan int, 2)
	decisions := []chan bool{make(chan bool, 1), make(chan bool, 1)}
	writers := []*raceWriter{
		newRaceWriter(0, firstCh, decisions[0]),
		newRaceWriter(1, firstCh, decisions[1]),
	}

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, _ = writers[idx].Write([]byte("data: x\n\n"))
		}(i)
	}

	winner := <-firstCh
	writers[winner].commit(target)
	for j := range decisions {
		decisions[j] <- (j == winner)
	}
	wg.Wait()
	// 只有胜者的内容被回放一次
	assert.Equal(t, "data: x\n\n", recorder.Body.String())
}

// captureWriter 缓冲全部写入
func TestCaptureWriter(t *testing.T) {
	w := newCaptureWriter()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte(`{"choices":[{"message":{"content":"hello"}}]}`))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Status())
	assert.True(t, w.Written())
	assert.JSONEq(t, `{"choices":[{"message":{"content":"hello"}}]}`, w.buf.String())
}

// sumUsages 汇总多个分支 usage
func TestSumUsages(t *testing.T) {
	assert.Nil(t, sumUsages(nil))
	assert.Nil(t, sumUsages([]*dto.Usage{nil, nil}))

	sum := sumUsages([]*dto.Usage{
		{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
		{PromptTokens: 20, CompletionTokens: 10, TotalTokens: 30, PromptCacheHitTokens: 3},
		nil,
		{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
	})
	require.NotNil(t, sum)
	assert.Equal(t, 31, sum.PromptTokens)
	assert.Equal(t, 16, sum.CompletionTokens)
	assert.Equal(t, 47, sum.TotalTokens)
	assert.Equal(t, 3, sum.PromptCacheHitTokens)
}

// prepareBranchRequest：改写模型名 + 强制非流式
func TestPrepareBranchRequest(t *testing.T) {
	info := &relaycommon.RelayInfo{
		Request: &dto.GeneralOpenAIRequest{
			Model: "virtual-auto",
			Messages: []dto.Message{
				{Role: "user", Content: "hi"},
			},
		},
	}

	branchReq, apiErr := prepareBranchRequest(info, "gpt-4o", false)
	require.Nil(t, apiErr)
	assert.Equal(t, "gpt-4o", branchReq.Model)
	// 深拷贝：不修改原请求
	assert.Equal(t, "virtual-auto", info.Request.(*dto.GeneralOpenAIRequest).Model)

	branchReq2, apiErr := prepareBranchRequest(info, "gpt-4o-mini", true)
	require.Nil(t, apiErr)
	require.NotNil(t, branchReq2.Stream)
	assert.False(t, *branchReq2.Stream)

	// 非 chat completions 请求类型应报错
	badInfo := &relaycommon.RelayInfo{Request: &dto.RerankRequest{Model: "x"}}
	_, apiErr = prepareBranchRequest(badInfo, "gpt-4o", false)
	assert.NotNil(t, apiErr)
}

// 默认聚合模板必须包含 {{answers}} 占位符
func TestDefaultAggregatorPromptTemplate(t *testing.T) {
	assert.Contains(t, defaultAggregatorPromptTemplate, "{{answers}}")
}
