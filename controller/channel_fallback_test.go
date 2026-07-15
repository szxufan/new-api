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

	"github.com/stretchr/testify/require"
)

func TestFilterOutUsedIDs_EmptyUsed(t *testing.T) {
	ids := []int{1, 2, 3}
	result := filterOutUsedIDs(ids, nil)
	require.Equal(t, []int{1, 2, 3}, result)
}

func TestFilterOutUsedIDs_NoOverlap(t *testing.T) {
	ids := []int{1, 2, 3}
	used := []int{4, 5}
	result := filterOutUsedIDs(ids, used)
	require.Equal(t, []int{1, 2, 3}, result)
}

func TestFilterOutUsedIDs_PartialOverlap(t *testing.T) {
	ids := []int{1, 2, 3, 4}
	used := []int{2, 4}
	result := filterOutUsedIDs(ids, used)
	require.Equal(t, []int{1, 3}, result)
}

func TestFilterOutUsedIDs_FullOverlap(t *testing.T) {
	ids := []int{1, 2}
	used := []int{1, 2}
	result := filterOutUsedIDs(ids, used)
	require.Equal(t, []int{}, result)
}

func TestFilterOutUsedIDs_EmptyIDs(t *testing.T) {
	result := filterOutUsedIDs([]int{}, []int{1, 2})
	require.Equal(t, []int{}, result)
}

func TestValidateFallbackChannelIds_SelfReference(t *testing.T) {
	err := validateFallbackChannelIds(1, []int{1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "不能将自己设为后备渠道")
}

func TestValidateFallbackChannelIds_EmptyList(t *testing.T) {
	err := validateFallbackChannelIds(1, []int{})
	require.NoError(t, err)
}

func TestValidateFallbackChannelIds_NilList(t *testing.T) {
	err := validateFallbackChannelIds(1, nil)
	require.NoError(t, err)
}
