package ali

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func TestIsHappyHorseModel(t *testing.T) {
	tests := []struct {
		model    string
		expected bool
	}{
		{"happyhorse-1.0-t2v", true},
		{"happyhorse-1.0-i2v", true},
		{"happyhorse-1.0-r2v", true},
		{"happyhorse-1.0-video-edit", true},
		{"wan2.5-i2v-preview", false},
		{"wan2.2-i2v-flash", false},
		{"wanx2.1-i2v-plus", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			result := isHappyHorseModel(tt.model)
			if result != tt.expected {
				t.Errorf("isHappyHorseModel(%q) = %v, want %v", tt.model, result, tt.expected)
			}
		})
	}
}

func TestConvertHappyHorseRequest_T2V(t *testing.T) {
	adaptor := &TaskAdaptor{}
	aliReq := &AliVideoRequest{Model: "happyhorse-1.0-t2v", Input: AliVideoInput{}, Parameters: &AliVideoParameters{}}

	req := relaycommon.TaskSubmitReq{
		Prompt: "一只猫在草地上奔跑",
		Model:  "happyhorse-1.0-t2v",
		Size:   "16:9",
	}

	adaptor.convertHappyHorseRequest(aliReq, req)

	if aliReq.Input.Prompt != "一只猫在草地上奔跑" {
		t.Errorf("prompt mismatch: got %q", aliReq.Input.Prompt)
	}
	if aliReq.Parameters.Ratio != "16:9" {
		t.Errorf("ratio mismatch: got %q, want %q", aliReq.Parameters.Ratio, "16:9")
	}
	if aliReq.Parameters.Resolution != "1080P" {
		t.Errorf("resolution mismatch: got %q, want %q", aliReq.Parameters.Resolution, "1080P")
	}
	if aliReq.Parameters.Duration != 5 {
		t.Errorf("duration mismatch: got %d, want %d", aliReq.Parameters.Duration, 5)
	}
}

func TestConvertHappyHorseRequest_I2V(t *testing.T) {
	adaptor := &TaskAdaptor{}
	aliReq := &AliVideoRequest{Model: "happyhorse-1.0-i2v", Input: AliVideoInput{}, Parameters: &AliVideoParameters{}}

	req := relaycommon.TaskSubmitReq{
		Prompt:         "猫在草地上奔跑",
		Model:          "happyhorse-1.0-i2v",
		InputReference: "https://example.com/cat.png",
	}

	adaptor.convertHappyHorseRequest(aliReq, req)

	if len(aliReq.Input.Media) != 1 {
		t.Fatalf("media count mismatch: got %d, want 1", len(aliReq.Input.Media))
	}
	if aliReq.Input.Media[0].Type != "first_frame" {
		t.Errorf("media type mismatch: got %q, want %q", aliReq.Input.Media[0].Type, "first_frame")
	}
	if aliReq.Input.Media[0].URL != "https://example.com/cat.png" {
		t.Errorf("media url mismatch: got %q", aliReq.Input.Media[0].URL)
	}
	if aliReq.Parameters.Resolution != "1080P" {
		t.Errorf("resolution mismatch: got %q, want %q", aliReq.Parameters.Resolution, "1080P")
	}
}

func TestConvertHappyHorseRequest_I2V_WithImages(t *testing.T) {
	adaptor := &TaskAdaptor{}
	aliReq := &AliVideoRequest{Model: "happyhorse-1.0-i2v", Input: AliVideoInput{}, Parameters: &AliVideoParameters{}}

	req := relaycommon.TaskSubmitReq{
		Prompt: "猫在草地上奔跑",
		Model:  "happyhorse-1.0-i2v",
		Images: []string{"https://example.com/cat.png"},
	}

	adaptor.convertHappyHorseRequest(aliReq, req)

	if len(aliReq.Input.Media) != 1 {
		t.Fatalf("media count mismatch: got %d, want 1", len(aliReq.Input.Media))
	}
	if aliReq.Input.Media[0].Type != "first_frame" {
		t.Errorf("media type mismatch: got %q, want %q", aliReq.Input.Media[0].Type, "first_frame")
	}
}

func TestConvertHappyHorseRequest_R2V(t *testing.T) {
	adaptor := &TaskAdaptor{}
	aliReq := &AliVideoRequest{Model: "happyhorse-1.0-r2v", Input: AliVideoInput{}, Parameters: &AliVideoParameters{}}

	req := relaycommon.TaskSubmitReq{
		Prompt: "[Image 1]中的人物在跳舞",
		Model:  "happyhorse-1.0-r2v",
		Images: []string{"https://example.com/ref1.png", "https://example.com/ref2.png"},
	}

	adaptor.convertHappyHorseRequest(aliReq, req)

	if len(aliReq.Input.Media) != 2 {
		t.Fatalf("media count mismatch: got %d, want 2", len(aliReq.Input.Media))
	}
	for i, m := range aliReq.Input.Media {
		if m.Type != "reference_image" {
			t.Errorf("media[%d] type mismatch: got %q, want %q", i, m.Type, "reference_image")
		}
	}
	if aliReq.Parameters.Ratio != "16:9" {
		t.Errorf("ratio mismatch: got %q, want %q", aliReq.Parameters.Ratio, "16:9")
	}
}

func TestConvertHappyHorseRequest_VideoEdit(t *testing.T) {
	adaptor := &TaskAdaptor{}
	aliReq := &AliVideoRequest{Model: "happyhorse-1.0-video-edit", Input: AliVideoInput{}, Parameters: &AliVideoParameters{}}

	req := relaycommon.TaskSubmitReq{
		Prompt:         "将视频风格改为水墨画",
		Model:          "happyhorse-1.0-video-edit",
		InputReference: "https://example.com/input-video.mp4",
		Images:         []string{"https://example.com/ref.png"},
	}

	adaptor.convertHappyHorseRequest(aliReq, req)

	if aliReq.Input.VideoURL != "https://example.com/input-video.mp4" {
		t.Errorf("video_url mismatch: got %q", aliReq.Input.VideoURL)
	}
	if len(aliReq.Input.Media) != 1 {
		t.Fatalf("media count mismatch: got %d, want 1", len(aliReq.Input.Media))
	}
	if aliReq.Input.Media[0].Type != "reference_image" {
		t.Errorf("media type mismatch: got %q, want %q", aliReq.Input.Media[0].Type, "reference_image")
	}
}

