package controller

import (
	"math/rand"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

// TestRetryIntervalMs_NoWait 验证 RetryIntervalMs <= 0 或 attempt < 1 时不等待（返回 0）。
func TestRetryIntervalMs_NoWait(t *testing.T) {
	orig := common.RetryIntervalMs
	defer func() { common.RetryIntervalMs = orig }()

	tests := []struct {
		name    string
		ms      int
		attempt int
	}{
		{"retry interval zero", 0, 1},
		{"retry interval negative", -100, 3},
		{"attempt below one zero", 500, 0},
		{"attempt below one negative", 500, -2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			common.RetryIntervalMs = tc.ms
			require.Equal(t, time.Duration(0), retryIntervalMs(tc.attempt),
				"expected no wait for ms=%d attempt=%d", tc.ms, tc.attempt)
		})
	}
}

// TestRetryIntervalMs_JitterRange 验证等待时长落在基础值的 [0.8, 1.2] 区间内。
func TestRetryIntervalMs_JitterRange(t *testing.T) {
	orig := common.RetryIntervalMs
	defer func() { common.RetryIntervalMs = orig }()
	rand.Seed(42)

	common.RetryIntervalMs = 500
	const attempt = 1
	baseMs := float64(common.RetryIntervalMs * attempt)
	const sampleCount = 2000

	for i := 0; i < sampleCount; i++ {
		wait := retryIntervalMs(attempt)
		gotMs := float64(wait) / float64(time.Millisecond)
		require.GreaterOrEqual(t, gotMs, baseMs*0.8-1.0,
			"wait should not be below base*0.8")
		require.LessOrEqual(t, gotMs, baseMs*1.2+1.0,
			"wait should not exceed base*1.2")
	}
}

// TestRetryIntervalMs_Increasing 验证递增性：基础等待随 attempt 线性增长，
// 且 attempt 越大均值越接近 RetryIntervalMs * attempt。
func TestRetryIntervalMs_Increasing(t *testing.T) {
	orig := common.RetryIntervalMs
	defer func() { common.RetryIntervalMs = orig }()
	rand.Seed(7)

	common.RetryIntervalMs = 200
	const sampleCount = 5000

	// 对多个 attempt 求平均等待时长，验证累进关系
	mean := func(attempt int) float64 {
		var sum float64
		for i := 0; i < sampleCount; i++ {
			sum += float64(retryIntervalMs(attempt))
		}
		return sum / float64(sampleCount)
	}

	mean1 := mean(1)
	mean2 := mean(2)
	mean3 := mean(3)

	// 期望值逐次翻倍（均值收敛于 RetryIntervalMs*attempt）
	expected1 := float64(common.RetryIntervalMs*1 * int(time.Millisecond))
	expected2 := float64(common.RetryIntervalMs*2 * int(time.Millisecond))
	expected3 := float64(common.RetryIntervalMs*3 * int(time.Millisecond))

	// 均值应落在期望值的 ±6% 内（抖动 t应用 ±20%，均值收敛于中值）
	require.InDelta(t, expected1, mean1, expected1*0.06)
	require.InDelta(t, expected2, mean2, expected2*0.06)
	require.InDelta(t, expected3, mean3, expected3*0.06)
}