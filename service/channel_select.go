package service

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
)

// getUsedChannelIDsFromContext 从 gin.Context 的 "use_channel" 键读取已使用的渠道ID字符串切片，
// 转换为 []int 用于在同一优先级层级内排除已用渠道。
// 转换失败的条目会被跳过（不影响其他条目）。
func getUsedChannelIDsFromContext(c *gin.Context) []int {
	useChannelStr := c.GetStringSlice("use_channel")
	if len(useChannelStr) == 0 {
		return nil
	}
	result := make([]int, 0, len(useChannelStr))
	for _, s := range useChannelStr {
		id, err := strconv.Atoi(s)
		if err != nil {
			continue
		}
		result = append(result, id)
	}
	return result
}

type RetryParam struct {
	Ctx          *gin.Context
	TokenGroup   string
	ModelName    string
	Retry        *int
	resetNextTry bool
	// PreferredChannelTypes 优先渠道类型列表（按顺序）。
	// 为空或 nil 时不启用类型优先选择。
	PreferredChannelTypes []int
	// channelTypeFallbackLogged 用于确保 fallback 日志只打印一次
	channelTypeFallbackLogged bool
}

func (p *RetryParam) GetRetry() int {
	if p.Retry == nil {
		return 0
	}
	return *p.Retry
}

func (p *RetryParam) SetRetry(retry int) {
	p.Retry = &retry
}

func (p *RetryParam) IncreaseRetry() {
	if p.resetNextTry {
		p.resetNextTry = false
		return
	}
	if p.Retry == nil {
		p.Retry = new(int)
	}
	*p.Retry++
}

func (p *RetryParam) ResetRetryNextTry() {
	p.resetNextTry = true
}

