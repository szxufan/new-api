package service

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
)

// RetryParam 封装单次请求重试过程中所需的状态与上下文。
//
// 预排序+顺序消费模式：
//   - channelSequence: 首次调用 CacheGetRandomSatisfiedChannel 时一次性构建的有序渠道列表
//   - perChannelAttempts: 每个渠道的尝试次数（由 calcPerChannelAttempts 计算）
//   - groupRanges: auto group 下每个分组在 channelSequence 中的 [start, end) 范围
//
// 重试时通过 getChannelFromSequence 按 channelIndex = retry / perChannelAttempts
// 顺序取出渠道，避免同优先级内重试选到同一渠道。
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

	// channelSequence 预排序的渠道尝试列表（首次调用时初始化）
	channelSequence []*model.Channel
	// perChannelAttempts 每个渠道的尝试次数
	perChannelAttempts int
	// groupRanges auto group 下每个分组的渠道范围 [start, end)
	groupRanges []GroupRange
}

// GroupRange 描述 auto group 中某个分组在 channelSequence 中的范围 [Start, End)。
type GroupRange struct {
	Group string
	Start int
	End   int
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

// calcPerChannelAttempts 计算每个渠道的尝试次数。
//
// 规则：max(1, ceil(RetryTimes / 10))
//   - RetryTimes=0 时返回 1（只尝试一次，不重试）
//   - RetryTimes=10 时返回 1（每个渠道尝试 1 次）
//   - RetryTimes=15 时返回 2
//   - RetryTimes=50 时返回 5
func calcPerChannelAttempts(retryTimes int) int {
	if retryTimes <= 0 {
		return 1
	}
	attempts := (retryTimes + 9) / 10
	if attempts < 1 {
		attempts = 1
	}
	return attempts
}

// getAffinityCountsForGroups 汇总指定分组列表下当前模型的渠道亲和性计数。
//
// 当请求匹配亲和性规则（IsChannelAffinityMatched）时，收集各分组下该模型的所有渠道 ID，
// 通过 GetChannelAffinityCounts 获取精确计数，用于排序时优先选择亲和性计数/权重最小的渠道。
// 未匹配亲和性规则时返回 nil（不启用亲和性感知排序）。
func getAffinityCountsForGroups(ctx *gin.Context, groups []string, modelName string) map[int]int64 {
	if !IsChannelAffinityMatched(ctx) {
		return nil
	}
	if len(groups) == 0 {
		return nil
	}
	// 汇总所有分组下的渠道 ID（去重）
	idSet := make(map[int]struct{})
	for _, g := range groups {
		for _, id := range model.GetChannelIDsForModel(g, modelName) {
			idSet[id] = struct{}{}
		}
	}
	if len(idSet) == 0 {
		return nil
	}
	channelIDs := make([]int, 0, len(idSet))
	for id := range idSet {
		channelIDs = append(channelIDs, id)
	}
	return GetChannelAffinityCounts(channelIDs)
}

// filterGroupBlacklistedChannels 过滤掉分组黑名单命中 userGroup 的渠道。
// userGroup 为用户账号自身分组（ContextKeyUserGroup），不受 token 分组覆盖影响。
// userGroup 为空时不过滤，原样返回。
func filterGroupBlacklistedChannels(ctx *gin.Context, channels []*model.Channel, userGroup string) []*model.Channel {
	if userGroup == "" || len(channels) == 0 {
		return channels
	}
	filtered := make([]*model.Channel, 0, len(channels))
	for _, ch := range channels {
		if ch.IsUserGroupBlacklisted(userGroup) {
			logger.LogDebug(ctx, fmt.Sprintf(
				"channel_select: channel %d skipped by group blacklist (userGroup=%s)", ch.Id, userGroup))
			continue
		}
		filtered = append(filtered, ch)
	}
	return filtered
}

// buildChannelSequence 一次性获取并排序所有可用渠道，返回预排序的渠道切片。
//
// 排序规则（由 model.GetSortedChannelsByPriorityAndAffinity 完成）：
//   - 优先级降序
//   - 同优先级内按亲和性计数/有效权重升序
//   - PreferredChannelTypes 非空时，匹配类型的渠道整体排在前面（仅内存缓存路径）
//
// 对非 auto group：获取 TokenGroup + ModelName 的所有渠道并排序。
// 对 auto group：遍历 autoGroups，逐组获取渠道并排序，按分组顺序合并到同一切片，
// 并通过 groupRanges 记录每个分组在合并切片中的 [start, end) 范围，供跨分组重试使用。
//
// 两种分支都会按 userGroup（用户账号自身分组）过滤掉分组黑名单命中的渠道。
//
// 内存缓存关闭时，model.GetSortedChannelsByPriorityAndAffinity 内部会回退到 DB 路径
// model.GetSortedChannelsDB。
func buildChannelSequence(param *RetryParam, userGroup string) []*model.Channel {
	preferredTypes := param.PreferredChannelTypes

	if param.TokenGroup != "auto" {
		affinityCounts := getAffinityCountsForGroups(param.Ctx, []string{param.TokenGroup}, param.ModelName)
		channels := model.GetSortedChannelsByPriorityAndAffinity(param.TokenGroup, param.ModelName, preferredTypes, affinityCounts)
		return filterGroupBlacklistedChannels(param.Ctx, channels, userGroup)
	}

	// auto group：遍历各分组，逐组获取并排序，按分组顺序合并
	autoGroups := GetUserAutoGroup(userGroup)
	param.groupRanges = make([]GroupRange, 0, len(autoGroups))
	var sequence []*model.Channel

	for _, g := range autoGroups {
		// 每个分组分别获取亲和性计数（不同分组下的渠道 ID 不同）
		affinityCounts := getAffinityCountsForGroups(param.Ctx, []string{g}, param.ModelName)
		channels := model.GetSortedChannelsByPriorityAndAffinity(g, param.ModelName, preferredTypes, affinityCounts)
		// 先按用户分组黑名单过滤，再并入序列，保证 groupRanges 范围正确
		channels = filterGroupBlacklistedChannels(param.Ctx, channels, userGroup)
		if len(channels) == 0 {
			continue
		}
		start := len(sequence)
		sequence = append(sequence, channels...)
		param.groupRanges = append(param.groupRanges, GroupRange{
			Group: g,
			Start: start,
			End:   len(sequence),
		})
	}
	return sequence
}

// getChannelFromSequence 根据全局 retry 计数从预排序列表中按顺序取出渠道。
//
// 映射：channelIndex = retry / perChannelAttempts
//   - 每个 channelIndex 对应一个渠道，每个渠道尝试 perChannelAttempts 次后才前进到下一个渠道
//   - 当 channelIndex 超出 channelSequence 长度时返回 nil（渠道已耗尽）
//
// getChannelFromSequence 根据全局 retry 计数从预排序列表中取渠道。
//
// channelIndex = retry / perChannelAttempts，每个渠道尝试 perChannelAttempts 次后切换到下一个。
// 不检查渠道状态——如果渠道在重试过程中被禁用，relay handler 会失败，
// processChannelError 处理后 shouldRetry 判断继续，retry 递增自然前进到下一个渠道。
func getChannelFromSequence(param *RetryParam) *model.Channel {
	if len(param.channelSequence) == 0 {
		return nil
	}
	if param.perChannelAttempts <= 0 {
		param.perChannelAttempts = 1
	}
	channelIndex := param.GetRetry() / param.perChannelAttempts
	if channelIndex >= len(param.channelSequence) {
		return nil
	}
	return param.channelSequence[channelIndex]
}

// determineSelectGroup 根据当前 retry 对应的 channelIndex 从 groupRanges 中找到所属分组。
//
// auto group 下，channelSequence 按分组顺序拼接，每个 GroupRange 记录了 [Start, End) 范围。
// 通过 channelIndex 落入哪个 range 即可确定实际使用的分组。
// 未匹配时返回空字符串。
func determineSelectGroup(param *RetryParam) string {
	if len(param.groupRanges) == 0 {
		return ""
	}
	if param.perChannelAttempts <= 0 {
		param.perChannelAttempts = 1
	}
	channelIndex := param.GetRetry() / param.perChannelAttempts
	for _, r := range param.groupRanges {
		if channelIndex >= r.Start && channelIndex < r.End {
			return r.Group
		}
	}
	return ""
}

// updateAutoGroupContext 更新 auto group 跨分组重试所需的 context 状态。
//
// 当 channelIndex 超出当前分组的范围时，意味着即将切换到下一个分组，
// 此时需要更新 ContextKeyAutoGroupIndex 以反映新的分组索引，
// 并通过 ResetRetryNextTry 避免下次重试时 retry 被错误递增。
//
// 参数:
//   - param: 重试参数（包含 groupRanges 与 retry 状态）
//   - currentGroupIndex: 当前所属分组在 groupRanges 中的索引
//   - crossGroupRetry: 是否启用了跨分组重试
func updateAutoGroupContext(param *RetryParam, currentGroupIndex int, crossGroupRetry bool) {
	if len(param.groupRanges) == 0 {
		return
	}
	if param.perChannelAttempts <= 0 {
		param.perChannelAttempts = 1
	}
	channelIndex := param.GetRetry() / param.perChannelAttempts
	currentRange := param.groupRanges[currentGroupIndex]

	// channelIndex 仍在当前分组范围内，无需切换
	if channelIndex < currentRange.End {
		common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, currentGroupIndex)
		return
	}

	// 已超出当前分组范围，准备切换到下一个分组
	// 本次请求仍使用当前渠道（由调用方已取出），但 context 状态更新为下一分组索引
	nextGroupIndex := currentGroupIndex + 1
	if crossGroupRetry && nextGroupIndex < len(param.groupRanges) {
		logger.LogDebug(param.Ctx, "auto group cross-group switch: from %s (range=[%d,%d)) to %s, channelIndex=%d",
			currentRange.Group, currentRange.Start, currentRange.End,
			param.groupRanges[nextGroupIndex].Group, channelIndex)
		common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, nextGroupIndex)
		param.SetRetry(0)
		param.ResetRetryNextTry()
	}
}

