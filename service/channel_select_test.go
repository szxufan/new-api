package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// newTestRetryParam 构造一个带预排序渠道序列的 RetryParam（用于顺序消费测试）。
func newTestRetryParam(channels []*model.Channel, perChannelAttempts int, retry int) *RetryParam {
	r := retry
	return &RetryParam{
		TokenGroup:         "default",
		ModelName:          "m",
		Retry:              &r,
		channelSequence:    channels,
		perChannelAttempts: perChannelAttempts,
	}
}

// TestCalcPerChannelAttempts 验证每个渠道尝试次数的计算规则。
func TestCalcPerChannelAttempts(t *testing.T) {
	cases := []struct {
		retryTimes int
		expected   int
	}{
		{-1, 1}, // 负数视为不重试
		{0, 1},  // 不重试，只尝试一次
		{1, 1},  // ceil(1/10)=1
		{9, 1},  // ceil(9/10)=1
		{10, 1}, // ceil(10/10)=1
		{11, 2}, // ceil(11/10)=2
		{15, 2}, // ceil(15/10)=2
		{20, 2}, // ceil(20/10)=2
		{50, 5}, // ceil(50/10)=5
		{100, 10},
	}
	for _, c := range cases {
		got := calcPerChannelAttempts(c.retryTimes)
		if got != c.expected {
			t.Errorf("calcPerChannelAttempts(%d) = %d, expected %d", c.retryTimes, got, c.expected)
		}
	}
}

// TestGetChannelFromSequence_Sequential 验证按顺序消费渠道。
// perChannelAttempts=2 时，retry 0/1 取第 0 个，retry 2/3 取第 1 个，以此类推。
func TestGetChannelFromSequence_Sequential(t *testing.T) {
	ch1 := &model.Channel{Id: 1, Status: common.ChannelStatusEnabled}
	ch2 := &model.Channel{Id: 2, Status: common.ChannelStatusEnabled}
	ch3 := &model.Channel{Id: 3, Status: common.ChannelStatusEnabled}
	channels := []*model.Channel{ch1, ch2, ch3}

	cases := []struct {
		retry    int
		expected int // 期望的渠道 Id，0 表示 nil
	}{
		{0, 1},
		{1, 1},
		{2, 2},
		{3, 2},
		{4, 3},
		{5, 3},
		{6, 0}, // 超出范围
		{100, 0},
	}
	for _, c := range cases {
		param := newTestRetryParam(channels, 2, c.retry)
		ch := getChannelFromSequence(param)
		if c.expected == 0 {
			if ch != nil {
				t.Errorf("retry=%d: expected nil, got channel #%d", c.retry, ch.Id)
			}
		} else {
			if ch == nil || ch.Id != c.expected {
				t.Errorf("retry=%d: expected channel #%d, got %v", c.retry, c.expected, ch)
			}
		}
	}
}

// TestGetChannelFromSequence_EmptySequence 验证空序列返回 nil。
func TestGetChannelFromSequence_EmptySequence(t *testing.T) {
	param := newTestRetryParam(nil, 1, 0)
	if ch := getChannelFromSequence(param); ch != nil {
		t.Errorf("expected nil for empty sequence, got %v", ch)
	}
}

// TestGetChannelFromSequence_PerChannelAttemptsOne 验证 perChannelAttempts=1 时每次重试都换渠道。
func TestGetChannelFromSequence_PerChannelAttemptsOne(t *testing.T) {
	ch1 := &model.Channel{Id: 1, Status: common.ChannelStatusEnabled}
	ch2 := &model.Channel{Id: 2, Status: common.ChannelStatusEnabled}
	channels := []*model.Channel{ch1, ch2}

	param := newTestRetryParam(channels, 1, 0)
	if ch := getChannelFromSequence(param); ch.Id != 1 {
		t.Errorf("retry=0: expected #1, got #%d", ch.Id)
	}
	param.SetRetry(1)
	if ch := getChannelFromSequence(param); ch.Id != 2 {
		t.Errorf("retry=1: expected #2, got #%d", ch.Id)
	}
	param.SetRetry(2)
	if ch := getChannelFromSequence(param); ch != nil {
		t.Errorf("retry=2: expected nil, got #%d", ch.Id)
	}
}

