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
	FundCacheRefreshStageQueryDetail = "query_detail"
	FundCacheRefreshStageBuildCache  = "build_cache"
	FundCacheRefreshStageWriteCache  = "write_cache"
	FundCacheRefreshStageDone        = "done"
	FundCacheRefreshStageError       = "error"
)

var fundCacheRefreshCodeRegexp = regexp.MustCompile(`^\d{6}$`)

// FundCacheRefreshOptions controls a full local fund-cache refresh.
type FundCacheRefreshOptions struct {
	WorkerCount           int
	RecommendationOptions Fund4433RecommendationOptions
}

// FundCacheRefreshProgress reports refresh progress for UI polling.
type FundCacheRefreshProgress struct {
	Stage               string `json:"stage"`
	RawFundCount        int    `json:"raw_fund_count"`
	Total               int    `json:"total"`
	Processed           int    `json:"processed"`
	Succeeded           int    `json:"succeeded"`
	Failed              int    `json:"failed"`
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
		WorkerCount:           4,
		RecommendationOptions: DefaultFund4433RecommendationOptions(),
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

// RefreshFundCache rebuilds the full local fund cache and derived 4433 data files.
func RefreshFundCache(ctx context.Context, options FundCacheRefreshOptions, progress FundCacheRefreshProgressFunc) (FundCacheRefreshProgress, error) {
	options = normalizeFundCacheRefreshOptions(options)
	status := FundCacheRefreshProgress{Stage: FundCacheRefreshStageQueryList}
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
	status.Stage = FundCacheRefreshStageQueryDetail
	status.Total = len(fundCodes)
	reportFundCacheRefreshProgress(status, progress)

	searcher := NewSearcher(ctx)
	var statusMu sync.Mutex
	fundMap, err := searcher.SearchFundsWithWorkerCountAndProgress(ctx, fundCodes, options.WorkerCount, func(code string, success bool, err error) {
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

	status.Stage = FundCacheRefreshStageBuildCache
	reportFundCacheRefreshProgress(status, progress)
	fundlist, fundtypes := buildFundCacheRefreshLists(fundCodes, fundMap)
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
