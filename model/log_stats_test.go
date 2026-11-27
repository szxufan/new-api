package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insertChannelStatLog(t *testing.T, channelId int, createdAt int64, logType int, quota, promptTokens, completionTokens int) {
	t.Helper()
	require.NoError(t, LOG_DB.Create(&Log{
		ChannelId:        channelId,
		CreatedAt:        createdAt,
		Type:             logType,
		Quota:            quota,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		ModelName:        "gpt-test",
	}).Error)
}

func TestGetChannelQuotaStats(t *testing.T) {
	truncateTables(t)
	// 渠道名回填写入主库，与日志库查询分离；测试库中两者指向同一连接
	require.NoError(t, DB.Create(&Channel{Id: 1, Name: "channel-a", Key: "sk-a"}).Error)
	require.NoError(t, DB.Create(&Channel{Id: 2, Name: "channel-b", Key: "sk-b"}).Error)

	base := time.Date(2026, 11, 27, 10, 30, 0, 0, time.UTC).Unix()
	hour := base / channelStatBucketSize * channelStatBucketSize
	nextHour := hour + channelStatBucketSize

	// 渠道 1 同一小时两条消费日志，应聚合为一行
	insertChannelStatLog(t, 1, base, LogTypeConsume, 100, 10, 5)
	insertChannelStatLog(t, 1, base+60, LogTypeConsume, 200, 20, 10)
	// 渠道 1 下一小时一条
	insertChannelStatLog(t, 1, nextHour+120, LogTypeConsume, 50, 3, 2)
	// 渠道 2 一条
	insertChannelStatLog(t, 2, base, LogTypeConsume, 500, 100, 50)
	// 已删除渠道（无 channels 记录）保留 channel_id、名称为空
	insertChannelStatLog(t, 99, base, LogTypeConsume, 7, 1, 1)
	// 无渠道消费日志归入 channel_id=0
	insertChannelStatLog(t, 0, base, LogTypeConsume, 1, 0, 0)
	// 非消费与退款日志均不计入
	insertChannelStatLog(t, 1, base, LogTypeManage, 999, 0, 0)
	insertChannelStatLog(t, 1, base, LogTypeRefund, 999, 0, 0)

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
		ranged, err := GetChannelQuotaStats(nextHour, nextHour+channelStatBucketSize-1)
		require.NoError(t, err)
		require.Len(t, ranged, 1)
		assert.Equal(t, 1, ranged[0].ChannelId)
		assert.EqualValues(t, nextHour, ranged[0].CreatedAt)
		assert.EqualValues(t, 50, ranged[0].Quota)
	})

	t.Run("empty result", func(t *testing.T) {
		future := nextHour + 100*channelStatBucketSize
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

func TestChannelStatBucketExprDialect(t *testing.T) {
	origin := common.LogSqlType
	t.Cleanup(func() { common.LogSqlType = origin })

	common.LogSqlType = common.DatabaseTypeMySQL
	assert.Contains(t, channelStatBucketExpr(), "FLOOR")

	common.LogSqlType = common.DatabaseTypePostgreSQL
	assert.NotContains(t, channelStatBucketExpr(), "FLOOR")

	common.LogSqlType = common.DatabaseTypeSQLite
	assert.NotContains(t, channelStatBucketExpr(), "FLOOR")
}
