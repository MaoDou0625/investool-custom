package core

import (
	"context"
	"testing"

	"github.com/axiaoxin-com/investool/models"
	"github.com/stretchr/testify/require"
)

func TestBuildFund4433RecommendationsRanksCandidates(t *testing.T) {
	ctx := context.Background()
	strong := buildRecommendationFund("000001", "强势候选", 12)
	medium := buildRecommendationFund("000002", "普通候选", 38)
	weak := buildRecommendationFund("000003", "数据不足", 0)

	result := BuildFund4433Recommendations(ctx, models.FundList{medium, weak, strong}, Fund4433RecommendationOptions{
		MaxCount:      2,
		MinRankFields: 3,
	})

	require.Len(t, result, 2)
	require.Equal(t, "000001", result[0].Code)
	require.Equal(t, "000002", result[1].Code)
}

func TestBuildFund4433ListFiltersStrictMatches(t *testing.T) {
	ctx := context.Background()
	strong := buildRecommendationFund("000001", "strict 4433 strong", 12)
	markStrict4433Fund(strong)
	strong.Performance.WeekProfitRatio = 2
	weaker := buildRecommendationFund("000002", "strict 4433 weaker", 18)
	markStrict4433Fund(weaker)
	weaker.Performance.WeekProfitRatio = 1
	excluded := buildRecommendationFund("000003", "not strict 4433", 40)
	markStrict4433Fund(excluded)
	excluded.Performance.Year1RankRatio = 40

	result := BuildFund4433List(ctx, models.FundList{weaker, excluded, strong})

	require.Len(t, result, 2)
	require.Equal(t, "000001", result[0].Code)
	require.Equal(t, "000002", result[1].Code)
}

func buildRecommendationFund(code string, name string, rankRatio float64) *models.Fund {
	fund := &models.Fund{
		Code:               code,
		Name:               name,
		Type:               "混合型",
		EstablishedDate:    "2018-01-01",
		NetAssetsScale:     20 * 100000000,
		SubscriptionStatus: "开放申购",
	}
	if rankRatio > 0 {
		fund.Performance.Month3RankRatio = rankRatio
		fund.Performance.Month6RankRatio = rankRatio
		fund.Performance.Year1RankRatio = rankRatio
		fund.Performance.Year2RankRatio = rankRatio
		fund.Performance.Year3RankRatio = rankRatio
		fund.Performance.ThisYearRankRatio = rankRatio
		fund.Performance.Year1ProfitRatio = 12
		fund.Performance.Month3ProfitRatio = 4
	}
	fund.Sharp.Avg135 = 1.2
	fund.MaxRetracement.Avg135 = 18
	fund.Stddev.Avg135 = 18
	fund.Manager.ManageDays = 5 * 365
	fund.Manager.ManageRepay = 40
	return fund
}

func markStrict4433Fund(fund *models.Fund) {
	fund.Performance.Month3RankRatio = 20
	fund.Performance.Month6RankRatio = 20
	fund.Performance.Year1RankRatio = 20
	fund.Performance.Year2RankRatio = 20
	fund.Performance.Year3RankRatio = 20
	fund.Performance.Year5RankRatio = 20
	fund.Performance.ThisYearRankRatio = 20
	fund.Performance.Year5ProfitRatio = 30
	fund.Performance.Year5RankNum = 10
}
