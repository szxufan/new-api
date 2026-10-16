package service

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/bytedance/gopkg/util/gopool"
)

const (
	channelBalanceResetTickInterval = 1 * time.Minute
	channelBalanceResetBatchSize    = 300
)

var (
	channelBalanceResetOnce    sync.Once
	channelBalanceResetRunning atomic.Bool
)

// StartChannelBalanceResetTask 启动渠道周期重置余额任务（仅主节点）。
// 每分钟扫描一次到期渠道，将余额重置为配置金额并推进下次重置时间。
func StartChannelBalanceResetTask() {
	channelBalanceResetOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("channel balance reset task started: tick=%s", channelBalanceResetTickInterval))
			ticker := time.NewTicker(channelBalanceResetTickInterval)
			defer ticker.Stop()

			runChannelBalanceResetOnce()
			for range ticker.C {
				runChannelBalanceResetOnce()
			}
		})
	})
}

func runChannelBalanceResetOnce() {
	if !channelBalanceResetRunning.CompareAndSwap(false, true) {
		return
	}
	defer channelBalanceResetRunning.Store(false)

	ctx := context.Background()
	totalReset := 0
	for {
		n, err := model.ResetDueChannelBalances(channelBalanceResetBatchSize)
		if err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("channel balance reset task failed: %v", err))
			return
		}
		if n == 0 {
			break
		}
		totalReset += n
		if n < channelBalanceResetBatchSize {
			break
		}
	}
	if totalReset > 0 {
		logger.LogInfo(ctx, fmt.Sprintf("channel balance reset task: reset_count=%d", totalReset))
	}
}
