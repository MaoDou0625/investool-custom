package models

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFundPortfolioStoreUpsertAndDelete(t *testing.T) {
	store := NewFundPortfolioStore(filepath.Join(t.TempDir(), "fund_portfolio.json"))

	err := store.Upsert(FundPortfolioItem{
		Code:          "260104",
		Status:        FundPortfolioStatusOwned,
		CostNav:       2.1,
		HoldingShares: 1000,
		TargetWeight:  20,
		Note:          "core holding",
	})
	require.NoError(t, err)

	portfolio, err := store.Load()
	require.NoError(t, err)
	require.Len(t, portfolio.Items, 1)
	require.Equal(t, FundPortfolioStatusOwned, portfolio.Items[0].Status)
	require.Equal(t, "已持有", portfolio.Items[0].StatusName())
	require.NotEmpty(t, portfolio.Items[0].CreatedAt)
	require.NotEmpty(t, portfolio.Items[0].UpdatedAt)

	err = store.Upsert(FundPortfolioItem{
		Code:   "260104",
		Status: FundPortfolioStatusWatch,
		Note:   "watch again",
	})
	require.NoError(t, err)

	portfolio, err = store.Load()
	require.NoError(t, err)
	require.Len(t, portfolio.Items, 1)
	require.Equal(t, FundPortfolioStatusWatch, portfolio.Items[0].Status)
	require.Equal(t, "watch again", portfolio.Items[0].Note)

	err = store.Delete("260104")
	require.NoError(t, err)

	portfolio, err = store.Load()
	require.NoError(t, err)
	require.Empty(t, portfolio.Items)
}

func TestFundPortfolioItemNormalizeRejectsInvalidCode(t *testing.T) {
	store := NewFundPortfolioStore(filepath.Join(t.TempDir(), "fund_portfolio.json"))
	err := store.Upsert(FundPortfolioItem{Code: "abc", Status: FundPortfolioStatusOwned})
	require.Error(t, err)
}
