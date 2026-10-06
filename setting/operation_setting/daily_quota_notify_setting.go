package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

// DailyQuotaNotifySetting 每日消费额度提醒配置
// 仅作提醒用途，达到阈值不影响用户继续使用。
// 阈值单位跟随站点货币显示类型（GetQuotaDisplayType）：
// USD/CNY/CUSTOM 按对应货币金额填写，TOKENS 按站内原始额度填写。
type DailyQuotaNotifySetting struct {
	Enabled    bool    `json:"enabled"`     // 是否启用每日消费额度提醒
	Threshold  float64 `json:"threshold"`   // 每日额度阈值，任一账号当日消费达到其整数倍时通知
	WebhookUrl string  `json:"webhook_url"` // 钉钉群机器人 webhook 地址
	Secret     string  `json:"secret"`      // 钉钉机器人加签密钥（安全设置为加签时必填，否则留空）
}

var dailyQuotaNotifySetting = DailyQuotaNotifySetting{}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("daily_quota_notify_setting", &dailyQuotaNotifySetting)
}

func GetDailyQuotaNotifySetting() *DailyQuotaNotifySetting {
	return &dailyQuotaNotifySetting
}
