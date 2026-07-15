package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
)

// helper 用于构造带优先级和权重的渠道指针
func newTestChannel(id, chType int, priority int64, weight uint) *Channel {
	return &Channel{
		Id:       id,
		Type:     chType,
		Priority: &priority,
		Weight:   &weight,
		Status:   common.ChannelStatusEnabled,
	}
}

// resetCache 测试前重置全局缓存
func resetCache() {
	channelSyncLock.Lock()
	defer channelSyncLock.Unlock()
	group2model2channels = nil
	channelsIDM = nil
}

// setupCache 构造测试用内存缓存
func setupCache(channels map[int]*Channel, groupModelChannels map[string]map[string][]int) {
	channelSyncLock.Lock()
	defer channelSyncLock.Unlock()
	channelsIDM = channels
	group2model2channels = groupModelChannels
}

// withMemoryCache 临时开启内存缓存并在测试结束后恢复
func withMemoryCache(t *testing.T, fn func()) {
	t.Helper()
	original := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	defer func() { common.MemoryCacheEnabled = original }()
	fn()
}

// ---- sortChannelsByPriorityAndAffinity 单元测试 ----

// TestSortChannels_Empty 测试空切片不 panic
func TestSortChannels_Empty(t *testing.T) {
	sortChannelsByPriorityAndAffinity(nil, nil)
	sortChannelsByPriorityAndAffinity([]*Channel{}, nil)
}

// TestSortChannels_PriorityDesc 测试按优先级降序排序
func TestSortChannels_PriorityDesc(t *testing.T) {
	chLow := newTestChannel(1, 1, 1, 100)
	chHigh := newTestChannel(2, 1, 10, 100)
	channels := []*Channel{chLow, chHigh}

	sortChannelsByPriorityAndAffinity(channels, nil)
	if channels[0].Id != 2 || channels[1].Id != 1 {
		t.Errorf("expected [high(2), low(1)], got ids=[%d, %d]", channels[0].Id, channels[1].Id)
	}
}

// TestSortChannels_AffinityScoreAsc 测试同优先级内按亲和性得分升序排序
func TestSortChannels_AffinityScoreAsc(t *testing.T) {
	// channel 1: count=5, weight=100, score=0.05
	// channel 2: count=0, weight=100, score=0
	ch1 := newTestChannel(1, 1, 10, 100)
	ch2 := newTestChannel(2, 1, 10, 100)
	channels := []*Channel{ch1, ch2}
	affinityCounts := map[int]int64{1: 5, 2: 0}

	sortChannelsByPriorityAndAffinity(channels, affinityCounts)
	if channels[0].Id != 2 {
		t.Errorf("expected channel #2 (score=0) first, got #%d", channels[0].Id)
	}
}

// TestSortChannels_AffinityNil_KeepOrder 测试 affinityCounts 为 nil 时保持原顺序
func TestSortChannels_AffinityNil_KeepOrder(t *testing.T) {
	ch1 := newTestChannel(1, 1, 10, 100)
	ch2 := newTestChannel(2, 1, 10, 100)
	channels := []*Channel{ch1, ch2}

	sortChannelsByPriorityAndAffinity(channels, nil)
	// 同优先级、无亲和性，应保持原顺序
	if channels[0].Id != 1 || channels[1].Id != 2 {
		t.Errorf("expected original order [1, 2], got [%d, %d]", channels[0].Id, channels[1].Id)
	}
}

// TestSortChannels_DifferentWeight 测试不同权重时 score 最小的优先
func TestSortChannels_DifferentWeight(t *testing.T) {
	// channel 1: weight=200, count=10, score=0.05
	// channel 2: weight=100, count=3,  score=0.03
	ch1 := newTestChannel(1, 1, 10, 200)
	ch2 := newTestChannel(2, 1, 10, 100)
	channels := []*Channel{ch1, ch2}
	affinityCounts := map[int]int64{1: 10, 2: 3}

	sortChannelsByPriorityAndAffinity(channels, affinityCounts)
	if channels[0].Id != 2 {
		t.Errorf("expected channel #2 (score=0.03 < 0.05) first, got #%d", channels[0].Id)
	}
}

// TestSortChannels_WeightZero 测试权重为 0 时视为 100
func TestSortChannels_WeightZero(t *testing.T) {
	// channel 1: weight=0 → effectiveWeight=100, count=5, score=0.05
	// channel 2: weight=100, count=1, score=0.01
	ch1 := newTestChannel(1, 1, 10, 0)
	ch2 := newTestChannel(2, 1, 10, 100)
	channels := []*Channel{ch1, ch2}
	affinityCounts := map[int]int64{1: 5, 2: 1}

	sortChannelsByPriorityAndAffinity(channels, affinityCounts)
	if channels[0].Id != 2 {
		t.Errorf("expected channel #2 (score=0.01 < 0.05) first, got #%d", channels[0].Id)
	}
}

