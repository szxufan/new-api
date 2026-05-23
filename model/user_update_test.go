package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

func TestUpdateDoesNotOverwritePasswordWhenUpdatePasswordFalse(t *testing.T) {
	setupTestDB(t)

	user := createTestUser(t, "pwguard", common.RoleAdminUser)
	originalHash := user.Password

	user.Username = "pwguard_updated"
	user.DisplayName = "New Display"
	if err := user.Update(false); err != nil {
		t.Fatalf("Update(false) failed: %v", err)
	}

	var dbUser User
	if err := DB.Where("id = ?", user.Id).First(&dbUser).Error; err != nil {
		t.Fatalf("failed to find user: %v", err)
	}

	if dbUser.Password != originalHash {
		t.Errorf("password was overwritten when updatePassword=false: original=%s, current=%s", originalHash, dbUser.Password)
	}

	if !common.ValidatePasswordAndHash("oldpassword123", dbUser.Password) {
		t.Error("original password no longer validates after Update(false)")
	}
}

func TestUpdateOverwritesPasswordWhenUpdatePasswordTrue(t *testing.T) {
	setupTestDB(t)

	user := createTestUser(t, "pwchange", common.RoleAdminUser)
	originalHash := user.Password

	user.Password = "newsecurepass123"
	if err := user.Update(true); err != nil {
		t.Fatalf("Update(true) failed: %v", err)
	}

	var dbUser User
	if err := DB.Where("id = ?", user.Id).First(&dbUser).Error; err != nil {
		t.Fatalf("failed to find user: %v", err)
	}

	if dbUser.Password == originalHash {
		t.Error("password was not updated when updatePassword=true")
	}

	if !common.ValidatePasswordAndHash("newsecurepass123", dbUser.Password) {
		t.Error("new password does not validate after Update(true)")
	}

	if common.ValidatePasswordAndHash("oldpassword123", dbUser.Password) {
		t.Error("old password still validates after Update(true)")
	}
}

func TestUpdateDoesNotResetOtherFieldsToZeroValues(t *testing.T) {
	setupTestDB(t)

	user := createTestUser(t, "fieldguard", common.RoleAdminUser)
	user.Email = "test@example.com"
	user.Group = "vip"
	user.Quota = 5000
	user.Status = common.UserStatusEnabled
	if err := DB.Save(user).Error; err != nil {
		t.Fatalf("failed to save user: %v", err)
	}

	partialUser := &User{
		Id:       user.Id,
		Username: "fieldguard_renamed",
	}

	if err := partialUser.Update(false); err != nil {
		t.Fatalf("Update(false) with partial user failed: %v", err)
	}

	var dbUser User
	if err := DB.Where("id = ?", user.Id).First(&dbUser).Error; err != nil {
		t.Fatalf("failed to find user: %v", err)
	}

	if dbUser.Username != "fieldguard_renamed" {
		t.Errorf("username was not updated: expected fieldguard_renamed, got %s", dbUser.Username)
	}

	if dbUser.Email != "test@example.com" {
		t.Errorf("email was reset to empty: expected test@example.com, got %s", dbUser.Email)
	}

	if dbUser.Group != "vip" {
		t.Errorf("group was reset to default: expected vip, got %s", dbUser.Group)
	}

	if dbUser.Quota != 5000 {
		t.Errorf("quota was reset to zero: expected 5000, got %d", dbUser.Quota)
	}

	if dbUser.Status != common.UserStatusEnabled {
		t.Errorf("status was reset: expected %d, got %d", common.UserStatusEnabled, dbUser.Status)
	}
}

func TestUpdatePasswordNotClearedWhenPartialUserObject(t *testing.T) {
	setupTestDB(t)

	user := createTestUser(t, "partialpw", common.RoleRootUser)
	originalHash := user.Password

	partialUser := &User{
		Id:       user.Id,
		Username: "partialpw",
	}

	if err := partialUser.Update(false); err != nil {
		t.Fatalf("Update(false) with partial user (empty password) failed: %v", err)
	}

	var dbUser User
	if err := DB.Where("id = ?", user.Id).First(&dbUser).Error; err != nil {
		t.Fatalf("failed to find user: %v", err)
	}

	if dbUser.Password != originalHash {
		t.Errorf("password hash was changed even though updatePassword=false: expected %s, got %s", originalHash, dbUser.Password)
	}

	if !common.ValidatePasswordAndHash("oldpassword123", dbUser.Password) {
		t.Error("password no longer validates after Update(false) with partial user object")
	}
}

