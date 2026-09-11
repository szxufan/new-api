package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupChannelBalanceTestDB 初始化内存 SQLite，仅迁移渠道表
func setupChannelBalanceTestDB(t *testing.T) {
	t.Helper()
	common.UsingSQLite = true
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false
	common.MemoryCacheEnabled = false
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.Channel{}))
	t.Cleanup(func() {
		db.Exec("DELETE FROM channels")
	})
}

// mockOpenAIUpstream 启动一个 mock 上游，模拟 OpenAI 兼容的 billing 接口，
// 返回 mock 服务器地址，用于渠道 BaseURL
func mockOpenAIUpstream(t *testing.T) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/dashboard/billing/subscription", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"billing_subscription","has_payment_method":true,"soft_limit_usd":100,"hard_limit_usd":100,"system_hard_limit_usd":100,"access_until":0}`))
	})
	mux.HandleFunc("/v1/dashboard/billing/usage", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","total_usage":2500}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL
}

// doUpdateChannelBalance 直接调用 UpdateChannelBalance 并解析响应
func doUpdateChannelBalance(t *testing.T, channelId int) (int, map[string]any) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/channel/update_balance/"+strconv.Itoa(channelId), nil)
	c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(channelId)}}

	UpdateChannelBalance(c)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return w.Code, resp
}

// TestUpdateChannelBalance_ReturnsUsedQuota 接口应在返回 balance 的同时返回 used_quota
func TestUpdateChannelBalance_ReturnsUsedQuota(t *testing.T) {
	setupChannelBalanceTestDB(t)
	baseURL := mockOpenAIUpstream(t)

	ch := &model.Channel{
		Name:      "balance-used-quota",
		Type:      constant.ChannelTypeOpenAI,
		Status:    common.ChannelStatusEnabled,
		Key:       "sk-test",
		Models:    "gpt-4o",
		Group:     "default",
		UsedQuota: 12345,
	}
	ch.BaseURL = &baseURL
	require.NoError(t, model.DB.Create(ch).Error)

	code, resp := doUpdateChannelBalance(t, ch.Id)
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, true, resp["success"], "response should be success")

	// balance = hard_limit_usd - total_usage/100 = 100 - 25 = 75
	balance, ok := resp["balance"].(float64)
	require.True(t, ok, "balance should be a number, got %v", resp["balance"])
	require.InDelta(t, 75.0, balance, 0.01)

	// used_quota 应随响应返回并与 DB 一致
	usedQuota, ok := resp["used_quota"].(float64)
	require.True(t, ok, "used_quota should be a number, got %v", resp["used_quota"])
	require.Equal(t, float64(12345), usedQuota)
}

// TestUpdateChannelBalance_UsedQuotaReadFromDB 内存缓存中的 used_quota 滞后时，
// 接口应返回 DB 中的最新值
func TestUpdateChannelBalance_UsedQuotaReadFromDB(t *testing.T) {
	setupChannelBalanceTestDB(t)
	baseURL := mockOpenAIUpstream(t)

	ch := &model.Channel{
		Name:      "balance-used-quota-fresh",
		Type:      constant.ChannelTypeOpenAI,
		Status:    common.ChannelStatusEnabled,
		Key:       "sk-test",
		Models:    "gpt-4o",
		Group:     "default",
		UsedQuota: 100,
	}
	ch.BaseURL = &baseURL
	require.NoError(t, model.DB.Create(ch).Error)

	// 模拟缓存滞后：向缓存写入 UsedQuota=100 的旧对象，随后 DB 更新为 999
	origCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	model.CacheUpdateChannel(&model.Channel{
		Id:        ch.Id,
		Name:      ch.Name,
		Type:      ch.Type,
		Status:    ch.Status,
		UsedQuota: 100,
	})
	common.MemoryCacheEnabled = origCacheEnabled

	require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", ch.Id).
		Update("used_quota", gorm.Expr("used_quota + ?", 899)).Error)

	code, resp := doUpdateChannelBalance(t, ch.Id)
	require.Equal(t, http.StatusOK, code)
	usedQuota, ok := resp["used_quota"].(float64)
	require.True(t, ok, "used_quota should be a number, got %v", resp["used_quota"])
	require.Equal(t, float64(999), usedQuota, "used_quota should reflect the latest DB value")
}

// TestUpdateChannelBalance_MultiKeyNoUsedQuota 多密钥渠道不支持余额查询，
// 不应返回 used_quota 字段
func TestUpdateChannelBalance_MultiKeyNoUsedQuota(t *testing.T) {
	setupChannelBalanceTestDB(t)
	baseURL := mockOpenAIUpstream(t)

	ch := &model.Channel{
		Name:      "multi-key-channel",
		Type:      constant.ChannelTypeOpenAI,
		Status:    common.ChannelStatusEnabled,
		Key:       "sk-a\nsk-b",
		Models:    "gpt-4o",
		Group:     "default",
		UsedQuota: 555,
	}
	ch.BaseURL = &baseURL
	ch.ChannelInfo.IsMultiKey = true
	require.NoError(t, model.DB.Create(ch).Error)

	code, resp := doUpdateChannelBalance(t, ch.Id)
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, false, resp["success"])
	_, hasUsedQuota := resp["used_quota"]
	require.False(t, hasUsedQuota, "multi-key channel response should not contain used_quota")
}
