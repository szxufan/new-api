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

package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

// TestResolveChannelAttempts_Global 验证 retry_times=0 时使用全局计算值
func TestResolveChannelAttempts_Global(t *testing.T) {
	attempts, forbidden := resolveChannelAttempts(0, 3)
	require.False(t, forbidden)
	require.Equal(t, 3, attempts)
}

// TestResolveChannelAttempts_Forbidden 验证 retry_times=-1 时禁止重试（仅尝试一次）
func TestResolveChannelAttempts_Forbidden(t *testing.T) {
	attempts, forbidden := resolveChannelAttempts(-1, 3)
	require.True(t, forbidden)
	require.Equal(t, 1, attempts)
}

// TestResolveChannelAttempts_Override 验证 retry_times=N>0 时覆盖为 N+1 次尝试
func TestResolveChannelAttempts_Override(t *testing.T) {
	attempts, forbidden := resolveChannelAttempts(2, 3)
	require.False(t, forbidden)
	require.Equal(t, 3, attempts)

	attempts, forbidden = resolveChannelAttempts(5, 1)
	require.False(t, forbidden)
	require.Equal(t, 6, attempts)
}

// TestChannel_GetRetryTimes 验证渠道 getter：nil 视为 0
func TestChannel_GetRetryTimes(t *testing.T) {
	ch := &model.Channel{}
	require.Equal(t, 0, ch.GetRetryTimes())

	ch.RetryTimes = common.GetPointer(-1)
	require.Equal(t, -1, ch.GetRetryTimes())

	ch.RetryTimes = common.GetPointer(3)
	require.Equal(t, 3, ch.GetRetryTimes())
}

// TestComputeExtraRetryBudget_NoOverride 验证无覆盖渠道时预算补偿为 0（行为不变的关键保证）
func TestComputeExtraRetryBudget_NoOverride(t *testing.T) {
	channels := []*model.Channel{
		{Id: 1},                                   // nil → 0 全局
		{Id: 2, RetryTimes: common.GetPointer(0)}, // 显式 0 全局
		{Id: 3, RetryTimes: common.GetPointer(-1)}, // 禁重试不贡献预算
	}
	require.Equal(t, 0, computeExtraRetryBudget(channels, 1))
}

// TestComputeExtraRetryBudget_SingleOverride 验证单个覆盖渠道贡献额外预算
func TestComputeExtraRetryBudget_SingleOverride(t *testing.T) {
	channels := []*model.Channel{
		{Id: 1, RetryTimes: common.GetPointer(2)}, // N=2 → 3 次尝试，default=1 时额外 +2
	}
	require.Equal(t, 2, computeExtraRetryBudget(channels, 1))
}

// TestComputeExtraRetryBudget_Mixed 验证混合配置各渠道贡献正确
func TestComputeExtraRetryBudget_Mixed(t *testing.T) {
	channels := []*model.Channel{
		{Id: 1, RetryTimes: common.GetPointer(-1)}, // 禁重试 → 0
		{Id: 2, RetryTimes: common.GetPointer(0)},  // 全局 default=2 → 0
		{Id: 3, RetryTimes: common.GetPointer(4)},  // N=4 → 5 次尝试，额外 +3
		{Id: 4, RetryTimes: common.GetPointer(1)},  // N=1 → 2 次尝试，不超过 default=2 → 0
	}
	require.Equal(t, 3, computeExtraRetryBudget(channels, 2))
}

// TestComputeExtraRetryBudget_NilChannel 验证 nil 渠道不 panic 且不贡献预算
func TestComputeExtraRetryBudget_NilChannel(t *testing.T) {
	channels := []*model.Channel{nil, {Id: 1, RetryTimes: common.GetPointer(1)}}
	require.Equal(t, 1, computeExtraRetryBudget(channels, 1))
}
