package model

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

// GetDueBalanceResetChannels 查询到期（next_time <= now）需要重置余额的渠道，
// 按下次重置时间升序返回，最多 limit 条。next_time 为 NULL 或 0 表示未启用，不会被命中。
func GetDueBalanceResetChannels(limit int) ([]*Channel, error) {
	if limit <= 0 {
		limit = 300
	}
	now := GetDBTimestamp()
	var channels []*Channel
	err := DB.Where("balance_reset_next_time IS NOT NULL AND balance_reset_next_time > 0 AND balance_reset_next_time <= ?", now).
		Order("balance_reset_next_time asc").
		Limit(limit).
		Find(&channels).Error
	return channels, err
}

// ResetDueChannelBalances 批量重置到期渠道的余额，返回成功重置的渠道数量。
// 每条渠道在独立事务中加行锁（FOR UPDATE）后二次确认到期条件，避免与计费扣减并发覆盖。
// 重置语义：余额设为 BalanceResetAmount（绝对值，非累加），并按 BalanceResetPeriodDays 循环
// 推进下次重置时间直至未来；period_days <= 0 时重置一次后自动关闭（next_time 置 NULL）。
// 重置金额 > 0 且渠道处于自动禁用状态时，重置后自动重新启用（手动禁用不动）。
func ResetDueChannelBalances(batchSize int) (int, error) {
	channels, err := GetDueBalanceResetChannels(batchSize)
	if err != nil {
		return 0, err
	}
	if len(channels) == 0 {
		return 0, nil
	}
	now := GetDBTimestamp()
	resetCount := 0
	for _, ch := range channels {
		// 配置不完整（缺金额或下次时间）的渠道跳过，等管理员修正配置
		if ch.BalanceResetAmount == nil || ch.BalanceResetNextTime == nil {
			continue
		}
		channelId := ch.Id
		amount := *ch.BalanceResetAmount
		wasAutoDisabled := false
		err := DB.Transaction(func(tx *gorm.DB) error {
			var locked Channel
			// SQLite 不支持 FOR UPDATE（且 SQLite 部署为单写入者，无需行锁）
			lockTx := tx
			if !common.UsingSQLite {
				lockTx = tx.Set("gorm:query_option", "FOR UPDATE")
			}
			if err := lockTx.
				Where("id = ? AND balance_reset_next_time IS NOT NULL AND balance_reset_next_time > 0 AND balance_reset_next_time <= ?", channelId, now).
				First(&locked).Error; err != nil {
				// 行已被其他进程处理或配置被清空，跳过
				return nil
			}
			updates := map[string]interface{}{
				"balance":              amount,
				"balance_updated_time": now,
			}
			// 重置为正余额时标记"曾经有过非0余额"，维持余额耗尽自动禁用判定的一致性
			if amount > 0 && !locked.ChannelInfo.BalanceEverNonZero {
				locked.ChannelInfo.BalanceEverNonZero = true
				updates["channel_info"] = locked.ChannelInfo
			}
			// 推进下次重置时间：循环加周期直至落到未来（追赶停机期间错过的周期，但不补跑）
			periodDays := 0
			if locked.BalanceResetPeriodDays != nil {
				periodDays = *locked.BalanceResetPeriodDays
			}
			if periodDays > 0 {
				next := *locked.BalanceResetNextTime
				step := int64(periodDays) * 86400
				for next <= now {
					next += step
				}
				updates["balance_reset_next_time"] = next
			} else {
				updates["balance_reset_next_time"] = nil
			}
			if err := tx.Model(&Channel{}).Where("id = ?", channelId).Updates(updates).Error; err != nil {
				return err
			}
			wasAutoDisabled = locked.Status == common.ChannelStatusAutoDisabled
			return nil
		})
		if err != nil {
			return resetCount, err
		}
		resetCount++
		common.SysLog(fmt.Sprintf("channel balance reset: channel_id=%d, amount=%v", channelId, amount))
		// best-effort 刷新内存缓存中的余额（接受短暂不一致）
		if common.MemoryCacheEnabled {
			if fresh, err := GetChannelById(channelId, true); err == nil {
				CacheUpdateChannel(fresh)
			}
		}
		// 重置为正余额且渠道此前被自动禁用时，自动重新启用（含缓存与 ability 联动）
		if amount > 0 && wasAutoDisabled {
			UpdateChannelScheduleStatus(channelId, common.ChannelStatusEnabled, "周期重置余额")
		}
	}
	return resetCount, nil
}
