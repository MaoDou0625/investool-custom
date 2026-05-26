package routes

import (
	"testing"
	"time"

	"github.com/axiaoxin-com/investool/core"
	"github.com/axiaoxin-com/investool/models"
	"github.com/stretchr/testify/require"
)

func TestBuildFundPortfolioSnapshot(t *testing.T) {
	now := time.Date(2026, 5, 26, 10, 30, 0, 0, time.Local)
	advices := []core.FundPortfolioAdvice{
		{
			Item:          models.FundPortfolioItem{Code: "002207", Status: models.FundPortfolioStatusOwned},
			Fund:          &models.Fund{Name: "前海开源金银珠宝混合C", UnitNav: 2.7},
			HasPosition:   true,
			CurrentAmount: 1200,
			CostAmount:    1300,
			ProfitAmount:  -100,
			ProfitRatio:   -7.69,
			CurrentWeight: 80,
		},
		{
			Item:          models.FundPortfolioItem{Code: "017057", Status: models.FundPortfolioStatusWatch},
			HasPosition:   true,
			CurrentAmount: 300,
			CostAmount:    280,
		},
	}

	snapshot, ok := buildFundPortfolioSnapshot(advices, now)

	require.True(t, ok)
	require.Equal(t, "2026-05-26", snapshot.Date)
	require.Len(t, snapshot.Items, 1)
	require.InDelta(t, 1200, snapshot.TotalCurrentAmount, 0.01)
	require.InDelta(t, 1300, snapshot.CostAmount, 0.01)
	require.InDelta(t, -100, snapshot.ProfitAmount, 0.01)
	require.InDelta(t, -7.69, snapshot.ProfitRatio, 0.01)
	require.Equal(t, "前海开源金银珠宝混合C", snapshot.Items[0].Name)
}
