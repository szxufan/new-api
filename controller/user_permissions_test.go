package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

// calculateUserPermissions 的角色分支由 if/else 链改写为 tagged switch（staticcheck QF1003）。
// 本测试锁定改写前后的行为契约：三种角色各自的 sidebar_settings 与 sidebar_modules 组合，
// 以及未知角色必须落入 default（与普通用户一致）。
func TestCalculateUserPermissionsByRole(t *testing.T) {
	tests := []struct {
		name             string
		userRole         int
		wantSidebarSet   bool
		wantAdminPresent bool
		wantAdminSetting bool // 仅当 admin 是 map 时有意义
	}{
		{
			name:             "root user has no sidebar settings and empty modules",
			userRole:         common.RoleRootUser,
			wantSidebarSet:   false,
			wantAdminPresent: false,
		},
		{
			name:             "admin user gets admin modules with system settings disabled",
			userRole:         common.RoleAdminUser,
			wantSidebarSet:   true,
			wantAdminPresent: true,
			wantAdminSetting: false,
		},
		{
			name:             "common user gets admin modules disabled",
			userRole:         common.RoleCommonUser,
			wantSidebarSet:   true,
			wantAdminPresent: true,
		},
		{
			name:             "unknown role falls back to common user behaviour",
			userRole:         999,
			wantSidebarSet:   true,
			wantAdminPresent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			permissions := calculateUserPermissions(tt.userRole)

			require.Equal(t, tt.wantSidebarSet, permissions["sidebar_settings"])

			modules, ok := permissions["sidebar_modules"].(map[string]interface{})
			require.True(t, ok, "sidebar_modules must be a string-keyed map")

			admin, present := modules["admin"]
			require.Equal(t, tt.wantAdminPresent, present)
			if !present {
				return
			}

			// 普通用户与未知角色的 admin 是布尔 false；管理员角色的 admin 是嵌套 map。
			if tt.userRole == common.RoleAdminUser {
				adminMap, ok := admin.(map[string]interface{})
				require.True(t, ok, "admin modules must be a nested map")
				require.Equal(t, tt.wantAdminSetting, adminMap["setting"])
			} else {
				require.Equal(t, false, admin)
			}
		})
	}
}
