package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupFallbackTestDB(t *testing.T) {
	t.Helper()
	common.UsingSQLite = true
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	require.NoError(t, DB.AutoMigrate(&Channel{}, &Ability{}, &Model{}))
}

func TestGetModelFallbackModel_Configured(t *testing.T) {
	setupFallbackTestDB(t)

	require.NoError(t, DB.Create(&Model{
		ModelName:    "gpt-4",
		FallbackModel: "gpt-4o",
		Status:       1,
	}).Error)

	RefreshPricing()

	result := GetModelFallbackModel("gpt-4")
	assert.Equal(t, "gpt-4o", result)
}

func TestGetModelFallbackModel_NotConfigured(t *testing.T) {
	setupFallbackTestDB(t)

	require.NoError(t, DB.Create(&Model{
		ModelName: "gpt-4",
		Status:    1,
	}).Error)

	RefreshPricing()

	result := GetModelFallbackModel("gpt-4")
	assert.Equal(t, "", result)
}

func TestGetModelFallbackModel_ModelNotFound(t *testing.T) {
	setupFallbackTestDB(t)

	RefreshPricing()

	result := GetModelFallbackModel("nonexistent-model")
	assert.Equal(t, "", result)
}

func TestGetModelFallbackModel_CacheRefresh(t *testing.T) {
	setupFallbackTestDB(t)

	require.NoError(t, DB.Create(&Model{
		ModelName:    "gpt-4",
		FallbackModel: "gpt-4o",
		Status:       1,
	}).Error)

	RefreshPricing()

	result := GetModelFallbackModel("gpt-4")
	assert.Equal(t, "gpt-4o", result)

	DB.Model(&Model{}).Where("model_name = ?", "gpt-4").Update("fallback_model", "gpt-4-turbo")
	RefreshPricing()

	result = GetModelFallbackModel("gpt-4")
	assert.Equal(t, "gpt-4-turbo", result)
}

func TestGetModelFallbackModel_EmptyFallbackClears(t *testing.T) {
	setupFallbackTestDB(t)

	require.NoError(t, DB.Create(&Model{
		ModelName:    "gpt-4",
		FallbackModel: "gpt-4o",
		Status:       1,
	}).Error)

	RefreshPricing()

	result := GetModelFallbackModel("gpt-4")
	assert.Equal(t, "gpt-4o", result)

	DB.Model(&Model{}).Where("model_name = ?", "gpt-4").Update("fallback_model", "")
	RefreshPricing()

	result = GetModelFallbackModel("gpt-4")
	assert.Equal(t, "", result)
}

func TestGetModelFallbackModel_DisabledModelStillCached(t *testing.T) {
	setupFallbackTestDB(t)

	require.NoError(t, DB.Create(&Model{
		ModelName:    "gpt-4",
		FallbackModel: "gpt-4o",
		Status:       0,
	}).Error)

	RefreshPricing()

	result := GetModelFallbackModel("gpt-4")
	assert.Equal(t, "gpt-4o", result)
}