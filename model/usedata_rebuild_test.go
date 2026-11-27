package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insertRebuildLog(t *testing.T, userId int, username, modelName string, channelId int, createdAt int64, logType, quota, promptTokens, completionTokens int) {
	t.Helper()
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:           userId,
		Username:         username,
		CreatedAt:        createdAt,
		Type:             logType,
		ModelName:        modelName,
		Quota:            quota,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		ChannelId:        channelId,
	}).Error)
}

func TestRebuildQuotaDataFromLogs(t *testing.T) {
	truncateTables(t)
	resetQuotaDataCache(t)
	require.NoError(t, DB.AutoMigrate(&QuotaData{}))

	base := time.Date(2026, 11, 27, 10, 30, 0, 0, time.UTC).Unix()
	hour := base / 3600 * 3600
	nextHour := hour + 3600

	// 同用户+模型+渠道+小时的两条消费日志：应聚合成一行
	insertRebuildLog(t, 1, "u1", "gpt-test", 1, base, LogTypeConsume, 100, 30, 10)
	insertRebuildLog(t, 1, "u1", "gpt-test", 1, base+60, LogTypeConsume, 200, 20, 5)
	// 另一渠道同一小时
	insertRebuildLog(t, 1, "u1", "gpt-test", 2, base, LogTypeConsume, 500, 100, 50)
	// 下一小时
	insertRebuildLog(t, 1, "u1", "gpt-test", 1, nextHour, LogTypeConsume, 50, 3, 2)
	// 非消费日志必须排除
	insertRebuildLog(t, 1, "u1", "gpt-test", 1, base, LogTypeManage, 9999, 0, 0)

	// 重建前塞一行旧数据，应被清空
	require.NoError(t, DB.Create(&QuotaData{
		UserID: 99, Username: "old", ModelName: "old-model", ChannelId: 0,
		CreatedAt: hour, Count: 1, Quota: 99999, TokenUsed: 1,
	}).Error)

	rows, err := RebuildQuotaDataFromLogs()
	require.NoError(t, err)
	assert.EqualValues(t, 3, rows)

	var quotaRows []QuotaData
	require.NoError(t, DB.Table("quota_data").Find(&quotaRows).Error)
	require.Len(t, quotaRows, 3)

	byKey := make(map[string]QuotaData, len(quotaRows))
	for _, row := range quotaRows {
		byKey[fmt.Sprintf("%d-%d", row.ChannelId, row.CreatedAt)] = row
	}

	first := byKey[fmt.Sprintf("1-%d", hour)]
	assert.Equal(t, 2, first.Count)
	assert.Equal(t, 300, first.Quota)
	assert.Equal(t, 65, first.TokenUsed) // (30+10)+(20+5)

	other := byKey[fmt.Sprintf("2-%d", hour)]
	assert.Equal(t, 500, other.Quota)
	assert.Equal(t, 150, other.TokenUsed)

	next := byKey[fmt.Sprintf("1-%d", nextHour)]
	assert.Equal(t, 50, next.Quota)

	// 内存缓存已被清空，避免下个周期重复累加
	CacheQuotaDataLock.Lock()
	assert.Len(t, CacheQuotaData, 0)
	CacheQuotaDataLock.Unlock()
}

func TestGetQuotaDataRebuildPreview(t *testing.T) {
	truncateTables(t)
	resetQuotaDataCache(t)
	require.NoError(t, DB.AutoMigrate(&QuotaData{}))

	base := time.Date(2026, 11, 27, 10, 30, 0, 0, time.UTC).Unix()
	insertRebuildLog(t, 1, "u1", "gpt-test", 1, base, LogTypeConsume, 100, 10, 5)
	insertRebuildLog(t, 1, "u1", "gpt-test", 1, base+3600, LogTypeManage, 999, 0, 0)
	require.NoError(t, DB.Create(&QuotaData{
		UserID: 1, Username: "u1", ModelName: "gpt-test", ChannelId: 0,
		CreatedAt: base - 7200, Count: 1, Quota: 50, TokenUsed: 5,
	}).Error)

	preview, err := GetQuotaDataRebuildPreview()
	require.NoError(t, err)
	assert.EqualValues(t, 1, preview.LogConsumeRows)
	assert.EqualValues(t, base, preview.LogFirstCreatedAt)
	assert.EqualValues(t, base, preview.LogLastCreatedAt)
	assert.EqualValues(t, 1, preview.QuotaDataRows)
	assert.EqualValues(t, base-7200, preview.QuotaFirstCreatedAt)
}
