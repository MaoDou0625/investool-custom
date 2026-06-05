package core

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/axiaoxin-com/investool/datacenter"
	"github.com/axiaoxin-com/investool/datacenter/eastmoney"
	"github.com/axiaoxin-com/investool/models"
)

const (
	FundCacheRefreshStageQueryList   = "query_list"
	FundCacheRefreshStageBuildPlan   = "build_plan"
	FundCacheRefreshStageQueryDetail = "query_detail"
	FundCacheRefreshStageBuildCache  = "build_cache"
	FundCacheRefreshStageWriteCache  = "write_cache"
	FundCacheRefreshStageDone        = "done"
	FundCacheRefreshStageError       = "error"
)

var fundCacheRefreshCodeRegexp = regexp.MustCompile(`^\d{6}$`)

// FundCacheRefreshOptions controls local fund-cache refresh behavior.
type FundCacheRefreshOptions struct {
	WorkerCount            int
	RecommendationOptions  Fund4433RecommendationOptions
	Mode                   string
	MaxFunds               int
	Priority4433StaleAfter time.Duration
	Non4433StaleAfter      time.Duration
	Now                    time.Time
}

// FundCacheRefreshProgress reports refresh progress for UI polling.
type FundCacheRefreshProgress struct {
	Stage               string `json:"stage"`
	Mode                string `json:"mode"`
	RawFundCount        int    `json:"raw_fund_count"`
	Planned             int    `json:"planned"`
	Total               int    `json:"total"`
	Processed           int    `json:"processed"`
	Succeeded           int    `json:"succeeded"`
	Failed              int    `json:"failed"`
	Priority4433Count   int    `json:"priority_4433_count"`
	MissingCount        int    `json:"missing_count"`
	StaleOtherCount     int    `json:"stale_other_count"`
	DeferredCount       int    `json:"deferred_count"`
	SkippedFreshCount   int    `json:"skipped_fresh_count"`
	FundCount           int    `json:"fund_count"`
	Fund4433Count       int    `json:"fund_4433_count"`
	RecommendationCount int    `json:"recommendation_count"`
	TypeCount           int    `json:"type_count"`
	Error               string `json:"error"`
}

// FundCacheRefreshProgressFunc receives a snapshot whenever refresh progress changes.
type FundCacheRefreshProgressFunc func(FundCacheRefreshProgress)

// DefaultFundCacheRefreshOptions returns conservative defaults for a manual cache refresh.
func DefaultFundCacheRefreshOptions() FundCacheRefreshOptions {
	return FundCacheRefreshOptions{
		WorkerCount:            4,
		RecommendationOptions:  DefaultFund4433RecommendationOptions(),
		Mode:                   FundCacheRefreshModePriority,
		MaxFunds:               160,
		Priority4433StaleAfter: 7 * 24 * time.Hour,
		Non4433StaleAfter:      30 * 24 * time.Hour,
		Now:                    time.Now(),
	}
}

// BuildFund4433List filters and sorts funds that strictly match 4433.
func BuildFund4433List(ctx context.Context, fundlist models.FundList) models.FundList {
	result := models.FundList{}
	for _, fund := range fundlist {
		if fund == nil {
			continue
		}
		if fund.Is4433(ctx) {
			result = append(result, fund)
		}
	}
	result.Sort(models.FundSortTypeWeek)
	return result
}

