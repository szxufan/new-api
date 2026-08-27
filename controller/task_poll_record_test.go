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

// TestTasksToDtoPollRecordOwnerOnly 轮询记录含敏感信息（上游 URL、请求/响应原文），
// 仅任务提交者本人可见：他人任务（含管理员查看）一律不下发；
// 本人任务暂无记录时下发空对象，供前端区分"无权限"与"暂无记录"。
func TestTasksToDtoPollRecordOwnerOnly(t *testing.T) {
	record := model.TaskPollRecord{
		Time:       1700000000,
		Method:     "GET",
		URL:        "https://api.example.com/v1/videos/abc",
		StatusCode: 200,
		Request:    json.RawMessage(`{"task_id":"abc"}`),
		Response:   json.RawMessage(`{"status":"SUCCESS"}`),
	}
	tasks := []*model.Task{
		{TaskID: "own-with-record", UserId: 1, PollRecord: record},
		{TaskID: "other-with-record", UserId: 2, PollRecord: record},
		{TaskID: "own-empty", UserId: 1},
	}

	result := tasksToDto(tasks, false, 1)
	require.Len(t, result, 3)

	// 本人任务且有记录：完整下发
	require.NotNil(t, result[0].PollRecord)
	assert.Equal(t, int64(1700000000), result[0].PollRecord.Time)
	assert.Equal(t, "https://api.example.com/v1/videos/abc", result[0].PollRecord.URL)

	// 他人任务：不下发（即使请求者是管理员）
	assert.Nil(t, result[1].PollRecord)

	// 本人任务但暂无记录：下发空对象
	require.NotNil(t, result[2].PollRecord)
	assert.Equal(t, int64(0), result[2].PollRecord.Time)
	assert.Empty(t, result[2].PollRecord.URL)
}

// TestTasksToDtoPollRecordUnauthenticatedRequester 未登录（requesterId=0）
// 时任何任务的轮询记录都不下发。
func TestTasksToDtoPollRecordUnauthenticatedRequester(t *testing.T) {
	record := model.TaskPollRecord{Time: 1700000000, Method: "GET"}
	tasks := []*model.Task{
		{TaskID: "t1", UserId: 1, PollRecord: record},
	}

	result := tasksToDto(tasks, false, 0)
	require.Len(t, result, 1)
	assert.Nil(t, result[0].PollRecord)
}
