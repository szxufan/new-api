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
