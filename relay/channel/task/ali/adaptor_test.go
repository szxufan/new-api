package ali

import (
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
		model           string
		resolution      string
		expectedKey     string
		expectedValue   float64
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
		TaskID:   "task_test_003",
		Status:   model.TaskStatusSubmitted,
		Data:     []byte(`{"output":{"task_status":"PENDING","task_id":"upstream-001"},"request_id":"r1"}`),
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
// （usage 含多个数字字段），曾因 string 字段声明导致 unmarshal 失败、任务卡死。
func TestParseTaskResultExactUpstreamBody(t *testing.T) {
	adaptor := &TaskAdaptor{}
	body := []byte(`{
  "output": {
    "end_time": "2026-08-27 16:02:09.980",
    "orig_prompt": "一个可爱的小宝宝开心地滚来滚去，小手小脚自然挥动，表情天真愉悦。镜头从低角度缓慢环绕跟拍，捕捉宝宝连续翻滚的动作与衣物褶皱变化。",
    "scheduled_time": "2026-08-27 15:59:59.335",
    "submit_time": "2026-08-27 15:59:59.299",
    "task_id": "fdd50dcc-7411-4a20-969c-f1e5f7d334c6",
    "task_status": "SUCCEEDED",
    "video_url": "https://dashscope-a717.oss-accelerate.aliyuncs.com/1d/1e/20260827/e4b11e52/fdd50dcc-7411-4a20-969c-f1e5f7d334c6.mp4?Expires=1787904126&OSSAccessKeyId=LTAI5tPxpiCM2hjmWrFXrym1&Signature=psCcNS9Tz3Ftwz8vUmbhRzYIJ%2Bo%3D"
  },
  "request_id": "f989289b-2844-9865-8e52-61d63c454cae",
  "usage": {
    "SR": 1080,
    "duration": 10,
    "fps": 30,
    "input_video_duration": 0,
    "output_video_duration": 10,
    "ratio": "3368:3409",
    "video_count": 1
  }
}`)

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
}
