package common

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
)

// ============================================================================
// 视频生成「模式」统一解析层（阶段 1）
//
// 设计文档：docs/video-generation-mode-design.md
//
// MediaPlan 是跨渠道统一的「模式 + 素材角色」标准化结果：
//   - 显式来源：统一 metadata 具名键（见下方 MetadataKey* 常量）
//   - 隐式来源：images 数量推导（默认规则可被渠道覆写，如阿里万相3.0）
//
// 零回归约束：隐式推导必须逐渠道复刻现有语义，不得"顺手统一"。
// ============================================================================

// 统一 metadata 具名键（阶段 1 的显式指定入口）。
const (
	MetadataKeyFirstFrame      = "first_frame_image"
	MetadataKeyLastFrame       = "last_frame_image"
	MetadataKeyReferenceImages = "reference_images"
	MetadataKeyReferenceVideos = "reference_videos"
	MetadataKeyReferenceAudios = "reference_audios"
)

// VideoGenerationMode 生成模式，取值为 constant.TaskAction* 字符串。
// 注意：TaskActionVideoV2Generate 是 hailuo v2 的协议版本标记，不属于模式枚举。
type VideoGenerationMode = string

// ExclusivePolicy 隐式路径下首尾帧与参考素材并存时的处理策略。
// 显式路径（阶段 2 的顶层字段）不受策略影响，一律返回 400。
type ExclusivePolicy int

const (
	// DowngradeFramesToReference 首尾帧降级为参考图，保留素材 —— hailuo v2 现状。
	DowngradeFramesToReference ExclusivePolicy = iota
	// DropFrames 丢弃首尾帧 —— ali minimax 现状。
	DropFrames
)

// MediaLimits 渠道声明的素材上限；0 表示不限制。
type MediaLimits struct {
	MaxFirstFrame     int
	MaxLastFrame      int
	MaxReferenceImage int
	MaxReferenceVideo int
	MaxReferenceAudio int
	// MaxTotalMedia 媒体总数上限（首尾帧 + 参考素材），0 表示不限制。
	MaxTotalMedia int
	// MutualExclusive 首尾帧与参考素材是否互斥（阿里 MiniMax/万相为 true，可灵为 false）。
	MutualExclusive bool
	// OnExclusiveConflict 隐式路径互斥冲突时的行为（仅 MutualExclusive 时生效）。
	OnExclusiveConflict ExclusivePolicy
}

// MediaPlan 跨渠道统一的「模式 + 素材角色」标准化结果。
// 由各渠道在提交期构造，再映射为自己的上游私有结构。
// Mode 是提交期中间产物，不统一回写 info.Action（见设计文档 §3.5）。
type MediaPlan struct {
	Mode            VideoGenerationMode
	Explicit        bool // 是否使用了 metadata 具名键（半显式指定）
	FirstFrame      string
	LastFrame       string
	ReferenceImages []string
	ReferenceVideos []string
	ReferenceAudios []string

	// Limits 为该渠道声明的素材上限，解析时按此截断。
	Limits MediaLimits
}

// HasMedia 是否存在任何媒体素材。
func (p MediaPlan) HasMedia() bool {
	return p.FirstFrame != "" || p.LastFrame != "" ||
		len(p.ReferenceImages) > 0 || len(p.ReferenceVideos) > 0 || len(p.ReferenceAudios) > 0
}

// Images 按「首帧 → 尾帧 → 参考图 → 参考视频 → 参考音频」顺序展平为 URL 列表，
// 供上游只接受扁平图片数组的渠道（如 vidu）使用。
func (p MediaPlan) Images() []string {
	out := make([]string, 0, 1+len(p.ReferenceImages))
	if p.FirstFrame != "" {
		out = append(out, p.FirstFrame)
	}
	if p.LastFrame != "" {
		out = append(out, p.LastFrame)
	}
	out = append(out, p.ReferenceImages...)
	return out
}

// ImplicitImageMapper 渠道对 images 数组隐式推导的覆写钩子。
// 例如阿里万相3.0：单图 → 首帧；多图 → 全部参考图（而非默认的首尾帧）。
type ImplicitImageMapper func(images []string, plan *MediaPlan)

