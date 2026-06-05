package core

import (
	"context"
	"sort"
	"time"

	"github.com/axiaoxin-com/investool/models"
)

const (
	FundCacheRefreshModePriority = "priority"
	FundCacheRefreshModeFull     = "full"

	FundCacheRefreshPriority4433       = "4433_stale"
	FundCacheRefreshPriorityMissing    = "missing"
	FundCacheRefreshPriorityStaleOther = "stale_other"
	FundCacheRefreshPriorityFull       = "full"
)

type FundCacheRefreshPlan struct {
	Codes             []string
	Priorities        map[string]string
	Priority4433Count int
	MissingCount      int
	StaleOtherCount   int
	DeferredCount     int
	SkippedFreshCount int
}

type fundCacheRefreshPlanCandidate struct {
	code      string
	priority  int
	kind      string
	updatedAt time.Time
	position  int
}

func BuildFundCacheRefreshPlan(
	ctx context.Context,
	rawCodes []string,
	cachedFunds models.FundList,
	current4433Funds models.FundList,
	meta models.FundDetailRefreshMeta,
	options FundCacheRefreshOptions,
) FundCacheRefreshPlan {
	options = normalizeFundCacheRefreshOptions(options)
	plan := FundCacheRefreshPlan{Priorities: map[string]string{}}
	codePositions := fundCacheRefreshCodePositions(rawCodes)
	cachedByCode := fundCacheRefreshFundMap(cachedFunds)
	strict4433Codes := fundCacheRefreshStrict4433Codes(ctx, cachedFunds, current4433Funds)

	if options.Mode == FundCacheRefreshModeFull {
		for _, code := range rawCodes {
			if !fundCacheRefreshCodeRegexp.MatchString(code) {
				continue
			}
			if _, exists := plan.Priorities[code]; exists {
				continue
			}
			plan.Codes = append(plan.Codes, code)
			plan.Priorities[code] = FundCacheRefreshPriorityFull
		}
		return plan
	}

	candidates := []fundCacheRefreshPlanCandidate{}
	seen := map[string]struct{}{}
	for _, code := range rawCodes {
		if !fundCacheRefreshCodeRegexp.MatchString(code) {
			continue
		}
		if _, exists := seen[code]; exists {
			continue
		}
		seen[code] = struct{}{}

		_, cached := cachedByCode[code]
		_, is4433 := strict4433Codes[code]
		updatedAt := meta.LastUpdatedAt(code)
		position := codePositions[code]

		switch {
		case is4433 && meta.IsStale(code, options.Now, options.Priority4433StaleAfter):
			candidates = append(candidates, fundCacheRefreshPlanCandidate{
				code:      code,
				priority:  0,
				kind:      FundCacheRefreshPriority4433,
				updatedAt: updatedAt,
				position:  position,
			})
		case !cached:
			candidates = append(candidates, fundCacheRefreshPlanCandidate{
				code:      code,
				priority:  1,
				kind:      FundCacheRefreshPriorityMissing,
				updatedAt: updatedAt,
				position:  position,
			})
		case meta.IsStale(code, options.Now, options.Non4433StaleAfter):
			candidates = append(candidates, fundCacheRefreshPlanCandidate{
				code:      code,
				priority:  2,
				kind:      FundCacheRefreshPriorityStaleOther,
				updatedAt: updatedAt,
				position:  position,
			})
		default:
			plan.SkippedFreshCount++
		}
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		left := candidates[i]
		right := candidates[j]
		if left.priority != right.priority {
			return left.priority < right.priority
		}
		if left.updatedAt.IsZero() != right.updatedAt.IsZero() {
			return left.updatedAt.IsZero()
		}
		if !left.updatedAt.Equal(right.updatedAt) {
			return left.updatedAt.Before(right.updatedAt)
		}
		if left.position != right.position {
			return left.position < right.position
		}
		return left.code < right.code
	})

	if options.MaxFunds > 0 && len(candidates) > options.MaxFunds {
		plan.DeferredCount = len(candidates) - options.MaxFunds
		candidates = candidates[:options.MaxFunds]
	}
	for _, candidate := range candidates {
		plan.Codes = append(plan.Codes, candidate.code)
		plan.Priorities[candidate.code] = candidate.kind
		switch candidate.kind {
		case FundCacheRefreshPriority4433:
			plan.Priority4433Count++
		case FundCacheRefreshPriorityMissing:
			plan.MissingCount++
		case FundCacheRefreshPriorityStaleOther:
			plan.StaleOtherCount++
		}
	}
	return plan
}

func fundCacheRefreshCodePositions(codes []string) map[string]int {
	positions := map[string]int{}
	for idx, code := range codes {
		if _, exists := positions[code]; !exists {
			positions[code] = idx
		}
	}
	return positions
}

func fundCacheRefreshFundMap(funds models.FundList) map[string]*models.Fund {
	result := map[string]*models.Fund{}
	for _, fund := range funds {
		if fund == nil || fund.Code == "" {
			continue
		}
		result[fund.Code] = fund
	}
	return result
}

func fundCacheRefreshStrict4433Codes(ctx context.Context, cachedFunds models.FundList, current4433Funds models.FundList) map[string]struct{} {
	result := map[string]struct{}{}
	for _, fund := range current4433Funds {
		if fund == nil || fund.Code == "" {
			continue
		}
		result[fund.Code] = struct{}{}
	}
	for _, fund := range cachedFunds {
		if fund == nil || fund.Code == "" {
			continue
		}
		if fund.Is4433(ctx) {
			result[fund.Code] = struct{}{}
		}
	}
	return result
}