// CacheGetRandomSatisfiedChannel tries to get a random channel that satisfies the requirements.
// 尝试获取一个满足要求的随机渠道。
//
// For "auto" tokenGroup with cross-group Retry enabled:
// 对于启用了跨分组重试的 "auto" tokenGroup：
//
//   - Each group will exhaust all its priorities before moving to the next group.
//     每个分组会用完所有优先级后才会切换到下一个分组。
//
//   - Uses ContextKeyAutoGroupIndex to track current group index.
//     使用 ContextKeyAutoGroupIndex 跟踪当前分组索引。
//
//   - Uses ContextKeyAutoGroupRetryIndex to track the global Retry count when current group started.
//     使用 ContextKeyAutoGroupRetryIndex 跟踪当前分组开始时的全局重试次数。
//
//   - priorityRetry = Retry - startRetryIndex, represents the priority level within current group.
//     priorityRetry = Retry - startRetryIndex，表示当前分组内的优先级级别。
//
//   - When GetRandomSatisfiedChannel returns nil (priorities exhausted), moves to next group.
//     当 GetRandomSatisfiedChannel 返回 nil（优先级用完）时，切换到下一个分组。
//
// Example flow (2 groups, each with 2 priorities, RetryTimes=3):
// 示例流程（2个分组，每个有2个优先级，RetryTimes=3）：
//
//	Retry=0: GroupA, priority0 (startRetryIndex=0, priorityRetry=0)
//	         分组A, 优先级0
//
//	Retry=1: GroupA, priority1 (startRetryIndex=0, priorityRetry=1)
//	         分组A, 优先级1
//
//	Retry=2: GroupA exhausted → GroupB, priority0 (startRetryIndex=2, priorityRetry=0)
//	         分组A用完 → 分组B, 优先级0
//
//	Retry=3: GroupB, priority1 (startRetryIndex=2, priorityRetry=1)
//	         分组B, 优先级1
func CacheGetRandomSatisfiedChannel(param *RetryParam) (*model.Channel, string, error) {
	var channel *model.Channel
	var err error
	selectGroup := param.TokenGroup
	userGroup := common.GetContextKeyString(param.Ctx, constant.ContextKeyUserGroup)
	// 读取当前请求已使用的渠道ID列表，用于在同一优先级层级内排除
	usedChannelIDs := getUsedChannelIDsFromContext(param.Ctx)

	if param.TokenGroup == "auto" {
		if len(setting.GetAutoGroups()) == 0 {
			return nil, selectGroup, errors.New("auto groups is not enabled")
		}
		autoGroups := GetUserAutoGroup(userGroup)

		// startGroupIndex: the group index to start searching from
		// startGroupIndex: 开始搜索的分组索引
		startGroupIndex := 0
		crossGroupRetry := common.GetContextKeyBool(param.Ctx, constant.ContextKeyTokenCrossGroupRetry)

		if lastGroupIndex, exists := common.GetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex); exists {
			if idx, ok := lastGroupIndex.(int); ok {
				startGroupIndex = idx
			}
		}

		for i := startGroupIndex; i < len(autoGroups); i++ {
			autoGroup := autoGroups[i]
			// Calculate priorityRetry for current group
			// 计算当前分组的 priorityRetry
			priorityRetry := param.GetRetry()
			// If moved to a new group, reset priorityRetry and update startRetryIndex
			// 如果切换到新分组，重置 priorityRetry 并更新 startRetryIndex
			if i > startGroupIndex {
				priorityRetry = 0
			}
			logger.LogDebug(param.Ctx, "Auto selecting group: %s, priorityRetry: %d", autoGroup, priorityRetry)

			// 同优先级内重试机制适配：
			// priorityRetry 是分组内的优先级层级索引（原设计语义），
			// 但底层 selectChannelFromList 内部会做 retry/budget 映射。
			// 为避免双重映射，将 priorityRetry 乘以 budget 后传给底层，
			// 使底层 (priorityRetry * budget) / budget = priorityRetry，还原为正确的层级索引。
			budget := model.CalcSamePriorityRetryBudget(common.RetryTimes)
			mappedRetry := priorityRetry
			if budget > 0 {
				mappedRetry = priorityRetry * budget
			}

			// 类型优先：auto group 每个分组都使用偏好类型过滤
			if len(param.PreferredChannelTypes) > 0 {
				var fellBack bool
				channel, err, fellBack = model.GetRandomSatisfiedChannelWithPreference(
					autoGroup, param.ModelName, mappedRetry,
					param.PreferredChannelTypes, usedChannelIDs,
				)
				if fellBack && !param.channelTypeFallbackLogged {
					param.channelTypeFallbackLogged = true
					logger.LogInfo(param.Ctx, fmt.Sprintf(
						"[channel_type_preference] fallback: group=%s model=%s types=%v exhausted, fallback to all",
						autoGroup, param.ModelName, param.PreferredChannelTypes,
					))
				}
			} else {
				channel, _ = model.GetRandomSatisfiedChannel(autoGroup, param.ModelName, mappedRetry, usedChannelIDs)
			}

			if err != nil {
				return nil, selectGroup, err
			}

			if channel == nil {
				// Current group has no available channel for this model, try next group
				// 当前分组没有该模型的可用渠道，尝试下一个分组
				logger.LogDebug(param.Ctx, "No available channel in group %s for model %s at priorityRetry %d, trying next group", autoGroup, param.ModelName, priorityRetry)
				// 重置状态以尝试下一个分组
				common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, i+1)
				common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupRetryIndex, 0)
				// Reset retry counter so outer loop can continue for next group
				// 重置重试计数器，以便外层循环可以为下一个分组继续
				param.SetRetry(0)
				continue
			}
			common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroup, autoGroup)
			selectGroup = autoGroup
			logger.LogDebug(param.Ctx, "Auto selected group: %s", autoGroup)

			// Prepare state for next retry
			// 为下一次重试准备状态
			if crossGroupRetry && priorityRetry >= common.RetryTimes {
				// Current group has exhausted all retries, prepare to switch to next group
				// This request still uses current group, but next retry will use next group
				// 当前分组已用完所有重试次数，准备切换到下一个分组
				// 本次请求仍使用当前分组，但下次重试将使用下一个分组
				logger.LogDebug(param.Ctx, "Current group %s retries exhausted (priorityRetry=%d >= RetryTimes=%d), preparing switch to next group for next retry", autoGroup, priorityRetry, common.RetryTimes)
				common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, i+1)
				// Reset retry counter so outer loop can continue for next group
				// 重置重试计数器，以便外层循环可以为下一个分组继续
				param.SetRetry(0)
				param.ResetRetryNextTry()
			} else {
				// Stay in current group, save current state
				// 保持在当前分组，保存当前状态
				common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, i)
			}
			break
		}
	} else {
		// 非 auto group 分支
		if len(param.PreferredChannelTypes) > 0 {
			var fellBack bool
			channel, err, fellBack = model.GetRandomSatisfiedChannelWithPreference(
				param.TokenGroup, param.ModelName, param.GetRetry(),
				param.PreferredChannelTypes, usedChannelIDs,
			)
			if fellBack && !param.channelTypeFallbackLogged {
				param.channelTypeFallbackLogged = true
				logger.LogInfo(param.Ctx, fmt.Sprintf(
					"[channel_type_preference] fallback: group=%s model=%s types=%v exhausted, fallback to all",
					param.TokenGroup, param.ModelName, param.PreferredChannelTypes,
				))
			}
		} else {
			channel, err = model.GetRandomSatisfiedChannel(param.TokenGroup, param.ModelName, param.GetRetry(), usedChannelIDs)
		}
		if err != nil {
			return nil, param.TokenGroup, err
		}
	}
	return channel, selectGroup, nil
}
