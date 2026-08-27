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
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
