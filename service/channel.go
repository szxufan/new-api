package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
)

func formatNotifyType(channelId int, status int) string {
	return fmt.Sprintf("%s_%d_%d", dto.NotifyTypeChannelUpdate, channelId, status)
}

// disable & notify
func DisableChannel(channelError types.ChannelError, reason string) {
	common.SysLog(fmt.Sprintf("通道「%s」（#%d）发生错误，准备禁用，原因：%s", channelError.ChannelName, channelError.ChannelId, reason))

	// 检查是否启用自动禁用功能
	if !channelError.AutoBan {
		common.SysLog(fmt.Sprintf("通道「%s」（#%d）未启用自动禁用功能，跳过禁用操作", channelError.ChannelName, channelError.ChannelId))
		return
	}

	success := model.UpdateChannelStatus(channelError.ChannelId, channelError.UsingKey, common.ChannelStatusAutoDisabled, reason)
	if success {
		subject := fmt.Sprintf("通道「%s」（#%d）已被禁用", channelError.ChannelName, channelError.ChannelId)
		content := fmt.Sprintf("通道「%s」（#%d）已被禁用，原因：%s", channelError.ChannelName, channelError.ChannelId, reason)
		NotifyRootUser(formatNotifyType(channelError.ChannelId, common.ChannelStatusAutoDisabled), subject, content)
		// 在 /usage-logs/common 普通日志页留下系统日志，记录禁用原因（归属 root 用户，便于管理员统一查看）
		if rootUser := model.GetRootUser(); rootUser != nil {
			model.RecordLog(rootUser.Id, model.LogTypeSystem, content)
		}
	}
}

func EnableChannel(channelId int, usingKey string, channelName string) {
	success := model.UpdateChannelStatus(channelId, usingKey, common.ChannelStatusEnabled, "")
	if success {
		subject := fmt.Sprintf("通道「%s」（#%d）已被启用", channelName, channelId)
		content := fmt.Sprintf("通道「%s」（#%d）已被启用", channelName, channelId)
		NotifyRootUser(formatNotifyType(channelId, common.ChannelStatusEnabled), subject, content)
	}
}

// RateLimitChannelKey429 对收到429的Key进行短时限流
func RateLimitChannelKey429(channelError types.ChannelError) {
	if !operation_setting.RateLimit429Enabled || !channelError.AutoBan {
		return
	}

	rateLimitUntil := time.Now().Add(time.Duration(operation_setting.RateLimit429DurationMinutes) * time.Minute).Unix()
	reason := fmt.Sprintf("429 rate limit for %d minutes", operation_setting.RateLimit429DurationMinutes)

	common.SysLog(fmt.Sprintf("通道「%s」（#%d）收到429，准备限流，时长：%d分钟", channelError.ChannelName, channelError.ChannelId, operation_setting.RateLimit429DurationMinutes))

	if channelError.IsMultiKey {
		// Key级别限流
		success := model.RateLimitChannelKey(channelError.ChannelId, channelError.UsingKey, rateLimitUntil, reason)
		if success {
			subject := fmt.Sprintf("通道「%s」（#%d）Key被429限流", channelError.ChannelName, channelError.ChannelId)
			content := fmt.Sprintf("通道「%s」（#%d）的某个Key因429错误被限流%d分钟", channelError.ChannelName, channelError.ChannelId, operation_setting.RateLimit429DurationMinutes)
			NotifyRootUser(formatNotifyType(channelError.ChannelId, common.ChannelStatusRateLimited429), subject, content)
		}
	} else {
		// 单Key渠道，整体限流
		success := model.RateLimitChannel(channelError.ChannelId, common.ChannelStatusRateLimited429, rateLimitUntil, reason)
		if success {
			subject := fmt.Sprintf("通道「%s」（#%d）被429限流", channelError.ChannelName, channelError.ChannelId)
			content := fmt.Sprintf("通道「%s」（#%d）因429错误被限流%d分钟", channelError.ChannelName, channelError.ChannelId, operation_setting.RateLimit429DurationMinutes)
			NotifyRootUser(formatNotifyType(channelError.ChannelId, common.ChannelStatusRateLimited429), subject, content)
		}
	}
}