// TestSortChannels_MissingAffinityCount 测试渠道不在 affinityCounts 中时 count 视为 0
func TestSortChannels_MissingAffinityCount(t *testing.T) {
	// channel 1 在 map 中 count=10, channel 2 不在 map 中 → count=0
	ch1 := newTestChannel(1, 1, 10, 100)
	ch2 := newTestChannel(2, 1, 10, 100)
	channels := []*Channel{ch1, ch2}
	affinityCounts := map[int]int64{1: 10}

	sortChannelsByPriorityAndAffinity(channels, affinityCounts)
	if channels[0].Id != 2 {
		t.Errorf("expected channel #2 (count=0) first, got #%d", channels[0].Id)
	}
}

// ---- GetSortedChannelsByPriorityAndAffinity 集成测试 ----

// TestGetSortedChannels_EmptyCache 测试空缓存返回 nil
func TestGetSortedChannels_EmptyCache(t *testing.T) {
	withMemoryCache(t, func() {
		resetCache()
		setupCache(map[int]*Channel{}, map[string]map[string][]int{})
		channels := GetSortedChannelsByPriorityAndAffinity("g", "m", nil, nil)
		if channels != nil {
			t.Errorf("expected nil for empty cache, got %v", channels)
		}
	})
}

// TestGetSortedChannels_SingleChannel 测试单渠道返回包含该渠道
func TestGetSortedChannels_SingleChannel(t *testing.T) {
	withMemoryCache(t, func() {
		resetCache()
		ch1 := newTestChannel(1, 1, 10, 100)
		setupCache(
			map[int]*Channel{1: ch1},
			map[string]map[string][]int{"g": {"m": {1}}},
		)
		channels := GetSortedChannelsByPriorityAndAffinity("g", "m", nil, nil)
		if len(channels) != 1 || channels[0].Id != 1 {
			t.Errorf("expected [channel #1], got %v", channels)
		}
	})
}

// TestGetSortedChannels_PriorityOrder 测试多优先级按降序返回
func TestGetSortedChannels_PriorityOrder(t *testing.T) {
	withMemoryCache(t, func() {
		resetCache()
		chLow := newTestChannel(1, 1, 1, 100)
		chHigh := newTestChannel(2, 1, 10, 100)
		setupCache(
			map[int]*Channel{1: chLow, 2: chHigh},
			map[string]map[string][]int{"g": {"m": {1, 2}}},
		)
		channels := GetSortedChannelsByPriorityAndAffinity("g", "m", nil, nil)
		if len(channels) != 2 {
			t.Fatalf("expected 2 channels, got %d", len(channels))
		}
		if channels[0].Id != 2 || channels[1].Id != 1 {
			t.Errorf("expected [high(2), low(1)], got ids=[%d, %d]", channels[0].Id, channels[1].Id)
		}
	})
}

// TestGetSortedChannels_PreferredTypeFilter 测试 preferredTypes 过滤
func TestGetSortedChannels_PreferredTypeFilter(t *testing.T) {
	withMemoryCache(t, func() {
		resetCache()
		// OpenAI(type=1) 和 Anthropic(type=14)
		ch1 := newTestChannel(1, 1, 10, 100)
		ch2 := newTestChannel(2, 14, 10, 100)
		setupCache(
			map[int]*Channel{1: ch1, 2: ch2},
			map[string]map[string][]int{"g": {"m": {1, 2}}},
		)
		// 偏好 Anthropic 类型 (14)，应只返回 channel 2
		channels := GetSortedChannelsByPriorityAndAffinity("g", "m", []int{14}, nil)
		if len(channels) != 1 || channels[0].Type != 14 {
			t.Fatalf("expected only Anthropic(type=14) channel, got %v", channels)
		}
	})
}

// TestGetSortedChannels_PreferredTypeFallback 测试 preferredTypes 无匹配时回退全量
func TestGetSortedChannels_PreferredTypeFallback(t *testing.T) {
	withMemoryCache(t, func() {
		resetCache()
		// 只有 OpenAI 渠道，没有 Anthropic
		ch1 := newTestChannel(1, 1, 10, 100)
		setupCache(
			map[int]*Channel{1: ch1},
			map[string]map[string][]int{"g": {"m": {1}}},
		)
		// 偏好 Anthropic 类型 (14)，但只有 OpenAI，应回退返回全量
		channels := GetSortedChannelsByPriorityAndAffinity("g", "m", []int{14}, nil)
		if len(channels) != 1 || channels[0].Type != 1 {
			t.Errorf("expected fallback to OpenAI(type=1), got %v", channels)
		}
	})
}

