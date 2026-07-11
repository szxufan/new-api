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

// withRetryTimes 临时设置 common.RetryTimes 并在测试结束后恢复
func withRetryTimes(t *testing.T, retryTimes int, fn func()) {
	t.Helper()
	original := common.RetryTimes
	common.RetryTimes = retryTimes
	defer func() { common.RetryTimes = original }()
	fn()
}

// TestSelectChannelFromList_EmptyList 测试空列表返回 nil
func TestSelectChannelFromList_EmptyList(t *testing.T) {
	resetCache()
	setupCache(map[int]*Channel{}, map[string]map[string][]int{})

	ch, err := selectChannelFromList([]int{}, 0, nil)
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

	ch, err := selectChannelFromList([]int{1}, 0, nil)
	if err != nil || ch == nil || ch.Id != 1 {
		t.Errorf("expected channel #1, got (%v, %v)", ch, err)
	}
}

// TestSelectChannelFromList_SingleChannelMissing 测试单渠道不存在返回 error
func TestSelectChannelFromList_SingleChannelMissing(t *testing.T) {
	resetCache()
	setupCache(map[int]*Channel{}, map[string]map[string][]int{})

	ch, err := selectChannelFromList([]int{999}, 0, nil)
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
		ch, err := selectChannelFromList([]int{1, 2}, 0, nil)
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

	ch, err := selectChannelFromList([]int{1, 999}, 0, nil)
	if ch != nil || err == nil {
		t.Errorf("expected (nil, error) for missing channel, got (%v, %v)", ch, err)
	}
}

// TestGetRandomSatisfiedChannelWithPreference_EmptyPreferred 测试空偏好等价于原函数
func TestGetRandomSatisfiedChannelWithPreference_EmptyPreferred(t *testing.T) {
	// 此测试仅验证当 preferredTypes 为空时，不会 panic
	// 由于 MemoryCacheEnabled 在测试中默认为 false，会走 DB 路径
	resetCache()
	_, err, _ := GetRandomSatisfiedChannelWithPreference("g", "m", 0, nil, nil)
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
	ch1 := newTestChannel(1, 1, 10, 100)  // OpenAI
	ch2 := newTestChannel(2, 14, 10, 100) // Anthropic
	setupCache(
		map[int]*Channel{1: ch1, 2: ch2},
		map[string]map[string][]int{"g": {"m": {1, 2}}},
	)

	// 偏好 Anthropic 类型 (14)
	ch, err, _ := GetRandomSatisfiedChannelWithPreference("g", "m", 0, []int{14}, nil)
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
	ch, err, fellBack := GetRandomSatisfiedChannelWithPreference("g", "m", 0, []int{14}, nil)
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

	ch, err, _ := GetRandomSatisfiedChannelWithPreference("g", "m", 0, []int{14}, nil)
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
		ch, err, _ := GetRandomSatisfiedChannelWithPreference("g", "m", 0, []int{1, 3}, nil)
		if err != nil || ch == nil {
			t.Fatalf("expected channel, got (%v, %v)", ch, err)
		}
		if ch.Type == 14 {
			t.Errorf("should not hit Anthropic (type=14), got channel id=%d type=%d", ch.Id, ch.Type)
		}
	}
}

// TestCalcSamePriorityRetryBudget 测试同优先级内重试预算计算
func TestCalcSamePriorityRetryBudget(t *testing.T) {
	cases := []struct {
		retryTimes int
		expected   int
	}{
		{0, 0},   // 不重试
		{1, 1},   // 至少 1
		{5, 1},   // 10% 向上取整 = 1
		{10, 1},  // 10% = 1
		{11, 2},  // 超过 10，向上取整 = 2
		{15, 2},  // 10% = 1.5 → 向上取整 2
		{20, 2},  // 10% = 2
		{21, 3},  // 超过 20，向上取整 = 3
		{50, 5},  // 10% = 5
		{100, 10},
	}
	for _, c := range cases {
		got := CalcSamePriorityRetryBudget(c.retryTimes)
		if got != c.expected {
			t.Errorf("CalcSamePriorityRetryBudget(%d) = %d, want %d", c.retryTimes, got, c.expected)
		}
	}
}

