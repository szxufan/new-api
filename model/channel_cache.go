package model

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

var group2model2channels map[string]map[string][]int // enabled channel
var channelsIDM map[int]*Channel                     // all channels include disabled
var channelSyncLock sync.RWMutex

// getInt64FromMap 从 map[string]interface{} 中安全读取 int64 值。
// encoding/json 将 JSON 数字反序列化为 float64 而非 int64，
// 此函数处理该类型差异，避免类型断言失败。
func getInt64FromMap(m map[string]interface{}, key string) (int64, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch val := v.(type) {
	case float64:
		return int64(val), true
	case int64:
		return val, true
	default:
		return 0, false
	}
}

func InitChannelCache() {
	if !common.MemoryCacheEnabled {
		return
	}
	newChannelId2channel := make(map[int]*Channel)
	var channels []*Channel
	DB.Find(&channels)
	now := time.Now().Unix()
	for _, channel := range channels {
		// 检查限流状态是否过期
		if channel.Status == common.ChannelStatusRateLimited429 || channel.Status == common.ChannelStatusManuallyRateLimited {
			info := channel.GetOtherInfo()
			if until, ok := getInt64FromMap(info, "rate_limit_until"); ok {
				if until > now {
					// 仍在限流期内，保持限流状态
					newChannelId2channel[channel.Id] = channel
					continue
				}
				// 限流已过期，恢复为启用状态
				channel.Status = common.ChannelStatusEnabled
				delete(info, "rate_limit_until")
				delete(info, "rate_limit_reason")
				channel.SetOtherInfo(info)
				_ = channel.SaveWithoutKey()
			}
		}
		newChannelId2channel[channel.Id] = channel
	}
	var abilities []*Ability
	DB.Find(&abilities)
	groups := make(map[string]bool)
	for _, ability := range abilities {
		groups[ability.Group] = true
	}
	newGroup2model2channels := make(map[string]map[string][]int)
	for group := range groups {
		newGroup2model2channels[group] = make(map[string][]int)
	}
	for _, channel := range channels {
		if channel.Status != common.ChannelStatusEnabled {
			continue // skip disabled channels
		}
		groups := strings.Split(channel.Group, ",")
		for _, group := range groups {
			models := strings.Split(channel.Models, ",")
			for _, model := range models {
				if _, ok := newGroup2model2channels[group][model]; !ok {
					newGroup2model2channels[group][model] = make([]int, 0)
				}
				newGroup2model2channels[group][model] = append(newGroup2model2channels[group][model], channel.Id)
			}
		}
	}

	// sort by priority
	for group, model2channels := range newGroup2model2channels {
		for model, channels := range model2channels {
			sort.Slice(channels, func(i, j int) bool {
				return newChannelId2channel[channels[i]].GetPriority() > newChannelId2channel[channels[j]].GetPriority()
			})
			newGroup2model2channels[group][model] = channels
		}
	}

	channelSyncLock.Lock()
	group2model2channels = newGroup2model2channels
	//channelsIDM = newChannelId2channel
	for i, channel := range newChannelId2channel {
		if channel.ChannelInfo.IsMultiKey {
			channel.Keys = channel.GetKeys()
			if channel.ChannelInfo.MultiKeyMode == constant.MultiKeyModePolling {
				if oldChannel, ok := channelsIDM[i]; ok {
					// 存在旧的渠道，如果是多key且轮询，保留轮询索引信息
					if oldChannel.ChannelInfo.IsMultiKey && oldChannel.ChannelInfo.MultiKeyMode == constant.MultiKeyModePolling {
						channel.ChannelInfo.MultiKeyPollingIndex = oldChannel.ChannelInfo.MultiKeyPollingIndex
					}
				}
			}
		}
	}
	channelsIDM = newChannelId2channel
	channelSyncLock.Unlock()
	common.SysLog("channels synced from database")
}

func SyncChannelCache(frequency int) {
	for {
		time.Sleep(time.Duration(frequency) * time.Second)
		common.SysLog("syncing channels from database")
		InitChannelCache()
	}
}

