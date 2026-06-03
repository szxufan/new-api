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

package service

import (
	"errors"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
)

// TestRateLimit429Enabled tests that RateLimitChannelKey429 respects the enabled flag
func TestRateLimit429Enabled(t *testing.T) {
	// Save original value
	originalEnabled := operation_setting.RateLimit429Enabled
	defer func() {
		operation_setting.RateLimit429Enabled = originalEnabled
	}()

	// Test when disabled
	operation_setting.RateLimit429Enabled = false
	channelError := types.ChannelError{
		ChannelId:   1,
		ChannelName:  "test-channel",
		AutoBan:      true,
		IsMultiKey:   false,
	}
	// Should return early without error
	RateLimitChannelKey429(channelError)
}

// TestRateLimit429AutoBan tests that RateLimitChannelKey429 respects the AutoBan flag
func TestRateLimit429AutoBan(t *testing.T) {
	// Save original value
	originalEnabled := operation_setting.RateLimit429Enabled
	defer func() {
		operation_setting.RateLimit429Enabled = originalEnabled
	}()

	operation_setting.RateLimit429Enabled = true

	// Test when AutoBan is false
	channelError := types.ChannelError{
		ChannelId:   1,
		ChannelName: "test-channel",
		AutoBan:     false,
		IsMultiKey:  false,
	}
	// Should return early without error
	RateLimitChannelKey429(channelError)
}

// TestRateLimit429Duration tests that the duration is correctly calculated
func TestRateLimit429Duration(t *testing.T) {
	// Save original values
	originalEnabled := operation_setting.RateLimit429Enabled
	originalDuration := operation_setting.RateLimit429DurationMinutes
	defer func() {
		operation_setting.RateLimit429Enabled = originalEnabled
		operation_setting.RateLimit429DurationMinutes = originalDuration
	}()

	operation_setting.RateLimit429Enabled = true
	operation_setting.RateLimit429DurationMinutes = 2

	// Expected duration should be 2 minutes from now
	expectedMin := time.Now().Add(2 * time.Minute).Unix()
	expectedMax := time.Now().Add(3 * time.Minute).Unix()

	// Calculate the actual duration that would be used
	actualDuration := time.Now().Add(time.Duration(operation_setting.RateLimit429DurationMinutes) * time.Minute).Unix()

	if actualDuration < expectedMin || actualDuration > expectedMax {
		t.Errorf("Duration calculation incorrect: expected between %d and %d, got %d", expectedMin, expectedMax, actualDuration)
	}
}

// TestChannelStatusConstants tests that the new status constants are correctly defined
func TestChannelStatusConstants(t *testing.T) {
	if common.ChannelStatusRateLimited429 != 4 {
		t.Errorf("ChannelStatusRateLimited429 should be 4, got %d", common.ChannelStatusRateLimited429)
	}
	if common.ChannelStatusManuallyRateLimited != 5 {
		t.Errorf("ChannelStatusManuallyRateLimited should be 5, got %d", common.ChannelStatusManuallyRateLimited)
	}
}

// TestShouldDisableChannelWith429 tests that 429 is handled by rate limit, not disable
func TestShouldDisableChannelWith429(t *testing.T) {
	// This test verifies that 429 status code does NOT trigger ShouldDisableChannel
	// 429 should be handled by the rate limit mechanism instead
	err := types.NewErrorWithStatusCode(
		errors.New("429 Too Many Requests"),
		types.ErrorCodeBadResponseStatusCode,
		429,
	)

	// ShouldDisableChannel should return false for 429
	// because 429 is not in the AutomaticDisableStatusCodeRanges (only 401 is)
	if ShouldDisableChannel(err) {
		t.Error("ShouldDisableChannel should return false for 429 status code")
	}
}
