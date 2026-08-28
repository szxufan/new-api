package hailuo

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
)

// newV2RelayInfo 构造带模型名与公开任务 ID 的最小 RelayInfo
func newV2RelayInfo(upstreamModel string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		OriginModelName: upstreamModel,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: upstreamModel,
		},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{
			PublicTaskID: "task_public_0001",
			Action:       constant.TaskActionGenerate,
		},
	}
}

func TestIsVideoV2Model(t *testing.T) {
	tests := []struct {
		model    string
		expected bool
	}{
		{"MiniMax-H3", true},
		{"minimax-h3", true},
		{"MiniMax-H3-Turbo", true},
		{"  MiniMax-H3  ", true},
		{"MiniMax-Hailuo-2.3", false},
		{"MiniMax-Hailuo-02", false},
		{"T2V-01", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			if got := IsVideoV2Model(tt.model); got != tt.expected {
				t.Errorf("IsVideoV2Model(%q) = %v, want %v", tt.model, got, tt.expected)
			}
		})
	}
}

func TestNormalizeV2Resolution(t *testing.T) {
	tests := []struct {
		name     string
		size     string
		metadata map[string]any
		expected string
	}{
		{"empty falls back to default", "", nil, V2Resolution2K},
		{"1080P rounds up to 2K", "1080P", nil, V2Resolution2K},
		{"720P maps to 768P", "720P", nil, V2Resolution768P},
		{"480P maps to 768P", "480P", nil, V2Resolution768P},
		{"2K stays 2K", "2K", nil, V2Resolution2K},
		{"unknown value falls back to default", "8K", nil, V2Resolution2K},
		{"metadata resolution wins", "1080P", map[string]any{"resolution": "768P"}, V2Resolution768P},
		{"nested parameters.resolution supported", "", map[string]any{"parameters": map[string]any{"resolution": "768P"}}, V2Resolution768P},
		{"metadata 2k lowercase normalized", "", map[string]any{"resolution": "2k"}, V2Resolution2K},
		{"metadata invalid falls back to size", "720P", map[string]any{"resolution": "nonsense"}, V2Resolution768P},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeV2Resolution(tt.size, tt.metadata); got != tt.expected {
				t.Errorf("normalizeV2Resolution(%q) = %q, want %q", tt.size, got, tt.expected)
			}
		})
	}
}

func TestClampV2Duration(t *testing.T) {
	tests := []struct {
		name     string
		duration int
		metadata map[string]any
		expected int
	}{
		{"zero uses default", 0, nil, V2DefaultDuration},
		{"smart duration -1 uses default", -1, nil, V2DefaultDuration},
		{"below min clamps up", 3, nil, V2DurationMin},
		{"above max clamps down", 30, nil, V2DurationMax},
		{"in range kept", 7, nil, 7},
		{"metadata overrides", 7, map[string]any{"duration": float64(12)}, 12},
		{"metadata string number", 7, map[string]any{"duration": "9"}, 9},
		{"metadata out of range clamped", 7, map[string]any{"duration": float64(99)}, V2DurationMax},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clampV2Duration(tt.duration, tt.metadata); got != tt.expected {
				t.Errorf("clampV2Duration(%d) = %d, want %d", tt.duration, got, tt.expected)
			}
		})
	}
}

