package service

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
)

func TestCheckResponseDetection(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		detection *dto.ResponseDetection
		wantHit   bool
		wantWords []string
	}{
		{
			name:      "nil detection",
			text:      "I cannot help with that",
			detection: nil,
			wantHit:   false,
			wantWords: nil,
		},
		{
			name: "disabled detection",
			text: "I cannot help with that",
			detection: &dto.ResponseDetection{
				Enabled:  false,
				Keywords: []string{"cannot"},
			},
			wantHit:   false,
			wantWords: nil,
		},
		{
			name: "empty keywords",
			text: "I cannot help with that",
			detection: &dto.ResponseDetection{
				Enabled:  true,
				Keywords: []string{},
			},
			wantHit:   false,
			wantWords: nil,
		},
		{
			name: "empty text",
			text: "",
			detection: &dto.ResponseDetection{
				Enabled:  true,
				Keywords: []string{"cannot"},
			},
			wantHit:   false,
			wantWords: nil,
		},
		{
			name: "single keyword hit",
			text: "I cannot help with that",
			detection: &dto.ResponseDetection{
				Enabled:  true,
				Keywords: []string{"cannot"},
			},
			wantHit:   true,
			wantWords: []string{"cannot"},
		},
		{
			name: "case insensitive hit",
			text: "I CANNOT help with that",
			detection: &dto.ResponseDetection{
				Enabled:  true,
				Keywords: []string{"cannot"},
			},
			wantHit:   true,
			wantWords: []string{"cannot"},
		},
		{
			name: "case insensitive hit - keyword uppercase",
			text: "I cannot help with that",
			detection: &dto.ResponseDetection{
				Enabled:  true,
				Keywords: []string{"CANNOT"},
			},
			wantHit:   true,
			wantWords: []string{"CANNOT"},
		},
		{
			name: "multiple keywords - one hit",
			text: "I cannot help with that",
			detection: &dto.ResponseDetection{
				Enabled:  true,
				Keywords: []string{"cannot", "As an AI", "抱歉我不能"},
			},
			wantHit:   true,
			wantWords: []string{"cannot"},
		},
		{
			name: "multiple keywords - multiple hits",
			text: "As an AI, I cannot help with that",
			detection: &dto.ResponseDetection{
				Enabled:  true,
				Keywords: []string{"cannot", "As an AI"},
			},
			wantHit:   true,
			wantWords: []string{"cannot", "As an AI"},
		},
		{
			name: "no hit",
			text: "The answer is 42",
			detection: &dto.ResponseDetection{
				Enabled:  true,
				Keywords: []string{"cannot", "As an AI"},
			},
			wantHit:   false,
			wantWords: nil,
		},
		{
			name: "chinese keyword hit",
			text: "抱歉我不能回答这个问题",
			detection: &dto.ResponseDetection{
				Enabled:  true,
				Keywords: []string{"抱歉我不能"},
			},
			wantHit:   true,
			wantWords: []string{"抱歉我不能"},
		},
		{
			name: "partial match - substring",
			text: "I cannot",
			detection: &dto.ResponseDetection{
				Enabled:  true,
				Keywords: []string{"cannot"},
			},
			wantHit:   true,
			wantWords: []string{"cannot"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// CheckResponseDetection 是兼容包装（hasToolCalls=false），不感知空回复命中
			hit, words := CheckResponseDetection(tt.text, tt.detection)
			if hit != tt.wantHit {
				t.Errorf("CheckResponseDetection() hit = %v, want %v", hit, tt.wantHit)
			}
			if tt.wantHit {
				if len(words) != len(tt.wantWords) {
					t.Errorf("CheckResponseDetection() words = %v, want %v", words, tt.wantWords)
					return
				}
				for i, w := range words {
					if w != tt.wantWords[i] {
						t.Errorf("CheckResponseDetection() words[%d] = %v, want %v", i, w, tt.wantWords[i])
					}
				}
			} else if words != nil {
				t.Errorf("CheckResponseDetection() words = %v, want nil", words)
			}
		})
	}
}

