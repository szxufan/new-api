package service

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cleanupDisableChannelLogs 清理测试产生的日志与渠道
func cleanupDisableChannelLogs(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		model.DB.Exec("DELETE FROM channels")
		model.DB.Exec("DELETE FROM users")
		model.LOG_DB.Exec("DELETE FROM logs")
	})
}

// seedRootUser 写入 root 用户，供 DisableChannel 记录日志归属
func seedRootUser(t *testing.T, id int) {
	t.Helper()
	user := &model.User{
		Id:       id,
		Username: "root",
		Role:     common.RoleRootUser,
		Status:   common.UserStatusEnabled,
	}
	require.NoError(t, model.DB.Create(user).Error)
}

// getLatestSystemLog 取最新一条系统日志
func getLatestSystemLog(t *testing.T) *model.Log {
	t.Helper()
	var log model.Log
	err := model.LOG_DB.Where("type = ?", model.LogTypeSystem).Order("id desc").First(&log).Error
	if err != nil {
		return nil
	}
	return &log
}

// TestDisableChannel_RecordsSystemLog 渠道自动禁用成功后，应在普通日志页（logs 表）留下 LogTypeSystem 日志并包含禁用原因
func TestDisableChannel_RecordsSystemLog(t *testing.T) {
	cleanupDisableChannelLogs(t)
	const rootUserId = 900
	seedRootUser(t, rootUserId)

	ch := makeBalanceChannel(t, 2001, 0, true, 1)

	ce := types.NewChannelError(
		ch.Id, ch.Type, ch.Name,
		ch.ChannelInfo.IsMultiKey, "", true, false,
	)
	reason := "upstream 401 unauthorized"
	DisableChannel(*ce, reason)

	// 渠道状态已变为 AutoDisabled
	var dbCh model.Channel
	require.NoError(t, model.DB.Where("id = ?", ch.Id).First(&dbCh).Error)
	assert.Equal(t, common.ChannelStatusAutoDisabled, dbCh.Status)

	// 已写入系统日志，归属 root 用户，内容包含禁用原因
	log := getLatestSystemLog(t)
	require.NotNil(t, log, "DisableChannel 成功后应记录 LogTypeSystem 日志")
	assert.Equal(t, model.LogTypeSystem, log.Type)
	assert.Equal(t, rootUserId, log.UserId)
	assert.Equal(t, "root", log.Username)
	assert.Contains(t, log.Content, fmt.Sprintf("（#%d）已被禁用", ch.Id))
	assert.Contains(t, log.Content, reason)
}

// TestDisableChannel_AutoBanOff_NoLog AutoBan=false 时跳过禁用，不应产生日志
func TestDisableChannel_AutoBanOff_NoLog(t *testing.T) {
	cleanupDisableChannelLogs(t)
	const rootUserId = 901
	seedRootUser(t, rootUserId)

	ch := makeBalanceChannel(t, 2002, 0, true, 1)

	ce := types.NewChannelError(ch.Id, ch.Type, ch.Name, false, "", false, false)
	DisableChannel(*ce, "should be skipped")

	// 渠道保持启用
	var dbCh model.Channel
	require.NoError(t, model.DB.Where("id = ?", ch.Id).First(&dbCh).Error)
	assert.Equal(t, common.ChannelStatusEnabled, dbCh.Status)

	// 不产生系统日志
	log := getLatestSystemLog(t)
	assert.Nil(t, log, "AutoBan=false 跳过禁用时不应记录日志")
}

// TestDisableChannelIfBalanceDepleted_RecordsSystemLog 余额耗尽自动禁用同样应留下系统日志
func TestDisableChannelIfBalanceDepleted_RecordsSystemLog(t *testing.T) {
	cleanupDisableChannelLogs(t)
	const rootUserId = 902
	seedRootUser(t, rootUserId)

	ch := makeBalanceChannel(t, 2003, 5.0, true, 1)
	ch.Balance = 0

	triggered := DisableChannelIfBalanceDepleted(ch, "余额耗尽（自动扣减）")
	require.True(t, triggered)

	log := getLatestSystemLog(t)
	require.NotNil(t, log, "余额耗尽禁用后应记录 LogTypeSystem 日志")
	assert.Equal(t, model.LogTypeSystem, log.Type)
	assert.Equal(t, rootUserId, log.UserId)
	assert.Contains(t, log.Content, "余额耗尽（自动扣减）")
}

// TestDisableChannelIfBalanceDepleted_NoRootUserNoPanic root 用户不存在时不 panic、不禁用日志缺失导致报错
func TestDisableChannelIfBalanceDepleted_NoRootUserNoPanic(t *testing.T) {
	cleanupDisableChannelLogs(t)
	// 不创建 root 用户
	ch := makeBalanceChannel(t, 2004, 0, true, 1)

	assert.NotPanics(t, func() {
		DisableChannelIfBalanceDepleted(ch, "余额耗尽")
	})

	// 渠道仍被禁用
	var dbCh model.Channel
	require.NoError(t, model.DB.Where("id = ?", ch.Id).First(&dbCh).Error)
	assert.Equal(t, common.ChannelStatusAutoDisabled, dbCh.Status)
}
