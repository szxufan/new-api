package service

import (
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/bytedance/gopkg/util/gopool"
	"github.com/go-redis/redis/v8"
)

// DailyQuotaNotifyTitle 每日消费额度提醒的消息标题，也可作为钉钉机器人"自定义关键词"安全设置的匹配词
const DailyQuotaNotifyTitle = "每日消费提醒"

const (
	dailyQuotaNotifyLockTTL = 10 * time.Second
)

// ---- 内存降级存储（无 Redis 时使用，仅单节点有效）----

type dailyQuotaMemEntry struct {
	mu       sync.Mutex
	date     string // "20060102"
	consumed int
	notified int
	sending  bool
}

var (
	dailyQuotaMemStore sync.Map // userId -> *dailyQuotaMemEntry
	dailyQuotaMemOnce  sync.Once
)

// OnUserQuotaConsumed 注册到 model.UserQuotaConsumedHooks 的回调：
// 任一账号发生一笔消费后检查其当日消费是否跨过每日额度阈值的整数倍。
// 快速路径仅读内存配置，重逻辑全部异步，不阻塞计费主流程。
func OnUserQuotaConsumed(userId int, quota int) {
	setting := operation_setting.GetDailyQuotaNotifySetting()
	if !setting.Enabled || quota <= 0 {
		return
	}
	gopool.Go(func() {
		checkDailyQuotaNotify(userId, quota, setting)
	})
}

func checkDailyQuotaNotify(userId int, quota int, setting *operation_setting.DailyQuotaNotifySetting) {
	thresholdQuota := displayCurrencyToQuota(setting.Threshold)
	if thresholdQuota <= 0 || setting.WebhookUrl == "" {
		return
	}

	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dayEnd := dayStart.AddDate(0, 0, 1)

	consumed, err := addDailyConsume(userId, quota, dayStart.Unix(), dayEnd.Unix())
	if err != nil {
		common.SysError(fmt.Sprintf("daily quota notify: failed to update daily consume counter for user %d: %s", userId, err.Error()))
		return
	}

	multiple := consumed / thresholdQuota
	if multiple <= 0 || multiple <= getDailyNotifiedMultiple(userId) {
		return
	}

	if !claimDailyNotifyLock(userId) {
		return
	}
	// 拿到锁后复查标记，避免并发下重复发送
	if multiple <= getDailyNotifiedMultiple(userId) {
		releaseDailyNotifyLock(userId)
		return
	}

	if err := SendDingTalkNotify(setting.WebhookUrl, setting.Secret, DailyQuotaNotifyTitle, buildDailyQuotaNotifyText(userId, consumed, multiple, thresholdQuota)); err != nil {
		// 发送失败不更新标记，放开锁让下一笔消费重试
		common.SysError(fmt.Sprintf("daily quota notify: failed to send dingtalk message for user %d: %s", userId, err.Error()))
		releaseDailyNotifyLock(userId)
		return
	}
	setDailyNotifiedMultiple(userId, multiple)
	releaseDailyNotifyLock(userId)
}

// buildDailyQuotaNotifyText 组装提醒正文：展示用户名（取不到时回退"用户ID"占位），
// 用户设置了显示名称时追加"（xxx）"，不展示用户 ID。
func buildDailyQuotaNotifyText(userId int, consumed int, multiple int, thresholdQuota int) string {
	username, err := model.GetUsernameById(userId, false)
	if err != nil || username == "" {
		username = fmt.Sprintf("用户%d", userId)
	}
	userLabel := fmt.Sprintf("**%s**", username)
	if displayName, dnErr := model.GetDisplayNameById(userId); dnErr == nil && displayName != "" && displayName != username {
		userLabel = fmt.Sprintf("**%s**（%s）", username, displayName)
	}
	return fmt.Sprintf("### %s\n\n用户 %s 今日消费已达到 **%s**，为每日额度 %s 的 **%d** 倍。\n\n日期：%s",
		DailyQuotaNotifyTitle, userLabel, logger.FormatQuota(consumed), logger.FormatQuota(thresholdQuota), multiple, time.Now().Format("2006-01-02"))
}

// displayCurrencyToQuota 按站点货币显示类型把阈值换算为站内额度，换算逻辑与 logger.FormatQuota 一致
func displayCurrencyToQuota(val float64) int {
	return convertDisplayCurrencyToQuota(val, operation_setting.GetQuotaDisplayType())
}

func convertDisplayCurrencyToQuota(val float64, displayType string) int {
	switch displayType {
	case operation_setting.QuotaDisplayTypeCNY:
		rate := operation_setting.USDExchangeRate
		if rate <= 0 {
			rate = 1
		}
		return int(val / rate * common.QuotaPerUnit)
	case operation_setting.QuotaDisplayTypeCustom:
		rate := operation_setting.GetGeneralSetting().CustomCurrencyExchangeRate
		if rate <= 0 {
			rate = 1
		}
		return int(val / rate * common.QuotaPerUnit)
	case operation_setting.QuotaDisplayTypeTokens:
		return int(val)
	default: // USD
		return int(val * common.QuotaPerUnit)
	}
}

// ---- 当日消费计数 ----

// addDailyConsume 累加用户当日消费并返回累计值。
// Redis 模式下计数器丢失（重启/首次出现）时先从日志表播种今日已消费，保证跨重启与多节点一致；
// 无 Redis 时退化为内存计数（仅单节点有效）。
func addDailyConsume(userId int, quota int, dayStart int64, dayEnd int64) (int, error) {
	if common.RedisEnabled {
		return addDailyConsumeRedis(userId, quota, dayStart, dayEnd)
	}
	return addDailyConsumeMemory(userId, quota), nil
}

