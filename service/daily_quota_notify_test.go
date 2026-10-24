package service

import (
	"fmt"
	"strings"
	"testing"
	"time"

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

// setupDailyQuotaNotifyTestDB 初始化内存 sqlite 作为用户/日志表，供消息文案与 Top 模型统计测试使用
func setupDailyQuotaNotifyTestDB(t *testing.T) {
	t.Helper()
	common.UsingSQLite = true
	common.RedisEnabled = false
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	model.DB = db
	model.LOG_DB = db
	if err := model.DB.AutoMigrate(&model.User{}, &model.Log{}); err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}
}

func TestBuildDailyQuotaNotifyTextShowsNameWithoutID(t *testing.T) {
	setupDailyQuotaNotifyTestDB(t)
	user := &model.User{Username: "alice", Status: common.UserStatusEnabled}
	if err := model.DB.Create(user).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	text := buildDailyQuotaNotifyText(user.Id, 1500, 3, 500, nil)
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

	text := buildDailyQuotaNotifyText(user.Id, 1500, 3, 500, nil)
	if !strings.Contains(text, "用户 **bob**（张三）") {
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

	text := buildDailyQuotaNotifyText(user.Id, 1500, 3, 500, nil)
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
	text := buildDailyQuotaNotifyText(missingUserId, 1500, 3, 500, nil)
	wantFallback := fmt.Sprintf("用户 **用户%d**", missingUserId)
	if !strings.Contains(text, wantFallback) {
		t.Errorf("notify text should fall back to %q, got %q", wantFallback, text)
	}
	if strings.Contains(text, "ID:") {
		t.Errorf("notify text should not contain user ID label, got %q", text)
	}
	if strings.Contains(text, "Top") {
		t.Errorf("notify text should not contain top models section when stats missing, got %q", text)
	}
}

func seedUserConsumeLog(t *testing.T, userId int, modelName string, quota int, logType int) {
	t.Helper()
	if err := model.LOG_DB.Create(&model.Log{
		UserId:    userId,
		CreatedAt: time.Now().Unix(),
		Type:      logType,
		ModelName: modelName,
		Quota:     quota,
	}).Error; err != nil {
		t.Fatalf("failed to create test log: %v", err)
	}
}

func TestGetUserTopConsumeModels(t *testing.T) {
	setupDailyQuotaNotifyTestDB(t)
	const userId = 42

	seedUserConsumeLog(t, userId, "gpt-4o", 700, model.LogTypeConsume)
	seedUserConsumeLog(t, userId, "gpt-4o", 500, model.LogTypeConsume)
	seedUserConsumeLog(t, userId, "claude-3-5-sonnet", 900, model.LogTypeConsume)
	seedUserConsumeLog(t, userId, "gemini-2.0-flash", 300, model.LogTypeConsume)
	seedUserConsumeLog(t, userId, "deepseek-chat", 200, model.LogTypeConsume)
	seedUserConsumeLog(t, userId, "qwen-max", 100, model.LogTypeConsume)
	seedUserConsumeLog(t, userId, "o1-preview", 50, model.LogTypeConsume)
	// 噪音数据：不应计入
	seedUserConsumeLog(t, userId, "gpt-4o", 999, model.LogTypeRefund)           // 非消费类型
	seedUserConsumeLog(t, 999, "other-user-model", 9999, model.LogTypeConsume)  // 其他用户
	seedUserConsumeLog(t, userId, "refunded-model", -400, model.LogTypeConsume) // 净负数应被过滤
	seedUserConsumeLog(t, userId, "", 888, model.LogTypeConsume)                // 空模型名应被过滤

	rows, err := model.GetUserTopConsumeModels(userId, time.Now().Add(-time.Hour).Unix(), time.Now().Add(time.Hour).Unix(), 5)
	if err != nil {
		t.Fatalf("GetUserTopConsumeModels failed: %v", err)
	}
	wantModels := []string{"gpt-4o", "claude-3-5-sonnet", "gemini-2.0-flash", "deepseek-chat", "qwen-max"}
	if len(rows) != len(wantModels) {
		t.Fatalf("GetUserTopConsumeModels returned %d rows, want %d: %+v", len(rows), len(wantModels), rows)
	}
	for i, want := range wantModels {
		if rows[i].ModelName != want {
			t.Errorf("rows[%d].ModelName = %q, want %q", i, rows[i].ModelName, want)
		}
	}
	if rows[0].Quota != 1200 {
		t.Errorf("rows[0].Quota = %d, want 1200 (gpt-4o aggregated)", rows[0].Quota)
	}
}

func TestBuildDailyQuotaNotifyTextShowsTopModels(t *testing.T) {
	setupDailyQuotaNotifyTestDB(t)
	user := &model.User{Username: "dave", Status: common.UserStatusEnabled}
	if err := model.DB.Create(user).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	topModels := []model.UserConsumeModelStat{
		{ModelName: "gpt-4o", Quota: 1200},
		{ModelName: "claude-3-5-sonnet", Quota: 900},
	}
	text := buildDailyQuotaNotifyText(user.Id, 1500, 3, 500, topModels)
	if !strings.Contains(text, "今日消费 Top2 模型：") {
		t.Errorf("notify text should contain top models header, got %q", text)
	}
	if !strings.Contains(text, "1. gpt-4o：") || !strings.Contains(text, "2. claude-3-5-sonnet：") {
		t.Errorf("notify text should list top models in order, got %q", text)
	}
}
