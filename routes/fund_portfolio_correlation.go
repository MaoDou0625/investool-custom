package routes

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/axiaoxin-com/investool/core"
	"github.com/axiaoxin-com/investool/datacenter"
	"github.com/axiaoxin-com/investool/datacenter/eastmoney"
)

const (
	fundPortfolioCorrelationNAVHistoryPageSize = 90
	fundPortfolioCorrelationFetchConcurrency   = 4
	fundPortfolioCorrelationFetchTimeout       = 12 * time.Second
	fundPortfolioCorrelationCacheTTL           = 30 * time.Minute
)

var fundPortfolioCorrelationNAVCache = struct {
	sync.Mutex
	items map[string]fundPortfolioCorrelationCacheEntry
}{
	items: map[string]fundPortfolioCorrelationCacheEntry{},
}

type fundPortfolioCorrelationSource struct {
	Code    string
	Name    string
	History eastmoney.FundNAVHistory
}

func loadFundPortfolioCorrelationSources(ctx context.Context, advices []core.FundPortfolioAdvice) ([]fundPortfolioCorrelationSource, []string) {
	requests := []fundPortfolioCorrelationRequest{}
	seen := map[string]bool{}
	for _, advice := range advices {
		code := strings.TrimSpace(advice.Item.Code)
		if code == "" || seen[code] {
			continue
		}
		seen[code] = true
		requests = append(requests, fundPortfolioCorrelationRequest{
			Code: code,
			Name: fundNameForAdvice(advice),
		})
	}
	if len(requests) == 0 {
		return nil, nil
	}

	results := make([]fundPortfolioCorrelationResult, len(requests))
	sem := make(chan struct{}, fundPortfolioCorrelationFetchConcurrency)
	var wg sync.WaitGroup
	for idx, request := range requests {
		wg.Add(1)
		go func(idx int, request fundPortfolioCorrelationRequest) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			history, err := queryFundPortfolioCorrelationHistory(ctx, request.Code)
			results[idx] = fundPortfolioCorrelationResult{
				Request: request,
				History: history,
				Err:     err,
			}
		}(idx, request)
	}
	wg.Wait()

	sources := []fundPortfolioCorrelationSource{}
	warnings := []string{}
	for _, result := range results {
		if result.Err != nil {
			warnings = append(warnings, fmt.Sprintf("%s 近期净值历史读取失败，相关性计算已跳过：%v", result.Request.Code, result.Err))
			continue
		}
		sources = append(sources, fundPortfolioCorrelationSource{
			Code:    result.Request.Code,
			Name:    result.Request.Name,
			History: result.History,
		})
	}
	return sources, warnings
}

func queryFundPortfolioCorrelationHistory(ctx context.Context, code string) (eastmoney.FundNAVHistory, error) {
	if history, ok := getCachedFundPortfolioCorrelationHistory(code, time.Now()); ok {
		return history, nil
	}

	queryCtx, cancel := context.WithTimeout(ctx, fundPortfolioCorrelationFetchTimeout)
	defer cancel()
	history, err := datacenter.EastMoney.QueryFundNAVHistory(queryCtx, code, fundPortfolioCorrelationNAVHistoryPageSize)
	if err != nil {
		return nil, err
	}
	if len(history) > 0 {
		setCachedFundPortfolioCorrelationHistory(code, history, time.Now())
	}
	return history, nil
}

func getCachedFundPortfolioCorrelationHistory(code string, now time.Time) (eastmoney.FundNAVHistory, bool) {
	fundPortfolioCorrelationNAVCache.Lock()
	defer fundPortfolioCorrelationNAVCache.Unlock()

	item, ok := fundPortfolioCorrelationNAVCache.items[code]
	if !ok || now.After(item.ExpiresAt) {
		return nil, false
	}
	return cloneFundNAVHistory(item.History), true
}

func setCachedFundPortfolioCorrelationHistory(code string, history eastmoney.FundNAVHistory, now time.Time) {
	fundPortfolioCorrelationNAVCache.Lock()
	defer fundPortfolioCorrelationNAVCache.Unlock()

	fundPortfolioCorrelationNAVCache.items[code] = fundPortfolioCorrelationCacheEntry{
		History:   cloneFundNAVHistory(history),
		ExpiresAt: now.Add(fundPortfolioCorrelationCacheTTL),
	}
}

func cloneFundNAVHistory(history eastmoney.FundNAVHistory) eastmoney.FundNAVHistory {
	cloned := make(eastmoney.FundNAVHistory, len(history))
	copy(cloned, history)
	return cloned
}

type fundPortfolioCorrelationRequest struct {
	Code string
	Name string
}

type fundPortfolioCorrelationResult struct {
	Request fundPortfolioCorrelationRequest
	History eastmoney.FundNAVHistory
	Err     error
}

type fundPortfolioCorrelationCacheEntry struct {
	History   eastmoney.FundNAVHistory
	ExpiresAt time.Time
}
