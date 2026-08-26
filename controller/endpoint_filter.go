package controller

import (
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// parseEndpointTypes 解析 ?endpoint=a,b,c（逗号分隔多值）查询参数为端点类型列表。
// 未传或为空时返回 nil，调用方保持原有（不过滤）行为。
func parseEndpointTypes(c *gin.Context) []constant.EndpointType {
	raw := strings.TrimSpace(c.Query("endpoint"))
	if raw == "" {
		return nil
	}
	var endpointTypes []constant.EndpointType
	for _, part := range strings.Split(raw, ",") {
		if part = strings.TrimSpace(part); part != "" {
			endpointTypes = append(endpointTypes, constant.EndpointType(part))
		}
	}
	return endpointTypes
}

// filterModelsByEndpoints 返回 models 中「支持 want 中任一端点类型」的子集。
// want 为空时原样返回，保持兼容。
// 支持端点信息来自 model.GetModelSupportEndpointTypes（启用渠道的类型映射 + 模型元数据自定义端点）。
func filterModelsByEndpoints(models []string, want []constant.EndpointType) []string {
	if len(want) == 0 {
		return models
	}
	filtered := make([]string, 0, len(models))
	for _, m := range models {
		if modelSupportsAnyEndpoint(m, want) {
			filtered = append(filtered, m)
		}
	}
	return filtered
}

// modelSupportsAnyEndpoint 判断模型是否支持 want 中的任一端点类型。
func modelSupportsAnyEndpoint(modelName string, want []constant.EndpointType) bool {
	for _, et := range model.GetModelSupportEndpointTypes(modelName) {
		if slices.Contains(want, et) {
			return true
		}
	}
	return false
}

// groupContainsSupportedEnabledModel 判断 group 下是否存在启用渠道支持 want 端点的模型。
// want 为空恒为 true（不执行额外查询，保持兼容）。
func groupContainsSupportedEnabledModel(group string, want []constant.EndpointType) bool {
	if len(want) == 0 {
		return true
	}
	for _, m := range model.GetGroupEnabledModels(group) {
		if modelSupportsAnyEndpoint(m, want) {
			return true
		}
	}
	return false
}
