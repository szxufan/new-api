package controller

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
)

func TestApplyAntiCache_Disabled(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Messages: []dto.Message{{Role: "user", Content: "hi"}},
	}
	got := applyAntiCache(req, false)
	if got != req {
		t.Error("disabled should return same request")
	}
}

func TestApplyAntiCache_Chat(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Messages: []dto.Message{
			{Role: "system", Content: "sys"},
			{Role: "user", Content: "hi"},
		},
	}
	got := applyAntiCache(req, true).(*dto.GeneralOpenAIRequest)
	content, ok := got.Messages[1].Content.(string)
	if !ok {
		t.Fatalf("expected string content, got %T", got.Messages[1].Content)
	}
	if !strings.HasPrefix(content, "hi ") {
		t.Errorf("expected prefix 'hi ', got %q", content)
	}
	if !strings.Contains(content, "[") {
		t.Error("expected timestamp bracket in content")
	}
	sysContent, ok := got.Messages[0].Content.(string)
	if !ok || sysContent != "sys" {
		t.Error("system message should not be modified")
	}
}

func TestApplyAntiCache_Responses(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Input: json.RawMessage(`[{"role":"user","content":"hi"}]`),
	}
	got := applyAntiCache(req, true).(*dto.OpenAIResponsesRequest)
	var msgs []map[string]string
	if err := json.Unmarshal(got.Input, &msgs); err != nil {
		t.Fatal(err)
	}
	content := msgs[0]["content"]
	if !strings.HasPrefix(content, "hi ") {
		t.Errorf("expected prefix 'hi ', got %q", content)
	}
	if !strings.Contains(content, "[") {
		t.Error("expected timestamp bracket in content")
	}
}

func TestApplyAntiCache_EmbeddingUnchanged(t *testing.T) {
	req := &dto.EmbeddingRequest{Input: []any{"hello world"}}
	got := applyAntiCache(req, true)
	if got != req {
		t.Error("embedding request should be returned unchanged")
	}
}
