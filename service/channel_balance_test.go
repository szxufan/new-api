package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

// cleanupBalanceChannels 清理测试创建的渠道
func cleanupBalanceChannels(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		model.DB.Exec("DELETE FROM channels")
	})
}

// makeBalanceChannel 构造一个测试渠道并写入 DB
func makeBalanceChannel(t *testing.T, id int, balance float64, everNonZero bool, autoBan int) *model.Channel {
	t.Helper()
	ch := &model.Channel{
		Id:      id,
		Name:    "svc-balance-test",
		Type:    constant.ChannelTypeOpenAI,
		Status:  common.ChannelStatusEnabled,
		Balance: balance,
		AutoBan: &autoBan,
	}
	ch.ChannelInfo.BalanceEverNonZero = everNonZero
	require.NoError(t, model.DB.Create(ch).Error)
	return ch
}

// TestDisableChannelIfBalanceDepleted_NilChannel nil 渠道返回 false
func TestDisableChannelIfBalanceDepleted_NilChannel(t *testing.T) {
	if DisableChannelIfBalanceDepleted(nil, "test") {
		t.Error("expected false for nil channel")
	}
}

// TestDisableChannelIfBalanceDepleted_NeverHadBalance 从未有过非0余额不禁用
func TestDisableChannelIfBalanceDepleted_NeverHadBalance(t *testing.T) {
	ch := &model.Channel{}
	ch.ChannelInfo.BalanceEverNonZero = false
	ch.Balance = 0
	autoBan := 1
	ch.AutoBan = &autoBan
	if DisableChannelIfBalanceDepleted(ch, "test") {
		t.Error("expected false when BalanceEverNonZero=false")
	}
}

// TestDisableChannelIfBalanceDepleted_HasBalance 余额>0不禁用
func TestDisableChannelIfBalanceDepleted_HasBalance(t *testing.T) {
	ch := &model.Channel{}
	ch.ChannelInfo.BalanceEverNonZero = true
	ch.Balance = 5.0
	autoBan := 1
	ch.AutoBan = &autoBan
	if DisableChannelIfBalanceDepleted(ch, "test") {
		t.Error("expected false when balance > 0")
	}
}

// TestDisableChannelIfBalanceDepleted_AutoBanOff AutoBan=false 不禁用
func TestDisableChannelIfBalanceDepleted_AutoBanOff(t *testing.T) {
	ch := &model.Channel{}
	ch.ChannelInfo.BalanceEverNonZero = true
	ch.Balance = 0
	autoBan := 0
	ch.AutoBan = &autoBan
	if DisableChannelIfBalanceDepleted(ch, "test") {
		t.Error("expected false when AutoBan=false")
	}
}

// TestDisableChannelIfBalanceDepleted_AllConditionsMet 所有条件满足时触发禁用
func TestDisableChannelIfBalanceDepleted_AllConditionsMet(t *testing.T) {
	cleanupBalanceChannels(t)
	ch := makeBalanceChannel(t, 1001, 5.0, true, 1)

	// DisableChannel 内部只检查 AutoBan，不检查 AutomaticDisableChannelEnabled
	// 余额设为0，触发禁用
	ch.Balance = 0
	triggered := DisableChannelIfBalanceDepleted(ch, "余额耗尽")
	if !triggered {
		t.Error("expected true when all conditions met")
	}

	// 验证 DB 中状态已变为 AutoDisabled
	var dbCh model.Channel
	require.NoError(t, model.DB.Where("id = ?", ch.Id).First(&dbCh).Error)
	if dbCh.Status != common.ChannelStatusAutoDisabled {
		t.Errorf("expected status AutoDisabled(%d), got %d", common.ChannelStatusAutoDisabled, dbCh.Status)
	}
}

// TestUpdateChannelQuotaAndBalance_QuotaNonPositive quota<=0 时跳过扣减
func TestUpdateChannelQuotaAndBalance_QuotaNonPositive(t *testing.T) {
	cleanupBalanceChannels(t)
	ch := makeBalanceChannel(t, 1002, 10.0, true, 1)

	originalQuotaPerUnit := common.QuotaPerUnit
	defer func() { common.QuotaPerUnit = originalQuotaPerUnit }()
	common.QuotaPerUnit = 500000

	// quota=0 不应扣减余额
	UpdateChannelQuotaAndBalance(ch.Id, 0)

	var dbCh model.Channel
	require.NoError(t, model.DB.Where("id = ?", ch.Id).First(&dbCh).Error)
	if dbCh.Balance < 9.99 || dbCh.Balance > 10.01 {
		t.Errorf("expected balance ~10.0 (no deduction), got %v", dbCh.Balance)
	}
}