// TestGetChannelFromSequence_ReturnByIndex 验证 getChannelFromSequence 按 index 返回渠道（不检查状态）
// 状态检查已移至外层循环（Relay 函数的 channel.Status != Enabled 判断）
func TestGetChannelFromSequence_ReturnByIndex(t *testing.T) {
	ch1 := &model.Channel{Id: 1, Status: common.ChannelStatusAutoDisabled}
	ch2 := &model.Channel{Id: 2, Status: common.ChannelStatusEnabled}
	channels := []*model.Channel{ch1, ch2}

	param := newTestRetryParam(channels, 1, 0)
	// retry=0: 返回 index=0 对应的 ch1（不跳过禁用渠道）
	if ch := getChannelFromSequence(param); ch == nil || ch.Id != 1 {
		t.Errorf("retry=0: expected #1 (no status check), got %v", ch)
	}
	// retry=1: 返回 index=1 对应的 ch2
	param.SetRetry(1)
	if ch := getChannelFromSequence(param); ch == nil || ch.Id != 2 {
		t.Errorf("retry=1: expected #2, got %v", ch)
	}
}

// TestDetermineSelectGroup 验证根据 channelIndex 从 groupRanges 中定位分组。
func TestDetermineSelectGroup(t *testing.T) {
	ch1 := &model.Channel{Id: 1, Status: common.ChannelStatusEnabled}
	ch2 := &model.Channel{Id: 2, Status: common.ChannelStatusEnabled}
	ch3 := &model.Channel{Id: 3, Status: common.ChannelStatusEnabled}
	ch4 := &model.Channel{Id: 4, Status: common.ChannelStatusEnabled}
	channels := []*model.Channel{ch1, ch2, ch3, ch4}

	param := newTestRetryParam(channels, 1, 0)
	param.TokenGroup = "auto"
	param.groupRanges = []GroupRange{
		{Group: "groupA", Start: 0, End: 2},
		{Group: "groupB", Start: 2, End: 4},
	}

	cases := []struct {
		retry    int
		expected string
	}{
		{0, "groupA"},
		{1, "groupA"},
		{2, "groupB"},
		{3, "groupB"},
		{4, ""}, // 超出范围
	}
	for _, c := range cases {
		param.SetRetry(c.retry)
		got := determineSelectGroup(param)
		if got != c.expected {
			t.Errorf("retry=%d: expected group %q, got %q", c.retry, c.expected, got)
		}
	}
}

// TestDetermineSelectGroup_EmptyRanges 验证无 groupRanges 时返回空字符串。
func TestDetermineSelectGroup_EmptyRanges(t *testing.T) {
	param := newTestRetryParam([]*model.Channel{{Id: 1, Status: common.ChannelStatusEnabled}}, 1, 0)
	if got := determineSelectGroup(param); got != "" {
		t.Errorf("expected empty string for no groupRanges, got %q", got)
	}
}

// TestUpdateAutoGroupContext_NoSwitch 验证 channelIndex 在当前分组范围内时不切换分组。
func TestUpdateAutoGroupContext_NoSwitch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(nil)
	ch1 := &model.Channel{Id: 1, Status: common.ChannelStatusEnabled}
	ch2 := &model.Channel{Id: 2, Status: common.ChannelStatusEnabled}
	param := newTestRetryParam([]*model.Channel{ch1, ch2}, 1, 0)
	param.TokenGroup = "auto"
	param.Ctx = ctx
	param.groupRanges = []GroupRange{
		{Group: "groupA", Start: 0, End: 2},
	}
	common.SetContextKey(ctx, constant.ContextKeyAutoGroupIndex, 99)

	// retry=0，channelIndex=0 < End=2，不应切换
	updateAutoGroupContext(param, 0, true)
	got := common.GetContextKeyInt(ctx, constant.ContextKeyAutoGroupIndex)
	if got != 0 {
		t.Errorf("expected AutoGroupIndex=0 (current), got %d", got)
	}
}

// TestUpdateAutoGroupContext_CrossGroupSwitch 验证跨分组时更新 context 状态。
func TestUpdateAutoGroupContext_CrossGroupSwitch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(nil)
	ch1 := &model.Channel{Id: 1, Status: common.ChannelStatusEnabled}
	ch2 := &model.Channel{Id: 2, Status: common.ChannelStatusEnabled}
	ch3 := &model.Channel{Id: 3, Status: common.ChannelStatusEnabled}
	param := newTestRetryParam([]*model.Channel{ch1, ch2, ch3}, 1, 2)
	param.TokenGroup = "auto"
	param.Ctx = ctx
	param.groupRanges = []GroupRange{
		{Group: "groupA", Start: 0, End: 2},
		{Group: "groupB", Start: 2, End: 3},
	}
	common.SetContextKey(ctx, constant.ContextKeyTokenCrossGroupRetry, true)

	// retry=2，channelIndex=2 >= End=2，应切换到 groupB（index=1）
	updateAutoGroupContext(param, 0, true)
	got := common.GetContextKeyInt(ctx, constant.ContextKeyAutoGroupIndex)
	if got != 1 {
		t.Errorf("expected AutoGroupIndex=1 (switched to groupB), got %d", got)
	}
	// retry 应被重置为 0
	if param.GetRetry() != 0 {
		t.Errorf("expected retry reset to 0, got %d", param.GetRetry())
	}
	// resetNextTry 应为 true（下次 IncreaseRetry 不递增）
	if !param.resetNextTry {
		t.Errorf("expected resetNextTry=true after cross-group switch")
	}
}

