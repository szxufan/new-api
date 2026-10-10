package dto

import (
	"fmt"
	"time"
)

// ChannelPriceSettings 渠道价格系数（存储于渠道 setting JSON 的 price 字段）。
// 系数为乘数：未配置（nil）= 1；显式 0 表示免费；合法值 >= 0。
// 最终价格 = 常规计算结果 × 单项系数（按项） × Total × 命中时段系数。
type ChannelPriceSettings struct {
	Total       *float64          `json:"total,omitempty"`        // 总价格系数
	Input       *float64          `json:"input,omitempty"`        // 输入
	Completion  *float64          `json:"completion,omitempty"`   // 补全
	CacheRead   *float64          `json:"cache_read,omitempty"`   // 缓存读
	CacheWrite  *float64          `json:"cache_write,omitempty"`  // 缓存写
	ImageInput  *float64          `json:"image_input,omitempty"`  // 图像输入
	AudioInput  *float64          `json:"audio_input,omitempty"`  // 音频输入
	AudioOutput *float64          `json:"audio_output,omitempty"` // 音频输出
	TimeWindows []PriceTimeWindow `json:"time_windows,omitempty"` // 时间段系数（按顺序取第一个命中项生效）
}

// PriceTimeWindow 时间段价格系数。End <= Start 表示跨天段（如 22:00-08:00），
// 覆盖 [22:00, 24:00) ∪ [00:00, 08:00)。
type PriceTimeWindow struct {
	TimeWindow
	Ratio float64 `json:"ratio"` // 该时段价格系数（>= 0，0=免费）
}

// PriceFactors 归一化后的渠道价格系数集合（未配置项为 1）。
// 时间基准使用时刻自身的时区（与渠道定时开启时段一致，取服务器本地时间）。
type PriceFactors struct {
	Total       float64
	Time        float64
	Input       float64
	Completion  float64
	CacheRead   float64
	CacheWrite  float64
	ImageInput  float64
	AudioInput  float64
	AudioOutput float64
}

// IdentityPriceFactors 返回全部为 1（不调整价格）的系数集合
func IdentityPriceFactors() PriceFactors {
	return PriceFactors{
		Total:       1,
		Time:        1,
		Input:       1,
		Completion:  1,
		CacheRead:   1,
		CacheWrite:  1,
		ImageInput:  1,
		AudioInput:  1,
		AudioOutput: 1,
	}
}

// Overall 通用乘数：总价格系数 × 命中时段系数
func (f PriceFactors) Overall() float64 {
	return f.Total * f.Time
}

// IsIdentity 是否所有系数均为 1（无需调整价格）
func (f PriceFactors) IsIdentity() bool {
	return f == IdentityPriceFactors()
}

// ToMap 返回非 1 的系数（键名与存储 JSON 字段一致，用于日志展示）
func (f PriceFactors) ToMap() map[string]float64 {
	identity := IdentityPriceFactors()
	result := make(map[string]float64)
	if f.Total != identity.Total {
		result["total"] = f.Total
	}
	if f.Time != identity.Time {
		result["time_window"] = f.Time
	}
	if f.Input != identity.Input {
		result["input"] = f.Input
	}
	if f.Completion != identity.Completion {
		result["completion"] = f.Completion
	}
	if f.CacheRead != identity.CacheRead {
		result["cache_read"] = f.CacheRead
	}
	if f.CacheWrite != identity.CacheWrite {
		result["cache_write"] = f.CacheWrite
	}
	if f.ImageInput != identity.ImageInput {
		result["image_input"] = f.ImageInput
	}
	if f.AudioInput != identity.AudioInput {
		result["audio_input"] = f.AudioInput
	}
	if f.AudioOutput != identity.AudioOutput {
		result["audio_output"] = f.AudioOutput
	}
	return result
}

// Validate 校验价格系数：单项/总系数 >= 0；时段格式合法、start != end、
// 系数 >= 0、段数不超过 MaxTimeWindows。
func (p *ChannelPriceSettings) Validate() error {
	if p == nil {
		return nil
	}
	ratios := []struct {
		name  string
		value *float64
	}{
		{"total", p.Total},
		{"input", p.Input},
		{"completion", p.Completion},
		{"cache_read", p.CacheRead},
		{"cache_write", p.CacheWrite},
		{"image_input", p.ImageInput},
		{"audio_input", p.AudioInput},
		{"audio_output", p.AudioOutput},
	}
	for _, r := range ratios {
		if r.value != nil && *r.value < 0 {
			return fmt.Errorf("price.%s must be >= 0", r.name)
		}
	}
	if len(p.TimeWindows) > MaxTimeWindows {
		return fmt.Errorf("too many price time windows, max %d", MaxTimeWindows)
	}
	for i, w := range p.TimeWindows {
		if err := w.TimeWindow.Validate(); err != nil {
			return fmt.Errorf("price time window #%d: %s", i+1, err.Error())
		}
		if w.Ratio < 0 {
			return fmt.Errorf("price time window #%d: ratio must be >= 0", i+1)
		}
	}
	return nil
}

// Resolve 归一化为 PriceFactors：nil 系数视为 1；按配置顺序取第一个命中的
// 时段系数（无命中则为 1）。p 为 nil 时返回全 1。
func (p *ChannelPriceSettings) Resolve(at time.Time) PriceFactors {
	factors := IdentityPriceFactors()
	if p == nil {
		return factors
	}
	factors.Total = ratioOrDefault(p.Total)
	factors.Input = ratioOrDefault(p.Input)
	factors.Completion = ratioOrDefault(p.Completion)
	factors.CacheRead = ratioOrDefault(p.CacheRead)
	factors.CacheWrite = ratioOrDefault(p.CacheWrite)
	factors.ImageInput = ratioOrDefault(p.ImageInput)
	factors.AudioInput = ratioOrDefault(p.AudioInput)
	factors.AudioOutput = ratioOrDefault(p.AudioOutput)
	for _, w := range p.TimeWindows {
		if isInTimeWindow(w.TimeWindow, at) {
			factors.Time = w.Ratio
			break
		}
	}
	return factors
}

func ratioOrDefault(r *float64) float64 {
	if r == nil {
		return 1
	}
	return *r
}