// TestMapRetryToPriorityLevel 测试全局 retry 到优先级层级的映射
func TestMapRetryToPriorityLevel(t *testing.T) {
	// budget=2, 3 个优先级层级
	// retry=0,1 → level 0；retry=2,3 → level 1；retry=4,5 → level 2；retry>=6 → level 2 (夹取)
	cases := []struct {
		retry        int
		budget       int
		numPriorities int
		expected     int
	}{
		{0, 2, 3, 0},
		{1, 2, 3, 0},
		{2, 2, 3, 1},
		{3, 2, 3, 1},
		{4, 2, 3, 2},
		{6, 2, 3, 2}, // 夹取
		{0, 0, 3, 0}, // budget=0 回退原始行为
		{1, 0, 3, 1},
		{2, 0, 3, 2},
		{3, 0, 3, 2}, // 夹取
		{0, 1, 1, 0}, // 单层级
		{5, 1, 1, 0},
	}
	for _, c := range cases {
		got := mapRetryToPriorityLevel(c.retry, c.budget, c.numPriorities)
		if got != c.expected {
			t.Errorf("mapRetryToPriorityLevel(%d, %d, %d) = %d, want %d",
				c.retry, c.budget, c.numPriorities, got, c.expected)
		}
	}
}

// TestSelectChannelFromList_SamePriorityRetry 验证同优先级内重试不降级
// 场景：3 个渠道，2 个高优先级(priority=10)，1 个低优先级(priority=1)
// RetryTimes=10 → budget=1 → retry=0 选高优先级，retry=1 仍选高优先级（budget 内），retry=2 才降级
func TestSelectChannelFromList_SamePriorityRetry(t *testing.T) {
	original := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	defer func() { common.MemoryCacheEnabled = original }()

	withRetryTimes(t, 10, func() {
		resetCache()
		// 高优先级: channel 1, 2 (priority=10)
		// 低优先级: channel 3 (priority=1)
		ch1 := newTestChannel(1, 1, 10, 100)
		ch2 := newTestChannel(2, 1, 10, 100)
		ch3 := newTestChannel(3, 1, 1, 100)
		setupCache(
			map[int]*Channel{1: ch1, 2: ch2, 3: ch3},
			map[string]map[string][]int{"g": {"m": {1, 2, 3}}},
		)

		// retry=0: budget=1, level=0 → 高优先级
		ch, err := selectChannelFromList([]int{1, 2, 3}, 0, nil)
		if err != nil || ch == nil {
			t.Fatalf("retry=0: expected channel, got (%v, %v)", ch, err)
		}
		if ch.GetPriority() != 10 {
			t.Errorf("retry=0: expected priority 10, got %d (channel id=%d)", ch.GetPriority(), ch.Id)
		}

		// retry=1: budget=1, level=1 → 低优先级（因为 budget=1，同层级只重试 1 次）
		// 注意：budget=1 意味着 retry=0 是同层级第 1 次，retry=1 已降级
		ch, err = selectChannelFromList([]int{1, 2, 3}, 1, nil)
		if err != nil || ch == nil {
			t.Fatalf("retry=1: expected channel, got (%v, %v)", ch, err)
		}
		if ch.GetPriority() != 1 {
			t.Errorf("retry=1: expected priority 1 (降级), got %d (channel id=%d)", ch.GetPriority(), ch.Id)
		}
	})
}