func TestConvertHappyHorseRequest_SizeResolution(t *testing.T) {
	adaptor := &TaskAdaptor{}

	tests := []struct {
		name               string
		size               string
		expectedRatio      string
		expectedResolution string
	}{
		{"ratio format", "9:16", "9:16", "1080P"},
		{"resolution format", "720P", "16:9", "720P"},
		{"pixel size format", "1920*1080", "16:9", "1080P"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aliReq := &AliVideoRequest{Model: "happyhorse-1.0-t2v", Input: AliVideoInput{}, Parameters: &AliVideoParameters{}}
			req := relaycommon.TaskSubmitReq{
				Prompt: "test",
				Model:  "happyhorse-1.0-t2v",
				Size:   tt.size,
			}
			adaptor.convertHappyHorseRequest(aliReq, req)
			if aliReq.Parameters.Ratio != tt.expectedRatio {
				t.Errorf("ratio mismatch: got %q, want %q", aliReq.Parameters.Ratio, tt.expectedRatio)
			}
			if aliReq.Parameters.Resolution != tt.expectedResolution {
				t.Errorf("resolution mismatch: got %q, want %q", aliReq.Parameters.Resolution, tt.expectedResolution)
			}
		})
	}
}

func TestConvertWanRequest(t *testing.T) {
	adaptor := &TaskAdaptor{}
	aliReq := &AliVideoRequest{Model: "wan2.5-i2v-preview", Input: AliVideoInput{}, Parameters: &AliVideoParameters{}}

	req := relaycommon.TaskSubmitReq{
		Prompt:         "测试",
		Model:          "wan2.5-i2v-preview",
		InputReference: "https://example.com/cat.png",
	}

	adaptor.convertWanRequest(aliReq, req)

	if aliReq.Input.ImgURL != "https://example.com/cat.png" {
		t.Errorf("img_url mismatch: got %q", aliReq.Input.ImgURL)
	}
	if !aliReq.Parameters.PromptExtend {
		t.Errorf("prompt_extend should be true for wan models")
	}
	if aliReq.Parameters.Resolution != "1080P" {
		t.Errorf("resolution mismatch: got %q, want %q", aliReq.Parameters.Resolution, "1080P")
	}
}

func TestProcessAliOtherRatios_HappyHorse(t *testing.T) {
	tests := []struct {
		model         string
		resolution    string
		expectedKey   string
		expectedValue float64
	}{
		{"happyhorse-1.0-t2v", "1080P", "resolution-1080P", 1.6 / 0.9},
		{"happyhorse-1.0-i2v", "720P", "resolution-720P", 1.0},
		{"happyhorse-1.0-r2v", "1080P", "resolution-1080P", 1.6 / 0.9},
		{"happyhorse-1.0-video-edit", "720P", "resolution-720P", 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.model+"_"+tt.resolution, func(t *testing.T) {
			aliReq := &AliVideoRequest{
				Model: tt.model,
				Parameters: &AliVideoParameters{
					Resolution: tt.resolution,
				},
			}
			ratios, err := ProcessAliOtherRatios(aliReq)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			val, ok := ratios[tt.expectedKey]
			if !ok {
				t.Fatalf("key %q not found in ratios", tt.expectedKey)
			}
			if val != tt.expectedValue {
				t.Errorf("ratio mismatch: got %v, want %v", val, tt.expectedValue)
			}
		})
	}
}

// ============================
// 万相3.0 (wan3.0-video) 适配测试
// ============================

func TestIsWan3Model(t *testing.T) {
	tests := []struct {
		model    string
		expected bool
	}{
		{"wan3.0-video", true},
		{"wan3.0-video-prime", true},
		{"wan2.5-i2v-preview", false},
		{"happyhorse-1.0-t2v", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			if result := isWan3Model(tt.model); result != tt.expected {
				t.Errorf("isWan3Model(%q) = %v, want %v", tt.model, result, tt.expected)
			}
		})
	}
}

func TestConvertWan3Request_T2V(t *testing.T) {
	adaptor := &TaskAdaptor{}
	aliReq := &AliVideoRequest{Model: "wan3.0-video", Input: AliVideoInput{}, Parameters: &AliVideoParameters{}}

	req := relaycommon.TaskSubmitReq{
		Prompt: "一只小猫在月光下的屋顶上奔跑",
		Model:  "wan3.0-video",
	}

	adaptor.convertWan3Request(aliReq, req)

	if aliReq.Input.Prompt != "一只小猫在月光下的屋顶上奔跑" {
		t.Errorf("prompt mismatch: got %q", aliReq.Input.Prompt)
	}
	if len(aliReq.Input.Media) != 0 {
		t.Errorf("media should be empty for t2v, got %d items", len(aliReq.Input.Media))
	}
	if !aliReq.Parameters.PromptExtend {
		t.Errorf("prompt_extend should be true for wan3.0 models")
	}
	if aliReq.Parameters.Watermark {
		t.Errorf("watermark should be false by default")
	}
	if aliReq.Parameters.Duration != 5 {
		t.Errorf("duration mismatch: got %d, want 5 (default)", aliReq.Parameters.Duration)
	}
	if aliReq.Parameters.Resolution != "" || aliReq.Parameters.Ratio != "" {
		t.Errorf("resolution/ratio should be unset when size is empty, got %q / %q",
			aliReq.Parameters.Resolution, aliReq.Parameters.Ratio)
	}
}