// TestUpdateAutoGroupContext_NoCrossGroupRetry 验证未启用跨分组重试时不切换。
func TestUpdateAutoGroupContext_NoCrossGroupRetry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(nil)
	ch1 := &model.Channel{Id: 1, Status: common.ChannelStatusEnabled}
	ch2 := &model.Channel{Id: 2, Status: common.ChannelStatusEnabled}
	ch3 := &model.Channel{Id: 3, Status: common.ChannelStatusEnabled}
	param := newTestRetryParam([]*model.Channel{ch1, ch2, ch3}, 1, 2)
	param.TokenGroup = "auto"
	param.Ctx = ctx
	param.groupRanges = []GroupRange{
		{Group: "groupA", Start: 0, End: 2},
		{Group: "groupB", Start: 2, End: 3},
	}
	// crossGroupRetry=false
	updateAutoGroupContext(param, 0, false)
	// 未启用跨分组重试，不应切换分组索引
	got := common.GetContextKeyInt(ctx, constant.ContextKeyAutoGroupIndex)
	if got != 0 {
		t.Errorf("expected AutoGroupIndex unchanged=0 when crossGroupRetry disabled, got %d", got)
	}
}

// TestRetryParamIncreaseResetNextTry 验证 ResetRetryNextTry 后 IncreaseRetry 不递增。
func TestRetryParamIncreaseResetNextTry(t *testing.T) {
	param := &RetryParam{}
	param.SetRetry(5)
	param.IncreaseRetry()
	if param.GetRetry() != 6 {
		t.Fatalf("expected retry=6, got %d", param.GetRetry())
	}

	// 标记下次重试重置，IncreaseRetry 应跳过本次递增
	param.ResetRetryNextTry()
	param.IncreaseRetry()
	if param.GetRetry() != 6 {
		t.Errorf("expected retry stay 6 after ResetRetryNextTry, got %d", param.GetRetry())
	}

	// 之后的 IncreaseRetry 恢复正常递增
	param.IncreaseRetry()
	if param.GetRetry() != 7 {
		t.Errorf("expected retry=7 after normal IncreaseRetry, got %d", param.GetRetry())
	}
}

// TestFilterGroupBlacklistedChannels 验证按用户自身分组过滤黑名单渠道。
func TestFilterGroupBlacklistedChannels(t *testing.T) {
	blacklist := "vip,internal"
	ch1 := &model.Channel{Id: 1}                              // 无黑名单
	ch2 := &model.Channel{Id: 2, GroupBlacklist: &blacklist}  // 拉黑 vip/internal
	ch3 := &model.Channel{Id: 3, GroupBlacklist: new(string)} // 空黑名单
	channels := []*model.Channel{ch1, ch2, ch3}

	// vip 用户：ch2 被过滤
	got := filterGroupBlacklistedChannels(nil, channels, "vip")
	if len(got) != 2 || got[0].Id != 1 || got[1].Id != 3 {
		ids := make([]int, 0, len(got))
		for _, ch := range got {
			ids = append(ids, ch.Id)
		}
		t.Errorf("userGroup=vip: got channel ids %v, expected [1 3]", ids)
	}

	// internal 用户：ch2 被过滤
	got = filterGroupBlacklistedChannels(nil, channels, "internal")
	if len(got) != 2 {
		t.Errorf("userGroup=internal: got %d channels, expected 2", len(got))
	}

	// default 用户：不过滤
	got = filterGroupBlacklistedChannels(nil, channels, "default")
	if len(got) != 3 {
		t.Errorf("userGroup=default: got %d channels, expected 3", len(got))
	}

	// 空 userGroup：不过滤，原样返回
	got = filterGroupBlacklistedChannels(nil, channels, "")
	if len(got) != 3 {
		t.Errorf("userGroup empty: got %d channels, expected 3", len(got))
	}

	// 空渠道列表：原样返回
	if got := filterGroupBlacklistedChannels(nil, nil, "vip"); len(got) != 0 {
		t.Errorf("nil channels: got %d channels, expected 0", len(got))
	}
}
