package mcp_setting

import (
	"github.com/QuantumNous/new-api/setting/config"
)

// GroupImageModelSetting 存储分组到文生图模型的映射
type GroupImageModelSetting struct {
	// GroupImageModels 是 group → model 的映射
	// 例如: {"default": "dall-e-3", "vip": "dall-e-3", "svip": "flux-pro-1.1"}
	GroupImageModels map[string]string `json:"group_image_models"`
}

var groupImageModelSetting = GroupImageModelSetting{
	GroupImageModels: map[string]string{
		"default": "dall-e-3",
	},
}

func init() {
	config.GlobalConfig.Register("mcp_setting", &groupImageModelSetting)
}

// GetGroupImageModelSetting 返回 MCP 分组文生图模型配置的指针
func GetGroupImageModelSetting() *GroupImageModelSetting {
	return &groupImageModelSetting
}

// GetGroupImageModel 根据分组名获取配置的文生图模型
// 如果分组未配置，回退到 "default" 分组的模型
// 如果 default 也没有，返回空字符串
func GetGroupImageModel(group string) string {
	if model, ok := groupImageModelSetting.GroupImageModels[group]; ok && model != "" {
		return model
	}
	if model, ok := groupImageModelSetting.GroupImageModels["default"]; ok && model != "" {
		return model
	}
	return ""
}
