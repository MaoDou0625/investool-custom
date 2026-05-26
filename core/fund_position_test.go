package core

import (
	"testing"

	"github.com/axiaoxin-com/investool/models"
	"github.com/stretchr/testify/require"
)

func TestCalculateFundPositionMetricsUsesInputCurrentAmount(t *testing.T) {
	fund := &models.Fund{UnitNav: 2.7}
	item := models.FundPortfolioItem{
		Status:        models.FundPortfolioStatusOwned,
		CostNav:       2.5,
		HoldingShares: 100,
		CurrentAmount: 275,
	}

	metrics, ok, warnings := CalculateFundPositionMetrics(item, fund)

	require.True(t, ok)
	require.Empty(t, warnings)
	require.InDelta(t, 250, metrics.CostAmount, 0.01)
	require.InDelta(t, 275, metrics.CurrentAmount, 0.01)
	require.InDelta(t, 10, metrics.ProfitRatio, 0.01)
	require.InDelta(t, 25, metrics.ProfitAmount, 0.01)
	require.Equal(t, positionCurrentAmountSourceInput, metrics.CurrentSource)
}

func TestCalculateFundPositionMetricsFallsBackToNavValue(t *testing.T) {
	fund := &models.Fund{UnitNav: 2.7}
	item := models.FundPortfolioItem{
		Status:        models.FundPortfolioStatusOwned,
		CostNav:       2.5,
		HoldingShares: 100,
	}

	metrics, ok, warnings := CalculateFundPositionMetrics(item, fund)

	require.True(t, ok)
	require.NotEmpty(t, warnings)
	require.InDelta(t, 250, metrics.CostAmount, 0.01)
	require.InDelta(t, 270, metrics.CurrentAmount, 0.01)
	require.InDelta(t, 8, metrics.ProfitRatio, 0.01)
	require.InDelta(t, 20, metrics.ProfitAmount, 0.01)
	require.Equal(t, positionCurrentAmountSourceNAV, metrics.CurrentSource)
}

func TestCalculateFundPositionMetricsRequiresCostAndShares(t *testing.T) {
	fund := &models.Fund{UnitNav: 2.7}
	item := models.FundPortfolioItem{
		Status:        models.FundPortfolioStatusOwned,
		CostNav:       2.5,
		CurrentAmount: 275,
	}

	_, ok, warnings := CalculateFundPositionMetrics(item, fund)

	require.False(t, ok)
	require.NotEmpty(t, warnings)
}
