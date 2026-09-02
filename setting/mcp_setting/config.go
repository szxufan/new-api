package mcp_setting

import (
	"github.com/QuantumNous/new-api/setting/config"
)

// GroupImageModelSetting 存储 MCP 各能力「分组 → 模型」的映射配置。
// 每类能力（文生图/图生图/文生视频/图生视频/首尾帧/参考图生视频）独立配置，
// 未配置的分组回退到 default 分组的模型，default 也未配置时返回空串（调用方报错）。
type GroupImageModelSetting struct {
	// GroupImageModels 文生图模型映射（generate_image 纯文生图路径）
	GroupImageModels map[string]string `json:"group_image_models"`
	// GroupI2IModels 图生图模型映射（generate_image 带 image_ids 路径）
	GroupI2IModels map[string]string `json:"group_i2i_models,omitempty"`
	// GroupVideoT2VModels 文生视频模型映射（generate_video）
	GroupVideoT2VModels map[string]string `json:"group_video_t2v_models,omitempty"`
	// GroupVideoI2VModels 图生视频（仅首帧）模型映射（generate_video_from_frames 仅传 first_frame_id）
	GroupVideoI2VModels map[string]string `json:"group_video_i2v_models,omitempty"`
	// GroupVideoKF2VModels 首尾帧生视频模型映射（generate_video_from_frames 同时传 first/last）
	GroupVideoKF2VModels map[string]string `json:"group_video_kf2v_models,omitempty"`
	// GroupVideoR2VModels 参考图生视频模型映射（generate_video_from_reference）
	GroupVideoR2VModels map[string]string `json:"group_video_r2v_models,omitempty"`
}

var groupImageModelSetting = GroupImageModelSetting{
	GroupImageModels: map[string]string{
		"default": "dall-e-3",
	},
	GroupI2IModels:       map[string]string{},
	GroupVideoT2VModels:  map[string]string{},
	GroupVideoI2VModels:  map[string]string{},
	GroupVideoKF2VModels: map[string]string{},
	GroupVideoR2VModels:  map[string]string{},
}

func init() {
	config.GlobalConfig.Register("mcp_setting", &groupImageModelSetting)
}

// GetGroupImageModelSetting 返回 MCP 分组模型配置的指针
func GetGroupImageModelSetting() *GroupImageModelSetting {
	return &groupImageModelSetting
}

// GetGroupImageModel 根据分组名获取配置的文生图模型
// 如果分组未配置，回退到 "default" 分组的模型
// 如果 default 也没有，返回空字符串
func GetGroupImageModel(group string) string {
	return lookupGroupModel(groupImageModelSetting.GroupImageModels, group)
}

// GetGroupI2IModel 根据分组名获取配置的图生图模型（generate_image 带 image_ids 时使用）
func GetGroupI2IModel(group string) string {
	return lookupGroupModel(groupImageModelSetting.GroupI2IModels, group)
}

// 视频模型池 kind 常量（GetGroupVideoModel 的 kind 参数取值）
const (
	VideoModelKindT2V = "t2v" // 文生视频
	VideoModelKindI2V = "i2v" // 图生视频（仅首帧）
	VideoModelKindKF2V = "kf2v" // 首尾帧生视频
	VideoModelKindR2V = "r2v" // 参考图生视频
)

// GetGroupVideoModel 根据视频模型池类型和分组名获取配置的视频生成模型
// kind 取值：VideoModelKindT2V / VideoModelKindI2V / VideoModelKindKF2V / VideoModelKindR2V
func GetGroupVideoModel(kind string, group string) string {
	var m map[string]string
	switch kind {
	case VideoModelKindT2V:
		m = groupImageModelSetting.GroupVideoT2VModels
	case VideoModelKindI2V:
		m = groupImageModelSetting.GroupVideoI2VModels
	case VideoModelKindKF2V:
		m = groupImageModelSetting.GroupVideoKF2VModels
	case VideoModelKindR2V:
		m = groupImageModelSetting.GroupVideoR2VModels
	default:
		return ""
	}
	return lookupGroupModel(m, group)
}

// lookupGroupModel 分组 → default → 空串 的回退查找
func lookupGroupModel(m map[string]string, group string) string {
	if model, ok := m[group]; ok && model != "" {
		return model
	}
	if model, ok := m["default"]; ok && model != "" {
		return model
	}
	return ""
}