func TestUpdateOAuthBindingFields(t *testing.T) {
	setupTestDB(t)

	user := createTestUser(t, "oauthbind", common.RoleCommonUser)

	user.GitHubId = "github_user_123"
	user.DiscordId = "discord_user_456"
	user.OidcId = "oidc_user_789"
	user.WeChatId = "wechat_user_abc"
	user.TelegramId = "telegram_user_def"
	user.LinuxDOId = "linuxdo_user_ghi"
	if err := user.Update(false); err != nil {
		t.Fatalf("Update(false) with OAuth fields failed: %v", err)
	}

	var dbUser User
	if err := DB.Where("id = ?", user.Id).First(&dbUser).Error; err != nil {
		t.Fatalf("failed to find user: %v", err)
	}

	if dbUser.GitHubId != "github_user_123" {
		t.Errorf("github_id not persisted: expected github_user_123, got %s", dbUser.GitHubId)
	}
	if dbUser.DiscordId != "discord_user_456" {
		t.Errorf("discord_id not persisted: expected discord_user_456, got %s", dbUser.DiscordId)
	}
	if dbUser.OidcId != "oidc_user_789" {
		t.Errorf("oidc_id not persisted: expected oidc_user_789, got %s", dbUser.OidcId)
	}
	if dbUser.WeChatId != "wechat_user_abc" {
		t.Errorf("wechat_id not persisted: expected wechat_user_abc, got %s", dbUser.WeChatId)
	}
	if dbUser.TelegramId != "telegram_user_def" {
		t.Errorf("telegram_id not persisted: expected telegram_user_def, got %s", dbUser.TelegramId)
	}
	if dbUser.LinuxDOId != "linuxdo_user_ghi" {
		t.Errorf("linux_do_id not persisted: expected linuxdo_user_ghi, got %s", dbUser.LinuxDOId)
	}
}

func TestUpdateOAuthBindingDoesNotClearExistingBindings(t *testing.T) {
	setupTestDB(t)

	user := createTestUser(t, "oauthmulti", common.RoleCommonUser)
	user.GitHubId = "existing_github"
	user.DiscordId = "existing_discord"
	if err := DB.Save(user).Error; err != nil {
		t.Fatalf("failed to save user: %v", err)
	}

	user.TelegramId = "new_telegram"
	if err := user.Update(false); err != nil {
		t.Fatalf("Update(false) with new Telegram binding failed: %v", err)
	}

	var dbUser User
	if err := DB.Where("id = ?", user.Id).First(&dbUser).Error; err != nil {
		t.Fatalf("failed to find user: %v", err)
	}

	if dbUser.GitHubId != "existing_github" {
		t.Errorf("existing github_id was cleared: expected existing_github, got %s", dbUser.GitHubId)
	}
	if dbUser.DiscordId != "existing_discord" {
		t.Errorf("existing discord_id was cleared: expected existing_discord, got %s", dbUser.DiscordId)
	}
	if dbUser.TelegramId != "new_telegram" {
		t.Errorf("telegram_id not persisted: expected new_telegram, got %s", dbUser.TelegramId)
	}
}

func TestUpdateAffCode(t *testing.T) {
	setupTestDB(t)

	user := createTestUser(t, "affcodetest", common.RoleCommonUser)
	if user.AffCode != "" {
		t.Fatalf("expected new user to have empty aff_code, got %s", user.AffCode)
	}

	user.AffCode = "abc1"
	if err := user.Update(false); err != nil {
		t.Fatalf("Update(false) with AffCode failed: %v", err)
	}

	var dbUser User
	if err := DB.Where("id = ?", user.Id).First(&dbUser).Error; err != nil {
		t.Fatalf("failed to find user: %v", err)
	}

	if dbUser.AffCode != "abc1" {
		t.Errorf("aff_code not persisted: expected abc1, got %s", dbUser.AffCode)
	}
}

func TestUpdateAccessToken(t *testing.T) {
	setupTestDB(t)

	user := createTestUser(t, "tokenupd", common.RoleAdminUser)

	token := "sk-test-access-token-12345"
	user.SetAccessToken(token)
	if err := user.Update(false); err != nil {
		t.Fatalf("Update(false) with AccessToken failed: %v", err)
	}

	var dbUser User
	if err := DB.Where("id = ?", user.Id).First(&dbUser).Error; err != nil {
		t.Fatalf("failed to find user: %v", err)
	}

	if dbUser.AccessToken == nil || *dbUser.AccessToken != token {
		t.Errorf("access_token not persisted: expected %s, got %v", token, dbUser.AccessToken)
	}
}

func TestUpdateSetting(t *testing.T) {
	setupTestDB(t)

	user := createTestUser(t, "settingupd", common.RoleCommonUser)

	user.SetSetting(dto.UserSetting{
		SidebarModules: "console,token",
		Language:       "zh",
	})
	if err := user.Update(false); err != nil {
		t.Fatalf("Update(false) with Setting failed: %v", err)
	}

	var dbUser User
	if err := DB.Where("id = ?", user.Id).First(&dbUser).Error; err != nil {
		t.Fatalf("failed to find user: %v", err)
	}

	s := dbUser.GetSetting()
	if s.SidebarModules != "console,token" {
		t.Errorf("setting.sidebar_modules not persisted: expected console,token, got %s", s.SidebarModules)
	}
	if s.Language != "zh" {
		t.Errorf("setting.language not persisted: expected zh, got %s", s.Language)
	}
}

