package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func buildChannelAffinityMatchContextForTest(path string, affinityValue string) *gin.Context {
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, path, nil)
	if affinityValue != "" {
		ctx.Set("affinity_test_key", affinityValue)
	}
	return ctx
}

// withChannelAffinityRules 临时替换全局亲和性配置，测试结束后恢复
func withChannelAffinityRules(t *testing.T, rules []operation_setting.ChannelAffinityRule) {
	t.Helper()
	setting := operation_setting.GetChannelAffinitySetting()
	origEnabled := setting.Enabled
	origRules := setting.Rules
	setting.Enabled = true
	setting.Rules = rules
	t.Cleanup(func() {
		setting.Enabled = origEnabled
		setting.Rules = origRules
	})
}

func boolPtr(b bool) *bool {
	return &b
}

func TestGetPreferredChannelByAffinity_DisabledRuleSkipped(t *testing.T) {
	withChannelAffinityRules(t, []operation_setting.ChannelAffinityRule{
		{
			Name:       "disabled-rule",
			ModelRegex: []string{"^gpt-.*$"},
			PathRegex:  []string{"/v1/responses"},
			KeySources: []operation_setting.ChannelAffinityKeySource{
				{Type: "context_string", Key: "affinity_test_key"},
			},
			Enabled: boolPtr(false),
		},
	})

	ctx := buildChannelAffinityMatchContextForTest("/v1/responses", "conv-123")
	channelID, found := GetPreferredChannelByAffinity(ctx, "gpt-4o", "default")
	require.False(t, found)
	require.Zero(t, channelID)
	require.False(t, IsChannelAffinityMatched(ctx))
}

func TestGetPreferredChannelByAffinity_NilEnabledTreatedAsEnabled(t *testing.T) {
	withChannelAffinityRules(t, []operation_setting.ChannelAffinityRule{
		{
			Name:       "legacy-rule",
			ModelRegex: []string{"^gpt-.*$"},
			PathRegex:  []string{"/v1/responses"},
			KeySources: []operation_setting.ChannelAffinityKeySource{
				{Type: "context_string", Key: "affinity_test_key"},
			},
		},
	})

	ctx := buildChannelAffinityMatchContextForTest("/v1/responses", "conv-123")
	_, _ = GetPreferredChannelByAffinity(ctx, "gpt-4o", "default")
	require.True(t, IsChannelAffinityMatched(ctx))
	meta, ok := getChannelAffinityMeta(ctx)
	require.True(t, ok)
	require.Equal(t, "legacy-rule", meta.RuleName)
}

func TestGetPreferredChannelByAffinity_ExplicitEnabled(t *testing.T) {
	withChannelAffinityRules(t, []operation_setting.ChannelAffinityRule{
		{
			Name:       "enabled-rule",
			ModelRegex: []string{"^gpt-.*$"},
			PathRegex:  []string{"/v1/responses"},
			KeySources: []operation_setting.ChannelAffinityKeySource{
				{Type: "context_string", Key: "affinity_test_key"},
			},
			Enabled: boolPtr(true),
		},
	})

	ctx := buildChannelAffinityMatchContextForTest("/v1/responses", "conv-123")
	_, _ = GetPreferredChannelByAffinity(ctx, "gpt-4o", "default")
	require.True(t, IsChannelAffinityMatched(ctx))
}

func TestGetPreferredChannelByAffinity_ModelRegexExclude(t *testing.T) {
	withChannelAffinityRules(t, []operation_setting.ChannelAffinityRule{
		{
			Name:              "exclude-image-model",
			ModelRegex:        []string{"^gpt-.*$"},
			ModelRegexExclude: []string{"^gpt-image-.*$"},
			KeySources: []operation_setting.ChannelAffinityKeySource{
				{Type: "context_string", Key: "affinity_test_key"},
			},
		},
	})

	// 命中排除正则：跳过
	ctx := buildChannelAffinityMatchContextForTest("/v1/images/generations", "conv-123")
	_, _ = GetPreferredChannelByAffinity(ctx, "gpt-image-1", "default")
	require.False(t, IsChannelAffinityMatched(ctx))

	// 未命中排除正则：正常匹配
	ctx2 := buildChannelAffinityMatchContextForTest("/v1/responses", "conv-123")
	_, _ = GetPreferredChannelByAffinity(ctx2, "gpt-4o", "default")
	require.True(t, IsChannelAffinityMatched(ctx2))
}

