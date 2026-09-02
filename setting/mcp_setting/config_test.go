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

// setI2IModels 设置图生图测试数据并返回还原函数
func setI2IModels(m map[string]string) func() {
	original := groupImageModelSetting.GroupI2IModels
	groupImageModelSetting.GroupI2IModels = m
	return func() { groupImageModelSetting.GroupI2IModels = original }
}

// TestGetGroupI2IModel 测试图生图模型获取：命中、default 回退、空配置
func TestGetGroupI2IModel(t *testing.T) {
	restore := setI2IModels(map[string]string{
		"default": "qwen-image-edit",
		"vip":     "gpt-image-1",
	})
	defer restore()

	if got := GetGroupI2IModel("vip"); got != "gpt-image-1" {
		t.Errorf("GetGroupI2IModel(\"vip\") = %q, want %q", got, "gpt-image-1")
	}
	if got := GetGroupI2IModel("unknown"); got != "qwen-image-edit" {
		t.Errorf("GetGroupI2IModel(\"unknown\") = %q, want default %q", got, "qwen-image-edit")
	}
}

// TestGetGroupI2IModel_Empty 测试图生图模型空配置返回空串
func TestGetGroupI2IModel_Empty(t *testing.T) {
	restore := setI2IModels(map[string]string{})
	defer restore()

	if got := GetGroupI2IModel("default"); got != "" {
		t.Errorf("GetGroupI2IModel with empty config = %q, want empty string", got)
	}
}

// setVideoModels 设置视频模型池测试数据并返回还原函数
func setVideoModels(field string, m map[string]string) func() {
	var original map[string]string
	switch field {
	case VideoModelKindT2V:
		original = groupImageModelSetting.GroupVideoT2VModels
		groupImageModelSetting.GroupVideoT2VModels = m
	case VideoModelKindI2V:
		original = groupImageModelSetting.GroupVideoI2VModels
		groupImageModelSetting.GroupVideoI2VModels = m
	case VideoModelKindKF2V:
		original = groupImageModelSetting.GroupVideoKF2VModels
		groupImageModelSetting.GroupVideoKF2VModels = m
	case VideoModelKindR2V:
		original = groupImageModelSetting.GroupVideoR2VModels
		groupImageModelSetting.GroupVideoR2VModels = m
	}
	return func() {
		switch field {
		case VideoModelKindT2V:
			groupImageModelSetting.GroupVideoT2VModels = original
		case VideoModelKindI2V:
			groupImageModelSetting.GroupVideoI2VModels = original
		case VideoModelKindKF2V:
			groupImageModelSetting.GroupVideoKF2VModels = original
		case VideoModelKindR2V:
			groupImageModelSetting.GroupVideoR2VModels = original
		}
	}
}

// TestGetGroupVideoModel_KindDispatch 测试视频模型池 kind 分发正确性
func TestGetGroupVideoModel_KindDispatch(t *testing.T) {
	restoreT2V := setVideoModels(VideoModelKindT2V, map[string]string{"default": "wan2.5-t2v-preview"})
	restoreI2V := setVideoModels(VideoModelKindI2V, map[string]string{"default": "wan2.5-i2v-preview"})
	restoreKF2V := setVideoModels(VideoModelKindKF2V, map[string]string{"default": "wan2.2-kf2v-flash"})
	restoreR2V := setVideoModels(VideoModelKindR2V, map[string]string{"default": "happyhorse-1.0-r2v"})
	defer restoreT2V()
	defer restoreI2V()
	defer restoreKF2V()
	defer restoreR2V()

	cases := []struct {
		kind string
		want string
	}{
		{VideoModelKindT2V, "wan2.5-t2v-preview"},
		{VideoModelKindI2V, "wan2.5-i2v-preview"},
		{VideoModelKindKF2V, "wan2.2-kf2v-flash"},
		{VideoModelKindR2V, "happyhorse-1.0-r2v"},
	}
	for _, tt := range cases {
		if got := GetGroupVideoModel(tt.kind, "default"); got != tt.want {
			t.Errorf("GetGroupVideoModel(%q, \"default\") = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

// TestGetGroupVideoModel_UnknownKind 未知 kind 返回空串
func TestGetGroupVideoModel_UnknownKind(t *testing.T) {
	if got := GetGroupVideoModel("nope", "default"); got != "" {
		t.Errorf("GetGroupVideoModel(\"nope\", ...) = %q, want empty string", got)
	}
}

// TestGetGroupVideoModel_Fallback 测试视频模型池分组回退：指定组 → default → 空串
func TestGetGroupVideoModel_Fallback(t *testing.T) {
	restore := setVideoModels(VideoModelKindT2V, map[string]string{
		"default": "wan2.5-t2v-preview",
		"vip":     "happyhorse-1.0-t2v",
	})
	defer restore()

	if got := GetGroupVideoModel(VideoModelKindT2V, "vip"); got != "happyhorse-1.0-t2v" {
		t.Errorf("GetGroupVideoModel t2v vip = %q", got)
	}
	if got := GetGroupVideoModel(VideoModelKindT2V, "unknown"); got != "wan2.5-t2v-preview" {
		t.Errorf("GetGroupVideoModel t2v unknown = %q, want default", got)
	}

	restoreEmpty := setVideoModels(VideoModelKindI2V, map[string]string{})
	defer restoreEmpty()
	if got := GetGroupVideoModel(VideoModelKindI2V, "vip"); got != "" {
		t.Errorf("GetGroupVideoModel i2v empty pool = %q, want empty string", got)
	}
}