// RecoverExpiredRateLimitedChannels 恢复已过期限流的渠道
func RecoverExpiredRateLimitedChannels() {
	now := time.Now().Unix()

	// 查找所有限流状态的渠道
	var channels []*Channel
	DB.Where("status IN ?", []int{common.ChannelStatusRateLimited429, common.ChannelStatusManuallyRateLimited}).Find(&channels)

	for _, channel := range channels {
		info := channel.GetOtherInfo()
		if until, ok := getInt64FromMap(info, "rate_limit_until"); ok && until <= now {
			// 限流已过期，恢复启用
			UpdateChannelStatus(channel.Id, "", common.ChannelStatusEnabled, "")
			common.SysLog(fmt.Sprintf("channel #%d rate limit expired, restored to enabled", channel.Id))
		}

		// 检查Key级别限流
		if channel.ChannelInfo.IsMultiKey && channel.ChannelInfo.MultiKeyRateLimitedUntil != nil {
			changed := false
			for keyIndex, keyUntil := range channel.ChannelInfo.MultiKeyRateLimitedUntil {
				if keyUntil <= now {
					delete(channel.ChannelInfo.MultiKeyRateLimitedUntil, keyIndex)
					// 如果该Key在MultiKeyStatusList中是限流状态，也清除
					if status, ok := channel.ChannelInfo.MultiKeyStatusList[keyIndex]; ok && status == common.ChannelStatusRateLimited429 {
						delete(channel.ChannelInfo.MultiKeyStatusList, keyIndex)
					}
					changed = true
				}
			}
			if changed {
				_ = channel.SaveWithoutKey()
				// 检查是否所有Key都恢复正常
				if len(channel.ChannelInfo.MultiKeyStatusList) < channel.ChannelInfo.MultiKeySize {
					// 更新abilities
					_ = UpdateAbilityStatus(channel.Id, true)
				}
			}
		}
	}
}

// StartRateLimitRecoveryTask 启动限流恢复定时任务
func StartRateLimitRecoveryTask() {
	go func() {
		for {
			time.Sleep(30 * time.Second) // 每30秒检查一次
			RecoverExpiredRateLimitedChannels()
		}
	}()
}

// getChannelIDsForModel 获取分组+模型对应的所有渠道ID（normalized 匹配）
func getChannelIDsForModel(group, model string) []int {
	channels := group2model2channels[group][model]
	if len(channels) == 0 {
		normalizedModel := ratio_setting.FormatMatchingModelName(model)
		channels = group2model2channels[group][normalizedModel]
	}
	return channels
}

