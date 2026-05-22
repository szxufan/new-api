package dto

import (
	"testing"
)

func TestExtractThinkTag(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		wantReasoning string
		wantRemaining string
	}{
		{
			name:          "no think tag",
			content:       "Hello world",
			wantReasoning: "",
			wantRemaining: "Hello world",
		},
		{
			name:          "think tag with content after",
			content:       "<think>\nLet me think\nStep 1\n</think>\nThe answer is 42",
			wantReasoning: "\nLet me think\nStep 1\n",
			wantRemaining: "The answer is 42",
		},
		{
			name:          "think tag only",
			content:       "<think>reasoning text</think>",
			wantReasoning: "reasoning text",
			wantRemaining: "",
		},
		{
			name:          "content before and after think tag",
			content:       "Before<think>reasoning</think>After",
			wantReasoning: "reasoning",
			wantRemaining: "Before\nAfter",
		},
		{
			name:          "unclosed think tag",
			content:       "<think>reasoning without close",
			wantReasoning: "",
			wantRemaining: "<think>reasoning without close",
		},
		{
			name:          "close before open",
			content:       "</think><think>reasoning</think>",
			wantReasoning: "",
			wantRemaining: "</think><think>reasoning</think>",
		},
		{
			name:          "empty think tag",
			content:       "<think></think>content",
			wantReasoning: "",
			wantRemaining: "content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reasoning, remaining := extractThinkTag(tt.content)
			if reasoning != tt.wantReasoning {
				t.Errorf("extractThinkTag(%q) reasoning = %q, want %q", tt.content, reasoning, tt.wantReasoning)
			}
			if remaining != tt.wantRemaining {
				t.Errorf("extractThinkTag(%q) remaining = %q, want %q", tt.content, remaining, tt.wantRemaining)
			}
		})
	}
}

func TestMessage_ExtractThinkTagToReasoningContent(t *testing.T) {
	t.Run("already has reasoning_content", func(t *testing.T) {
		existing := "existing"
		msg := Message{
			Role:             "assistant",
			Content:          "<think>new reasoning</think>content",
			ReasoningContent: &existing,
		}
		changed := msg.ExtractThinkTagToReasoningContent()
		if changed {
			t.Error("should not change when ReasoningContent already set")
		}
		if *msg.ReasoningContent != "existing" {
			t.Errorf("ReasoningContent = %q, want %q", *msg.ReasoningContent, "existing")
		}
	})

	t.Run("content is not string", func(t *testing.T) {
		msg := Message{
			Role:    "assistant",
			Content: []interface{}{map[string]string{"type": "text", "text": "hello"}},
		}
		changed := msg.ExtractThinkTagToReasoningContent()
		if changed {
			t.Error("should not change when Content is not a string")
		}
	})

	t.Run("extract from think tag", func(t *testing.T) {
		msg := Message{
			Role:    "assistant",
			Content: "<think>my reasoning</think>my answer",
		}
		changed := msg.ExtractThinkTagToReasoningContent()
		if !changed {
			t.Error("should return true when extraction succeeds")
		}
		if msg.ReasoningContent == nil || *msg.ReasoningContent != "my reasoning" {
			t.Errorf("ReasoningContent = %v, want %q", msg.ReasoningContent, "my reasoning")
		}
		if msg.Content != "my answer" {
			t.Errorf("Content = %q, want %q", msg.Content, "my answer")
		}
	})

	t.Run("no think tag in content", func(t *testing.T) {
		msg := Message{
			Role:    "assistant",
			Content: "just regular content",
		}
		changed := msg.ExtractThinkTagToReasoningContent()
		if changed {
			t.Error("should return false when no think tag found")
		}
		if msg.ReasoningContent != nil {
			t.Error("ReasoningContent should remain nil")
		}
	})
}
