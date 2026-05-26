package models

import (
	"os"
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
		CurrentAmount: 2200,
		TargetWeight:  20,
		Note:          "core holding",
	})
	require.NoError(t, err)

	portfolio, err := store.Load()
	require.NoError(t, err)
	require.Len(t, portfolio.Items, 1)
	require.Equal(t, FundPortfolioStatusOwned, portfolio.Items[0].Status)
	require.Equal(t, "已持有", portfolio.Items[0].StatusName())
	require.Equal(t, float64(2200), portfolio.Items[0].CurrentAmount)
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

func TestFundPortfolioStoreMigratesLegacyHoldingAmount(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "fund_portfolio.json")
	err := os.WriteFile(filename, append([]byte{0xEF, 0xBB, 0xBF}, []byte(`{
		"items": [
			{
				"code": "260104",
				"status": "owned",
				"cost_nav": 2.1,
				"holding_shares": 1000,
				"holding_amount": 2200,
				"target_weight": 20
			}
		]
	}`)...), 0644)
	require.NoError(t, err)

	store := NewFundPortfolioStore(filename)
	portfolio, err := store.Load()

	require.NoError(t, err)
	require.Len(t, portfolio.Items, 1)
	require.Equal(t, float64(2200), portfolio.Items[0].CurrentAmount)
	require.Zero(t, portfolio.Items[0].HoldingAmount)
}
