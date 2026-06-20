package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/axiaoxin-com/investool/datacenter/eastmoney"
	"github.com/axiaoxin-com/investool/models"
)

type fakeFundDailyIndustryPublicDataProvider struct {
	pe       map[string]eastmoney.HistoricalPEList
	predicts map[string]eastmoney.ProfitPredictList
	peErr    error
	predErr  error
	err      error
}

func (p fakeFundDailyIndustryPublicDataProvider) QueryHistoricalPEList(ctx context.Context, secuCode string) (eastmoney.HistoricalPEList, error) {
	if p.err != nil {
		return nil, p.err
	}
	if p.peErr != nil {
		return nil, p.peErr
	}
	return p.pe[secuCode], nil
}

func (p fakeFundDailyIndustryPublicDataProvider) QueryProfitPredict(ctx context.Context, secuCode string) (eastmoney.ProfitPredictList, error) {
	if p.err != nil {
		return nil, p.err
	}
	if p.predErr != nil {
		return nil, p.predErr
	}
	return p.predicts[secuCode], nil
}

func TestBuildFundDailyIndustryPublicSignalsComputesValuationAndRevision(t *testing.T) {
	action := FundDailyAction{
		Code:          "300001",
		Name:          "AI growth holding C",
		FundType:      "index equity",
		CurrentAmount: 1200,
		CurrentWeight: 24,
		TopStocks: []FundDailyTopHolding{
			{Code: "600000", Name: "sample stock", HoldRatio: 10},
		},
	}
	report := FundDailyAdviceReport{
		GeneratedAt: time.Date(2026, 6, 20, 10, 0, 0, 0, time.Local),
		Config:      DefaultFundDailyAdviceConfig(),
		DecisionPortfolioActions: []FundDailyAction{
			action,
		},
		InvestableAmount: 5000,
	}
	cache := models.FundDailyIndustrySignalCache{
		Items: map[string]models.FundDailyIndustrySignalCacheItem{
			"600000": {
				Code:      "600000",
				Name:      "sample stock",
				UpdatedAt: "2026-06-19 10:00:00",
				ProfitPredicts: []models.FundDailyIndustryProfitPredictCachePoint{
					{Year: 2026, EPS: 2.0, PE: 20},
				},
			},
		},
	}
	provider := fakeFundDailyIndustryPublicDataProvider{
		pe: map[string]eastmoney.HistoricalPEList{
			"600000.SH": highPercentilePEList(),
		},
		predicts: map[string]eastmoney.ProfitPredictList{
			"600000.SH": {
				{PredictYear: 2026, Eps: 1.8, Pe: 22},
			},
		},
	}

	signals, updatedCache, warnings := buildFundDailyIndustryPublicSignals(context.Background(), report, cache, provider, report.GeneratedAt)

	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if len(updatedCache.Items["600000"].HistoricalPE) == 0 {
		t.Fatalf("expected PE history to be cached")
	}
	if !hasIndustrySignal(signals, "valuation_percentile", "extreme_expensive", "ready") {
		t.Fatalf("expected extreme valuation signal, got %+v", signals)
	}
	if !hasIndustrySignal(signals, "earnings_revision", "downward_revision", "ready") {
		t.Fatalf("expected downward revision signal, got %+v", signals)
	}

	report.IndustryExpectationSignals = signals
	contextData := BuildFundDailyIndustryExpectationContext(report)
	if contextData.Status != "ready" {
		t.Fatalf("expected ready context, got %+v", contextData)
	}
	if contextData.Themes[0].PricingState != FundDailyIndustryPricingNotFullyPriced {
		t.Fatalf("expected not fully priced risk, got %+v", contextData.Themes[0])
	}
}