// TestGetSortedChannels_AffinityOrdering 测试亲和性排序在完整路径生效
func TestGetSortedChannels_AffinityOrdering(t *testing.T) {
	withMemoryCache(t, func() {
		resetCache()
		// 同优先级，channel 1 有亲和性记录，channel 2 无
		ch1 := newTestChannel(1, 1, 10, 100)
		ch2 := newTestChannel(2, 1, 10, 100)
		setupCache(
			map[int]*Channel{1: ch1, 2: ch2},
			map[string]map[string][]int{"g": {"m": {1, 2}}},
		)
		affinityCounts := map[int]int64{1: 5, 2: 0}
		channels := GetSortedChannelsByPriorityAndAffinity("g", "m", nil, affinityCounts)
		if len(channels) != 2 {
			t.Fatalf("expected 2 channels, got %d", len(channels))
		}
		if channels[0].Id != 2 {
			t.Errorf("expected channel #2 (affinityCount=0) first, got #%d", channels[0].Id)
		}
	})
}

// ---- GetRandomSatisfiedChannelWithPreference 兼容性测试 ----

// TestGetRandomSatisfiedChannelWithPreference_EmptyPreferred 测试空偏好等价于原函数
func TestGetRandomSatisfiedChannelWithPreference_EmptyPreferred(t *testing.T) {
	resetCache()
	_, err, _ := GetRandomSatisfiedChannelWithPreference("g", "m", 0, nil, nil, nil)
	_ = err
}

// TestGetRandomSatisfiedChannelWithPreference_PreferredTypeHit 测试优先类型命中
func TestGetRandomSatisfiedChannelWithPreference_PreferredTypeHit(t *testing.T) {
	withMemoryCache(t, func() {
		resetCache()
		ch1 := newTestChannel(1, 1, 10, 100)  // OpenAI
		ch2 := newTestChannel(2, 14, 10, 100) // Anthropic
		setupCache(
			map[int]*Channel{1: ch1, 2: ch2},
			map[string]map[string][]int{"g": {"m": {1, 2}}},
		)

		ch, err, fellBack := GetRandomSatisfiedChannelWithPreference("g", "m", 0, []int{14}, nil, nil)
		if err != nil || ch == nil {
			t.Fatalf("expected channel, got (%v, %v)", ch, err)
		}
		if ch.Type != 14 {
			t.Errorf("expected channel type 14 (Anthropic), got type %d (channel id=%d)", ch.Type, ch.Id)
		}
		if fellBack {
			t.Errorf("expected fellBack=false for type hit, got true")
		}
	})
}

// TestGetRandomSatisfiedChannelWithPreference_FallbackToAll 测试回退到全量渠道
func TestGetRandomSatisfiedChannelWithPreference_FallbackToAll(t *testing.T) {
	withMemoryCache(t, func() {
		resetCache()
		ch1 := newTestChannel(1, 1, 10, 100) // OpenAI
		setupCache(
			map[int]*Channel{1: ch1},
			map[string]map[string][]int{"g": {"m": {1}}},
		)

		ch, err, fellBack := GetRandomSatisfiedChannelWithPreference("g", "m", 0, []int{14}, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ch == nil {
			t.Fatalf("expected fallback channel, got nil")
		}
		if ch.Type != 1 {
			t.Errorf("expected fallback to OpenAI (type=1), got type %d", ch.Type)
		}
		if !fellBack {
			t.Errorf("expected fellBack=true, got false")
		}
	})
}

// TestGetRandomSatisfiedChannelWithPreference_NoChannelsAtAll 测试无任何渠道
func TestGetRandomSatisfiedChannelWithPreference_NoChannelsAtAll(t *testing.T) {
	withMemoryCache(t, func() {
		resetCache()
		setupCache(map[int]*Channel{}, map[string]map[string][]int{})

		ch, err, _ := GetRandomSatisfiedChannelWithPreference("g", "m", 0, []int{14}, nil, nil)
		if err != nil || ch != nil {
			t.Errorf("expected (nil, nil), got (%v, %v)", ch, err)
		}
	})
}

// TestGetRandomSatisfiedChannelWithPreference_MultiplePreferredTypes 测试多类型优先
func TestGetRandomSatisfiedChannelWithPreference_MultiplePreferredTypes(t *testing.T) {
	withMemoryCache(t, func() {
		resetCache()
		ch1 := newTestChannel(1, 1, 10, 100)
		ch2 := newTestChannel(2, 3, 10, 100)
		ch3 := newTestChannel(3, 14, 10, 100)
		setupCache(
			map[int]*Channel{1: ch1, 2: ch2, 3: ch3},
			map[string]map[string][]int{"g": {"m": {1, 2, 3}}},
		)

		for i := 0; i < 20; i++ {
			ch, err, _ := GetRandomSatisfiedChannelWithPreference("g", "m", 0, []int{1, 3}, nil, nil)
			if err != nil || ch == nil {
				t.Fatalf("expected channel, got (%v, %v)", ch, err)
			}
			if ch.Type == 14 {
				t.Errorf("should not hit Anthropic (type=14), got channel id=%d type=%d", ch.Id, ch.Type)
			}
		}
	})
}