// MediaPlanOptions 渠道声明的解析选项。
type MediaPlanOptions struct {
	Limits MediaLimits
	// ImplicitImages 非 nil 时替代默认数量推导（1 张→首帧，2 张→首尾帧，≥3 张→参考图）。
	// 注意：input_reference / image 兜底仅在默认规则下生效；
	// 自定义规则的渠道若需要该兜底，应在钩子内自行处理。
	ImplicitImages ImplicitImageMapper
}

// BuildMediaPlan 按设计文档 §4.2 的解析流程构造 MediaPlan。
// 阶段 1 不返回校验错误（保留 error 返回值供阶段 2 的显式严格校验使用）。
func BuildMediaPlan(req TaskSubmitReq, opts MediaPlanOptions) (MediaPlan, error) {
	plan := MediaPlan{Limits: opts.Limits}

	firstFrame, hasFirst := MetadataString(req.Metadata, MetadataKeyFirstFrame)
	lastFrame, hasLast := MetadataString(req.Metadata, MetadataKeyLastFrame)
	refImages := MetadataStrings(req.Metadata, MetadataKeyReferenceImages)
	refVideos := MetadataStrings(req.Metadata, MetadataKeyReferenceVideos)
	refAudios := MetadataStrings(req.Metadata, MetadataKeyReferenceAudios)
	plan.Explicit = hasFirst || hasLast || len(refImages) > 0 || len(refVideos) > 0 || len(refAudios) > 0

	// metadata 具名键优先于 images 推导；具名参考素材先入桶，保持与
	// 现有实现（hailuo v2 buildV2Content）一致的素材顺序。
	plan.ReferenceImages = append(plan.ReferenceImages, refImages...)

	switch {
	case hasFirst || hasLast:
		// 显式指定了首/尾帧：images 作为参考素材并入（现状语义）
		plan.FirstFrame = firstFrame
		plan.LastFrame = lastFrame
		plan.ReferenceImages = append(plan.ReferenceImages, nonEmptyStrings(req.Images)...)
	case len(req.Images) > 0:
		if opts.ImplicitImages != nil {
			opts.ImplicitImages(req.Images, &plan)
		} else {
			defaultImplicitImages(req.Images, &plan)
		}
	case opts.ImplicitImages == nil:
		// 无图片且无自定义规则：回落 input_reference / image（现状语义）
		if s := strings.TrimSpace(req.InputReference); s != "" {
			plan.FirstFrame = s
		} else if s := strings.TrimSpace(req.Image); s != "" {
			plan.FirstFrame = s
		}
	}

	plan.ReferenceVideos = append(plan.ReferenceVideos, refVideos...)
	plan.ReferenceAudios = append(plan.ReferenceAudios, refAudios...)

	plan.applyExclusivity(req.Model)
	plan.applyLimits()
	plan.Mode = deriveMode(plan)
	return plan, nil
}

// defaultImplicitImages 默认数量推导：1 张→首帧，2 张→首尾帧，≥3 张→参考图。
func defaultImplicitImages(images []string, plan *MediaPlan) {
	images = nonEmptyStrings(images)
	switch len(images) {
	case 0:
	case 1:
		plan.FirstFrame = images[0]
	case 2:
		plan.FirstFrame, plan.LastFrame = images[0], images[1]
	default:
		plan.ReferenceImages = append(plan.ReferenceImages, images...)
	}
}

// applyExclusivity 互斥处理：隐式路径按渠道策略收敛 + 系统日志。
func (p *MediaPlan) applyExclusivity(model string) {
	if !p.Limits.MutualExclusive {
		return
	}
	hasFrames := p.FirstFrame != "" || p.LastFrame != ""
	hasReference := len(p.ReferenceImages) > 0 || len(p.ReferenceVideos) > 0 || len(p.ReferenceAudios) > 0
	if !hasFrames || !hasReference {
		return
	}

	switch p.Limits.OnExclusiveConflict {
	case DropFrames:
		common.SysLog("video media plan: first_frame/last_frame dropped in favor of reference media (model=" + model + ")")
		p.FirstFrame, p.LastFrame = "", ""
	default: // DowngradeFramesToReference
		common.SysLog("video media plan: first_frame/last_frame downgraded to reference images (model=" + model + ")")
		downgraded := make([]string, 0, 2)
		if p.FirstFrame != "" {
			downgraded = append(downgraded, p.FirstFrame)
		}
		if p.LastFrame != "" {
			downgraded = append(downgraded, p.LastFrame)
		}
		p.ReferenceImages = append(downgraded, p.ReferenceImages...)
		p.FirstFrame, p.LastFrame = "", ""
	}
}

