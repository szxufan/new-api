package helper

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// withFollowTestEnv 备份并替换全局倍率/计费配置，测试结束后恢复。
func withFollowTestEnv(t *testing.T, options map[string]string) {
	t.Helper()

	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		saved[key] = value
		return nil
	}))
	savedModelRatio := ratio_setting.ModelRatio2JSONString()
	savedModelPrice := ratio_setting.ModelPrice2JSONString()
	savedCompletionRatio := ratio_setting.CompletionRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(savedModelRatio))
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(savedModelPrice))
		require.NoError(t, ratio_setting.UpdateCompletionRatioByJSONString(savedCompletionRatio))
	})

	require.NoError(t, config.GlobalConfig.LoadFromDB(options))
}

func newFollowTestContext(t *testing.T) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/api/channel/test/1", nil)
	req.Body = nil
	req.ContentLength = 0
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req
	ctx.Set("group", "default")
	return ctx
}

func TestModelPriceHelperFollowTokenTarget(t *testing.T) {
	withFollowTestEnv(t, map[string]string{
		"billing_setting.billing_follow": `{"follow-token-model":{"target_model":"target-token-model","coefficient":1.5}}`,
	})
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"target-token-model":2}`))
	require.NoError(t, ratio_setting.UpdateCompletionRatioByJSONString(`{"target-token-model":2}`))

	ctx := newFollowTestContext(t)
	info := &relaycommon.RelayInfo{
		OriginModelName: "follow-token-model",
		UserGroup:       "default",
		UsingGroup:      "default",
	}

	priceData, err := ModelPriceHelper(ctx, info, 1000, &types.TokenCountMeta{})
	require.NoError(t, err)
	require.False(t, priceData.UsePrice)
	require.InDelta(t, 3.0, priceData.ModelRatio, 1e-9)
	require.InDelta(t, 2.0, priceData.CompletionRatio, 1e-9)
	require.Nil(t, info.TieredBillingSnapshot)
}

func TestModelPriceHelperFollowPriceTarget(t *testing.T) {
	withFollowTestEnv(t, map[string]string{
		"billing_setting.billing_follow": `{"follow-price-model":{"target_model":"target-price-model","coefficient":2}}`,
	})
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"target-price-model":0.5}`))

	ctx := newFollowTestContext(t)
	info := &relaycommon.RelayInfo{
		OriginModelName: "follow-price-model",
		UserGroup:       "default",
		UsingGroup:      "default",
	}

	priceData, err := ModelPriceHelper(ctx, info, 1000, &types.TokenCountMeta{})
	require.NoError(t, err)
	require.True(t, priceData.UsePrice)
	require.InDelta(t, 1.0, priceData.ModelPrice, 1e-9)
	require.Equal(t, int(1.0*common.QuotaPerUnit), priceData.QuotaToPreConsume)
	require.Nil(t, info.TieredBillingSnapshot)
}

func TestModelPriceHelperFollowTieredTarget(t *testing.T) {
	withFollowTestEnv(t, map[string]string{
		"billing_setting.billing_mode":   `{"tiered-target-model":"tiered_expr"}`,
		"billing_setting.billing_expr":   `{"tiered-target-model":"tier(\"base\", p * 2)"}`,
		"billing_setting.billing_follow": `{"follow-tiered-model":{"target_model":"tiered-target-model","coefficient":2}}`,
	})

	ctx := newFollowTestContext(t)
	requestInput := billingexpr.RequestInput{
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    []byte(`{}`),
	}
	info := &relaycommon.RelayInfo{
		OriginModelName:     "follow-tiered-model",
		UserGroup:           "default",
		UsingGroup:          "default",
		RequestHeaders:      map[string]string{"Content-Type": "application/json"},
		BillingRequestInput: &requestInput,
	}

	priceData, err := ModelPriceHelper(ctx, info, 1000, &types.TokenCountMeta{})
	require.NoError(t, err)
	// 目标表达式 p * 2 在 p=1000 时输出 2000，乘系数 2 后为 4000 ($/1M)
	require.Equal(t, billingexpr.QuotaRound(4000.0/1_000_000*common.QuotaPerUnit), priceData.QuotaToPreConsume)
	require.NotNil(t, info.TieredBillingSnapshot)
	// 结算重放的是包装后的表达式，系数在结算时同样生效
	require.Equal(t, `(tier("base", p * 2)) * 2`, info.TieredBillingSnapshot.ExprString)
	require.Equal(t, "base", info.TieredBillingSnapshot.EstimatedTier)
}

func TestModelPriceHelperFollowUnpricedTarget(t *testing.T) {
	withFollowTestEnv(t, map[string]string{
		"billing_setting.billing_follow": `{"follow-orphan-model":{"target_model":"orphan-target-model","coefficient":1}}`,
	})

	ctx := newFollowTestContext(t)
	info := &relaycommon.RelayInfo{
		OriginModelName: "follow-orphan-model",
		UserGroup:       "default",
		UsingGroup:      "default",
	}

	_, err := ModelPriceHelper(ctx, info, 1000, &types.TokenCountMeta{})
	require.Error(t, err)
}

func TestModelPriceHelperFollowCycleFallsThrough(t *testing.T) {
	withFollowTestEnv(t, map[string]string{
		"billing_setting.billing_follow": `{"cyclic-model":{"target_model":"cyclic-model","coefficient":1}}`,
	})

	ctx := newFollowTestContext(t)
	info := &relaycommon.RelayInfo{
		OriginModelName: "cyclic-model",
		UserGroup:       "default",
		UsingGroup:      "default",
	}

	// 环配置视为未配置跟随，回退到模型自身计费（此处未配置 → 报未配置价格错误）
	_, err := ModelPriceHelper(ctx, info, 1000, &types.TokenCountMeta{})
	require.Error(t, err)
}

func TestHasModelBillingConfigFollow(t *testing.T) {
	withFollowTestEnv(t, map[string]string{
		"billing_setting.billing_follow": `{"follow-known-model":{"target_model":"target-token-model","coefficient":1}}`,
	})
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"target-token-model":2}`))

	require.True(t, HasModelBillingConfig("follow-known-model"))
	require.False(t, HasModelBillingConfig("plain-unpriced-model"))
}
