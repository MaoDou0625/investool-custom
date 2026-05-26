package routes

import (
	"testing"
	"time"

	"github.com/axiaoxin-com/investool/models"
	"github.com/stretchr/testify/require"
)

func TestBuildFundPortfolioCorrelationRefreshDataDetectsMissingAndStaleCache(t *testing.T) {
	now := time.Date(2026, 5, 26, 10, 0, 0, 0, time.Local)
	funds := []fundPortfolioCorrelationFund{
		{Code: "002207", Name: "前海开源金银珠宝混合C"},
		{Code: "000001", Name: "候选基金"},
	}
	cache := models.FundNAVHistoryCache{
		Items: map[string]models.FundNAVHistoryCacheItem{
			"002207": {
				Code:      "002207",
				Name:      "前海开源金银珠宝混合C",
				UpdatedAt: "2026-05-25 20:00:00",
				Points:    buildCacheNAVPoints(1.00, 1.01, 1.03, 1.02, 1.04, 1.05, 1.07, 1.06, 1.08),
			},
		},
	}

	refresh := buildFundPortfolioCorrelationRefreshData(funds, cache, now)

	require.True(t, refresh.Needed)
	require.Equal(t, 1, refresh.Missing)
	require.Equal(t, 1, refresh.Stale)
	require.Len(t, refresh.Funds, 2)
}

func TestSourcesFromFundNAVCacheUsesUsableCachedHistory(t *testing.T) {
	funds := []fundPortfolioCorrelationFund{{Code: "002207", Name: "前海开源金银珠宝混合C"}}
	cache := models.FundNAVHistoryCache{
		Items: map[string]models.FundNAVHistoryCacheItem{
			"002207": {
				Code:      "002207",
				Name:      "前海开源金银珠宝混合C",
				UpdatedAt: "2026-05-26 20:00:00",
				Points:    buildCacheNAVPoints(1.00, 1.01, 1.03, 1.02, 1.04, 1.05, 1.07, 1.06, 1.08),
			},
		},
	}

	sources := sourcesFromFundNAVCache(funds, cache)

	require.Len(t, sources, 1)
	require.Equal(t, "002207", sources[0].Code)
	require.Len(t, sources[0].History, 9)
}

func buildCacheNAVPoints(unitNAVs ...float64) []models.FundNAVHistoryCachePoint {
	dates := []string{
		"2026-05-14",
		"2026-05-15",
		"2026-05-18",
		"2026-05-19",
		"2026-05-20",
		"2026-05-21",
		"2026-05-22",
		"2026-05-25",
		"2026-05-26",
	}
	points := make([]models.FundNAVHistoryCachePoint, 0, len(unitNAVs))
	for idx, unitNAV := range unitNAVs {
		points = append(points, models.FundNAVHistoryCachePoint{
			Date:    dates[idx],
			UnitNAV: unitNAV,
		})
	}
	return points
}
