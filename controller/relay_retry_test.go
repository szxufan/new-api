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
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestIsFallbackEligibleError_429 验证 429 错误触发 fallback（disable429Ban=false 时维持现状）
func TestIsFallbackEligibleError_429(t *testing.T) {
	err := types.NewErrorWithStatusCode(errors.New("rate limited"), types.ErrorCodeBadResponseStatusCode, http.StatusTooManyRequests)
	require.True(t, isFallbackEligibleError(err, false))
}

// TestIsFallbackEligibleError_429_Disable429BanTrue 验证 disable429Ban=true 时 429 不触发 fallback（走正常重试）
func TestIsFallbackEligibleError_429_Disable429BanTrue(t *testing.T) {
	err := types.NewErrorWithStatusCode(errors.New("rate limited"), types.ErrorCodeBadResponseStatusCode, http.StatusTooManyRequests)
	require.False(t, isFallbackEligibleError(err, true))
}

// TestIsFallbackEligibleError_NilError 验证 nil 错误不触发 fallback
func TestIsFallbackEligibleError_NilError(t *testing.T) {
	require.False(t, isFallbackEligibleError(nil, false))
}

// TestIsFallbackEligibleError_500 验证普通 500 错误不触发 fallback
func TestIsFallbackEligibleError_500(t *testing.T) {
	err := types.NewErrorWithStatusCode(errors.New("server error"), types.ErrorCodeBadResponseStatusCode, http.StatusInternalServerError)
	require.False(t, isFallbackEligibleError(err, false))
}

// TestIsFallbackEligibleError_400 验证 400 错误不触发 fallback
func TestIsFallbackEligibleError_400(t *testing.T) {
	err := types.NewErrorWithStatusCode(errors.New("bad request"), types.ErrorCodeBadResponseStatusCode, http.StatusBadRequest)
	require.False(t, isFallbackEligibleError(err, false))
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

// TestMakeChannelError 验证 makeChannelError 正确构造 ChannelError（含 Disable429Ban 透传）
func TestMakeChannelError(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "test-key")

	autoBan := 1
	channel := &model.Channel{
		Id:      42,
		Type:    1,
		Name:    "test-channel",
		AutoBan: &autoBan,
		ChannelInfo: model.ChannelInfo{
			Disable429Ban: true,
		},
	}

	ce := makeChannelError(channel, c)
	require.Equal(t, 42, ce.ChannelId)
	require.Equal(t, 1, ce.ChannelType)
	require.Equal(t, "test-channel", ce.ChannelName)
	require.Equal(t, "test-key", ce.UsingKey)
	require.True(t, ce.AutoBan)
	require.False(t, ce.IsMultiKey)
	require.True(t, ce.Disable429Ban) // 验证 Disable429Ban 透传
}

// TestMakeChannelError_Disable429BanDefaultFalse 验证未设置时 Disable429Ban 默认为 false
func TestMakeChannelError_Disable429BanDefaultFalse(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "test-key")

	autoBan := 1
	channel := &model.Channel{
		Id:      43,
		Type:    1,
		Name:    "test-channel-no-429-ban",
		AutoBan: &autoBan,
	}

	ce := makeChannelError(channel, c)
	require.False(t, ce.Disable429Ban) // 默认 false = 维持现状（限流+fallback）
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

// TestShouldRetryChannel_429_NotInRetryRanges 验证全局重试状态码不含 429 时 429 不重试（覆盖行为矩阵情况 ③④）
// 此用例临时移除 429 所在的 409-499 重试范围，测试后还原
func TestShouldRetryChannel_429_NotInRetryRanges(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	err := types.NewErrorWithStatusCode(errors.New("rate limited"), types.ErrorCodeBadResponseStatusCode, http.StatusTooManyRequests)

	// 备份原范围并临时替换为不含 429 的范围
	originalRanges := operation_setting.AutomaticRetryStatusCodeRanges
	operation_setting.AutomaticRetryStatusCodeRanges = []operation_setting.StatusCodeRange{
		{Start: 100, End: 199},
		{Start: 300, End: 399},
		{Start: 401, End: 407}, // 409-499 被移除，429 不在重试范围
		{Start: 500, End: 503},
	}
	defer func() {
		operation_setting.AutomaticRetryStatusCodeRanges = originalRanges
	}()

	require.False(t, shouldRetryChannel(c, err)) // 429 不在重试范围 → 不重试
}
