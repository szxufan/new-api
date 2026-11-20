package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
)

// ParseModelPurposes 解析并校验 JSON 字符串数组，按输入顺序去重。
func ParseModelPurposes(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if common.GetJsonType([]byte(raw)) != "array" {
		return nil, errors.New("模型用途必须是字符串数组")
	}

	var purposes []string
	if err := common.UnmarshalJsonStr(raw, &purposes); err != nil {
		return nil, fmt.Errorf("模型用途不是合法的字符串数组: %w", err)
	}
	normalized := make([]string, 0, len(purposes))
	seen := make(map[string]struct{}, len(purposes))
	for _, purpose := range purposes {
		if !constant.IsValidModelPurpose(purpose) {
			return nil, fmt.Errorf("未知的模型用途: %s", purpose)
		}
		if _, ok := seen[purpose]; ok {
			continue
		}
		seen[purpose] = struct{}{}
		normalized = append(normalized, purpose)
	}
	return normalized, nil
}

// NormalizeModelPurposes 将模型用途规范化为可存储的紧凑 JSON 数组字符串。
func NormalizeModelPurposes(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	purposes, err := ParseModelPurposes(raw)
	if err != nil {
		return "", err
	}
	data, err := common.Marshal(purposes)
	if err != nil {
		return "", fmt.Errorf("模型用途序列化失败: %w", err)
	}
	return string(data), nil
}
