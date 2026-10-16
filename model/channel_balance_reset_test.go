package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/require"
)

// createResetChannel 创建一个配置了周期重置余额的测试渠道
func createResetChannel(t *testing.T, status int, amount *float64, periodDays *int, nextTime *int64) *Channel {
	t.Helper()
	autoBan := 1
	ch := &Channel{
		Name:                 "balance-reset-test",
		Type:                 constant.ChannelTypeOpenAI,
		Status:               status,
		Key:                  "sk-test",
		AutoBan:              &autoBan,
		BalanceResetAmount:   amount,
		BalanceResetNextTime: nextTime,
	}
	if periodDays != nil {
		ch.BalanceResetPeriodDays = periodDays
	}
	require.NoError(t, DB.Create(ch).Error)
	return ch
}

// TestGetDueBalanceResetChannels_OnlyDue 仅命中到期且启用（next_time 非空且 > 0）的渠道
func TestGetDueBalanceResetChannels_OnlyDue(t *testing.T) {
	cleanupBalanceTestChannels(t)
	now := GetDBTimestamp()
	amount := 10.0
	period := 1
	past := now - 60
	future := now + 3600

	due := createResetChannel(t, common.ChannelStatusEnabled, &amount, &period, &past)
	createResetChannel(t, common.ChannelStatusEnabled, &amount, &period, &future)
	createResetChannel(t, common.ChannelStatusEnabled, &amount, &period, nil)

	dueChannels, err := GetDueBalanceResetChannels(100)
	require.NoError(t, err)
	require.Len(t, dueChannels, 1)
	require.Equal(t, due.Id, dueChannels[0].Id)
}

// TestResetDueChannelBalances_ResetsAndAdvances 到期后余额重置为设定金额，下次时间按周期推进到未来
func TestResetDueChannelBalances_ResetsAndAdvances(t *testing.T) {
	cleanupBalanceTestChannels(t)
	now := GetDBTimestamp()
	amount := 10.0
	period := 1
	past := now - 60

	ch := createResetChannel(t, common.ChannelStatusEnabled, &amount, &period, &past)

	n, err := ResetDueChannelBalances(100)
	require.NoError(t, err)
	require.Equal(t, 1, n)

	var dbCh Channel
	require.NoError(t, DB.Where("id = ?", ch.Id).First(&dbCh).Error)
	require.InDelta(t, 10.0, dbCh.Balance, 0.001)
	require.True(t, dbCh.ChannelInfo.BalanceEverNonZero)
	require.NotNil(t, dbCh.BalanceResetNextTime)
	// 推进后的下次时间必须落在未来，且与原时间对齐到周期边界（now-60 + 86400）
	require.Greater(t, *dbCh.BalanceResetNextTime, now)
	require.Equal(t, past+86400, *dbCh.BalanceResetNextTime)
	require.Equal(t, now, dbCh.BalanceUpdatedTime)
}

// TestResetDueChannelBalances_NoPeriodDisables period_days<=0 时重置一次后自动关闭
func TestResetDueChannelBalances_NoPeriodDisables(t *testing.T) {
	cleanupBalanceTestChannels(t)
	now := GetDBTimestamp()
	amount := 5.0
	period := 0
	past := now - 60

	ch := createResetChannel(t, common.ChannelStatusEnabled, &amount, &period, &past)

	n, err := ResetDueChannelBalances(100)
	require.NoError(t, err)
	require.Equal(t, 1, n)

	var dbCh Channel
	require.NoError(t, DB.Where("id = ?", ch.Id).First(&dbCh).Error)
	require.InDelta(t, 5.0, dbCh.Balance, 0.001)
	require.Nil(t, dbCh.BalanceResetNextTime)
}

// TestResetDueChannelBalances_ReenablesAutoDisabled 自动禁用的渠道重置为正余额后自动启用
func TestResetDueChannelBalances_ReenablesAutoDisabled(t *testing.T) {
	cleanupBalanceTestChannels(t)
	now := GetDBTimestamp()
	amount := 10.0
	period := 1
	past := now - 60

	autoDisabled := createResetChannel(t, common.ChannelStatusAutoDisabled, &amount, &period, &past)
	manualDisabled := createResetChannel(t, common.ChannelStatusManuallyDisabled, &amount, &period, &past)

	n, err := ResetDueChannelBalances(100)
	require.NoError(t, err)
	require.Equal(t, 2, n)

	var dbAuto, dbManual Channel
	require.NoError(t, DB.Where("id = ?", autoDisabled.Id).First(&dbAuto).Error)
	require.NoError(t, DB.Where("id = ?", manualDisabled.Id).First(&dbManual).Error)
	require.Equal(t, common.ChannelStatusEnabled, dbAuto.Status)
	require.Equal(t, common.ChannelStatusManuallyDisabled, dbManual.Status)
}

// TestResetDueChannelBalances_ZeroAmountKeepsDisabled 重置金额为 0 时不恢复自动禁用状态
func TestResetDueChannelBalances_ZeroAmountKeepsDisabled(t *testing.T) {
	cleanupBalanceTestChannels(t)
	now := GetDBTimestamp()
	amount := 0.0
	period := 1
	past := now - 60

	ch := createResetChannel(t, common.ChannelStatusAutoDisabled, &amount, &period, &past)

	n, err := ResetDueChannelBalances(100)
	require.NoError(t, err)
	require.Equal(t, 1, n)

	var dbCh Channel
	require.NoError(t, DB.Where("id = ?", ch.Id).First(&dbCh).Error)
	require.Equal(t, 0.0, dbCh.Balance)
	require.Equal(t, common.ChannelStatusAutoDisabled, dbCh.Status)
}

// TestResetDueChannelBalances_SkipsIncompleteConfig 配置不完整（缺金额）的渠道被跳过
func TestResetDueChannelBalances_SkipsIncompleteConfig(t *testing.T) {
	cleanupBalanceTestChannels(t)
	now := GetDBTimestamp()
	period := 1
	past := now - 60

	ch := createResetChannel(t, common.ChannelStatusEnabled, nil, &period, &past)

	n, err := ResetDueChannelBalances(100)
	require.NoError(t, err)
	require.Equal(t, 0, n)

	var dbCh Channel
	require.NoError(t, DB.Where("id = ?", ch.Id).First(&dbCh).Error)
	require.Equal(t, 0.0, dbCh.Balance)
}

// TestResetDueChannelBalances_NotDue 未到期的渠道不被重置
func TestResetDueChannelBalances_NotDue(t *testing.T) {
	cleanupBalanceTestChannels(t)
	now := GetDBTimestamp()
	amount := 10.0
	period := 1
	future := now + 3600

	ch := createResetChannel(t, common.ChannelStatusEnabled, &amount, &period, &future)

	n, err := ResetDueChannelBalances(100)
	require.NoError(t, err)
	require.Equal(t, 0, n)

	var dbCh Channel
	require.NoError(t, DB.Where("id = ?", ch.Id).First(&dbCh).Error)
	require.Equal(t, 0.0, dbCh.Balance)
}