// GetSortedChannelList 返回预排序的渠道列表（首次调用时构建）。
// 取代旧的 CacheGetRandomSatisfiedChannel 的"选择"逻辑，改为一次性排序后顺序消费。
// 对 auto group：设置 ContextKeyAutoGroup 为第一个分组。
func GetSortedChannelList(param *RetryParam) []*model.Channel {
	if param.channelSequence == nil {
		userGroup := common.GetContextKeyString(param.Ctx, constant.ContextKeyUserGroup)
		param.channelSequence = buildChannelSequence(param, userGroup)
		param.perChannelAttempts = calcPerChannelAttempts(common.RetryTimes)

		// auto group：设置首个分组到 context
		if param.TokenGroup == "auto" && len(param.groupRanges) > 0 {
			firstGroup := param.groupRanges[0].Group
			common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroup, firstGroup)
		}
	}
	return param.channelSequence
}

// ResetChannelSequence 清空缓存的渠道序列，使下次 GetSortedChannelList 重新构建。
// 用于轮次循环中后续轮次重新获取渠道列表。
func (p *RetryParam) ResetChannelSequence() {
	p.channelSequence = nil
	p.groupRanges = nil
}

// CalcPerChannelAttempts 导出 perChannelAttempts 计算逻辑供 controller 使用。
func CalcPerChannelAttempts(retryTimes int) int {
	return calcPerChannelAttempts(retryTimes)
}