func TestConvertWan3Request_I2V(t *testing.T) {
	adaptor := &TaskAdaptor{}
	aliReq := &AliVideoRequest{Model: "wan3.0-video", Input: AliVideoInput{}, Parameters: &AliVideoParameters{}}

	req := relaycommon.TaskSubmitReq{
		Prompt: "女孩在雪地中行走",
		Model:  "wan3.0-video",
		Images: []string{"https://example.com/first-frame.png"},
	}

	adaptor.convertWan3Request(aliReq, req)

	if len(aliReq.Input.Media) != 1 {
		t.Fatalf("media count mismatch: got %d, want 1", len(aliReq.Input.Media))
	}
	if aliReq.Input.Media[0].Type != "first_frame" {
		t.Errorf("media type mismatch: got %q, want %q", aliReq.Input.Media[0].Type, "first_frame")
	}
	if aliReq.Input.Media[0].URL != "https://example.com/first-frame.png" {
		t.Errorf("media url mismatch: got %q", aliReq.Input.Media[0].URL)
	}
	// wan3.0 不使用 wan2.x 的 img_url 字段
	if aliReq.Input.ImgURL != "" {
		t.Errorf("img_url should be empty for wan3.0, got %q", aliReq.Input.ImgURL)
	}
}

func TestConvertWan3Request_R2V(t *testing.T) {
	adaptor := &TaskAdaptor{}
	aliReq := &AliVideoRequest{Model: "wan3.0-video-prime", Input: AliVideoInput{}, Parameters: &AliVideoParameters{}}

	req := relaycommon.TaskSubmitReq{
		Prompt: "图1拿着图2，在图3的椅子上弹奏",
		Model:  "wan3.0-video-prime",
		Images: []string{"https://example.com/ref1.png", "https://example.com/ref2.png", "https://example.com/ref3.png"},
	}

	adaptor.convertWan3Request(aliReq, req)

	if len(aliReq.Input.Media) != 3 {
		t.Fatalf("media count mismatch: got %d, want 3", len(aliReq.Input.Media))
	}
	for i, m := range aliReq.Input.Media {
		if m.Type != "reference_image" {
			t.Errorf("media[%d] type mismatch: got %q, want %q", i, m.Type, "reference_image")
		}
		if m.URL != req.Images[i] {
			t.Errorf("media[%d] url mismatch: got %q, want %q", i, m.URL, req.Images[i])
		}
	}
}

func TestConvertWan3Request_Size(t *testing.T) {
	adaptor := &TaskAdaptor{}

	tests := []struct {
		name               string
		size               string
		expectedRatio      string
		expectedResolution string
	}{
		{"official ratio", "9:16", "9:16", ""},
		{"official ratio 4:3", "4:3", "4:3", ""},
		{"hd resolution", "1080P", "", "1080P"},
		{"720 resolution", "720P", "", "720P"},
		{"pixel 1080p", "1920*1080", "16:9", "1080P"},
		{"pixel 720p", "1280x720", "16:9", "720P"},
		{"square pixel", "1024*1024", "1:1", "480P"},
		{"portrait pixel", "1080*1920", "9:16", "1080P"},
		{"empty", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aliReq := &AliVideoRequest{Model: "wan3.0-video", Input: AliVideoInput{}, Parameters: &AliVideoParameters{}}
			req := relaycommon.TaskSubmitReq{
				Prompt: "test",
				Model:  "wan3.0-video",
				Size:   tt.size,
			}
			adaptor.convertWan3Request(aliReq, req)
			if aliReq.Parameters.Ratio != tt.expectedRatio {
				t.Errorf("ratio mismatch: got %q, want %q", aliReq.Parameters.Ratio, tt.expectedRatio)
			}
			if aliReq.Parameters.Resolution != tt.expectedResolution {
				t.Errorf("resolution mismatch: got %q, want %q", aliReq.Parameters.Resolution, tt.expectedResolution)
			}
		})
	}
}

func TestConvertWan3Request_Duration(t *testing.T) {
	adaptor := &TaskAdaptor{}

	tests := []struct {
		name     string
		duration int
		seconds  string
		expected int
	}{
		{"default", 0, "", 5},
		{"smart duration", -1, "", -1},
		{"max 30", 30, "", 30},
		{"clamp below min", 1, "", 2},
		{"clamp above max", 45, "", 30},
		{"seconds string", 0, "15", 15},
		{"seconds invalid", 0, "abc", 5},
		{"duration wins over seconds", 10, "15", 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aliReq := &AliVideoRequest{Model: "wan3.0-video", Input: AliVideoInput{}, Parameters: &AliVideoParameters{}}
			req := relaycommon.TaskSubmitReq{
				Prompt:   "test",
				Model:    "wan3.0-video",
				Duration: tt.duration,
				Seconds:  tt.seconds,
			}
			adaptor.convertWan3Request(aliReq, req)
			if aliReq.Parameters.Duration != tt.expected {
				t.Errorf("duration mismatch: got %d, want %d", aliReq.Parameters.Duration, tt.expected)
			}
		})
	}
}

func TestProcessAliOtherRatios_Wan3(t *testing.T) {
	tests := []struct {
		model         string
		resolution    string
		expectedValue float64
	}{
		{"wan3.0-video", "480P", 1.0},
		{"wan3.0-video", "720P", 2.0},
		{"wan3.0-video", "1080P", 4.0},
		{"wan3.0-video-prime", "1080P", 4.0},
	}

	for _, tt := range tests {
		t.Run(tt.model+"_"+tt.resolution, func(t *testing.T) {
			aliReq := &AliVideoRequest{
				Model: tt.model,
				Parameters: &AliVideoParameters{
					Resolution: tt.resolution,
				},
			}
			ratios, err := ProcessAliOtherRatios(aliReq)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			key := "resolution-" + tt.resolution
			val, ok := ratios[key]
			if !ok {
				t.Fatalf("key %q not found in ratios", key)
			}
			if val != tt.expectedValue {
				t.Errorf("ratio mismatch: got %v, want %v", val, tt.expectedValue)
			}
		})
	}
}

// ============================
// ConvertToOpenAIVideo：状态与 URL 必须取数据库实时字段
// （创建时保存的 task.Data 仅含 PENDING 且无 video_url）
// ============================

