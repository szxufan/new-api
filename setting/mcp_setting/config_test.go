package mcp_setting

import (
	"testing"
)

// TestGetGroupImageModel 测试按分组获取模型，包括命中、回退到 default、均未配置
func TestGetGroupImageModel(t *testing.T) {
	// 保存原始状态
	original := groupImageModelSetting.GroupImageModels
	defer func() {
		groupImageModelSetting.GroupImageModels = original
	}()

	// 设置测试数据
	groupImageModelSetting.GroupImageModels = map[string]string{
		"default": "dall-e-3",
		"vip":     "flux-pro-1.1",
	}

	tests := []struct {
		name      string
		group     string
		wantModel string
	}{
		{
			name:      "命中已配置分组",
			group:     "vip",
			wantModel: "flux-pro-1.1",
		},
		{
			name:      "命中 default 分组",
			group:     "default",
			wantModel: "dall-e-3",
		},
		{
			name:      "未配置分组回退到 default",
			group:     "unknown_group",
			wantModel: "dall-e-3",
		},
		{
			name:      "空分组回退到 default",
			group:     "",
			wantModel: "dall-e-3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetGroupImageModel(tt.group)
			if got != tt.wantModel {
				t.Errorf("GetGroupImageModel(%q) = %q, want %q", tt.group, got, tt.wantModel)
			}
		})
	}
}

// TestGetGroupImageModel_EmptyConfig 测试空配置情况
func TestGetGroupImageModel_EmptyConfig(t *testing.T) {
	original := groupImageModelSetting.GroupImageModels
	defer func() {
		groupImageModelSetting.GroupImageModels = original
	}()

	// 设置空配置
	groupImageModelSetting.GroupImageModels = map[string]string{}

	// 没有任何配置时应返回空字符串
	got := GetGroupImageModel("default")
	if got != "" {
		t.Errorf("GetGroupImageModel with empty config = %q, want empty string", got)
	}
}

// TestGetGroupImageModel_EmptyValue 测试分组值为空字符串时回退到 default
func TestGetGroupImageModel_EmptyValue(t *testing.T) {
	original := groupImageModelSetting.GroupImageModels
	defer func() {
		groupImageModelSetting.GroupImageModels = original
	}()

	groupImageModelSetting.GroupImageModels = map[string]string{
		"default":  "dall-e-3",
		"svip":     "", // svip 分组值为空
	}

	// svip 分组值为空应回退到 default
	got := GetGroupImageModel("svip")
	if got != "dall-e-3" {
		t.Errorf("GetGroupImageModel(\"svip\") with empty value = %q, want %q", got, "dall-e-3")
	}
}

// TestGroupImageModelSetting_DefaultValues 测试默认值
func TestGroupImageModelSetting_DefaultValues(t *testing.T) {
	setting := GetGroupImageModelSetting()
	if setting == nil {
		t.Fatal("GetGroupImageModelSetting() returned nil")
	}
	// 验证 default 分组有默认值
	model, ok := setting.GroupImageModels["default"]
	if !ok {
		t.Fatal("default group not found in GroupImageModels")
	}
	if model == "" {
		t.Error("default group model should not be empty")
	}
}

// TestGroupImageModelSetting_LoadFromDB 测试从 map 加载配置（模拟 LoadFromDB 行为）
func TestGroupImageModelSetting_LoadFromDB(t *testing.T) {
	// 直接修改配置以模拟从 DB 加载
	original := groupImageModelSetting.GroupImageModels
	defer func() {
		groupImageModelSetting.GroupImageModels = original
	}()

	// 模拟从 DB 加载新配置
	newConfig := map[string]string{
		"default": "gpt-image-1",
		"vip":     "flux-pro-1.1-ultra",
		"svip":    "dall-e-3",
	}
	groupImageModelSetting.GroupImageModels = newConfig

	// 验证配置已加载
	for group, expectedModel := range newConfig {
		got := GetGroupImageModel(group)
		if got != expectedModel {
			t.Errorf("GetGroupImageModel(%q) = %q, want %q", group, got, expectedModel)
		}
	}
}
