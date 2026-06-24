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

const defaultFundDailyProfitTakingStateFilename = "./fund_daily_profit_taking_state.json"

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

type fundDailyAdviceBundle struct {
	Report      core.FundDailyAdviceReport
	Config      core.FundDailyAdviceConfig
	CacheCount  int
	SourceCount int
	SourceName  string
	PageError   string
}

func FundDailyAdvicePage(c *gin.Context) {
	bundle := buildFundDailyAdviceBundle(c, true)
	data := fundDailyAdviceViewData{
		Env:         viper.GetString("env"),
		HostURL:     viper.GetString("server.host_url"),
		Version:     version.Version,
		PageTitle:   "InvesTool | 每日基金操作建议",
		Error:       bundle.PageError,
		Report:      bundle.Report,
		Config:      bundle.Config,
		CacheCount:  bundle.CacheCount,
		SourceCount: bundle.SourceCount,
		SourceName:  bundle.SourceName,
	}
	c.HTML(http.StatusOK, "fund_daily_advice.html", data)
}

func FundDailyAdviceContextJSON(c *gin.Context) {
	bundle := buildFundDailyAdviceBundle(c, false)
	c.JSON(http.StatusOK, bundle.Report.AIContext)
}

func buildFundDailyAdviceBundle(c *gin.Context, persistProfitTakingState bool) fundDailyAdviceBundle {
	portfolioContext := loadFundPortfolioAnalysisContextWithOptions(c, "", fundPortfolioAnalysisLoadOptions{
		UseLocalFundCache: true,
	})
	config := fundDailyAdviceConfigFromSettings(c)
	navCache, navWarnings := loadFundDailyAdviceNAVCache()
	marketContext := core.BuildFundDailyMarketContext(c)
	newsContext := core.BuildFundDailyNewsContext(c)
	candidatePoolCount := maxInt(config.AICandidateCount, config.CandidateCount*18)
	selection := core.SelectDailyAdviceCandidatesWithStrategy(c, models.FundAllList, models.Fund4433RecommendationList, portfolioContext.AllAdvices, navCache, candidatePoolCount)
	subscriptionWarnings := core.RefreshFundDailySubscriptionStatuses(c, selection.Funds, portfolioContext.AllAdvices)
	report := core.BuildFundDailyAdviceReportWithEvidence(c, portfolioContext.AllAdvices, selection.Funds, selection.Evidence, config)
	report.MarketContext = marketContext
	report.NewsContext = newsContext
	industrySignals, industrySignalWarnings := loadFundDailyIndustryPublicSignals(c, report)
	report.IndustryExpectationSignals = industrySignals
	report.IndustryExpectationContext = core.BuildFundDailyIndustryExpectationContext(report)
	report.Warnings = append(report.Warnings, navWarnings...)
	report.Warnings = append(report.Warnings, marketContext.Warnings...)
	report.Warnings = append(report.Warnings, newsContext.Warnings...)
	report.Warnings = append(report.Warnings, industrySignalWarnings...)
	report.Warnings = append(report.Warnings, report.IndustryExpectationContext.Warnings...)
	report.Warnings = append(report.Warnings, subscriptionWarnings...)
	report.Warnings = append(report.Warnings, selection.Warnings...)
	report.AIContext = core.BuildFundDailyAIContext(report)
	profitTakingState, profitTakingWarnings := loadFundDailyProfitTakingState()
	report.Warnings = append(report.Warnings, profitTakingWarnings...)
	decision, updatedProfitTakingState := core.BuildFundDailyLocalDecisionWithProfitTakingState(report.AIContext, profitTakingState)
	report.AIDecision = decision
	if persistProfitTakingState {
		report.Warnings = append(report.Warnings, saveFundDailyProfitTakingState(updatedProfitTakingState)...)
	}

	return fundDailyAdviceBundle{
		Report:      report,
		Config:      config,
		CacheCount:  len(models.FundAllList),
		SourceCount: selection.SourceCount,
		SourceName:  selection.SourceName,
		PageError:   portfolioContext.PageError,
	}
}

func loadFundDailyProfitTakingState() (core.FundDailyProfitTakingState, []string) {
	state, err := newFundDailyProfitTakingStateStore().Load()
	if err != nil {
		return state, []string{fmt.Sprintf("止盈状态读取失败，本轮止盈累计上限将按首次触发处理：%v", err)}
	}
	return state, nil
}

func saveFundDailyProfitTakingState(state core.FundDailyProfitTakingState) []string {
	if err := newFundDailyProfitTakingStateStore().Save(state); err != nil {
		return []string{fmt.Sprintf("止盈状态保存失败，后续可能重复给出同一轮止盈建议：%v", err)}
	}
	return nil
}

func newFundDailyProfitTakingStateStore() *core.FundDailyProfitTakingStateStore {
	filename := viper.GetString("fund_advice.profit_taking_state_filename")
	if filename == "" {
		filename = defaultFundDailyProfitTakingStateFilename
	}
	return core.NewFundDailyProfitTakingStateStore(filename)
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
	if viper.IsSet("fund_advice.ai_candidate_count") {
		config.AICandidateCount = viper.GetInt("fund_advice.ai_candidate_count")
	}
	if viper.IsSet("fund_advice.min_candidate_score") {
		config.MinCandidateScore = viper.GetInt("fund_advice.min_candidate_score")
	}
	if viper.IsSet("fund_advice.min_core_candidate_score") {
		config.MinCoreCandidateScore = viper.GetInt("fund_advice.min_core_candidate_score")
	}
	if viper.IsSet("fund_advice.disable_buy_on_non_workday") {
		config.DisableBuyOnNonWorkday = viper.GetBool("fund_advice.disable_buy_on_non_workday")
		config.DisableBuyOnNonWorkdayConfigured = true
	}
	if viper.IsSet("fund_advice.non_workday_dates") {
		config.NonWorkdayDates = viper.GetStringSlice("fund_advice.non_workday_dates")
	}
	if viper.IsSet("fund_advice.workday_dates") {
		config.WorkdayDates = viper.GetStringSlice("fund_advice.workday_dates")
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
	if c.Query("ai_candidates") != "" {
		if value, ok := parseQueryInt(c.Query("ai_candidates")); ok {
			config.AICandidateCount = value
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

func parseQueryInt(raw string) (int, bool) {
	var value int
	if _, err := fmt.Sscanf(raw, "%d", &value); err != nil {
		return 0, false
	}
	return value, true
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
