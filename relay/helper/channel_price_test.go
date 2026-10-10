package helper

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func pf(v float64) *float64 {
	return &v
}

// setChannelPriceSetting 将渠道价格系数注入 gin context（模拟预扣费时
// handler 尚未初始化 ChannelMeta 的场景）。
func setChannelPriceSetting(t *testing.T, ctx *gin.Context, price *dto.ChannelPriceSettings) {
	t.Helper()
	common.SetContextKey(ctx, constant.ContextKeyChannelSetting, dto.ChannelSettings{Price: price})
}

func TestModelPriceHelperAppliesChannelPriceFactors(t *testing.T) {
	withFollowTestEnv(t, map[string]string{})
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"channel-price-model":1}`))

	ctx := newFollowTestContext(t)
	setChannelPriceSetting(t, ctx, &dto.ChannelPriceSettings{
		Total:      pf(0.5),
		Input:      pf(2),
		Completion: pf(3),
	})
	info := &relaycommon.RelayInfo{
		OriginModelName: "channel-price-model",
		UserGroup:       "default",
		UsingGroup:      "default",
	}

	priceData, err := ModelPriceHelper(ctx, info, 1000, &types.TokenCountMeta{MaxTokens: 500})
	require.NoError(t, err)
	// (1000×2 + 500×3) × modelRatio(1) × groupRatio(1) × overall(0.5) = 1750
	require.Equal(t, 1750, priceData.QuotaToPreConsume)
}

func TestModelPriceHelperIdentityChannelFactorsMatchesLegacy(t *testing.T) {
	withFollowTestEnv(t, map[string]string{})
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"channel-price-model":1}`))

	ctx := newFollowTestContext(t)
	info := &relaycommon.RelayInfo{
		OriginModelName: "channel-price-model",
		UserGroup:       "default",
		UsingGroup:      "default",
	}

	priceData, err := ModelPriceHelper(ctx, info, 1000, &types.TokenCountMeta{MaxTokens: 500})
	require.NoError(t, err)
	// 未配置渠道系数时与旧公式一致：(1000 + 500) × 1 × 1 = 1500
	require.Equal(t, 1500, priceData.QuotaToPreConsume)
}

func TestModelPriceHelperChannelTimeWindowFactor(t *testing.T) {
	withFollowTestEnv(t, map[string]string{})
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"channel-price-model":1}`))

	ctx := newFollowTestContext(t)
	setChannelPriceSetting(t, ctx, &dto.ChannelPriceSettings{
		TimeWindows: []dto.PriceTimeWindow{
			{TimeWindow: dto.TimeWindow{Start: "12:00", End: "14:00"}, Ratio: 0.5},
		},
	})

	// 命中时段：13:00 → overall = 0.5
	info := &relaycommon.RelayInfo{
		OriginModelName: "channel-price-model",
		UserGroup:       "default",
		UsingGroup:      "default",
		StartTime:       time.Date(2026, 10, 10, 13, 0, 0, 0, time.Local),
	}
	priceData, err := ModelPriceHelper(ctx, info, 1000, &types.TokenCountMeta{})
	require.NoError(t, err)
	require.Equal(t, 500, priceData.QuotaToPreConsume)

	// 未命中时段：15:00 → overall = 1
	info2 := &relaycommon.RelayInfo{
		OriginModelName: "channel-price-model",
		UserGroup:       "default",
		UsingGroup:      "default",
		StartTime:       time.Date(2026, 10, 10, 15, 0, 0, 0, time.Local),
	}
	priceData2, err := ModelPriceHelper(ctx, info2, 1000, &types.TokenCountMeta{})
	require.NoError(t, err)
	require.Equal(t, 1000, priceData2.QuotaToPreConsume)
}

func TestModelPriceHelperPerCallAppliesChannelFactors(t *testing.T) {
	withFollowTestEnv(t, map[string]string{})
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"channel-price-percall":0.5}`))

	ctx := newFollowTestContext(t)
	setChannelPriceSetting(t, ctx, &dto.ChannelPriceSettings{Total: pf(2)})
	info := &relaycommon.RelayInfo{
		OriginModelName: "channel-price-percall",
		UserGroup:       "default",
		UsingGroup:      "default",
	}

	priceData, err := ModelPriceHelperPerCall(ctx, info)
	require.NoError(t, err)
	require.True(t, priceData.UsePrice)
	// 0.5 × QuotaPerUnit × groupRatio(1) × overall(2)
	require.Equal(t, int(0.5*common.QuotaPerUnit*2), priceData.Quota)

	// 无系数渠道：0.5 × QuotaPerUnit
	ctx2 := newFollowTestContext(t)
	info2 := &relaycommon.RelayInfo{
		OriginModelName: "channel-price-percall",
		UserGroup:       "default",
		UsingGroup:      "default",
	}
	priceData2, err := ModelPriceHelperPerCall(ctx2, info2)
	require.NoError(t, err)
	require.Equal(t, int(0.5*common.QuotaPerUnit), priceData2.Quota)
}

func TestModelPriceHelperTieredAppliesChannelFactors(t *testing.T) {
	withFollowTestEnv(t, map[string]string{
		"billing_setting.billing_mode": `{"channel-price-tiered":"tiered_expr"}`,
		"billing_setting.billing_expr": `{"channel-price-tiered":"tier(\"base\", p * 2)"}`,
	})

	ctx := newFollowTestContext(t)
	setChannelPriceSetting(t, ctx, &dto.ChannelPriceSettings{Total: pf(2)})
	requestInput := billingexpr.RequestInput{
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    []byte(`{}`),
	}
	info := &relaycommon.RelayInfo{
		OriginModelName:     "channel-price-tiered",
		UserGroup:           "default",
		UsingGroup:          "default",
		RequestHeaders:      map[string]string{"Content-Type": "application/json"},
		BillingRequestInput: &requestInput,
	}

	priceData, err := ModelPriceHelper(ctx, info, 1000, &types.TokenCountMeta{})
	require.NoError(t, err)
	// 表达式 p*2 在 p=1000 输出 2000 → quotaBeforeGroup=1000 → ×groupRatio(1)×overall(2)
	require.Equal(t, 2000, priceData.QuotaToPreConsume)
	require.NotNil(t, info.TieredBillingSnapshot)
}