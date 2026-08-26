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

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

// TestSortChannelsByAffinityCount 测试按亲和性计数升/降序稳定排序
func TestSortChannelsByAffinityCount(t *testing.T) {
	channels := []*model.Channel{
		{Id: 1, AffinityCount: 2},
		{Id: 2, AffinityCount: 5},
		{Id: 3, AffinityCount: 0},
		{Id: 4, AffinityCount: 5}, // 与 Id 2 相同计数，验证稳定性
	}

	// 降序
	sortChannelsByAffinityCount(channels, true)
	ids := []int{channels[0].Id, channels[1].Id, channels[2].Id, channels[3].Id}
	require.Equal(t, []int{2, 4, 1, 3}, ids)

	// 升序
	sortChannelsByAffinityCount(channels, false)
	ids = []int{channels[0].Id, channels[1].Id, channels[2].Id, channels[3].Id}
	require.Equal(t, []int{3, 1, 2, 4}, ids)

	// 空列表 / 单元素不应 panic
	sortChannelsByAffinityCount(nil, true)
	sortChannelsByAffinityCount([]*model.Channel{{Id: 1}}, false)
}

// TestApplyTagAffinityOrder 测试按 Tag 组亲和性合计重排
func TestApplyTagAffinityOrder(t *testing.T) {
	// 收集顺序：tagA（2+3=5）、tagB（0+1=1）、tagC（9）
	channels := []*model.Channel{
		{Id: 1, AffinityCount: 2}, // tagA
		{Id: 2, AffinityCount: 3}, // tagA
		{Id: 3, AffinityCount: 0}, // tagB
		{Id: 4, AffinityCount: 1}, // tagB
		{Id: 5, AffinityCount: 9}, // tagC
	}
	ranges := []tagChannelRange{
		{tag: "a", start: 0, end: 2},
		{tag: "b", start: 2, end: 4},
		{tag: "c", start: 4, end: 5},
	}

	// 降序：tagC(9)、tagA(5)、tagB(1)
	applyTagAffinityOrder(channels, ranges, true)
	ids := []int{channels[0].Id, channels[1].Id, channels[2].Id, channels[3].Id, channels[4].Id}
	require.Equal(t, []int{5, 1, 2, 3, 4}, ids)

	// 重新收集后升序：tagB(1)、tagA(5)、tagC(9)
	channels = []*model.Channel{
		{Id: 1, AffinityCount: 2}, // tagA
		{Id: 2, AffinityCount: 3}, // tagA
		{Id: 3, AffinityCount: 0}, // tagB
		{Id: 4, AffinityCount: 1}, // tagB
		{Id: 5, AffinityCount: 9}, // tagC
	}
	ranges = []tagChannelRange{
		{tag: "a", start: 0, end: 2},
		{tag: "b", start: 2, end: 4},
		{tag: "c", start: 4, end: 5},
	}
	applyTagAffinityOrder(channels, ranges, false)
	ids = []int{channels[0].Id, channels[1].Id, channels[2].Id, channels[3].Id, channels[4].Id}
	require.Equal(t, []int{3, 4, 1, 2, 5}, ids)

	// 合计相等时保持稳定：tagA(5)、tagC(5)、tagB(1)
	channels = []*model.Channel{
		{Id: 1, AffinityCount: 5}, // tagA
		{Id: 2, AffinityCount: 1}, // tagB
		{Id: 3, AffinityCount: 5}, // tagC
	}
	ranges = []tagChannelRange{
		{tag: "a", start: 0, end: 1},
		{tag: "b", start: 1, end: 2},
		{tag: "c", start: 2, end: 3},
	}
	applyTagAffinityOrder(channels, ranges, true)
	ids = []int{channels[0].Id, channels[1].Id, channels[2].Id}
	require.Equal(t, []int{1, 3, 2}, ids)

	// 少于 2 组时不重排
	single := []*model.Channel{{Id: 1, AffinityCount: 5}}
	applyTagAffinityOrder(single, []tagChannelRange{{tag: "a", start: 0, end: 1}}, true)
	require.Equal(t, 1, single[0].Id)
}

// TestNewChannelSortOptionsAffinityCount 测试排序选项识别 affinity_count
func TestNewChannelSortOptionsAffinityCount(t *testing.T) {
	opts := model.NewChannelSortOptions("affinity_count", "asc", false)
	require.True(t, opts.SortByAffinityCount)
	require.Equal(t, "asc", opts.SortOrder)

	opts = model.NewChannelSortOptions("affinity_count", "desc", false)
	require.True(t, opts.SortByAffinityCount)
	require.Equal(t, "desc", opts.SortOrder)

	// 无效方向默认 desc
	opts = model.NewChannelSortOptions("affinity_count", "invalid", false)
	require.True(t, opts.SortByAffinityCount)
	require.Equal(t, "desc", opts.SortOrder)

	// 普通列不受影响
	opts = model.NewChannelSortOptions("balance", "asc", false)
	require.False(t, opts.SortByAffinityCount)
	require.Equal(t, "balance", opts.SortBy)

	// 未知列回退为空
	opts = model.NewChannelSortOptions("unknown_col", "asc", false)
	require.False(t, opts.SortByAffinityCount)
	require.Equal(t, "", opts.SortBy)
}