/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
)

func TestGetFallbackChannelIDs_Empty(t *testing.T) {
	channel := &Channel{OtherInfo: ""}
	ids := channel.GetFallbackChannelIDs()
	if len(ids) != 0 {
		t.Errorf("expected empty, got %v", ids)
	}
}

func TestGetFallbackChannelIDs_NoFallbackKey(t *testing.T) {
	channel := &Channel{OtherInfo: `{"status_reason":"test"}`}
	ids := channel.GetFallbackChannelIDs()
	if len(ids) != 0 {
		t.Errorf("expected empty, got %v", ids)
	}
}

func TestGetFallbackChannelIDs_WithIDs(t *testing.T) {
	channel := &Channel{OtherInfo: `{"fallback_channel_ids":[1,2,3]}`}
	ids := channel.GetFallbackChannelIDs()
	if len(ids) != 3 {
		t.Fatalf("expected 3 IDs, got %d", len(ids))
	}
	expected := []int{1, 2, 3}
	for i, id := range ids {
		if id != expected[i] {
			t.Errorf("index %d: expected %d, got %d", i, expected[i], id)
		}
	}
}

func TestGetFallbackChannelIDs_InvalidType(t *testing.T) {
	channel := &Channel{OtherInfo: `{"fallback_channel_ids":"not-an-array"}`}
	ids := channel.GetFallbackChannelIDs()
	if len(ids) != 0 {
		t.Errorf("expected empty for non-array value, got %v", ids)
	}
}

func TestSetFallbackChannelIDs_SetIDs(t *testing.T) {
	channel := &Channel{OtherInfo: "{}"}
	channel.SetFallbackChannelIDs([]int{10, 20})
	ids := channel.GetFallbackChannelIDs()
	if len(ids) != 2 || ids[0] != 10 || ids[1] != 20 {
		t.Errorf("expected [10, 20], got %v", ids)
	}
}

func TestSetFallbackChannelIDs_ClearIDs(t *testing.T) {
	channel := &Channel{OtherInfo: `{"fallback_channel_ids":[1,2]}`}
	channel.SetFallbackChannelIDs([]int{})
	ids := channel.GetFallbackChannelIDs()
	if len(ids) != 0 {
		t.Errorf("expected empty after clearing, got %v", ids)
	}
}

func TestSetFallbackChannelIDs_PreservesOtherInfo(t *testing.T) {
	channel := &Channel{OtherInfo: `{"status_reason":"test","rate_limit_until":123}`}
	channel.SetFallbackChannelIDs([]int{5})
	ids := channel.GetFallbackChannelIDs()
	if len(ids) != 1 || ids[0] != 5 {
		t.Errorf("expected [5], got %v", ids)
	}
	// Verify other fields are preserved
	info := channel.GetOtherInfo()
	if info["status_reason"] != "test" {
		t.Errorf("expected status_reason preserved, got %v", info["status_reason"])
	}
}

func TestCacheGetFallbackChannel_EmptyList(t *testing.T) {
	ch, err := CacheGetFallbackChannel(nil, "gpt-4")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if ch != nil {
		t.Errorf("expected nil channel for empty list")
	}

	ch, err = CacheGetFallbackChannel([]int{}, "gpt-4")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if ch != nil {
		t.Errorf("expected nil channel for empty list")
	}
}

func TestCacheGetFallbackChannel_NonExistentChannel(t *testing.T) {
	// Initialize cache if needed for testing
	if !common.MemoryCacheEnabled {
		t.Skip("memory cache not enabled")
	}
	ch, err := CacheGetFallbackChannel([]int{99999}, "gpt-4")
	if err != nil {
		// Expected: channel doesn't exist
		return
	}
	if ch != nil {
		t.Errorf("expected nil for non-existent channel, got %v", ch)
	}
}

func TestIsChannelInGroup(t *testing.T) {
	tests := []struct {
		name     string
		group    string
		channel  *Channel
		expected bool
	}{
		{
			name:     "single group match",
			group:    "default",
			channel:  &Channel{Group: "default"},
			expected: true,
		},
		{
			name:     "single group no match",
			group:    "vip",
			channel:  &Channel{Group: "default"},
			expected: false,
		},
		{
			name:     "multi group contains target",
			group:    "vip",
			channel:  &Channel{Group: "default,vip"},
			expected: true,
		},
		{
			name:     "multi group not contains target",
			group:    "premium",
			channel:  &Channel{Group: "default,vip"},
			expected: false,
		},
		{
			name:     "empty group",
			group:    "default",
			channel:  &Channel{Group: ""},
			expected: false,
		},
		{
			name:     "group with spaces",
			group:    "vip",
			channel:  &Channel{Group: "default, vip"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isChannelInGroup(tt.channel, tt.group)
			if result != tt.expected {
				t.Errorf("isChannelInGroup(%q, %q) = %v, want %v", tt.channel.Group, tt.group, result, tt.expected)
			}
		})
	}
}

func TestCacheGetDisabledChannelsWithFallback_GroupFilter(t *testing.T) {
	original := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	defer func() { common.MemoryCacheEnabled = original }()

	// 设置缓存：渠道1属于 default 分组（自动禁用），渠道2属于 vip 分组（启用）
	// 渠道1 配置了后备渠道ID为2
	ch1 := &Channel{
		Id:        1,
		Group:     "default",
		Status:    common.ChannelStatusAutoDisabled,
		Models:    "gpt-4",
		OtherInfo: `{"fallback_channel_ids":[2]}`,
	}
	ch2 := &Channel{
		Id:     2,
		Group:  "vip",
		Status: common.ChannelStatusEnabled,
		Models: "gpt-4",
	}

	resetCache()
	setupCache(
		map[int]*Channel{1: ch1, 2: ch2},
		map[string]map[string][]int{
			"vip": {"gpt-4": {2}},
		},
	)

	// 查找 default 分组的后备渠道：渠道1属于 default，其后备渠道2可用
	fallbackCh, originalId, err := CacheGetDisabledChannelsWithFallback("default", "gpt-4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fallbackCh == nil {
		t.Error("expected fallback channel to be found for default group, got nil")
	}
	if originalId != 1 {
		t.Errorf("expected originalId=1, got %d", originalId)
	}

	// 查找 premium 分组的后备渠道：没有渠道属于 premium，应返回 nil
	fallbackCh, _, err = CacheGetDisabledChannelsWithFallback("premium", "gpt-4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fallbackCh != nil {
		t.Error("expected nil for non-matching group, got a channel")
	}
}

func TestCacheGetDisabledChannelsWithFallback_EnabledChannel(t *testing.T) {
	original := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	defer func() { common.MemoryCacheEnabled = original }()

	// 正常启用的渠道配置了后备渠道，也应该能被找到
	ch1 := &Channel{
		Id:        1,
		Group:     "default",
		Status:    common.ChannelStatusEnabled,
		Models:    "gpt-4",
		OtherInfo: `{"fallback_channel_ids":[2]}`,
	}
	ch2 := &Channel{
		Id:     2,
		Group:  "vip",
		Status: common.ChannelStatusEnabled,
		Models: "gpt-4",
	}

	resetCache()
	setupCache(
		map[int]*Channel{1: ch1, 2: ch2},
		map[string]map[string][]int{
			"default": {"gpt-4": {1}},
			"vip":     {"gpt-4": {2}},
		},
	)

	fallbackCh, originalId, err := CacheGetDisabledChannelsWithFallback("default", "gpt-4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fallbackCh == nil {
		t.Error("expected fallback channel from enabled channel, got nil")
	}
	if originalId != 1 {
		t.Errorf("expected originalId=1, got %d", originalId)
	}
}
