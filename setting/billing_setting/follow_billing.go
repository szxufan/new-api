package billing_setting

import (
	"strings"

	"github.com/samber/lo"
)

// maxFollowDepth bounds how many follow hops ResolveFollow walks, so a long
// or accidentally self-referential chain cannot spin the billing hot path.
const maxFollowDepth = 5

// FollowConfig 描述「跟随其他模型」计费：本模型按目标模型的计费结果 × 系数结算。
// 系数 <= 0 视为 1.0（与前端默认值一致）。
type FollowConfig struct {
	TargetModel string  `json:"target_model"`
	Coefficient float64 `json:"coefficient"`
}

// NormalizedCoefficient 返回修正后的系数：非正数视为 1.0。
func (f FollowConfig) NormalizedCoefficient() float64 {
	if f.Coefficient <= 0 {
		return 1.0
	}
	return f.Coefficient
}

// ---------------------------------------------------------------------------
// Read accessors (hot path, must be fast)
// ---------------------------------------------------------------------------

func GetBillingFollow(model string) (FollowConfig, bool) {
	follow, ok := billingSetting.BillingFollow[model]
	return follow, ok
}

func GetBillingFollowCopy() map[string]FollowConfig {
	return lo.Assign(billingSetting.BillingFollow)
}

// ResolveFollow 沿跟随链解析模型最终落点及其计费目标与累计系数。
// 支持链式跟随（A→B→C），系数逐级相乘；自跟随、环与超过 maxFollowDepth
// 的链路按未配置处理（ok=false，调用方回退到模型自身计费配置）。
func ResolveFollow(model string) (target string, coefficient float64, ok bool) {
	visited := map[string]struct{}{model: {}}
	current := model
	coefficient = 1.0
	found := false
	for range maxFollowDepth {
		follow, exists := billingSetting.BillingFollow[current]
		if !exists {
			break
		}
		next := strings.TrimSpace(follow.TargetModel)
		if next == "" {
			break
		}
		if _, cycle := visited[next]; cycle {
			return "", 0, false
		}
		coefficient *= follow.NormalizedCoefficient()
		visited[next] = struct{}{}
		current = next
		found = true
	}
	if !found {
		return "", 0, false
	}
	return current, coefficient, true
}
