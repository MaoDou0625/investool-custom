package models

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFundPortfolioHistoryStoreUpsertSnapshot(t *testing.T) {
	store := NewFundPortfolioHistoryStore(filepath.Join(t.TempDir(), "fund_portfolio_history.json"))

	require.NoError(t, store.UpsertSnapshot(FundPortfolioSnapshot{
		Date:               "2026-05-25",
		TotalCurrentAmount: 100,
		ProfitRatio:        1.5,
	}))
	require.NoError(t, store.UpsertSnapshot(FundPortfolioSnapshot{
		Date:               "2026-05-26",
		TotalCurrentAmount: 120,
		ProfitRatio:        2.5,
	}))
	require.NoError(t, store.UpsertSnapshot(FundPortfolioSnapshot{
		Date:               "2026-05-25",
		TotalCurrentAmount: 130,
		ProfitRatio:        3.5,
	}))

	history, err := store.Load()

	require.NoError(t, err)
	require.Len(t, history.Snapshots, 2)
	require.Equal(t, "2026-05-25", history.Snapshots[0].Date)
	require.InDelta(t, 130, history.Snapshots[0].TotalCurrentAmount, 0.01)
	require.Equal(t, "2026-05-26", history.Snapshots[1].Date)
}

func TestFundPortfolioHistoryStoreRejectsEmptyDate(t *testing.T) {
	store := NewFundPortfolioHistoryStore(filepath.Join(t.TempDir(), "fund_portfolio_history.json"))

	err := store.UpsertSnapshot(FundPortfolioSnapshot{})

	require.Error(t, err)
}
