package dto

import "encoding/json"

// ResponsesContextEntry 缓存条目，用于存储响应上下文以支持 previous_response_id 功能
// 仅在回退场景（上游不支持 /v1/responses）下使用
type ResponsesContextEntry struct {
	Model        string                `json:"model"`
	Instructions json.RawMessage       `json:"instructions,omitempty"`
	Input        json.RawMessage       `json:"input,omitempty"`
	Output       []ResponsesOutput     `json:"output"`
	Tools        json.RawMessage       `json:"tools,omitempty"`
}