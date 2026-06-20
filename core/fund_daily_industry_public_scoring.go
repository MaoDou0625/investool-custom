package core

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/axiaoxin-com/investool/models"
)

func fundDailyIndustryPublicThemeAggFor(aggs map[string]*fundDailyIndustryPublicThemeAgg, theme string) *fundDailyIndustryPublicThemeAgg {
	agg := aggs[theme]
	if agg == nil {
		agg = &fundDailyIndustryPublicThemeAgg{Theme: theme}
		aggs[theme] = agg
	}
	return agg
}

func fundDailyIndustryPublicSignalsFromAggs(aggs map[string]*fundDailyIndustryPublicThemeAgg) []FundDailyIndustryExpectationSignal {
	signals := []FundDailyIndustryExpectationSignal{}
	for _, agg := range aggs {
		if agg.TargetCount == 0 {
			continue
		}
		signals = append(signals, fundDailyIndustryValuationSignal(*agg))
		signals = append(signals, fundDailyIndustryEarningsSignal(*agg))
	}
	sort.SliceStable(signals, func(i, j int) bool {
		if signals[i].Theme != signals[j].Theme {
			return signals[i].Theme < signals[j].Theme
		}
		return signals[i].Evidence.Category < signals[j].Evidence.Category
	})
	return signals
}

func fundDailyIndustryValuationSignal(agg fundDailyIndustryPublicThemeAgg) FundDailyIndustryExpectationSignal {
	if agg.ValuationCount == 0 || agg.ValuationWeight <= 0 {
		return FundDailyIndustryExpectationSignal{
			Theme: agg.Theme,
			Evidence: FundDailyIndustryExpectationEvidence{
				Category: "valuation_percentile",
				Status:   "missing",
				Signal:   "missing",
				Reason:   fmt.Sprintf("%s 暂未从免费公开源取得足够历史 PE-TTM 样本。", agg.Theme),
			},
		}
	}
	percentile := agg.ValuationPercentile / agg.ValuationWeight
	signal := "fair"
	switch {
	case percentile >= 90:
		signal = "extreme_expensive"
	case percentile >= 75:
		signal = "expensive"
	case percentile >= 60:
		signal = "elevated"
	case percentile <= 25:
		signal = "cheap"
	}
	return FundDailyIndustryExpectationSignal{
		Theme: agg.Theme,
		Evidence: FundDailyIndustryExpectationEvidence{
			Category: "valuation_percentile",
			Status:   "ready",
			Signal:   signal,
			Value:    roundFloat(percentile, 1),
			Weight:   1,
			Reason: fmt.Sprintf(
				"%s 公开源重仓股 PE-TTM 估值分位约 %.0f%%，覆盖 %d/%d 个样本股，代表股票：%s。",
				agg.Theme,
				percentile,
				agg.ValuationCount,
				agg.TargetCount,
				strings.Join(compactUniqueDailyStrings(agg.StockNames, 3), "、"),
			),
		},
	}
}

func fundDailyIndustryEarningsSignal(agg fundDailyIndustryPublicThemeAgg) FundDailyIndustryExpectationSignal {
	if agg.EarningsRevisionCount == 0 || agg.EarningsRevisionWeight <= 0 {
		status := "missing"
		signal := "missing"
		reason := fmt.Sprintf("%s 暂未从免费公开源取得可比较的盈利预测修正样本。", agg.Theme)
		if agg.EarningsSnapshotCount > 0 {
			status = "partial"
			signal = "snapshot_only"
			reason = fmt.Sprintf("%s 已建立 %d 个重仓股盈利预测快照；需要后续交易日缓存对比后才能判断上修或下修。", agg.Theme, agg.EarningsSnapshotCount)
		}
		return FundDailyIndustryExpectationSignal{
			Theme: agg.Theme,
			Evidence: FundDailyIndustryExpectationEvidence{
				Category: "earnings_revision",
				Status:   status,
				Signal:   signal,
				Reason:   reason,
			},
		}
	}
	revision := agg.EarningsRevision / agg.EarningsRevisionWeight
	signal := "stable"
	switch {
	case revision >= 5:
		signal = "upward_revision"
	case revision <= -5:
		signal = "downward_revision"
	}
	return FundDailyIndustryExpectationSignal{
		Theme: agg.Theme,
		Evidence: FundDailyIndustryExpectationEvidence{
			Category: "earnings_revision",
			Status:   "ready",
			Signal:   signal,
			Value:    roundFloat(revision, 1),
			Weight:   1,
			Reason: fmt.Sprintf(
				"%s 公开源重仓股盈利预测较上一缓存快照变化约 %.1f%%，覆盖 %d/%d 个样本股。",
				agg.Theme,
				revision,
				agg.EarningsRevisionCount,
				agg.TargetCount,
			),
		},
	}
}

func fundDailyIndustryPEPercentile(points []models.FundDailyIndustryPEPoint) (float64, float64, string, bool) {
	values := []float64{}
	latestDate := ""
	latestPE := 0.0
	for _, point := range points {
		if point.PE <= 0 || strings.TrimSpace(point.Date) == "" {
			continue
		}
		values = append(values, point.PE)
		if point.Date >= latestDate {
			latestDate = point.Date
			latestPE = point.PE
		}
	}
	if len(values) < fundDailyIndustryPublicSignalMinPECount || latestPE <= 0 {
		return 0, 0, "", false
	}
	lessOrEqual := 0
	for _, value := range values {
		if value <= latestPE {
			lessOrEqual++
		}
	}
	return float64(lessOrEqual) / float64(len(values)) * 100, latestPE, latestDate, true
}

func fundDailyIndustryEarningsRevision(
	current []models.FundDailyIndustryProfitPredictCachePoint,
	previous []models.FundDailyIndustryProfitPredictCachePoint,
	previousUpdatedAt string,
	hadPrevious bool,
	now time.Time,
) (float64, string) {
	year, currentEPS, ok := fundDailyIndustryCurrentEPS(current, now)
	if !ok {
		return 0, "missing"
	}
	if !hadPrevious || fundDailyIndustryCacheDate(previousUpdatedAt) == now.Format("2006-01-02") {
		return 0, "partial"
	}
	_, previousEPS, ok := fundDailyIndustryEPSForYear(previous, year)
	if !ok || previousEPS == 0 {
		return 0, "partial"
	}
	denominator := math.Abs(previousEPS)
	if denominator < 0.01 {
		denominator = 0.01
	}
	return (currentEPS - previousEPS) / denominator * 100, "ready"
}

func fundDailyIndustryCurrentEPS(points []models.FundDailyIndustryProfitPredictCachePoint, now time.Time) (int, float64, bool) {
	for _, point := range points {
		if point.Year >= now.Year() && point.EPS != 0 {
			return point.Year, point.EPS, true
		}
	}
	for _, point := range points {
		if point.EPS != 0 {
			return point.Year, point.EPS, true
		}
	}
	return 0, 0, false
}

func fundDailyIndustryEPSForYear(points []models.FundDailyIndustryProfitPredictCachePoint, year int) (int, float64, bool) {
	for _, point := range points {
		if point.Year == year && point.EPS != 0 {
			return point.Year, point.EPS, true
		}
	}
	return 0, 0, false
}

func fundDailyIndustryCacheDate(updatedAt string) string {
	parsed, err := time.ParseInLocation("2006-01-02 15:04:05", updatedAt, time.Local)
	if err != nil {
		return ""
	}
	return parsed.Format("2006-01-02")
}

func compactUniqueDailyStrings(items []string, max int) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		result = append(result, item)
		if max > 0 && len(result) >= max {
			break
		}
	}
	return result
}
