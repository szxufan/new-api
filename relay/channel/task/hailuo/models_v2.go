package hailuo

import "encoding/json"

// ============================================================================
// 视频生成 v2（MiniMax-H3）DTO
// 与 v1 的扁平结构不同，v2 通过多模态 content 数组承载文本 / 图片 / 视频 / 音频输入。
// ============================================================================

// V2URLPart content 元素中的媒体地址载体
type V2URLPart struct {
	URL string `json:"url"`
}

// V2ContentItem v2 多模态输入元素。
// Type 为 text 时使用 Text；为 image_url / video_url / audio_url 时使用对应的 *URL 字段。
// Role 标注素材用途，图生视频（first_frame/last_frame）与多模态参考（reference_*）互斥。
type V2ContentItem struct {
	Type     string     `json:"type"`
	Text     string     `json:"text,omitempty"`
	ImageURL *V2URLPart `json:"image_url,omitempty"`
	VideoURL *V2URLPart `json:"video_url,omitempty"`
	AudioURL *V2URLPart `json:"audio_url,omitempty"`
	Role     string     `json:"role,omitempty"`
}

// V2VideoRequest v2 创建任务请求体。
// resolution 与 duration 为上游必填项；ratio 依生成场景决定是否必填。
type V2VideoRequest struct {
	Model         string          `json:"model"`
	Content       []V2ContentItem `json:"content"`
	Resolution    string          `json:"resolution"`
	Duration      int             `json:"duration"`
	Ratio         string          `json:"ratio,omitempty"`
	CallbackURL   string          `json:"callback_url,omitempty"`
	AigcWatermark *bool           `json:"aigc_watermark,omitempty"`
}

// V2CreateResponse v2 创建任务成功响应（无 v1 的 base_resp 信封）
type V2CreateResponse struct {
	TaskID string `json:"task_id"`
}

// V2ErrorEnvelope v2 错误响应：{"type":"error","error":{...},"request_id":"..."}。
// http_code 上游下发为字符串，用 RawMessage 兼容字符串/数字两种形态。
type V2ErrorEnvelope struct {
	Type  string `json:"type"`
	Error struct {
		Type     string          `json:"type"`
		Message  string          `json:"message"`
		HTTPCode json.RawMessage `json:"http_code"`
	} `json:"error"`
	RequestID string `json:"request_id"`
}

// V2QueryResponse v2 查询任务响应
type V2QueryResponse struct {
	Task V2Task `json:"task"`
}

// V2Task v2 共享任务对象（生成 / 再生成 / H3-Context-IR 共用同一结构）
type V2Task struct {
	ID         string          `json:"id"`
	Model      string          `json:"model"`
	Status     string          `json:"status"`
	CreatedAt  int64           `json:"created_at"`
	UpdatedAt  int64           `json:"updated_at"`
	Content    V2TaskContent   `json:"content"`
	Resolution string          `json:"resolution"`
	Duration   int             `json:"duration"`
	Ratio      string          `json:"ratio"`
	Usage      V2TaskUsage     `json:"usage"`
	TaskType   string          `json:"task_type"`
	Modality   string          `json:"modality"`
	Error      json.RawMessage `json:"error,omitempty"`
}

// V2TaskContent 任务产出。视频任务为 url 直链；H3-Context-IR 任务为 prompt。
type V2TaskContent struct {
	URL    string `json:"url,omitempty"`
	Prompt string `json:"prompt,omitempty"`
}

// V2TaskUsage 上游用量信息，仅用于日志与排障，不参与本次按次（seconds × resolution）计费。
type V2TaskUsage struct {
	TotalSeconds     float64 `json:"total_seconds"`
	InputSeconds     float64 `json:"input_seconds"`
	OutputSeconds    float64 `json:"output_seconds"`
	InputImageCount  int     `json:"input_image_count"`
	InputAudioSecond float64 `json:"input_audio_seconds"`
	TotalTokens      int     `json:"total_tokens"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
}
