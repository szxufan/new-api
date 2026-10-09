package service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestConvertDisplayCurrencyToQuota(t *testing.T) {
	common.QuotaPerUnit = 500000
	operation_setting.USDExchangeRate = 7.3
	operation_setting.GetGeneralSetting().CustomCurrencyExchangeRate = 2.0

	cases := []struct {
		name   string
		val    float64
		disp   string
		expect int
	}{
		{"USD 1", 1, operation_setting.QuotaDisplayTypeUSD, 500000},
		{"USD 0.5", 0.5, operation_setting.QuotaDisplayTypeUSD, 250000},
		{"CNY 7.3", 7.3, operation_setting.QuotaDisplayTypeCNY, 500000},
		{"CUSTOM 2", 2, operation_setting.QuotaDisplayTypeCustom, 500000},
		{"TOKENS 123", 123, operation_setting.QuotaDisplayTypeTokens, 123},
	}
	for _, c := range cases {
		if got := convertDisplayCurrencyToQuota(c.val, c.disp); got != c.expect {
			t.Errorf("%s: convertDisplayCurrencyToQuota(%v, %s) = %d, want %d", c.name, c.val, c.disp, got, c.expect)
		}
	}
}

func TestConvertDisplayCurrencyToQuotaGuardedRates(t *testing.T) {
	common.QuotaPerUnit = 500000

	operation_setting.USDExchangeRate = 0
	if got := convertDisplayCurrencyToQuota(1, operation_setting.QuotaDisplayTypeCNY); got != 500000 {
		t.Errorf("CNY with zero rate: got %d, want 500000", got)
	}

	operation_setting.GetGeneralSetting().CustomCurrencyExchangeRate = -1
	if got := convertDisplayCurrencyToQuota(1, operation_setting.QuotaDisplayTypeCustom); got != 500000 {
		t.Errorf("CUSTOM with negative rate: got %d, want 500000", got)
	}
}

// setupDailyQuotaNotifyTestDB 初始化内存 sqlite 作为用户表，供消息文案测试使用
func setupDailyQuotaNotifyTestDB(t *testing.T) {
	t.Helper()
	common.UsingSQLite = true
	common.RedisEnabled = false
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	model.DB = db
	if err := model.DB.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}
}

func TestBuildDailyQuotaNotifyTextShowsNameWithoutID(t *testing.T) {
	setupDailyQuotaNotifyTestDB(t)
	user := &model.User{Username: "alice", Status: common.UserStatusEnabled}
	if err := model.DB.Create(user).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	text := buildDailyQuotaNotifyText(user.Id, 1500, 3, 500)
	if !strings.Contains(text, "用户 **alice**") {
		t.Errorf("notify text should show username, got %q", text)
	}
	if strings.Contains(text, "显示名称：") {
		t.Errorf("notify text should not show display name when unset, got %q", text)
	}
	if strings.Contains(text, "ID:") {
		t.Errorf("notify text should not contain user ID label, got %q", text)
	}
	if !strings.Contains(text, DailyQuotaNotifyTitle) {
		t.Errorf("notify text should contain title %q, got %q", DailyQuotaNotifyTitle, text)
	}
}

func TestBuildDailyQuotaNotifyTextShowsDisplayName(t *testing.T) {
	setupDailyQuotaNotifyTestDB(t)
	user := &model.User{Username: "bob", DisplayName: "张三", Status: common.UserStatusEnabled}
	if err := model.DB.Create(user).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	text := buildDailyQuotaNotifyText(user.Id, 1500, 3, 500)
	if !strings.Contains(text, "用户 **bob**（显示名称：张三）") {
		t.Errorf("notify text should show username with display name, got %q", text)
	}
	if strings.Contains(text, "ID:") {
		t.Errorf("notify text should not contain user ID label, got %q", text)
	}
}

func TestBuildDailyQuotaNotifyTextSkipsDisplayNameEqualToUsername(t *testing.T) {
	setupDailyQuotaNotifyTestDB(t)
	user := &model.User{Username: "carol", DisplayName: "carol", Status: common.UserStatusEnabled}
	if err := model.DB.Create(user).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	text := buildDailyQuotaNotifyText(user.Id, 1500, 3, 500)
	if strings.Contains(text, "显示名称：") {
		t.Errorf("notify text should skip display name when same as username, got %q", text)
	}
	if !strings.Contains(text, "用户 **carol**") {
		t.Errorf("notify text should show username, got %q", text)
	}
}

func TestBuildDailyQuotaNotifyTextFallbackWhenUserMissing(t *testing.T) {
	setupDailyQuotaNotifyTestDB(t)

	const missingUserId = 987654
	text := buildDailyQuotaNotifyText(missingUserId, 1500, 3, 500)
	wantFallback := fmt.Sprintf("用户 **用户%d**", missingUserId)
	if !strings.Contains(text, wantFallback) {
		t.Errorf("notify text should fall back to %q, got %q", wantFallback, text)
	}
	if strings.Contains(text, "ID:") {
		t.Errorf("notify text should not contain user ID label, got %q", text)
	}
}
