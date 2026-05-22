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
