package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
)

// createChannelWithAbility 在测试 DB 中创建一个渠道及其 ability 记录。
// priority/weight 写入 ability 表（DB 路径以 ability 为准）。
func createChannelWithAbility(t *testing.T, id, chType int, group, model string, priority int64, weight uint) {
	t.Helper()
	ch := &Channel{
		Id:     id,
		Type:   chType,
		Status: common.ChannelStatusEnabled,
		Group:  group,
		Models: model,
	}
	if err := DB.Create(ch).Error; err != nil {
		t.Fatalf("failed to create channel #%d: %v", id, err)
	}
	ability := &Ability{
		Group:     group,
		Model:     model,
		ChannelId: id,
		Enabled:   true,
		Priority:  &priority,
		Weight:    weight,
	}
	if err := DB.Create(ability).Error; err != nil {
		t.Fatalf("failed to create ability for channel #%d: %v", id, err)
	}
}

func cleanupChannelsAndAbilities(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		DB.Exec("DELETE FROM channels")
		DB.Exec("DELETE FROM abilities")
	})
}

// TestGetSortedChannelsDB_PriorityDesc 验证 DB 路径按优先级降序返回渠道。
func TestGetSortedChannelsDB_PriorityDesc(t *testing.T) {
	cleanupChannelsAndAbilities(t)
	createChannelWithAbility(t, 1, 1, "g", "m", 5, 100)
	createChannelWithAbility(t, 2, 1, "g", "m", 10, 100)
	createChannelWithAbility(t, 3, 1, "g", "m", 1, 100)

	channels := GetSortedChannelsDB("g", "m", nil)
	if len(channels) != 3 {
		t.Fatalf("expected 3 channels, got %d", len(channels))
	}
	// 优先级降序：10 > 5 > 1
	if channels[0].Id != 2 || channels[1].Id != 1 || channels[2].Id != 3 {
		t.Errorf("expected order [2,1,3] by priority desc, got [%d,%d,%d]",
			channels[0].Id, channels[1].Id, channels[2].Id)
	}
}

// TestGetSortedChannelsDB_AffinityAscending 验证同优先级内按亲和性计数/权重升序排序。
func TestGetSortedChannelsDB_AffinityAscending(t *testing.T) {
	cleanupChannelsAndAbilities(t)
	// 同优先级、同权重，亲和性计数不同
	createChannelWithAbility(t, 1, 1, "g", "m", 10, 100)
	createChannelWithAbility(t, 2, 1, "g", "m", 10, 100)
	createChannelWithAbility(t, 3, 1, "g", "m", 10, 100)

	affinityCounts := map[int]int64{1: 10, 2: 0, 3: 5}
	channels := GetSortedChannelsDB("g", "m", affinityCounts)
	if len(channels) != 3 {
		t.Fatalf("expected 3 channels, got %d", len(channels))
	}
	// score = count/weight，权重均为 100：ch1=0.1, ch2=0, ch3=0.05
	// 升序：ch2(0) < ch3(0.05) < ch1(0.1)
	if channels[0].Id != 2 || channels[1].Id != 3 || channels[2].Id != 1 {
		t.Errorf("expected order [2,3,1] by affinity score asc, got [%d,%d,%d]",
			channels[0].Id, channels[1].Id, channels[2].Id)
	}
}

// TestGetSortedChannelsDB_Empty 验证无渠道时返回 nil。
func TestGetSortedChannelsDB_Empty(t *testing.T) {
	cleanupChannelsAndAbilities(t)
	channels := GetSortedChannelsDB("nonexistent", "m", nil)
	if channels != nil {
		t.Errorf("expected nil for no channels, got %v", channels)
	}
}

