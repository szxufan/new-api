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

// TestTasksToDtoPollRecordOwnerOnly 轮询记录中的敏感字段（请求 URL、请求体、
// 响应原文）仅任务提交者本人可见；他人（含管理员）仅保留非敏感字段
// （轮询时间、状态码）。本人任务暂无记录时下发空对象，供前端显示"暂无轮询记录"。
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
		{TaskID: "other-empty", UserId: 2},
	}

	result := tasksToDto(tasks, false, 1)
	require.Len(t, result, 4)

	// 本人任务且有记录：完整下发
	require.NotNil(t, result[0].PollRecord)
	assert.Equal(t, int64(1700000000), result[0].PollRecord.Time)
	assert.Equal(t, "GET", result[0].PollRecord.Method)
	assert.Equal(t, "https://api.example.com/v1/videos/abc", result[0].PollRecord.URL)
	assert.Equal(t, 200, result[0].PollRecord.StatusCode)
	assert.JSONEq(t, `{"task_id":"abc"}`, string(result[0].PollRecord.Request))
	assert.JSONEq(t, `{"status":"SUCCESS"}`, string(result[0].PollRecord.Response))

	// 他人任务：保留轮询时间与状态码，剔除请求 URL、请求体、响应原文
	require.NotNil(t, result[1].PollRecord)
	assert.Equal(t, int64(1700000000), result[1].PollRecord.Time)
	assert.Equal(t, 200, result[1].PollRecord.StatusCode)
	assert.Empty(t, result[1].PollRecord.Method)
	assert.Empty(t, result[1].PollRecord.URL)
	assert.Nil(t, result[1].PollRecord.Request)
	assert.Nil(t, result[1].PollRecord.Response)

	// 本人任务但暂无记录：下发空对象
	require.NotNil(t, result[2].PollRecord)
	assert.Equal(t, int64(0), result[2].PollRecord.Time)
	assert.Empty(t, result[2].PollRecord.URL)

	// 他人任务且无记录：不下发
	assert.Nil(t, result[3].PollRecord)
}

// TestTasksToDtoPollRecordUnauthenticatedRequester 未登录（requesterId=0）
// 时按他人处理：有记录仅见时间与状态码，无记录不下发。
func TestTasksToDtoPollRecordUnauthenticatedRequester(t *testing.T) {
	record := model.TaskPollRecord{Time: 1700000000, Method: "GET", StatusCode: 200}
	tasks := []*model.Task{
		{TaskID: "t1", UserId: 1, PollRecord: record},
		{TaskID: "t2", UserId: 1},
	}

	result := tasksToDto(tasks, false, 0)
	require.Len(t, result, 2)
	require.NotNil(t, result[0].PollRecord)
	assert.Equal(t, int64(1700000000), result[0].PollRecord.Time)
	assert.Equal(t, 200, result[0].PollRecord.StatusCode)
	assert.Empty(t, result[0].PollRecord.Method)
	assert.Nil(t, result[1].PollRecord)
}
