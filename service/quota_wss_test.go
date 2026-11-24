package service

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

type recordingWssBillingSettler struct {
	preConsumed int
	reserved    []int
}

func (s *recordingWssBillingSettler) Settle(actualQuota int) error { return nil }
func (s *recordingWssBillingSettler) Refund(c *gin.Context)        {}
func (s *recordingWssBillingSettler) NeedsRefund() bool            { return false }
func (s *recordingWssBillingSettler) GetPreConsumedQuota() int     { return s.preConsumed }
func (s *recordingWssBillingSettler) Reserve(targetQuota int) error {
	s.reserved = append(s.reserved, targetQuota)
	s.preConsumed = targetQuota
	return nil
}

func TestReserveWssQuota_SubscriptionUsesSubscriptionFunding(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	billing := &recordingWssBillingSettler{}
	info := &relaycommon.RelayInfo{
		OriginModelName: "sambert-zhichu-v1",
		UsingGroup:      "default",
		BillingSource:   BillingSourceSubscription,
		Billing:         billing,
	}
	usage := &dto.RealtimeUsage{TotalTokens: 10, InputTokens: 10}
	usage.InputTokenDetails.TextTokens = 10

	// 订阅计费不应读取钱包余额；测试环境未初始化用户表也能完成预留。
	assert.NoError(t, ReserveWssQuota(c, info, usage))
	assert.Len(t, billing.reserved, 1)
	assert.Positive(t, billing.reserved[0])

	// 相同累计用量不重复预留，增加用量时只把目标值提升到新总量。
	assert.NoError(t, ReserveWssQuota(c, info, usage))
	assert.Len(t, billing.reserved, 1)
	usage.TotalTokens = 20
	usage.InputTokens = 20
	usage.InputTokenDetails.TextTokens = 20
	assert.NoError(t, ReserveWssQuota(c, info, usage))
	assert.Len(t, billing.reserved, 2)
	assert.Greater(t, billing.reserved[1], billing.reserved[0])
}

func TestWssQuota_PriceDataUsePriceSkipsTokenReservation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	billing := &recordingWssBillingSettler{}
	info := &relaycommon.RelayInfo{
		OriginModelName: "sambert-zhichu-v1",
		Billing:         billing,
	}
	info.PriceData.UsePrice = true
	usage := &dto.RealtimeUsage{TotalTokens: 100, InputTokens: 100}
	usage.InputTokenDetails.TextTokens = 100

	assert.NoError(t, ReserveWssQuota(c, info, usage))
	assert.Empty(t, billing.reserved, "fixed-price requests must not reserve additional token-based quota")
	// 同一权威标志也适用于现有 realtime 预扣路径，且不应访问用户/令牌数据库。
	assert.NoError(t, PreWssConsumeQuota(c, info, usage))
}
