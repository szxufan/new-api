package model

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"

	"github.com/bytedance/gopkg/util/gopool"
	"gorm.io/gorm"
)

type Log struct {
	Id               int    `json:"id" gorm:"index:idx_created_at_id,priority:1;index:idx_user_id_id,priority:2"`
	UserId           int    `json:"user_id" gorm:"index;index:idx_user_id_id,priority:1"`
	CreatedAt        int64  `json:"created_at" gorm:"bigint;index:idx_created_at_id,priority:2;index:idx_created_at_type"`
	Type             int    `json:"type" gorm:"index:idx_created_at_type"`
	Content          string `json:"content"`
	Username         string `json:"username" gorm:"index;index:index_username_model_name,priority:2;default:''"`
	TokenName        string `json:"token_name" gorm:"index;default:''"`
	ModelName        string `json:"model_name" gorm:"index;index:index_username_model_name,priority:1;default:''"`
	Quota            int    `json:"quota" gorm:"default:0"`
	PromptTokens     int    `json:"prompt_tokens" gorm:"default:0"`
	CompletionTokens int    `json:"completion_tokens" gorm:"default:0"`
	UseTime          int    `json:"use_time" gorm:"default:0"`
	IsStream         bool   `json:"is_stream"`
	ChannelId        int    `json:"channel" gorm:"index"`
	ChannelName      string `json:"channel_name" gorm:"->"`
	TokenId          int    `json:"token_id" gorm:"default:0;index"`
	Group            string `json:"group" gorm:"index"`
	Ip               string `json:"ip" gorm:"index;default:''"`
	RequestId         string `json:"request_id,omitempty" gorm:"type:varchar(64);index:idx_logs_request_id;default:''"`
	UpstreamRequestId string `json:"upstream_request_id,omitempty" gorm:"type:varchar(128);index:idx_logs_upstream_request_id;default:''"`
	Other             string `json:"other"`
	DisplayName       string `json:"display_name" gorm:"->"`
}

// don't use iota, avoid change log type value
const (
	LogTypeUnknown = 0
	LogTypeTopup   = 1
	LogTypeConsume = 2
	LogTypeManage  = 3
	LogTypeSystem  = 4
	LogTypeError   = 5
	LogTypeRefund  = 6
)

func formatUserLogs(logs []*Log, startIdx int) {
	for i := range logs {
		logs[i].ChannelName = ""
		var otherMap map[string]interface{}
		otherMap, _ = common.StrToMap(logs[i].Other)
		if otherMap != nil {
			// Remove admin-only debug fields.
			delete(otherMap, "admin_info")
			// delete(otherMap, "reject_reason")
			delete(otherMap, "stream_status")
			// Remove model mapping info — non-admin users should not see
			// the upstream (actual) model name that the request was routed to.
			delete(otherMap, "is_model_mapped")
			delete(otherMap, "upstream_model_name")
			// 重试信息仅管理员可见
			delete(otherMap, "retry_count")
			delete(otherMap, "retry_duration_ms")
		}
		logs[i].Other = common.MapToJsonStr(otherMap)
		logs[i].Id = startIdx + i + 1
	}
}

func GetLogByTokenId(tokenId int) (logs []*Log, err error) {
	err = LOG_DB.Model(&Log{}).Where("token_id = ?", tokenId).Order("id desc").Limit(common.MaxRecentItems).Find(&logs).Error
	formatUserLogs(logs, 0)
	return logs, err
}

func RecordLog(userId int, logType int, content string) {
	if logType == LogTypeConsume && !common.LogConsumeEnabled {
		return
	}
	username, _ := GetUsernameById(userId, false)
	log := &Log{
		UserId:    userId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      logType,
		Content:   content,
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		common.SysLog("failed to record log: " + err.Error())
	}
}

// RecordLogWithAdminInfo 记录操作日志，并将管理员相关信息存入 Other.admin_info，
func RecordLogWithAdminInfo(userId int, logType int, content string, adminInfo map[string]interface{}) {
	if logType == LogTypeConsume && !common.LogConsumeEnabled {
		return
	}
	username, _ := GetUsernameById(userId, false)
	log := &Log{
		UserId:    userId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      logType,
		Content:   content,
	}
	if len(adminInfo) > 0 {
		other := map[string]interface{}{
			"admin_info": adminInfo,
		}
		log.Other = common.MapToJsonStr(other)
	}
	if err := LOG_DB.Create(log).Error; err != nil {
		common.SysLog("failed to record log: " + err.Error())
	}
}

