package model

import (
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// QuotaData 柱状图数据
// ChannelId 为后加的渠道维度：历史存量行 channel_id=0（无渠道），
// 渠道调用分析看板按 channel_id + created_at 聚合；其余看板的 GROUP BY 不变，
// 多渠道行在模型/用户维度被 SUM 合并，统计口径不受影响。
type QuotaData struct {
	Id        int    `json:"id"`
	UserID    int    `json:"user_id" gorm:"index"`
	Username  string `json:"username" gorm:"index:idx_qdt_model_user_name,priority:2;size:64;default:''"`
	ModelName string `json:"model_name" gorm:"index:idx_qdt_model_user_name,priority:1;size:64;default:''"`
	ChannelId int    `json:"channel_id" gorm:"default:0;index"`
	CreatedAt int64  `json:"created_at" gorm:"bigint;index:idx_qdt_created_at,priority:2"`
	TokenUsed int    `json:"token_used" gorm:"default:0"`
	Count     int    `json:"count" gorm:"default:0"`
	Quota     int    `json:"quota" gorm:"default:0"`
}

func UpdateQuotaData() {
	for {
		if common.DataExportEnabled {
			common.SysLog("正在更新数据看板数据...")
			SaveQuotaDataCache()
		}
		time.Sleep(time.Duration(common.DataExportInterval) * time.Minute)
	}
}

var CacheQuotaData = make(map[string]*QuotaData)
var CacheQuotaDataLock = sync.Mutex{}

// quotaDataMaintenanceLock 串行化缓存批量落库（SaveQuotaDataCache）与全量重建，
// 避免重建执行期间后台导出把旧缓存写入 quota_data 造成重复计数。
var quotaDataMaintenanceLock = sync.Mutex{}

func logQuotaDataCache(userId int, username string, modelName string, channelId int, quota int, createdAt int64, tokenUsed int) {
	key := fmt.Sprintf("%d-%s-%s-%d-%d", userId, username, modelName, channelId, createdAt)
	quotaData, ok := CacheQuotaData[key]
	if ok {
		quotaData.Count += 1
		quotaData.Quota += quota
		quotaData.TokenUsed += tokenUsed
	} else {
		quotaData = &QuotaData{
			UserID:    userId,
			Username:  username,
			ModelName: modelName,
			ChannelId: channelId,
			CreatedAt: createdAt,
			Count:     1,
			Quota:     quota,
			TokenUsed: tokenUsed,
		}
	}
	CacheQuotaData[key] = quotaData
}

func LogQuotaData(userId int, username string, modelName string, channelId int, quota int, createdAt int64, tokenUsed int) {
	// 只精确到小时
	createdAt = createdAt - (createdAt % 3600)

	CacheQuotaDataLock.Lock()
	defer CacheQuotaDataLock.Unlock()
	logQuotaDataCache(userId, username, modelName, channelId, quota, createdAt, tokenUsed)
}

func SaveQuotaDataCache() {
	quotaDataMaintenanceLock.Lock()
	defer quotaDataMaintenanceLock.Unlock()
	CacheQuotaDataLock.Lock()
	defer CacheQuotaDataLock.Unlock()
	size := len(CacheQuotaData)
	// 如果缓存中有数据，就保存到数据库中
	// 1. 先查询数据库中是否有数据
	// 2. 如果有数据，就更新数据
	// 3. 如果没有数据，就插入数据
	for _, quotaData := range CacheQuotaData {
		quotaDataDB := &QuotaData{}
		DB.Table("quota_data").Where("user_id = ? and username = ? and model_name = ? and channel_id = ? and created_at = ?",
			quotaData.UserID, quotaData.Username, quotaData.ModelName, quotaData.ChannelId, quotaData.CreatedAt).First(quotaDataDB)
		if quotaDataDB.Id > 0 {
			//quotaDataDB.Count += quotaData.Count
			//quotaDataDB.Quota += quotaData.Quota
			//DB.Table("quota_data").Save(quotaDataDB)
			increaseQuotaData(quotaData.UserID, quotaData.Username, quotaData.ModelName, quotaData.ChannelId, quotaData.Count, quotaData.Quota, quotaData.CreatedAt, quotaData.TokenUsed)
		} else {
			DB.Table("quota_data").Create(quotaData)
		}
	}
	CacheQuotaData = make(map[string]*QuotaData)
	common.SysLog(fmt.Sprintf("保存数据看板数据成功，共保存%d条数据", size))
}

func increaseQuotaData(userId int, username string, modelName string, channelId int, count int, quota int, createdAt int64, tokenUsed int) {
	err := DB.Table("quota_data").Where("user_id = ? and username = ? and model_name = ? and channel_id = ? and created_at = ?",
		userId, username, modelName, channelId, createdAt).Updates(map[string]interface{}{
		"count":      gorm.Expr("count + ?", count),
		"quota":      gorm.Expr("quota + ?", quota),
		"token_used": gorm.Expr("token_used + ?", tokenUsed),
	}).Error
	if err != nil {
		common.SysLog(fmt.Sprintf("increaseQuotaData error: %s", err))
	}
}

func GetQuotaDataByUsername(username string, startTime int64, endTime int64) (quotaData []*QuotaData, err error) {
	var quotaDatas []*QuotaData
	// 从quota_data表中查询数据
	err = DB.Table("quota_data").Where("LOWER(username) = LOWER(?) and created_at >= ? and created_at <= ?", username, startTime, endTime).Find(&quotaDatas).Error
	return quotaDatas, err
}

func GetQuotaDataByUserId(userId int, startTime int64, endTime int64) (quotaData []*QuotaData, err error) {
	var quotaDatas []*QuotaData
	// 从quota_data表中查询数据
	err = DB.Table("quota_data").Where("user_id = ? and created_at >= ? and created_at <= ?", userId, startTime, endTime).Find(&quotaDatas).Error
	return quotaDatas, err
}

func GetQuotaDataGroupByUser(startTime int64, endTime int64) (quotaData []*QuotaData, err error) {
	var quotaDatas []*QuotaData
	err = DB.Table("quota_data").
		Select("username, created_at, sum(count) as count, sum(quota) as quota, sum(token_used) as token_used").
		Where("created_at >= ? and created_at <= ?", startTime, endTime).
		Group("username, created_at").
		Find(&quotaDatas).Error
	return quotaDatas, err
}

func GetQuotaDataGroupByUserAndModel(startTime int64, endTime int64) (quotaData []*QuotaData, err error) {
	var quotaDatas []*QuotaData
	err = DB.Table("quota_data").
		Select("username, model_name, sum(count) as count, sum(quota) as quota, sum(token_used) as token_used").
		Where("created_at >= ? and created_at <= ?", startTime, endTime).
		Group("username, model_name").
		Find(&quotaDatas).Error
	return quotaDatas, err
}

// ============================================================================
// 从消费日志全量重建 quota_data（管理员手动触发的一次性运维操作）
// ============================================================================

// quotaDataHourBucketExpr 将日志的 created_at 截断到小时桶。
// 聚合在 LOG_DB 上执行，方言按 common.LogSqlType 选择（日志库可能与主库不同引擎）。
func quotaDataHourBucketExpr() string {
	if common.LogSqlType == common.DatabaseTypeMySQL {
		return "FLOOR(created_at / 3600) * 3600"
	}
	// SQLite/PostgreSQL 的整型除法天然截断
	return "(created_at / 3600) * 3600"
}

// QuotaDataRebuildPreview 重建前的对比信息，避免误删比日志更完整的历史数据。
type QuotaDataRebuildPreview struct {
	LogConsumeRows      int64 `json:"log_consume_rows"`
	LogFirstCreatedAt   int64 `json:"log_first_created_at"`
	LogLastCreatedAt    int64 `json:"log_last_created_at"`
	QuotaDataRows       int64 `json:"quota_data_rows"`
	QuotaFirstCreatedAt int64 `json:"quota_data_first_created_at"`
	QuotaLastCreatedAt  int64 `json:"quota_data_last_created_at"`
}

// GetQuotaDataRebuildPreview 对比 logs(type=消费) 与 quota_data 的行数和时间覆盖。
// 如果日志的起始时间明显晚于 quota_data，说明日志被清理过，重建会丢失历史。
func GetQuotaDataRebuildPreview() (QuotaDataRebuildPreview, error) {
	var preview QuotaDataRebuildPreview
	if err := LOG_DB.Table("logs").
		Select("count(*) as log_consume_rows, coalesce(min(created_at), 0) as log_first_created_at, coalesce(max(created_at), 0) as log_last_created_at").
		Where("type = ?", LogTypeConsume).
		Scan(&preview).Error; err != nil {
		return preview, err
	}
	if err := DB.Table("quota_data").
		Select("count(*) as quota_data_rows, coalesce(min(created_at), 0) as quota_first_created_at, coalesce(max(created_at), 0) as quota_last_created_at").
		Scan(&preview).Error; err != nil {
		return preview, err
	}
	return preview, nil
}

// RebuildQuotaDataFromLogs 从 logs 的 type=消费 记录全量重建 quota_data。
// 口径说明：日志中任务计费（RecordTaskBillingLog）也是 type=消费，会被算进来，
// 因此重建后的模型看板总额可能与重建前不同（这是全量重建方案的已知差异）。
// 聚合按 用户+用户名+模型+渠道+小时桶 分组，生成的数据自带 channel_id。
func RebuildQuotaDataFromLogs() (int64, error) {
	quotaDataMaintenanceLock.Lock()
	defer quotaDataMaintenanceLock.Unlock()

	bucketExpr := quotaDataHourBucketExpr()
	var rows []QuotaData
	// 先读出全部聚合结果再开启事务写入，避免在读游标未关闭时占用数据库连接（SQLite 单连接会死锁）
	err := LOG_DB.Table("logs").
		Select(fmt.Sprintf("user_id, username, model_name, channel_id, %s as created_at, "+
			"count(*) as count, COALESCE(sum(quota), 0) as quota, "+
			"COALESCE(sum(prompt_tokens) + sum(completion_tokens), 0) as token_used", bucketExpr)).
		Where("type = ?", LogTypeConsume).
		Group(fmt.Sprintf("user_id, username, model_name, channel_id, %s", bucketExpr)).
		Find(&rows).Error
	if err != nil {
		return 0, err
	}

	// 聚合结果已覆盖缓存中条目的来源日志；写入前清空缓存，
	// 避免重建后旧缓存再次落库被重复计数。
	CacheQuotaDataLock.Lock()
	CacheQuotaData = make(map[string]*QuotaData)
	CacheQuotaDataLock.Unlock()

	err = DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM quota_data").Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.Table("quota_data").CreateInBatches(rows, 1000).Error
	})
	if err != nil {
		return 0, err
	}

	return int64(len(rows)), nil
}

func GetAllQuotaDates(startTime int64, endTime int64, username string) (quotaData []*QuotaData, err error) {
	if username != "" {
		return GetQuotaDataByUsername(username, startTime, endTime)
	}
	var quotaDatas []*QuotaData
	// 从quota_data表中查询数据
	// only select model_name, sum(count) as count, sum(quota) as quota, model_name, created_at from quota_data group by model_name, created_at;
	//err = DB.Table("quota_data").Where("created_at >= ? and created_at <= ?", startTime, endTime).Find(&quotaDatas).Error
	err = DB.Table("quota_data").Select("model_name, sum(count) as count, sum(quota) as quota, sum(token_used) as token_used, created_at").Where("created_at >= ? and created_at <= ?", startTime, endTime).Group("model_name, created_at").Find(&quotaDatas).Error
	return quotaDatas, err
}