// TestGetSortedChannelsByPriorityAndAffinity_MemoryCache 验证内存缓存路径排序。
func TestGetSortedChannelsByPriorityAndAffinity_MemoryCache(t *testing.T) {
	original := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	defer func() { common.MemoryCacheEnabled = original }()

	resetCache()
	ch1 := newTestChannel(1, 1, 5, 100)
	ch2 := newTestChannel(2, 1, 10, 100)
	ch3 := newTestChannel(3, 1, 10, 100)
	setupCache(
		map[int]*Channel{1: ch1, 2: ch2, 3: ch3},
		map[string]map[string][]int{"g": {"m": {1, 2, 3}}},
	)

	// 无亲和性：优先级降序，同优先级保持原顺序
	channels := GetSortedChannelsByPriorityAndAffinity("g", "m", nil, nil)
	if len(channels) != 3 {
		t.Fatalf("expected 3 channels, got %d", len(channels))
	}
	// ch2、ch3 优先级 10 在前，ch1 优先级 5 在后
	if channels[0].Id != 2 || channels[1].Id != 3 || channels[2].Id != 1 {
		t.Errorf("expected [2,3,1] by priority desc, got [%d,%d,%d]",
			channels[0].Id, channels[1].Id, channels[2].Id)
	}

	// 带亲和性：同优先级 10 内 ch2(count=10) > ch3(count=0)，升序后 ch3 在前
	affinityCounts := map[int]int64{1: 0, 2: 10, 3: 0}
	channels = GetSortedChannelsByPriorityAndAffinity("g", "m", nil, affinityCounts)
	if channels[0].Id != 3 || channels[1].Id != 2 {
		t.Errorf("expected ch3 before ch2 by affinity asc, got [%d,%d]",
			channels[0].Id, channels[1].Id)
	}
}

// TestGetSortedChannelsByPriorityAndAffinity_PreferredTypes 验证偏好类型过滤语义：
// 当存在匹配偏好类型的渠道时，仅返回偏好类型渠道（已排序）。
func TestGetSortedChannelsByPriorityAndAffinity_PreferredTypes(t *testing.T) {
	original := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	defer func() { common.MemoryCacheEnabled = original }()

	resetCache()
	// ch1 类型 1（偏好），优先级 5；ch2 类型 1（偏好），优先级 10；ch3 类型 2（非偏好），优先级 10
	ch1 := newTestChannel(1, 1, 5, 100)
	ch2 := newTestChannel(2, 1, 10, 100)
	ch3 := newTestChannel(3, 2, 10, 100)
	setupCache(
		map[int]*Channel{1: ch1, 2: ch2, 3: ch3},
		map[string]map[string][]int{"g": {"m": {1, 2, 3}}},
	)

	channels := GetSortedChannelsByPriorityAndAffinity("g", "m", []int{1}, nil)
	// 偏好类型为 1，仅 ch1/ch2 匹配，ch3 被过滤掉
	if len(channels) != 2 {
		t.Fatalf("expected 2 preferred channels, got %d", len(channels))
	}
	// 排序后按优先级降序：ch2(10) 在 ch1(5) 之前
	if channels[0].Id != 2 || channels[1].Id != 1 {
		t.Errorf("expected [2,1] by priority desc within preferred types, got [%d,%d]",
			channels[0].Id, channels[1].Id)
	}
}

// TestGetSortedChannelsByPriorityAndAffinity_PreferredTypesFallback 验证偏好类型无匹配时回退到全量。
func TestGetSortedChannelsByPriorityAndAffinity_PreferredTypesFallback(t *testing.T) {
	original := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	defer func() { common.MemoryCacheEnabled = original }()

	resetCache()
	// 所有渠道类型均为 1，偏好类型 99 无匹配
	ch1 := newTestChannel(1, 1, 5, 100)
	ch2 := newTestChannel(2, 1, 10, 100)
	setupCache(
		map[int]*Channel{1: ch1, 2: ch2},
		map[string]map[string][]int{"g": {"m": {1, 2}}},
	)

	channels := GetSortedChannelsByPriorityAndAffinity("g", "m", []int{99}, nil)
	// 偏好类型无匹配，回退到全量渠道
	if len(channels) != 2 {
		t.Fatalf("expected 2 channels after fallback, got %d", len(channels))
	}
	// 排序后按优先级降序：ch2(10) 在 ch1(5) 之前
	if channels[0].Id != 2 || channels[1].Id != 1 {
		t.Errorf("expected [2,1] after fallback, got [%d,%d]",
			channels[0].Id, channels[1].Id)
	}
}
