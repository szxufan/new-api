package service

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/bytedance/gopkg/util/gopool"
)

// 渠道定时开关任务：按渠道配置的定时开启时段（time_windows）自动切换渠道状态，
// 用于适配峰谷电价等按时段启停渠道的场景。
// 仅在窗口内启用 / 窗口外定时关闭之间切换，不干预手动禁用、自动禁用、限流等状态。

const channelScheduleTickInterval = 30 * time.Second

var (
	channelScheduleOnce    sync.Once
	channelScheduleRunning atomic.Bool
)

func StartChannelScheduleTask() {
	channelScheduleOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("channel schedule task started: tick=%s", channelScheduleTickInterval))
			ticker := time.NewTicker(channelScheduleTickInterval)
			defer ticker.Stop()
			for range ticker.C {
				runChannelScheduleTick()
			}
		})
	})
}

// scheduleAction 定时调度决策结果
type scheduleAction int

const (
	scheduleNoop    scheduleAction = iota // 保持现状
	scheduleEnable                        // 恢复启用
	scheduleDisable                       // 定时关闭
)

// resolveScheduleAction 依据时段配置与渠道当前状态计算应执行的动作。
// 无有效时段配置的渠道不应停留在定时关闭状态（配置已被清空或非法时恢复启用）。
func resolveScheduleAction(windows []dto.TimeWindow, status int, now time.Time) scheduleAction {
	if len(windows) == 0 {
		if status == common.ChannelStatusScheduledDisabled {
			return scheduleEnable
		}
		return scheduleNoop
	}
	inWindow := dto.IsInTimeWindows(windows, now)
	switch {
	case inWindow && status == common.ChannelStatusScheduledDisabled:
		return scheduleEnable
	case !inWindow && status == common.ChannelStatusEnabled:
		return scheduleDisable
	default:
		// 其他状态（手动禁用/自动禁用/限流）一律不干预
		return scheduleNoop
	}
}

func runChannelScheduleTick() {
	if !channelScheduleRunning.CompareAndSwap(false, true) {
		return
	}
	defer channelScheduleRunning.Store(false)

	channels, err := model.GetTimeWindowChannels()
	if err != nil {
		common.SysLog(fmt.Sprintf("channel schedule task failed to load channels: %v", err))
		return
	}
	now := time.Now()
	for _, channel := range channels {
		windows := channel.GetTimeWindowList()
		switch resolveScheduleAction(windows, channel.Status, now) {
		case scheduleEnable:
			// 进入开启时段（或时段配置已被移除），恢复启用
			if model.UpdateChannelScheduleStatus(channel.Id, common.ChannelStatusEnabled, "") {
				common.SysLog(fmt.Sprintf("channel #%d scheduled switching: enabled", channel.Id))
			}
		case scheduleDisable:
			// 离开开启时段，定时关闭
			if model.UpdateChannelScheduleStatus(channel.Id, common.ChannelStatusScheduledDisabled, "scheduled off: outside configured time windows") {
				common.SysLog(fmt.Sprintf("channel #%d scheduled switching: disabled (outside time windows)", channel.Id))
			}
		}
	}
}