// ManualRateLimitChannel 手动设置渠道小时级限流
func ManualRateLimitChannel(channelId int, channelName string, durationHours float64, reason string) {
	rateLimitUntil := time.Now().Add(time.Duration(durationHours) * time.Hour).Unix()
	success := model.RateLimitChannel(channelId, common.ChannelStatusManuallyRateLimited, rateLimitUntil, reason)
	if success {
		subject := fmt.Sprintf("通道「%s」（#%d）已被手动限流", channelName, channelId)
		content := fmt.Sprintf("通道「%s」（#%d）已被手动限流%.1f小时，原因：%s", channelName, channelId, durationHours, reason)
		NotifyRootUser(formatNotifyType(channelId, common.ChannelStatusManuallyRateLimited), subject, content)
	}
}

// UnrateLimitChannel 解除渠道限流
func UnrateLimitChannel(channelId int, usingKey string, channelName string) {
	success := model.UpdateChannelStatus(channelId, usingKey, common.ChannelStatusEnabled, "")
	if success {
		subject := fmt.Sprintf("通道「%s」（#%d）已解除限流", channelName, channelId)
		content := fmt.Sprintf("通道「%s」（#%d）已解除限流", channelName, channelId)
		NotifyRootUser(formatNotifyType(channelId, common.ChannelStatusEnabled), subject, content)
	}
}

// DisableChannelIfBalanceDepleted 检查渠道余额是否耗尽并自动禁用
// 统一封装禁用条件，供消费扣减、余额查询、手动编辑等所有路径复用
// 返回 true 表示已触发禁用
func DisableChannelIfBalanceDepleted(channel *model.Channel, reason string) bool {
	if channel == nil {
		return false
	}
	// 条件1：曾经有过非0余额（从未有过余额的渠道不禁用）
	if !channel.ChannelInfo.BalanceEverNonZero {
		return false
	}
	// 条件2：当前余额已耗尽（<=0）
	if channel.Balance > 0 {
		return false
	}
	// 条件3：渠道启用了自动禁用（尊重 AutoBan 设置）
	if !channel.GetAutoBan() {
		return false
	}
	DisableChannel(*types.NewChannelError(
		channel.Id, channel.Type, channel.Name,
		channel.ChannelInfo.IsMultiKey, "", channel.GetAutoBan(),
		channel.GetDisable429Ban(),
	), reason)
	return true
}

// UpdateChannelQuotaAndBalance 更新渠道已用额度并扣减余额，检查余额耗尽自动禁用
// 供所有计费路径调用，替代直接调用 model.UpdateChannelUsedQuota
func UpdateChannelQuotaAndBalance(channelId int, quota int) {
	// 1. 更新已用额度统计（可能批量延迟）
	model.UpdateChannelUsedQuota(channelId, quota)
	if quota <= 0 {
		return
	}
	// 2. 防御 QuotaPerUnit 异常配置（除零保护）
	if common.QuotaPerUnit <= 0 {
		common.SysLog(fmt.Sprintf("skip balance deduction: QuotaPerUnit is invalid (%v), channel_id=%d", common.QuotaPerUnit, channelId))
		return
	}
	// 3. 即时扣减余额（独立于批量更新，使用 quota/QuotaPerUnit 作为近似上游成本）
	deduction := float64(quota) / common.QuotaPerUnit
	channel, err := model.DeductChannelBalance(channelId, deduction)
	if err != nil || channel == nil {
		return
	}
	// 4. best-effort 更新缓存中的余额（接受短暂不一致）
	if common.MemoryCacheEnabled {
		model.CacheUpdateChannel(channel)
	}
	// 5. 检查余额耗尽自动禁用
	DisableChannelIfBalanceDepleted(channel, "余额耗尽（自动扣减）")
}

func ShouldDisableChannel(err *types.NewAPIError) bool {
	if !common.AutomaticDisableChannelEnabled {
		return false
	}
	if err == nil {
		return false
	}
	if types.IsChannelError(err) {
		return true
	}
	if types.IsSkipRetryError(err) {
		return false
	}
	if operation_setting.ShouldDisableByStatusCode(err.StatusCode) {
		return true
	}

	lowerMessage := strings.ToLower(err.Error())
	search, _ := AcSearch(lowerMessage, operation_setting.AutomaticDisableKeywords, true)
	return search
}

func ShouldEnableChannel(newAPIError *types.NewAPIError, status int) bool {
	if !common.AutomaticEnableChannelEnabled {
		return false
	}
	if newAPIError != nil {
		return false
	}
	if status != common.ChannelStatusAutoDisabled {
		return false
	}
	return true
}
