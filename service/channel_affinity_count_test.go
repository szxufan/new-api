package service

import (
	"testing"
)

// resetAffinityCountMapForTest 清空全局亲和性计数 map，确保测试间互不影响。
func resetAffinityCountMapForTest() {
	resetChannelAffinityCounts()
}

// TestChannelAffinityCount_IncrementDecrementAndGet 测试递增、递减和获取计数
func TestChannelAffinityCount_IncrementDecrementAndGet(t *testing.T) {
	resetAffinityCountMapForTest()

	// 初始为 0
	counts := GetChannelAffinityCounts([]int{1, 2})
	if counts[1] != 0 || counts[2] != 0 {
		t.Errorf("expected 0, got channel1=%d, channel2=%d", counts[1], counts[2])
	}

	// 递增 channel 1 三次
	incrementChannelAffinityCount(1)
	incrementChannelAffinityCount(1)
	incrementChannelAffinityCount(1)
	// 递增 channel 2 一次
	incrementChannelAffinityCount(2)

	counts = GetChannelAffinityCounts([]int{1, 2})
	if counts[1] != 3 {
		t.Errorf("expected channel 1 count=3, got %d", counts[1])
	}
	if counts[2] != 1 {
		t.Errorf("expected channel 2 count=1, got %d", counts[2])
	}

	// 递减 channel 1 一次
	decrementChannelAffinityCount(1)

	counts = GetChannelAffinityCounts([]int{1, 2})
	if counts[1] != 2 {
		t.Errorf("expected channel 1 count=2 after decrement, got %d", counts[1])
	}
	if counts[2] != 1 {
		t.Errorf("expected channel 2 count=1, got %d", counts[2])
	}

	resetAffinityCountMapForTest()
}

// TestChannelAffinityCount_Reset 测试重置计数
func TestChannelAffinityCount_Reset(t *testing.T) {
	resetAffinityCountMapForTest()

	incrementChannelAffinityCount(10)
	incrementChannelAffinityCount(20)
	incrementChannelAffinityCount(30)

	counts := GetChannelAffinityCounts([]int{10, 20, 30})
	if counts[10] != 1 || counts[20] != 1 || counts[30] != 1 {
		t.Errorf("expected all counts=1, got %d, %d, %d", counts[10], counts[20], counts[30])
	}

	resetChannelAffinityCounts()

	counts = GetChannelAffinityCounts([]int{10, 20, 30})
	if counts[10] != 0 || counts[20] != 0 || counts[30] != 0 {
		t.Errorf("expected all counts=0 after reset, got %d, %d, %d", counts[10], counts[20], counts[30])
	}
}

// TestChannelAffinityCount_DecrementNotBelowZero 测试递减不低于 0
func TestChannelAffinityCount_DecrementNotBelowZero(t *testing.T) {
	resetAffinityCountMapForTest()

	// 不存在的渠道递减不应 panic
	decrementChannelAffinityCount(999)

	// 递增一次后递减两次，计数不应低于 0
	incrementChannelAffinityCount(1)
	decrementChannelAffinityCount(1)
	decrementChannelAffinityCount(1)
	decrementChannelAffinityCount(1)

	counts := GetChannelAffinityCounts([]int{1})
	if counts[1] != 0 {
		t.Errorf("expected count=0 (not below zero), got %d", counts[1])
	}

	resetAffinityCountMapForTest()
}

// TestChannelAffinityCount_GetMemNil 测试空 channelIDs 列表
func TestChannelAffinityCount_GetMemNil(t *testing.T) {
	resetAffinityCountMapForTest()

	counts := getChannelAffinityCountsMem(nil)
	if len(counts) != 0 {
		t.Errorf("expected empty map for nil channelIDs, got %v", counts)
	}

	counts = getChannelAffinityCountsMem([]int{})
	if len(counts) != 0 {
		t.Errorf("expected empty map for empty channelIDs, got %v", counts)
	}

	resetAffinityCountMapForTest()
}
