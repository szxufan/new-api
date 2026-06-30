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

// TestSelectChannelFromList_EmptyList 测试空列表返回 nil
func TestSelectChannelFromList_EmptyList(t *testing.T) {
	resetCache()
	setupCache(map[int]*Channel{}, map[string]map[string][]int{})

	ch, err := selectChannelFromList([]int{}, 0)
	if ch != nil || err != nil {
		t.Errorf("expected (nil, nil), got (%v, %v)", ch, err)
	}
}

// TestSelectChannelFromList_SingleChannel 测试单渠道直接返回
func TestSelectChannelFromList_SingleChannel(t *testing.T) {
	resetCache()
	ch1 := newTestChannel(1, 1, 10, 100)
	setupCache(
		map[int]*Channel{1: ch1},
		map[string]map[string][]int{"g": {"m": {1}}},
	)

	ch, err := selectChannelFromList([]int{1}, 0)
	if err != nil || ch == nil || ch.Id != 1 {
		t.Errorf("expected channel #1, got (%v, %v)", ch, err)
	}
}

// TestSelectChannelFromList_SingleChannelMissing 测试单渠道不存在返回 error
func TestSelectChannelFromList_SingleChannelMissing(t *testing.T) {
	resetCache()
	setupCache(map[int]*Channel{}, map[string]map[string][]int{})

	ch, err := selectChannelFromList([]int{999}, 0)
	if ch != nil || err == nil {
		t.Errorf("expected (nil, error), got (%v, %v)", ch, err)
	}
}

// TestSelectChannelFromList_MultipleChannelsSamePriority 测试同优先级加权随机
func TestSelectChannelFromList_MultipleChannelsSamePriority(t *testing.T) {
	resetCache()
	ch1 := newTestChannel(1, 1, 10, 100)
	ch2 := newTestChannel(2, 1, 10, 100)
	setupCache(
		map[int]*Channel{1: ch1, 2: ch2},
		map[string]map[string][]int{"g": {"m": {1, 2}}},
	)

	// 运行多次，确保能返回其中一个
	for i := 0; i < 20; i++ {
		ch, err := selectChannelFromList([]int{1, 2}, 0)
		if err != nil || ch == nil || (ch.Id != 1 && ch.Id != 2) {
			t.Errorf("expected channel #1 or #2, got (%v, %v)", ch, err)
		}
	}
}

// TestSelectChannelFromList_MissingChannelInList 测试列表中渠道不存在返回 error
func TestSelectChannelFromList_MissingChannelInList(t *testing.T) {
	resetCache()
	ch1 := newTestChannel(1, 1, 10, 100)
	setupCache(
		map[int]*Channel{1: ch1},
		map[string]map[string][]int{"g": {"m": {1, 999}}},
	)

	ch, err := selectChannelFromList([]int{1, 999}, 0)
	if ch != nil || err == nil {
		t.Errorf("expected (nil, error) for missing channel, got (%v, %v)", ch, err)
	}
}

// TestGetRandomSatisfiedChannelWithPreference_EmptyPreferred 测试空偏好等价于原函数
func TestGetRandomSatisfiedChannelWithPreference_EmptyPreferred(t *testing.T) {
	// 此测试仅验证当 preferredTypes 为空时，不会 panic
	// 由于 MemoryCacheEnabled 在测试中默认为 false，会走 DB 路径
	resetCache()
	_, err, _ := GetRandomSatisfiedChannelWithPreference("g", "m", 0, nil)
	// 不关心结果，只验证不 panic
	_ = err
}

// TestGetRandomSatisfiedChannelWithPreference_PreferredTypeHit 测试优先类型命中
func TestGetRandomSatisfiedChannelWithPreference_PreferredTypeHit(t *testing.T) {
	// 需要 MemoryCacheEnabled 为 true 才能走内存缓存路径
	original := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	defer func() { common.MemoryCacheEnabled = original }()

	resetCache()
	// 构造两个渠道：OpenAI(type=1) 和 Anthropic(type=14)
	ch1 := newTestChannel(1, 1, 10, 100)   // OpenAI
	ch2 := newTestChannel(2, 14, 10, 100)  // Anthropic
	setupCache(
		map[int]*Channel{1: ch1, 2: ch2},
		map[string]map[string][]int{"g": {"m": {1, 2}}},
	)

	// 偏好 Anthropic 类型 (14)
	ch, err, _ := GetRandomSatisfiedChannelWithPreference("g", "m", 0, []int{14})
	if err != nil || ch == nil {
		t.Fatalf("expected channel, got (%v, %v)", ch, err)
	}
	if ch.Type != 14 {
		t.Errorf("expected channel type 14 (Anthropic), got type %d (channel id=%d)", ch.Type, ch.Id)
	}
}

// TestGetRandomSatisfiedChannelWithPreference_FallbackToAll 测试回退到全量渠道
func TestGetRandomSatisfiedChannelWithPreference_FallbackToAll(t *testing.T) {
	original := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	defer func() { common.MemoryCacheEnabled = original }()

	resetCache()
	// 只有 OpenAI 渠道，没有 Anthropic
	ch1 := newTestChannel(1, 1, 10, 100) // OpenAI
	setupCache(
		map[int]*Channel{1: ch1},
		map[string]map[string][]int{"g": {"m": {1}}},
	)

	// 偏好 Anthropic 类型 (14)，但只有 OpenAI，应回退
	ch, err, fellBack := GetRandomSatisfiedChannelWithPreference("g", "m", 0, []int{14})
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
}

// TestGetRandomSatisfiedChannelWithPreference_NoChannelsAtAll 测试无任何渠道
func TestGetRandomSatisfiedChannelWithPreference_NoChannelsAtAll(t *testing.T) {
	original := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	defer func() { common.MemoryCacheEnabled = original }()

	resetCache()
	setupCache(map[int]*Channel{}, map[string]map[string][]int{})

	ch, err, _ := GetRandomSatisfiedChannelWithPreference("g", "m", 0, []int{14})
	if err != nil || ch != nil {
		t.Errorf("expected (nil, nil), got (%v, %v)", ch, err)
	}
}

// TestGetRandomSatisfiedChannelWithPreference_MultiplePreferredTypes 测试多类型优先
func TestGetRandomSatisfiedChannelWithPreference_MultiplePreferredTypes(t *testing.T) {
	original := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	defer func() { common.MemoryCacheEnabled = original }()

	resetCache()
	// OpenAI(1), Azure(3), Anthropic(14)
	ch1 := newTestChannel(1, 1, 10, 100)
	ch2 := newTestChannel(2, 3, 10, 100)
	ch3 := newTestChannel(3, 14, 10, 100)
	setupCache(
		map[int]*Channel{1: ch1, 2: ch2, 3: ch3},
		map[string]map[string][]int{"g": {"m": {1, 2, 3}}},
	)

	// 偏好 [OpenAI(1), Azure(3)]，不应命中 Anthropic(14)
	for i := 0; i < 20; i++ {
		ch, err, _ := GetRandomSatisfiedChannelWithPreference("g", "m", 0, []int{1, 3})
		if err != nil || ch == nil {
			t.Fatalf("expected channel, got (%v, %v)", ch, err)
		}
		if ch.Type == 14 {
			t.Errorf("should not hit Anthropic (type=14), got channel id=%d type=%d", ch.Id, ch.Type)
		}
	}
}
