package core

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/axiaoxin-com/investool/models"
	"github.com/stretchr/testify/require"
)

func TestSelectDailyAdviceCandidatesWithStrategyUsesAllFundTrendData(t *testing.T) {
	ctx := context.Background()
	trendFund := buildRecommendationFund("100001", "AI趋势基金C", 0)
	trendFund.Performance.Month1ProfitRatio = 7
	trendFund.Performance.Month3ProfitRatio = 16
	trendFund.Performance.Month6ProfitRatio = 22
	trendFund.Performance.ThisYearProfitRatio = 18
	trendFund.Performance.Month3RankRatio = 0
	trendFund.Performance.Month6RankRatio = 0
	trendFund.Performance.Year1RankRatio = 0
	trendFund.Performance.Year2RankRatio = 0
	trendFund.Performance.Year3RankRatio = 0
	trendFund.Performance.ThisYearRankRatio = 0
	fallback4433 := buildRecommendationFund("200001", "兜底4433基金", 12)

	selection := SelectDailyAdviceCandidatesWithStrategy(
		ctx,
		models.FundList{trendFund},
		models.FundList{fallback4433},
		nil,
		models.FundNAVHistoryCache{},
		10,
	)

	require.Len(t, selection.Funds, 1)
	require.Equal(t, "100001", selection.Funds[0].Code)
	require.Contains(t, selection.SourceName, "全量")
	require.Greater(t, selection.Evidence["100001"].StrategyScore, 0.0)
	require.True(t, evidenceHasReason(selection.Evidence["100001"], "4433 不是硬门槛"))
}

func TestSelectDailyAdviceCandidatesWithStrategyUsesNAVCorrelation(t *testing.T) {
	ctx := context.Background()
	owned := buildRecommendationFund("900001", "AI持仓基金", 15)
	owned.Type = "混合型"
	candidate := buildRecommendationFund("100002", "黄金分散基金C", 15)
	candidate.Performance.Month1ProfitRatio = 4
	candidate.Performance.Month3ProfitRatio = 10
	candidate.Performance.Month6ProfitRatio = 15

	cache := models.FundNAVHistoryCache{Items: map[string]models.FundNAVHistoryCacheItem{
		"900001": {Code: "900001", Points: fundDailyTestNAVPoints([]float64{0.010, -0.004, 0.012, -0.006, 0.008, 0.003, -0.005, 0.011, -0.002, 0.006, -0.004, 0.009})},
		"100002": {Code: "100002", Points: fundDailyTestNAVPoints([]float64{-0.006, 0.008, -0.005, 0.010, -0.004, -0.002, 0.007, -0.006, 0.004, -0.003, 0.006, -0.005})},
	}}
	portfolio := []FundPortfolioAdvice{{
		Item:          models.FundPortfolioItem{Code: "900001", Status: models.FundPortfolioStatusOwned},
		Fund:          owned,
		CurrentAmount: 1000,
		CurrentWeight: 100,
	}}

	selection := SelectDailyAdviceCandidatesWithStrategy(
		ctx,
		models.FundList{candidate},
		nil,
		portfolio,
		cache,
		10,
	)

	require.Len(t, selection.Funds, 1)
	evidence := selection.Evidence["100002"]
	require.True(t, evidence.HasCorrelation)
	require.Less(t, evidence.MaxCorrelation, 0.5)
	require.True(t, evidenceHasReason(evidence, "相关性"))
}

func TestBuildFundDailyAdviceReportWithEvidenceSortsCandidatesByStrategyScore(t *testing.T) {
	ctx := context.Background()
	low := buildRecommendationFund("300001", "Alpha基金", 12)
	high := buildRecommendationFund("300002", "Beta基金", 12)

	report := BuildFundDailyAdviceReportWithEvidence(ctx, nil, models.FundList{low, high}, map[string]FundDailyCandidateEvidence{
		"300001": {Code: "300001", Theme: "均衡", StrategyScore: 60, Reasons: []string{"数据评分：60"}},
		"300002": {Code: "300002", Theme: "科技", StrategyScore: 90, Reasons: []string{"数据评分：90"}},
	}, FundDailyAdviceConfig{
		TargetAnnualReturn:    5,
		MaxTotalAmount:        5000,
		CashBufferWeight:      10,
		MaxSingleFundWeight:   35,
		TacticalWeight:        20,
		CandidateCount:        2,
		MinCandidateScore:     1,
		MinCoreCandidateScore: 75,
		Now:                   time.Date(2026, 6, 15, 11, 0, 0, 0, time.Local),
	})

	require.Len(t, report.CandidateActions, 2)
	require.Equal(t, "300002", report.CandidateActions[0].Code)
	require.Equal(t, 90.0, report.CandidateActions[0].StrategyScore)
	require.True(t, strings.HasPrefix(report.CandidateActions[0].Reasons[0], "数据评分"))
}

func evidenceHasReason(evidence FundDailyCandidateEvidence, needle string) bool {
	for _, reason := range evidence.Reasons {
		if strings.Contains(reason, needle) {
			return true
		}
	}
	return false
}

func fundDailyTestNAVPoints(returns []float64) []models.FundNAVHistoryCachePoint {
	points := make([]models.FundNAVHistoryCachePoint, 0, len(returns)+1)
	nav := 1.0
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	points = append(points, models.FundNAVHistoryCachePoint{
		Date:    start.Format("2006-01-02"),
		UnitNAV: nav,
	})
	for idx, ret := range returns {
		nav *= 1 + ret
		points = append(points, models.FundNAVHistoryCachePoint{
			Date:    start.AddDate(0, 0, idx+1).Format("2006-01-02"),
			UnitNAV: nav,
		})
	}
	return points
}
