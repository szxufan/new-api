package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/stretchr/testify/require"
)

// withBillingFollow 在测试期间写入跟随计费配置，结束后恢复全部配置。
func withBillingFollow(t *testing.T, follows map[string]billing_setting.FollowConfig) {
	t.Helper()

	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		saved[key] = value
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
	})

	followBytes, err := common.Marshal(follows)
	require.NoError(t, err)
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting." + billing_setting.BillingFollowField: string(followBytes),
	}))
}

func pricingByName(t *testing.T) map[string]Pricing {
	t.Helper()

	pricing := GetPricing()
	byName := make(map[string]Pricing, len(pricing))
	for _, item := range pricing {
		byName[item.ModelName] = item
	}
	return byName
}

func seedCapabilitiesChannel(t *testing.T, modelNames ...string) {
	t.Helper()

	require.NoError(t, DB.Create(&Channel{
		Id:     1,
		Type:   constant.ChannelTypeOpenAI,
		Key:    "test-key",
		Status: common.ChannelStatusEnabled,
		Name:   "capabilities-test-channel",
	}).Error)

	abilities := make([]Ability, 0, len(modelNames))
	for _, name := range modelNames {
		abilities = append(abilities, Ability{Group: "default", Model: name, ChannelId: 1, Enabled: true})
	}
	require.NoError(t, DB.Create(&abilities).Error)
}

// 模型广场依赖 /api/pricing 下发模型能力信息，这里锁定「已配置返回、未配置省略」的约定。
func TestGetPricingIncludesModelCapabilities(t *testing.T) {
	setupModelPurposeTestDB(t)

	seedCapabilitiesChannel(t, "caps-configured", "caps-unconfigured")
	require.NoError(t, DB.Create(&[]Model{
		{
			ModelName:         "caps-configured",
			Status:            1,
			ContextLength:     200000,
			MaxOutputTokens:   32768,
			SupportImageInput: true,
			ReasoningOptions:  `["low","high"]`,
		},
		{ModelName: "caps-unconfigured", Status: 1},
	}).Error)

	RefreshPricing()
	byName := pricingByName(t)
	require.Len(t, byName, 2)

	configured := byName["caps-configured"]
	require.Equal(t, 200000, configured.ContextLength)
	require.Equal(t, 32768, configured.MaxOutputTokens)
	require.True(t, configured.SupportImageInput)
	require.False(t, configured.SupportVideoInput)
	require.Equal(t, []string{"low", "high"}, configured.ReasoningOptions)
	require.Empty(t, configured.CapabilitySource)

	unconfigured := byName["caps-unconfigured"]
	require.Zero(t, unconfigured.ContextLength)
	require.Zero(t, unconfigured.MaxOutputTokens)
	require.False(t, unconfigured.SupportImageInput)
	require.False(t, unconfigured.SupportVideoInput)
	require.Empty(t, unconfigured.ReasoningOptions)
	require.Empty(t, unconfigured.CapabilitySource)
}