// applyLimits 数量收敛：按角色上限与总数上限截断，保留靠前者。
func (p *MediaPlan) applyLimits() {
	l := p.Limits
	p.ReferenceImages = firstNStrings(p.ReferenceImages, l.MaxReferenceImage)
	p.ReferenceVideos = firstNStrings(p.ReferenceVideos, l.MaxReferenceVideo)
	p.ReferenceAudios = firstNStrings(p.ReferenceAudios, l.MaxReferenceAudio)

	if l.MaxTotalMedia <= 0 {
		return
	}
	remaining := l.MaxTotalMedia
	if p.FirstFrame != "" {
		remaining--
	}
	if p.LastFrame != "" {
		remaining--
	}
	if remaining < 0 {
		remaining = 0
	}
	// 注意：此处 0 表示「无剩余名额」，与角色上限的 0=不限制语义不同，
	// 因此使用 sliceFirstN 而非 firstNStrings。
	p.ReferenceImages = sliceFirstN(p.ReferenceImages, remaining)
	remaining -= len(p.ReferenceImages)
	p.ReferenceVideos = sliceFirstN(p.ReferenceVideos, remaining)
	remaining -= len(p.ReferenceVideos)
	p.ReferenceAudios = sliceFirstN(p.ReferenceAudios, remaining)
}

// deriveMode 由素材组合推导模式：参考素材 → 参考生视频；
// 首尾帧 → 首尾帧插值；仅首帧 → 图生视频；无素材 → 文生视频。
func deriveMode(p MediaPlan) VideoGenerationMode {
	switch {
	case len(p.ReferenceImages) > 0 || len(p.ReferenceVideos) > 0 || len(p.ReferenceAudios) > 0:
		return constant.TaskActionReferenceGenerate
	case p.FirstFrame != "" && p.LastFrame != "":
		return constant.TaskActionFirstTailGenerate
	case p.FirstFrame != "":
		return constant.TaskActionGenerate
	default:
		return constant.TaskActionTextGenerate
	}
}

// HasVideoMediaKeys 请求是否使用了统一 metadata 具名键（阶段 1 的显式指定入口）。
func HasVideoMediaKeys(metadata map[string]any) bool {
	for _, key := range []string{
		MetadataKeyFirstFrame, MetadataKeyLastFrame,
		MetadataKeyReferenceImages, MetadataKeyReferenceVideos, MetadataKeyReferenceAudios,
	} {
		if _, ok := metadata[key]; ok {
			return true
		}
	}
	return false
}

// MetadataString 从 metadata 读取字符串；非字符串或空白返回 false。
func MetadataString(metadata map[string]any, key string) (string, bool) {
	if metadata == nil {
		return "", false
	}
	value, ok := metadata[key].(string)
	if !ok || strings.TrimSpace(value) == "" {
		return "", false
	}
	return strings.TrimSpace(value), true
}

// MetadataStrings 从 metadata 读取字符串数组，兼容 []string / []any / 单个 string 三种写法。
func MetadataStrings(metadata map[string]any, key string) []string {
	if metadata == nil {
		return nil
	}
	raw, ok := metadata[key]
	if !ok {
		return nil
	}
	switch value := raw.(type) {
	case []string:
		return nonEmptyStrings(value)
	case []any:
		result := make([]string, 0, len(value))
		for _, item := range value {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				result = append(result, strings.TrimSpace(s))
			}
		}
		return result
	case string:
		if strings.TrimSpace(value) == "" {
			return nil
		}
		return []string{strings.TrimSpace(value)}
	default:
		return nil
	}
}

// nonEmptyStrings 剔除空白项并 Trim。
func nonEmptyStrings(items []string) []string {
	result := make([]string, 0, len(items))
	for _, s := range items {
		if s = strings.TrimSpace(s); s != "" {
			result = append(result, s)
		}
	}
	return result
}

// firstNStrings 按上限截断；limit <= 0 表示不限制。
func firstNStrings(items []string, limit int) []string {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	return items[:limit]
}

// sliceFirstN 按精确名额截断；n <= 0 返回空（用于总数上限分配）。
func sliceFirstN(items []string, n int) []string {
	if n <= 0 {
		return nil
	}
	if len(items) <= n {
		return items
	}
	return items[:n]
}
