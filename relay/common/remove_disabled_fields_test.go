package common

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRemoveDisabledFields_PreservesRawMessage verifies that RemoveDisabledFields
// does not corrupt json.RawMessage fields when deleting other top-level keys.
// This was the root cause of "prefill failed: unexpected content after document" errors
// from Claude API when the old map[string]interface{} round-trip corrupted OutputConfig.
func TestRemoveDisabledFields_PreservesRawMessage(t *testing.T) {
	// Simulate a Claude request with json.RawMessage fields (OutputConfig, Metadata, etc.)
	// and a service_tier field that should be removed.
	input := map[string]any{
		"model":    "claude-opus-4-6-low",
		"messages": []any{map[string]any{"role": "user", "content": "hello"}},
		"output_config": map[string]any{
			"effort": "low",
		},
		"metadata": map[string]any{
			"user_id": "test-user",
		},
		"service_tier": "auto",
		"max_tokens":   4096,
	}

	inputJSON, err := json.Marshal(input)
	require.NoError(t, err)

	// settings that will cause service_tier to be removed
	settings := dto.ChannelOtherSettings{
		AllowServiceTier: false, // should remove service_tier
	}

	result, err := RemoveDisabledFields(inputJSON, settings, false)
	require.NoError(t, err)

	// Verify service_tier was removed
	var resultMap map[string]any
	err = json.Unmarshal(result, &resultMap)
	require.NoError(t, err)

	_, hasServiceTier := resultMap["service_tier"]
	assert.False(t, hasServiceTier, "service_tier should have been removed")

	// Verify output_config is preserved correctly (not corrupted)
	outputConfig, ok := resultMap["output_config"].(map[string]any)
	require.True(t, ok, "output_config should be a map")
	assert.Equal(t, "low", outputConfig["effort"], "output_config.effort should be preserved")

	// Verify metadata is preserved correctly
	metadata, ok := resultMap["metadata"].(map[string]any)
	require.True(t, ok, "metadata should be a map")
	assert.Equal(t, "test-user", metadata["user_id"], "metadata.user_id should be preserved")

	// Verify other fields are untouched
	assert.Equal(t, "claude-opus-4-6-low", resultMap["model"])
	assert.Equal(t, float64(4096), resultMap["max_tokens"])
}

// TestRemoveDisabledFields_PreservesComplexRawMessage tests that deeply nested
// json.RawMessage-like content (as found in ClaudeMediaMessage.Content with type `any`)
// is not corrupted by the field removal process.
func TestRemoveDisabledFields_PreservesComplexRawMessage(t *testing.T) {
	// Simulate a Claude request with complex nested content
	// that includes tool_use blocks with arbitrary input objects
	input := map[string]any{
		"model": "claude-sonnet-4-20250514",
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "text", "text": "What's the weather?"},
				},
			},
			map[string]any{
				"role": "assistant",
				"content": []any{
					map[string]any{
						"type":  "tool_use",
						"id":    "toolu_123",
						"name":  "get_weather",
						"input": map[string]any{"location": "San Francisco", "unit": "celsius"},
					},
				},
			},
		},
		"thinking": map[string]any{
			"type":         "enabled",
			"budget_tokens": 2048,
		},
		"output_config": map[string]any{
			"effort": "high",
		},
		"service_tier": "default",
		"speed":        "fast",
	}

	inputJSON, err := json.Marshal(input)
	require.NoError(t, err)

	settings := dto.ChannelOtherSettings{
		AllowServiceTier:  false, // removes service_tier
		AllowSpeed:        false, // removes speed
		AllowInferenceGeo: true,  // keeps inference_geo (if present)
	}

	result, err := RemoveDisabledFields(inputJSON, settings, false)
	require.NoError(t, err)

	var resultMap map[string]any
	err = json.Unmarshal(result, &resultMap)
	require.NoError(t, err)

	// Verify removed fields are gone
	_, hasServiceTier := resultMap["service_tier"]
	assert.False(t, hasServiceTier, "service_tier should be removed")
	_, hasSpeed := resultMap["speed"]
	assert.False(t, hasSpeed, "speed should be removed")

	// Verify complex nested content is preserved correctly
	messages, ok := resultMap["messages"].([]any)
	require.True(t, ok)
	require.Len(t, messages, 2)

	// Check assistant message with tool_use
	assistantMsg, ok := messages[1].(map[string]any)
	require.True(t, ok)
	content, ok := assistantMsg["content"].([]any)
	require.True(t, ok)
	require.Len(t, content, 1)
	toolUse, ok := content[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "tool_use", toolUse["type"])
	assert.Equal(t, "toolu_123", toolUse["id"])
	assert.Equal(t, "get_weather", toolUse["name"])

	// Verify tool input is preserved as a proper object
	toolInput, ok := toolUse["input"].(map[string]any)
	require.True(t, ok, "tool input should be a map")
	assert.Equal(t, "San Francisco", toolInput["location"])
	assert.Equal(t, "celsius", toolInput["unit"])

	// Verify output_config is preserved
	outputConfig, ok := resultMap["output_config"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "high", outputConfig["effort"])

	// Verify thinking is preserved
	thinking, ok := resultMap["thinking"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "enabled", thinking["type"])
}

