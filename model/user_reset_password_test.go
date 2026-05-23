package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) {
	t.Helper()
	common.UsingSQLite = true
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	DB = db
	err = DB.AutoMigrate(&User{})
	if err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}
}

func createTestUser(t *testing.T, username string, role int) *User {
	t.Helper()
	hashedPassword, err := common.Password2Hash("oldpassword123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	user := &User{
		Username: username,
		Password: hashedPassword,
		Role:     role,
		Status:   common.UserStatusEnabled,
	}
	if err := DB.Create(user).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	return user
}

func TestResetUserPasswordByUsername(t *testing.T) {
	setupTestDB(t)

	createTestUser(t, "testroot", common.RoleRootUser)

	err := ResetUserPasswordByUsername("testroot", "newpassword123")
	if err != nil {
		t.Fatalf("ResetUserPasswordByUsername failed: %v", err)
	}

	var user User
	if err := DB.Where("username = ?", "testroot").First(&user).Error; err != nil {
		t.Fatalf("failed to find user: %v", err)
	}

	if !common.ValidatePasswordAndHash("newpassword123", user.Password) {
		t.Error("new password does not match after reset")
	}

	if common.ValidatePasswordAndHash("oldpassword123", user.Password) {
		t.Error("old password still matches after reset")
	}
}

func TestResetUserPasswordByUsernameNotFound(t *testing.T) {
	setupTestDB(t)

	err := ResetUserPasswordByUsername("nonexistent", "newpassword123")
	if err == nil {
		t.Error("expected error for non-existent username, got nil")
	}
}

func TestResetUserPasswordByUsernameEmpty(t *testing.T) {
	setupTestDB(t)

	err := ResetUserPasswordByUsername("", "newpassword123")
	if err == nil {
		t.Error("expected error for empty username, got nil")
	}

	err = ResetUserPasswordByUsername("testroot", "")
	if err == nil {
		t.Error("expected error for empty password, got nil")
	}
}

func TestResetUserPasswordByID(t *testing.T) {
	setupTestDB(t)

	user := createTestUser(t, "testadmin", common.RoleAdminUser)

	err := ResetUserPasswordByID(user.Id, "newpassword456")
	if err != nil {
		t.Fatalf("ResetUserPasswordByID failed: %v", err)
	}

	var updatedUser User
	if err := DB.Where("id = ?", user.Id).First(&updatedUser).Error; err != nil {
		t.Fatalf("failed to find user: %v", err)
	}

	if !common.ValidatePasswordAndHash("newpassword456", updatedUser.Password) {
		t.Error("new password does not match after reset by ID")
	}
}

func TestResetUserPasswordByIDNotFound(t *testing.T) {
	setupTestDB(t)

	err := ResetUserPasswordByID(99999, "newpassword123")
	if err == nil {
		t.Error("expected error for non-existent user ID, got nil")
	}
}

func TestResetUserPasswordByIDZero(t *testing.T) {
	setupTestDB(t)

	err := ResetUserPasswordByID(0, "newpassword123")
	if err == nil {
		t.Error("expected error for zero user ID, got nil")
	}

	err = ResetUserPasswordByID(1, "")
	if err == nil {
		t.Error("expected error for empty password, got nil")
	}
}

func TestResetUserPasswordByUsernamePasswordHashed(t *testing.T) {
	setupTestDB(t)

	createTestUser(t, "hashtest", common.RoleCommonUser)

	newPassword := "hashedpass123"
	err := ResetUserPasswordByUsername("hashtest", newPassword)
	if err != nil {
		t.Fatalf("ResetUserPasswordByUsername failed: %v", err)
	}

	var user User
	if err := DB.Where("username = ?", "hashtest").First(&user).Error; err != nil {
		t.Fatalf("failed to find user: %v", err)
	}

	if user.Password == newPassword {
		t.Error("password stored as plaintext instead of bcrypt hash")
	}

	if len(user.Password) < 20 {
		t.Error("password hash appears too short to be a valid bcrypt hash")
	}
}
