package helper

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
)

func TestCheckNonStreamResponse(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		info      *relaycommon.RelayInfo
		wantErr   bool
	}{
		{
			name: "no detection config",
			text: "I cannot help",
			info: &relaycommon.RelayInfo{
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelSetting: dto.ChannelSettings{},
				},
			},
			wantErr: false,
		},
		{
			name: "nil channel meta",
			text: "I cannot help",
			info: &relaycommon.RelayInfo{},
			wantErr: false,
		},
		{
			name: "detection hit",
			text: "I cannot help with that",
			info: &relaycommon.RelayInfo{
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelSetting: dto.ChannelSettings{
						ResponseDetection: &dto.ResponseDetection{
							Enabled:  true,
							Keywords: []string{"cannot"},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "detection no hit",
			text: "The answer is 42",
			info: &relaycommon.RelayInfo{
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelSetting: dto.ChannelSettings{
						ResponseDetection: &dto.ResponseDetection{
							Enabled:  true,
							Keywords: []string{"cannot"},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckNonStreamResponse(tt.text, tt.info)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckNonStreamResponse() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && !IsDetectionHitError(err) {
				t.Errorf("CheckNonStreamResponse() error should be detection hit error, got code=%v", err.GetErrorCode())
			}
		})
	}
}

func TestIsDetectionHitError(t *testing.T) {
	tests := []struct {
		name string
		err  *types.NewAPIError
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "detection hit error",
			err:  types.NewError(fmt.Errorf("test"), types.ErrorCodeResponseDetectionHit),
			want: true,
		},
		{
			name: "other error",
			err:  types.NewError(fmt.Errorf("test"), types.ErrorCodeInvalidRequest),
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsDetectionHitError(tt.err); got != tt.want {
				t.Errorf("IsDetectionHitError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractOpenAIStreamText(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{
			name: "content field",
			data: `{"choices":[{"delta":{"content":"Hello"}}]}`,
			want: "Hello",
		},
		{
			name: "reasoning_content field",
			data: `{"choices":[{"delta":{"reasoning_content":"thinking..."}}]}`,
			want: "thinking...",
		},
		{
			name: "no content",
			data: `{"choices":[{"delta":{"role":"assistant"}}]}`,
			want: "",
		},
		{
			name: "empty data",
			data: `{}`,
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractOpenAIStreamText(tt.data); got != tt.want {
				t.Errorf("extractOpenAIStreamText() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractClaudeStreamText(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{
			name: "text delta",
			data: `{"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}`,
			want: "Hello",
		},
		{
			name: "thinking delta",
			data: `{"type":"content_block_delta","delta":{"type":"thinking_delta","thinking":"hmm..."}}`,
			want: "hmm...",
		},
		{
			name: "no text",
			data: `{"type":"message_start"}`,
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractClaudeStreamText(tt.data); got != tt.want {
				t.Errorf("extractClaudeStreamText() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractGeminiStreamText(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{
			name: "single candidate",
			data: `{"candidates":[{"content":{"parts":[{"text":"Hello"}]}}]}`,
			want: "Hello",
		},
		{
			name: "multiple parts",
			data: `{"candidates":[{"content":{"parts":[{"text":"Hello"},{"text":" World"}]}}]}`,
			want: "Hello World",
		},
		{
			name: "no text",
			data: `{"candidates":[{"content":{"parts":[]}}]}`,
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractGeminiStreamText(tt.data); got != tt.want {
				t.Errorf("extractGeminiStreamText() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStreamDetectionWrapper_NoConfig(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelSetting: dto.ChannelSettings{},
		},
	}
	called := false
	original := func(data string, sr *StreamResult) {
		called = true
	}
	wrapped := StreamDetectionWrapper(original, info)
	sr := &StreamResult{}
	wrapped("test", sr)
	if !called {
		t.Error("original handler should be called when no detection config")
	}
	if info.DetectionHit {
		t.Error("DetectionHit should be false when no detection config")
	}
}

func TestStreamDetectionWrapper_HitDoesNotStop(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelSetting: dto.ChannelSettings{
				ResponseDetection: &dto.ResponseDetection{
					Enabled:  true,
					Keywords: []string{"cannot"},
				},
			},
		},
	}
	callCount := 0
	original := func(data string, sr *StreamResult) {
		callCount++
	}
	wrapped := StreamDetectionWrapper(original, info)
	sr := newStreamResult(nil)

	// First chunk - no hit
	wrapped(`{"choices":[{"delta":{"content":"Hello "}}]}`, sr)
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
	if info.DetectionHit {
		t.Error("DetectionHit should be false after non-matching chunk")
	}
	if sr.IsStopped() {
		t.Error("stream should NOT be stopped on detection hit")
	}

	// Second chunk - hit
	wrapped(`{"choices":[{"delta":{"content":"I cannot help"}}]}`, sr)
	if callCount != 2 {
		t.Errorf("expected 2 calls, got %d", callCount)
	}
	if !info.DetectionHit {
		t.Error("DetectionHit should be true after matching chunk")
	}
	if sr.IsStopped() {
		t.Error("stream should NOT be stopped - detection is transparent")
	}

	// Third chunk - should still forward (not stopped)
	wrapped(`{"choices":[{"delta":{"content":" more text"}}]}`, sr)
	if callCount != 3 {
		t.Errorf("expected 3 calls, got %d - detection hit should not stop forwarding", callCount)
	}
}

func TestStreamDetectionWrapper_AlreadyHitSkipsDetection(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelSetting: dto.ChannelSettings{
				ResponseDetection: &dto.ResponseDetection{
					Enabled:  true,
					Keywords: []string{"cannot"},
				},
			},
		},
	}
	// Pre-set DetectionHit
	info.SetDetectionHit([]string{"cannot"})

	callCount := 0
	original := func(data string, sr *StreamResult) {
		callCount++
	}
	wrapped := StreamDetectionWrapper(original, info)
	sr := newStreamResult(nil)

	// Should still forward but skip detection
	wrapped(`{"choices":[{"delta":{"content":"more text"}}]}`, sr)
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
}

func TestExtractFullTextFromResponse(t *testing.T) {
	tests := []struct {
		name         string
		relayFormat  types.RelayFormat
		responseBody []byte
		want         string
	}{
		{
			name:         "openai format",
			relayFormat:  types.RelayFormatOpenAI,
			responseBody: []byte(`{"choices":[{"message":{"content":"Hello world"}}]}`),
			want:         "Hello world",
		},
		{
			name:         "openai with reasoning",
			relayFormat:  types.RelayFormatOpenAI,
			responseBody: []byte(`{"choices":[{"message":{"content":"Hello","reasoning_content":"thinking"}}]}`),
			want:         "Hello thinking",
		},
		{
			name:         "claude format",
			relayFormat:  types.RelayFormatClaude,
			responseBody: []byte(`{"content":[{"type":"text","text":"Hello"},{"type":"text","text":"world"}]}`),
			want:         "Hello world",
		},
		{
			name:         "gemini format",
			relayFormat:  types.RelayFormatGemini,
			responseBody: []byte(`{"candidates":[{"content":{"parts":[{"text":"Hello"}]}}]}`),
			want:         "Hello",
		},
		{
			name:         "unknown format fallback",
			relayFormat:  "unknown",
			responseBody: []byte(`raw text`),
			want:         "raw text",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &relaycommon.RelayInfo{RelayFormat: tt.relayFormat}
			got := ExtractFullTextFromResponse(info, tt.responseBody)
			if got != tt.want {
				t.Errorf("ExtractFullTextFromResponse() = %v, want %v", got, tt.want)
			}
		})
	}
}
