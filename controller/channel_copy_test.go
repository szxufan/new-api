package controller

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupChannelCopyTestDB(t *testing.T) {
	t.Helper()
	common.UsingSQLite = true
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}, &model.Ability{}))
}

func TestCopyChannelCreatesDisabledClone(t *testing.T) {
	setupChannelCopyTestDB(t)

	// create an enabled channel to be copied
	origin := &model.Channel{
		Name:      "origin-channel",
		Key:       "sk-test",
		Status:    common.ChannelStatusEnabled,
		Models:    "gpt-4o",
		Group:     "default",
		Balance:   10.0,
		UsedQuota: 100,
	}
	require.NoError(t, model.DB.Create(origin).Error)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/channel/copy/"+strconv.Itoa(origin.Id), nil)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(origin.Id)}}

	CopyChannel(ctx)

	require.Equal(t, http.StatusOK, w.Code)

	// original channel should remain enabled and untouched
	require.Equal(t, common.ChannelStatusEnabled, origin.Status)

	// cloned channel should be disabled
	var cloned model.Channel
	require.NoError(t, model.DB.First(&cloned, "name = ?", "origin-channel_复制").Error)
	require.Equal(t, common.ChannelStatusManuallyDisabled, cloned.Status, "copied channel should be disabled")
	require.NotEqual(t, origin.Id, cloned.Id)
	require.Equal(t, 0.0, cloned.Balance)
	require.Equal(t, int64(0), cloned.UsedQuota)

	// abilities of the cloned channel should also be disabled
	var abilities []model.Ability
	require.NoError(t, model.DB.Where("channel_id = ?", cloned.Id).Find(&abilities).Error)
	require.NotEmpty(t, abilities)
	for _, ability := range abilities {
		require.False(t, ability.Enabled, "ability for copied channel should be disabled")
	}
}
