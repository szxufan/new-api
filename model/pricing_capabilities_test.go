package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/require"
)

// 模型广场依赖 /api/pricing 下发模型能力信息，这里锁定「已配置返回、未配置省略」的约定。
func TestGetPricingIncludesModelCapabilities(t *testing.T) {
	setupModelPurposeTestDB(t)

	require.NoError(t, DB.Create(&Channel{
		Id:     1,
		Type:   constant.ChannelTypeOpenAI,
		Key:    "test-key",
		Status: common.ChannelStatusEnabled,
		Name:   "capabilities-test-channel",
	}).Error)
	require.NoError(t, DB.Create(&[]Ability{
		{Group: "default", Model: "caps-configured", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "caps-unconfigured", ChannelId: 1, Enabled: true},
	}).Error)
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

	pricing := GetPricing()
	require.Len(t, pricing, 2)
	byName := make(map[string]Pricing, len(pricing))
	for _, item := range pricing {
		byName[item.ModelName] = item
	}

	configured := byName["caps-configured"]
	require.Equal(t, 200000, configured.ContextLength)
	require.Equal(t, 32768, configured.MaxOutputTokens)
	require.True(t, configured.SupportImageInput)
	require.False(t, configured.SupportVideoInput)
	require.Equal(t, []string{"low", "high"}, configured.ReasoningOptions)

	unconfigured := byName["caps-unconfigured"]
	require.Zero(t, unconfigured.ContextLength)
	require.Zero(t, unconfigured.MaxOutputTokens)
	require.False(t, unconfigured.SupportImageInput)
	require.False(t, unconfigured.SupportVideoInput)
	require.Empty(t, unconfigured.ReasoningOptions)
}

func TestPricingCapabilitiesJSON(t *testing.T) {
	capabilityKeys := []string{
		"context_length",
		"max_output_tokens",
		"support_image_input",
		"support_video_input",
		"reasoning_options",
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
	})
}