// logCacheMiss 记录缓存未命中的调试日志
func logCacheMiss(group, model string) {
	cacheNil := group2model2channels == nil
	groupExists := false
	modelKeys := ""
	modelHex := fmt.Sprintf("%x", model)
	if !cacheNil {
		if models, ok := group2model2channels[group]; ok {
			groupExists = true
			keys := make([]string, 0, len(models))
			for k := range models {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			modelKeys = strings.Join(keys, ",")
			modelLower := strings.ToLower(model)
			for _, k := range keys {
				if strings.ToLower(k) == modelLower {
					logger.LogWarn(nil, fmt.Sprintf("[cache_miss_case_mismatch] requested_model=%s cache_key=%s requested_hex=%s key_hex=%x",
						model, k, modelHex, k))
					break
				}
			}
		}
	}
	logger.LogWarn(nil, fmt.Sprintf("[cache_miss] group=%s model=%s model_hex=%s cache_enabled=true cache_nil=%t group_exists=%t group_models=[%s]",
		group, model, modelHex, cacheNil, groupExists, modelKeys))
}

// CalcSamePriorityRetryBudget 计算同一优先级层级内的重试预算。
//
// 在引入"同优先级内重试"机制后，全局 retry 计数不再直接映射到优先级层级索引，
// 而是先在同一优先级层级内重试 budget 次后再降级到下一层级。
//
// 规则：
//   - RetryTimes <= 0（不重试）时返回 0，保持原行为
//   - 否则返回 max(1, ceil(RetryTimes * 10%))，即总重试次数的 10% 向上取整，至少 1 次
//
// 示例：
//   - RetryTimes=0  → budget=0（不重试）
//   - RetryTimes=1  → budget=1（同层级重试 1 次后降级）
//   - RetryTimes=10 → budget=1
//   - RetryTimes=15 → budget=2
//   - RetryTimes=20 → budget=2
//   - RetryTimes=50 → budget=5
func CalcSamePriorityRetryBudget(retryTimes int) int {
	if retryTimes <= 0 {
		return 0
	}
	// 向上取整 10%：(retryTimes + 9) / 10
	budget := (retryTimes + 9) / 10
	if budget < 1 {
		budget = 1
	}
	return budget
}

// mapRetryToPriorityLevel 将全局 retry 计数映射为优先级层级索引。
//
// 在同优先级内重试 budget 次后再降级到下一优先级层级。
// 当 budget 为 0（RetryTimes=0）时，等价于直接用 retry 作为层级索引（原行为）。
//
// 参数:
//   - retry: 全局重试计数
//   - budget: 同优先级内重试预算（由 CalcSamePriorityRetryBudget 计算）
//   - numPriorities: 优先级层级数量
//
// 返回: 优先级层级索引（已夹取到 [0, numPriorities-1]）
func mapRetryToPriorityLevel(retry, budget, numPriorities int) int {
	if numPriorities <= 0 {
		return 0
	}
	var level int
	if budget <= 0 {
		// RetryTimes=0 或 budget 未启用，回退到原始行为
		level = retry
	} else {
		level = retry / budget
	}
	if level >= numPriorities {
		level = numPriorities - 1
	}
	return level
}

// selectChannelFromList 从给定渠道ID列表中按优先级+权重选择
// 复用现有的分层优先级 + 加权随机算法
//
// 同优先级内重试机制：
//   - retry 不再直接作为优先级层级索引，而是先按 CalcSamePriorityRetryBudget(RetryTimes)
//     计算 budget，在同一层级内重试 budget 次后才降级到下一层级
//   - usedChannelIDs 用于在同一优先级层级内排除已使用的渠道，避免重复选中同一渠道
//   - 若排除后当前层级无可用渠道，回退到不排除（避免无渠道可选）
//
// 返回 nil, nil 表示当前 retry 层级无匹配渠道（非数据错误）
// 返回 nil, error 表示数据一致性问题（渠道 ID 不存在）
func selectChannelFromList(channels []int, retry int, usedChannelIDs []int) (*Channel, error) {
	if len(channels) == 0 {
		return nil, nil
	}
	if len(channels) == 1 {
		if channel, ok := channelsIDM[channels[0]]; ok {
			return channel, nil
		}
		return nil, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", channels[0])
	}

	uniquePriorities := make(map[int]bool)
	for _, channelId := range channels {
		if channel, ok := channelsIDM[channelId]; ok {
			uniquePriorities[int(channel.GetPriority())] = true
		} else {
			return nil, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", channelId)
		}
	}
	var sortedUniquePriorities []int
	for priority := range uniquePriorities {
		sortedUniquePriorities = append(sortedUniquePriorities, priority)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(sortedUniquePriorities)))

	// 同优先级内重试：将全局 retry 映射为优先级层级索引
	budget := CalcSamePriorityRetryBudget(common.RetryTimes)
	priorityLevel := mapRetryToPriorityLevel(retry, budget, len(uniquePriorities))
	targetPriority := int64(sortedUniquePriorities[priorityLevel])

	var sumWeight = 0
	var targetChannels []*Channel
	for _, channelId := range channels {
		if channel, ok := channelsIDM[channelId]; ok {
			if channel.GetPriority() == targetPriority {
				sumWeight += channel.GetWeight()
				targetChannels = append(targetChannels, channel)
			}
		} else {
			return nil, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", channelId)
		}
	}

	if len(targetChannels) == 0 {
		return nil, nil
	}

	// 在同一优先级层级内排除已使用的渠道，避免重复选中
	if len(usedChannelIDs) > 0 && len(targetChannels) > 1 {
		usedSet := make(map[int]bool, len(usedChannelIDs))
		for _, id := range usedChannelIDs {
			usedSet[id] = true
		}
		filtered := make([]*Channel, 0, len(targetChannels))
		filteredSumWeight := 0
		for _, ch := range targetChannels {
			if !usedSet[ch.Id] {
				filtered = append(filtered, ch)
				filteredSumWeight += ch.GetWeight()
			}
		}
		// 只有当过滤后仍有可用渠道时才使用过滤结果，否则回退到全量（避免无渠道可选）
		if len(filtered) > 0 {
			targetChannels = filtered
			sumWeight = filteredSumWeight
		}
	}

	smoothingFactor := 1
	smoothingAdjustment := 0

	if sumWeight == 0 {
		sumWeight = len(targetChannels) * 100
		smoothingAdjustment = 100
	} else if sumWeight/len(targetChannels) < 10 {
		smoothingFactor = 100
	}

	totalWeight := sumWeight * smoothingFactor
	randomWeight := rand.Intn(totalWeight)

	for _, channel := range targetChannels {
		randomWeight -= channel.GetWeight()*smoothingFactor + smoothingAdjustment
		if randomWeight < 0 {
			return channel, nil
		}
	}
	return nil, nil
}

