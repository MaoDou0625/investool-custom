package routes

import (
	"fmt"
	"net/http"

	"github.com/axiaoxin-com/investool/core"
	"github.com/axiaoxin-com/investool/models"
	"github.com/axiaoxin-com/investool/version"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

type fundDailyAdviceViewData struct {
	Env         string
	HostURL     string
	Version     string
	PageTitle   string
	Error       string
	Report      core.FundDailyAdviceReport
	Config      core.FundDailyAdviceConfig
	CacheCount  int
	SourceCount int
	SourceName  string
}

func FundDailyAdvicePage(c *gin.Context) {
	portfolioContext := loadFundPortfolioAnalysisContextWithOptions(c, "", fundPortfolioAnalysisLoadOptions{
		UseLocalFundCache: true,
	})
	config := fundDailyAdviceConfigFromSettings(c)
	navCache, navWarnings := loadFundDailyAdviceNAVCache()
	selection := core.SelectDailyAdviceCandidatesWithStrategy(c, models.FundAllList, models.Fund4433RecommendationList, portfolioContext.AllAdvices, navCache, config.CandidateCount*18)
	report := core.BuildFundDailyAdviceReportWithEvidence(c, portfolioContext.AllAdvices, selection.Funds, selection.Evidence, config)
	report.Warnings = append(report.Warnings, navWarnings...)
	report.Warnings = append(report.Warnings, selection.Warnings...)
	pageErr := portfolioContext.PageError

	data := fundDailyAdviceViewData{
		Env:         viper.GetString("env"),
		HostURL:     viper.GetString("server.host_url"),
		Version:     version.Version,
		PageTitle:   "InvesTool | 每日基金操作建议",
		Error:       pageErr,
		Report:      report,
		Config:      config,
		CacheCount:  len(models.FundAllList),
		SourceCount: selection.SourceCount,
		SourceName:  selection.SourceName,
	}
	c.HTML(http.StatusOK, "fund_daily_advice.html", data)
}

func loadFundDailyAdviceNAVCache() (models.FundNAVHistoryCache, []string) {
	cache := models.FundNAVHistoryCache{Items: map[string]models.FundNAVHistoryCacheItem{}}
	loaded, err := newFundNAVHistoryCacheStore().Load()
	if err != nil {
		return cache, []string{fmt.Sprintf("历史净值缓存读取失败，候选相关性将降级为主题重叠估算：%v", err)}
	}
	return loaded, nil
}

func fundDailyAdviceConfigFromSettings(c *gin.Context) core.FundDailyAdviceConfig {
	config := core.DefaultFundDailyAdviceConfig()
	if viper.IsSet("fund_advice.target_annual_return") {
		config.TargetAnnualReturn = viper.GetFloat64("fund_advice.target_annual_return")
	}
	if viper.IsSet("fund_advice.max_total_amount") {
		config.MaxTotalAmount = viper.GetFloat64("fund_advice.max_total_amount")
	}
	if viper.IsSet("fund_advice.cash_buffer_weight") {
		config.CashBufferWeight = viper.GetFloat64("fund_advice.cash_buffer_weight")
	}
	if viper.IsSet("fund_advice.max_single_fund_weight") {
		config.MaxSingleFundWeight = viper.GetFloat64("fund_advice.max_single_fund_weight")
	}
	if viper.IsSet("fund_advice.tactical_weight") {
		config.TacticalWeight = viper.GetFloat64("fund_advice.tactical_weight")
	}
	if viper.IsSet("fund_advice.max_daily_buy_weight") {
		config.MaxDailyBuyWeight = viper.GetFloat64("fund_advice.max_daily_buy_weight")
	}
	if viper.IsSet("fund_advice.candidate_count") {
		config.CandidateCount = viper.GetInt("fund_advice.candidate_count")
	}
	if viper.IsSet("fund_advice.min_candidate_score") {
		config.MinCandidateScore = viper.GetInt("fund_advice.min_candidate_score")
	}
	if viper.IsSet("fund_advice.min_core_candidate_score") {
		config.MinCoreCandidateScore = viper.GetInt("fund_advice.min_core_candidate_score")
	}

	if c.Query("target_return") != "" {
		if value, ok := parseQueryFloat(c.Query("target_return")); ok {
			config.TargetAnnualReturn = value
		}
	}
	if c.Query("max_amount") != "" {
		if value, ok := parseQueryFloat(c.Query("max_amount")); ok {
			config.MaxTotalAmount = value
		}
	}
	if c.Query("daily_buy_weight") != "" {
		if value, ok := parseQueryFloat(c.Query("daily_buy_weight")); ok {
			config.MaxDailyBuyWeight = value
		}
	}
	return config
}

func parseQueryFloat(raw string) (float64, bool) {
	var value float64
	if _, err := fmt.Sscanf(raw, "%f", &value); err != nil {
		return 0, false
	}
	return value, true
}
