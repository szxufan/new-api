package config

import (
	"reflect"
	"testing"
)

type testConfigWithMap struct {
	Modes map[string]string `json:"modes"`
	Exprs map[string]string `json:"exprs"`
	Name  string            `json:"name"`
}

func TestUpdateConfigFromMap_MapReplacement(t *testing.T) {
	cfg := &testConfigWithMap{
		Modes: map[string]string{
			"model-a": "tiered_expr",
			"model-b": "tiered_expr",
		},
		Exprs: map[string]string{
			"model-a": "p * 5 + c * 25",
			"model-b": "p * 10 + c * 50",
		},
		Name: "billing",
	}

	// Simulate removing model-a: new value only has model-b
	err := UpdateConfigFromMap(cfg, map[string]string{
		"modes": `{"model-b": "tiered_expr"}`,
		"exprs": `{"model-b": "p * 10 + c * 50"}`,
	})
	if err != nil {
		t.Fatalf("UpdateConfigFromMap failed: %v", err)
	}

	if _, ok := cfg.Modes["model-a"]; ok {
		t.Errorf("Modes still contains model-a after it was removed from the update; got %v", cfg.Modes)
	}
	if _, ok := cfg.Exprs["model-a"]; ok {
		t.Errorf("Exprs still contains model-a after it was removed from the update; got %v", cfg.Exprs)
	}

	if cfg.Modes["model-b"] != "tiered_expr" {
		t.Errorf("Modes[model-b] = %q, want %q", cfg.Modes["model-b"], "tiered_expr")
	}
	if cfg.Exprs["model-b"] != "p * 10 + c * 50" {
		t.Errorf("Exprs[model-b] = %q, want %q", cfg.Exprs["model-b"], "p * 10 + c * 50")
	}
}

func TestUpdateConfigFromMap_EmptyMapClearsAll(t *testing.T) {
	cfg := &testConfigWithMap{
		Modes: map[string]string{
			"model-a": "tiered_expr",
		},
		Exprs: map[string]string{
			"model-a": "p * 5 + c * 25",
		},
	}

	err := UpdateConfigFromMap(cfg, map[string]string{
		"modes": `{}`,
		"exprs": `{}`,
	})
	if err != nil {
		t.Fatalf("UpdateConfigFromMap failed: %v", err)
	}

	if len(cfg.Modes) != 0 {
		t.Errorf("Modes should be empty after updating with {}, got %v", cfg.Modes)
	}
	if len(cfg.Exprs) != 0 {
		t.Errorf("Exprs should be empty after updating with {}, got %v", cfg.Exprs)
	}
}

func TestUpdateConfigFromMap_ScalarFieldsUnchanged(t *testing.T) {
	cfg := &testConfigWithMap{
		Modes: map[string]string{"m": "v"},
		Name:  "old",
	}

	err := UpdateConfigFromMap(cfg, map[string]string{
		"name": "new",
	})
	if err != nil {
		t.Fatalf("UpdateConfigFromMap failed: %v", err)
	}

	if cfg.Name != "new" {
		t.Errorf("Name = %q, want %q", cfg.Name, "new")
	}
	// modes was not in configMap, should remain unchanged
	if cfg.Modes["m"] != "v" {
		t.Errorf("Modes should be unchanged, got %v", cfg.Modes)
	}
}

type testConfigWithOmitEmptyMap struct {
	// 带 omitempty 的 map 字段：曾因 json tag 未截断 options 导致
	// LoadFromDB / UpdateOption 时以 "modes,omitempty" 匹配存储键而永远失配，
	// 配置静默失效（如 mcp_setting.group_video_*_models）。
	Modes map[string]string `json:"modes,omitempty"`
	Name  string            `json:"name,omitempty"`
}

