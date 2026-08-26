package ali

import (
	"testing"

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