func TestBuildFundDailyIndustryPublicSignalsCreatesFirstSnapshotWithoutRevision(t *testing.T) {
	action := FundDailyAction{
		Code:     "300001",
		Name:     "AI growth holding C",
		FundType: "index equity",
		TopStocks: []FundDailyTopHolding{
			{Code: "000001", Name: "sample stock", HoldRatio: 8},
		},
	}
	report := FundDailyAdviceReport{
		GeneratedAt: time.Date(2026, 6, 20, 10, 0, 0, 0, time.Local),
		Config:      DefaultFundDailyAdviceConfig(),
		DecisionCandidateActions: []FundDailyAction{
			action,
		},
		InvestableAmount: 5000,
	}
	provider := fakeFundDailyIndustryPublicDataProvider{
		pe: map[string]eastmoney.HistoricalPEList{
			"000001.SZ": highPercentilePEList(),
		},
		predicts: map[string]eastmoney.ProfitPredictList{
			"000001.SZ": {
				{PredictYear: 2026, Eps: 1.2, Pe: 15},
			},
		},
	}

	signals, updatedCache, warnings := buildFundDailyIndustryPublicSignals(
		context.Background(),
		report,
		models.FundDailyIndustrySignalCache{},
		provider,
		report.GeneratedAt,
	)

	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if len(updatedCache.Items["000001"].ProfitPredicts) != 1 {
		t.Fatalf("expected first snapshot to be cached, got %+v", updatedCache.Items["000001"])
	}
	if !hasIndustrySignal(signals, "earnings_revision", "snapshot_only", "partial") {
		t.Fatalf("expected partial snapshot-only earnings signal, got %+v", signals)
	}
}

func TestBuildFundDailyIndustryPublicSignalsKeepsSameDayRevisionStable(t *testing.T) {
	action := FundDailyAction{
		Code:          "300001",
		Name:          "AI growth holding C",
		FundType:      "index equity",
		CurrentWeight: 24,
		TopStocks: []FundDailyTopHolding{
			{Code: "600000", Name: "sample stock", HoldRatio: 10},
		},
	}
	report := FundDailyAdviceReport{
		GeneratedAt: time.Date(2026, 6, 20, 10, 0, 0, 0, time.Local),
		Config:      DefaultFundDailyAdviceConfig(),
		DecisionPortfolioActions: []FundDailyAction{
			action,
		},
		InvestableAmount: 5000,
	}
	cache := models.FundDailyIndustrySignalCache{
		Items: map[string]models.FundDailyIndustrySignalCacheItem{
			"600000": {
				Code:                    "600000",
				Name:                    "sample stock",
				UpdatedAt:               "2026-06-19 10:00:00",
				ProfitPredictsUpdatedAt: "2026-06-19 10:00:00",
				ProfitPredicts: []models.FundDailyIndustryProfitPredictCachePoint{
					{Year: 2026, EPS: 2.0, PE: 20},
				},
			},
		},
	}
	provider := fakeFundDailyIndustryPublicDataProvider{
		pe: map[string]eastmoney.HistoricalPEList{
			"600000.SH": highPercentilePEList(),
		},
		predicts: map[string]eastmoney.ProfitPredictList{
			"600000.SH": {
				{PredictYear: 2026, Eps: 1.8, Pe: 22},
			},
		},
	}

	firstSignals, updatedCache, warnings := buildFundDailyIndustryPublicSignals(context.Background(), report, cache, provider, report.GeneratedAt)
	if len(warnings) != 0 {
		t.Fatalf("expected no first warnings, got %v", warnings)
	}
	secondSignals, _, warnings := buildFundDailyIndustryPublicSignals(
		context.Background(),
		report,
		updatedCache,
		fakeFundDailyIndustryPublicDataProvider{err: errors.New("should not be called when cache is fresh")},
		report.GeneratedAt,
	)
	if len(warnings) != 0 {
		t.Fatalf("expected fresh cache to avoid provider warnings, got %v", warnings)
	}
	if !hasIndustrySignal(firstSignals, "earnings_revision", "downward_revision", "ready") {
		t.Fatalf("expected first run downward revision, got %+v", firstSignals)
	}
	if !hasIndustrySignal(secondSignals, "earnings_revision", "downward_revision", "ready") {
		t.Fatalf("expected same-day second run to preserve downward revision, got %+v", secondSignals)
	}
}

