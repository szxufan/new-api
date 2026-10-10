package service

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func servicePricePtr(v float64) *float64 {
	return &v
}

// newChannelPriceTestContext 构造带（或不带）渠道价格系数的 gin context。
func newChannelPriceTestContext(t *testing.T, price *dto.ChannelPriceSettings) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	if price != nil {
		common.SetContextKey(ctx, constant.ContextKeyChannelSetting, dto.ChannelSettings{Price: price})
	}
	return ctx
}

func TestCalculateTextQuotaSummaryAppliesChannelPriceFactors(t *testing.T) {
	ctx := newChannelPriceTestContext(t, &dto.ChannelPriceSettings{
		Total:      servicePricePtr(0.5),
		Input:      servicePricePtr(2),
		Completion: servicePricePtr(3),
		CacheRead:  servicePricePtr(4),
		CacheWrite: servicePricePtr(5),
	})

	usage := &dto.Usage{
		PromptTokens:     1000,
		CompletionTokens: 200,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens:         100,
			CachedCreationTokens: 50,
		},
	}
	relayInfo := &relaycommon.RelayInfo{
		RelayFormat:     types.RelayFormatOpenAI,
		OriginModelName: "channel-price-model",
		PriceData: types.PriceData{
			ModelRatio:         1,
			CompletionRatio:    2,
			CacheRatio:         0.1,
			CacheCreationRatio: 1.25,
			GroupRatioInfo:     types.GroupRatioInfo{GroupRatio: 1},
		},
		StartTime: time.Now(),
	}

	summary := calculateTextQuotaSummary(ctx, relayInfo, usage)
	// base = 1000-100-50 = 850，850×2 = 1700
	// cache read = 100×0.1×4 = 40
	// cache write = 50×1.25×5 = 312.5
	// completion = 200×2×3 = 1200
	// 合计 (1700+40+312.5+1200) × modelRatio × groupRatio × overall(0.5) = 1626.25 → 1626
	require.Equal(t, 1626, summary.Quota)
}

func TestCalculateTextQuotaSummaryIdentityFactorsMatchesLegacy(t *testing.T) {
	// 未配置渠道系数时结果与旧逻辑一致
	ctx := newChannelPriceTestContext(t, nil)

	usage := &dto.Usage{
		PromptTokens:     1000,
		CompletionTokens: 200,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens:         100,
			CachedCreationTokens: 50,
		},
	}
	relayInfo := &relaycommon.RelayInfo{
		RelayFormat:     types.RelayFormatOpenAI,
		OriginModelName: "channel-price-model",
		PriceData: types.PriceData{
			ModelRatio:         1,
			CompletionRatio:    2,
			CacheRatio:         0.1,
			CacheCreationRatio: 1.25,
			GroupRatioInfo:     types.GroupRatioInfo{GroupRatio: 1},
		},
		StartTime: time.Now(),
	}

	summary := calculateTextQuotaSummary(ctx, relayInfo, usage)
	// (850 + 100×0.1 + 50×1.25 + 200×2) × 1 × 1 = 1322.5 → 1323
	require.Equal(t, 1323, summary.Quota)
}

func TestCalculateTextQuotaSummaryChannelTimeWindowFactor(t *testing.T) {
	ctx := newChannelPriceTestContext(t, &dto.ChannelPriceSettings{
		TimeWindows: []dto.PriceTimeWindow{
			{TimeWindow: dto.TimeWindow{Start: "22:00", End: "08:00"}, Ratio: 0.25},
		},
	})

	usage := &dto.Usage{PromptTokens: 1000, CompletionTokens: 0, TotalTokens: 1000}
	relayInfo := &relaycommon.RelayInfo{
		RelayFormat:     types.RelayFormatOpenAI,
		OriginModelName: "channel-price-model",
		PriceData: types.PriceData{
			ModelRatio:     1,
			GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1},
		},
		// 23:00 命中跨天时段 22:00-08:00
		StartTime: time.Date(2026, 10, 10, 23, 0, 0, 0, time.Local),
	}

	summary := calculateTextQuotaSummary(ctx, relayInfo, usage)
	// 1000 × overall(0.25) = 250
	require.Equal(t, 250, summary.Quota)
}

