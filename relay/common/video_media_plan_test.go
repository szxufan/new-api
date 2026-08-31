package common

import (
	"reflect"
	"testing"

	"github.com/QuantumNous/new-api/constant"
)

// TestBuildMediaPlanDefaultImplicit 锚定默认数量推导：
// 1 张→首帧，2 张→首尾帧，≥3 张→参考图，0 张→文生视频。
func TestBuildMediaPlanDefaultImplicit(t *testing.T) {
	tests := []struct {
		name      string
		req       TaskSubmitReq
		wantMode  VideoGenerationMode
		wantFirst string
		wantLast  string
		wantRefs  []string
	}{
		{
			name:     "no media -> text2video",
			req:      TaskSubmitReq{Prompt: "p"},
			wantMode: constant.TaskActionTextGenerate,
		},
		{
			name:      "one image -> first frame",
			req:       TaskSubmitReq{Prompt: "p", Images: []string{"https://x/a.png"}},
			wantMode:  constant.TaskActionGenerate,
			wantFirst: "https://x/a.png",
		},
		{
			name:      "two images -> first-last frame",
			req:       TaskSubmitReq{Prompt: "p", Images: []string{"https://x/a.png", "https://x/b.png"}},
			wantMode:  constant.TaskActionFirstTailGenerate,
			wantFirst: "https://x/a.png",
			wantLast:  "https://x/b.png",
		},
		{
			name:     "three images -> references",
			req:      TaskSubmitReq{Prompt: "p", Images: []string{"https://x/a.png", "https://x/b.png", "https://x/c.png"}},
			wantMode: constant.TaskActionReferenceGenerate,
			wantRefs: []string{"https://x/a.png", "https://x/b.png", "https://x/c.png"},
		},
		{
			name:      "input_reference fallback -> first frame",
			req:       TaskSubmitReq{Prompt: "p", InputReference: "https://x/ref.png"},
			wantMode:  constant.TaskActionGenerate,
			wantFirst: "https://x/ref.png",
		},
		{
			name:      "image fallback -> first frame",
			req:       TaskSubmitReq{Prompt: "p", Image: "https://x/single.png"},
			wantMode:  constant.TaskActionGenerate,
			wantFirst: "https://x/single.png",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := BuildMediaPlan(tt.req, MediaPlanOptions{})
			if err != nil {
				t.Fatalf("BuildMediaPlan() error = %v", err)
			}
			if plan.Mode != tt.wantMode {
				t.Errorf("plan.Mode = %q, want %q", plan.Mode, tt.wantMode)
			}
			if plan.FirstFrame != tt.wantFirst {
				t.Errorf("plan.FirstFrame = %q, want %q", plan.FirstFrame, tt.wantFirst)
			}
			if plan.LastFrame != tt.wantLast {
				t.Errorf("plan.LastFrame = %q, want %q", plan.LastFrame, tt.wantLast)
			}
			if len(plan.ReferenceImages) == 0 && len(tt.wantRefs) == 0 {
				return
			}
			if !reflect.DeepEqual(plan.ReferenceImages, tt.wantRefs) {
				t.Errorf("plan.ReferenceImages = %v, want %v", plan.ReferenceImages, tt.wantRefs)
			}
		})
	}
}

// TestBuildMediaPlanNamedMetadataKeys 锚定统一 metadata 具名键（阶段 1 显式入口）。
func TestBuildMediaPlanNamedMetadataKeys(t *testing.T) {
	req := TaskSubmitReq{
		Prompt: "p",
		Metadata: map[string]any{
			MetadataKeyFirstFrame:      "https://x/first.png",
			MetadataKeyLastFrame:       "https://x/last.png",
			MetadataKeyReferenceImages: []any{"https://x/r1.png", "https://x/r2.png"},
			MetadataKeyReferenceVideos: []string{"https://x/v1.mp4"},
			MetadataKeyReferenceAudios: "https://x/a1.mp3",
		},
	}
	plan, err := BuildMediaPlan(req, MediaPlanOptions{})
	if err != nil {
		t.Fatalf("BuildMediaPlan() error = %v", err)
	}
	if !plan.Explicit {
		t.Errorf("plan.Explicit = false, want true")
	}
	if plan.FirstFrame != "https://x/first.png" || plan.LastFrame != "https://x/last.png" {
		t.Errorf("frames = (%q, %q), want first/last from metadata", plan.FirstFrame, plan.LastFrame)
	}
	wantRefs := []string{"https://x/r1.png", "https://x/r2.png"}
	if !reflect.DeepEqual(plan.ReferenceImages, wantRefs) {
		t.Errorf("plan.ReferenceImages = %v, want %v", plan.ReferenceImages, wantRefs)
	}
	if !reflect.DeepEqual(plan.ReferenceVideos, []string{"https://x/v1.mp4"}) {
		t.Errorf("plan.ReferenceVideos = %v", plan.ReferenceVideos)
	}
	if !reflect.DeepEqual(plan.ReferenceAudios, []string{"https://x/a1.mp3"}) {
		t.Errorf("plan.ReferenceAudios = %v", plan.ReferenceAudios)
	}
	if plan.Mode != constant.TaskActionReferenceGenerate {
		t.Errorf("plan.Mode = %q, want referenceGenerate", plan.Mode)
	}
}

