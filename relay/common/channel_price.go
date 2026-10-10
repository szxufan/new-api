package common

import (
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"

	"github.com/gin-gonic/gin"
)

// ResolveChannelPriceFactors 解析当前请求渠道的价格系数（含时间段判定）。
// 优先使用 RelayInfo.ChannelMeta 中已解析的渠道设置（结算时为最终渠道），
// 其次回退到 gin context 中的渠道设置（预扣费时 handler 尚未初始化 ChannelMeta）；
// 均不可用时返回全 1（不调整价格）。
// 时间基准为请求开始时间（info.StartTime），时间段判定使用其本地时区（服务器本地时间）。
func ResolveChannelPriceFactors(c *gin.Context, info *RelayInfo) dto.PriceFactors {
	var price *dto.ChannelPriceSettings
	if info != nil && info.ChannelMeta != nil {
		price = info.ChannelMeta.ChannelSetting.Price
	}
	if price == nil && c != nil {
		if setting, ok := common.GetContextKeyType[dto.ChannelSettings](c, constant.ContextKeyChannelSetting); ok {
			price = setting.Price
		}
	}
	if price == nil {
		return dto.IdentityPriceFactors()
	}
	at := time.Now()
	if info != nil && !info.StartTime.IsZero() {
		at = info.StartTime
	}
	return price.Resolve(at)
}