// CacheGetRandomSatisfiedChannel 尝试获取一个满足要求的渠道。
//
// 预排序+顺序消费模式：
//   - 首次调用时通过 buildChannelSequence 一次性获取并排序所有可用渠道
//   - 每个渠道尝试 perChannelAttempts 次（由 calcPerChannelAttempts(common.RetryTimes) 计算）
//   - 重试时按顺序前进到下一个渠道，避免同优先级内重试选到同一渠道
//
// 对 auto tokenGroup：
//   - channelSequence 按分组顺序拼接，每个分组内的渠道尝试 perChannelAttempts 次后自然进入下一分组的渠道
//   - 通过 groupRanges 判断当前所属分组，并更新 ContextKeyAutoGroup 等 context 状态
//   - 当 setting.GetAutoGroups() 为空时返回错误 "auto groups is not enabled"
//
// 返回:
//   - (channel, selectGroup, nil): 选中渠道及其所属分组
//   - (nil, selectGroup, nil): 渠道序列已耗尽
//   - (nil, selectGroup, error): auto group 未启用等错误
func CacheGetRandomSatisfiedChannel(param *RetryParam) (*model.Channel, string, error) {
	selectGroup := param.TokenGroup
	userGroup := common.GetContextKeyString(param.Ctx, constant.ContextKeyUserGroup)

	// auto group 前置校验
	if param.TokenGroup == "auto" {
		if len(setting.GetAutoGroups()) == 0 {
			return nil, selectGroup, errors.New("auto groups is not enabled")
		}
	}

	// 首次调用时构建渠道序列
	if param.channelSequence == nil {
		param.channelSequence = buildChannelSequence(param, userGroup)
		param.perChannelAttempts = calcPerChannelAttempts(common.RetryTimes)
	}

	channel := getChannelFromSequence(param)
	if channel == nil {
		return nil, selectGroup, nil
	}

	// auto group: 确定实际分组并更新跨分组重试 context 状态
	if param.TokenGroup == "auto" {
		selectGroup = determineSelectGroup(param)
		if selectGroup != "" {
			common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroup, selectGroup)
			// 计算当前所属分组在 groupRanges 中的索引，更新跨分组 context 状态
			currentGroupIndex := 0
			for i, r := range param.groupRanges {
				if r.Group == selectGroup {
					currentGroupIndex = i
					break
				}
			}
			crossGroupRetry := common.GetContextKeyBool(param.Ctx, constant.ContextKeyTokenCrossGroupRetry)
			updateAutoGroupContext(param, currentGroupIndex, crossGroupRetry)
		}
	}

	logger.LogDebug(param.Ctx, fmt.Sprintf(
		"channel_select: tokenGroup=%s model=%s retry=%d perChannelAttempts=%d channelIndex=%d channelId=%d selectGroup=%s",
		param.TokenGroup, param.ModelName, param.GetRetry(), param.perChannelAttempts,
		func() int {
			if param.perChannelAttempts <= 0 {
				param.perChannelAttempts = 1
			}
			return param.GetRetry() / param.perChannelAttempts
		}(),
		channel.Id, selectGroup,
	))

	return channel, selectGroup, nil
}