func TestCalculateTextQuotaSummaryUsePriceAppliesChannelFactors(t *testing.T) {
	ctx := newChannelPriceTestContext(t, &dto.ChannelPriceSettings{Total: servicePricePtr(2)})

	usage := &dto.Usage{PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150}
	relayInfo := &relaycommon.RelayInfo{
		OriginModelName: "channel-price-price-model",
		PriceData: types.PriceData{
			UsePrice:       true,
			ModelPrice:     0.5,
			GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1},
		},
		StartTime: time.Now(),
	}

	summary := calculateTextQuotaSummary(ctx, relayInfo, usage)
	// 0.5 × QuotaPerUnit × groupRatio(1) × overall(2)
	require.Equal(t, int(0.5*common.QuotaPerUnit*2), summary.Quota)
}

func TestCalculateAudioQuotaAppliesChannelPriceFactors(t *testing.T) {
	info := QuotaInfo{
		InputDetails: TokenDetails{
			TextTokens:  1000,
			AudioTokens: 100,
		},
		OutputDetails: TokenDetails{
			TextTokens:  300,
			AudioTokens: 50,
		},
		ModelName:  "channel-price-audio-model",
		ModelRatio: 1,
		GroupRatio: 1,
	}

	// nil 与全 1 均等价于未配置
	base := calculateAudioQuota(info)
	identity := dto.IdentityPriceFactors()
	info.PriceFactors = &identity
	require.Equal(t, base, calculateAudioQuota(info))
	info.PriceFactors = nil
	require.Equal(t, base, calculateAudioQuota(info))

	// 总系数 × 时间段系数作用于整单
	discount := dto.IdentityPriceFactors()
	discount.Total = 0.5
	info.PriceFactors = &discount
	discounted := calculateAudioQuota(info)
	require.InDelta(t, float64(base)*0.5, float64(discounted), 1.5)
}

func TestTryTieredSettleAppliesChannelPriceFactors(t *testing.T) {
	expr := `tier("base", p * 2)`
	relayInfo := &relaycommon.RelayInfo{
		OriginModelName: "channel-price-tiered-model",
		StartTime:       time.Now(),
		TieredBillingSnapshot: &billingexpr.BillingSnapshot{
			BillingMode:  "tiered_expr",
			ExprString:   expr,
			ExprHash:     billingexpr.ExprHashString(expr),
			GroupRatio:   1,
			QuotaPerUnit: common.QuotaPerUnit,
			ExprVersion:  billingexpr.ExprVersion(expr),
		},
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelSetting: dto.ChannelSettings{
				Price: &dto.ChannelPriceSettings{Total: servicePricePtr(2)},
			},
		},
	}

	ok, quota, result := TryTieredSettle(relayInfo, billingexpr.TokenParams{P: 1000, C: 0, Len: 1000})
	require.True(t, ok)
	// rawCost = 2000 ($/1M) → quotaBeforeGroup = 1000 → ×groupRatio(1)×overall(2) = 2000
	require.Equal(t, 2000, quota)
	require.NotNil(t, result)
	require.Equal(t, 2000, result.ActualQuotaAfterGroup)
}

func TestTryTieredSettleIdentityFactorsMatchesLegacy(t *testing.T) {
	expr := `tier("base", p * 2)`
	relayInfo := &relaycommon.RelayInfo{
		OriginModelName: "channel-price-tiered-model",
		StartTime:       time.Now(),
		TieredBillingSnapshot: &billingexpr.BillingSnapshot{
			BillingMode:  "tiered_expr",
			ExprString:   expr,
			ExprHash:     billingexpr.ExprHashString(expr),
			GroupRatio:   1,
			QuotaPerUnit: common.QuotaPerUnit,
			ExprVersion:  billingexpr.ExprVersion(expr),
		},
	}

	ok, quota, _ := TryTieredSettle(relayInfo, billingexpr.TokenParams{P: 1000, C: 0, Len: 1000})
	require.True(t, ok)
	require.Equal(t, 1000, quota)
}