func TestConvertToOpenAIVideo_Success(t *testing.T) {
	task := &model.Task{
		TaskID:     "task_test_001",
		Status:     model.TaskStatusSuccess,
		Progress:   "100%",
		CreatedAt:  1000,
		UpdatedAt:  2000,
		Properties: model.Properties{OriginModelName: "wan3.0-video"},
	}
	task.PrivateData.ResultURL = "https://example.com/result.mp4"

	adaptor := &TaskAdaptor{}
	body, err := adaptor.ConvertToOpenAIVideo(task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var openAIResp dto.OpenAIVideo
	if err := common.Unmarshal(body, &openAIResp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if openAIResp.ID != "task_test_001" {
		t.Errorf("id mismatch: got %q", openAIResp.ID)
	}
	if openAIResp.Status != dto.VideoStatusCompleted {
		t.Errorf("status mismatch: got %q, want %q", openAIResp.Status, dto.VideoStatusCompleted)
	}
	url, _ := openAIResp.Metadata["url"].(string)
	if url != "https://example.com/result.mp4" {
		t.Errorf("metadata url mismatch: got %q", url)
	}
}

func TestConvertToOpenAIVideo_InProgress(t *testing.T) {
	task := &model.Task{
		TaskID:     "task_test_002",
		Status:     model.TaskStatusInProgress,
		Progress:   "45%",
		Properties: model.Properties{OriginModelName: "wan3.0-video"},
	}

	adaptor := &TaskAdaptor{}
	body, err := adaptor.ConvertToOpenAIVideo(task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var openAIResp dto.OpenAIVideo
	if err := common.Unmarshal(body, &openAIResp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if openAIResp.Status != dto.VideoStatusInProgress {
		t.Errorf("status mismatch: got %q, want %q", openAIResp.Status, dto.VideoStatusInProgress)
	}
	if openAIResp.Progress != 45 {
		t.Errorf("progress mismatch: got %d, want 45", openAIResp.Progress)
	}
}

func TestConvertToOpenAIVideo_Queued(t *testing.T) {
	// 创建时仅保存 PENDING 响应；即使 task.Data 里是 PENDING，
	// 状态也必须取数据库实时字段（此时为 SUBMITTED → queued）
	task := &model.Task{
		TaskID:     "task_test_003",
		Status:     model.TaskStatusSubmitted,
		Data:       []byte(`{"output":{"task_status":"PENDING","task_id":"upstream-001"},"request_id":"r1"}`),
		Properties: model.Properties{OriginModelName: "wan3.0-video"},
	}

	adaptor := &TaskAdaptor{}
	body, err := adaptor.ConvertToOpenAIVideo(task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var openAIResp dto.OpenAIVideo
	if err := common.Unmarshal(body, &openAIResp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if openAIResp.Status != dto.VideoStatusQueued {
		t.Errorf("status mismatch: got %q, want %q", openAIResp.Status, dto.VideoStatusQueued)
	}
}

func TestConvertToOpenAIVideo_Failed(t *testing.T) {
	task := &model.Task{
		TaskID:     "task_test_004",
		Status:     model.TaskStatusFailure,
		FailReason: "task failed, code: InvalidParameter",
		Properties: model.Properties{OriginModelName: "wan3.0-video"},
	}

	adaptor := &TaskAdaptor{}
	body, err := adaptor.ConvertToOpenAIVideo(task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var openAIResp dto.OpenAIVideo
	if err := common.Unmarshal(body, &openAIResp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if openAIResp.Status != dto.VideoStatusFailed {
		t.Errorf("status mismatch: got %q, want %q", openAIResp.Status, dto.VideoStatusFailed)
	}
	if openAIResp.Error == nil || openAIResp.Error.Message == "" {
		t.Errorf("error message should be set from FailReason, got %+v", openAIResp.Error)
	}
}

// ============================
// ParseTaskResult：兼容上游以数字形式返回字符串字段
// （中转/上游格式差异曾导致 unmarshal 失败、任务卡死）
// ============================

func TestParseTaskResultNumericStringFields(t *testing.T) {
	adaptor := &TaskAdaptor{}
	body := []byte(`{
		"request_id": 123456,
		"code": 0,
		"message": 0,
		"output": {
			"task_id": 987654,
			"task_status": "SUCCEEDED",
			"submit_time": 1756281599,
			"end_time": 1756281600,
			"video_url": "https://example.com/v.mp4"
		},
		"usage": {"duration": 5, "video_count": 1, "SR": 720}
	}`)

	result, err := adaptor.ParseTaskResult(body)
	if err != nil {
		t.Fatalf("ParseTaskResult should tolerate numeric string fields, got error: %v", err)
	}
	if result.Status != model.TaskStatusSuccess {
		t.Errorf("status mismatch: got %q, want %q", result.Status, model.TaskStatusSuccess)
	}
	if result.Url != "https://example.com/v.mp4" {
		t.Errorf("url mismatch: got %q", result.Url)
	}
}

func TestParseTaskResultFailedWithNumericCode(t *testing.T) {
	adaptor := &TaskAdaptor{}
	body := []byte(`{
		"request_id": 1,
		"code": 400,
		"message": 10086,
		"output": {"task_id": 555, "task_status": "FAILED", "code": 400, "message": 10086}
	}`)

	result, err := adaptor.ParseTaskResult(body)
	if err != nil {
		t.Fatalf("ParseTaskResult should tolerate numeric code/message, got error: %v", err)
	}
	if result.Status != model.TaskStatusFailure {
		t.Errorf("status mismatch: got %q, want %q", result.Status, model.TaskStatusFailure)
	}
}

func TestParseTaskResultUnwrapsDataEnvelopeFullResponse(t *testing.T) {
	adaptor := &TaskAdaptor{}
	// 中转信封：data 内为完整阿里响应，code 为数字
	body := []byte(`{
		"code": 0,
		"message": "success",
		"data": {
			"request_id": "r-1",
			"output": {"task_id": "t-1", "task_status": "SUCCEEDED", "video_url": "https://example.com/v.mp4"},
			"usage": {"duration": 5, "video_count": 1, "SR": 720}
		}
	}`)

	result, err := adaptor.ParseTaskResult(body)
	if err != nil {
		t.Fatalf("ParseTaskResult should unwrap data envelope, got error: %v", err)
	}
	if result.Status != model.TaskStatusSuccess {
		t.Errorf("status mismatch: got %q, want %q", result.Status, model.TaskStatusSuccess)
	}
	if result.Url != "https://example.com/v.mp4" {
		t.Errorf("url mismatch: got %q", result.Url)
	}
}

func TestParseTaskResultUnwrapsDataEnvelopeOutputOnly(t *testing.T) {
	adaptor := &TaskAdaptor{}
	// 中转信封：data 内仅为 output 对象
	body := []byte(`{
		"code": 0,
		"data": {"task_id": "t-2", "task_status": "RUNNING"}
	}`)

	result, err := adaptor.ParseTaskResult(body)
	if err != nil {
		t.Fatalf("ParseTaskResult should unwrap data envelope (output only), got error: %v", err)
	}
	if result.Status != model.TaskStatusInProgress {
		t.Errorf("status mismatch: got %q, want %q", result.Status, model.TaskStatusInProgress)
	}
}

func TestParseTaskResultErrorEnvelopeReturnsError(t *testing.T) {
	adaptor := &TaskAdaptor{}
	// 中转错误信封：无 task_status，仅有数字 code —— 应返回错误触发重试，而非回退状态
	body := []byte(`{"code": 429, "message": "too many requests"}`)

	if _, err := adaptor.ParseTaskResult(body); err == nil {
		t.Fatal("ParseTaskResult should return error for envelope without task_status")
	}
}

// TestParseTaskResultExactUpstreamBody 回归测试：2026-08-27 真实上游响应原文
// （逐字节复制自线上轮询日志）。注意 usage.duration / output_video_duration
// 为浮点数 10.0——dto.IntValue 不支持浮点时曾报
// "cannot unmarshal number into Go value of type string"，导致任务卡死。
func TestParseTaskResultExactUpstreamBody(t *testing.T) {
	adaptor := &TaskAdaptor{}
	body := []byte(`{"request_id":"70d7aa51-f4ea-944e-a930-198edaab9be3","output":{"task_id":"fdd50dcc-7411-4a20-969c-f1e5f7d334c6","task_status":"SUCCEEDED","submit_time":"2026-08-27 15:59:59.299","scheduled_time":"2026-08-27 15:59:59.335","end_time":"2026-08-27 16:02:09.980","orig_prompt":"一个可爱的小宝宝开心地滚来滚去，小手小脚自然挥动，表情天真愉悦。镜头从低角度缓慢环绕跟拍，捕捉宝宝连续翻滚的动作与衣物褶皱变化。","video_url":"https://dashscope-a717.oss-accelerate.aliyuncs.com/1d/1e/20260827/e4b11e52/fdd50dcc-7411-4a20-969c-f1e5f7d334c6.mp4?Expires=1787904126&OSSAccessKeyId=LTAI5tPxpiCM2hjmWrFXrym1&Signature=psCcNS9Tz3Ftwz8vUmbhRzYIJ%2Bo%3D"},"usage":{"duration":10.0,"input_video_duration":0,"output_video_duration":10.0,"fps":30,"video_count":1,"SR":1080,"ratio":"3368:3409"}}`)

	result, err := adaptor.ParseTaskResult(body)
	if err != nil {
		t.Fatalf("ParseTaskResult failed on exact upstream body: %v", err)
	}
	if result.Status != model.TaskStatusSuccess {
		t.Errorf("status mismatch: got %q, want %q", result.Status, model.TaskStatusSuccess)
	}
	if result.Url == "" {
		t.Error("url should be set from video_url")
	}

	// usage 浮点字段应被 dto.IntValue 正确截断为整数
	var aliResp AliVideoResponse
	if err := common.Unmarshal(body, &aliResp); err != nil {
		t.Fatalf("unmarshal AliVideoResponse failed: %v", err)
	}
	if aliResp.Usage == nil {
		t.Fatal("usage should be parsed")
	}
	if int(aliResp.Usage.Duration) != 10 {
		t.Errorf("usage.duration mismatch: got %d, want 10", int(aliResp.Usage.Duration))
	}
	if int(aliResp.Usage.SR) != 1080 {
		t.Errorf("usage.SR mismatch: got %d, want 1080", int(aliResp.Usage.SR))
	}
}

// ============================================================================
// 百炼第三方托管 MiniMax（MiniMax/MiniMax-H3）视频生成
// ============================================================================

func newMiniMaxAliReq() *AliVideoRequest {
	return &AliVideoRequest{
		Model:      "MiniMax/MiniMax-H3",
		Input:      AliVideoInput{},
		Parameters: &AliVideoParameters{},
	}
}

func mediaTypesOf(media []AliMedia) []string {
	types := make([]string, len(media))
	for i, m := range media {
		types[i] = m.Type
	}
	return types
}

// newMiniMaxRelayInfo 构造可用的 RelayInfo。
// IsModelMapped / UpstreamModelName 位于匿名嵌入的 *ChannelMeta 上，
// 指针为 nil 时读取会 panic，因此必须显式初始化。
func newMiniMaxRelayInfo(isMapped bool, upstreamModel string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			IsModelMapped:     isMapped,
			UpstreamModelName: upstreamModel,
		},
	}
}

func TestIsMiniMaxModel(t *testing.T) {
	tests := []struct {
		model    string
		expected bool
	}{
		{"MiniMax/MiniMax-H3", true},
		{"MiniMax-H3", true},
		{"minimax-h3", true},
		{"MiniMax/MiniMax-H3-Fast", true},
		{"wan3.0-video", false},
		{"wan2.5-i2v-preview", false},
		{"happyhorse-1.0-t2v", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			if got := isMiniMaxModel(tt.model); got != tt.expected {
				t.Errorf("isMiniMaxModel(%q) = %v, want %v", tt.model, got, tt.expected)
			}
		})
	}
}

func TestConvertMiniMaxRequest_T2V(t *testing.T) {
	adaptor := &TaskAdaptor{}
	aliReq := newMiniMaxAliReq()

	adaptor.convertMiniMaxRequest(aliReq, relaycommon.TaskSubmitReq{
		Model:  "MiniMax/MiniMax-H3",
		Prompt: "史诗级太空歌剧院线预告",
	})

	if aliReq.Input.Prompt != "史诗级太空歌剧院线预告" {
		t.Errorf("prompt mismatch: got %q", aliReq.Input.Prompt)
	}
	if len(aliReq.Input.Media) != 0 {
		t.Errorf("文生视频不应有 media, got %v", mediaTypesOf(aliReq.Input.Media))
	}
	if aliReq.Parameters.Resolution != "768P" {
		t.Errorf("resolution mismatch: got %q, want 768P", aliReq.Parameters.Resolution)
	}
	if aliReq.Parameters.Ratio != "16:9" {
		t.Errorf("ratio mismatch: got %q, want 16:9（文生视频必填且不得为 adaptive）", aliReq.Parameters.Ratio)
	}
	if aliReq.Parameters.Duration != 5 {
		t.Errorf("duration mismatch: got %d, want 5", aliReq.Parameters.Duration)
	}
}

func TestConvertMiniMaxRequest_T2V_SizeRatio(t *testing.T) {
	tests := []struct {
		name           string
		size           string
		wantRatio      string
		wantResolution string
	}{
		{"比例串直接透传", "9:16", "9:16", "768P"},
		{"21:9 超宽比例", "21:9", "21:9", "768P"},
		{"像素尺寸换算比例与档位", "1920*1080", "16:9", "2K"},
		{"竖屏像素尺寸", "1080*1920", "9:16", "2K"},
		{"小尺寸仅影响档位", "832*480", "16:9", "768P"},
		{"非法比例回落默认", "5:3", "16:9", "768P"},
		{"adaptive 文生视频回落", "adaptive", "16:9", "768P"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adaptor := &TaskAdaptor{}
			aliReq := newMiniMaxAliReq()
			adaptor.convertMiniMaxRequest(aliReq, relaycommon.TaskSubmitReq{
				Model:  "MiniMax/MiniMax-H3",
				Prompt: "p",
				Size:   tt.size,
			})
			if aliReq.Parameters.Ratio != tt.wantRatio {
				t.Errorf("ratio mismatch: got %q, want %q", aliReq.Parameters.Ratio, tt.wantRatio)
			}
			if aliReq.Parameters.Resolution != tt.wantResolution {
				t.Errorf("resolution mismatch: got %q, want %q", aliReq.Parameters.Resolution, tt.wantResolution)
			}
		})
	}
}

func TestConvertMiniMaxRequest_I2V_SingleImage(t *testing.T) {
	adaptor := &TaskAdaptor{}
	aliReq := newMiniMaxAliReq()

	adaptor.convertMiniMaxRequest(aliReq, relaycommon.TaskSubmitReq{
		Model:  "MiniMax/MiniMax-H3",
		Prompt: "让图片中的人物动起来",
		Images: []string{"https://example.com/a.webp"},
	})

	if len(aliReq.Input.Media) != 1 {
		t.Fatalf("media count mismatch: got %d, want 1", len(aliReq.Input.Media))
	}
	if aliReq.Input.Media[0].Type != "first_frame" {
		t.Errorf("media type mismatch: got %q, want first_frame", aliReq.Input.Media[0].Type)
	}
	if aliReq.Input.Media[0].URL != "https://example.com/a.webp" {
		t.Errorf("media url mismatch: got %q", aliReq.Input.Media[0].URL)
	}
	// 文档：图生视频宽高比由输入图片决定，恒为 adaptive
	if aliReq.Parameters.Ratio != "adaptive" {
		t.Errorf("ratio mismatch: got %q, want adaptive", aliReq.Parameters.Ratio)
	}
}

func TestConvertMiniMaxRequest_FirstLastFrame(t *testing.T) {
	adaptor := &TaskAdaptor{}
	aliReq := newMiniMaxAliReq()

	adaptor.convertMiniMaxRequest(aliReq, relaycommon.TaskSubmitReq{
		Model:  "MiniMax/MiniMax-H3",
		Prompt: "首尾帧生视频",
		Images: []string{"https://example.com/first.png", "https://example.com/last.png"},
	})

	want := []string{"first_frame", "last_frame"}
	got := mediaTypesOf(aliReq.Input.Media)
	if len(got) != len(want) {
		t.Fatalf("media types mismatch: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("media[%d] type mismatch: got %q, want %q", i, got[i], want[i])
		}
	}
	if aliReq.Parameters.Ratio != "adaptive" {
		t.Errorf("ratio mismatch: got %q, want adaptive", aliReq.Parameters.Ratio)
	}
}

func TestConvertMiniMaxRequest_ReferenceImages(t *testing.T) {
	adaptor := &TaskAdaptor{}
	aliReq := newMiniMaxAliReq()

	adaptor.convertMiniMaxRequest(aliReq, relaycommon.TaskSubmitReq{
		Model:  "MiniMax/MiniMax-H3",
		Prompt: "多模态参考生视频",
		Images: []string{"https://example.com/1.jpg", "https://example.com/2.jpg", "https://example.com/3.jpg", "https://example.com/4.jpg"},
	})

	for _, m := range aliReq.Input.Media {
		if m.Type != "image_url" {
			t.Fatalf("≥3 张图应全部映射为 image_url, got %v", mediaTypesOf(aliReq.Input.Media))
		}
	}
	if len(aliReq.Input.Media) != 4 {
		t.Errorf("media count mismatch: got %d, want 4", len(aliReq.Input.Media))
	}
	// 参考模式默认 adaptive，可显式指定
	if aliReq.Parameters.Ratio != "adaptive" {
		t.Errorf("ratio mismatch: got %q, want adaptive", aliReq.Parameters.Ratio)
	}
}

func TestConvertMiniMaxRequest_MixedDegrade(t *testing.T) {
	adaptor := &TaskAdaptor{}
	aliReq := newMiniMaxAliReq()

	adaptor.convertMiniMaxRequest(aliReq, relaycommon.TaskSubmitReq{
		Model:  "MiniMax/MiniMax-H3",
		Prompt: "参考视频中的角色缓缓转头",
		Images: []string{"https://example.com/first.png"},
		Metadata: map[string]interface{}{
			"reference_videos": []any{"https://example.com/ref.mp4"},
		},
	})

	// 百炼规定首尾帧与参考素材互斥：存在参考素材时剔除首帧，降级为参考模式
	got := mediaTypesOf(aliReq.Input.Media)
	if len(got) != 1 || got[0] != "feature" {
		t.Fatalf("互斥降级失败: got %v, want [feature]", got)
	}
	if aliReq.Parameters.Ratio != "adaptive" {
		t.Errorf("ratio mismatch: got %q, want adaptive", aliReq.Parameters.Ratio)
	}
}

func TestConvertMiniMaxRequest_MetadataMediaPassthrough(t *testing.T) {
	adaptor := &TaskAdaptor{}
	info := newMiniMaxRelayInfo(false, "")
	req := relaycommon.TaskSubmitReq{
		Model:  "MiniMax/MiniMax-H3",
		Prompt: "尾帧生视频",
		Images: []string{"https://example.com/auto-first.png"},
		Metadata: map[string]interface{}{
			"input": map[string]interface{}{
				"media": []any{
					map[string]interface{}{"type": "last_frame", "url": "https://example.com/specified.png"},
				},
			},
			"parameters": map[string]interface{}{"resolution": "2K"},
		},
	}

	aliReq, err := adaptor.convertToAliRequest(info, req)
	if err != nil {
		t.Fatalf("convertToAliRequest failed: %v", err)
	}
	// metadata.input.media 优先级最高，原样覆盖自动映射结果
	if len(aliReq.Input.Media) != 1 || aliReq.Input.Media[0].Type != "last_frame" {
		t.Fatalf("metadata.input.media 未生效: got %v", mediaTypesOf(aliReq.Input.Media))
	}
	if aliReq.Input.Media[0].URL != "https://example.com/specified.png" {
		t.Errorf("media url mismatch: got %q", aliReq.Input.Media[0].URL)
	}
	// metadata.parameters.resolution 同样覆盖 size 推导结果
	if aliReq.Parameters.Resolution != "2K" {
		t.Errorf("resolution mismatch: got %q, want 2K", aliReq.Parameters.Resolution)
	}
}

func TestConvertMiniMaxRequest_MediaLimits(t *testing.T) {
	adaptor := &TaskAdaptor{}
	aliReq := newMiniMaxAliReq()

	images := make([]string, 12)
	for i := range images {
		images[i] = "https://example.com/img" + string(rune('a'+i)) + ".jpg"
	}
	videos := []any{
		"https://example.com/1.mp4", "https://example.com/2.mp4",
		"https://example.com/3.mp4", "https://example.com/4.mp4",
	}

	adaptor.convertMiniMaxRequest(aliReq, relaycommon.TaskSubmitReq{
		Model:  "MiniMax/MiniMax-H3",
		Prompt: "p",
		Images: images,
		Metadata: map[string]interface{}{
			"reference_videos": videos,
		},
	})

	var imgCount, vidCount int
	for _, m := range aliReq.Input.Media {
		switch m.Type {
		case "image_url":
			imgCount++
		case "feature":
			vidCount++
		default:
			t.Errorf("意外素材类型: %q", m.Type)
		}
	}
	if imgCount != 9 {
		t.Errorf("参考图应截断为 9 张, got %d", imgCount)
	}
	if vidCount != 3 {
		t.Errorf("参考视频应截断为 3 个, got %d", vidCount)
	}
}

func TestNormalizeMiniMaxDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration int
		seconds  string
		want     int
	}{
		{"未指定回落默认", 0, "", 5},
		{"智能时长 -1 不支持", -1, "", 5},
		{"0 秒", 0, "0", 5},
		{"小于下限", 3, "", 4},
		{"下限", 4, "", 4},
		{"区间内", 8, "", 8},
		{"上限", 15, "", 15},
		{"超上限", 20, "", 15},
		{"seconds 回落", 0, "10", 10},
		{"duration 优先于 seconds", 6, "10", 6},
		{"seconds 非法字符串", 0, "abc", 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeMiniMaxDuration(relaycommon.TaskSubmitReq{Duration: tt.duration, Seconds: tt.seconds})
			if got != tt.want {
				t.Errorf("normalizeMiniMaxDuration(duration=%d, seconds=%q) = %d, want %d", tt.duration, tt.seconds, got, tt.want)
			}
		})
	}
}

func TestMiniMaxResolutionFromSize(t *testing.T) {
	tests := []struct {
		size string
		want string
	}{
		{"2K", "2K"},
		{"2k", "2K"},
		{"1080P", "2K"},
		{"1920*1080", "2K"},
		{"1080*1920", "2K"},
		{"2560x1440", "2K"},
		{"768P", "768P"},
		{"720P", "768P"},
		{"512P", "768P"},
		{"480P", "768P"},
		{"832*480", "768P"},
		{"1280*720", "768P"},
		{"16:9", ""},
		{"", ""},
		{"abc", ""},
	}

	for _, tt := range tests {
		t.Run(tt.size, func(t *testing.T) {
			if got := miniMaxResolutionFromSize(tt.size); got != tt.want {
				t.Errorf("miniMaxResolutionFromSize(%q) = %q, want %q", tt.size, got, tt.want)
			}
		})
	}
}

func TestProcessAliOtherRatios_MiniMax2K(t *testing.T) {
	// 回归保护：归一化曾把 2K 补成 2KP，导致档位倍率永远查不到
	aliReq := &AliVideoRequest{
		Model:      "MiniMax/MiniMax-H3",
		Parameters: &AliVideoParameters{Resolution: "2K", Duration: 5},
	}

	ratios, err := ProcessAliOtherRatios(aliReq)
	if err != nil {
		t.Fatalf("ProcessAliOtherRatios failed: %v", err)
	}
	if _, ok := ratios["resolution-2KP"]; ok {
		t.Error("resolution key 不应带 P 后缀")
	}
	if ratio, ok := ratios["resolution-2K"]; !ok || ratio != 2 {
		t.Errorf("resolution-2K mismatch: got %v (present=%v), want 2", ratio, ok)
	}
}

func TestProcessAliOtherRatios_MiniMax768P(t *testing.T) {
	aliReq := &AliVideoRequest{
		Model:      "MiniMax/MiniMax-H3",
		Parameters: &AliVideoParameters{Resolution: "768P", Duration: 8},
	}

	ratios, err := ProcessAliOtherRatios(aliReq)
	if err != nil {
		t.Fatalf("ProcessAliOtherRatios failed: %v", err)
	}
	if ratio, ok := ratios["resolution-768P"]; !ok || ratio != 1 {
		t.Errorf("resolution-768P mismatch: got %v (present=%v), want 1", ratio, ok)
	}
}

func TestProcessAliOtherRatios_WanUnaffected(t *testing.T) {
	// 回归保护：MiniMax 引入的 K 后缀豁免不得影响万相档位
	aliReq := &AliVideoRequest{
		Model:      "wan2.5-i2v-preview",
		Parameters: &AliVideoParameters{Resolution: "1080P", Duration: 5},
	}

	ratios, err := ProcessAliOtherRatios(aliReq)
	if err != nil {
		t.Fatalf("ProcessAliOtherRatios failed: %v", err)
	}
	ratio, ok := ratios["resolution-1080P"]
	if !ok {
		t.Fatal("resolution-1080P key missing")
	}
	if diff := ratio - 1/0.3; diff > 1e-9 && diff < -1e-9 {
		t.Errorf("wan2.5 1080P ratio mismatch: got %v, want %v", ratio, 1/0.3)
	}
}

func TestConvertMiniMaxRequest_NoUnsupportedFields(t *testing.T) {
	adaptor := &TaskAdaptor{}
	aliReq := newMiniMaxAliReq()

	adaptor.convertMiniMaxRequest(aliReq, relaycommon.TaskSubmitReq{
		Model:  "MiniMax/MiniMax-H3",
		Prompt: "一只猫在草地上奔跑",
		Images: []string{"https://example.com/a.png"},
	})

	body, err := common.Marshal(aliReq)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	jsonStr := string(body)

	// MiniMax 不接受万相专有字段，出现即会被上游拒绝
	for _, forbidden := range []string{`"size"`, `"prompt_extend"`, `"img_url"`, `"audio"`, `"seed"`, `"negative_prompt"`, `"template"`, `"video_url"`} {
		if strings.Contains(jsonStr, forbidden) {
			t.Errorf("请求体不应包含 MiniMax 不支持的字段 %s, body=%s", forbidden, jsonStr)
		}
	}
	for _, required := range []string{`"resolution"`, `"ratio"`, `"duration"`, `"prompt"`, `"media"`} {
		if !strings.Contains(jsonStr, required) {
			t.Errorf("请求体应包含字段 %s, body=%s", required, jsonStr)
		}
	}
}

func TestConvertToAliRequest_MiniMaxBranch(t *testing.T) {
	adaptor := &TaskAdaptor{}
	info := newMiniMaxRelayInfo(false, "")

	aliReq, err := adaptor.convertToAliRequest(info, relaycommon.TaskSubmitReq{
		Model:  "MiniMax/MiniMax-H3",
		Prompt: "p",
	})
	if err != nil {
		t.Fatalf("convertToAliRequest failed: %v", err)
	}
	// 分支路由生效：走 MiniMax 而非 wan 兜底（wan 兜底会给出 720P + prompt_extend）
	if aliReq.Parameters.Resolution != "768P" {
		t.Errorf("resolution mismatch: got %q, want 768P（说明未走 MiniMax 分支）", aliReq.Parameters.Resolution)
	}
	if aliReq.Parameters.Ratio != "16:9" {
		t.Errorf("ratio mismatch: got %q, want 16:9", aliReq.Parameters.Ratio)
	}
	if aliReq.Parameters.PromptExtend {
		t.Error("MiniMax 分支不应设置 prompt_extend")
	}
	if aliReq.Model != "MiniMax/MiniMax-H3" {
		t.Errorf("model mismatch: got %q", aliReq.Model)
	}
}

func TestConvertToAliRequest_MiniMaxModelMapped(t *testing.T) {
	adaptor := &TaskAdaptor{}
	info := newMiniMaxRelayInfo(true, "MiniMax/MiniMax-H3")

	aliReq, err := adaptor.convertToAliRequest(info, relaycommon.TaskSubmitReq{
		Model:  "minimax-h3",
		Prompt: "p",
	})
	if err != nil {
		t.Fatalf("convertToAliRequest failed: %v", err)
	}
	if aliReq.Model != "MiniMax/MiniMax-H3" {
		t.Errorf("模型映射后应使用上游模型名, got %q", aliReq.Model)
	}
	if aliReq.Parameters.Resolution != "768P" {
		t.Errorf("resolution mismatch: got %q, want 768P", aliReq.Parameters.Resolution)
	}
}

func TestConvertToAliRequest_MiniMaxMetadataCannotChangeModel(t *testing.T) {
	adaptor := &TaskAdaptor{}
	info := newMiniMaxRelayInfo(false, "")

	_, err := adaptor.convertToAliRequest(info, relaycommon.TaskSubmitReq{
		Model:  "MiniMax/MiniMax-H3",
		Prompt: "p",
		Metadata: map[string]interface{}{
			"model": "wan3.0-video",
		},
	})
	if err == nil {
		t.Fatal("metadata 改写 model 应被拒绝（计费绕过防护）")
	}
}