func addDailyConsumeRedis(userId int, quota int, dayStart int64, dayEnd int64) (int, error) {
	key := dailyQuotaConsumeKey(userId)
	ttl := dailyQuotaKeyTTL()

	val, err := common.RedisGet(key)
	if err != nil && !errors.Is(err, redis.Nil) {
		return 0, fmt.Errorf("failed to get daily consume counter: %w", err)
	}

	if val == "" {
		// 计数器不存在：从日志表播种今日已消费，避免节点重启或多节点计数不一致
		base, seedErr := model.SumUserConsumedQuota(userId, dayStart, dayEnd)
		if seedErr != nil {
			common.SysError(fmt.Sprintf("daily quota notify: failed to seed daily consume for user %d: %s", userId, seedErr.Error()))
			base = 0
		}
		seeded := base + quota
		ok, setErr := common.RedisSetNX(key, strconv.Itoa(seeded), ttl)
		if setErr == nil && ok {
			return seeded, nil
		}
		// 其他请求已抢先播种，退回增量累加
	}

	if err := common.RedisIncr(key, int64(quota)); err != nil {
		return 0, fmt.Errorf("failed to incr daily consume counter: %w", err)
	}
	val, err = common.RedisGet(key)
	if err != nil {
		return 0, fmt.Errorf("failed to get daily consume counter: %w", err)
	}
	consumed, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("invalid daily consume counter value %q: %w", val, err)
	}
	return consumed, nil
}

func addDailyConsumeMemory(userId int, quota int) int {
	entry := loadDailyQuotaMemEntry(userId)
	entry.mu.Lock()
	defer entry.mu.Unlock()
	if entry.date != todayStr() {
		entry.date = todayStr()
		entry.consumed = 0
		entry.notified = 0
		entry.sending = false
	}
	entry.consumed += quota
	return entry.consumed
}

// ---- 已通知倍数标记 ----

func getDailyNotifiedMultiple(userId int) int {
	if common.RedisEnabled {
		val, err := common.RedisGet(dailyQuotaNotifiedKey(userId))
		if err != nil || val == "" {
			return 0
		}
		n, _ := strconv.Atoi(val)
		return n
	}
	entry := loadDailyQuotaMemEntry(userId)
	entry.mu.Lock()
	defer entry.mu.Unlock()
	if entry.date != todayStr() {
		return 0
	}
	return entry.notified
}

func setDailyNotifiedMultiple(userId int, multiple int) {
	if common.RedisEnabled {
		_ = common.RedisSet(dailyQuotaNotifiedKey(userId), strconv.Itoa(multiple), dailyQuotaKeyTTL())
		return
	}
	entry := loadDailyQuotaMemEntry(userId)
	entry.mu.Lock()
	defer entry.mu.Unlock()
	if entry.date != todayStr() {
		entry.date = todayStr()
		entry.consumed = 0
		entry.sending = false
	}
	entry.notified = multiple
}

// ---- 发送去重锁 ----

// claimDailyNotifyLock 领取短锁以去重并发发送；Redis 异常时放行，避免彻底丢失通知
func claimDailyNotifyLock(userId int) bool {
	if common.RedisEnabled {
		ok, err := common.RedisSetNX(dailyQuotaLockKey(userId), "1", dailyQuotaNotifyLockTTL)
		if err != nil {
			return true
		}
		return ok
	}
	entry := loadDailyQuotaMemEntry(userId)
	entry.mu.Lock()
	defer entry.mu.Unlock()
	if entry.date != todayStr() {
		entry.date = todayStr()
		entry.consumed = 0
		entry.notified = 0
		entry.sending = false
	}
	if entry.sending {
		return false
	}
	entry.sending = true
	return true
}

func releaseDailyNotifyLock(userId int) {
	if common.RedisEnabled {
		_ = common.RedisDel(dailyQuotaLockKey(userId))
		return
	}
	entry := loadDailyQuotaMemEntry(userId)
	entry.mu.Lock()
	defer entry.mu.Unlock()
	entry.sending = false
}

// ---- 工具 ----

func todayStr() string {
	return time.Now().Format("20060102")
}

// dailyQuotaKeyTTL 当日 key 的过期时间：次日凌晨再加 1 小时缓冲
func dailyQuotaKeyTTL() time.Duration {
	now := time.Now()
	nextMidnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, 1)
	return time.Until(nextMidnight) + time.Hour
}

func dailyQuotaConsumeKey(userId int) string {
	return fmt.Sprintf("dq_consume:%d:%s", userId, todayStr())
}

func dailyQuotaNotifiedKey(userId int) string {
	return fmt.Sprintf("dq_notified:%d:%s", userId, todayStr())
}

func dailyQuotaLockKey(userId int) string {
	return fmt.Sprintf("dq_lock:%d:%s", userId, todayStr())
}

func loadDailyQuotaMemEntry(userId int) *dailyQuotaMemEntry {
	dailyQuotaMemOnce.Do(startDailyQuotaMemCleanup)
	if v, ok := dailyQuotaMemStore.Load(userId); ok {
		return v.(*dailyQuotaMemEntry)
	}
	v, _ := dailyQuotaMemStore.LoadOrStore(userId, &dailyQuotaMemEntry{date: todayStr()})
	return v.(*dailyQuotaMemEntry)
}

// startDailyQuotaMemCleanup 定期清理过期的内存计数条目，仿 notify-limit 的清理协程
func startDailyQuotaMemCleanup() {
	gopool.Go(func() {
		for {
			time.Sleep(time.Hour)
			today := todayStr()
			dailyQuotaMemStore.Range(func(key, value interface{}) bool {
				if entry, ok := value.(*dailyQuotaMemEntry); ok && entry.date != today {
					dailyQuotaMemStore.Delete(key)
				}
				return true
			})
		}
	})
}
