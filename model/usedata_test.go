package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupQuotaDataTestDB(t *testing.T) {
	t.Helper()
	common.UsingSQLite = true
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	DB = db
	err = DB.AutoMigrate(&User{}, &QuotaData{})
	if err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}
}

func TestGetQuotaDataGroupByUserAndModel_Grouping(t *testing.T) {
	setupQuotaDataTestDB(t)

	records := []QuotaData{
		{Username: "alice", ModelName: "gpt-4", Count: 10, Quota: 100, TokenUsed: 500, CreatedAt: 1000},
		{Username: "alice", ModelName: "gpt-4", Count: 5, Quota: 50, TokenUsed: 250, CreatedAt: 1001},
		{Username: "alice", ModelName: "gpt-3.5", Count: 20, Quota: 30, TokenUsed: 1000, CreatedAt: 1002},
		{Username: "bob", ModelName: "gpt-4", Count: 3, Quota: 30, TokenUsed: 150, CreatedAt: 1003},
	}
	for i := range records {
		if err := DB.Create(&records[i]).Error; err != nil {
			t.Fatalf("failed to create quota data: %v", err)
		}
	}

	results, err := GetQuotaDataGroupByUserAndModel(0, 2000)
	if err != nil {
		t.Fatalf("GetQuotaDataGroupByUserAndModel failed: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 grouped results, got %d", len(results))
	}

	resultMap := make(map[string]*QuotaData)
	for _, r := range results {
		key := r.Username + "|" + r.ModelName
		resultMap[key] = r
	}

	aliceGpt4 := resultMap["alice|gpt-4"]
	if aliceGpt4 == nil {
		t.Fatal("missing grouped result for alice|gpt-4")
	}
	if aliceGpt4.Count != 15 {
		t.Errorf("alice|gpt-4 count: expected 15, got %d", aliceGpt4.Count)
	}
	if aliceGpt4.Quota != 150 {
		t.Errorf("alice|gpt-4 quota: expected 150, got %d", aliceGpt4.Quota)
	}
	if aliceGpt4.TokenUsed != 750 {
		t.Errorf("alice|gpt-4 token_used: expected 750, got %d", aliceGpt4.TokenUsed)
	}

	aliceGpt35 := resultMap["alice|gpt-3.5"]
	if aliceGpt35 == nil {
		t.Fatal("missing grouped result for alice|gpt-3.5")
	}
	if aliceGpt35.Count != 20 {
		t.Errorf("alice|gpt-3.5 count: expected 20, got %d", aliceGpt35.Count)
	}
	if aliceGpt35.Quota != 30 {
		t.Errorf("alice|gpt-3.5 quota: expected 30, got %d", aliceGpt35.Quota)
	}
	if aliceGpt35.TokenUsed != 1000 {
		t.Errorf("alice|gpt-3.5 token_used: expected 1000, got %d", aliceGpt35.TokenUsed)
	}

	bobGpt4 := resultMap["bob|gpt-4"]
	if bobGpt4 == nil {
		t.Fatal("missing grouped result for bob|gpt-4")
	}
	if bobGpt4.Count != 3 {
		t.Errorf("bob|gpt-4 count: expected 3, got %d", bobGpt4.Count)
	}
	if bobGpt4.Quota != 30 {
		t.Errorf("bob|gpt-4 quota: expected 30, got %d", bobGpt4.Quota)
	}
	if bobGpt4.TokenUsed != 150 {
		t.Errorf("bob|gpt-4 token_used: expected 150, got %d", bobGpt4.TokenUsed)
	}
}

func TestGetQuotaDataGroupByUserAndModel_SumAggregation(t *testing.T) {
	setupQuotaDataTestDB(t)

	records := []QuotaData{
		{Username: "charlie", ModelName: "claude", Count: 1, Quota: 10, TokenUsed: 100, CreatedAt: 500},
		{Username: "charlie", ModelName: "claude", Count: 2, Quota: 20, TokenUsed: 200, CreatedAt: 501},
		{Username: "charlie", ModelName: "claude", Count: 3, Quota: 30, TokenUsed: 300, CreatedAt: 502},
	}
	for i := range records {
		if err := DB.Create(&records[i]).Error; err != nil {
			t.Fatalf("failed to create quota data: %v", err)
		}
	}

	results, err := GetQuotaDataGroupByUserAndModel(0, 1000)
	if err != nil {
		t.Fatalf("GetQuotaDataGroupByUserAndModel failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 grouped result, got %d", len(results))
	}

	r := results[0]
	if r.Count != 6 {
		t.Errorf("sum count: expected 6, got %d", r.Count)
	}
	if r.Quota != 60 {
		t.Errorf("sum quota: expected 60, got %d", r.Quota)
	}
	if r.TokenUsed != 600 {
		t.Errorf("sum token_used: expected 600, got %d", r.TokenUsed)
	}
}

func TestGetQuotaDataGroupByUserAndModel_TimeRangeFiltering(t *testing.T) {
	setupQuotaDataTestDB(t)

	records := []QuotaData{
		{Username: "dave", ModelName: "gpt-4", Count: 1, Quota: 10, TokenUsed: 100, CreatedAt: 100},
		{Username: "dave", ModelName: "gpt-4", Count: 2, Quota: 20, TokenUsed: 200, CreatedAt: 200},
		{Username: "dave", ModelName: "gpt-4", Count: 3, Quota: 30, TokenUsed: 300, CreatedAt: 300},
		{Username: "dave", ModelName: "gpt-4", Count: 4, Quota: 40, TokenUsed: 400, CreatedAt: 400},
	}
	for i := range records {
		if err := DB.Create(&records[i]).Error; err != nil {
			t.Fatalf("failed to create quota data: %v", err)
		}
	}

	results, err := GetQuotaDataGroupByUserAndModel(200, 300)
	if err != nil {
		t.Fatalf("GetQuotaDataGroupByUserAndModel failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 grouped result, got %d", len(results))
	}

	r := results[0]
	if r.Count != 5 {
		t.Errorf("filtered sum count: expected 5, got %d", r.Count)
	}
	if r.Quota != 50 {
		t.Errorf("filtered sum quota: expected 50, got %d", r.Quota)
	}
	if r.TokenUsed != 500 {
		t.Errorf("filtered sum token_used: expected 500, got %d", r.TokenUsed)
	}
}

func TestGetQuotaDataGroupByUserAndModel_EmptyResult(t *testing.T) {
	setupQuotaDataTestDB(t)

	results, err := GetQuotaDataGroupByUserAndModel(0, 9999)
	if err != nil {
		t.Fatalf("GetQuotaDataGroupByUserAndModel failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results for empty table, got %d", len(results))
	}

	records := []QuotaData{
		{Username: "eve", ModelName: "gpt-4", Count: 5, Quota: 50, TokenUsed: 500, CreatedAt: 500},
	}
	for i := range records {
		if err := DB.Create(&records[i]).Error; err != nil {
			t.Fatalf("failed to create quota data: %v", err)
		}
	}

	results, err = GetQuotaDataGroupByUserAndModel(1000, 2000)
	if err != nil {
		t.Fatalf("GetQuotaDataGroupByUserAndModel failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results for non-matching time range, got %d", len(results))
	}
}