// TestBuildMediaPlanExplicitFramesMergeImages 锚定现状语义：
// 显式指定首/尾帧时，images 并入参考素材（顺序在具名参考之后）。
func TestBuildMediaPlanExplicitFramesMergeImages(t *testing.T) {
	req := TaskSubmitReq{
		Prompt: "p",
		Images: []string{"https://x/i1.png", "https://x/i2.png"},
		Metadata: map[string]any{
			MetadataKeyFirstFrame:      "https://x/first.png",
			MetadataKeyReferenceImages: []string{"https://x/r1.png"},
		},
	}
	plan, err := BuildMediaPlan(req, MediaPlanOptions{})
	if err != nil {
		t.Fatalf("BuildMediaPlan() error = %v", err)
	}
	wantRefs := []string{"https://x/r1.png", "https://x/i1.png", "https://x/i2.png"}
	if !reflect.DeepEqual(plan.ReferenceImages, wantRefs) {
		t.Errorf("plan.ReferenceImages = %v, want %v", plan.ReferenceImages, wantRefs)
	}
}

// TestBuildMediaPlanExclusiveDowngrade 锚定 hailuo v2 现状：
// 互斥冲突时首尾帧降级为参考图（保留素材，位于参考素材之前）。
func TestBuildMediaPlanExclusiveDowngrade(t *testing.T) {
	req := TaskSubmitReq{
		Prompt: "p",
		Images: []string{"https://x/a.png", "https://x/b.png"},
		Metadata: map[string]any{
			MetadataKeyReferenceVideos: []string{"https://x/v.mp4"},
		},
	}
	opts := MediaPlanOptions{Limits: MediaLimits{
		MutualExclusive:     true,
		OnExclusiveConflict: DowngradeFramesToReference,
	}}
	plan, err := BuildMediaPlan(req, opts)
	if err != nil {
		t.Fatalf("BuildMediaPlan() error = %v", err)
	}
	if plan.FirstFrame != "" || plan.LastFrame != "" {
		t.Errorf("frames should be downgraded, got (%q, %q)", plan.FirstFrame, plan.LastFrame)
	}
	wantRefs := []string{"https://x/a.png", "https://x/b.png"}
	if !reflect.DeepEqual(plan.ReferenceImages, wantRefs) {
		t.Errorf("plan.ReferenceImages = %v, want %v", plan.ReferenceImages, wantRefs)
	}
	if plan.Mode != constant.TaskActionReferenceGenerate {
		t.Errorf("plan.Mode = %q, want referenceGenerate", plan.Mode)
	}
}

// TestBuildMediaPlanExclusiveDrop 锚定 ali minimax 现状：
// 互斥冲突时丢弃首尾帧。
func TestBuildMediaPlanExclusiveDrop(t *testing.T) {
	req := TaskSubmitReq{
		Prompt: "p",
		Images: []string{"https://x/a.png", "https://x/b.png"},
		Metadata: map[string]any{
			MetadataKeyReferenceVideos: []string{"https://x/v.mp4"},
		},
	}
	opts := MediaPlanOptions{Limits: MediaLimits{
		MutualExclusive:     true,
		OnExclusiveConflict: DropFrames,
	}}
	plan, err := BuildMediaPlan(req, opts)
	if err != nil {
		t.Fatalf("BuildMediaPlan() error = %v", err)
	}
	if plan.FirstFrame != "" || plan.LastFrame != "" {
		t.Errorf("frames should be dropped, got (%q, %q)", plan.FirstFrame, plan.LastFrame)
	}
	if len(plan.ReferenceImages) != 0 {
		t.Errorf("plan.ReferenceImages = %v, want empty (dropped, not downgraded)", plan.ReferenceImages)
	}
	if !reflect.DeepEqual(plan.ReferenceVideos, []string{"https://x/v.mp4"}) {
		t.Errorf("plan.ReferenceVideos = %v", plan.ReferenceVideos)
	}
}