func RecordTopupLog(userId int, content string, callerIp string, paymentMethod string, callbackPaymentMethod string) {
	username, _ := GetUsernameById(userId, false)
	adminInfo := map[string]interface{}{
		"server_ip":               common.GetIp(),
		"node_name":               common.NodeName,
		"caller_ip":               callerIp,
		"payment_method":          paymentMethod,
		"callback_payment_method": callbackPaymentMethod,
		"version":                 common.Version,
	}
	other := map[string]interface{}{
		"admin_info": adminInfo,
	}
	log := &Log{
		UserId:    userId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      LogTypeTopup,
		Content:   content,
		Ip:        callerIp,
		Other:     common.MapToJsonStr(other),
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		common.SysLog("failed to record topup log: " + err.Error())
	}
}

func RecordErrorLog(c *gin.Context, userId int, channelId int, modelName string, tokenName string, content string, tokenId int, useTimeSeconds int,
	isStream bool, group string, other map[string]interface{}) {
	logger.LogInfo(c, fmt.Sprintf("record error log: userId=%d, channelId=%d, modelName=%s, tokenName=%s, content=%s", userId, channelId, modelName, tokenName, content))
	username := c.GetString("username")
	requestId := c.GetString(common.RequestIdKey)
	upstreamRequestId := c.GetString(common.UpstreamRequestIdKey)
	otherStr := common.MapToJsonStr(other)
	// 判断是否需要记录 IP
	needRecordIp := false
	if settingMap, err := GetUserSetting(userId, false); err == nil {
		if settingMap.RecordIpLog {
			needRecordIp = true
		}
	}
	log := &Log{
		UserId:           userId,
		Username:         username,
		CreatedAt:        common.GetTimestamp(),
		Type:             LogTypeError,
		Content:          content,
		PromptTokens:     0,
		CompletionTokens: 0,
		TokenName:        tokenName,
		ModelName:        modelName,
		Quota:            0,
		ChannelId:        channelId,
		TokenId:          tokenId,
		UseTime:          useTimeSeconds,
		IsStream:         isStream,
		Group:            group,
		Ip: func() string {
			if needRecordIp {
				return c.ClientIP()
			}
			return ""
		}(),
		RequestId:         requestId,
		UpstreamRequestId: upstreamRequestId,
		Other:             otherStr,
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		logger.LogError(c, "failed to record log: "+err.Error())
	}
}

type RecordConsumeLogParams struct {
	ChannelId        int                    `json:"channel_id"`
	PromptTokens     int                    `json:"prompt_tokens"`
	CompletionTokens int                    `json:"completion_tokens"`
	ModelName        string                 `json:"model_name"`
	TokenName        string                 `json:"token_name"`
	Quota            int                    `json:"quota"`
	Content          string                 `json:"content"`
	TokenId          int                    `json:"token_id"`
	UseTimeSeconds   int                    `json:"use_time_seconds"`
	IsStream         bool                   `json:"is_stream"`
	Group            string                 `json:"group"`
	Other            map[string]interface{} `json:"other"`
}

func RecordConsumeLog(c *gin.Context, userId int, params RecordConsumeLogParams) {
	if !common.LogConsumeEnabled {
		return
	}
	logger.LogInfo(c, fmt.Sprintf("record consume log: userId=%d, params=%s", userId, common.GetJsonString(params)))
	username := c.GetString("username")
	requestId := c.GetString(common.RequestIdKey)
	upstreamRequestId := c.GetString(common.UpstreamRequestIdKey)
	otherStr := common.MapToJsonStr(params.Other)
	// 判断是否需要记录 IP
	needRecordIp := false
	if settingMap, err := GetUserSetting(userId, false); err == nil {
		if settingMap.RecordIpLog {
			needRecordIp = true
		}
	}
	log := &Log{
		UserId:           userId,
		Username:         username,
		CreatedAt:        common.GetTimestamp(),
		Type:             LogTypeConsume,
		Content:          params.Content,
		PromptTokens:     params.PromptTokens,
		CompletionTokens: params.CompletionTokens,
		TokenName:        params.TokenName,
		ModelName:        params.ModelName,
		Quota:            params.Quota,
		ChannelId:        params.ChannelId,
		TokenId:          params.TokenId,
		UseTime:          params.UseTimeSeconds,
		IsStream:         params.IsStream,
		Group:            params.Group,
		Ip: func() string {
			if needRecordIp {
				return c.ClientIP()
			}
			return ""
		}(),
		RequestId:         requestId,
		UpstreamRequestId: upstreamRequestId,
		Other:             otherStr,
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		logger.LogError(c, "failed to record log: "+err.Error())
	}
	if common.DataExportEnabled {
		gopool.Go(func() {
			LogQuotaData(userId, username, params.ModelName, params.Quota, common.GetTimestamp(), params.PromptTokens+params.CompletionTokens)
		})
	}
}

