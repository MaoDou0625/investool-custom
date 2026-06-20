package models

import (
	"path/filepath"
	"testing"
)

func TestFundDailyIndustrySignalCacheStoreSaveAndLoad(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "fund_daily_industry_signal_cache.json")
	store := NewFundDailyIndustrySignalCacheStore(filename)
	cache := FundDailyIndustrySignalCache{
		UpdatedAt: "2026-06-20 10:00:00",
		Items: map[string]FundDailyIndustrySignalCacheItem{
			"600000": {
				Code:      "600000",
				Name:      "sample",
				UpdatedAt: "2026-06-20 10:00:00",
				HistoricalPE: []FundDailyIndustryPEPoint{
					{Date: "2026-06-19", PE: 12.3},
				},
				ProfitPredicts: []FundDailyIndustryProfitPredictCachePoint{
					{Year: 2026, EPS: 1.23, PE: 10.2},
				},
			},
		},
	}

	if err := store.Save(cache); err != nil {
		t.Fatalf("save cache: %v", err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load cache: %v", err)
	}

	if loaded.Items["600000"].ProfitPredicts[0].EPS != 1.23 {
		t.Fatalf("expected cached EPS 1.23, got %+v", loaded.Items["600000"].ProfitPredicts)
	}
}

func TestFundDailyIndustrySignalCacheStoreLoadMissingFile(t *testing.T) {
	store := NewFundDailyIndustrySignalCacheStore(filepath.Join(t.TempDir(), "missing.json"))
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load missing cache: %v", err)
	}
	if len(loaded.Items) != 0 {
		t.Fatalf("expected empty cache, got %+v", loaded)
	}
}
