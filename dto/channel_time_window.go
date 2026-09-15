package dto

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// TimeWindow 渠道定时开启时段（每日重复）。End <= Start 表示跨天段，如 22:00-08:00
// 覆盖 [22:00, 24:00) ∪ [00:00, 08:00)。
type TimeWindow struct {
	Start string `json:"start"` // HH:mm
	End   string `json:"end"`   // HH:mm
}

// MaxTimeWindows 单渠道允许配置的最大时段数
const MaxTimeWindows = 20

// ParseTimeWindows 解析定时开启时段 JSON（如 [{"start":"22:00","end":"08:00"}]），
// 空字符串返回 nil（未启用）。同时做格式与业务校验。
func ParseTimeWindows(data string) ([]TimeWindow, error) {
	trimmed := strings.TrimSpace(data)
	if trimmed == "" {
		return nil, nil
	}
	var windows []TimeWindow
	if err := common.Unmarshal([]byte(trimmed), &windows); err != nil {
		return nil, fmt.Errorf("must be a json array of {start, end}: %s", err.Error())
	}
	if len(windows) > MaxTimeWindows {
		return nil, fmt.Errorf("too many time windows, max %d", MaxTimeWindows)
	}
	for i, w := range windows {
		if err := w.Validate(); err != nil {
			return nil, fmt.Errorf("time window #%d: %s", i+1, err.Error())
		}
	}
	return windows, nil
}

// Validate 校验单个时段：HH:mm 格式合法且 start != end
func (w TimeWindow) Validate() error {
	start, err := parseHHmm(w.Start)
	if err != nil {
		return fmt.Errorf("start %s", err.Error())
	}
	end, err := parseHHmm(w.End)
	if err != nil {
		return fmt.Errorf("end %s", err.Error())
	}
	if start == end {
		return fmt.Errorf("start and end cannot be the same")
	}
	return nil
}

// IsInTimeWindows 判断时刻 t（使用其本地时区）是否落在任一时段内
func IsInTimeWindows(windows []TimeWindow, t time.Time) bool {
	minutes := t.Hour()*60 + t.Minute()
	for _, w := range windows {
		start, err1 := parseHHmm(w.Start)
		end, err2 := parseHHmm(w.End)
		if err1 != nil || err2 != nil {
			continue
		}
		if start < end {
			if minutes >= start && minutes < end {
				return true
			}
		} else if start > end {
			// 跨天段
			if minutes >= start || minutes < end {
				return true
			}
		}
	}
	return false
}

// parseHHmm 解析 "HH:mm" 为当日分钟数
func parseHHmm(s string) (int, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("must be in HH:mm format")
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return 0, fmt.Errorf("hour must be in [00, 23]")
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return 0, fmt.Errorf("minute must be in [00, 59]")
	}
	return hour*60 + minute, nil
}