// GetRandomSatisfiedChannel 从内存缓存中按优先级+权重选择渠道。
//
// 参数:
//   - group: 用户分组
//   - model: 模型名称
//   - retry: 全局重试计数（内部按同优先级内重试预算映射为优先级层级）
//   - usedChannelIDs: 当前请求已使用的渠道ID列表，用于在同一优先级层级内排除
//
// 内存缓存关闭时回退到 DB 路径 GetChannel。
func GetRandomSatisfiedChannel(group string, model string, retry int, usedChannelIDs []int) (*Channel, error) {
	// if memory cache is disabled, get channel directly from database
	if !common.MemoryCacheEnabled {
		return GetChannel(group, model, retry, usedChannelIDs)
	}

	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	allChannelIDs := getChannelIDsForModel(group, model)
	if len(allChannelIDs) == 0 {
		logCacheMiss(group, model)
		return nil, nil
	}

	return selectChannelFromList(allChannelIDs, retry, usedChannelIDs)
}

// GetRandomSatisfiedChannelWithPreference 优先从 preferredTypes 中选择渠道。
// 如果优先类型全部不可用，回退到所有渠道。
// 支持 auto group 场景下的跨组回退。
//
// 参数:
//   - group: 用户分组
//   - model: 模型名称
//   - retry: 全局重试次数（内部按同优先级内重试预算映射为优先级层级）
//   - preferredTypes: 优先渠道类型列表，为空则等价于 GetRandomSatisfiedChannel
//   - usedChannelIDs: 当前请求已使用的渠道ID列表，用于在同一优先级层级内排除
//
// 返回:
//   - (*Channel, nil): 选中渠道
//   - (nil, nil): 无可用渠道或优先类型未命中且已回退
//   - (nil, error): 数据一致性问题
//   - fellBack: true 表示发生了类型回退（优先类型有渠道但不可用，回退到全量）
func GetRandomSatisfiedChannelWithPreference(
	group string, model string, retry int, preferredTypes []int, usedChannelIDs []int,
) (channel *Channel, err error, fellBack bool) {
	if len(preferredTypes) == 0 {
		channel, err = GetRandomSatisfiedChannel(group, model, retry, usedChannelIDs)
		return channel, err, false
	}

	if !common.MemoryCacheEnabled {
		// DB 回退路径：暂不支持类型优先，走原有逻辑。
		// 未来可扩展为带类型过滤的 SQL 查询：WHERE type IN (...)
		channel, err = GetChannel(group, model, retry, usedChannelIDs)
		return channel, err, false
	}

	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	// 获取模型对应的所有渠道ID
	allChannelIDs := getChannelIDsForModel(group, model)
	if len(allChannelIDs) == 0 {
		return nil, nil, false
	}

	// 第一步：按类型优先级过滤
	typeSet := make(map[int]bool, len(preferredTypes))
	for _, t := range preferredTypes {
		typeSet[t] = true
	}

	preferredChannelIDs := make([]int, 0, len(allChannelIDs))
	for _, channelId := range allChannelIDs {
		if ch, ok := channelsIDM[channelId]; ok && typeSet[ch.Type] {
			preferredChannelIDs = append(preferredChannelIDs, channelId)
		}
	}

	if len(preferredChannelIDs) > 0 {
		channel, err := selectChannelFromList(preferredChannelIDs, retry, usedChannelIDs)
		if err != nil {
			return nil, err, false
		}
		if channel != nil {
			// 类型优先命中
			logger.LogDebug(nil, fmt.Sprintf(
				"[channel_type_preference] hit: group=%s model=%s types=%v channel_id=%d type=%d",
				group, model, preferredTypes, channel.Id, channel.Type,
			))
			return channel, nil, false
		}
		// 优先类型有渠道但当前 retry 层级无匹配，回退到全量
		// 注：重试循环会递增 retry，后续可能匹配到更低优先级层级
	}

	// 第二步：回退到所有渠道
	fallbackChannel, fallbackErr := selectChannelFromList(allChannelIDs, retry, usedChannelIDs)
	return fallbackChannel, fallbackErr, true
}