// RefreshFundCache updates the local fund cache and derived 4433 data files.
func RefreshFundCache(ctx context.Context, options FundCacheRefreshOptions, progress FundCacheRefreshProgressFunc) (FundCacheRefreshProgress, error) {
	options = normalizeFundCacheRefreshOptions(options)
	status := FundCacheRefreshProgress{Stage: FundCacheRefreshStageQueryList, Mode: options.Mode}
	reportFundCacheRefreshProgress(status, progress)

	rawFundList, err := datacenter.EastMoney.QueryAllFundList(ctx, eastmoney.FundTypeALL)
	if err != nil {
		status.Stage = FundCacheRefreshStageError
		status.Error = err.Error()
		reportFundCacheRefreshProgress(status, progress)
		return status, err
	}
	status.RawFundCount = len(rawFundList)

	fundCodes := extractFundCacheRefreshCodes(rawFundList)
	status.Stage = FundCacheRefreshStageBuildPlan
	reportFundCacheRefreshProgress(status, progress)

	metaStore := models.NewFundDetailRefreshMetaStore(models.FundDetailRefreshMetaFilename)
	detailMeta, err := metaStore.Load()
	if err != nil {
		detailMeta = models.NewFundDetailRefreshMeta()
	}
	plan := BuildFundCacheRefreshPlan(ctx, fundCodes, models.FundAllList, models.Fund4433List, detailMeta, options)
	status.Planned = len(plan.Codes)
	status.Total = len(plan.Codes)
	status.Priority4433Count = plan.Priority4433Count
	status.MissingCount = plan.MissingCount
	status.StaleOtherCount = plan.StaleOtherCount
	status.DeferredCount = plan.DeferredCount
	status.SkippedFreshCount = plan.SkippedFreshCount
	status.Stage = FundCacheRefreshStageQueryDetail
	reportFundCacheRefreshProgress(status, progress)

	searcher := NewSearcher(ctx)
	var statusMu sync.Mutex
	fundMap := map[string]*models.Fund{}
	if len(plan.Codes) > 0 {
		fundMap, err = searcher.SearchFundsWithWorkerCountAndProgress(ctx, plan.Codes, options.WorkerCount, func(code string, success bool, err error) {
			statusMu.Lock()
			defer statusMu.Unlock()
			status.Processed++
			if success {
				status.Succeeded++
			} else {
				status.Failed++
			}
			if status.Processed == status.Total || status.Processed%10 == 0 {
				reportFundCacheRefreshProgress(status, progress)
			}
		})
		if err != nil {
			status.Stage = FundCacheRefreshStageError
			status.Error = err.Error()
			reportFundCacheRefreshProgress(status, progress)
			return status, err
		}
	}

	status.Stage = FundCacheRefreshStageBuildCache
	reportFundCacheRefreshProgress(status, progress)
	mergedFundMap := mergeFundCacheRefreshFunds(models.FundAllList, fundMap)
	for code := range fundMap {
		detailMeta.Touch(code, options.Now, plan.Priorities[code])
	}
	fundlist, fundtypes := buildFundCacheRefreshLists(fundCodes, mergedFundMap)
	fund4433List := BuildFund4433List(ctx, fundlist)
	recommendations, recommendationSource, recommendationSourceCount, err := RefreshFund4433Recommendations(ctx, fundlist, options.RecommendationOptions)
	if err != nil {
		recommendations = BuildFund4433Recommendations(ctx, fundlist, options.RecommendationOptions)
		recommendationSource = "本地全量基金缓存"
		recommendationSourceCount = len(fundlist)
	}
	status.FundCount = len(fundlist)
	status.TypeCount = len(fundtypes)
	status.Fund4433Count = len(fund4433List)
	status.RecommendationCount = len(recommendations)

	status.Stage = FundCacheRefreshStageWriteCache
	reportFundCacheRefreshProgress(status, progress)
	if err := saveFundCacheRefreshFiles(rawFundList, fundlist, fundtypes, fund4433List, recommendations, recommendationSource, recommendationSourceCount); err != nil {
		status.Stage = FundCacheRefreshStageError
		status.Error = err.Error()
		reportFundCacheRefreshProgress(status, progress)
		return status, err
	}
	if err := metaStore.Save(detailMeta); err != nil {
		status.Stage = FundCacheRefreshStageError
		status.Error = err.Error()
		reportFundCacheRefreshProgress(status, progress)
		return status, err
	}

	models.FundAllList = fundlist
	models.FundTypeList = fundtypes
	models.Fund4433List = fund4433List
	models.Fund4433TypeList = fund4433List.Types()
	models.SyncFundTime = time.Now()

	status.Stage = FundCacheRefreshStageDone
	reportFundCacheRefreshProgress(status, progress)
	return status, nil
}