func TestEditDoesNotOverwritePasswordWhenUpdatePasswordFalse(t *testing.T) {
	setupTestDB(t)

	user := createTestUser(t, "editpwguard", common.RoleCommonUser)
	originalHash := user.Password

	user.Username = "editpwguard_new"
	user.DisplayName = "Edited Display"
	user.Group = "vip"
	user.Remark = "test remark"
	if err := user.Edit(false); err != nil {
		t.Fatalf("Edit(false) failed: %v", err)
	}

	var dbUser User
	if err := DB.Where("id = ?", user.Id).First(&dbUser).Error; err != nil {
		t.Fatalf("failed to find user: %v", err)
	}

	if dbUser.Password != originalHash {
		t.Errorf("password was overwritten when updatePassword=false: original=%s, current=%s", originalHash, dbUser.Password)
	}

	if !common.ValidatePasswordAndHash("oldpassword123", dbUser.Password) {
		t.Error("original password no longer validates after Edit(false)")
	}

	if dbUser.Username != "editpwguard_new" {
		t.Errorf("username was not updated: expected editpwguard_new, got %s", dbUser.Username)
	}

	if dbUser.DisplayName != "Edited Display" {
		t.Errorf("display_name was not updated: expected 'Edited Display', got '%s'", dbUser.DisplayName)
	}

	if dbUser.Group != "vip" {
		t.Errorf("group was not updated: expected vip, got %s", dbUser.Group)
	}

	if dbUser.Remark != "test remark" {
		t.Errorf("remark was not updated: expected 'test remark', got '%s'", dbUser.Remark)
	}
}

func TestEditOverwritesPasswordWhenUpdatePasswordTrue(t *testing.T) {
	setupTestDB(t)

	user := createTestUser(t, "editpwchange", common.RoleCommonUser)
	originalHash := user.Password

	user.Username = "editpwchange_new"
	user.Password = "newsecurepass999"
	if err := user.Edit(true); err != nil {
		t.Fatalf("Edit(true) failed: %v", err)
	}

	var dbUser User
	if err := DB.Where("id = ?", user.Id).First(&dbUser).Error; err != nil {
		t.Fatalf("failed to find user: %v", err)
	}

	if dbUser.Password == originalHash {
		t.Error("password was not updated when updatePassword=true")
	}

	if !common.ValidatePasswordAndHash("newsecurepass999", dbUser.Password) {
		t.Error("new password does not validate after Edit(true)")
	}

	if common.ValidatePasswordAndHash("oldpassword123", dbUser.Password) {
		t.Error("old password still validates after Edit(true)")
	}

	if dbUser.Username != "editpwchange_new" {
		t.Errorf("username was not updated: expected editpwchange_new, got %s", dbUser.Username)
	}
}

func TestEditOnlyModifiesIntendedFields(t *testing.T) {
	setupTestDB(t)

	user := createTestUser(t, "editscope", common.RoleAdminUser)
	user.Email = "original@example.com"
	user.Quota = 3000
	user.Status = common.UserStatusEnabled
	if err := DB.Save(user).Error; err != nil {
		t.Fatalf("failed to save user: %v", err)
	}

	user.Username = "editscope_new"
	user.DisplayName = "New Scope Display"
	user.Group = "premium"
	user.Remark = "scope remark"
	user.Email = ""
	user.Quota = 0
	if err := user.Edit(false); err != nil {
		t.Fatalf("Edit(false) failed: %v", err)
	}

	var dbUser User
	if err := DB.Where("id = ?", user.Id).First(&dbUser).Error; err != nil {
		t.Fatalf("failed to find user: %v", err)
	}

	if dbUser.Username != "editscope_new" {
		t.Errorf("username was not updated: expected editscope_new, got %s", dbUser.Username)
	}

	if dbUser.DisplayName != "New Scope Display" {
		t.Errorf("display_name was not updated: expected 'New Scope Display', got '%s'", dbUser.DisplayName)
	}

	if dbUser.Group != "premium" {
		t.Errorf("group was not updated: expected premium, got %s", dbUser.Group)
	}

	if dbUser.Remark != "scope remark" {
		t.Errorf("remark was not updated: expected 'scope remark', got '%s'", dbUser.Remark)
	}

	if dbUser.Email != "original@example.com" {
		t.Errorf("email was unexpectedly modified: expected original@example.com, got %s", dbUser.Email)
	}

	if dbUser.Quota != 3000 {
		t.Errorf("quota was unexpectedly modified: expected 3000, got %d", dbUser.Quota)
	}

	if dbUser.Status != common.UserStatusEnabled {
		t.Errorf("status was unexpectedly modified: expected %d, got %d", common.UserStatusEnabled, dbUser.Status)
	}
}
