package service

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// withChannelAffinityProbeSetting 临时修改全局亲和性配置，测试结束后恢复
func withChannelAffinityProbeSetting(t *testing.T, mutate func(*operation_setting.ChannelAffinitySetting)) {
	t.Helper()
	setting := operation_setting.GetChannelAffinitySetting()
	orig := *setting
	mutate(setting)
	t.Cleanup(func() {
		*setting = orig
	})
}

// stubProbeChannelSources 替换回探的渠道列表与 auto 分组来源，测试结束后恢复
func stubProbeChannelSources(t *testing.T, groupChannels map[string][]*model.Channel, autoGroups []string) {
	t.Helper()
	origSorted := getSortedChannelsForProbe
	origAuto := getUserAutoGroupsForProbe
	getSortedChannelsForProbe = func(group, modelName string, preferredTypes []int, affinityCounts map[int]int64) []*model.Channel {
		return groupChannels[group]
	}
	getUserAutoGroupsForProbe = func(userGroup string) []string {
		return autoGroups
	}
	t.Cleanup(func() {
		getSortedChannelsForProbe = origSorted
		getUserAutoGroupsForProbe = origAuto
	})
}

func probeTestChannel(id int, priority int64) *model.Channel {
	p := priority
	return &model.Channel{Id: id, Priority: &p}
}

func probeTestChannelWithBlacklist(id int, priority int64, blacklist string) *model.Channel {
	ch := probeTestChannel(id, priority)
	ch.GroupBlacklist = &blacklist
	return ch
}

func buildProbeContextForTest(cacheKey string, skipRetry bool) *gin.Context {
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	setChannelAffinityContext(ctx, channelAffinityMeta{
		CacheKey:    cacheKey,
		TTLSeconds:  3600,
		RuleName:    "probe-test-rule",
		SkipRetry:   skipRetry,
		UsingGroup:  "default",
		ModelName:   "gpt-4o",
		RequestPath: "/v1/chat/completions",
	})
	return ctx
}

// probeTestCacheKey 为每个测试生成独立的亲和键，避免冷却标记在测试间互相污染
func probeTestCacheKey(t *testing.T) string {
	return channelAffinityCacheNamespace + ":probe-test:" + t.Name()
}

func TestShouldProbeHigherPriorityChannel_SettingDisabled(t *testing.T) {
	withChannelAffinityProbeSetting(t, func(s *operation_setting.ChannelAffinitySetting) {
		s.Enabled = false
		s.ProbeEnabled = true
		s.ProbePercent = 100
	})
	stubProbeChannelSources(t, map[string][]*model.Channel{
		"default": {probeTestChannel(2, 100)},
	}, nil)

	ctx := buildProbeContextForTest(probeTestCacheKey(t), false)
	require.False(t, ShouldProbeHigherPriorityChannel(ctx, probeTestChannel(1, 10), "gpt-4o", "default", "default"))
}

func TestShouldProbeHigherPriorityChannel_ProbeDisabled(t *testing.T) {
	withChannelAffinityProbeSetting(t, func(s *operation_setting.ChannelAffinitySetting) {
		s.Enabled = true
		s.ProbeEnabled = false
		s.ProbePercent = 100
	})
	stubProbeChannelSources(t, map[string][]*model.Channel{
		"default": {probeTestChannel(2, 100)},
	}, nil)

	ctx := buildProbeContextForTest(probeTestCacheKey(t), false)
	require.False(t, ShouldProbeHigherPriorityChannel(ctx, probeTestChannel(1, 10), "gpt-4o", "default", "default"))
}

func TestShouldProbeHigherPriorityChannel_SkipRetryRuleSkipped(t *testing.T) {
	withChannelAffinityProbeSetting(t, func(s *operation_setting.ChannelAffinitySetting) {
		s.Enabled = true
		s.ProbeEnabled = true
		s.ProbePercent = 100
	})
	stubProbeChannelSources(t, map[string][]*model.Channel{
		"default": {probeTestChannel(2, 100)},
	}, nil)

	// skip_retry_on_failure 的规则要求严格钉住渠道，回探失败无法回退，因此不回探
	ctx := buildProbeContextForTest(probeTestCacheKey(t), true)
	require.False(t, ShouldProbeHigherPriorityChannel(ctx, probeTestChannel(1, 10), "gpt-4o", "default", "default"))
}

func TestShouldProbeHigherPriorityChannel_NoHigherPriorityChannel(t *testing.T) {
	withChannelAffinityProbeSetting(t, func(s *operation_setting.ChannelAffinitySetting) {
		s.Enabled = true
		s.ProbeEnabled = true
		s.ProbePercent = 100
	})
	// 最高优先级渠道就是亲和渠道本身（或更低），无需回探
	stubProbeChannelSources(t, map[string][]*model.Channel{
		"default": {probeTestChannel(1, 10), probeTestChannel(2, 5)},
	}, nil)

	ctx := buildProbeContextForTest(probeTestCacheKey(t), false)
	require.False(t, ShouldProbeHigherPriorityChannel(ctx, probeTestChannel(1, 10), "gpt-4o", "default", "default"))
}

