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
