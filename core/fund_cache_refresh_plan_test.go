package core

import (
	"context"
	"testing"
	"time"

	"github.com/axiaoxin-com/investool/models"
	"github.com/stretchr/testify/require"
)

func TestBuildFundCacheRefreshPlanPrioritizesStale4433(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.Local)
	stale4433 := buildRecommendationFund("000001", "stale 4433", 10)
	markStrict4433Fund(stale4433)
	fresh4433 := buildRecommendationFund("000002", "fresh 4433", 10)
	markStrict4433Fund(fresh4433)
	staleOther := buildRecommendationFund("000003", "stale other", 40)
	meta := models.NewFundDetailRefreshMeta()
	meta.Touch("000001", now.Add(-8*24*time.Hour), "seed")
	meta.Touch("000002", now.Add(-2*24*time.Hour), "seed")
	meta.Touch("000003", now.Add(-31*24*time.Hour), "seed")

	plan := BuildFundCacheRefreshPlan(
		ctx,
		[]string{"000004", "000001", "000002", "000003"},
		models.FundList{stale4433, fresh4433, staleOther},
		models.FundList{},
		meta,
		FundCacheRefreshOptions{
			Mode:                   FundCacheRefreshModePriority,
			MaxFunds:               2,
			Priority4433StaleAfter: 7 * 24 * time.Hour,
			Non4433StaleAfter:      30 * 24 * time.Hour,
			Now:                    now,
		},
	)

	require.Equal(t, []string{"000001", "000004"}, plan.Codes)
	require.Equal(t, FundCacheRefreshPriority4433, plan.Priorities["000001"])
	require.Equal(t, FundCacheRefreshPriorityMissing, plan.Priorities["000004"])
	require.Equal(t, 1, plan.Priority4433Count)
	require.Equal(t, 1, plan.MissingCount)
	require.Equal(t, 1, plan.DeferredCount)
	require.Equal(t, 1, plan.SkippedFreshCount)
}

func TestBuildFundCacheRefreshPlanFullKeepsAllCodes(t *testing.T) {
	plan := BuildFundCacheRefreshPlan(
		context.Background(),
		[]string{"000001", "bad", "000002", "000001"},
		nil,
		nil,
		models.NewFundDetailRefreshMeta(),
		FundCacheRefreshOptions{Mode: FundCacheRefreshModeFull, MaxFunds: 1},
	)

	require.Equal(t, []string{"000001", "000002"}, plan.Codes)
	require.Equal(t, FundCacheRefreshPriorityFull, plan.Priorities["000001"])
	require.Equal(t, FundCacheRefreshPriorityFull, plan.Priorities["000002"])
}

func TestMergeFundCacheRefreshFundsKeepsUntouchedCachedFunds(t *testing.T) {
	oldFund := &models.Fund{Code: "000001", Name: "old"}
	keptFund := &models.Fund{Code: "000002", Name: "kept"}
	updatedFund := &models.Fund{Code: "000001", Name: "new"}

	merged := mergeFundCacheRefreshFunds(models.FundList{oldFund, keptFund}, map[string]*models.Fund{
		"000001": updatedFund,
	})
	fundlist, _ := buildFundCacheRefreshLists([]string{"000001", "000002"}, merged)

	require.Len(t, fundlist, 2)
	require.Equal(t, "new", fundlist[0].Name)
	require.Equal(t, "kept", fundlist[1].Name)
}