func TestGetPreferredChannelByAffinity_PathRegexExclude(t *testing.T) {
	withChannelAffinityRules(t, []operation_setting.ChannelAffinityRule{
		{
			Name:             "exclude-media-paths",
			ModelRegex:       []string{"^gpt-.*$"},
			PathRegexExclude: []string{"/v1/images", "/v1/videos"},
			KeySources: []operation_setting.ChannelAffinityKeySource{
				{Type: "context_string", Key: "affinity_test_key"},
			},
		},
	})

	// 生图路径被排除（前缀子串匹配）
	ctx := buildChannelAffinityMatchContextForTest("/v1/images/generations", "conv-123")
	_, _ = GetPreferredChannelByAffinity(ctx, "gpt-4o", "default")
	require.False(t, IsChannelAffinityMatched(ctx))

	// 生视频路径被排除
	ctx2 := buildChannelAffinityMatchContextForTest("/v1/videos", "conv-123")
	_, _ = GetPreferredChannelByAffinity(ctx2, "gpt-4o", "default")
	require.False(t, IsChannelAffinityMatched(ctx2))

	// 普通路径不受影响
	ctx3 := buildChannelAffinityMatchContextForTest("/v1/responses", "conv-123")
	_, _ = GetPreferredChannelByAffinity(ctx3, "gpt-4o", "default")
	require.True(t, IsChannelAffinityMatched(ctx3))
}

func TestGetPreferredChannelByAffinity_ExcludePrioritizedOverInclude(t *testing.T) {
	withChannelAffinityRules(t, []operation_setting.ChannelAffinityRule{
		{
			Name:              "conflict-rule",
			ModelRegex:        []string{"^gpt-.*$"},
			ModelRegexExclude: []string{"^gpt-4o$"},
			PathRegex:         []string{"/v1/responses"},
			PathRegexExclude:  []string{"/v1/responses"},
			KeySources: []operation_setting.ChannelAffinityKeySource{
				{Type: "context_string", Key: "affinity_test_key"},
			},
		},
	})

	ctx := buildChannelAffinityMatchContextForTest("/v1/responses", "conv-123")
	_, _ = GetPreferredChannelByAffinity(ctx, "gpt-4o", "default")
	require.False(t, IsChannelAffinityMatched(ctx))
}

func TestGetPreferredChannelByAffinity_DisabledThenEnabledFallback(t *testing.T) {
	// 第一条被禁用时应继续尝试后续规则
	withChannelAffinityRules(t, []operation_setting.ChannelAffinityRule{
		{
			Name:       "first-disabled",
			ModelRegex: []string{"^gpt-.*$"},
			KeySources: []operation_setting.ChannelAffinityKeySource{
				{Type: "context_string", Key: "affinity_test_key"},
			},
			Enabled: boolPtr(false),
		},
		{
			Name:       "second-enabled",
			ModelRegex: []string{"^gpt-.*$"},
			KeySources: []operation_setting.ChannelAffinityKeySource{
				{Type: "context_string", Key: "affinity_test_key"},
			},
		},
	})

	ctx := buildChannelAffinityMatchContextForTest("/v1/responses", "conv-123")
	_, _ = GetPreferredChannelByAffinity(ctx, "gpt-4o", "default")
	require.True(t, IsChannelAffinityMatched(ctx))
	meta, ok := getChannelAffinityMeta(ctx)
	require.True(t, ok)
	require.Equal(t, "second-enabled", meta.RuleName)
}
