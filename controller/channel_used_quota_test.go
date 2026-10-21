package controller

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupUsedQuotaTestDB(t *testing.T) *model.Channel {
	t.Helper()
	common.UsingSQLite = true
	common.MemoryCacheEnabled = false
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}, &model.Ability{}))

	channel := &model.Channel{
		Name:    "used-quota-test",
		Key:     "sk-test",
		Status:  common.ChannelStatusEnabled,
		Models:  "gpt-4o",
		Group:   "default",
		Balance: 10.0,
	}
	channel.UsedQuota = 123456
	require.NoError(t, model.DB.Create(channel).Error)
	return channel
}

func callUpdateChannelWithBody(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/channel/", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	UpdateChannel(ctx)
	return w
}

// TestUpdateChannel_ExplicitUsedQuotaZero 显式提交 used_quota=0 时必须写入（GORM 结构体更新会跳过零值）
func TestUpdateChannel_ExplicitUsedQuotaZero(t *testing.T) {
	ch := setupUsedQuotaTestDB(t)

	w := callUpdateChannelWithBody(t, `{"id":`+strconv.Itoa(ch.Id)+`,"used_quota":0}`)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"success":true`)

	var dbCh model.Channel
	require.NoError(t, model.DB.First(&dbCh, "id = ?", ch.Id).Error)
	require.Equal(t, int64(0), dbCh.UsedQuota)
}

// TestUpdateChannel_ExplicitUsedQuotaValue 显式提交非零消耗额度正常写入
func TestUpdateChannel_ExplicitUsedQuotaValue(t *testing.T) {
	ch := setupUsedQuotaTestDB(t)

	w := callUpdateChannelWithBody(t, `{"id":`+strconv.Itoa(ch.Id)+`,"used_quota":500000}`)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"success":true`)

	var dbCh model.Channel
	require.NoError(t, model.DB.First(&dbCh, "id = ?", ch.Id).Error)
	require.Equal(t, int64(500000), dbCh.UsedQuota)
}

// TestUpdateChannel_NegativeUsedQuotaRejected 负数消耗额度被拒绝且不落库
func TestUpdateChannel_NegativeUsedQuotaRejected(t *testing.T) {
	ch := setupUsedQuotaTestDB(t)

	w := callUpdateChannelWithBody(t, `{"id":`+strconv.Itoa(ch.Id)+`,"used_quota":-5}`)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"success":false`)

	var dbCh model.Channel
	require.NoError(t, model.DB.First(&dbCh, "id = ?", ch.Id).Error)
	require.Equal(t, int64(123456), dbCh.UsedQuota)
}

// TestUpdateChannel_WithoutUsedQuotaKept 未显式提交 used_quota 的常规更新不改动消耗额度
func TestUpdateChannel_WithoutUsedQuotaKept(t *testing.T) {
	ch := setupUsedQuotaTestDB(t)

	w := callUpdateChannelWithBody(t, `{"id":`+strconv.Itoa(ch.Id)+`,"weight":10}`)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"success":true`)

	var dbCh model.Channel
	require.NoError(t, model.DB.First(&dbCh, "id = ?", ch.Id).Error)
	require.Equal(t, int64(123456), dbCh.UsedQuota)
}


