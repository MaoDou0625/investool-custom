package models

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

var fundDailyIndustrySignalCacheStoreMu sync.Mutex

type FundDailyIndustrySignalCache struct {
	UpdatedAt string                                      `json:"updated_at"`
	Items     map[string]FundDailyIndustrySignalCacheItem `json:"items"`
}

type FundDailyIndustrySignalCacheItem struct {
	Code                            string                                     `json:"code"`
	Name                            string                                     `json:"name,omitempty"`
	UpdatedAt                       string                                     `json:"updated_at"`
	HistoricalPEUpdatedAt           string                                     `json:"historical_pe_updated_at,omitempty"`
	ProfitPredictsUpdatedAt         string                                     `json:"profit_predicts_updated_at,omitempty"`
	PreviousProfitPredictsUpdatedAt string                                     `json:"previous_profit_predicts_updated_at,omitempty"`
	HistoricalPE                    []FundDailyIndustryPEPoint                 `json:"historical_pe,omitempty"`
	ProfitPredicts                  []FundDailyIndustryProfitPredictCachePoint `json:"profit_predicts,omitempty"`
	PreviousProfitPredicts          []FundDailyIndustryProfitPredictCachePoint `json:"previous_profit_predicts,omitempty"`
}

type FundDailyIndustryPEPoint struct {
	Date string  `json:"date"`
	PE   float64 `json:"pe_ttm"`
}

type FundDailyIndustryProfitPredictCachePoint struct {
	Year int     `json:"year"`
	EPS  float64 `json:"eps"`
	PE   float64 `json:"pe"`
}

type FundDailyIndustrySignalCacheStore struct {
	filename string
}

func NewFundDailyIndustrySignalCacheStore(filename string) *FundDailyIndustrySignalCacheStore {
	return &FundDailyIndustrySignalCacheStore{filename: filename}
}

func (s *FundDailyIndustrySignalCacheStore) Load() (FundDailyIndustrySignalCache, error) {
	fundDailyIndustrySignalCacheStoreMu.Lock()
	defer fundDailyIndustrySignalCacheStoreMu.Unlock()

	return s.loadUnlocked()
}

func (s *FundDailyIndustrySignalCacheStore) Save(cache FundDailyIndustrySignalCache) error {
	fundDailyIndustrySignalCacheStoreMu.Lock()
	defer fundDailyIndustrySignalCacheStoreMu.Unlock()

	return s.saveUnlocked(cache)
}

func (s *FundDailyIndustrySignalCacheStore) loadUnlocked() (FundDailyIndustrySignalCache, error) {
	if s.filename == "" {
		return emptyFundDailyIndustrySignalCache(), errors.New("empty fund daily industry signal cache filename")
	}
	content, err := os.ReadFile(s.filename)
	if errors.Is(err, os.ErrNotExist) {
		return emptyFundDailyIndustrySignalCache(), nil
	}
	if err != nil {
		return emptyFundDailyIndustrySignalCache(), err
	}
	if len(content) == 0 {
		return emptyFundDailyIndustrySignalCache(), nil
	}
	cache := FundDailyIndustrySignalCache{}
	if err := json.Unmarshal(content, &cache); err != nil {
		return emptyFundDailyIndustrySignalCache(), err
	}
	if cache.Items == nil {
		cache.Items = map[string]FundDailyIndustrySignalCacheItem{}
	}
	return cache, nil
}

func (s *FundDailyIndustrySignalCacheStore) saveUnlocked(cache FundDailyIndustrySignalCache) error {
	if s.filename == "" {
		return errors.New("empty fund daily industry signal cache filename")
	}
	if cache.Items == nil {
		cache.Items = map[string]FundDailyIndustrySignalCacheItem{}
	}
	content, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	if dir := filepath.Dir(s.filename); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	tmpFilename := s.filename + ".tmp.json"
	if err := os.WriteFile(tmpFilename, content, 0644); err != nil {
		return err
	}
	return os.Rename(tmpFilename, s.filename)
}

func emptyFundDailyIndustrySignalCache() FundDailyIndustrySignalCache {
	return FundDailyIndustrySignalCache{
		Items: map[string]FundDailyIndustrySignalCacheItem{},
	}
}
