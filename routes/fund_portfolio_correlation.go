package routes

import (
	"time"

	"github.com/axiaoxin-com/investool/datacenter/eastmoney"
)

const (
	defaultFundNAVHistoryCacheFilename         = "./fund_nav_history_cache.json"
	fundPortfolioCorrelationNAVHistoryPageSize = 90
	fundPortfolioCorrelationFetchConcurrency   = 4
	fundPortfolioCorrelationFetchTimeout       = 12 * time.Second
	fundPortfolioCorrelationTimestampLayout    = "2006-01-02 15:04:05"
)

type fundPortfolioCorrelationSource struct {
	Code    string
	Name    string
	History eastmoney.FundNAVHistory
}

type fundPortfolioCorrelationRefreshData struct {
	Needed      bool                           `json:"needed"`
	URL         string                         `json:"url"`
	Message     string                         `json:"message"`
	Missing     int                            `json:"missing"`
	Stale       int                            `json:"stale"`
	LastUpdated string                         `json:"lastUpdated"`
	Funds       []fundPortfolioCorrelationFund `json:"funds"`
}

type fundPortfolioCorrelationFund struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type fundPortfolioCorrelationRefreshRequest struct {
	Funds []fundPortfolioCorrelationFund `json:"funds"`
}

type fundPortfolioCorrelationRefreshResponse struct {
	Correlation fundPortfolioCorrelationChartData   `json:"correlation"`
	Refresh     fundPortfolioCorrelationRefreshData `json:"refresh"`
	Warnings    []string                            `json:"warnings"`
}

type fundPortfolioCorrelationRefreshResult struct {
	Fund    fundPortfolioCorrelationFund
	History eastmoney.FundNAVHistory
	Err     error
}
