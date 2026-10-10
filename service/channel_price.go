package service

import (
	"time"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

// ResolveChannelPriceFactorsByChannelId 按渠道 ID 解析价格系数（异步任务结算用，
// 无 gin context）。渠道不存在或解析失败时返回全 1（不调整价格）。
// at 为时间基准（通常为任务提交时间），时间段判定使用其本地时区。
func ResolveChannelPriceFactorsByChannelId(channelId int, at time.Time) dto.PriceFactors {
	if channelId <= 0 {
		return dto.IdentityPriceFactors()
	}
	channel, err := model.CacheGetChannel(channelId)
	if err != nil || channel == nil {
		return dto.IdentityPriceFactors()
	}
	return channel.GetSetting().Price.Resolve(at)
}