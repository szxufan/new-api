/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskPollRecordIsEmpty(t *testing.T) {
	assert.True(t, TaskPollRecord{}.IsEmpty())
	assert.False(t, TaskPollRecord{Time: 1}.IsEmpty())
	assert.False(t, TaskPollRecord{URL: "https://example.com"}.IsEmpty())
	assert.False(t, TaskPollRecord{Request: json.RawMessage(`{}`)}.IsEmpty())
}

func TestTaskPollRecordValueZeroReturnsNil(t *testing.T) {
	record := TaskPollRecord{}
	value, err := record.Value()
	require.NoError(t, err)
	assert.Nil(t, value)
}

func TestTaskPollRecordScanValueRoundTrip(t *testing.T) {
	original := TaskPollRecord{
		Time:       1700000000,
		Method:     "GET",
		URL:        "https://api.example.com/v1/videos/abc",
		StatusCode: 200,
		Request:    json.RawMessage(`{"task_id":"abc","action":"GENERATE"}`),
		Response:   json.RawMessage(`{"status":"SUCCESS"}`),
	}

	value, err := original.Value()
	require.NoError(t, err)
	bytesValue, ok := value.([]byte)
	require.True(t, ok)

	var restored TaskPollRecord
	require.NoError(t, restored.Scan(bytesValue))
	assert.True(t, EqualPollRecord(original, restored))
}

func TestTaskPollRecordScanEmpty(t *testing.T) {
	record := TaskPollRecord{Time: 123}
	require.NoError(t, record.Scan(nil))
	assert.True(t, record.IsEmpty())

	record = TaskPollRecord{Time: 123}
	require.NoError(t, record.Scan([]byte{}))
	assert.True(t, record.IsEmpty())
}

func TestEqualPollRecord(t *testing.T) {
	a := TaskPollRecord{
		Time:     1700000000,
		Method:   "GET",
		URL:      "https://api.example.com/v1/videos/abc",
		Request:  json.RawMessage(`{"task_id":"abc"}`),
		Response: json.RawMessage(`{"status":"SUCCESS"}`),
	}
	b := a
	assert.True(t, EqualPollRecord(a, b))

	b.URL = "https://api.example.com/v1/videos/other"
	assert.False(t, EqualPollRecord(a, b))

	b = a
	b.Response = json.RawMessage(`{"status":"FAILURE"}`)
	assert.False(t, EqualPollRecord(a, b))
}

func TestTaskPollRecordPersistenceRoundTrip(t *testing.T) {
	truncateTables(t)

	task := &Task{
		TaskID:   "task_poll_record_test",
		Platform: "kling",
		UserId:   1,
		Status:   TaskStatusInProgress,
		PollRecord: TaskPollRecord{
			Time:       1700000000,
			Method:     "GET",
			URL:        "https://api.example.com/v1/videos/abc",
			StatusCode: 200,
			Request:    json.RawMessage(`{"task_id":"abc"}`),
			Response:   json.RawMessage(`{"status":"IN_PROGRESS"}`),
		},
	}
	insertTask(t, task)

	var loaded Task
	require.NoError(t, DB.Where("task_id = ?", "task_poll_record_test").First(&loaded).Error)
	assert.True(t, EqualPollRecord(task.PollRecord, loaded.PollRecord))

	// 未轮询过的任务读取后应为空记录
	empty := &Task{
		TaskID:   "task_poll_record_empty",
		Platform: "kling",
		UserId:   1,
		Status:   TaskStatusSubmitted,
	}
	insertTask(t, empty)

	var loadedEmpty Task
	require.NoError(t, DB.Where("task_id = ?", "task_poll_record_empty").First(&loadedEmpty).Error)
	assert.True(t, loadedEmpty.PollRecord.IsEmpty())
}

func TestTaskSnapshotIncludesPollRecord(t *testing.T) {
	task := &Task{
		Status: TaskStatusInProgress,
		PollRecord: TaskPollRecord{
			Time: 1700000000,
			URL:  "https://api.example.com/v1/videos/abc",
		},
	}
	snap := task.Snapshot()

	// 仅轮询记录变化时，快照应判定不相等（触发写库）
	task.PollRecord.Time = 1700000015
	assert.False(t, snap.Equal(task.Snapshot()))

	// 轮询记录一致时应判定相等
	snap2 := task.Snapshot()
	assert.True(t, snap2.Equal(task.Snapshot()))
}
