package model

import (
	"fmt"
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
		// 只跳过手动禁用的渠道，429限流和自动禁用的渠道保留（重试时需要走 fallback）
		if channel.Status == common.ChannelStatusManuallyDisabled {
			continue
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

// GetChannelIDsForModel 返回指定分组和模型对应的所有渠道ID列表（公开接口）。
func GetChannelIDsForModel(group, model string) []int {
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()
	return getChannelIDsForModel(group, model)
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

// effectiveAffinityWeight 计算渠道的有效权重（weight==0 时视为 100）。
func effectiveAffinityWeight(ch *Channel) int {
	w := ch.GetWeight()
	if w == 0 {
		return 100
	}
	return w
}

// affinityScore 计算渠道的亲和性得分 = affinityCount / effectiveWeight。
// 当 affinityCounts 为 nil 或渠道不在 map 中时，count 视为 0。
func affinityScore(ch *Channel, affinityCounts map[int]int64) float64 {
	var count int64
	if affinityCounts != nil {
		count = affinityCounts[ch.Id]
	}
	return float64(count) / float64(effectiveAffinityWeight(ch))
}

// sortChannelsByPriorityAndAffinity 对渠道切片排序：
//   - 先按优先级降序
//   - 同优先级内：若 affinityCounts != nil，按 affinityCount/effectiveWeight 升序；
//     否则保持原始顺序（调用方应已按优先级降序排序）
//
// 排序为稳定排序（同优先级且同 score 时保持原顺序）。
func sortChannelsByPriorityAndAffinity(channels []*Channel, affinityCounts map[int]int64) {
	if len(channels) == 0 {
		return
	}
	// 使用稳定排序，避免同 score 时顺序被打乱
	sort.SliceStable(channels, func(i, j int) bool {
		pi := channels[i].GetPriority()
		pj := channels[j].GetPriority()
		if pi != pj {
			return pi > pj // 优先级降序
		}
		if affinityCounts != nil {
			si := affinityScore(channels[i], affinityCounts)
			sj := affinityScore(channels[j], affinityCounts)
			if si != sj {
				return si < sj // 亲和性得分升序
			}
		}
		return false // 保持原顺序
	})
}

// GetSortedChannelsByPriorityAndAffinity 返回按优先级降序+亲和性升序排序的渠道列表。
//
// 设计目标：一次性获取所有渠道并排序返回，由调用方按顺序逐个尝试，
// 取代旧的"每次重试时随机选择单个渠道"模式。
//
// 参数:
//   - group: 用户分组
//   - model: 模型名称
//   - preferredTypes: 优先渠道类型列表，非空时先按类型过滤（仅内存缓存路径支持）
//   - affinityCounts: 渠道亲和性计数，非空时同优先级内按 affinityCount/effectiveWeight 升序排列
//
// 实现逻辑:
//  1. MemoryCacheEnabled 关闭时回退到 GetSortedChannelsDB（DB 路径不支持 preferredTypes）
//  2. 获取 allChannelIDs := getChannelIDsForModel(group, model)，为空返回 nil
//  3. preferredTypes 非空时先过滤出类型匹配的渠道
//  4. 收集所有渠道到 []*Channel 切片
//  5. 排序：先按优先级降序，同优先级内按亲和性得分升序（affinityCounts 为 nil 时保持原顺序）
//  6. 返回排序后的渠道列表
func GetSortedChannelsByPriorityAndAffinity(group, model string, preferredTypes []int, affinityCounts map[int]int64) []*Channel {
	// 内存缓存关闭时回退到 DB 路径
	if !common.MemoryCacheEnabled {
		return GetSortedChannelsDB(group, model, affinityCounts)
	}

	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	allChannelIDs := getChannelIDsForModel(group, model)
	if len(allChannelIDs) == 0 {
		logCacheMiss(group, model)
		return nil
	}

	// preferredTypes 非空时先按类型过滤
	if len(preferredTypes) > 0 {
		typeSet := make(map[int]bool, len(preferredTypes))
		for _, t := range preferredTypes {
			typeSet[t] = true
		}
		filteredIDs := make([]int, 0, len(allChannelIDs))
		for _, id := range allChannelIDs {
			if ch, ok := channelsIDM[id]; ok && typeSet[ch.Type] {
				filteredIDs = append(filteredIDs, id)
			}
		}
		// 类型过滤后无渠道，回退到全量渠道
		if len(filteredIDs) > 0 {
			allChannelIDs = filteredIDs
		} else {
			logger.LogDebug(nil, fmt.Sprintf(
				"[channel_type_preference] no channel matches types=%v, fallback to all: group=%s model=%s",
				preferredTypes, group, model,
			))
		}
	}

	// 收集所有渠道到切片
	channels := make([]*Channel, 0, len(allChannelIDs))
	for _, id := range allChannelIDs {
		if ch, ok := channelsIDM[id]; ok {
			channels = append(channels, ch)
		}
	}
	if len(channels) == 0 {
		return nil
	}

	// 排序（group2model2channels 中已按优先级降序排序，这里补充同优先级内的亲和性排序）
	sortChannelsByPriorityAndAffinity(channels, affinityCounts)
	return channels
}

// pickFirstChannel 从已排序渠道列表中取出第一个渠道。
// 供简化后的 GetRandomSatisfiedChannel / GetRandomSatisfiedChannelWithPreference 使用。
func pickFirstChannel(channels []*Channel) (*Channel, error) {
	if len(channels) == 0 {
		return nil, nil
	}
	return channels[0], nil
}

// GetRandomSatisfiedChannel 从内存缓存中按优先级+亲和性选择渠道。
//
// 已废弃：保留签名以兼容 service 层调用，内部简化为调用
// GetSortedChannelsByPriorityAndAffinity 并返回排序后的第一个渠道。
//
// 参数:
//   - group: 用户分组
//   - model: 模型名称
//   - retry: 全局重试计数（已废弃，新实现一次性返回排序列表，调用方按顺序重试）
//   - usedChannelIDs: 已废弃，保留签名兼容
//   - affinityCounts: 渠道亲和性计数（为 nil 时不启用亲和性感知选择）
func GetRandomSatisfiedChannel(group string, model string, retry int, usedChannelIDs []int, affinityCounts map[int]int64) (*Channel, error) {
	channels := GetSortedChannelsByPriorityAndAffinity(group, model, nil, affinityCounts)
	return pickFirstChannel(channels)
}

// GetRandomSatisfiedChannelWithPreference 优先从 preferredTypes 中选择渠道。
//
// 已废弃：保留签名以兼容 service 层调用，内部简化为调用
// GetSortedChannelsByPriorityAndAffinity（传入 preferredTypes）并返回排序后的第一个渠道。
//
// 参数:
//   - group: 用户分组
//   - model: 模型名称
//   - retry: 全局重试次数（已废弃）
//   - preferredTypes: 优先渠道类型列表，为空则等价于 GetRandomSatisfiedChannel
//   - usedChannelIDs: 已废弃，保留签名兼容
//   - affinityCounts: 渠道亲和性计数（为 nil 时不启用亲和性感知选择）
//
// 返回:
//   - (*Channel, nil): 选中渠道
//   - (nil, nil): 无可用渠道
//   - (nil, error): 数据一致性问题
//   - fellBack: true 表示发生了类型回退（优先类型无渠道，回退到全量）
func GetRandomSatisfiedChannelWithPreference(
	group string, model string, retry int, preferredTypes []int, usedChannelIDs []int, affinityCounts map[int]int64,
) (channel *Channel, err error, fellBack bool) {
	if len(preferredTypes) == 0 {
		channel, err = GetRandomSatisfiedChannel(group, model, retry, usedChannelIDs, affinityCounts)
		return channel, err, false
	}

	// 先尝试仅按类型过滤的排序列表
	preferred := GetSortedChannelsByPriorityAndAffinity(group, model, preferredTypes, affinityCounts)
	if len(preferred) > 0 {
		// 判断是否发生类型回退：检查返回的第一个渠道类型是否在 preferredTypes 中
		if typeInList(preferred[0].Type, preferredTypes) {
			return preferred[0], nil, false
		}
		// 否则为回退结果
		return preferred[0], nil, true
	}

	// 兜底：无任何渠道
	return nil, nil, false
}

// typeInList 判断 t 是否在 list 中。
func typeInList(t int, list []int) bool {
	for _, v := range list {
		if v == t {
			return true
		}
	}
	return false
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

// CacheGetFallbackChannel 从后备渠道列表中按顺序查找第一个支持指定模型且可用的渠道。
// 后备渠道无视分组限制，只要该渠道支持该模型（不论在哪个分组）即可作为后备。
// 被禁用的后备渠道（手动禁用/自动禁用）不生效，会被跳过。
func CacheGetFallbackChannel(fallbackChannelIDs []int, model string) (*Channel, error) {
	if len(fallbackChannelIDs) == 0 {
		return nil, nil
	}
	for _, id := range fallbackChannelIDs {
		channel, err := CacheGetChannel(id)
		if err != nil || channel == nil {
			continue
		}
		// 检查渠道是否启用（被禁用的后备渠道不生效）
		if channel.Status != common.ChannelStatusEnabled {
			continue
		}
		// 检查渠道是否支持该模型（无视分组限制）
		if IsChannelEnabledForAnyGroupModel(channel.GetGroups(), model, id) {
			return channel, nil
		}
	}
	return nil, nil
}

// isChannelInGroupModel 检查渠道是否属于指定分组且支持指定模型（通过 channel.Group/Models 字段判断）。
// 与 IsChannelEnabledForGroupModel 不同，此函数不依赖 group2model2channels 缓存，
// 因此即使渠道被禁用后从缓存中移除，仍能正确判断分组归属和模型支持。
func isChannelInGroupModel(channel *Channel, group string, modelName string) bool {
	groupMatch := false
	for _, g := range channel.GetGroups() {
		if g == group {
			groupMatch = true
			break
		}
	}
	if !groupMatch {
		return false
	}
	for _, m := range channel.GetModels() {
		if m == modelName {
			return true
		}
	}
	return false
}

// CacheGetDisabledChannelsWithFallback 查找所有配置了后备渠道的渠道，
// 并尝试为指定模型找到可用的后备渠道。
// 当常规渠道选择找不到可用渠道时调用，用于在所有渠道中寻找后备路径。
// 不限制渠道状态，因为：
// 1. 渠道被禁用后已从 group2model2channels 移除，不能用 IsChannelEnabledForGroupModel 过滤；
// 2. 正常启用的渠道也可能需要后备（常规选择未选到它但后备渠道可用）。
// 仍需检查渠道是否属于当前分组（通过 channel.Group 字段判断），避免跨分组匹配无关渠道。
// 后备渠道的可用性由 CacheGetFallbackChannel 内部保证（状态必须为启用且支持该模型）。
func CacheGetDisabledChannelsWithFallback(group string, model string) (*Channel, int, error) {
	if !common.MemoryCacheEnabled {
		return getDisabledChannelsWithFallbackDB(group, model)
	}

	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	for _, channel := range channelsIDM {
		// 检查渠道是否属于当前分组
		if !isChannelInGroupModel(channel, group, model) {
			continue
		}
		// 检查该渠道是否配置了后备渠道
		fallbackIds := channel.GetFallbackChannelIDs()
		if len(fallbackIds) == 0 {
			continue
		}
		// 尝试查找可用的后备渠道（内部会检查后备渠道状态和模型支持）
		fallbackChannel, fallbackErr := CacheGetFallbackChannel(fallbackIds, model)
		if fallbackErr == nil && fallbackChannel != nil {
			return fallbackChannel, channel.Id, nil
		}
	}
	return nil, 0, nil
}

// getDisabledChannelsWithFallbackDB 数据库模式下的后备渠道查找
// 逻辑与缓存模式一致：遍历所有渠道，查找配置了后备渠道且后备渠道可用的。
func getDisabledChannelsWithFallbackDB(group string, model string) (*Channel, int, error) {
	var channels []Channel
	err := DB.Find(&channels).Error
	if err != nil {
		return nil, 0, err
	}

	for _, channel := range channels {
		// 检查渠道是否属于当前分组
		if !isChannelInGroupModel(&channel, group, model) {
			continue
		}
		fallbackIds := channel.GetFallbackChannelIDs()
		if len(fallbackIds) == 0 {
			continue
		}
		fallbackChannel, fallbackErr := CacheGetFallbackChannel(fallbackIds, model)
		if fallbackErr == nil && fallbackChannel != nil {
			return fallbackChannel, channel.Id, nil
		}
	}
	return nil, 0, nil
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
