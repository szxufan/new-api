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
		name         string
		text         string
		hasToolCalls bool
		info         *relaycommon.RelayInfo
		wantErr      bool
	}{
		{
			name:         "no detection config",
			text:         "I cannot help",
			hasToolCalls: false,
			info: &relaycommon.RelayInfo{
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelSetting: dto.ChannelSettings{},
				},
			},
			wantErr: false,
		},
		{
			name:         "nil channel meta",
			text:         "I cannot help",
			hasToolCalls: false,
			info:         &relaycommon.RelayInfo{},
			wantErr:      false,
		},
		{
			name:         "detection hit",
			text:         "I cannot help with that",
			hasToolCalls: false,
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
			name:         "detection no hit",
			text:         "The answer is 42",
			hasToolCalls: false,
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
		{
			name:         "AllowEmpty + empty text + no tool calls → hit",
			text:         "",
			hasToolCalls: false,
			info: &relaycommon.RelayInfo{
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelSetting: dto.ChannelSettings{
						ResponseDetection: &dto.ResponseDetection{
							Enabled:    true,
							AllowEmpty: true,
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name:         "AllowEmpty + empty text + has tool calls → no hit",
			text:         "",
			hasToolCalls: true,
			info: &relaycommon.RelayInfo{
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelSetting: dto.ChannelSettings{
						ResponseDetection: &dto.ResponseDetection{
							Enabled:    true,
							AllowEmpty: true,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:         "AllowEmpty=false + empty text → no hit (backward compatibility)",
			text:         "",
			hasToolCalls: false,
			info: &relaycommon.RelayInfo{
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelSetting: dto.ChannelSettings{
						ResponseDetection: &dto.ResponseDetection{
							Enabled:    true,
							AllowEmpty: false,
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckNonStreamResponse(tt.text, tt.hasToolCalls, tt.info)
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
	wrapped, finalizer := StreamDetectionWrapper(original, info)
	sr := &StreamResult{}
	wrapped("test", sr)
	if !called {
		t.Error("original handler should be called when no detection config")
	}
	if info.DetectionHit {
		t.Error("DetectionHit should be false when no detection config")
	}
	if finalizer != nil {
		t.Error("finalizer should be nil when no detection config")
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
	wrapped, finalizer := StreamDetectionWrapper(original, info)
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

	// finalizer 应存在（AllowEmpty=false 但有 keywords）
	if finalizer == nil {
		t.Error("finalizer should not be nil when detection enabled with keywords")
	}
	// 调用 finalizer 不应改变已命中状态
	finalizer()
	if !info.DetectionHit {
		t.Error("DetectionHit should remain true after finalizer")
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
	wrapped, _ := StreamDetectionWrapper(original, info)
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

// TestStreamDetectionWrapper_EmptyResponseHit 验证流式空回复命中：
// AllowEmpty=true 且无内容 chunk 且无工具调用 → finalizer 调用后 DetectionHit=true
func TestStreamDetectionWrapper_EmptyResponseHit(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelSetting: dto.ChannelSettings{
				ResponseDetection: &dto.ResponseDetection{
					Enabled:    true,
					AllowEmpty: true,
				},
			},
		},
	}
	callCount := 0
	original := func(data string, sr *StreamResult) {
		callCount++
	}
	wrapped, finalizer := StreamDetectionWrapper(original, info)
	if finalizer == nil {
		t.Fatal("finalizer should not be nil when AllowEmpty=true")
	}
	sr := newStreamResult(nil)

	// 模拟一个空回复流：只有 role chunk，无 content
	wrapped(`{"choices":[{"delta":{"role":"assistant"}}]}`, sr)
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
	if info.DetectionHit {
		t.Error("DetectionHit should be false before finalizer (empty response judged at stream end)")
	}

	// 流结束：调用 finalizer，应触发空回复命中
	finalizer()
	if !info.DetectionHit {
		t.Error("DetectionHit should be true after finalizer for empty response with AllowEmpty")
	}
	if info.DetectionHitKeywords != nil {
		t.Errorf("DetectionHitKeywords should be nil for empty response hit, got %v", info.DetectionHitKeywords)
	}
}

// TestStreamDetectionWrapper_EmptyResponseWithToolCallsNoHit 验证流式有工具调用时不触发空回复命中
func TestStreamDetectionWrapper_EmptyResponseWithToolCallsNoHit(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelSetting: dto.ChannelSettings{
				ResponseDetection: &dto.ResponseDetection{
					Enabled:    true,
					AllowEmpty: true,
				},
			},
		},
	}
	original := func(data string, sr *StreamResult) {}
	wrapped, finalizer := StreamDetectionWrapper(original, info)
	if finalizer == nil {
		t.Fatal("finalizer should not be nil when AllowEmpty=true")
	}
	sr := newStreamResult(nil)

	// 模拟仅工具调用、无文本的流
	wrapped(`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"get_weather","arguments":"{\"location\":\"SF\"}"}}]}}]}`, sr)

	// 流结束：调用 finalizer，不应触发命中（有工具调用）
	finalizer()
	if info.DetectionHit {
		t.Error("DetectionHit should be false when response has tool calls (not empty)")
	}
}

// TestStreamDetectionWrapper_NonEmptyResponseNoHit 验证非空文本不触发空回复命中
func TestStreamDetectionWrapper_NonEmptyResponseNoHit(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelSetting: dto.ChannelSettings{
				ResponseDetection: &dto.ResponseDetection{
					Enabled:    true,
					AllowEmpty: true,
				},
			},
		},
	}
	original := func(data string, sr *StreamResult) {}
	wrapped, finalizer := StreamDetectionWrapper(original, info)
	if finalizer == nil {
		t.Fatal("finalizer should not be nil when AllowEmpty=true")
	}
	sr := newStreamResult(nil)

	// 模拟非空文本流
	wrapped(`{"choices":[{"delta":{"content":"Hello world"}}]}`, sr)

	// 流结束：调用 finalizer，不应触发空回复命中（有内容）
	finalizer()
	if info.DetectionHit {
		t.Error("DetectionHit should be false when response has content")
	}
}

// TestStreamDetectionWrapper_AllowEmptyFalseNoFinalizer 验证未开启 AllowEmpty 时
// 若无关键词，finalizer 为 nil（无需包装）
func TestStreamDetectionWrapper_AllowEmptyFalseNoFinalizer(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelSetting: dto.ChannelSettings{
				ResponseDetection: &dto.ResponseDetection{
					Enabled:    true,
					AllowEmpty: false,
				},
			},
		},
	}
	original := func(data string, sr *StreamResult) {}
	wrapped, finalizer := StreamDetectionWrapper(original, info)
	if finalizer != nil {
		t.Error("finalizer should be nil when AllowEmpty=false and no keywords")
	}
	if wrapped == nil {
		t.Error("wrapped handler should not be nil")
	}
}

// TestStreamDetectionWrapper_AllowEmptyWithKeywordsEmptyHit 验证 AllowEmpty + 关键词场景下
// 空文本先于关键词命中（finalizer 触发空回复命中）
func TestStreamDetectionWrapper_AllowEmptyWithKeywordsEmptyHit(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelSetting: dto.ChannelSettings{
				ResponseDetection: &dto.ResponseDetection{
					Enabled:    true,
					Keywords:   []string{"cannot"},
					AllowEmpty: true,
				},
			},
		},
	}
	original := func(data string, sr *StreamResult) {}
	wrapped, finalizer := StreamDetectionWrapper(original, info)
	if finalizer == nil {
		t.Fatal("finalizer should not be nil when AllowEmpty=true")
	}
	sr := newStreamResult(nil)

	// 空文本流（无 content chunk）
	wrapped(`{"choices":[{"delta":{"role":"assistant"}}]}`, sr)
	if info.DetectionHit {
		t.Error("DetectionHit should be false before finalizer")
	}

	finalizer()
	if !info.DetectionHit {
		t.Error("DetectionHit should be true after finalizer for empty response")
	}
	if info.DetectionHitKeywords != nil {
		t.Errorf("DetectionHitKeywords should be nil for empty response hit, got %v", info.DetectionHitKeywords)
	}
}

// TestExtractStreamHasToolCalls 验证各格式流式工具调用检测
func TestExtractStreamHasToolCalls(t *testing.T) {
	tests := []struct {
		name        string
		data        string
		relayFormat types.RelayFormat
		want        bool
	}{
		{
			name:        "openai with tool_calls",
			data:        `{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function"}]}}]}`,
			relayFormat: types.RelayFormatOpenAI,
			want:        true,
		},
		{
			name:        "openai without tool_calls",
			data:        `{"choices":[{"delta":{"content":"Hello"}}]}`,
			relayFormat: types.RelayFormatOpenAI,
			want:        false,
		},
		{
			name:        "claude tool_use content_block_start",
			data:        `{"type":"content_block_start","content_block":{"type":"tool_use","id":"tool_1","name":"get_weather"}}`,
			relayFormat: types.RelayFormatClaude,
			want:        true,
		},
		{
			name:        "claude input_json_delta",
			data:        `{"type":"content_block_delta","delta":{"type":"input_json_delta","partial_json":"{\"a\":"}}`,
			relayFormat: types.RelayFormatClaude,
			want:        true,
		},
		{
			name:        "claude text_delta",
			data:        `{"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}`,
			relayFormat: types.RelayFormatClaude,
			want:        false,
		},
		{
			name:        "gemini functionCall",
			data:        `{"candidates":[{"content":{"parts":[{"functionCall":{"name":"get_weather","args":{"location":"SF"}}}]}}]}`,
			relayFormat: types.RelayFormatGemini,
			want:        true,
		},
		{
			name:        "gemini text only",
			data:        `{"candidates":[{"content":{"parts":[{"text":"Hello"}]}}]}`,
			relayFormat: types.RelayFormatGemini,
			want:        false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &relaycommon.RelayInfo{RelayFormat: tt.relayFormat}
			got := extractStreamHasToolCalls(tt.data, info)
			if got != tt.want {
				t.Errorf("extractStreamHasToolCalls() = %v, want %v", got, tt.want)
			}
		})
	}
}
