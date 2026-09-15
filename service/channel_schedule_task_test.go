package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

func scheduleTimeAt(hour, minute int) time.Time {
	return time.Date(2026, 9, 15, hour, minute, 0, 0, time.Local)
}

func TestResolveScheduleAction(t *testing.T) {
	sameDay := []dto.TimeWindow{{Start: "12:00", End: "14:00"}}
	crossDay := []dto.TimeWindow{{Start: "22:00", End: "08:00"}}

	cases := []struct {
		name    string
		windows []dto.TimeWindow
		status  int
		now     time.Time
		want    scheduleAction
	}{
		// 无时段配置：只有定时关闭状态需要恢复，其余不动
		{"no windows + scheduled disabled recovers", nil, common.ChannelStatusScheduledDisabled, scheduleTimeAt(16, 0), scheduleEnable},
		{"no windows + empty slice + scheduled disabled recovers", []dto.TimeWindow{}, common.ChannelStatusScheduledDisabled, scheduleTimeAt(16, 0), scheduleEnable},
		{"no windows + enabled stays", nil, common.ChannelStatusEnabled, scheduleTimeAt(16, 0), scheduleNoop},
		{"no windows + manually disabled stays", nil, common.ChannelStatusManuallyDisabled, scheduleTimeAt(16, 0), scheduleNoop},
		// 有时段配置：仅在 启用↔定时关闭 之间切换
		{"in window + scheduled disabled recovers", crossDay, common.ChannelStatusScheduledDisabled, scheduleTimeAt(23, 0), scheduleEnable},
		{"in window + enabled stays", crossDay, common.ChannelStatusEnabled, scheduleTimeAt(23, 0), scheduleNoop},
		{"outside window + enabled disables", sameDay, common.ChannelStatusEnabled, scheduleTimeAt(16, 0), scheduleDisable},
		{"outside window + scheduled disabled stays", sameDay, common.ChannelStatusScheduledDisabled, scheduleTimeAt(16, 0), scheduleNoop},
		// 手动禁用/自动禁用/限流状态一律不干预
		{"outside window + manually disabled untouched", sameDay, common.ChannelStatusManuallyDisabled, scheduleTimeAt(16, 0), scheduleNoop},
		{"outside window + auto disabled untouched", sameDay, common.ChannelStatusAutoDisabled, scheduleTimeAt(16, 0), scheduleNoop},
		{"outside window + rate limited untouched", sameDay, common.ChannelStatusRateLimited429, scheduleTimeAt(16, 0), scheduleNoop},
		{"in window + manually disabled untouched", crossDay, common.ChannelStatusManuallyDisabled, scheduleTimeAt(23, 0), scheduleNoop},
		// 边界时刻
		{"window start boundary disables after", sameDay, common.ChannelStatusEnabled, scheduleTimeAt(14, 0), scheduleDisable},
		{"window start boundary exact stays enabled", sameDay, common.ChannelStatusEnabled, scheduleTimeAt(12, 0), scheduleNoop},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := resolveScheduleAction(c.windows, c.status, c.now); got != c.want {
				t.Fatalf("resolveScheduleAction() = %v, want %v", got, c.want)
			}
		})
	}
}
