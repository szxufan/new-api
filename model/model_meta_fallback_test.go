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

func TestGetModelFallbackModel_PrefixMatch(t *testing.T) {
	setupFallbackTestDB(t)

	// 配置模型名 deepseek-v4，前缀匹配，降级到 deepseek-vl
	require.NoError(t, DB.Create(&Model{
		ModelName:    "deepseek-v4",
		FallbackModel: "deepseek-vl",
		NameRule:     NameRulePrefix,
		Status:       1,
	}).Error)

	RefreshPricing()

	// 精确匹配应返回降级模型
	assert.Equal(t, "deepseek-vl", GetModelFallbackModel("deepseek-v4"))
	// 前缀匹配：deepseek-v4-flash 以 deepseek-v4 为前缀，应返回降级模型
	assert.Equal(t, "deepseek-vl", GetModelFallbackModel("deepseek-v4-flash"))
	// 前缀匹配：deepseek-v4-chat 也应匹配
	assert.Equal(t, "deepseek-vl", GetModelFallbackModel("deepseek-v4-chat"))
	// 不匹配的前缀应返回空
	assert.Equal(t, "", GetModelFallbackModel("deepseek-v3-flash"))
}

func TestGetModelFallbackModel_SuffixMatch(t *testing.T) {
	setupFallbackTestDB(t)

	require.NoError(t, DB.Create(&Model{
		ModelName:    "-vision",
		FallbackModel: "gpt-4o",
		NameRule:     NameRuleSuffix,
		Status:       1,
	}).Error)

	RefreshPricing()

	assert.Equal(t, "gpt-4o", GetModelFallbackModel("claude-vision"))
	assert.Equal(t, "gpt-4o", GetModelFallbackModel("gpt-vision"))
	assert.Equal(t, "", GetModelFallbackModel("claude-text"))
}

func TestGetModelFallbackModel_ContainsMatch(t *testing.T) {
	setupFallbackTestDB(t)

	require.NoError(t, DB.Create(&Model{
		ModelName:    "turbo",
		FallbackModel: "gpt-4o",
		NameRule:     NameRuleContains,
		Status:       1,
	}).Error)

	RefreshPricing()

	assert.Equal(t, "gpt-4o", GetModelFallbackModel("gpt-turbo-3.5"))
	assert.Equal(t, "gpt-4o", GetModelFallbackModel("turbo-model"))
	assert.Equal(t, "", GetModelFallbackModel("gpt-4o"))
}

func TestGetModelFallbackModel_ExactMatchTakesPrecedence(t *testing.T) {
	setupFallbackTestDB(t)

	// 精确匹配模型
	require.NoError(t, DB.Create(&Model{
		ModelName:    "gpt-4",
		FallbackModel: "gpt-4o-exact",
		NameRule:     NameRuleExact,
		Status:       1,
	}).Error)

	// 前缀匹配模型（gpt-4 也匹配前缀，但精确匹配应优先）
	require.NoError(t, DB.Create(&Model{
		ModelName:    "gpt-",
		FallbackModel: "gpt-4o-prefix",
		NameRule:     NameRulePrefix,
		Status:       1,
	}).Error)

	RefreshPricing()

	// 精确匹配 gpt-4 应返回 gpt-4o-exact
	assert.Equal(t, "gpt-4o-exact", GetModelFallbackModel("gpt-4"))
	// 前缀匹配 gpt-4o 应返回 gpt-4o-prefix
	assert.Equal(t, "gpt-4o-prefix", GetModelFallbackModel("gpt-4o"))
}

func TestGetModelFallbackModel_EmptyModelName(t *testing.T) {
	setupFallbackTestDB(t)
	RefreshPricing()
	assert.Equal(t, "", GetModelFallbackModel(""))
}