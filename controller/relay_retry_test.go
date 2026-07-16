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
	"errors"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestIsFallbackEligibleError_429 验证 429 错误触发 fallback
func TestIsFallbackEligibleError_429(t *testing.T) {
	err := types.NewErrorWithStatusCode(errors.New("rate limited"), types.ErrorCodeBadResponseStatusCode, http.StatusTooManyRequests)
	require.True(t, isFallbackEligibleError(err))
}

// TestIsFallbackEligibleError_NilError 验证 nil 错误不触发 fallback
func TestIsFallbackEligibleError_NilError(t *testing.T) {
	require.False(t, isFallbackEligibleError(nil))
}

// TestIsFallbackEligibleError_500 验证普通 500 错误不触发 fallback
func TestIsFallbackEligibleError_500(t *testing.T) {
	err := types.NewErrorWithStatusCode(errors.New("server error"), types.ErrorCodeBadResponseStatusCode, http.StatusInternalServerError)
	require.False(t, isFallbackEligibleError(err))
}

// TestIsFallbackEligibleError_400 验证 400 错误不触发 fallback
func TestIsFallbackEligibleError_400(t *testing.T) {
	err := types.NewErrorWithStatusCode(errors.New("bad request"), types.ErrorCodeBadResponseStatusCode, http.StatusBadRequest)
	require.False(t, isFallbackEligibleError(err))
}

// TestShouldRetryChannel_NilError 验证 nil 错误不重试
func TestShouldRetryChannel_NilError(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	require.False(t, shouldRetryChannel(c, nil))
}

// TestShouldRetryChannel_429 验证 429 应该重试
func TestShouldRetryChannel_429(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	err := types.NewErrorWithStatusCode(errors.New("rate limited"), types.ErrorCodeBadResponseStatusCode, http.StatusTooManyRequests)
	// 429 默认应该重试（除非被配置为跳过）
	require.True(t, shouldRetryChannel(c, err))
}

// TestShouldRetryChannel_400 验证 400 不应该重试
func TestShouldRetryChannel_400(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	err := types.NewErrorWithStatusCode(errors.New("bad request"), types.ErrorCodeBadResponseStatusCode, http.StatusBadRequest)
	require.False(t, shouldRetryChannel(c, err))
}

// TestShouldRetryChannel_SpecificChannelId 验证指定渠道 ID 时不重试
func TestShouldRetryChannel_SpecificChannelId(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	c.Set("specific_channel_id", 123)
	err := types.NewErrorWithStatusCode(errors.New("server error"), types.ErrorCodeBadResponseStatusCode, http.StatusInternalServerError)
	require.False(t, shouldRetryChannel(c, err))
}

// TestShouldRetryChannel_2xx 验证 2xx 不重试
func TestShouldRetryChannel_2xx(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	err := types.NewErrorWithStatusCode(errors.New("ok"), types.ErrorCodeBadResponseStatusCode, http.StatusOK)
	require.False(t, shouldRetryChannel(c, err))
}

// TestMakeChannelError 验证 makeChannelError 正确构造 ChannelError
func TestMakeChannelError(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "test-key")

	autoBan := 1
	channel := &model.Channel{
		Id:      42,
		Type:    1,
		Name:    "test-channel",
		AutoBan: &autoBan,
	}

	ce := makeChannelError(channel, c)
	require.Equal(t, 42, ce.ChannelId)
	require.Equal(t, 1, ce.ChannelType)
	require.Equal(t, "test-channel", ce.ChannelName)
	require.Equal(t, "test-key", ce.UsingKey)
	require.True(t, ce.AutoBan)
	require.False(t, ce.IsMultiKey)
}

// TestAddUsedChannel_Normal 验证普通渠道记录
func TestAddUsedChannel_Normal(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	addUsedChannel(c, 10)
	addUsedChannel(c, 20)
	useChannel := c.GetStringSlice("use_channel")
	require.Equal(t, []string{"10", "20"}, useChannel)
}

// TestAddUsedChannel_Fallback 验证 fallback 渠道记录格式
func TestAddUsedChannel_Fallback(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	common.SetContextKey(c, constant.ContextKeyFallbackFromChannelId, 5)
	addUsedChannel(c, 10)
	useChannel := c.GetStringSlice("use_channel")
	require.Equal(t, []string{"10(fallback_from_5)"}, useChannel)
}

// TestAddUsedChannel_RetryCount 验证重试计数更新
func TestAddUsedChannel_RetryCount(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	addUsedChannel(c, 10) // 第一次，retryCount=0
	addUsedChannel(c, 20) // 第二次，retryCount=1
	retryCount := common.GetContextKeyInt(c, constant.ContextKeyRetryCount)
	require.Equal(t, 1, retryCount)
}