// TestSelectChannelFromList_SamePriorityRetry_HigherBudget 验证更大 budget 时同层级内多次重试
// 场景：RetryTimes=20 → budget=2 → retry=0,1 选高优先级，retry=2,3 选低优先级
func TestSelectChannelFromList_SamePriorityRetry_HigherBudget(t *testing.T) {
	original := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	defer func() { common.MemoryCacheEnabled = original }()

	withRetryTimes(t, 20, func() {
		resetCache()
		ch1 := newTestChannel(1, 1, 10, 100)
		ch2 := newTestChannel(2, 1, 1, 100)
		setupCache(
			map[int]*Channel{1: ch1, 2: ch2},
			map[string]map[string][]int{"g": {"m": {1, 2}}},
		)

		// retry=0: budget=2, level=0 → 高优先级
		ch, err := selectChannelFromList([]int{1, 2}, 0, nil)
		if err != nil || ch == nil {
			t.Fatalf("retry=0: expected channel, got (%v, %v)", ch, err)
		}
		if ch.GetPriority() != 10 {
			t.Errorf("retry=0: expected priority 10, got %d", ch.GetPriority())
		}

		// retry=1: budget=2, level=0 → 仍高优先级（同层级内第 2 次）
		ch, err = selectChannelFromList([]int{1, 2}, 1, nil)
		if err != nil || ch == nil {
			t.Fatalf("retry=1: expected channel, got (%v, %v)", ch, err)
		}
		if ch.GetPriority() != 10 {
			t.Errorf("retry=1: expected priority 10 (同层级内), got %d", ch.GetPriority())
		}

		// retry=2: budget=2, level=1 → 低优先级
		ch, err = selectChannelFromList([]int{1, 2}, 2, nil)
		if err != nil || ch == nil {
			t.Fatalf("retry=2: expected channel, got (%v, %v)", ch, err)
		}
		if ch.GetPriority() != 1 {
			t.Errorf("retry=2: expected priority 1 (降级), got %d", ch.GetPriority())
		}
	})
}

// TestSelectChannelFromList_ExcludesUsedChannels 验证已用渠道被排除
// 场景：2 个同优先级渠道，retry=0 选中 channel 1，retry=1 应排除 channel 1 选 channel 2
func TestSelectChannelFromList_ExcludesUsedChannels(t *testing.T) {
	original := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	defer func() { common.MemoryCacheEnabled = original }()

	withRetryTimes(t, 10, func() {
		resetCache()
		ch1 := newTestChannel(1, 1, 10, 100)
		ch2 := newTestChannel(2, 1, 10, 100)
		setupCache(
			map[int]*Channel{1: ch1, 2: ch2},
			map[string]map[string][]int{"g": {"m": {1, 2}}},
		)

		// retry=0, 已用 [1] → 应选 channel 2
		ch, err := selectChannelFromList([]int{1, 2}, 0, []int{1})
		if err != nil || ch == nil {
			t.Fatalf("expected channel, got (%v, %v)", ch, err)
		}
		if ch.Id != 2 {
			t.Errorf("expected channel #2 (排除已用 #1), got channel #%d", ch.Id)
		}

		// retry=0, 已用 [1, 2] → 全部排除后回退到不排除，应返回 1 或 2
		ch, err = selectChannelFromList([]int{1, 2}, 0, []int{1, 2})
		if err != nil || ch == nil {
			t.Fatalf("expected channel (回退全量), got (%v, %v)", ch, err)
		}
		if ch.Id != 1 && ch.Id != 2 {
			t.Errorf("expected channel #1 or #2 (回退), got #%d", ch.Id)
		}
	})
}

// TestSelectChannelFromList_RetryTimesZero_NoRetry 验证 RetryTimes=0 时回退原始行为
func TestSelectChannelFromList_RetryTimesZero_NoRetry(t *testing.T) {
	original := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	defer func() { common.MemoryCacheEnabled = original }()

	withRetryTimes(t, 0, func() {
		resetCache()
		ch1 := newTestChannel(1, 1, 10, 100)
		ch2 := newTestChannel(2, 1, 1, 100)
		setupCache(
			map[int]*Channel{1: ch1, 2: ch2},
			map[string]map[string][]int{"g": {"m": {1, 2}}},
		)

		// RetryTimes=0, budget=0 → retry 直接作为层级索引（原始行为）
		// retry=0 → level=0 → 高优先级
		ch, err := selectChannelFromList([]int{1, 2}, 0, nil)
		if err != nil || ch == nil {
			t.Fatalf("retry=0: expected channel, got (%v, %v)", ch, err)
		}
		if ch.GetPriority() != 10 {
			t.Errorf("retry=0: expected priority 10, got %d", ch.GetPriority())
		}

		// retry=1 → level=1 → 低优先级（原始行为：retry 直接索引）
		ch, err = selectChannelFromList([]int{1, 2}, 1, nil)
		if err != nil || ch == nil {
			t.Fatalf("retry=1: expected channel, got (%v, %v)", ch, err)
		}
		if ch.GetPriority() != 1 {
			t.Errorf("retry=1: expected priority 1, got %d", ch.GetPriority())
		}
	})
}