// TestRemoveDisabledFields_StreamOptionsIncludeObfuscation tests the nested
// stream_options.include_obfuscation removal and cleanup of empty stream_options.
func TestRemoveDisabledFields_StreamOptionsIncludeObfuscation(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		settings       dto.ChannelOtherSettings
		expectSOExists bool
		expectSOValue  string
	}{
		{
			name: "removes include_obfuscation and cleans up empty stream_options",
			input: `{"model":"gpt-4","stream_options":{"include_obfuscation":true}}`,
			settings: dto.ChannelOtherSettings{
				AllowIncludeObfuscation: false,
			},
			expectSOExists: false,
		},
		{
			name: "removes include_obfuscation but keeps other stream_options",
			input: `{"model":"gpt-4","stream_options":{"include_usage":true,"include_obfuscation":true}}`,
			settings: dto.ChannelOtherSettings{
				AllowIncludeObfuscation: false,
			},
			expectSOExists: true,
			expectSOValue:  `{"include_usage":true}`,
		},
		{
			name: "keeps include_obfuscation when allowed",
			input: `{"model":"gpt-4","stream_options":{"include_obfuscation":true}}`,
			settings: dto.ChannelOtherSettings{
				AllowIncludeObfuscation: true,
			},
			expectSOExists: true,
			expectSOValue:  `{"include_obfuscation":true}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := RemoveDisabledFields([]byte(tt.input), tt.settings, false)
			require.NoError(t, err)

			var resultMap map[string]any
			err = json.Unmarshal(result, &resultMap)
			require.NoError(t, err)

			so, exists := resultMap["stream_options"]
			assert.Equal(t, tt.expectSOExists, exists, "stream_options existence mismatch")

			if tt.expectSOExists && tt.expectSOValue != "" {
				soJSON, err := json.Marshal(so)
				require.NoError(t, err)
				assert.JSONEq(t, tt.expectSOValue, string(soJSON))
			}
		})
	}
}

// TestRemoveDisabledFields_PassThrough skips all removal.
func TestRemoveDisabledFields_PassThrough(t *testing.T) {
	input := `{"model":"gpt-4","service_tier":"auto","speed":"fast"}`
	result, err := RemoveDisabledFields([]byte(input), dto.ChannelOtherSettings{}, true)
	require.NoError(t, err)
	assert.JSONEq(t, input, string(result), "passthrough should return original data unchanged")
}

// TestRemoveDisabledFields_NoFieldsToRemove returns original when no removable fields exist.
func TestRemoveDisabledFields_NoFieldsToRemove(t *testing.T) {
	input := `{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}`
	result, err := RemoveDisabledFields([]byte(input), dto.ChannelOtherSettings{}, false)
	require.NoError(t, err)
	assert.JSONEq(t, input, string(result))
}

// TestRemoveDisabledFields_MultipleFieldsToRemove removes several fields at once.
func TestRemoveDisabledFields_MultipleFieldsToRemove(t *testing.T) {
	input := `{"model":"gpt-4","service_tier":"auto","inference_geo":"us","speed":"fast","store":true,"safety_identifier":"abc123"}`
	settings := dto.ChannelOtherSettings{
		AllowServiceTier:      false,
		AllowInferenceGeo:     false,
		AllowSpeed:            false,
		DisableStore:          true,
		AllowSafetyIdentifier: false,
	}
	result, err := RemoveDisabledFields([]byte(input), settings, false)
	require.NoError(t, err)

	var resultMap map[string]any
	err = json.Unmarshal(result, &resultMap)
	require.NoError(t, err)

	assert.Equal(t, "gpt-4", resultMap["model"])
	_, hasST := resultMap["service_tier"]
	assert.False(t, hasST)
	_, hasIG := resultMap["inference_geo"]
	assert.False(t, hasIG)
	_, hasSp := resultMap["speed"]
	assert.False(t, hasSp)
	_, hasStore := resultMap["store"]
	assert.False(t, hasStore)
	_, hasSI := resultMap["safety_identifier"]
	assert.False(t, hasSI)
}

// TestRemoveDisabledFields_RawMessageByteLevelIntegrity verifies that the raw bytes
// of json.RawMessage fields are preserved exactly (no re-encoding, no float64 conversion,
// no key reordering, no whitespace changes).
func TestRemoveDisabledFields_RawMessageByteLevelIntegrity(t *testing.T) {
	// Claude request with OutputConfig as json.RawMessage
	input := `{"model":"claude-opus-4-6-low","output_config":{"effort":"low"},"thinking":{"type":"adaptive"},"service_tier":"auto"}`

	settings := dto.ChannelOtherSettings{
		AllowServiceTier: false,
	}

	result, err := RemoveDisabledFields([]byte(input), settings, false)
	require.NoError(t, err)

	var resultMap map[string]any
	err = json.Unmarshal(result, &resultMap)
	require.NoError(t, err)

	// service_tier should be gone
	_, hasST := resultMap["service_tier"]
	assert.False(t, hasST)

	// output_config should be exactly preserved
	oc, ok := resultMap["output_config"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "low", oc["effort"])

	// thinking should be exactly preserved
	th, ok := resultMap["thinking"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "adaptive", th["type"])
}

// TestRemoveGeminiDisabledFields_PreservesRawMessage verifies that
// RemoveGeminiDisabledFields does not corrupt other fields when removing functionResponse.id.
func TestRemoveGeminiDisabledFields_PreservesRawMessage(t *testing.T) {
	// This test requires model_setting to be initialized, so we test the
	// collectGeminiFunctionResponseIdPaths helper directly instead.
	input := `{
		"contents": [
			{
				"role": "user",
				"parts": [{"text": "hello"}]
			},
			{
				"role": "model",
				"parts": [
					{
						"functionResponse": {
							"name": "get_weather",
							"id": "call_123",
							"response": {"temperature": 72}
						}
					}
				]
			}
		],
		"tools": [{"name": "get_weather"}]
	}`

	paths := collectGeminiFunctionResponseIdPaths([]byte(input))
	require.Len(t, paths, 1)
	assert.Equal(t, "contents.1.parts.0.functionResponse.id", paths[0])
}

// TestCollectGeminiFunctionResponseIdPaths_SnakeCase tests snake_case variant.
func TestCollectGeminiFunctionResponseIdPaths_SnakeCase(t *testing.T) {
	input := `{
		"contents": [
			{
				"role": "model",
				"parts": [
					{
						"function_response": {
							"name": "search",
							"id": "call_456",
							"response": {"results": []}
						}
					}
				]
			}
		]
	}`

	paths := collectGeminiFunctionResponseIdPaths([]byte(input))
	require.Len(t, paths, 1)
	assert.Equal(t, "contents.0.parts.0.function_response.id", paths[0])
}

// TestCollectGeminiFunctionResponseIdPaths_Multiple tests multiple function responses.
func TestCollectGeminiFunctionResponseIdPaths_Multiple(t *testing.T) {
	input := `{
		"contents": [
			{
				"role": "model",
				"parts": [
					{
						"functionResponse": {
							"name": "fn1",
							"id": "call_1",
							"response": {}
						}
					},
					{
						"functionResponse": {
							"name": "fn2",
							"id": "call_2",
							"response": {}
						}
					}
				]
			}
		]
	}`

	paths := collectGeminiFunctionResponseIdPaths([]byte(input))
	require.Len(t, paths, 2)
	assert.Equal(t, "contents.0.parts.0.functionResponse.id", paths[0])
	assert.Equal(t, "contents.0.parts.1.functionResponse.id", paths[1])
}

// TestCollectGeminiFunctionResponseIdPaths_NoContents tests with no contents array.
func TestCollectGeminiFunctionResponseIdPaths_NoContents(t *testing.T) {
	input := `{"model":"gemini-pro"}`
	paths := collectGeminiFunctionResponseIdPaths([]byte(input))
	assert.Nil(t, paths)
}
