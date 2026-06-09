package reasoning

import (
	"testing"
)

func TestIsDeepSeekThinkingModel(t *testing.T) {
	tests := []struct {
		name      string
		modelName string
		want      bool
	}{
		{
			name:      "deepseek-reasoner",
			modelName: "deepseek-reasoner",
			want:      true,
		},
		{
			name:      "deepseek-reasoner with suffix",
			modelName: "deepseek-reasoner-max",
			want:      true,
		},
		{
			name:      "deepseek-chat",
			modelName: "deepseek-chat",
			want:      false,
		},
		{
			name:      "deepseek-v4-flash",
			modelName: "deepseek-v4-flash",
			want:      true,
		},
		{
			name:      "deepseek-v4-flash-max",
			modelName: "deepseek-v4-flash-max",
			want:      true,
		},
		{
			name:      "deepseek-v4-flash-none",
			modelName: "deepseek-v4-flash-none",
			want:      true,
		},
		{
			name:      "deepseek-v4-pro",
			modelName: "deepseek-v4-pro",
			want:      true,
		},
		{
			name:      "deepseek-v4-pro-max",
			modelName: "deepseek-v4-pro-max",
			want:      true,
		},
		{
			name:      "deepseek-v4-pro-none",
			modelName: "deepseek-v4-pro-none",
			want:      true,
		},
		{
			name:      "gpt-4",
			modelName: "gpt-4",
			want:      false,
		},
		{
			name:      "deepseek-coder",
			modelName: "deepseek-coder",
			want:      false,
		},
		{
			name:      "claude-3-opus",
			modelName: "claude-3-opus",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsDeepSeekThinkingModel(tt.modelName)
			if got != tt.want {
				t.Errorf("IsDeepSeekThinkingModel(%q) = %v, want %v", tt.modelName, got, tt.want)
			}
		})
	}
}

func TestIsMimoThinkingModel(t *testing.T) {
	tests := []struct {
		name      string
		modelName string
		want      bool
	}{
		{
			name:      "mimo-v2.5-pro",
			modelName: "mimo-v2.5-pro",
			want:      true,
		},
		{
			name:      "mimo-v2.5",
			modelName: "mimo-v2.5",
			want:      true,
		},
		{
			name:      "mimo-v2-pro",
			modelName: "mimo-v2-pro",
			want:      true,
		},
		{
			name:      "mimo-v2-omni",
			modelName: "mimo-v2-omni",
			want:      true,
		},
		{
			name:      "mimo-v2-flash",
			modelName: "mimo-v2-flash",
			want:      true,
		},
		{
			name:      "mimo-v2.5-pro with suffix should not match",
			modelName: "mimo-v2.5-pro-max",
			want:      false,
		},
		{
			name:      "gpt-4",
			modelName: "gpt-4",
			want:      false,
		},
		{
			name:      "deepseek-v4-flash",
			modelName: "deepseek-v4-flash",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsMimoThinkingModel(tt.modelName)
			if got != tt.want {
				t.Errorf("IsMimoThinkingModel(%q) = %v, want %v", tt.modelName, got, tt.want)
			}
		})
	}
}

func TestIsThinkingModel(t *testing.T) {
	tests := []struct {
		name      string
		modelName string
		want      bool
	}{
		{
			name:      "deepseek-reasoner",
			modelName: "deepseek-reasoner",
			want:      true,
		},
		{
			name:      "deepseek-v4-flash",
			modelName: "deepseek-v4-flash",
			want:      true,
		},
		{
			name:      "mimo-v2.5-pro",
			modelName: "mimo-v2.5-pro",
			want:      true,
		},
		{
			name:      "mimo-v2-flash",
			modelName: "mimo-v2-flash",
			want:      true,
		},
		{
			name:      "gpt-4 should not match",
			modelName: "gpt-4",
			want:      false,
		},
		{
			name:      "deepseek-chat should not match",
			modelName: "deepseek-chat",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsThinkingModel(tt.modelName)
			if got != tt.want {
				t.Errorf("IsThinkingModel(%q) = %v, want %v", tt.modelName, got, tt.want)
			}
		})
	}
}
