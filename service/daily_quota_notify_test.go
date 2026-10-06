package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

func TestConvertDisplayCurrencyToQuota(t *testing.T) {
	common.QuotaPerUnit = 500000
	operation_setting.USDExchangeRate = 7.3
	operation_setting.GetGeneralSetting().CustomCurrencyExchangeRate = 2.0

	cases := []struct {
		name   string
		val    float64
		disp   string
		expect int
	}{
		{"USD 1", 1, operation_setting.QuotaDisplayTypeUSD, 500000},
		{"USD 0.5", 0.5, operation_setting.QuotaDisplayTypeUSD, 250000},
		{"CNY 7.3", 7.3, operation_setting.QuotaDisplayTypeCNY, 500000},
		{"CUSTOM 2", 2, operation_setting.QuotaDisplayTypeCustom, 500000},
		{"TOKENS 123", 123, operation_setting.QuotaDisplayTypeTokens, 123},
	}
	for _, c := range cases {
		if got := convertDisplayCurrencyToQuota(c.val, c.disp); got != c.expect {
			t.Errorf("%s: convertDisplayCurrencyToQuota(%v, %s) = %d, want %d", c.name, c.val, c.disp, got, c.expect)
		}
	}
}

func TestConvertDisplayCurrencyToQuotaGuardedRates(t *testing.T) {
	common.QuotaPerUnit = 500000

	operation_setting.USDExchangeRate = 0
	if got := convertDisplayCurrencyToQuota(1, operation_setting.QuotaDisplayTypeCNY); got != 500000 {
		t.Errorf("CNY with zero rate: got %d, want 500000", got)
	}

	operation_setting.GetGeneralSetting().CustomCurrencyExchangeRate = -1
	if got := convertDisplayCurrencyToQuota(1, operation_setting.QuotaDisplayTypeCustom); got != 500000 {
		t.Errorf("CUSTOM with negative rate: got %d, want 500000", got)
	}
}
