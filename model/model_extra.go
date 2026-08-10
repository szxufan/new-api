package model

import "strings"

func GetModelEnableGroups(modelName string) []string {
	// 确保缓存最新
	GetPricing()

	if modelName == "" {
		return make([]string, 0)
	}

	modelEnableGroupsLock.RLock()
	groups, ok := modelEnableGroups[modelName]
	modelEnableGroupsLock.RUnlock()
	if !ok {
		return make([]string, 0)
	}
	return groups
}

// GetModelQuotaTypes 返回指定模型的计费类型集合（来自缓存）
func GetModelQuotaTypes(modelName string) []int {
	GetPricing()

	modelEnableGroupsLock.RLock()
	quota, ok := modelQuotaTypeMap[modelName]
	modelEnableGroupsLock.RUnlock()
	if !ok {
		return []int{}
	}
	return []int{quota}
}

// GetModelFallbackModel 返回指定模型的降级模型名称。
// 查找顺序：精确匹配 → 前缀匹配 → 后缀匹配 → 包含匹配。
// 精确匹配通过 modelFallbackModel 缓存实现；
// 规则匹配通过 modelFallback{Prefix,Suffix,Contains}List 缓存实现，
// 用于支持 NameRule 为前缀/后缀/包含匹配的模型元数据。
func GetModelFallbackModel(modelName string) string {
	if modelName == "" {
		return ""
	}
	GetPricing()
	modelFallbackLock.RLock()
	defer modelFallbackLock.RUnlock()

	// 1. 精确匹配
	if fb, ok := modelFallbackModel[modelName]; ok {
		return fb
	}

	// 2. 前缀匹配：请求的模型名以元数据模型名为前缀
	for _, m := range modelFallbackPrefixList {
		if strings.HasPrefix(modelName, m.ModelName) {
			return m.FallbackModel
		}
	}

	// 3. 后缀匹配：请求的模型名以元数据模型名为后缀
	for _, m := range modelFallbackSuffixList {
		if strings.HasSuffix(modelName, m.ModelName) {
			return m.FallbackModel
		}
	}

	// 4. 包含匹配：请求的模型名包含元数据模型名
	for _, m := range modelFallbackContainsList {
		if strings.Contains(modelName, m.ModelName) {
			return m.FallbackModel
		}
	}

	return ""
}