type RecordTaskBillingLogParams struct {
	UserId    int
	LogType   int
	Content   string
	ChannelId int
	ModelName string
	Quota     int
	TokenId   int
	Group     string
	Other     map[string]interface{}
}

func RecordTaskBillingLog(params RecordTaskBillingLogParams) {
	if params.LogType == LogTypeConsume && !common.LogConsumeEnabled {
		return
	}
	username, _ := GetUsernameById(params.UserId, false)
	tokenName := ""
	if params.TokenId > 0 {
		if token, err := GetTokenById(params.TokenId); err == nil {
			tokenName = token.Name
		}
	}
	log := &Log{
		UserId:    params.UserId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      params.LogType,
		Content:   params.Content,
		TokenName: tokenName,
		ModelName: params.ModelName,
		Quota:     params.Quota,
		ChannelId: params.ChannelId,
		TokenId:   params.TokenId,
		Group:     params.Group,
		Other:     common.MapToJsonStr(params.Other),
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		common.SysLog("failed to record task billing log: " + err.Error())
	}
}

// pgJsonObjectExpr 构造 PostgreSQL 表达式：当 column 是合法 JSON 对象/数组时返回该 jsonb 值，否则返回 NULL。
// 历史数据中 other 字段可能存在非 JSON 文本（如空字符串、普通文本），直接执行 column::jsonb
// 会报 invalid input syntax for type json (SQLSTATE 22P02)，因此先按 JSON 对象/数组的首字符特征
// 做前置校验，避免按 retry_count / node_name 过滤时整个查询报错。
func pgJsonObjectExpr(column string) string {
	return fmt.Sprintf("CASE WHEN %s ~ '^[[:space:]]*[{[]' THEN %s::jsonb ELSE NULL END", column, column)
}