// TestBuildMediaPlanLimits 锚定数量收敛：按角色上限截断，保留靠前者。
func TestBuildMediaPlanLimits(t *testing.T) {
	req := TaskSubmitReq{
		Prompt: "p",
		Metadata: map[string]any{
			MetadataKeyReferenceImages: []string{"r1", "r2", "r3", "r4"},
			MetadataKeyReferenceVideos: []string{"v1", "v2"},
		},
	}
	opts := MediaPlanOptions{Limits: MediaLimits{
		MaxReferenceImage: 2,
		MaxReferenceVideo: 1,
	}}
	plan, err := BuildMediaPlan(req, opts)
	if err != nil {
		t.Fatalf("BuildMediaPlan() error = %v", err)
	}
	if !reflect.DeepEqual(plan.ReferenceImages, []string{"r1", "r2"}) {
		t.Errorf("plan.ReferenceImages = %v, want [r1 r2]", plan.ReferenceImages)
	}
	if !reflect.DeepEqual(plan.ReferenceVideos, []string{"v1"}) {
		t.Errorf("plan.ReferenceVideos = %v, want [v1]", plan.ReferenceVideos)
	}
}

// TestBuildMediaPlanTotalLimit 锚定总数上限（hailuo v2 的 V2MaxMixedAssets 语义）：
// 按「首尾帧 → 参考图 → 参考视频 → 参考音频」顺序保留靠前者。
func TestBuildMediaPlanTotalLimit(t *testing.T) {
	req := TaskSubmitReq{
		Prompt: "p",
		Metadata: map[string]any{
			MetadataKeyFirstFrame:      "first.png",
			MetadataKeyReferenceImages: []string{"r1", "r2"},
			MetadataKeyReferenceVideos: []string{"v1"},
		},
	}
	opts := MediaPlanOptions{Limits: MediaLimits{MaxTotalMedia: 3}}
	plan, err := BuildMediaPlan(req, opts)
	if err != nil {
		t.Fatalf("BuildMediaPlan() error = %v", err)
	}
	if plan.FirstFrame != "first.png" {
		t.Errorf("plan.FirstFrame = %q, want kept", plan.FirstFrame)
	}
	if !reflect.DeepEqual(plan.ReferenceImages, []string{"r1", "r2"}) {
		t.Errorf("plan.ReferenceImages = %v, want [r1 r2]", plan.ReferenceImages)
	}
	if len(plan.ReferenceVideos) != 0 {
		t.Errorf("plan.ReferenceVideos = %v, want dropped by total limit", plan.ReferenceVideos)
	}
}

// TestBuildMediaPlanWan3Override 锚定阿里万相3.0 的渠道覆写（§9 决策：保留现状）：
// 单图 → 首帧；多图 → 全部参考图（而非默认的首尾帧）。
func TestBuildMediaPlanWan3Override(t *testing.T) {
	wan3 := MediaPlanOptions{
		ImplicitImages: func(images []string, plan *MediaPlan) {
			if len(images) == 1 {
				plan.FirstFrame = images[0]
				return
			}
			plan.ReferenceImages = append(plan.ReferenceImages, images...)
		},
	}

	plan, err := BuildMediaPlan(TaskSubmitReq{Prompt: "p", Images: []string{"a.png", "b.png"}}, wan3)
	if err != nil {
		t.Fatalf("BuildMediaPlan() error = %v", err)
	}
	if plan.Mode != constant.TaskActionReferenceGenerate {
		t.Errorf("two images: plan.Mode = %q, want referenceGenerate (wan3 semantics)", plan.Mode)
	}
	if !reflect.DeepEqual(plan.ReferenceImages, []string{"a.png", "b.png"}) {
		t.Errorf("plan.ReferenceImages = %v", plan.ReferenceImages)
	}

	plan, err = BuildMediaPlan(TaskSubmitReq{Prompt: "p", Images: []string{"a.png"}}, wan3)
	if err != nil {
		t.Fatalf("BuildMediaPlan() error = %v", err)
	}
	if plan.Mode != constant.TaskActionGenerate || plan.FirstFrame != "a.png" {
		t.Errorf("one image: mode=%q first=%q, want generate/a.png", plan.Mode, plan.FirstFrame)
	}
}

func TestMetadataStringsCoercion(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  []string
	}{
		{"[]string", []string{"a", "", "b"}, []string{"a", "b"}},
		{"[]any", []any{"a", 1, "b"}, []string{"a", "b"}},
		{"single string", "a", []string{"a"}},
		{"blank string", "  ", nil},
		{"wrong type", 123, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MetadataStrings(map[string]any{"k": tt.value}, "k")
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MetadataStrings() = %v, want %v", got, tt.want)
			}
		})
	}
	if got := MetadataStrings(nil, "k"); got != nil {
		t.Errorf("MetadataStrings(nil) = %v, want nil", got)
	}
}

func TestMediaPlanImagesFlatten(t *testing.T) {
	plan := MediaPlan{
		FirstFrame:      "first.png",
		LastFrame:       "last.png",
		ReferenceImages: []string{"r1.png"},
	}
	want := []string{"first.png", "last.png", "r1.png"}
	if got := plan.Images(); !reflect.DeepEqual(got, want) {
		t.Errorf("plan.Images() = %v, want %v", got, want)
	}
}