// 跟随模型自身未配置能力信息时，沿用跟随目标的能力信息并标记来源。
func TestGetPricingFollowModelInheritsCapabilities(t *testing.T) {
	setupModelPurposeTestDB(t)
	withBillingFollow(t, map[string]billing_setting.FollowConfig{
		"caps-follow-alias":   {TargetModel: "caps-follow-configured"},
		"caps-follow-own":     {TargetModel: "caps-follow-configured"},
		"caps-follow-empty":   {TargetModel: "caps-follow-unconfigured"},
		"caps-follow-chain-a": {TargetModel: "caps-follow-chain-b"},
		"caps-follow-chain-b": {TargetModel: "caps-follow-chain-c"},
	})

	seedCapabilitiesChannel(t,
		"caps-follow-configured",
		"caps-follow-unconfigured",
		"caps-follow-alias",
		"caps-follow-own",
		"caps-follow-empty",
		"caps-follow-chain-a",
		"caps-follow-chain-b",
		"caps-follow-chain-c",
	)
	require.NoError(t, DB.Create(&[]Model{
		{
			ModelName:         "caps-follow-configured",
			Status:            1,
			ContextLength:     200000,
			MaxOutputTokens:   32768,
			SupportImageInput: true,
			ReasoningOptions:  `["low","high"]`,
		},
		{ModelName: "caps-follow-unconfigured", Status: 1},
		{ModelName: "caps-follow-own", Status: 1, ContextLength: 32000},
		{
			ModelName:         "caps-follow-chain-c",
			Status:            1,
			ContextLength:     1000000,
			SupportVideoInput: true,
		},
	}).Error)

	RefreshPricing()
	byName := pricingByName(t)
	require.Len(t, byName, 8)

	// 自身未配置：完整沿用跟随目标
	alias := byName["caps-follow-alias"]
	require.Equal(t, "caps-follow-configured", alias.FollowTarget)
	require.Equal(t, 200000, alias.ContextLength)
	require.Equal(t, 32768, alias.MaxOutputTokens)
	require.True(t, alias.SupportImageInput)
	require.False(t, alias.SupportVideoInput)
	require.Equal(t, []string{"low", "high"}, alias.ReasoningOptions)
	require.Equal(t, CapabilitySourceFollow, alias.CapabilitySource)

	// 自身配置过：以自身为准，不继承
	own := byName["caps-follow-own"]
	require.Equal(t, "caps-follow-configured", own.FollowTarget)
	require.Equal(t, 32000, own.ContextLength)
	require.Zero(t, own.MaxOutputTokens)
	require.False(t, own.SupportImageInput)
	require.Empty(t, own.ReasoningOptions)
	require.Empty(t, own.CapabilitySource)

	// 跟随目标也未配置：保持未配置
	empty := byName["caps-follow-empty"]
	require.Equal(t, "caps-follow-unconfigured", empty.FollowTarget)
	require.Zero(t, empty.ContextLength)
	require.Empty(t, empty.CapabilitySource)

	// 链式跟随：与价格一样取跟随链终点
	chainA := byName["caps-follow-chain-a"]
	require.Equal(t, "caps-follow-chain-c", chainA.FollowTarget)
	require.Equal(t, 1000000, chainA.ContextLength)
	require.True(t, chainA.SupportVideoInput)
	require.Equal(t, CapabilitySourceFollow, chainA.CapabilitySource)

	// 被跟随的目标本身保持自身配置，不带来源标记
	target := byName["caps-follow-configured"]
	require.Equal(t, 200000, target.ContextLength)
	require.Empty(t, target.CapabilitySource)
}

func TestPricingCapabilitiesJSON(t *testing.T) {
	capabilityKeys := []string{
		"context_length",
		"max_output_tokens",
		"support_image_input",
		"support_video_input",
		"reasoning_options",
		"capability_source",
	}

	t.Run("omitted when unset", func(t *testing.T) {
		data, err := common.Marshal(Pricing{ModelName: "m"})
		require.NoError(t, err)

		var payload map[string]any
		require.NoError(t, common.Unmarshal(data, &payload))
		for _, key := range capabilityKeys {
			require.NotContains(t, payload, key)
		}
	})

	t.Run("present when configured", func(t *testing.T) {
		data, err := common.Marshal(Pricing{
			ContextLength:     131072,
			MaxOutputTokens:   8192,
			SupportVideoInput: true,
			ReasoningOptions:  []string{"high"},
			CapabilitySource:  CapabilitySourceFollow,
		})
		require.NoError(t, err)

		var payload map[string]any
		require.NoError(t, common.Unmarshal(data, &payload))
		require.Equal(t, float64(131072), payload["context_length"])
		require.Equal(t, float64(8192), payload["max_output_tokens"])
		require.Equal(t, true, payload["support_video_input"])
		// false 与未配置同形，前端据此展示为「未设置」
		require.NotContains(t, payload, "support_image_input")
		require.Equal(t, []any{"high"}, payload["reasoning_options"])
		require.Equal(t, CapabilitySourceFollow, payload["capability_source"])
	})
}
