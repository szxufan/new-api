package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupModelPurposeTestDB(t *testing.T) {
	t.Helper()

	originalDB := DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalCapabilities := modelCapabilitiesMap

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	DB = db
	require.NoError(t, DB.AutoMigrate(&Channel{}, &Ability{}, &Model{}, &Vendor{}))

	t.Cleanup(func() {
		InvalidatePricingCache()
		DB = originalDB
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		modelCapabilitiesLock.Lock()
		modelCapabilitiesMap = originalCapabilities
		modelCapabilitiesLock.Unlock()
		require.NoError(t, sqlDB.Close())
	})
}

func TestNormalizeModelPurposes(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		expected string
		parsed   []string
	}{
		{name: "empty", raw: "", expected: ""},
		{name: "whitespace", raw: "  \n\t", expected: ""},
		{name: "empty array", raw: " [ ] ", expected: "[]", parsed: []string{}},
		{
			name:     "multiple values and duplicates",
			raw:      `["writing","chat","writing","programming","chat"]`,
			expected: `["writing","chat","programming"]`,
			parsed:   []string{constant.ModelPurposeWriting, constant.ModelPurposeChat, constant.ModelPurposeProgramming},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalized, err := NormalizeModelPurposes(tt.raw)
			require.NoError(t, err)
			require.Equal(t, tt.expected, normalized)

			purposes, err := ParseModelPurposes(normalized)
			require.NoError(t, err)
			require.Equal(t, tt.parsed, purposes)
		})
	}
}

func TestNormalizeModelPurposesRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name        string
		raw         string
		errorSubstr string
	}{
		{name: "malformed JSON", raw: `["chat",`, errorSubstr: "字符串数组"},
		{name: "object", raw: `{"purpose":"chat"}`, errorSubstr: "必须是字符串数组"},
		{name: "string", raw: `"chat"`, errorSubstr: "必须是字符串数组"},
		{name: "null", raw: `null`, errorSubstr: "必须是字符串数组"},
		{name: "mixed array", raw: `["chat",1]`, errorSubstr: "字符串数组"},
		{name: "unknown purpose", raw: `["chat","coding"]`, errorSubstr: "coding"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalized, err := NormalizeModelPurposes(tt.raw)
			require.Error(t, err)
			require.Empty(t, normalized)
			require.Contains(t, err.Error(), tt.errorSubstr)
		})
	}
}

func TestModelUpdatePurposesPersistsAndClears(t *testing.T) {
	setupModelPurposeTestDB(t)

	item := &Model{
		ModelName: "purpose-update",
		Status:    1,
		Purposes:  `["chat","programming"]`,
	}
	require.NoError(t, item.Insert())

	item.Purposes = `["writing","chat"]`
	require.NoError(t, item.Update())
	var stored Model
	require.NoError(t, DB.First(&stored, item.Id).Error)
	require.Equal(t, `["writing","chat"]`, stored.Purposes)

	item.Purposes = ""
	require.NoError(t, item.Update())
	require.NoError(t, DB.First(&stored, item.Id).Error)
	require.Empty(t, stored.Purposes)
}

func TestModelPurposesCapabilitiesCache(t *testing.T) {
	setupModelPurposeTestDB(t)

	require.NoError(t, DB.Create(&Channel{
		Id:     1,
		Type:   constant.ChannelTypeOpenAI,
		Key:    "test-key",
		Status: common.ChannelStatusEnabled,
		Name:   "purpose-test-channel",
	}).Error)
	require.NoError(t, DB.Create(&[]Ability{
		{Group: "default", Model: "purpose-exact", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "purpose-prefixed", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "purpose-corrupt", ChannelId: 1, Enabled: true},
	}).Error)
	require.NoError(t, DB.Create(&[]Model{
		{ModelName: "purpose-exact", Status: 1, Purposes: `["chat","chat","writing"]`},
		{ModelName: "purpose-", Status: 1, NameRule: NameRulePrefix, Purposes: `["programming"]`},
		{ModelName: "purpose-corrupt", Status: 1, Purposes: `["coding"]`},
	}).Error)

	RefreshPricing()
	require.Equal(t, []string{"chat", "writing"}, GetModelCapabilities("purpose-exact").Purposes)
	require.Equal(t, []string{"programming"}, GetModelCapabilities("purpose-prefixed").Purposes)
	require.Empty(t, GetModelCapabilities("purpose-corrupt").Purposes)

	require.NoError(t, DB.Model(&Model{}).Where("model_name = ?", "purpose-exact").Update("purposes", "").Error)
	RefreshPricing()
	require.Empty(t, GetModelCapabilities("purpose-exact").Purposes)
}
