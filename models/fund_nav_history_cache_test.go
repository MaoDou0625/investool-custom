package models

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFundNAVHistoryCacheStoreSaveAndLoad(t *testing.T) {
	store := NewFundNAVHistoryCacheStore(filepath.Join(t.TempDir(), "fund_nav_history_cache.json"))

	err := store.Save(FundNAVHistoryCache{
		Items: map[string]FundNAVHistoryCacheItem{
			"002207": {
				Code:      "002207",
				Name:      "前海开源金银珠宝混合C",
				UpdatedAt: "2026-05-26 20:00:00",
				Points: []FundNAVHistoryCachePoint{
					{Date: "2026-05-26", UnitNAV: 2.8020},
					{Date: "2026-05-24", UnitNAV: 0},
					{Date: "2026-05-25", UnitNAV: 2.7040},
				},
			},
		},
	})
	require.NoError(t, err)

	cache, err := store.Load()

	require.NoError(t, err)
	require.Len(t, cache.Items, 1)
	item := cache.Items["002207"]
	require.Equal(t, "前海开源金银珠宝混合C", item.Name)
	require.Len(t, item.Points, 2)
	require.Equal(t, "2026-05-25", item.Points[0].Date)
	require.InDelta(t, 2.7040, item.Points[0].UnitNAV, 0.00001)
	require.Equal(t, "2026-05-26", item.Points[1].Date)
}

func TestFundNAVHistoryCacheStoreRejectsEmptyFilename(t *testing.T) {
	store := NewFundNAVHistoryCacheStore("")

	err := store.Save(FundNAVHistoryCache{})

	require.Error(t, err)
}