func CacheGetChannel(id int) (*Channel, error) {
	if !common.MemoryCacheEnabled {
		return GetChannelById(id, true)
	}
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	c, ok := channelsIDM[id]
	if !ok {
		return nil, fmt.Errorf("渠道# %d，已不存在", id)
	}
	return c, nil
}

func CacheGetChannelInfo(id int) (*ChannelInfo, error) {
	if !common.MemoryCacheEnabled {
		channel, err := GetChannelById(id, true)
		if err != nil {
			return nil, err
		}
		return &channel.ChannelInfo, nil
	}
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	c, ok := channelsIDM[id]
	if !ok {
		return nil, fmt.Errorf("渠道# %d，已不存在", id)
	}
	return &c.ChannelInfo, nil
}

func CacheUpdateChannelStatus(id int, status int) {
	if !common.MemoryCacheEnabled {
		return
	}
	channelSyncLock.Lock()
	defer channelSyncLock.Unlock()
	if channel, ok := channelsIDM[id]; ok {
		channel.Status = status
	}
	if status != common.ChannelStatusEnabled {
		// delete the channel from group2model2channels
		for group, model2channels := range group2model2channels {
			for model, channels := range model2channels {
				for i, channelId := range channels {
					if channelId == id {
						// remove the channel from the slice
						group2model2channels[group][model] = append(channels[:i], channels[i+1:]...)
						break
					}
				}
			}
		}
	}
}

func CacheUpdateChannel(channel *Channel) {
	if !common.MemoryCacheEnabled {
		return
	}
	channelSyncLock.Lock()
	defer channelSyncLock.Unlock()
	if channel == nil {
		return
	}

	if channelsIDM == nil {
		channelsIDM = make(map[int]*Channel)
	}
	if oldChannel, ok := channelsIDM[channel.Id]; ok {
		logger.LogDebug(nil, "CacheUpdateChannel before: id=%d, name=%s, status=%d, polling_index=%d", channel.Id, channel.Name, channel.Status, oldChannel.ChannelInfo.MultiKeyPollingIndex)
	}
	channelsIDM[channel.Id] = channel
	logger.LogDebug(nil, "CacheUpdateChannel after: id=%d, name=%s, status=%d, polling_index=%d", channel.Id, channel.Name, channel.Status, channel.ChannelInfo.MultiKeyPollingIndex)
}