func TestShouldProbeHigherPriorityChannel_ZeroPercent(t *testing.T) {
	withChannelAffinityProbeSetting(t, func(s *operation_setting.ChannelAffinitySetting) {
		s.Enabled = true
		s.ProbeEnabled = true
		s.ProbePercent = 0
	})
	stubProbeChannelSources(t, map[string][]*model.Channel{
		"default": {probeTestChannel(2, 100)},
	}, nil)

	ctx := buildProbeContextForTest(probeTestCacheKey(t), false)
	require.False(t, ShouldProbeHigherPriorityChannel(ctx, probeTestChannel(1, 10), "gpt-4o", "default", "default"))
}

func TestShouldProbeHigherPriorityChannel_ProbeAndCooldown(t *testing.T) {
	withChannelAffinityProbeSetting(t, func(s *operation_setting.ChannelAffinitySetting) {
		s.Enabled = true
		s.ProbeEnabled = true
		s.ProbePercent = 100
		s.ProbeCooldownSeconds = 300
	})
	stubProbeChannelSources(t, map[string][]*model.Channel{
		"default": {probeTestChannel(2, 100), probeTestChannel(1, 10)},
	}, nil)

	cacheKey := probeTestCacheKey(t)
	preferred := probeTestChannel(1, 10)

	// 第一次：存在更高优先级渠道且不在冷却窗口内，触发回探
	ctx1 := buildProbeContextForTest(cacheKey, false)
	require.True(t, ShouldProbeHigherPriorityChannel(ctx1, preferred, "gpt-4o", "default", "default"))

	// 回探事件写入管理日志信息
	anyInfo, ok := ctx1.Get(ginKeyChannelAffinityLogInfo)
	require.True(t, ok)
	info, ok := anyInfo.(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, true, info["probe"])
	require.Equal(t, "probe_higher_priority", info["reason"])
	require.Equal(t, 1, info["channel_id"])

	// 第二次（同一亲和键）：冷却窗口内，不再回探
	ctx2 := buildProbeContextForTest(cacheKey, false)
	require.False(t, ShouldProbeHigherPriorityChannel(ctx2, preferred, "gpt-4o", "default", "default"))
}

func TestShouldProbeHigherPriorityChannel_AutoGroupHigherPriority(t *testing.T) {
	withChannelAffinityProbeSetting(t, func(s *operation_setting.ChannelAffinitySetting) {
		s.Enabled = true
		s.ProbeEnabled = true
		s.ProbePercent = 100
		s.ProbeCooldownSeconds = 300
	})
	// auto 模式：亲和渠道所在分组无更高优先级，但另一分组存在更高优先级渠道
	stubProbeChannelSources(t, map[string][]*model.Channel{
		"group-a": {probeTestChannel(1, 10)},
		"group-b": {probeTestChannel(3, 100)},
	}, []string{"group-a", "group-b"})

	ctx := buildProbeContextForTest(probeTestCacheKey(t), false)
	require.True(t, ShouldProbeHigherPriorityChannel(ctx, probeTestChannel(1, 10), "gpt-4o", "auto", "default"))
}

func TestFindTopAvailableChannelPriority_AggregatesAutoGroups(t *testing.T) {
	stubProbeChannelSources(t, map[string][]*model.Channel{
		"group-a": {probeTestChannel(1, 50)},
		"group-b": {probeTestChannel(2, 100), probeTestChannel(3, 20)},
		"group-c": nil,
	}, []string{"group-a", "group-b", "group-c"})

	top, ok := findTopAvailableChannelPriority("gpt-4o", "auto", "default")
	require.True(t, ok)
	require.Equal(t, int64(100), top)
}

func TestFindTopAvailableChannelPriority_NoChannel(t *testing.T) {
	stubProbeChannelSources(t, map[string][]*model.Channel{}, nil)

	_, ok := findTopAvailableChannelPriority("gpt-4o", "default", "default")
	require.False(t, ok)
}

func TestTopPriorityInGroup_BlacklistedChannelSkipped(t *testing.T) {
	stubProbeChannelSources(t, map[string][]*model.Channel{
		"default": {
			probeTestChannelWithBlacklist(1, 100, "vip"),
			probeTestChannel(2, 50),
		},
	}, nil)

	// 最高优先级渠道拉黑了用户分组 vip，应取下一个可用渠道的优先级
	top, ok := topPriorityInGroup("default", "gpt-4o", "vip")
	require.True(t, ok)
	require.Equal(t, int64(50), top)

	// 未被拉黑的用户分组仍取最高优先级
	top, ok = topPriorityInGroup("default", "gpt-4o", "default")
	require.True(t, ok)
	require.Equal(t, int64(100), top)
}

func TestMarkChannelAffinityProbeCooldown_RepeatKeyBlocked(t *testing.T) {
	key := fmt.Sprintf("%s:cooldown-test:%s", channelAffinityCacheNamespace, t.Name())

	ctx1 := buildProbeContextForTest(key, false)
	require.True(t, markChannelAffinityProbeCooldown(ctx1, 300))

	ctx2 := buildProbeContextForTest(key, false)
	require.False(t, markChannelAffinityProbeCooldown(ctx2, 300))
}

func TestMarkChannelAffinityProbeCooldown_NoCacheKey(t *testing.T) {
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	require.False(t, markChannelAffinityProbeCooldown(ctx, 300))
}
