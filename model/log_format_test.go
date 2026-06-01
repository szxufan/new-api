package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
)

func TestFormatUserLogsRemovesAdminOnlyFields(t *testing.T) {
	other := map[string]interface{}{
		"model_ratio":        1.0,
		"group_ratio":        1.0,
		"model_price":        0.01,
		"admin_info":         map[string]interface{}{"use_channel": []string{"ch1"}},
		"stream_status":      "active",
		"is_model_mapped":    true,
		"upstream_model_name": "gpt-4o-upstream",
	}
	otherJson := common.MapToJsonStr(other)

	logs := []*Log{
		{
			Id:    1,
			Other: otherJson,
		},
	}

	formatUserLogs(logs, 0)

	var result map[string]interface{}
	err := common.UnmarshalJsonStr(logs[0].Other, &result)
	assert.NoError(t, err)

	assert.NotContains(t, result, "admin_info", "admin_info should be removed for non-admin users")
	assert.NotContains(t, result, "stream_status", "stream_status should be removed for non-admin users")
	assert.NotContains(t, result, "is_model_mapped", "is_model_mapped should be removed for non-admin users")
	assert.NotContains(t, result, "upstream_model_name", "upstream_model_name should be removed for non-admin users")

	assert.Contains(t, result, "model_ratio", "model_ratio should be kept")
	assert.Contains(t, result, "group_ratio", "group_ratio should be kept")
	assert.Contains(t, result, "model_price", "model_price should be kept")
}

func TestFormatUserLogsHandlesNilOther(t *testing.T) {
	logs := []*Log{
		{
			Id:    1,
			Other: "",
		},
	}

	formatUserLogs(logs, 0)
	assert.Equal(t, 1, logs[0].Id)
}

func TestFormatUserLogsHandlesNilOtherMap(t *testing.T) {
	logs := []*Log{
		{
			Id:    1,
			Other: "invalid json",
		},
	}

	formatUserLogs(logs, 0)
	assert.Equal(t, 1, logs[0].Id)
}

func TestFormatUserLogsKeepsNonSensitiveFields(t *testing.T) {
	other := map[string]interface{}{
		"model_ratio":        2.0,
		"group_ratio":        0.5,
		"completion_ratio":   1.0,
		"cache_tokens":       100,
		"cache_ratio":        0.1,
		"model_price":        0.02,
		"user_group_ratio":   1.0,
		"frt":                150.0,
		"reasoning_effort":   "high",
		"request_path":       "/v1/chat/completions",
		"billing_mode":       "tiered_expr",
		"matched_tier":       "base",
		"expr_b64":           "ZXhwcg==",
	}
	otherJson := common.MapToJsonStr(other)

	logs := []*Log{
		{
			Id:    1,
			Other: otherJson,
		},
	}

	formatUserLogs(logs, 0)

	var result map[string]interface{}
	err := common.UnmarshalJsonStr(logs[0].Other, &result)
	assert.NoError(t, err)

	for key := range other {
		assert.Contains(t, result, key, "non-sensitive field %s should be kept", key)
	}
}

func TestFormatUserLogsSetsChannelNameEmpty(t *testing.T) {
	logs := []*Log{
		{
			Id:          1,
			ChannelName: "some-channel",
			Other:       "{}",
		},
	}

	formatUserLogs(logs, 0)
	assert.Empty(t, logs[0].ChannelName, "ChannelName should be cleared")
}

func TestFormatUserLogsSetsSequentialId(t *testing.T) {
	logs := []*Log{
		{Id: 100, Other: "{}"},
		{Id: 101, Other: "{}"},
		{Id: 102, Other: "{}"},
	}

	formatUserLogs(logs, 5)
	assert.Equal(t, 6, logs[0].Id)
	assert.Equal(t, 7, logs[1].Id)
	assert.Equal(t, 8, logs[2].Id)
}
