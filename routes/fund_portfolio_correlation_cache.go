package routes

import (
	"fmt"
	"strings"
	"time"

	"github.com/axiaoxin-com/investool/core"
	"github.com/axiaoxin-com/investool/datacenter/eastmoney"
	"github.com/axiaoxin-com/investool/models"
	"github.com/spf13/viper"
)

func loadFundPortfolioCorrelationSources(
	advices []core.FundPortfolioAdvice,
	now time.Time,
) ([]fundPortfolioCorrelationSource, fundPortfolioCorrelationRefreshData, []string) {
	funds := fundsForCorrelationRefresh(advices)
	cache, err := newFundNAVHistoryCacheStore().Load()
	if err != nil {
		refresh := buildFundPortfolioCorrelationRefreshData(funds, models.FundNAVHistoryCache{}, now)
		return nil, refresh, []string{fmt.Sprintf("历史净值缓存读取失败，相关性图将等待刷新：%v", err)}
	}

	sources := sourcesFromFundNAVCache(funds, cache)
	refresh := buildFundPortfolioCorrelationRefreshData(funds, cache, now)
	return sources, refresh, nil
}

func fundsForCorrelationRefresh(advices []core.FundPortfolioAdvice) []fundPortfolioCorrelationFund {
	funds := make([]fundPortfolioCorrelationFund, 0, len(advices))
	for _, advice := range advices {
		funds = append(funds, fundPortfolioCorrelationFund{
			Code: advice.Item.Code,
			Name: fundNameForAdvice(advice),
		})
	}
	return normalizeCorrelationFunds(funds)
}

func normalizeCorrelationFunds(funds []fundPortfolioCorrelationFund) []fundPortfolioCorrelationFund {
	normalized := []fundPortfolioCorrelationFund{}
	seen := map[string]bool{}
	for _, fund := range funds {
		code := strings.TrimSpace(fund.Code)
		if code == "" || seen[code] {
			continue
		}
		name := strings.TrimSpace(fund.Name)
		if name == "" {
			name = code
		}
		seen[code] = true
		normalized = append(normalized, fundPortfolioCorrelationFund{
			Code: code,
			Name: name,
		})
	}
	return normalized
}

func sourcesFromFundNAVCache(
	funds []fundPortfolioCorrelationFund,
	cache models.FundNAVHistoryCache,
) []fundPortfolioCorrelationSource {
	sources := []fundPortfolioCorrelationSource{}
	for _, fund := range funds {
		item, ok := cache.Items[fund.Code]
		if !ok || !fundNAVCacheItemUsable(item) {
			continue
		}
		name := fund.Name
		if name == "" {
			name = item.Name
		}
		sources = append(sources, fundPortfolioCorrelationSource{
			Code:    fund.Code,
			Name:    name,
			History: historyFromFundNAVCacheItem(item),
		})
	}
	return sources
}

func buildFundPortfolioCorrelationRefreshData(
	funds []fundPortfolioCorrelationFund,
	cache models.FundNAVHistoryCache,
	now time.Time,
) fundPortfolioCorrelationRefreshData {
	refresh := fundPortfolioCorrelationRefreshData{
		URL:   "/fund/portfolio/correlation/refresh",
		Funds: funds,
	}
	for _, fund := range funds {
		item, ok := cache.Items[fund.Code]
		if !ok || !fundNAVCacheItemUsable(item) {
			refresh.Missing++
			continue
		}
		if refresh.LastUpdated == "" || item.UpdatedAt > refresh.LastUpdated {
			refresh.LastUpdated = item.UpdatedAt
		}
		if fundNAVCacheItemNeedsRefresh(item, now) {
			refresh.Stale++
		}
	}
	refresh.Needed = refresh.Missing > 0 || refresh.Stale > 0
	switch {
	case refresh.Missing > 0 && refresh.Stale > 0:
		refresh.Message = fmt.Sprintf("正在刷新历史净值：%d 只基金缺少缓存，%d 只基金需要更新。", refresh.Missing, refresh.Stale)
	case refresh.Missing > 0:
		refresh.Message = fmt.Sprintf("正在刷新历史净值：%d 只基金缺少缓存。", refresh.Missing)
	case refresh.Stale > 0:
		refresh.Message = fmt.Sprintf("正在刷新历史净值：%d 只基金缓存需要更新。", refresh.Stale)
	default:
		refresh.Message = "历史净值缓存已是最新。"
	}
	return refresh
}

func fundNAVCacheItemFromHistory(
	fund fundPortfolioCorrelationFund,
	history eastmoney.FundNAVHistory,
	now time.Time,
) models.FundNAVHistoryCacheItem {
	points := make([]models.FundNAVHistoryCachePoint, 0, len(history))
	for _, point := range history {
		points = append(points, models.FundNAVHistoryCachePoint{
			Date:    point.Date,
			UnitNAV: point.UnitNAV,
		})
	}
	return models.FundNAVHistoryCacheItem{
		Code:      fund.Code,
		Name:      fund.Name,
		UpdatedAt: now.Format(fundPortfolioCorrelationTimestampLayout),
		Points:    points,
	}
}

func historyFromFundNAVCacheItem(item models.FundNAVHistoryCacheItem) eastmoney.FundNAVHistory {
	history := make(eastmoney.FundNAVHistory, 0, len(item.Points))
	for _, point := range item.Points {
		history = append(history, eastmoney.FundNAVHistoryPoint{
			Date:    point.Date,
			UnitNAV: point.UnitNAV,
		})
	}
	return history
}

func fundNAVCacheItemUsable(item models.FundNAVHistoryCacheItem) bool {
	return len(item.Points) >= minFundCorrelationReturnPairs+1
}

func fundNAVCacheItemNeedsRefresh(item models.FundNAVHistoryCacheItem, now time.Time) bool {
	if !fundNAVCacheItemUsable(item) {
		return true
	}
	updatedAt, err := time.ParseInLocation(fundPortfolioCorrelationTimestampLayout, item.UpdatedAt, time.Local)
	if err != nil {
		return true
	}
	return updatedAt.Format("2006-01-02") != now.Format("2006-01-02")
}

func newFundNAVHistoryCacheStore() *models.FundNAVHistoryCacheStore {
	return models.NewFundNAVHistoryCacheStore(fundNAVHistoryCacheFilename())
}

func fundNAVHistoryCacheFilename() string {
	filename := viper.GetString("fund_portfolio.nav_history_cache_filename")
	if filename == "" {
		return defaultFundNAVHistoryCacheFilename
	}
	return filename
}
