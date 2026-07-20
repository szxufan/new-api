package model

import (
	"reflect"
	"testing"
)

// strPtr 返回字符串指针，便于构造 GroupBlacklist。
func strPtr(s string) *string {
	return &s
}

// TestGetGroupBlacklist 验证分组黑名单的解析：trim、去空项、nil/空串处理。
func TestGetGroupBlacklist(t *testing.T) {
	cases := []struct {
		name      string
		blacklist *string
		expected  []string
	}{
		{"nil 指针", nil, []string{}},
		{"空字符串", strPtr(""), []string{}},
		{"单个分组", strPtr("vip"), []string{"vip"}},
		{"多个分组", strPtr("vip,internal"), []string{"vip", "internal"}},
		{"含空格与空项", strPtr(" vip , , internal ,"), []string{"vip", "internal"}},
		{"首尾逗号", strPtr(",vip,"), []string{"vip"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			channel := &Channel{GroupBlacklist: c.blacklist}
			got := channel.GetGroupBlacklist()
			if !reflect.DeepEqual(got, c.expected) {
				t.Errorf("GetGroupBlacklist() = %v, expected %v", got, c.expected)
			}
		})
	}
}

// TestIsUserGroupBlacklisted 验证用户分组黑名单命中判定。
func TestIsUserGroupBlacklisted(t *testing.T) {
	cases := []struct {
		name      string
		blacklist *string
		userGroup string
		expected  bool
	}{
		{"nil 黑名单不命中", nil, "vip", false},
		{"空黑名单不命中", strPtr(""), "vip", false},
		{"空用户分组不命中", strPtr("vip"), "", false},
		{"命中", strPtr("vip,internal"), "vip", true},
		{"命中第二个", strPtr("vip,internal"), "internal", true},
		{"未命中", strPtr("vip,internal"), "default", false},
		{"含空格仍精确命中", strPtr(" vip , internal "), "vip", true},
		{"大小写敏感不命中", strPtr("VIP"), "vip", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			channel := &Channel{GroupBlacklist: c.blacklist}
			if got := channel.IsUserGroupBlacklisted(c.userGroup); got != c.expected {
				t.Errorf("IsUserGroupBlacklisted(%q) = %v, expected %v", c.userGroup, got, c.expected)
			}
		})
	}
}
