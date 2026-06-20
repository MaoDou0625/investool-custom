package core

import (
	"context"
	"time"

	"github.com/axiaoxin-com/investool/datacenter/eastmoney"
	"github.com/axiaoxin-com/investool/models"
)

const (
	fundDailyIndustryPublicSignalProvider   = "eastmoney-public-industry-signals"
	fundDailyIndustryPublicSignalTimeout    = 20 * time.Second
	fundDailyIndustryPublicSignalMaxStocks  = 6
	fundDailyIndustryPublicSignalMinPECount = 30
)

type fundDailyIndustryPublicDataProvider interface {
	QueryHistoricalPEList(ctx context.Context, secuCode string) (eastmoney.HistoricalPEList, error)
	QueryProfitPredict(ctx context.Context, secuCode string) (eastmoney.ProfitPredictList, error)
}

type liveFundDailyIndustryPublicDataProvider struct {
	eastMoney eastmoney.EastMoney
}

type fundDailyIndustryStockTarget struct {
	Code   string
	Name   string
	Theme  string
	Weight float64
}

type fundDailyIndustryPublicThemeAgg struct {
	Theme                  string
	TargetCount            int
	ValuationCount         int
	ValuationWeight        float64
	ValuationPercentile    float64
	EarningsRevisionCount  int
	EarningsRevisionWeight float64
	EarningsRevision       float64
	EarningsSnapshotCount  int
	MissingValuationCount  int
	MissingEarningsCount   int
	StockNames             []string
}

func (p liveFundDailyIndustryPublicDataProvider) QueryHistoricalPEList(ctx context.Context, secuCode string) (eastmoney.HistoricalPEList, error) {
	return p.eastMoney.QueryHistoricalPEList(ctx, secuCode)
}

func (p liveFundDailyIndustryPublicDataProvider) QueryProfitPredict(ctx context.Context, secuCode string) (eastmoney.ProfitPredictList, error) {
	return p.eastMoney.QueryProfitPredict(ctx, secuCode)
}

func BuildFundDailyIndustryPublicSignals(
	ctx context.Context,
	report FundDailyAdviceReport,
	cache models.FundDailyIndustrySignalCache,
) ([]FundDailyIndustryExpectationSignal, models.FundDailyIndustrySignalCache, []string) {
	queryCtx, cancel := context.WithTimeout(ctx, fundDailyIndustryPublicSignalTimeout)
	defer cancel()

	return buildFundDailyIndustryPublicSignals(
		queryCtx,
		report,
		cache,
		liveFundDailyIndustryPublicDataProvider{eastMoney: eastmoney.NewEastMoney()},
		time.Now(),
	)
}

func buildFundDailyIndustryPublicSignals(
	ctx context.Context,
	report FundDailyAdviceReport,
	cache models.FundDailyIndustrySignalCache,
	provider fundDailyIndustryPublicDataProvider,
	now time.Time,
) ([]FundDailyIndustryExpectationSignal, models.FundDailyIndustrySignalCache, []string) {
	if cache.Items == nil {
		cache.Items = map[string]models.FundDailyIndustrySignalCacheItem{}
	}
	warnings := []string{}
	targets := fundDailyIndustryStockTargets(report)
	if len(targets) == 0 {
		return nil, cache, nil
	}

	selectedCodes := fundDailyIndustrySelectedStockCodes(targets, fundDailyIndustryPublicSignalMaxStocks)
	selected := map[string]bool{}
	for _, code := range selectedCodes {
		selected[code] = true
	}

	aggs := map[string]*fundDailyIndustryPublicThemeAgg{}
	for _, target := range targets {
		if !selected[target.Code] {
			continue
		}
		scoreItem, cacheItem, previousPredicts, previousUpdatedAt, hadPrevious, itemWarnings := fundDailyIndustrySignalCacheItem(ctx, provider, cache, target, now)
		warnings = append(warnings, itemWarnings...)
		cache.Items[target.Code] = cacheItem

		agg := fundDailyIndustryPublicThemeAggFor(aggs, target.Theme)
		agg.TargetCount++
		agg.StockNames = append(agg.StockNames, target.Name)

		weight := target.Weight
		if weight <= 0 {
			weight = 1
		}
		if percentile, _, _, ok := fundDailyIndustryPEPercentile(scoreItem.HistoricalPE); ok {
			agg.ValuationCount++
			agg.ValuationWeight += weight
			agg.ValuationPercentile += percentile * weight
		} else {
			agg.MissingValuationCount++
		}

		revision, status := fundDailyIndustryEarningsRevision(scoreItem.ProfitPredicts, previousPredicts, previousUpdatedAt, hadPrevious, now)
		switch status {
		case "ready":
			agg.EarningsRevisionCount++
			agg.EarningsRevisionWeight += weight
			agg.EarningsRevision += revision * weight
		case "partial":
			agg.EarningsSnapshotCount++
		default:
			agg.MissingEarningsCount++
		}
	}

	cache.UpdatedAt = now.Format("2006-01-02 15:04:05")
	return fundDailyIndustryPublicSignalsFromAggs(aggs), cache, compactDailyReasons(warnings, 6)
}
