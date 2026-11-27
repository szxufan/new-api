package model

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
)

// ChannelQuotaStat 渠道维度的小时级消耗统计，供数据看板“渠道调用分析”使用。
// CreatedAt 为小时桶起点（Unix 秒），与 quota_data 的按小时预聚合粒度对齐，
// 前端可复用模型看板的时间粒度聚合逻辑。
type ChannelQuotaStat struct {
	ChannelId   int    `json:"channel_id"`
	ChannelName string `json:"channel_name" gorm:"-"`
	CreatedAt   int64  `json:"created_at"`
	Count       int64  `json:"count"`
	Quota       int64  `json:"quota"`
	TokenUsed   int64  `json:"token_used"`
}

const channelStatBucketSize = 3600

// channelStatBucketExpr 将 created_at 截断到小时桶。
// 查询运行在 LOG_DB 上，方言必须按 common.LogSqlType 选择（日志库可能与主库不同引擎）：
// MySQL 的 / 返回小数需要 FLOOR，SQLite/PostgreSQL 整数除法天然截断。
func channelStatBucketExpr() string {
	if common.LogSqlType == common.DatabaseTypeMySQL {
		return fmt.Sprintf("FLOOR(created_at / %d) * %d", channelStatBucketSize, channelStatBucketSize)
	}
	return fmt.Sprintf("(created_at / %d) * %d", channelStatBucketSize, channelStatBucketSize)
}

// GetChannelQuotaStats 按渠道 + 小时桶实时聚合消费日志。
// 口径与模型看板（quota_data）保持一致：只统计 LogTypeConsume，不冲减退款日志，
// 渠道测试产生的消费日志同样计入。已删除渠道保留 channel_id，channel_name 为空，
// 由前端回退显示 #id。
func GetChannelQuotaStats(startTime int64, endTime int64) ([]ChannelQuotaStat, error) {
	bucketExpr := channelStatBucketExpr()
	var stats []ChannelQuotaStat
	query := LOG_DB.Table("logs").
		Select(fmt.Sprintf("channel_id, %s as created_at, count(*) as count, "+
			"COALESCE(sum(quota), 0) as quota, "+
			"COALESCE(sum(prompt_tokens), 0) + COALESCE(sum(completion_tokens), 0) as token_used", bucketExpr)).
		Where("type = ?", LogTypeConsume).
		Group(fmt.Sprintf("channel_id, %s", bucketExpr)).
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

// resolveChannelNames 批量解析渠道名称。渠道在主库 DB，与日志库 LOG_DB 可能分离，
// 因此不能跨库 JOIN；内存缓存开启时优先走缓存。
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
