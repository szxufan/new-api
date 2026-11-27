package model

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
)

// ChannelQuotaStat 渠道维度的小时级消耗统计，供数据看板“渠道调用分析”使用。
// 数据来自 quota_data 预聚合表（created_at 已是小时桶起点），
// 与模型看板同源同保留期，不受消费日志清理影响。
type ChannelQuotaStat struct {
	ChannelId   int    `json:"channel_id"`
	ChannelName string `json:"channel_name" gorm:"-"`
	CreatedAt   int64  `json:"created_at"`
	Count       int64  `json:"count"`
	Quota       int64  `json:"quota"`
	TokenUsed   int64  `json:"token_used"`
}

// GetChannelQuotaStats 按渠道 + 小时桶聚合 quota_data。
// 口径与模型看板一致：只覆盖 RecordConsumeLog 产生的消费数据（不含任务退款冲减）。
// 历史存量行没有渠道维度（channel_id=0），由前端归入 Unknown；
// 已删除渠道保留 channel_id，channel_name 为空，由前端回退显示 #id。
func GetChannelQuotaStats(startTime int64, endTime int64) ([]ChannelQuotaStat, error) {
	var stats []ChannelQuotaStat
	query := DB.Table("quota_data").
		Select("channel_id, created_at, sum(count) as count, sum(quota) as quota, sum(token_used) as token_used").
		Group("channel_id, created_at").
		Order("created_at ASC")
	if startTime > 0 {
		query = query.Where("created_at >= ?", startTime)
	}
	if endTime > 0 {
		query = query.Where("created_at <= ?", endTime)
	}
	if err := query.Find(&stats).Error; err != nil {
		return nil, err
	}

	channelIds := types.NewSet[int]()
	for i := range stats {
		if stats[i].ChannelId != 0 {
			channelIds.Add(stats[i].ChannelId)
		}
	}
	channelMap, err := resolveChannelNames(channelIds.Items())
	if err != nil {
		return nil, err
	}
	for i := range stats {
		stats[i].ChannelName = channelMap[stats[i].ChannelId]
	}
	return stats, nil
}

// resolveChannelNames 批量解析渠道名称。
// 内存缓存开启时优先走缓存，否则批量查主库 channels 表。
// 返回 map 中不存在的 id 表示渠道已删除或不可用。
func resolveChannelNames(channelIds []int) (map[int]string, error) {
	channelMap := make(map[int]string, len(channelIds))
	if len(channelIds) == 0 {
		return channelMap, nil
	}
	if common.MemoryCacheEnabled {
		// Cache get channel
		for _, channelId := range channelIds {
			if cacheChannel, err := CacheGetChannel(channelId); err == nil {
				channelMap[channelId] = cacheChannel.Name
			}
		}
		return channelMap, nil
	}
	// Bulk query channels from DB
	var channels []struct {
		Id   int    `gorm:"column:id"`
		Name string `gorm:"column:name"`
	}
	if err := DB.Table("channels").Select("id, name").Where("id IN ?", channelIds).Find(&channels).Error; err != nil {
		return nil, err
	}
	for _, channel := range channels {
		channelMap[channel.Id] = channel.Name
	}
	return channelMap, nil
}
