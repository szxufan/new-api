package service

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/dto"
)

func TestParseStoreField(t *testing.T) {
	tests := []struct {
		name     string
		storeRaw json.RawMessage
		expected bool
	}{
		{
			name:     "empty store field",
			storeRaw: nil,
			expected: false,
		},
		{
			name:     "store is true",
			storeRaw: json.RawMessage(`true`),
			expected: true,
		},
		{
			name:     "store is false",
			storeRaw: json.RawMessage(`false`),
			expected: false,
		},
		{
			name:     "store is invalid json",
			storeRaw: json.RawMessage(`"invalid"`),
			expected: false,
		},
		{
			name:     "store is number",
			storeRaw: json.RawMessage(`1`),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseStoreField(tt.storeRaw)
			if result != tt.expected {
				t.Errorf("ParseStoreField(%v) = %v, expected %v", tt.storeRaw, result, tt.expected)
			}
		})
	}
}

func TestStoreAndLookupResponsesContext(t *testing.T) {
	// This test verifies the store and lookup functions work correctly
	// Note: The actual cache uses Redis/Memory, so we just verify the logic works

	tests := []struct {
		name      string
		responseID string
		entry     *dto.ResponsesContextEntry
	}{
		{
			name:      "valid entry",
			responseID: "resp_test123",
			entry: &dto.ResponsesContextEntry{
				Model:  "gpt-4",
				Output: []dto.ResponsesOutput{},
			},
		},
		{
			name:      "empty responseID",
			responseID: "",
			entry: &dto.ResponsesContextEntry{
				Model:  "gpt-4",
			},
		},
		{
			name:      "nil entry",
			responseID: "resp_test123",
			entry: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Store should not panic
			StoreResponsesContext(tt.responseID, tt.entry)

			// Lookup empty responseID should return false
			if tt.responseID == "" {
				_, found := LookupResponsesContext("")
				if found {
					t.Error("LookupResponsesContext('') should return false")
				}
			}
		})
	}
}