// TestUpdateChannelQuotaAndBalance_QuotaPerUnitInvalid QuotaPerUnit<=0 时不 panic、不扣减
func TestUpdateChannelQuotaAndBalance_QuotaPerUnitInvalid(t *testing.T) {
	cleanupBalanceChannels(t)
	ch := makeBalanceChannel(t, 1003, 10.0, true, 1)

	originalQuotaPerUnit := common.QuotaPerUnit
	defer func() { common.QuotaPerUnit = originalQuotaPerUnit }()
	common.QuotaPerUnit = 0

	// 不应 panic，余额不变
	UpdateChannelQuotaAndBalance(ch.Id, 1000)

	var dbCh model.Channel
	require.NoError(t, model.DB.Where("id = ?", ch.Id).First(&dbCh).Error)
	if dbCh.Balance < 9.99 || dbCh.Balance > 10.01 {
		t.Errorf("expected balance ~10.0 (no deduction), got %v", dbCh.Balance)
	}

	// 测试负值
	common.QuotaPerUnit = -1
	UpdateChannelQuotaAndBalance(ch.Id, 1000)
}

// TestUpdateChannelQuotaAndBalance_BalanceDepletedTriggersDisable 余额耗尽触发禁用
func TestUpdateChannelQuotaAndBalance_BalanceDepletedTriggersDisable(t *testing.T) {
	cleanupBalanceChannels(t)
	ch := makeBalanceChannel(t, 1004, 0.01, true, 1)

	originalQuotaPerUnit := common.QuotaPerUnit
	defer func() { common.QuotaPerUnit = originalQuotaPerUnit }()
	common.QuotaPerUnit = 500000

	// 消费 10000 quota = 0.02 USD > 0.01 余额，应归零并触发禁用
	UpdateChannelQuotaAndBalance(ch.Id, 10000)

	var dbCh model.Channel
	require.NoError(t, model.DB.Where("id = ?", ch.Id).First(&dbCh).Error)
	if dbCh.Balance != 0 {
		t.Errorf("expected balance 0, got %v", dbCh.Balance)
	}
	if dbCh.Status != common.ChannelStatusAutoDisabled {
		t.Errorf("expected status AutoDisabled, got %d", dbCh.Status)
	}
}

// TestUpdateChannelQuotaAndBalance_AutoBanOffNoDisable AutoBan=false 时余额耗尽不禁用
func TestUpdateChannelQuotaAndBalance_AutoBanOffNoDisable(t *testing.T) {
	cleanupBalanceChannels(t)
	ch := makeBalanceChannel(t, 1005, 0.01, true, 0)

	originalQuotaPerUnit := common.QuotaPerUnit
	defer func() { common.QuotaPerUnit = originalQuotaPerUnit }()
	common.QuotaPerUnit = 500000

	UpdateChannelQuotaAndBalance(ch.Id, 10000)

	var dbCh model.Channel
	require.NoError(t, model.DB.Where("id = ?", ch.Id).First(&dbCh).Error)
	if dbCh.Balance != 0 {
		t.Errorf("expected balance 0, got %v", dbCh.Balance)
	}
	// AutoBan=false，不应禁用
	if dbCh.Status != common.ChannelStatusEnabled {
		t.Errorf("expected status Enabled (AutoBan off), got %d", dbCh.Status)
	}
}

// TestUpdateChannelQuotaAndBalance_NeverHadBalanceNoDisable 从未有过非0余额不禁用
func TestUpdateChannelQuotaAndBalance_NeverHadBalanceNoDisable(t *testing.T) {
	cleanupBalanceChannels(t)
	// 余额=0，标志=false（从未有过余额）
	ch := makeBalanceChannel(t, 1006, 0, false, 1)

	originalQuotaPerUnit := common.QuotaPerUnit
	defer func() { common.QuotaPerUnit = originalQuotaPerUnit }()
	common.QuotaPerUnit = 500000

	UpdateChannelQuotaAndBalance(ch.Id, 10000)

	var dbCh model.Channel
	require.NoError(t, model.DB.Where("id = ?", ch.Id).First(&dbCh).Error)
	// 余额本来就是0，扣减后仍为0，但不应禁用
	if dbCh.Status != common.ChannelStatusEnabled {
		t.Errorf("expected status Enabled (never had balance), got %d", dbCh.Status)
	}
}

// TestUpdateChannelQuotaAndBalance_PartialDeduction 余额充足时部分扣减，不禁用
func TestUpdateChannelQuotaAndBalance_PartialDeduction(t *testing.T) {
	cleanupBalanceChannels(t)
	ch := makeBalanceChannel(t, 1007, 10.0, true, 1)

	originalQuotaPerUnit := common.QuotaPerUnit
	defer func() { common.QuotaPerUnit = originalQuotaPerUnit }()
	common.QuotaPerUnit = 500000

	// 消费 1000000 quota = 2.0 USD，余额 10 -> 8
	UpdateChannelQuotaAndBalance(ch.Id, 1000000)

	var dbCh model.Channel
	require.NoError(t, model.DB.Where("id = ?", ch.Id).First(&dbCh).Error)
	if dbCh.Balance < 7.99 || dbCh.Balance > 8.01 {
		t.Errorf("expected balance ~8.0, got %v", dbCh.Balance)
	}
	if dbCh.Status != common.ChannelStatusEnabled {
		t.Errorf("expected status Enabled (balance remaining), got %d", dbCh.Status)
	}
}
