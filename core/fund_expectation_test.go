package core

import (
	"testing"

	"github.com/axiaoxin-com/investool/models"
	"github.com/stretchr/testify/require"
)

func TestEstimateFundAnnualReturnUsesMultiPeriodHistory(t *testing.T) {
	fund := &models.Fund{}
	fund.Performance.Year1ProfitRatio = 12
	fund.Performance.Year2ProfitRatio = 25
	fund.Performance.Year3ProfitRatio = 40
	fund.Performance.Year5ProfitRatio = 80
	fund.Sharp.Avg135 = 1.3

	estimate, ok := EstimateFundAnnualReturn(fund)

	require.True(t, ok)
	require.Greater(t, estimate, 10.0)
	require.LessOrEqual(t, estimate, 35.0)
}

func TestRecommendFundWeightCapsHighRiskFund(t *testing.T) {
	weight := RecommendFundWeight(90, "偏高", 25, true)

	require.Equal(t, 8.0, weight)
}
