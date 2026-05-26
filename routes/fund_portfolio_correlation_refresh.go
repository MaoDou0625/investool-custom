package routes

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/axiaoxin-com/investool/datacenter"
	"github.com/axiaoxin-com/investool/models"
	"github.com/gin-gonic/gin"
)

func FundPortfolioCorrelationRefresh(c *gin.Context) {
	req := fundPortfolioCorrelationRefreshRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	funds := normalizeCorrelationFunds(req.Funds)
	if len(funds) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "empty funds"})
		return
	}

	sources, refresh, warnings := refreshFundPortfolioCorrelationSources(c.Request.Context(), funds, time.Now())
	c.JSON(http.StatusOK, fundPortfolioCorrelationRefreshResponse{
		Correlation: buildCorrelationChartData(sources),
		Refresh:     refresh,
		Warnings:    warnings,
	})
}

func refreshFundPortfolioCorrelationSources(
	ctx context.Context,
	funds []fundPortfolioCorrelationFund,
	now time.Time,
) ([]fundPortfolioCorrelationSource, fundPortfolioCorrelationRefreshData, []string) {
	store := newFundNAVHistoryCacheStore()
	cache, err := store.Load()
	if err != nil {
		refresh := buildFundPortfolioCorrelationRefreshData(funds, models.FundNAVHistoryCache{}, now)
		return nil, refresh, []string{fmt.Sprintf("历史净值缓存读取失败：%v", err)}
	}

	results := make([]fundPortfolioCorrelationRefreshResult, len(funds))
	sem := make(chan struct{}, fundPortfolioCorrelationFetchConcurrency)
	var wg sync.WaitGroup
	for idx, fund := range funds {
		wg.Add(1)
		go func(idx int, fund fundPortfolioCorrelationFund) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			cachedItem, hasCache := cache.Items[fund.Code]
			if hasCache && !fundNAVCacheItemNeedsRefresh(cachedItem, now) {
				results[idx] = fundPortfolioCorrelationRefreshResult{
					Fund:    fund,
					History: historyFromFundNAVCacheItem(cachedItem),
				}
				return
			}

			queryCtx, cancel := context.WithTimeout(ctx, fundPortfolioCorrelationFetchTimeout)
			defer cancel()
			history, err := datacenter.EastMoney.QueryFundNAVHistory(queryCtx, fund.Code, fundPortfolioCorrelationNAVHistoryPageSize)
			results[idx] = fundPortfolioCorrelationRefreshResult{
				Fund:    fund,
				History: history,
				Err:     err,
			}
		}(idx, fund)
	}
	wg.Wait()

	changed := false
	warnings := []string{}
	for _, result := range results {
		if result.Err != nil {
			if cachedItem, ok := cache.Items[result.Fund.Code]; ok && fundNAVCacheItemUsable(cachedItem) {
				warnings = append(warnings, fmt.Sprintf("%s 历史净值刷新失败，已沿用本地缓存：%v", result.Fund.Code, result.Err))
			} else {
				warnings = append(warnings, fmt.Sprintf("%s 历史净值刷新失败，相关性计算已跳过：%v", result.Fund.Code, result.Err))
			}
			continue
		}
		if len(result.History) == 0 {
			warnings = append(warnings, fmt.Sprintf("%s 未返回历史净值数据，相关性计算已跳过", result.Fund.Code))
			continue
		}

		cache.Items[result.Fund.Code] = fundNAVCacheItemFromHistory(result.Fund, result.History, now)
		changed = true
	}

	if changed {
		if err := store.Save(cache); err != nil {
			warnings = append(warnings, fmt.Sprintf("历史净值缓存保存失败：%v", err))
		} else if savedCache, err := store.Load(); err == nil {
			cache = savedCache
		}
	}

	sources := sourcesFromFundNAVCache(funds, cache)
	refresh := buildFundPortfolioCorrelationRefreshData(funds, cache, now)
	return sources, refresh, warnings
}