func normalizeFundCacheRefreshOptions(options FundCacheRefreshOptions) FundCacheRefreshOptions {
	defaults := DefaultFundCacheRefreshOptions()
	if options.WorkerCount <= 0 {
		options.WorkerCount = defaults.WorkerCount
	}
	if options.Mode != FundCacheRefreshModeFull {
		options.Mode = FundCacheRefreshModePriority
	}
	if options.Mode == FundCacheRefreshModePriority && options.MaxFunds <= 0 {
		options.MaxFunds = defaults.MaxFunds
	}
	if options.Priority4433StaleAfter <= 0 {
		options.Priority4433StaleAfter = defaults.Priority4433StaleAfter
	}
	if options.Non4433StaleAfter <= 0 {
		options.Non4433StaleAfter = defaults.Non4433StaleAfter
	}
	if options.Now.IsZero() {
		options.Now = defaults.Now
	}
	options.RecommendationOptions = normalizeFund4433RecommendationOptions(options.RecommendationOptions)
	return options
}

func extractFundCacheRefreshCodes(rawFundList eastmoney.FundList) []string {
	codes := make([]string, 0, len(rawFundList))
	seen := map[string]struct{}{}
	for _, fund := range rawFundList {
		if !fundCacheRefreshCodeRegexp.MatchString(fund.Fcode) {
			continue
		}
		if _, exists := seen[fund.Fcode]; exists {
			continue
		}
		seen[fund.Fcode] = struct{}{}
		codes = append(codes, fund.Fcode)
	}
	return codes
}

func buildFundCacheRefreshLists(fundCodes []string, fundMap map[string]*models.Fund) (models.FundList, []string) {
	fundlist := make(models.FundList, 0, len(fundMap))
	typeMap := map[string]struct{}{}
	for _, code := range fundCodes {
		fund := fundMap[code]
		if fund == nil {
			continue
		}
		fundlist = append(fundlist, fund)
		if fund.Type != "" {
			typeMap[fund.Type] = struct{}{}
		}
	}

	fundtypes := make([]string, 0, len(typeMap))
	for fundType := range typeMap {
		fundtypes = append(fundtypes, fundType)
	}
	sort.Strings(fundtypes)
	return fundlist, fundtypes
}

func mergeFundCacheRefreshFunds(existing models.FundList, updated map[string]*models.Fund) map[string]*models.Fund {
	merged := fundCacheRefreshFundMap(existing)
	for code, fund := range updated {
		if fund == nil || code == "" {
			continue
		}
		merged[code] = fund
	}
	return merged
}

func saveFundCacheRefreshFiles(rawFundList eastmoney.FundList, fundlist models.FundList, fundtypes []string, fund4433List models.FundList, recommendations models.FundList, recommendationSource string, recommendationSourceCount int) error {
	files := []struct {
		name string
		data interface{}
	}{
		{models.RawFundAllListFilename, rawFundList},
		{models.FundAllListFilename, fundlist},
		{models.FundTypeListFilename, fundtypes},
		{models.Fund4433ListFilename, fund4433List},
	}
	for _, file := range files {
		if err := writeFundCacheRefreshJSON(file.name, file.data); err != nil {
			return err
		}
	}

	return models.SaveFund4433RecommendationCache(models.Fund4433RecommendationCache{
		UpdatedAt:   time.Now(),
		Source:      recommendationSource,
		SourceCount: recommendationSourceCount,
		Items:       recommendations,
	})
}

func writeFundCacheRefreshJSON(filename string, data interface{}) error {
	content, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", filename, err)
	}
	if err := ioutil.WriteFile(filename, content, 0666); err != nil {
		return fmt.Errorf("write %s: %w", filename, err)
	}
	return nil
}

func reportFundCacheRefreshProgress(status FundCacheRefreshProgress, progress FundCacheRefreshProgressFunc) {
	if progress != nil {
		progress(status)
	}
}
