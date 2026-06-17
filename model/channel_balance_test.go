package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/require"
)

// cleanupBalanceTestChannels 清理测试创建的渠道
func cleanupBalanceTestChannels(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		DB.Exec("DELETE FROM channels")
	})
}

// createBalanceChannel 创建一个测试渠道并写入 DB
func createBalanceChannel(t *testing.T, balance float64, everNonZero bool) *Channel {
	t.Helper()
	autoBan := 1
	ch := &Channel{
		Name:    "balance-test",
		Type:    constant.ChannelTypeOpenAI,
		Status:  common.ChannelStatusEnabled,
		Key:     "sk-test",
		Balance: balance,
		AutoBan: &autoBan,
	}
	ch.ChannelInfo.BalanceEverNonZero = everNonZero
	require.NoError(t, DB.Create(ch).Error)
	return ch
}

// TestDeductChannelBalance_NormalDeduction 余额大于扣减额时正确扣减
func TestDeductChannelBalance_NormalDeduction(t *testing.T) {
	cleanupBalanceTestChannels(t)
	ch := createBalanceChannel(t, 10.0, true)

	result, err := DeductChannelBalance(ch.Id, 3.0)
	require.NoError(t, err)
	require.NotNil(t, result)
	if result.Balance < 6.99 || result.Balance > 7.01 {
		t.Errorf("expected balance ~7.0, got %v", result.Balance)
	}
}

// TestDeductChannelBalance_BalanceLessThanDeduction 余额小于扣减额时归零，不出现负数
func TestDeductChannelBalance_BalanceLessThanDeduction(t *testing.T) {
	cleanupBalanceTestChannels(t)
	ch := createBalanceChannel(t, 2.0, true)

	result, err := DeductChannelBalance(ch.Id, 5.0)
	require.NoError(t, err)
	if result.Balance != 0 {
		t.Errorf("expected balance 0, got %v", result.Balance)
	}
}

// TestDeductChannelBalance_BalanceEqualsDeduction 余额等于扣减额时归零
func TestDeductChannelBalance_BalanceEqualsDeduction(t *testing.T) {
	cleanupBalanceTestChannels(t)
	ch := createBalanceChannel(t, 5.0, true)

	result, err := DeductChannelBalance(ch.Id, 5.0)
	require.NoError(t, err)
	if result.Balance != 0 {
		t.Errorf("expected balance 0, got %v", result.Balance)
	}
}

// TestDeductChannelBalance_ZeroBalance 余额为0时不扣减，保持0
func TestDeductChannelBalance_ZeroBalance(t *testing.T) {
	cleanupBalanceTestChannels(t)
	ch := createBalanceChannel(t, 0.0, false)

	result, err := DeductChannelBalance(ch.Id, 5.0)
	require.NoError(t, err)
	if result.Balance != 0 {
		t.Errorf("expected balance 0, got %v", result.Balance)
	}
}

// TestDeductChannelBalance_NonPositiveDeduction 扣减值<=0时不扣减，直接返回渠道
func TestDeductChannelBalance_NonPositiveDeduction(t *testing.T) {
	cleanupBalanceTestChannels(t)
	ch := createBalanceChannel(t, 10.0, true)

	result, err := DeductChannelBalance(ch.Id, 0)
	require.NoError(t, err)
	if result.Balance != 10.0 {
		t.Errorf("expected balance 10.0 (no deduction), got %v", result.Balance)
	}

	result, err = DeductChannelBalance(ch.Id, -1.0)
	require.NoError(t, err)
	if result.Balance != 10.0 {
		t.Errorf("expected balance 10.0 (no deduction), got %v", result.Balance)
	}
}

// TestDeductChannelBalance_MultipleDeductions 多次扣减累计正确
func TestDeductChannelBalance_MultipleDeductions(t *testing.T) {
	cleanupBalanceTestChannels(t)
	ch := createBalanceChannel(t, 10.0, true)

	// 第一次扣 3
	_, err := DeductChannelBalance(ch.Id, 3.0)
	require.NoError(t, err)
	// 第二次扣 4
	result, err := DeductChannelBalance(ch.Id, 4.0)
	require.NoError(t, err)
	// 10 - 3 - 4 = 3
	if result.Balance < 2.99 || result.Balance > 3.01 {
		t.Errorf("expected balance ~3.0, got %v", result.Balance)
	}
}

// TestUpdateBalance_SetsFlagWhenPositive balance>0时设置 BalanceEverNonZero 标志
func TestUpdateBalance_SetsFlagWhenPositive(t *testing.T) {
	cleanupBalanceTestChannels(t)
	ch := createBalanceChannel(t, 0.0, false)

	ch.UpdateBalance(5.0)

	// 重新从 DB 读取验证
	var dbCh Channel
	require.NoError(t, DB.Where("id = ?", ch.Id).First(&dbCh).Error)
	if !dbCh.ChannelInfo.BalanceEverNonZero {
		t.Error("expected BalanceEverNonZero=true after setting positive balance")
	}
	if dbCh.Balance < 4.99 || dbCh.Balance > 5.01 {
		t.Errorf("expected balance ~5.0, got %v", dbCh.Balance)
	}
}

// TestUpdateBalance_DoesNotSetFlagWhenZero balance=0时不设置标志（若之前未设置）
func TestUpdateBalance_DoesNotSetFlagWhenZero(t *testing.T) {
	cleanupBalanceTestChannels(t)
	ch := createBalanceChannel(t, 0.0, false)

	ch.UpdateBalance(0.0)

	var dbCh Channel
	require.NoError(t, DB.Where("id = ?", ch.Id).First(&dbCh).Error)
	if dbCh.ChannelInfo.BalanceEverNonZero {
		t.Error("expected BalanceEverNonZero=false after setting zero balance")
	}
	if dbCh.Balance != 0 {
		t.Errorf("expected balance 0, got %v", dbCh.Balance)
	}
}

// TestUpdateBalance_DoesNotResetFlag balance>0且标志已为true时不重复设置（值仍为true）
func TestUpdateBalance_DoesNotResetFlag(t *testing.T) {
	cleanupBalanceTestChannels(t)
	ch := createBalanceChannel(t, 10.0, true)

	// 更新为0，标志应保持true（不被重置）
	ch.UpdateBalance(0.0)

	var dbCh Channel
	require.NoError(t, DB.Where("id = ?", ch.Id).First(&dbCh).Error)
	if !dbCh.ChannelInfo.BalanceEverNonZero {
		t.Error("expected BalanceEverNonZero to remain true after setting balance to 0")
	}
}