func GetAllLogs(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, startIdx int, num int, channel int, group string, requestId string, upstreamRequestId string, retryCount int, nodeName string) (logs []*Log, total int64, err error) {
	var tx *gorm.DB
	if logType == LogTypeUnknown {
		tx = LOG_DB
	} else {
		tx = LOG_DB.Where("logs.type = ?", logType)
	}

	if modelName != "" {
		// 先包裹 % 实现部分匹配（若用户未自行包含 %），再 sanitize
		if !strings.Contains(modelName, "%") {
			modelName = "%" + modelName + "%"
		}
		modelNamePattern, err := sanitizeLikePattern(modelName)
		if err != nil {
			return nil, 0, err
		}
		tx = tx.Where("LOWER(logs.model_name) LIKE LOWER(?) ESCAPE '!'", modelNamePattern)
	}
	if username != "" {
		// 先包裹 % 实现部分匹配
		if !strings.Contains(username, "%") {
			username = "%" + username + "%"
		}
		usernamePattern, err := sanitizeLikePattern(username)
		if err != nil {
			return nil, 0, err
		}
		// 两步查询：用户表在主库 DB，日志可能在独立的 LOG_DB，跨库子查询不可行
		var userIds []int
		if err = DB.Model(&User{}).Select("id").
			Where("LOWER(username) LIKE LOWER(?) OR LOWER(display_name) LIKE LOWER(?)",
				usernamePattern, usernamePattern).
			Pluck("id", &userIds).Error; err != nil {
			return nil, 0, err
		}
		if len(userIds) == 0 {
			// 无匹配用户，直接返回空结果
			return []*Log{}, 0, nil
		}
		tx = tx.Where("logs.user_id IN ?", userIds)
	}
	if tokenName != "" {
		if !strings.Contains(tokenName, "%") {
			tokenName = "%" + tokenName + "%"
		}
		tokenNamePattern, err := sanitizeLikePattern(tokenName)
		if err != nil {
			return nil, 0, err
		}
		tx = tx.Where("LOWER(logs.token_name) LIKE LOWER(?) ESCAPE '!'", tokenNamePattern)
	}
	if requestId != "" {
		tx = tx.Where("logs.request_id = ?", requestId)
	}
	if upstreamRequestId != "" {
		tx = tx.Where("logs.upstream_request_id = ?", upstreamRequestId)
	}
	if startTimestamp != 0 {
		tx = tx.Where("logs.created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("logs.created_at <= ?", endTimestamp)
	}
	if channel != 0 {
		tx = tx.Where("logs.channel_id = ?", channel)
	}
	if group != "" {
		tx = tx.Where("logs."+logGroupCol+" = ?", group)
	}
	if retryCount > 0 {
		// 重试次数存储在 other 字段的 JSON 中，按数据库类型选择提取语法。
		// MySQL/SQLite 的 JSON 提取函数对非法 JSON 返回 NULL（不报错），PostgreSQL 的 ::jsonb
		// 转换会抛 22P02，需要先经 pgJsonObjectExpr 前置校验容错。
		var retryCondition string
		if common.UsingPostgreSQL {
			retryCondition = fmt.Sprintf("(%s ->> 'retry_count')::int > ?", pgJsonObjectExpr("logs.other"))
		} else if common.UsingMySQL {
			retryCondition = "CAST(JSON_EXTRACT(logs.other, '$.retry_count') AS UNSIGNED) > ?"
		} else { // SQLite
			retryCondition = "CAST(json_extract(logs.other, '$.retry_count') AS INTEGER) > ?"
		}
		tx = tx.Where(retryCondition, retryCount)
	}
	if nodeName != "" {
		var nodeCondition string
		if common.UsingPostgreSQL {
			nodeCondition = fmt.Sprintf("(%s) -> 'admin_info' ->> 'node_name' = ?", pgJsonObjectExpr("logs.other"))
		} else if common.UsingMySQL {
			nodeCondition = "JSON_UNQUOTE(JSON_EXTRACT(logs.other, '$.admin_info.node_name')) = ?"
		} else { // SQLite
			nodeCondition = "json_extract(logs.other, '$.admin_info.node_name') = ?"
		}
		tx = tx.Where(nodeCondition, nodeName)
	}
	err = tx.Model(&Log{}).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}
	err = tx.Order("logs.id desc").Limit(num).Offset(startIdx).Find(&logs).Error
	if err != nil {
		return nil, 0, err
	}

	channelIds := types.NewSet[int]()
	for _, log := range logs {
		if log.ChannelId != 0 {
			channelIds.Add(log.ChannelId)
		}
	}

	if channelIds.Len() > 0 {
		var channels []struct {
			Id   int    `gorm:"column:id"`
			Name string `gorm:"column:name"`
		}
		if common.MemoryCacheEnabled {
			// Cache get channel
			for _, channelId := range channelIds.Items() {
				if cacheChannel, err := CacheGetChannel(channelId); err == nil {
					channels = append(channels, struct {
						Id   int    `gorm:"column:id"`
						Name string `gorm:"column:name"`
					}{
						Id:   channelId,
						Name: cacheChannel.Name,
					})
				}
			}
		} else {
			// Bulk query channels from DB
			if err = DB.Table("channels").Select("id, name").Where("id IN ?", channelIds.Items()).Find(&channels).Error; err != nil {
				return logs, total, err
			}
		}
		channelMap := make(map[int]string, len(channels))
		for _, channel := range channels {
			channelMap[channel.Id] = channel.Name
		}
		for i := range logs {
			logs[i].ChannelName = channelMap[logs[i].ChannelId]
		}
	}

	// 批量查询用户表填充 DisplayName（用户表在主库 DB）
	userIds := types.NewSet[int]()
	for _, log := range logs {
		if log.UserId != 0 {
			userIds.Add(log.UserId)
		}
	}
	if userIds.Len() > 0 {
		var users []struct {
			Id          int    `gorm:"column:id"`
			DisplayName string `gorm:"column:display_name"`
		}
		if err = DB.Table("users").Select("id, display_name").Where("id IN ?", userIds.Items()).Find(&users).Error; err != nil {
			return logs, total, err
		}
		userMap := make(map[int]string, len(users))
		for _, u := range users {
			userMap[u.Id] = u.DisplayName
		}
		for i := range logs {
			if name, ok := userMap[logs[i].UserId]; ok && name != "" {
				logs[i].DisplayName = name
			}
		}
	}

	return logs, total, err
}