// TestCheckResponseDetectionWithEmpty 覆盖空回复命中（TreatEmptyAsHit）场景
func TestCheckResponseDetectionWithEmpty(t *testing.T) {
	tests := []struct {
		name         string
		text         string
		hasToolCalls bool
		detection    *dto.ResponseDetection
		wantHit      bool
		wantWords    []string
	}{
		{
			name:         "TreatEmptyAsHit + empty text + no tool calls → hit (nil keywords)",
			text:         "",
			hasToolCalls: false,
			detection: &dto.ResponseDetection{
				Enabled:         true,
				TreatEmptyAsHit: true,
			},
			wantHit:   true,
			wantWords: nil,
		},
		{
			name:         "TreatEmptyAsHit + whitespace-only text + no tool calls → hit",
			text:         "   \n\t  ",
			hasToolCalls: false,
			detection: &dto.ResponseDetection{
				Enabled:         true,
				TreatEmptyAsHit: true,
			},
			wantHit:   true,
			wantWords: nil,
		},
		{
			name:         "TreatEmptyAsHit + empty text + has tool calls → no hit (tool calls is valid response)",
			text:         "",
			hasToolCalls: true,
			detection: &dto.ResponseDetection{
				Enabled:         true,
				TreatEmptyAsHit: true,
			},
			wantHit:   false,
			wantWords: nil,
		},
		{
			name:         "TreatEmptyAsHit + non-empty text + no keywords → no hit (keyword detection unchanged)",
			text:         "The answer is 42",
			hasToolCalls: false,
			detection: &dto.ResponseDetection{
				Enabled:         true,
				TreatEmptyAsHit: true,
			},
			wantHit:   false,
			wantWords: nil,
		},
		{
			name:         "TreatEmptyAsHit=false (default) + empty text → no hit (backward compatibility)",
			text:         "",
			hasToolCalls: false,
			detection: &dto.ResponseDetection{
				Enabled:         true,
				TreatEmptyAsHit: false,
			},
			wantHit:   false,
			wantWords: nil,
		},
		{
			name:         "TreatEmptyAsHit + non-empty text + keyword hit → hit keywords (non-nil)",
			text:         "I cannot help",
			hasToolCalls: false,
			detection: &dto.ResponseDetection{
				Enabled:         true,
				Keywords:        []string{"cannot"},
				TreatEmptyAsHit: true,
			},
			wantHit:   true,
			wantWords: []string{"cannot"},
		},
		{
			name:         "TreatEmptyAsHit + empty text + keyword present → hit empty (nil) takes precedence",
			text:         "",
			hasToolCalls: false,
			detection: &dto.ResponseDetection{
				Enabled:         true,
				Keywords:        []string{"cannot"},
				TreatEmptyAsHit: true,
			},
			wantHit:   true,
			wantWords: nil,
		},
		{
			name:         "TreatEmptyAsHit + non-empty text + keyword present + has tool calls → keyword detection still runs",
			text:         "I cannot help",
			hasToolCalls: true,
			detection: &dto.ResponseDetection{
				Enabled:         true,
				Keywords:        []string{"cannot"},
				TreatEmptyAsHit: true,
			},
			wantHit:   true,
			wantWords: []string{"cannot"},
		},
		{
			name:         "disabled + TreatEmptyAsHit + empty text → no hit (Enabled=false short-circuits)",
			text:         "",
			hasToolCalls: false,
			detection: &dto.ResponseDetection{
				Enabled:         false,
				TreatEmptyAsHit: true,
			},
			wantHit:   false,
			wantWords: nil,
		},
		{
			name:         "nil detection → no hit",
			text:         "",
			hasToolCalls: false,
			detection:    nil,
			wantHit:      false,
			wantWords:    nil,
		},
		{
			name:         "TreatEmptyAsHit + whitespace text + has tool calls → no hit",
			text:         "   ",
			hasToolCalls: true,
			detection: &dto.ResponseDetection{
				Enabled:         true,
				TreatEmptyAsHit: true,
			},
			wantHit:   false,
			wantWords: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hit, words := CheckResponseDetectionWithEmpty(tt.text, tt.hasToolCalls, tt.detection)
			if hit != tt.wantHit {
				t.Errorf("CheckResponseDetectionWithEmpty() hit = %v, want %v", hit, tt.wantHit)
			}
			if tt.wantHit {
				if tt.wantWords == nil {
					// 空回复命中场景：keywords 应为 nil
					if words != nil {
						t.Errorf("CheckResponseDetectionWithEmpty() words = %v, want nil", words)
					}
				} else {
					if len(words) != len(tt.wantWords) {
						t.Errorf("CheckResponseDetectionWithEmpty() words = %v, want %v", words, tt.wantWords)
						return
					}
					for i, w := range words {
						if w != tt.wantWords[i] {
							t.Errorf("CheckResponseDetectionWithEmpty() words[%d] = %v, want %v", i, w, tt.wantWords[i])
						}
					}
				}
			} else if words != nil {
				t.Errorf("CheckResponseDetectionWithEmpty() words = %v, want nil", words)
			}
		})
	}
}
