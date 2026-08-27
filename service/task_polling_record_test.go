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
package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestTruncatePollPayloadWithinLimit(t *testing.T) {
	payload := []byte(`{"status":"SUCCESS"}`)
	result := truncatePollPayload(payload)
	assert.Equal(t, payload, result)
}

func TestTruncatePollPayloadExactLimit(t *testing.T) {
	payload := bytes.Repeat([]byte("a"), maxPollRecordPayloadBytes)
	result := truncatePollPayload(payload)
	assert.Equal(t, payload, result)
}

func TestTruncatePollPayloadOverLimit(t *testing.T) {
	payload := bytes.Repeat([]byte("a"), maxPollRecordPayloadBytes+100)
	result := truncatePollPayload(payload)
	assert.Len(t, result, maxPollRecordPayloadBytes+len(pollPayloadTruncatedSuffix))
	assert.True(t, bytes.HasPrefix(result, bytes.Repeat([]byte("a"), 1024)))
	assert.True(t, strings.HasSuffix(string(result), pollPayloadTruncatedSuffix))
}

func newTestPollResponse(t *testing.T, method, rawURL string, statusCode int) *http.Response {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	require.NoError(t, err)
	req := &http.Request{Method: method, URL: parsed}
	return &http.Response{StatusCode: statusCode, Request: req}
}

func TestNewPollRecordCapturesRequestInfo(t *testing.T) {
	resp := newTestPollResponse(t, http.MethodGet, "https://api.example.com/v1/videos/abc", http.StatusOK)
	before := time.Now().Unix()

	record := newPollRecord(resp, map[string]any{
		"task_id": "abc",
		"action":  "GENERATE",
	}, []byte(`{"status":"IN_PROGRESS"}`))

	assert.Equal(t, http.MethodGet, record.Method)
	assert.Equal(t, "https://api.example.com/v1/videos/abc", record.URL)
	assert.Equal(t, http.StatusOK, record.StatusCode)
	assert.JSONEq(t, `{"task_id":"abc","action":"GENERATE"}`, string(record.Request))
	assert.JSONEq(t, `{"status":"IN_PROGRESS"}`, string(record.Response))
	assert.GreaterOrEqual(t, record.Time, before)
}

func TestNewPollRecordNilResponse(t *testing.T) {
	record := newPollRecord(nil, map[string]any{"ids": []string{"a", "b"}}, nil)
	assert.Empty(t, record.Method)
	assert.Empty(t, record.URL)
	assert.Zero(t, record.StatusCode)
	assert.JSONEq(t, `{"ids":["a","b"]}`, string(record.Request))
	assert.Empty(t, record.Response)
}

func TestNewPollRecordNilRequestOnResponse(t *testing.T) {
	resp := &http.Response{StatusCode: http.StatusTooManyRequests}
	record := newPollRecord(resp, nil, []byte(`{"error":"rate limited"}`))
	assert.Equal(t, http.StatusTooManyRequests, record.StatusCode)
	assert.Empty(t, record.Method)
	assert.Empty(t, record.URL)
	assert.Empty(t, record.Request)
}

func TestNewPollRecordTruncatesLargePayloads(t *testing.T) {
	resp := newTestPollResponse(t, http.MethodPost, "https://api.example.com/fetch", http.StatusOK)
	largeResponse := bytes.Repeat([]byte("x"), maxPollRecordPayloadBytes+10)

	record := newPollRecord(resp, nil, largeResponse)
	assert.Len(t, record.Response, maxPollRecordPayloadBytes+len(pollPayloadTruncatedSuffix))
	assert.True(t, strings.HasSuffix(string(record.Response), pollPayloadTruncatedSuffix))
}

// parseFailAdaptor 解析永远失败的轮询适配器 mock，
// 用于验证解析失败提前返回路径仍会持久化轮询记录。
type parseFailAdaptor struct{}

func (parseFailAdaptor) Init(info *relaycommon.RelayInfo) {}

func (parseFailAdaptor) FetchTask(baseURL, key string, body map[string]any, proxy string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, baseURL+"/api/v1/tasks/up-1", nil)
	if err != nil {
		return nil, err
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Request:    req,
		Body:       io.NopCloser(strings.NewReader(`{"code":400,"message":"bad request"}`)),
	}, nil
}

func (parseFailAdaptor) ParseTaskResult(body []byte) (*relaycommon.TaskInfo, error) {
	return nil, errors.New("unmarshal task result failed: json: cannot unmarshal number into Go value of type string")
}

func (parseFailAdaptor) AdjustBillingOnComplete(task *model.Task, taskResult *relaycommon.TaskInfo) int {
	return 0
}

func TestUpdateVideoSingleTaskPersistsPollRecordOnParseFailure(t *testing.T) {
	require.NoError(t, model.DB.Session(&gorm.Session{}).Exec("DELETE FROM tasks").Error)

	task := &model.Task{
		TaskID: "public-poll-1",
		Status: model.TaskStatusInProgress,
	}
	require.NoError(t, model.DB.Create(task).Error)

	ch := &model.Channel{Type: 1, Key: "test-key"}
	err := updateVideoSingleTask(context.Background(), parseFailAdaptor{}, ch, "up-1", map[string]*model.Task{"up-1": task})
	require.Error(t, err)

	var got model.Task
	require.NoError(t, model.DB.First(&got, "id = ?", task.ID).Error)
	require.NotEmpty(t, got.PollRecord.Response, "解析失败时也应持久化轮询记录")
	assert.Contains(t, string(got.PollRecord.Response), `"code":400`)
	assert.Equal(t, http.StatusOK, got.PollRecord.StatusCode)
	assert.Equal(t, http.MethodGet, got.PollRecord.Method)
}
