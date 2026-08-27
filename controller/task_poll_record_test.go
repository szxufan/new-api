package controller

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPollRecordToDtoNil(t *testing.T) {
	assert.Nil(t, pollRecordToDto(nil))
}

func TestPollRecordToDtoEmpty(t *testing.T) {
	record := &model.TaskPollRecord{}
	assert.Nil(t, pollRecordToDto(record))
}

func TestPollRecordToDtoMapping(t *testing.T) {
	record := &model.TaskPollRecord{
		Time:       1700000000,
		Method:     "GET",
		URL:        "https://api.example.com/v1/videos/abc",
		StatusCode: 200,
		Request:    json.RawMessage(`{"task_id":"abc"}`),
		Response:   json.RawMessage(`{"status":"SUCCESS"}`),
	}

	result := pollRecordToDto(record)
	require.NotNil(t, result)
	assert.Equal(t, record.Time, result.Time)
	assert.Equal(t, record.Method, result.Method)
	assert.Equal(t, record.URL, result.URL)
	assert.Equal(t, record.StatusCode, result.StatusCode)
	assert.JSONEq(t, `{"task_id":"abc"}`, string(result.Request))
	assert.JSONEq(t, `{"status":"SUCCESS"}`, string(result.Response))
}
