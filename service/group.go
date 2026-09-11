package service

import (
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

func GetUserUsableGroups(userGroup string) map[string]string {
	groupsCopy := setting.GetUserUsableGroupsCopy()
	if userGroup != "" {
		specialSettings, b := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.Get(userGroup)
		if b {
			// 处理特殊可用分组
			for specialGroup, desc := range specialSettings {
				if strings.HasPrefix(specialGroup, "-:") {
					// 移除分组
					groupToRemove := strings.TrimPrefix(specialGroup, "-:")
					delete(groupsCopy, groupToRemove)
				} else if strings.HasPrefix(specialGroup, "+:") {
					// 添加分组
					groupToAdd := strings.TrimPrefix(specialGroup, "+:")
					groupsCopy[groupToAdd] = desc
				} else {
					// 直接添加分组
					groupsCopy[specialGroup] = desc
				}
			}
		}
		// 如果userGroup不在UserUsableGroups中，返回UserUsableGroups + userGroup
		if _, ok := groupsCopy[userGroup]; !ok {
			groupsCopy[userGroup] = "用户分组"
		}
	}
	return groupsCopy
}

func GroupInUserUsableGroups(userGroup, groupName string) bool {
	_, ok := GetUserUsableGroups(userGroup)[groupName]
	return ok
}

// GetUserAutoGroup 根据用户分组获取自动分组设置
func GetUserAutoGroup(userGroup string) []string {
	groups := GetUserUsableGroups(userGroup)
	autoGroups := make([]string, 0)
	for _, group := range setting.GetAutoGroups() {
		if _, ok := groups[group]; ok {
			autoGroups = append(autoGroups, group)
		}
	}
	return autoGroups
}

// FindGroupForModel 在用户可用分组中查找提供指定模型的分组。
// userGroup 为用户账号自身分组；可用分组 = GetUserUsableGroups(userGroup) 的键集合。
// 匹配顺序：优先 userGroup（若在可用集合内且拥有该模型），其余按分组名字典序遍历，
// 返回第一个命中分组；模型名匹配与渠道选择保持一致（FormatMatchingModelName 归一化）。
// 返回 (分组名, true)；无任何可用分组拥有该模型时返回 ("", false)。
func FindGroupForModel(userGroup, modelName string) (string, bool) {
	if modelName == "" {
		return "", false
	}
	normalized := ratio_setting.FormatMatchingModelName(modelName)
	groups := GetUserUsableGroups(userGroup)

	// 构造遍历顺序：优先当前分组，其余按字典序
	order := make([]string, 0, len(groups))
	if _, ok := groups[userGroup]; ok {
		order = append(order, userGroup)
	}
	rest := make([]string, 0, len(groups))
	for g := range groups {
		if g != userGroup {
			rest = append(rest, g)
		}
	}
	sort.Strings(rest)
	order = append(order, rest...)

	for _, g := range order {
		for _, m := range model.GetGroupEnabledModels(g) {
			if m == modelName || m == normalized {
				return g, true
			}
		}
	}
	return "", false
}

// GetUserGroupRatio 获取用户使用某个分组的倍率
// userGroup 用户分组
// group 需要获取倍率的分组
func GetUserGroupRatio(userGroup, group string) float64 {
	ratio, ok := ratio_setting.GetGroupGroupRatio(userGroup, group)
	if ok {
		return ratio
	}
	return ratio_setting.GetGroupRatio(group)
}
