package service

import (
	"time"

	"github.com/QuantumNous/new-api/common"
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

// GetModelsWithZeroPriceChannel 返回指定分组下，存在价格系数（总系数 × 时段系数）
// 为 0 的启用渠道的模型集合。此类渠道上未配置价格的模型也可正常调用，
// 模型列表据此放行。
func GetModelsWithZeroPriceChannel(groups []string, at time.Time) map[string]bool {
	freeModels := make(map[string]bool)
	rows, err := model.GetEnabledModelChannelPriceSettings(groups)
	if err != nil {
		common.SysLog("failed to query model channel price settings: " + err.Error())
		return freeModels
	}
	for _, row := range rows {
		var setting dto.ChannelSettings
		if row.Setting != nil && *row.Setting != "" {
			if err := common.Unmarshal([]byte(*row.Setting), &setting); err != nil {
				continue
			}
		}
		if setting.Price.Resolve(at).Overall() == 0 {
			freeModels[row.Model] = true
		}
	}
	return freeModels
}
