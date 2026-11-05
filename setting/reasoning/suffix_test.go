package reasoning

import (
	"testing"
)

func TestParseDeepSeekThinkingSuffix(t *testing.T) {
	tests := []struct {
		name         string
		modelName    string
		wantBase     string
		wantThinking string
		wantEffort   string
		wantOk       bool
	}{
		{
			name:       "deepseek-flash without suffix",
			modelName:  "deepseek-flash",
			wantBase:   "deepseek-flash",
			wantOk:     false,
		},
		{
			name:         "deepseek-flash-none disables thinking",
			modelName:    "deepseek-flash-none",
			wantBase:     "deepseek-flash",
			wantThinking: "disabled",
			wantOk:       true,
		},
		{
			name:         "deepseek-flash-max enables thinking",
			modelName:    "deepseek-flash-max",
			wantBase:     "deepseek-flash",
			wantThinking: "enabled",
			wantEffort:   "max",
			wantOk:       true,
		},
		{
			name:         "deepseek-v4-flash-none still parses",
			modelName:    "deepseek-v4-flash-none",
			wantBase:     "deepseek-v4-flash",
			wantThinking: "disabled",
			wantOk:       true,
		},
		{
			name:         "deepseek-v4-pro-max still parses",
			modelName:    "deepseek-v4-pro-max",
			wantBase:     "deepseek-v4-pro",
			wantThinking: "enabled",
			wantEffort:   "max",
			wantOk:       true,
		},
		{
			name:      "deepseek-v4-flash without suffix",
			modelName: "deepseek-v4-flash",
			wantBase:  "deepseek-v4-flash",
			wantOk:    false,
		},
		{
			name:      "non deepseek model with suffix should not parse",
			modelName: "gpt-4-none",
			wantBase:  "gpt-4-none",
			wantOk:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseModel, thinkingType, effort, ok := ParseDeepSeekThinkingSuffix(tt.modelName)
			if baseModel != tt.wantBase || thinkingType != tt.wantThinking || effort != tt.wantEffort || ok != tt.wantOk {
				t.Errorf("ParseDeepSeekThinkingSuffix(%q) = (%q, %q, %q, %v), want (%q, %q, %q, %v)",
					tt.modelName, baseModel, thinkingType, effort, ok, tt.wantBase, tt.wantThinking, tt.wantEffort, tt.wantOk)
			}
		})
	}
}