func TestUpdateConfigFromMap_OmitEmptyTagKey(t *testing.T) {
	cfg := &testConfigWithOmitEmptyMap{
		Modes: map[string]string{"default": "dall-e-3"},
	}

	// 模拟数据库中保存的 option：key 为 "modes"（无 options 后缀）
	err := UpdateConfigFromMap(cfg, map[string]string{
		"modes": `{"monthly": "sora-2"}`,
	})
	if err != nil {
		t.Fatalf("UpdateConfigFromMap failed: %v", err)
	}

	if cfg.Modes["monthly"] != "sora-2" {
		t.Errorf("Modes[monthly] = %q, want %q; omitempty tag must not break key matching, got %v", cfg.Modes["monthly"], "sora-2", cfg.Modes)
	}
	// map 更新为整体替换：新 JSON 中不存在的旧 key 应被清除
	if _, ok := cfg.Modes["default"]; ok {
		t.Errorf("Modes[default] should be removed after update, got %v", cfg.Modes)
	}
}

func TestConfigToMap_OmitEmptyTagKey(t *testing.T) {
	cfg := &testConfigWithOmitEmptyMap{
		Modes: map[string]string{"default": "dall-e-3"},
		Name:  "x",
	}

	m, err := ConfigToMap(cfg)
	if err != nil {
		t.Fatalf("ConfigToMap failed: %v", err)
	}

	if _, ok := m["modes"]; !ok {
		t.Errorf("ConfigToMap should export key %q (without options suffix), got keys %v", "modes", m)
	}
	if m["modes"] != `{"default":"dall-e-3"}` {
		t.Errorf("ConfigToMap[modes] = %q, want %q", m["modes"], `{"default":"dall-e-3"}`)
	}
	if m["name"] != "x" {
		t.Errorf("ConfigToMap[name] = %q, want %q", m["name"], "x")
	}
}

func TestUpdateConfigFromMap_OmitEmptyRoundTrip(t *testing.T) {
	// 序列化 → 反序列化回路：带 omitempty 的字段经 ConfigToMap 存库后，
	// 必须能被 UpdateConfigFromMap 以相同 key 加载回来。
	cfg := &testConfigWithOmitEmptyMap{}
	exported, err := ConfigToMap(&testConfigWithOmitEmptyMap{
		Modes: map[string]string{"default": "m"},
	})
	if err != nil {
		t.Fatalf("ConfigToMap failed: %v", err)
	}
	if err := UpdateConfigFromMap(cfg, exported); err != nil {
		t.Fatalf("UpdateConfigFromMap failed: %v", err)
	}
	if cfg.Modes["default"] != "m" {
		t.Errorf("round trip lost data: Modes = %v", cfg.Modes)
	}
}

func TestJsonConfigKey(t *testing.T) {
	cases := []struct {
		field reflect.StructField
		want  string
	}{
		{mustField(testConfigWithMap{}, "Modes"), "modes"},
		{mustField(testConfigWithOmitEmptyMap{}, "Modes"), "modes"},
		{mustField(testConfigWithOmitEmptyMap{}, "Name"), "name"},
	}
	for _, c := range cases {
		if got := jsonConfigKey(c.field); got != c.want {
			t.Errorf("jsonConfigKey(%s) = %q, want %q", c.field.Name, got, c.want)
		}
	}

	// json:"-" 应跳过；无 tag / 仅 options 应回退字段名
	if got := jsonConfigKey(mustField(struct {
		Skip string `json:"-"`
	}{}, "Skip")); got != "" {
		t.Errorf("jsonConfigKey(json:\"-\") = %q, want empty", got)
	}
	if got := jsonConfigKey(mustField(struct {
		NoTag string
	}{}, "NoTag")); got != "NoTag" {
		t.Errorf("jsonConfigKey(no tag) = %q, want %q", got, "NoTag")
	}
	if got := jsonConfigKey(mustField(struct {
		OptsOnly string `json:",omitempty"`
	}{}, "OptsOnly")); got != "OptsOnly" {
		t.Errorf("jsonConfigKey(json:\",omitempty\") = %q, want %q", got, "OptsOnly")
	}
}

func mustField(v interface{}, name string) reflect.StructField {
	t := reflect.TypeOf(v)
	f, ok := t.FieldByName(name)
	if !ok {
		panic("field not found: " + name)
	}
	return f
}