func TestBuildFundDailyIndustryPublicSignalsDoesNotScoreStaleFailedComponent(t *testing.T) {
	action := FundDailyAction{
		Code:     "300001",
		Name:     "AI growth holding C",
		FundType: "index equity",
		TopStocks: []FundDailyTopHolding{
			{Code: "600000", Name: "sample stock", HoldRatio: 8},
		},
	}
	report := FundDailyAdviceReport{
		GeneratedAt: time.Date(2026, 6, 20, 10, 0, 0, 0, time.Local),
		Config:      DefaultFundDailyAdviceConfig(),
		DecisionCandidateActions: []FundDailyAction{
			action,
		},
		InvestableAmount: 5000,
	}
	cache := models.FundDailyIndustrySignalCache{
		Items: map[string]models.FundDailyIndustrySignalCacheItem{
			"600000": {
				Code:                    "600000",
				UpdatedAt:               "2026-06-19 10:00:00",
				ProfitPredictsUpdatedAt: "2026-06-19 10:00:00",
				ProfitPredicts: []models.FundDailyIndustryProfitPredictCachePoint{
					{Year: 2026, EPS: 2.0, PE: 20},
				},
			},
		},
	}
	provider := fakeFundDailyIndustryPublicDataProvider{
		pe: map[string]eastmoney.HistoricalPEList{
			"600000.SH": highPercentilePEList(),
		},
		predErr: errors.New("profit source offline"),
	}

	signals, updatedCache, warnings := buildFundDailyIndustryPublicSignals(context.Background(), report, cache, provider, report.GeneratedAt)

	if len(warnings) == 0 {
		t.Fatalf("expected profit source warning")
	}
	if !hasIndustrySignal(signals, "valuation_percentile", "extreme_expensive", "ready") {
		t.Fatalf("expected fresh valuation signal, got %+v", signals)
	}
	if hasIndustrySignal(signals, "earnings_revision", "stable", "ready") ||
		hasIndustrySignal(signals, "earnings_revision", "downward_revision", "ready") ||
		hasIndustrySignal(signals, "earnings_revision", "upward_revision", "ready") {
		t.Fatalf("expected stale failed earnings component not to be scored ready, got %+v", signals)
	}
	if updatedCache.Items["600000"].ProfitPredictsUpdatedAt == "2026-06-20 10:00:00" {
		t.Fatalf("expected failed profit component not to get today's timestamp, got %+v", updatedCache.Items["600000"])
	}
}

func TestBuildFundDailyIndustryPublicSignalsKeepsCacheOnProviderFailure(t *testing.T) {
	action := FundDailyAction{
		Code:     "300001",
		Name:     "AI growth holding C",
		FundType: "index equity",
		TopStocks: []FundDailyTopHolding{
			{Code: "600000", Name: "sample stock", HoldRatio: 8},
		},
	}
	report := FundDailyAdviceReport{
		GeneratedAt: time.Date(2026, 6, 20, 10, 0, 0, 0, time.Local),
		Config:      DefaultFundDailyAdviceConfig(),
		DecisionCandidateActions: []FundDailyAction{
			action,
		},
		InvestableAmount: 5000,
	}
	cache := models.FundDailyIndustrySignalCache{
		Items: map[string]models.FundDailyIndustrySignalCacheItem{
			"600000": {
				Code:      "600000",
				UpdatedAt: "2026-06-19 10:00:00",
				HistoricalPE: []models.FundDailyIndustryPEPoint{
					{Date: "2026-06-19", PE: 10},
				},
			},
		},
	}

	_, updatedCache, warnings := buildFundDailyIndustryPublicSignals(
		context.Background(),
		report,
		cache,
		fakeFundDailyIndustryPublicDataProvider{err: errors.New("offline")},
		report.GeneratedAt,
	)

	if len(warnings) == 0 {
		t.Fatalf("expected provider warning")
	}
	if updatedCache.Items["600000"].UpdatedAt != "2026-06-19 10:00:00" {
		t.Fatalf("expected stale cache timestamp to be preserved, got %+v", updatedCache.Items["600000"])
	}
}

func highPercentilePEList() eastmoney.HistoricalPEList {
	result := eastmoney.HistoricalPEList{}
	for idx := 1; idx <= 35; idx++ {
		result = append(result, eastmoney.HistoricalPE{
			Date:  "2026-05-" + twoDigit(idx),
			Value: float64(idx),
		})
	}
	result = append(result, eastmoney.HistoricalPE{Date: "2026-06-19", Value: 60})
	return result
}

func twoDigit(value int) string {
	if value < 10 {
		return "0" + string(rune('0'+value))
	}
	return string(rune('0'+value/10)) + string(rune('0'+value%10))
}

func hasIndustrySignal(signals []FundDailyIndustryExpectationSignal, category string, signal string, status string) bool {
	for _, item := range signals {
		if item.Evidence.Category == category && item.Evidence.Signal == signal && item.Evidence.Status == status {
			return true
		}
	}
	return false
}
