package dto

import (
	"encoding/json"
	"testing"
)

func TestClaudeMessage_HasThinkingContent(t *testing.T) {
	tests := []struct {
		name    string
		message ClaudeMessage
		want    bool
	}{
		{
			name: "string content has no thinking",
			message: ClaudeMessage{
				Role:    "assistant",
				Content: "hello",
			},
			want: false,
		},
		{
			name: "structured content with thinking block",
			message: ClaudeMessage{
				Role: "assistant",
				Content: []ClaudeMediaMessage{
					{Type: "thinking", Thinking: strPtr("reasoning here")},
					{Type: "text", Text: strPtr("response")},
				},
			},
			want: true,
		},
		{
			name: "structured content without thinking block",
			message: ClaudeMessage{
				Role: "assistant",
				Content: []ClaudeMediaMessage{
					{Type: "text", Text: strPtr("response")},
				},
			},
			want: false,
		},
		{
			name: "structured content with tool_use only",
			message: ClaudeMessage{
				Role: "assistant",
				Content: []ClaudeMediaMessage{
					{Type: "tool_use", Id: "tool_1", Name: "test_func"},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.message.HasThinkingContent()
			if got != tt.want {
				t.Errorf("HasThinkingContent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClaudeMessage_GetTextAndToolUseContent(t *testing.T) {
	tests := []struct {
		name           string
		message        ClaudeMessage
		wantText       string
		wantToolUseNil bool
	}{
		{
			name: "string content",
			message: ClaudeMessage{
				Role:    "assistant",
				Content: "hello world",
			},
			wantText:       "hello world",
			wantToolUseNil: true,
		},
		{
			name: "structured content with text and tool_use",
			message: ClaudeMessage{
				Role: "assistant",
				Content: []ClaudeMediaMessage{
					{Type: "text", Text: strPtr("response")},
					{Type: "tool_use", Id: "tool_1", Name: "test_func", Input: map[string]any{"key": "value"}},
				},
			},
			wantText:       "response",
			wantToolUseNil: false,
		},
		{
			name: "structured content with thinking and text",
			message: ClaudeMessage{
				Role: "assistant",
				Content: []ClaudeMediaMessage{
					{Type: "thinking", Thinking: strPtr("reasoning")},
					{Type: "text", Text: strPtr("response")},
				},
			},
			wantText:       "response",
			wantToolUseNil: true,
		},
		{
			name: "multiple text blocks",
			message: ClaudeMessage{
				Role: "assistant",
				Content: []ClaudeMediaMessage{
					{Type: "text", Text: strPtr("part1")},
					{Type: "text", Text: strPtr("part2")},
				},
			},
			wantText:       "part1part2",
			wantToolUseNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, toolUse := tt.message.GetTextAndToolUseContent()
			if text != tt.wantText {
				t.Errorf("GetTextAndToolUseContent() text = %v, want %v", text, tt.wantText)
			}
			if (toolUse == nil) != tt.wantToolUseNil {
				t.Errorf("GetTextAndToolUseContent() toolUse nil = %v, want %v", toolUse == nil, tt.wantToolUseNil)
			}
		})
	}
}

func TestClaudeMessage_PrependThinkingBlock(t *testing.T) {
	tests := []struct {
		name         string
		message      ClaudeMessage
		thinking     string
		wantThinking string
		wantText     string
	}{
		{
			name: "string content gets converted to structured",
			message: ClaudeMessage{
				Role:    "assistant",
				Content: "hello",
			},
			thinking:     "my reasoning",
			wantThinking: "my reasoning",
			wantText:     "hello",
		},
		{
			name: "structured content prepends thinking",
			message: ClaudeMessage{
				Role: "assistant",
				Content: []ClaudeMediaMessage{
					{Type: "text", Text: strPtr("response")},
					{Type: "tool_use", Id: "tool_1", Name: "test_func"},
				},
			},
			thinking:     "my reasoning",
			wantThinking: "my reasoning",
			wantText:     "response",
		},
		{
			name: "empty thinking",
			message: ClaudeMessage{
				Role:    "assistant",
				Content: "hello",
			},
			thinking:     "",
			wantThinking: "",
			wantText:     "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.message.PrependThinkingBlock(tt.thinking)
			contents, err := tt.message.ParseContent()
			if err != nil {
				t.Fatalf("ParseContent() error = %v", err)
			}
			if len(contents) == 0 {
				t.Fatal("ParseContent() returned empty contents")
			}
			first := contents[0]
			if first.Type != "thinking" {
				t.Fatalf("first block type = %v, want thinking", first.Type)
			}
			if first.Thinking == nil || *first.Thinking != tt.wantThinking {
				var got string
				if first.Thinking != nil {
					got = *first.Thinking
				}
				t.Errorf("thinking content = %v, want %v", got, tt.wantThinking)
			}
			// Find text block
			var textContent string
			for _, c := range contents {
				if c.Type == "text" && c.Text != nil {
					textContent = *c.Text
					break
				}
			}
			if textContent != tt.wantText {
				t.Errorf("text content = %v, want %v", textContent, tt.wantText)
			}
		})
	}
}

func TestClaudeMessage_PrependThinkingBlock_RoundTrip(t *testing.T) {
	// Test that prepend thinking block can be serialized and deserialized correctly
	msg := ClaudeMessage{
		Role:    "assistant",
		Content: "hello world",
	}
	msg.PrependThinkingBlock("my reasoning process")

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal error = %v", err)
	}

	var decoded ClaudeMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error = %v", err)
	}

	if decoded.Role != "assistant" {
		t.Errorf("Role = %v, want assistant", decoded.Role)
	}

	contents, err := decoded.ParseContent()
	if err != nil {
		t.Fatalf("ParseContent error = %v", err)
	}

	if len(contents) < 2 {
		t.Fatalf("expected at least 2 content blocks, got %d", len(contents))
	}

	if contents[0].Type != "thinking" {
		t.Errorf("first block type = %v, want thinking", contents[0].Type)
	}
	if contents[1].Type != "text" {
		t.Errorf("second block type = %v, want text", contents[1].Type)
	}
}

func strPtr(s string) *string {
	return &s
}