const logSearchCountLimit = 10000

func GetUserLogs(userId int, logType int, startTimestamp int64, endTimestamp int64, modelName string, tokenName string, startIdx int, num int, group string, requestId string, upstreamRequestId string) (logs []*Log, total int64, err error) {
	var tx *gorm.DB
	if logType == LogTypeUnknown {
		tx = LOG_DB.Where("logs.user_id = ?", userId)
	} else {
		tx = LOG_DB.Where("logs.user_id = ? and logs.type = ?", userId, logType)
	}

	if modelName != "" {
		if !strings.Contains(modelName, "%") {
			modelName = "%" + modelName + "%"
		}
		modelNamePattern, err := sanitizeLikePattern(modelName)
		if err != nil {
			return nil, 0, err
		}
		tx = tx.Where("LOWER(logs.model_name) LIKE LOWER(?) ESCAPE '!'", modelNamePattern)
	}
	if tokenName != "" {
		if !strings.Contains(tokenName, "%") {
			tokenName = "%" + tokenName + "%"
		}
		tokenNamePattern, err := sanitizeLikePattern(tokenName)
		if err != nil {
			return nil, 0, err
		}
		tx = tx.Where("LOWER(logs.token_name) LIKE LOWER(?) ESCAPE '!'", tokenNamePattern)
	}
	if requestId != "" {
		tx = tx.Where("logs.request_id = ?", requestId)
	}
	if upstreamRequestId != "" {
		tx = tx.Where("logs.upstream_request_id = ?", upstreamRequestId)
	}
	if startTimestamp != 0 {
		tx = tx.Where("logs.created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("logs.created_at <= ?", endTimestamp)
	}
	if group != "" {
		tx = tx.Where("logs."+logGroupCol+" = ?", group)
	}
	err = tx.Model(&Log{}).Limit(logSearchCountLimit).Count(&total).Error
	if err != nil {
		common.SysError("failed to count user logs: " + err.Error())
		return nil, 0, errors.New("查询日志失败")
	}
	err = tx.Order("logs.id desc").Limit(num).Offset(startIdx).Find(&logs).Error
	if err != nil {
		common.SysError("failed to search user logs: " + err.Error())
		return nil, 0, errors.New("查询日志失败")
	}

	formatUserLogs(logs, startIdx)
	return logs, total, err
}

type Stat struct {
	Quota int `json:"quota"`
	Rpm   int `json:"rpm"`
	Tpm   int `json:"tpm"`
}

func SumUsedQuota(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, channel int, group string, retryCount int) (stat Stat, err error) {
	tx := LOG_DB.Table("logs").Select("sum(quota) quota")

	// 为rpm和tpm创建单独的查询
	rpmTpmQuery := LOG_DB.Table("logs").Select("count(*) rpm, sum(prompt_tokens) + sum(completion_tokens) tpm")

	if username != "" {
		// 两步查询：用户表在主库 DB，日志可能在独立的 LOG_DB，跨库子查询不可行
		if !strings.Contains(username, "%") {
			username = "%" + username + "%"
		}
		usernamePattern, err := sanitizeLikePattern(username)
		if err != nil {
			return stat, err
		}
		var userIds []int
		if err = DB.Model(&User{}).Select("id").
			Where("LOWER(username) LIKE LOWER(?) OR LOWER(display_name) LIKE LOWER(?)",
				usernamePattern, usernamePattern).
			Pluck("id", &userIds).Error; err != nil {
			return stat, err
		}
		if len(userIds) == 0 {
			return stat, nil // 无匹配用户，统计为 0
		}
		tx = tx.Where("user_id IN ?", userIds)
		rpmTpmQuery = rpmTpmQuery.Where("user_id IN ?", userIds)
	}
	if tokenName != "" {
		if !strings.Contains(tokenName, "%") {
			tokenName = "%" + tokenName + "%"
		}
		tokenNamePattern, err := sanitizeLikePattern(tokenName)
		if err != nil {
			return stat, err
		}
		tx = tx.Where("LOWER(token_name) LIKE LOWER(?) ESCAPE '!'", tokenNamePattern)
		rpmTpmQuery = rpmTpmQuery.Where("LOWER(token_name) LIKE LOWER(?) ESCAPE '!'", tokenNamePattern)
	}
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	if modelName != "" {
		if !strings.Contains(modelName, "%") {
			modelName = "%" + modelName + "%"
		}
		modelNamePattern, err := sanitizeLikePattern(modelName)
		if err != nil {
			return stat, err
		}
		tx = tx.Where("LOWER(model_name) LIKE LOWER(?) ESCAPE '!'", modelNamePattern)
		rpmTpmQuery = rpmTpmQuery.Where("LOWER(model_name) LIKE LOWER(?) ESCAPE '!'", modelNamePattern)
	}
	if channel != 0 {
		tx = tx.Where("channel_id = ?", channel)
		rpmTpmQuery = rpmTpmQuery.Where("channel_id = ?", channel)
	}
	if group != "" {
		tx = tx.Where(logGroupCol+" = ?", group)
		rpmTpmQuery = rpmTpmQuery.Where(logGroupCol+" = ?", group)
	}
	if retryCount > 0 {
		// 重试次数存储在 other 字段的 JSON 中，按数据库类型选择提取语法。
		// MySQL/SQLite 的 JSON 提取函数对非法 JSON 返回 NULL（不报错），PostgreSQL 的 ::jsonb
		// 转换会抛 22P02，需要先经 pgJsonObjectExpr 前置校验容错。
		var retryCondition string
		if common.UsingPostgreSQL {
			retryCondition = fmt.Sprintf("(%s ->> 'retry_count')::int > ?", pgJsonObjectExpr("other"))
		} else if common.UsingMySQL {
			retryCondition = "CAST(JSON_EXTRACT(other, '$.retry_count') AS UNSIGNED) > ?"
		} else { // SQLite
			retryCondition = "CAST(json_extract(other, '$.retry_count') AS INTEGER) > ?"
		}
		tx = tx.Where(retryCondition, retryCount)
		rpmTpmQuery = rpmTpmQuery.Where(retryCondition, retryCount)
	}

	tx = tx.Where("type = ?", LogTypeConsume)
	rpmTpmQuery = rpmTpmQuery.Where("type = ?", LogTypeConsume)

	// 只统计最近60秒的rpm和tpm
	rpmTpmQuery = rpmTpmQuery.Where("created_at >= ?", time.Now().Add(-60*time.Second).Unix())

	// 执行查询
	if err := tx.Scan(&stat).Error; err != nil {
		common.SysError("failed to query log stat: " + err.Error())
		return stat, errors.New("查询统计数据失败")
	}
	if err := rpmTpmQuery.Scan(&stat).Error; err != nil {
		common.SysError("failed to query rpm/tpm stat: " + err.Error())
		return stat, errors.New("查询统计数据失败")
	}

	return stat, nil
}

func SumUsedToken(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string) (token int) {
	tx := LOG_DB.Table("logs").Select("ifnull(sum(prompt_tokens),0) + ifnull(sum(completion_tokens),0)")
	if username != "" {
		tx = tx.Where("username = ?", username)
	}
	if tokenName != "" {
		tx = tx.Where("token_name = ?", tokenName)
	}
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	if modelName != "" {
		tx = tx.Where("model_name = ?", modelName)
	}
	tx.Where("type = ?", LogTypeConsume).Scan(&token)
	return token
}

func DeleteOldLog(ctx context.Context, targetTimestamp int64, limit int) (int64, error) {
	var total int64 = 0

	for {
		if nil != ctx.Err() {
			return total, ctx.Err()
		}

		result := LOG_DB.Where("created_at < ?", targetTimestamp).Limit(limit).Delete(&Log{})
		if nil != result.Error {
			return total, result.Error
		}

		total += result.RowsAffected

		if result.RowsAffected < int64(limit) {
			break
		}
	}

	return total, nil
}
