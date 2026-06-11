package openaicompat

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

func TestOutputsToInputItems(t *testing.T) {
	tests := []struct {
		name     string
		outputs  []dto.ResponsesOutput
		expected int
	}{
		{
			name:     "empty outputs",
			outputs:  []dto.ResponsesOutput{},
			expected: 0,
		},
		{
			name: "message output",
			outputs: []dto.ResponsesOutput{
				{
					Type: "message",
					Role: "assistant",
					Content: []dto.ResponsesOutputContent{
						{Type: "output_text", Text: "Hello"},
					},
				},
			},
			expected: 1,
		},
		{
			name: "function_call output",
			outputs: []dto.ResponsesOutput{
				{
					Type:      "function_call",
					CallId:     "call_123",
					Name:       "test_func",
					Arguments:  json.RawMessage(`{"arg": "value"}`),
				},
			},
			expected: 1,
		},
		{
			name: "function_call_output",
			outputs: []dto.ResponsesOutput{
				{
					Type:    "function_call_output",
					CallId:   "call_123",
					Content: []dto.ResponsesOutputContent{
						{Type: "text", Text: "result"},
					},
				},
			},
			expected: 1,
		},
		{
			name: "mixed outputs",
			outputs: []dto.ResponsesOutput{
				{
					Type: "message",
					Role: "assistant",
					Content: []dto.ResponsesOutputContent{
						{Type: "output_text", Text: "Hello"},
					},
				},
				{
					Type:      "function_call",
					CallId:     "call_123",
					Name:       "test_func",
					Arguments:  json.RawMessage(`{"arg": "value"}`),
				},
				{
					Type:    "function_call_output",
					CallId:   "call_123",
					Content: []dto.ResponsesOutputContent{
						{Type: "text", Text: "result"},
					},
				},
			},
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items := OutputsToInputItems(tt.outputs)
			if len(items) != tt.expected {
				t.Errorf("OutputsToInputItems() returned %d items, expected %d", len(items), tt.expected)
			}

			// Verify message type has correct structure
			for i, item := range items {
				if tt.outputs[i].Type == "message" {
					if item["type"] != "message" {
						t.Errorf("message item should have type='message'")
					}
					if item["role"] != tt.outputs[i].Role {
						t.Errorf("message item role mismatch")
					}
				}
				if tt.outputs[i].Type == "function_call" {
					if item["type"] != "function_call" {
						t.Errorf("function_call item should have type='function_call'")
					}
					if item["call_id"] != tt.outputs[i].CallId {
						t.Errorf("function_call item call_id mismatch")
					}
				}
			}
		})
	}
}

func TestOutputsToMessages(t *testing.T) {
	tests := []struct {
		name     string
		outputs  []dto.ResponsesOutput
		expected int
	}{
		{
			name:     "empty outputs",
			outputs:  []dto.ResponsesOutput{},
			expected: 0,
		},
		{
			name: "message output",
			outputs: []dto.ResponsesOutput{
				{
					Type: "message",
					Role: "assistant",
					Content: []dto.ResponsesOutputContent{
						{Type: "output_text", Text: "Hello"},
					},
				},
			},
			expected: 1,
		},
		{
			name: "function_call output",
			outputs: []dto.ResponsesOutput{
				{
					Type:      "function_call",
					CallId:     "call_123",
					Name:       "test_func",
					Arguments:  json.RawMessage(`{"arg": "value"}`),
				},
			},
			expected: 1,
		},
		{
			name: "function_call_output",
			outputs: []dto.ResponsesOutput{
				{
					Type:    "function_call_output",
					CallId:   "call_123",
					Content: []dto.ResponsesOutputContent{
						{Type: "text", Text: "result"},
					},
				},
			},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			messages := OutputsToMessages(tt.outputs)
			if len(messages) != tt.expected {
				t.Errorf("OutputsToMessages() returned %d messages, expected %d", len(messages), tt.expected)
			}

			// Verify message roles
			for i, msg := range messages {
				if tt.outputs[i].Type == "message" {
					if msg.Role != tt.outputs[i].Role {
						t.Errorf("message role mismatch: got %s, expected %s", msg.Role, tt.outputs[i].Role)
					}
				}
				if tt.outputs[i].Type == "function_call" {
					if msg.Role != "assistant" {
						t.Errorf("function_call should convert to assistant message")
					}
				}
				if tt.outputs[i].Type == "function_call_output" {
					if msg.Role != "tool" {
						t.Errorf("function_call_output should convert to tool message")
					}
					if msg.ToolCallId != tt.outputs[i].CallId {
						t.Errorf("tool message ToolCallId mismatch")
					}
				}
			}
		})
	}
}

func TestRebuildResponsesInput(t *testing.T) {
	tests := []struct {
		name        string
		previous    *dto.ResponsesContextEntry
		current     json.RawMessage
		expectError bool
	}{
		{
			name: "merge with array input",
			previous: &dto.ResponsesContextEntry{
				Output: []dto.ResponsesOutput{
					{
						Type: "message",
						Role: "assistant",
						Content: []dto.ResponsesOutputContent{
							{Type: "output_text", Text: "Previous response"},
						},
					},
				},
			},
			current:     json.RawMessage(`[{"type": "message", "role": "user", "content": "New input"}]`),
			expectError: false,
		},
		{
			name: "merge with string input",
			previous: &dto.ResponsesContextEntry{
				Output: []dto.ResponsesOutput{
					{
						Type: "message",
						Role: "assistant",
						Content: []dto.ResponsesOutputContent{
							{Type: "output_text", Text: "Previous response"},
						},
					},
				},
			},
			current:     json.RawMessage(`"Simple user input"`),
			expectError: false,
		},
		{
			name: "empty current input",
			previous: &dto.ResponsesContextEntry{
				Output: []dto.ResponsesOutput{
					{
						Type: "message",
						Role: "assistant",
						Content: []dto.ResponsesOutputContent{
							{Type: "output_text", Text: "Previous response"},
						},
					},
				},
			},
			current:     nil,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := RebuildResponsesInput(tt.previous, tt.current)
			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !tt.expectError {
				// Verify result is valid JSON
				var items []map[string]any
				if err := common.Unmarshal(result, &items); err != nil {
					t.Errorf("result is not valid JSON array: %v", err)
				}

				// Should have at least previous items
				if len(items) < len(tt.previous.Output) {
					t.Errorf("result should contain previous items")
				}
			}
		})
	}
}