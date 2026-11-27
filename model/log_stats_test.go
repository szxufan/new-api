package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insertChannelStatRow(t *testing.T, channelId int, hourBucket int64, count, quota, tokenUsed int) {
	t.Helper()
	require.NoError(t, DB.Create(&QuotaData{
		UserID:    1,
		Username:  "channel-stat-user",
		ModelName: "gpt-test",
		ChannelId: channelId,
		CreatedAt: hourBucket,
		Count:     count,
		Quota:     quota,
		TokenUsed: tokenUsed,
	}).Error)
}

func TestGetChannelQuotaStats(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&QuotaData{}))
	// 渠道名回填写入主库 channels 表
	require.NoError(t, DB.Create(&Channel{Id: 1, Name: "channel-a", Key: "sk-a"}).Error)
	require.NoError(t, DB.Create(&Channel{Id: 2, Name: "channel-b", Key: "sk-b"}).Error)

	base := time.Date(2026, 11, 27, 10, 30, 0, 0, time.UTC).Unix()
	hour := base / 3600 * 3600
	nextHour := hour + 3600

	// 渠道 1 同一小时两条预聚合行（可能来自不同用户/模型），应聚合为一行
	insertChannelStatRow(t, 1, hour, 1, 100, 30)
	insertChannelStatRow(t, 1, hour, 1, 200, 15)
	// 渠道 1 下一小时一条
	insertChannelStatRow(t, 1, nextHour, 1, 50, 5)
	// 渠道 2 一条
	insertChannelStatRow(t, 2, hour, 1, 500, 150)
	// 已删除渠道（无 channels 记录）保留 channel_id、名称为空
	insertChannelStatRow(t, 99, hour, 1, 7, 2)
	// 历史存量行没有渠道维度，归入 channel_id=0
	insertChannelStatRow(t, 0, hour, 1, 1, 0)

	stats, err := GetChannelQuotaStats(0, 0)
	require.NoError(t, err)
	require.Len(t, stats, 5)

	rows := make(map[string]ChannelQuotaStat, len(stats))
	for _, stat := range stats {
		rows[fmt.Sprintf("%d-%d", stat.ChannelId, stat.CreatedAt)] = stat
	}

	first := rows[fmt.Sprintf("1-%d", hour)]
	assert.Equal(t, "channel-a", first.ChannelName)
	assert.EqualValues(t, 2, first.Count)
	assert.EqualValues(t, 300, first.Quota)
	assert.EqualValues(t, 45, first.TokenUsed)
	assert.EqualValues(t, hour, first.CreatedAt)

	second := rows[fmt.Sprintf("1-%d", nextHour)]
	assert.EqualValues(t, 1, second.Count)
	assert.EqualValues(t, 50, second.Quota)
	assert.EqualValues(t, 5, second.TokenUsed)

	other := rows[fmt.Sprintf("2-%d", hour)]
	assert.Equal(t, "channel-b", other.ChannelName)
	assert.EqualValues(t, 1, other.Count)
	assert.EqualValues(t, 500, other.Quota)
	assert.EqualValues(t, 150, other.TokenUsed)

	deleted := rows[fmt.Sprintf("99-%d", hour)]
	assert.Empty(t, deleted.ChannelName)
	assert.EqualValues(t, 7, deleted.Quota)

	noChannel := rows[fmt.Sprintf("0-%d", hour)]
	assert.Empty(t, noChannel.ChannelName)
	assert.EqualValues(t, 1, noChannel.Quota)

	t.Run("time range filter", func(t *testing.T) {
		ranged, err := GetChannelQuotaStats(nextHour, nextHour)
		require.NoError(t, err)
		require.Len(t, ranged, 1)
		assert.Equal(t, 1, ranged[0].ChannelId)
		assert.EqualValues(t, nextHour, ranged[0].CreatedAt)
		assert.EqualValues(t, 50, ranged[0].Quota)
	})

	t.Run("empty result", func(t *testing.T) {
		future := nextHour + 100*3600
		empty, err := GetChannelQuotaStats(future, future)
		require.NoError(t, err)
		assert.Len(t, empty, 0)
	})
}

func TestResolveChannelNamesEmpty(t *testing.T) {
	names, err := resolveChannelNames(nil)
	require.NoError(t, err)
	assert.NotNil(t, names)
	assert.Len(t, names, 0)
}

func resetQuotaDataCache(t *testing.T) {
	t.Helper()
	CacheQuotaDataLock.Lock()
	CacheQuotaData = make(map[string]*QuotaData)
	CacheQuotaDataLock.Unlock()
	t.Cleanup(func() {
		CacheQuotaDataLock.Lock()
		CacheQuotaData = make(map[string]*QuotaData)
		CacheQuotaDataLock.Unlock()
	})
}

// 写入链路：缓存 key 与落库合并都必须带渠道维度，
// 同渠道同时段合并、不同渠道独立成行；模型看板的 SUM 聚合不受影响。
func TestQuotaDataChannelAggregationOnSave(t *testing.T) {
	truncateTables(t)
	resetQuotaDataCache(t)
	require.NoError(t, DB.AutoMigrate(&QuotaData{}))

	base := time.Date(2026, 11, 27, 10, 30, 0, 0, time.UTC).Unix()
	hour := base / 3600 * 3600

	LogQuotaData(1, "u1", "gpt-test", 1, 100, base, 30)
	LogQuotaData(1, "u1", "gpt-test", 1, 200, base+60, 15) // 同渠道同时段 → 合并
	LogQuotaData(1, "u1", "gpt-test", 2, 500, base, 150)   // 不同渠道 → 独立行
	SaveQuotaDataCache()

	var rows []QuotaData
	require.NoError(t, DB.Table("quota_data").Order("channel_id").Find(&rows).Error)
	require.Len(t, rows, 2)
	assert.Equal(t, 1, rows[0].ChannelId)
	assert.Equal(t, 2, rows[0].Count)
	assert.Equal(t, 300, rows[0].Quota)
	assert.Equal(t, 45, rows[0].TokenUsed)
	assert.EqualValues(t, hour, rows[0].CreatedAt)
	assert.Equal(t, 2, rows[1].ChannelId)
	assert.Equal(t, 1, rows[1].Count)
	assert.Equal(t, 500, rows[1].Quota)

	// 再次写入同渠道同时段，走 increaseQuotaData 累加而非新增行
	LogQuotaData(1, "u1", "gpt-test", 1, 50, base, 5)
	SaveQuotaDataCache()

	require.NoError(t, DB.Table("quota_data").Order("channel_id").Find(&rows).Error)
	require.Len(t, rows, 2)
	assert.Equal(t, 3, rows[0].Count)
	assert.Equal(t, 350, rows[0].Quota)
	assert.Equal(t, 50, rows[0].TokenUsed)

	// 模型看板回归：同模型同小时的多渠道行被 SUM 合并为一行
	modelRows, err := GetAllQuotaDates(hour, hour, "")
	require.NoError(t, err)
	require.Len(t, modelRows, 1)
	assert.Equal(t, "gpt-test", modelRows[0].ModelName)
	assert.Equal(t, 4, modelRows[0].Count)
	assert.Equal(t, 850, modelRows[0].Quota)
	assert.Equal(t, 200, modelRows[0].TokenUsed)
}