func TestResolveV2Ratio(t *testing.T) {
	textOnly := []V2ContentItem{{Type: V2TypeText, Text: "hi"}}
	firstFrame := []V2ContentItem{
		{Type: V2TypeText, Text: "hi"},
		{Type: V2TypeImageURL, Role: V2RoleFirstFrame, ImageURL: &V2URLPart{URL: "https://x/a.png"}},
	}
	reference := []V2ContentItem{
		{Type: V2TypeText, Text: "hi"},
		{Type: V2TypeImageURL, Role: V2RoleReferenceImage, ImageURL: &V2URLPart{URL: "https://x/a.png"}},
	}

	tests := []struct {
		name     string
		content  []V2ContentItem
		size     string
		metadata map[string]any
		expected string
	}{
		{"t2va without ratio", textOnly, "", nil, V2DefaultT2VRatio},
		{"t2va adaptive falls back", textOnly, "adaptive", nil, V2DefaultT2VRatio},
		{"t2va explicit ratio kept", textOnly, "9:16", nil, "9:16"},
		{"t2va image-like size rejected", textOnly, "1024x1024", nil, V2DefaultT2VRatio},
		{"t2va metadata ratio wins", textOnly, "", map[string]any{"ratio": "4:3"}, "4:3"},
		{"i2va always adaptive", firstFrame, "16:9", nil, V2RatioAdaptive},
		{"r2va without ratio", reference, "", nil, V2RatioAdaptive},
		{"r2va explicit ratio kept", reference, "4:3", nil, "4:3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveV2Ratio(tt.content, tt.size, tt.metadata); got != tt.expected {
				t.Errorf("resolveV2Ratio() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestBuildV2Content(t *testing.T) {
	t.Run("text only", func(t *testing.T) {
		items, err := buildV2Content(relaycommon.TaskSubmitReq{Prompt: "hello"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(items) != 1 || items[0].Type != V2TypeText || items[0].Text != "hello" {
			t.Fatalf("unexpected content: %+v", items)
		}
	})

	t.Run("single image becomes first frame", func(t *testing.T) {
		items, err := buildV2Content(relaycommon.TaskSubmitReq{
			Prompt: "p", Images: []string{"https://x/1.png"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(items) != 2 || items[1].Role != V2RoleFirstFrame || items[1].ImageURL == nil {
			t.Fatalf("unexpected content: %+v", items)
		}
	})

	t.Run("two images become first and last frame", func(t *testing.T) {
		items, err := buildV2Content(relaycommon.TaskSubmitReq{
			Prompt: "p", Images: []string{"https://x/1.png", "https://x/2.png"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if items[1].Role != V2RoleFirstFrame || items[2].Role != V2RoleLastFrame {
			t.Fatalf("unexpected roles: %+v", items)
		}
	})

	t.Run("three images become reference images", func(t *testing.T) {
		items, err := buildV2Content(relaycommon.TaskSubmitReq{
			Prompt: "p", Images: []string{"https://x/1.png", "https://x/2.png", "https://x/3.png"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, item := range items[1:] {
			if item.Role != V2RoleReferenceImage {
				t.Fatalf("expected reference_image, got %+v", item)
			}
		}
	})

	t.Run("input reference used as first frame", func(t *testing.T) {
		items, err := buildV2Content(relaycommon.TaskSubmitReq{
			Prompt: "p", InputReference: "https://x/ref.png",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(items) != 2 || items[1].Role != V2RoleFirstFrame {
			t.Fatalf("unexpected content: %+v", items)
		}
	})

	t.Run("metadata content passthrough", func(t *testing.T) {
		items, err := buildV2Content(relaycommon.TaskSubmitReq{
			Prompt: "ignored",
			Metadata: map[string]any{
				"content": []any{
					map[string]any{"type": "text", "text": "custom"},
					map[string]any{"type": "video_url", "video_url": map[string]any{"url": "https://x/a.mp4"}, "role": "reference_video"},
				},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(items) != 2 || items[0].Text != "custom" || items[1].VideoURL == nil || items[1].Role != V2RoleReferenceVideo {
			t.Fatalf("unexpected content: %+v", items)
		}
	})

	t.Run("reference videos and audios from metadata", func(t *testing.T) {
		items, err := buildV2Content(relaycommon.TaskSubmitReq{
			Prompt: "p",
			Metadata: map[string]any{
				"reference_videos": []any{"https://x/1.mp4"},
				"reference_audios": []any{"https://x/1.wav"},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(items) != 3 || items[1].Role != V2RoleReferenceVideo || items[2].Role != V2RoleReferenceAudio {
			t.Fatalf("unexpected content: %+v", items)
		}
	})

	t.Run("prompt truncated", func(t *testing.T) {
		items, err := buildV2Content(relaycommon.TaskSubmitReq{Prompt: strings.Repeat("字", V2PromptMaxLen+500)})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := len([]rune(items[0].Text)); got != V2PromptMaxLen {
			t.Fatalf("prompt rune length = %d, want %d", got, V2PromptMaxLen)
		}
	})

	t.Run("empty prompt rejected", func(t *testing.T) {
		if _, err := buildV2Content(relaycommon.TaskSubmitReq{Prompt: "   "}); err == nil {
			t.Fatal("expected error for empty prompt")
		}
	})
}

func TestSanitizeV2Content(t *testing.T) {
	t.Run("frames downgraded when mixed with references", func(t *testing.T) {
		items := sanitizeV2Content([]V2ContentItem{
			{Type: V2TypeText, Text: "p"},
			{Type: V2TypeImageURL, Role: V2RoleFirstFrame, ImageURL: &V2URLPart{URL: "https://x/1.png"}},
			{Type: V2TypeImageURL, Role: V2RoleReferenceImage, ImageURL: &V2URLPart{URL: "https://x/2.png"}},
		})
		for _, item := range items[1:] {
			if item.Role != V2RoleReferenceImage {
				t.Fatalf("expected all images downgraded to reference_image, got %+v", item)
			}
		}
	})

	t.Run("reference image count truncated", func(t *testing.T) {
		items := make([]V2ContentItem, 0, V2MaxReferenceImages+5)
		items = append(items, V2ContentItem{Type: V2TypeText, Text: "p"})
		for i := 0; i < V2MaxReferenceImages+5; i++ {
			items = append(items, V2ContentItem{
				Type: V2TypeImageURL, Role: V2RoleReferenceImage,
				ImageURL: &V2URLPart{URL: "https://x/" + string(rune('a'+i)) + ".png"},
			})
		}
		result := sanitizeV2Content(items)
		if got := len(result) - 1; got != V2MaxReferenceImages {
			t.Fatalf("reference image count = %d, want %d", got, V2MaxReferenceImages)
		}
	})

	t.Run("empty url and unknown type dropped", func(t *testing.T) {
		result := sanitizeV2Content([]V2ContentItem{
			{Type: V2TypeText, Text: "p"},
			{Type: V2TypeImageURL, Role: V2RoleFirstFrame, ImageURL: &V2URLPart{URL: "  "}},
			{Type: "file", Text: "x"},
		})
		if len(result) != 1 {
			t.Fatalf("expected only the text item, got %+v", result)
		}
	})

	t.Run("missing role defaulted by type", func(t *testing.T) {
		result := sanitizeV2Content([]V2ContentItem{
			{Type: V2TypeText, Text: "p"},
			{Type: V2TypeVideoURL, VideoURL: &V2URLPart{URL: "https://x/a.mp4"}},
		})
		if result[1].Role != V2RoleReferenceVideo {
			t.Fatalf("expected default role reference_video, got %q", result[1].Role)
		}
	})
}

func TestBuildV2Request(t *testing.T) {
	adaptor := &TaskAdaptor{}
	info := newV2RelayInfo("MiniMax-H3")

	t.Run("converges illegal values", func(t *testing.T) {
		req := relaycommon.TaskSubmitReq{
			Prompt:   "p",
			Size:     "1080P",
			Duration: 30,
			Metadata: map[string]any{"resolution": "1080P", "duration": float64(99)},
		}
		v2Req, err := adaptor.buildV2Request(req, info)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v2Req.Resolution != V2Resolution2K {
			t.Errorf("resolution = %q, want %q", v2Req.Resolution, V2Resolution2K)
		}
		if v2Req.Duration != V2DurationMax {
			t.Errorf("duration = %d, want %d", v2Req.Duration, V2DurationMax)
		}
		if v2Req.Ratio != V2RatioAdaptive && v2Req.Ratio != V2DefaultT2VRatio {
			t.Errorf("unexpected ratio %q", v2Req.Ratio)
		}
	})

	t.Run("metadata cannot override model", func(t *testing.T) {
		req := relaycommon.TaskSubmitReq{
			Prompt:   "p",
			Metadata: map[string]any{"model": "gpt-4", "resolution": "768P"},
		}
		v2Req, err := adaptor.buildV2Request(req, info)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v2Req.Model != "MiniMax-H3" {
			t.Fatalf("model = %q, want MiniMax-H3", v2Req.Model)
		}
		if v2Req.Resolution != V2Resolution768P {
			t.Fatalf("resolution = %q, want %q", v2Req.Resolution, V2Resolution768P)
		}
	})

	t.Run("metadata content passthrough survives re-convergence", func(t *testing.T) {
		req := relaycommon.TaskSubmitReq{
			Prompt: "p",
			Metadata: map[string]any{
				"content": []any{
					map[string]any{"type": "text", "text": "custom prompt"},
					map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://x/a.png"}, "role": "reference_image"},
				},
				"ratio": "9:16",
			},
		}
		v2Req, err := adaptor.buildV2Request(req, info)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(v2Req.Content) != 2 || v2Req.Content[1].Role != V2RoleReferenceImage {
			t.Fatalf("unexpected content: %+v", v2Req.Content)
		}
		if v2Req.Ratio != "9:16" {
			t.Fatalf("ratio = %q, want 9:16", v2Req.Ratio)
		}
	})

	t.Run("aigc watermark pointer preserved", func(t *testing.T) {
		req := relaycommon.TaskSubmitReq{Prompt: "p", Metadata: map[string]any{"aigc_watermark": false}}
		v2Req, err := adaptor.buildV2Request(req, info)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v2Req.AigcWatermark == nil || *v2Req.AigcWatermark != false {
			t.Fatalf("aigc_watermark = %v, want explicit false", v2Req.AigcWatermark)
		}
		data, err := common.Marshal(v2Req)
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}
		if !strings.Contains(string(data), `"aigc_watermark":false`) {
			t.Fatalf("explicit zero value must be serialized, got %s", data)
		}
	})
}

func TestParseV2TaskResult(t *testing.T) {
	adaptor := &TaskAdaptor{}

	tests := []struct {
		name           string
		body           string
		wantOK         bool
		wantStatus     string
		wantProgress   string
		wantURL        string
		wantReasonPart string
	}{
		{
			name:         "queued",
			body:         `{"task":{"id":"1","status":"queued"}}`,
			wantOK:       true,
			wantStatus:   model.TaskStatusQueued,
			wantProgress: "20%",
		},
		{
			name:         "running",
			body:         `{"task":{"id":"1","status":"running"}}`,
			wantOK:       true,
			wantStatus:   model.TaskStatusInProgress,
			wantProgress: "50%",
		},
		{
			name:   "succeeded",
			body:   `{"task":{"id":"1","status":"succeeded","content":{"url":"https://cdn/x.mp4"},"usage":{"total_seconds":5}}}`,
			wantOK: true, wantStatus: model.TaskStatusSuccess,
			wantProgress: "100%", wantURL: "https://cdn/x.mp4",
		},
		{
			name:   "succeeded without url",
			body:   `{"task":{"id":"1","status":"succeeded","content":{}}}`,
			wantOK: true, wantStatus: model.TaskStatusFailure,
			wantProgress: "100%", wantReasonPart: "empty content.url",
		},
		{
			name:   "failed with error object",
			body:   `{"task":{"id":"1","status":"failed","error":{"code":"1027","message":"content policy"}}}`,
			wantOK: true, wantStatus: model.TaskStatusFailure,
			wantProgress: "100%", wantReasonPart: "content policy",
		},
		{
			name:   "cancelled",
			body:   `{"task":{"id":"1","status":"cancelled"}}`,
			wantOK: true, wantStatus: model.TaskStatusFailure,
			wantProgress: "100%", wantReasonPart: "task cancelled",
		},
		{
			name:         "unknown status keeps in progress",
			body:         `{"task":{"id":"1","status":"something_else"}}`,
			wantOK:       true,
			wantStatus:   model.TaskStatusInProgress,
			wantProgress: "30%",
		},
		{
			name:   "v1 response is not v2",
			body:   `{"task_id":"1","status":"Success","file_id":"f1","base_resp":{"status_code":0,"status_msg":"ok"}}`,
			wantOK: false,
		},
		{
			name:   "malformed json",
			body:   `not json`,
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok, err := adaptor.parseV2TaskResult([]byte(tt.body))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if got.Status != tt.wantStatus {
				t.Errorf("status = %q, want %q", got.Status, tt.wantStatus)
			}
			if tt.wantProgress != "" && got.Progress != tt.wantProgress {
				t.Errorf("progress = %q, want %q", got.Progress, tt.wantProgress)
			}
			if tt.wantURL != "" && got.Url != tt.wantURL {
				t.Errorf("url = %q, want %q", got.Url, tt.wantURL)
			}
			if tt.wantReasonPart != "" && !strings.Contains(got.Reason, tt.wantReasonPart) {
				t.Errorf("reason = %q, want substring %q", got.Reason, tt.wantReasonPart)
			}
			// usage 不参与计费：不得填 TotalTokens，否则会触发按 token 重算
			if got.TotalTokens != 0 || got.CompletionTokens != 0 {
				t.Errorf("token fields must stay zero, got total=%d completion=%d", got.TotalTokens, got.CompletionTokens)
			}
		})
	}
}

func TestParseTaskResultFallsBackToV1(t *testing.T) {
	adaptor := &TaskAdaptor{}
	body := `{"task_id":"1","status":"Processing","base_resp":{"status_code":0,"status_msg":"ok"}}`

	got, err := adaptor.ParseTaskResult([]byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != model.TaskStatusInProgress || got.Progress != "50%" {
		t.Fatalf("v1 parsing broken: %+v", got)
	}
}

func TestBuildQueryURL(t *testing.T) {
	tests := []struct {
		name     string
		body     map[string]any
		expected string
	}{
		{
			name:     "v2 action uses path parameter",
			body:     map[string]any{"task_id": "424", "action": constant.TaskActionVideoV2Generate},
			expected: "https://api.minimaxi.com/v2/query/video_generation/424",
		},
		{
			name:     "v1 action uses query parameter",
			body:     map[string]any{"task_id": "424", "action": constant.TaskActionGenerate},
			expected: "https://api.minimaxi.com/v1/query/video_generation?task_id=424",
		},
		{
			name:     "missing action defaults to v1",
			body:     map[string]any{"task_id": "424"},
			expected: "https://api.minimaxi.com/v1/query/video_generation?task_id=424",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildQueryURL("https://api.minimaxi.com", tt.body); got != tt.expected {
				t.Errorf("buildQueryURL() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestBuildRequestURL(t *testing.T) {
	adaptor := &TaskAdaptor{baseURL: "https://api.minimaxi.com"}

	v2URL, err := adaptor.BuildRequestURL(newV2RelayInfo("MiniMax-H3"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v2URL != "https://api.minimaxi.com/v2/video_generation" {
		t.Errorf("v2 url = %q", v2URL)
	}

	v1URL, err := adaptor.BuildRequestURL(newV2RelayInfo("MiniMax-Hailuo-2.3"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v1URL != "https://api.minimaxi.com/v1/video_generation" {
		t.Errorf("v1 url = %q", v1URL)
	}
}

func newContextWithRequest(req relaycommon.TaskSubmitReq) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
	c.Set("task_request", req)
	return c, recorder
}

func TestEstimateBilling(t *testing.T) {
	adaptor := &TaskAdaptor{}

	t.Run("v2 returns seconds and resolution ratios and marks action", func(t *testing.T) {
		info := newV2RelayInfo("MiniMax-H3")
		c, _ := newContextWithRequest(relaycommon.TaskSubmitReq{
			Prompt: "p", Duration: 8, Metadata: map[string]any{"resolution": "2K"},
		})

		ratios := adaptor.EstimateBilling(c, info)
		if ratios["seconds"] != 8 {
			t.Errorf("seconds ratio = %v, want 8", ratios["seconds"])
		}
		if ratios["resolution-2K"] != 2.0 {
			t.Errorf("resolution-2K ratio = %v, want 2", ratios["resolution-2K"])
		}
		if info.Action != constant.TaskActionVideoV2Generate {
			t.Errorf("action = %q, want %q", info.Action, constant.TaskActionVideoV2Generate)
		}
	})

	t.Run("v2 768P ratio is 1", func(t *testing.T) {
		info := newV2RelayInfo("MiniMax-H3")
		c, _ := newContextWithRequest(relaycommon.TaskSubmitReq{
			Prompt: "p", Duration: 5, Metadata: map[string]any{"resolution": "768P"},
		})
		ratios := adaptor.EstimateBilling(c, info)
		if ratios["resolution-768P"] != 1.0 {
			t.Errorf("resolution-768P ratio = %v, want 1", ratios["resolution-768P"])
		}
	})

	t.Run("v1 keeps per-call billing and untouched action", func(t *testing.T) {
		info := newV2RelayInfo("MiniMax-Hailuo-2.3")
		c, _ := newContextWithRequest(relaycommon.TaskSubmitReq{Prompt: "p", Duration: 6})

		if ratios := adaptor.EstimateBilling(c, info); ratios != nil {
			t.Errorf("v1 ratios = %v, want nil", ratios)
		}
		if info.Action != constant.TaskActionGenerate {
			t.Errorf("v1 action changed to %q", info.Action)
		}
	})
}

func TestDoV2Response(t *testing.T) {
	adaptor := &TaskAdaptor{}
	info := newV2RelayInfo("MiniMax-H3")

	t.Run("success", func(t *testing.T) {
		c, recorder := newContextWithRequest(relaycommon.TaskSubmitReq{Prompt: "p"})
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"task_id":"424010985738629"}`)),
		}

		taskID, taskData, taskErr := adaptor.doV2Response(c, resp, info)
		if taskErr != nil {
			t.Fatalf("unexpected task error: %+v", taskErr)
		}
		if taskID != "424010985738629" {
			t.Errorf("taskID = %q", taskID)
		}
		if !strings.Contains(string(taskData), "424010985738629") {
			t.Errorf("taskData should keep upstream create response, got %s", taskData)
		}

		var video dto.OpenAIVideo
		if err := common.Unmarshal(recorder.Body.Bytes(), &video); err != nil {
			t.Fatalf("response is not OpenAI video json: %v (%s)", err, recorder.Body.String())
		}
		if video.ID != "task_public_0001" || video.Object != "video" || video.Status != dto.VideoStatusQueued {
			t.Errorf("unexpected video payload: %+v", video)
		}
	})

	t.Run("error envelope", func(t *testing.T) {
		c, _ := newContextWithRequest(relaycommon.TaskSubmitReq{Prompt: "p"})
		resp := &http.Response{
			StatusCode: http.StatusBadRequest,
			Body: io.NopCloser(strings.NewReader(
				`{"type":"error","error":{"type":"bad_request_error","message":"invalid params, content must include a non-empty text item (prompt is required) (2013)","http_code":"400"},"request_id":"req1"}`,
			)),
		}

		_, _, taskErr := adaptor.doV2Response(c, resp, info)
		if taskErr == nil {
			t.Fatal("expected task error")
		}
		if taskErr.Code != "bad_request_error" {
			t.Errorf("code = %q, want bad_request_error", taskErr.Code)
		}
		if !strings.Contains(taskErr.Message, "non-empty text item") {
			t.Errorf("message = %q", taskErr.Message)
		}
	})

	t.Run("empty body without error envelope", func(t *testing.T) {
		c, _ := newContextWithRequest(relaycommon.TaskSubmitReq{Prompt: "p"})
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{}`)),
		}
		_, _, taskErr := adaptor.doV2Response(c, resp, info)
		if taskErr == nil || taskErr.Code != "minimax_v2_error" {
			t.Fatalf("expected minimax_v2_error, got %+v", taskErr)
		}
	})
}

func TestConvertToOpenAIVideo(t *testing.T) {
	adaptor := &TaskAdaptor{}

	t.Run("success reads live db fields", func(t *testing.T) {
		task := &model.Task{
			TaskID:      "task_public_0001",
			Status:      model.TaskStatusSuccess,
			Progress:    "100%",
			PrivateData: model.TaskPrivateData{ResultURL: "https://cdn/out.mp4"},
			Properties:  model.Properties{OriginModelName: "MiniMax-H3"},
			// 提交阶段的 Data：v2 创建响应无 base_resp，不得被误判
			Data: json.RawMessage(`{"task_id":"424"}`),
		}
		body, err := adaptor.ConvertToOpenAIVideo(task)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var video dto.OpenAIVideo
		if err := common.Unmarshal(body, &video); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		if video.Status != dto.VideoStatusCompleted {
			t.Errorf("status = %q, want %q", video.Status, dto.VideoStatusCompleted)
		}
		if video.Error != nil {
			t.Errorf("unexpected error object: %+v", video.Error)
		}
		if got, _ := video.Metadata["url"].(string); got != "https://cdn/out.mp4" {
			t.Errorf("metadata.url = %q, want https://cdn/out.mp4", got)
		}
	})

	t.Run("failure prefers FailReason", func(t *testing.T) {
		task := &model.Task{
			TaskID:     "task_public_0002",
			Status:     model.TaskStatusFailure,
			FailReason: "upstream busy",
			Data:       json.RawMessage(`{"task":{"id":"1","status":"failed","error":{"message":"ignored"}}}`),
		}
		body, err := adaptor.ConvertToOpenAIVideo(task)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var video dto.OpenAIVideo
		if err := common.Unmarshal(body, &video); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		if video.Error == nil || video.Error.Message != "upstream busy" || video.Error.Code != "task_failed" {
			t.Fatalf("unexpected error object: %+v", video.Error)
		}
	})

	t.Run("failure falls back to v2 task error", func(t *testing.T) {
		task := &model.Task{
			TaskID: "task_public_0003",
			Status: model.TaskStatusFailure,
			Data:   json.RawMessage(`{"task":{"id":"1","status":"failed","error":{"message":"sensitive content"}}}`),
		}
		body, err := adaptor.ConvertToOpenAIVideo(task)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var video dto.OpenAIVideo
		if err := common.Unmarshal(body, &video); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		if video.Error == nil || !strings.Contains(video.Error.Message, "sensitive content") {
			t.Fatalf("expected v2 error surfaced, got %+v", video.Error)
		}
	})

	t.Run("failure falls back to v1 base_resp", func(t *testing.T) {
		task := &model.Task{
			TaskID: "task_public_0004",
			Status: model.TaskStatusFailure,
			Data:   json.RawMessage(`{"task_id":"1","status":"Fail","base_resp":{"status_code":1026,"status_msg":"sensitive"}}`),
		}
		body, err := adaptor.ConvertToOpenAIVideo(task)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var video dto.OpenAIVideo
		if err := common.Unmarshal(body, &video); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		if video.Error == nil || video.Error.Code != "1026" || video.Error.Message != "sensitive" {
			t.Fatalf("expected v1 base_resp surfaced, got %+v", video.Error)
		}
	})

	t.Run("in progress has no error object", func(t *testing.T) {
		task := &model.Task{
			TaskID:   "task_public_0005",
			Status:   model.TaskStatusInProgress,
			Progress: "50%",
			Data:     json.RawMessage(`{"task":{"id":"1","status":"running"}}`),
		}
		body, err := adaptor.ConvertToOpenAIVideo(task)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var video dto.OpenAIVideo
		if err := common.Unmarshal(body, &video); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		if video.Status != dto.VideoStatusInProgress || video.Error != nil {
			t.Fatalf("unexpected video: %+v", video)
		}
	})
}

func TestBuildRequestBodyV2(t *testing.T) {
	adaptor := &TaskAdaptor{}
	info := newV2RelayInfo("MiniMax-H3")
	c, _ := newContextWithRequest(relaycommon.TaskSubmitReq{
		Prompt:   "epic teaser",
		Duration: 5,
		Size:     "16:9",
		Images:   []string{"https://x/1.png"},
	})

	reader, err := adaptor.BuildRequestBody(c, info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read body failed: %v", err)
	}

	var payload V2VideoRequest
	if err := common.Unmarshal(data, &payload); err != nil {
		t.Fatalf("payload is not valid v2 request: %v (%s)", err, data)
	}
	if payload.Model != "MiniMax-H3" {
		t.Errorf("model = %q", payload.Model)
	}
	if payload.Resolution != V2DefaultResolution {
		t.Errorf("resolution = %q, want %q", payload.Resolution, V2DefaultResolution)
	}
	if payload.Duration != 5 {
		t.Errorf("duration = %d, want 5", payload.Duration)
	}
	// 含首帧图片 → 图生视频，ratio 必须被收敛为 adaptive
	if payload.Ratio != V2RatioAdaptive {
		t.Errorf("ratio = %q, want %q", payload.Ratio, V2RatioAdaptive)
	}
	if len(payload.Content) != 2 || payload.Content[1].ImageURL == nil || payload.Content[1].Role != V2RoleFirstFrame {
		t.Fatalf("unexpected content: %+v", payload.Content)
	}
}
