package core

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

const (
	fundDailyIndustryExpectationProvider = "local-industry-expectation-v1"

	FundDailyIndustryPricingNotFullyPriced   = "not_fully_priced"
	FundDailyIndustryPricingPartiallyPriced  = "partially_priced"
	FundDailyIndustryPricingMostlyPriced     = "mostly_priced"
	FundDailyIndustryPricingInsufficientData = "insufficient_data"
)

type FundDailyIndustryExpectationContext struct {
	Status           string                              `json:"status"`
	Provider         string                              `json:"provider,omitempty"`
	GeneratedAt      string                              `json:"generated_at,omitempty"`
	Summary          string                              `json:"summary,omitempty"`
	BudgetMultiplier float64                             `json:"budget_multiplier"`
	Themes           []FundDailyIndustryExpectationTheme `json:"themes,omitempty"`
	Reasons          []string                            `json:"reasons,omitempty"`
	Warnings         []string                            `json:"warnings,omitempty"`
}

type FundDailyIndustryExpectationTheme struct {
	Theme              string                                 `json:"theme"`
	RiskPressure       string                                 `json:"risk_pressure"`
	PricingState       string                                 `json:"pricing_state"`
	Confidence         float64                                `json:"confidence"`
	ExposureWeight     float64                                `json:"exposure_weight_percent"`
	PortfolioFundCount int                                    `json:"portfolio_fund_count"`
	CandidateFundCount int                                    `json:"candidate_fund_count"`
	RiskScore          float64                                `json:"risk_score"`
	PricingScore       float64                                `json:"pricing_score"`
	Effect             string                                 `json:"effect"`
	Evidence           []FundDailyIndustryExpectationEvidence `json:"evidence,omitempty"`
	Reasons            []string                               `json:"reasons,omitempty"`
	Warnings           []string                               `json:"warnings,omitempty"`
}

type FundDailyIndustryExpectationEvidence struct {
	Category string  `json:"category"`
	Status   string  `json:"status"`
	Signal   string  `json:"signal,omitempty"`
	Value    float64 `json:"value,omitempty"`
	Weight   float64 `json:"weight,omitempty"`
	Reason   string  `json:"reason,omitempty"`
}

type FundDailyIndustryExpectationSignal struct {
	Theme    string                               `json:"theme"`
	Evidence FundDailyIndustryExpectationEvidence `json:"evidence"`
}

type fundDailyIndustryExpectationThemeAgg struct {
	theme              string
	exposureWeight     float64
	portfolioFundCount int
	candidateFundCount int
	recentReturns      []fundDailyIndustryRecentReturn
	evidence           []FundDailyIndustryExpectationEvidence
	warnings           []string
}

type fundDailyIndustryRecentReturn struct {
	month1 float64
	month3 float64
	month6 float64
}

// BuildFundDailyIndustryExpectationContext estimates whether industry/theme risks
// are already reflected in prices. It deliberately separates hard evidence from
// missing data so the daily advice page does not overstate objectivity.
func BuildFundDailyIndustryExpectationContext(report FundDailyAdviceReport) FundDailyIndustryExpectationContext {
	now := report.GeneratedAt
	if now.IsZero() {
		now = time.Now()
	}
	contextData := FundDailyIndustryExpectationContext{
		Status:           "unavailable",
		Provider:         fundDailyIndustryExpectationProvider,
		GeneratedAt:      now.Format("2006-01-02 15:04:05"),
		BudgetMultiplier: 1,
		Warnings:         []string{},
	}

	portfolioActions := report.DecisionPortfolioActions
	if len(portfolioActions) == 0 {
		portfolioActions = report.PortfolioActions
	}
	candidateActions := report.DecisionCandidateActions
	if len(candidateActions) == 0 {
		candidateActions = report.CandidateActions
	}

	aggregates := buildFundDailyIndustryExpectationAggregates(report, portfolioActions, candidateActions)
	if len(aggregates) == 0 {
		contextData.Summary = "行业风险定价数据暂不可用，本次不调整行业风险因子。"
		contextData.Warnings = append(contextData.Warnings, "未识别到可用于行业风险定价分析的持仓或候选主题。")
		return contextData
	}

	themes := make([]FundDailyIndustryExpectationTheme, 0, len(aggregates))
	for _, aggregate := range aggregates {
		themes = append(themes, analyzeFundDailyIndustryExpectationTheme(aggregate, report.MarketContext, report.NewsContext))
	}
	sort.SliceStable(themes, func(i, j int) bool {
		if math.Abs(themes[i].RiskScore-themes[j].RiskScore) > 0.001 {
			return themes[i].RiskScore > themes[j].RiskScore
		}
		if math.Abs(themes[i].ExposureWeight-themes[j].ExposureWeight) > 0.001 {
			return themes[i].ExposureWeight > themes[j].ExposureWeight
		}
		return themes[i].Theme < themes[j].Theme
	})
	if len(themes) > 6 {
		themes = themes[:6]
	}

	contextData.Themes = themes
	contextData.Status = fundDailyIndustryExpectationStatus(themes)
	contextData.BudgetMultiplier = fundDailyIndustryExpectationBudgetMultiplier(themes)
	contextData.Reasons = compactDailyReasons(fundDailyIndustryExpectationReasons(themes), 6)
	contextData.Summary = fundDailyIndustryExpectationSummary(contextData)
	contextData.Warnings = append(contextData.Warnings, fundDailyIndustryExpectationWarnings(themes)...)
	return contextData
}

func buildFundDailyIndustryExpectationAggregates(report FundDailyAdviceReport, portfolioActions []FundDailyAction, candidateActions []FundDailyAction) map[string]fundDailyIndustryExpectationThemeAgg {
	aggregates := map[string]fundDailyIndustryExpectationThemeAgg{}
	for _, action := range portfolioActions {
		fund := fundDailyAIFundFromAction(action, fundDailyBudgetSourcePortfolio)
		themes := fundDailyIndustryThemesForFund(fund)
		if len(themes) == 0 {
			continue
		}
		exposureWeight := fundDailyIndustryExposureWeight(fund, report) / float64(len(themes))
		for _, theme := range themes {
			agg := aggregates[theme]
			agg.theme = theme
			agg.portfolioFundCount++
			agg.exposureWeight += exposureWeight
			agg.recentReturns = append(agg.recentReturns, fundDailyIndustryRecentReturn{
				month1: fund.RecentReturns.Month1,
				month3: fund.RecentReturns.Month3,
				month6: fund.RecentReturns.Month6,
			})
			aggregates[theme] = agg
		}
	}
	for _, action := range candidateActions {
		fund := fundDailyAIFundFromAction(action, fundDailyBudgetSourceCandidate)
		for _, theme := range fundDailyIndustryThemesForFund(fund) {
			agg := aggregates[theme]
			agg.theme = theme
			agg.candidateFundCount++
			aggregates[theme] = agg
		}
	}
	for _, signal := range report.IndustryExpectationSignals {
		theme := strings.TrimSpace(signal.Theme)
		if theme == "" || strings.TrimSpace(signal.Evidence.Category) == "" {
			continue
		}
		agg := aggregates[theme]
		agg.theme = theme
		agg.evidence = append(agg.evidence, signal.Evidence)
		aggregates[theme] = agg
	}
	return aggregates
}

func fundDailyIndustryExposureWeight(fund FundDailyAIFund, report FundDailyAdviceReport) float64 {
	if fund.CurrentWeight > 0 {
		return fund.CurrentWeight
	}
	if fund.CurrentAmount > 0 && report.InvestableAmount > 0 {
		return fund.CurrentAmount / report.InvestableAmount * 100
	}
	return 0
}

func fundDailyIndustryThemesForFund(fund FundDailyAIFund) []string {
	themes := inferFundDailyMarketThemes(fund)
	if len(themes) == 0 {
		role := fundDailyLocalDiversificationRole(fund)
		if role != "" {
			themes = append(themes, role)
		}
	}
	return uniqueStrings(themes)
}

func analyzeFundDailyIndustryExpectationTheme(aggregate fundDailyIndustryExpectationThemeAgg, market FundDailyMarketContext, newsContext FundDailyNewsContext) FundDailyIndustryExpectationTheme {
	evidence := []FundDailyIndustryExpectationEvidence{}
	riskScore := 0.0
	pricingScore := 0.0
	availableEvidence := 0
	coreMissing := 0

	addEvidence := func(item FundDailyIndustryExpectationEvidence, riskDelta float64, pricingDelta float64) {
		evidence = append(evidence, item)
		if item.Status == "ready" || item.Status == "proxy" {
			availableEvidence++
			riskScore += riskDelta * item.Weight
			pricingScore += pricingDelta * item.Weight
		}
		if item.Status == "missing" {
			coreMissing++
		}
	}

	for _, item := range aggregate.evidence {
		addEvidence(item, fundDailyIndustryExternalRiskDelta(item), fundDailyIndustryExternalPricingDelta(item))
	}

	exposureWeight := aggregate.exposureWeight
	switch {
	case exposureWeight >= 35:
		addEvidence(FundDailyIndustryExpectationEvidence{Category: "portfolio_exposure", Status: "ready", Signal: "concentrated", Value: exposureWeight, Weight: 1, Reason: fmt.Sprintf("%s 持仓暴露约 %.1f%%，组合集中度高。", aggregate.theme, exposureWeight)}, 24, -8)
	case exposureWeight >= 20:
		addEvidence(FundDailyIndustryExpectationEvidence{Category: "portfolio_exposure", Status: "ready", Signal: "meaningful", Value: exposureWeight, Weight: 1, Reason: fmt.Sprintf("%s 持仓暴露约 %.1f%%，需要单独跟踪。", aggregate.theme, exposureWeight)}, 14, -4)
	case exposureWeight > 0:
		addEvidence(FundDailyIndustryExpectationEvidence{Category: "portfolio_exposure", Status: "ready", Signal: "limited", Value: exposureWeight, Weight: 0.6, Reason: fmt.Sprintf("%s 持仓暴露约 %.1f%%。", aggregate.theme, exposureWeight)}, 5, 0)
	}

	if len(aggregate.recentReturns) > 0 {
		avg1, avg3, avg6 := averageFundDailyIndustryRecentReturns(aggregate.recentReturns)
		switch {
		case avg1 >= 10 || avg3 >= 20 || avg6 >= 35:
			addEvidence(FundDailyIndustryExpectationEvidence{Category: "price_momentum", Status: "ready", Signal: "runup_not_priced", Value: maxFloat(avg1, avg3, avg6), Weight: 1, Reason: fmt.Sprintf("%s 相关持仓近 1/3/6 月平均收益 %.1f%% / %.1f%% / %.1f%%，价格仍在强势区间。", aggregate.theme, avg1, avg3, avg6)}, 18, -22)
		case avg1 <= -8 || avg3 <= -15:
			addEvidence(FundDailyIndustryExpectationEvidence{Category: "price_momentum", Status: "ready", Signal: "drawdown_priced", Value: minFloat(avg1, avg3), Weight: 1, Reason: fmt.Sprintf("%s 相关持仓近 1/3 月平均收益 %.1f%% / %.1f%%，风险已有价格回撤反映。", aggregate.theme, avg1, avg3)}, 8, 24)
		default:
			addEvidence(FundDailyIndustryExpectationEvidence{Category: "price_momentum", Status: "ready", Signal: "neutral", Value: avg3, Weight: 0.7, Reason: fmt.Sprintf("%s 相关持仓近期收益没有触发极端阈值。", aggregate.theme)}, 4, 4)
		}
	}

	if tilt, ok := findFundDailyThemeTilt(market.ThemeTilts, aggregate.theme); ok {
		switch {
		case tilt.Score <= -8:
			addEvidence(FundDailyIndustryExpectationEvidence{Category: "market_price_response", Status: "ready", Signal: "risk_reflected", Value: tilt.Score, Weight: 1, Reason: fmt.Sprintf("市场因子对 %s 为 %.1f：%s。", aggregate.theme, tilt.Score, tilt.Reason)}, 10, 24)
		case tilt.Score >= 8:
			addEvidence(FundDailyIndustryExpectationEvidence{Category: "market_price_response", Status: "ready", Signal: "risk_not_reflected", Value: tilt.Score, Weight: 1, Reason: fmt.Sprintf("市场因子对 %s 为 %.1f：%s，尚未体现明显风险折价。", aggregate.theme, tilt.Score, tilt.Reason)}, 8, -18)
		default:
			addEvidence(FundDailyIndustryExpectationEvidence{Category: "market_price_response", Status: "ready", Signal: "neutral", Value: tilt.Score, Weight: 0.6, Reason: fmt.Sprintf("市场因子对 %s 偏中性。", aggregate.theme)}, 2, 3)
		}
	}

	if tilt, ok := findFundDailyThemeTilt(newsContext.ThemeTilts, aggregate.theme); ok {
		switch {
		case tilt.Score <= -6:
			addEvidence(FundDailyIndustryExpectationEvidence{Category: "news_risk", Status: "ready", Signal: "negative_risk", Value: tilt.Score, Weight: 1, Reason: fmt.Sprintf("新闻/国际局势对 %s 为 %.1f：%s。", aggregate.theme, tilt.Score, tilt.Reason)}, 16, -4)
		case tilt.Score >= 6:
			addEvidence(FundDailyIndustryExpectationEvidence{Category: "news_risk", Status: "ready", Signal: "positive_offset", Value: tilt.Score, Weight: 0.8, Reason: fmt.Sprintf("新闻/国际局势对 %s 为 %.1f：%s。", aggregate.theme, tilt.Score, tilt.Reason)}, -3, 8)
		}
	}

	flowEvidence, hasFlow := fundDailyIndustryFlowProxyEvidence(aggregate.theme, market)
	if hasFlow {
		addEvidence(flowEvidence, flowEvidenceRiskDelta(flowEvidence), flowEvidencePricingDelta(flowEvidence))
	}

	industryEvidence, hasIndustryCycle := fundDailyIndustryCycleEvidence(aggregate.theme, market)
	if hasIndustryCycle {
		addEvidence(industryEvidence, industryCycleRiskDelta(industryEvidence), industryCyclePricingDelta(industryEvidence))
	}

	if !hasFundDailyIndustryEvidenceCategory(evidence, "valuation_percentile") {
		addEvidence(FundDailyIndustryExpectationEvidence{Category: "valuation_percentile", Status: "missing", Signal: "missing", Weight: 0, Reason: "尚未接入行业估值分位数据；本项不参与打分，只降低置信度。"}, 0, 0)
	}
	if !hasFundDailyIndustryEvidenceCategory(evidence, "earnings_revision") {
		addEvidence(FundDailyIndustryExpectationEvidence{Category: "earnings_revision", Status: "missing", Signal: "missing", Weight: 0, Reason: "尚未接入行业盈利预期修正数据；本项不参与打分，只降低置信度。"}, 0, 0)
	}

	riskScore = clampFloat(riskScore, 0, 100)
	pricingScore = clampFloat(pricingScore, -60, 80)
	confidence := fundDailyIndustryExpectationConfidence(availableEvidence, coreMissing, riskScore)
	pricingState := fundDailyIndustryPricingState(riskScore, pricingScore, availableEvidence)
	riskPressure := fundDailyIndustryRiskPressure(riskScore)
	reasons := compactDailyReasons(fundDailyIndustryThemeReasons(aggregate.theme, riskPressure, pricingState, evidence), 4)

	return FundDailyIndustryExpectationTheme{
		Theme:              aggregate.theme,
		RiskPressure:       riskPressure,
		PricingState:       pricingState,
		Confidence:         confidence,
		ExposureWeight:     roundFloat(exposureWeight, 2),
		PortfolioFundCount: aggregate.portfolioFundCount,
		CandidateFundCount: aggregate.candidateFundCount,
		RiskScore:          roundFloat(riskScore, 1),
		PricingScore:       roundFloat(pricingScore, 1),
		Effect:             fundDailyIndustryExpectationEffect(riskPressure, pricingState),
		Evidence:           evidence,
		Reasons:            reasons,
		Warnings:           fundDailyIndustryThemeWarnings(evidence),
	}
}

func averageFundDailyIndustryRecentReturns(items []fundDailyIndustryRecentReturn) (float64, float64, float64) {
	if len(items) == 0 {
		return 0, 0, 0
	}
	sum1, sum3, sum6 := 0.0, 0.0, 0.0
	for _, item := range items {
		sum1 += item.month1
		sum3 += item.month3
		sum6 += item.month6
	}
	count := float64(len(items))
	return sum1 / count, sum3 / count, sum6 / count
}

func findFundDailyThemeTilt(tilts []FundDailyMarketThemeTilt, theme string) (FundDailyMarketThemeTilt, bool) {
	for _, tilt := range tilts {
		if tilt.Theme == theme {
			return tilt, true
		}
	}
	return FundDailyMarketThemeTilt{}, false
}

func fundDailyIndustryFlowProxyEvidence(theme string, market FundDailyMarketContext) (FundDailyIndustryExpectationEvidence, bool) {
	quotes := append([]FundDailyMarketQuote{}, market.IndustryGainers...)
	quotes = append(quotes, market.IndustryLosers...)
	best, ok := findFundDailyIndustryQuoteForTheme(theme, quotes)
	if !ok || best.Turnover <= 0 {
		return FundDailyIndustryExpectationEvidence{}, false
	}
	signal := "neutral"
	reason := fmt.Sprintf("%s 作为 %s 的资金/成交代理，涨跌 %.2f%%，成交额 %.0f。", best.Name, theme, best.ChangePercent, best.Turnover)
	if best.ChangePercent >= 2 {
		signal = "crowded_inflow_proxy"
		reason = fmt.Sprintf("%s 作为 %s 的资金/成交代理，放量上涨 %.2f%%，更像风险尚未充分折价。", best.Name, theme, best.ChangePercent)
	}
	if best.ChangePercent <= -2 {
		signal = "derisking_proxy"
		reason = fmt.Sprintf("%s 作为 %s 的资金/成交代理，放量下跌 %.2f%%，风险已有部分价格反应。", best.Name, theme, best.ChangePercent)
	}
	return FundDailyIndustryExpectationEvidence{
		Category: "fund_flow_proxy",
		Status:   "proxy",
		Signal:   signal,
		Value:    best.ChangePercent,
		Weight:   0.7,
		Reason:   reason,
	}, true
}

func fundDailyIndustryCycleEvidence(theme string, market FundDailyMarketContext) (FundDailyIndustryExpectationEvidence, bool) {
	quotes := append([]FundDailyMarketQuote{}, market.IndustryGainers...)
	quotes = append(quotes, market.IndustryLosers...)
	best, ok := findFundDailyIndustryQuoteForTheme(theme, quotes)
	if !ok {
		return FundDailyIndustryExpectationEvidence{}, false
	}
	signal := "neutral"
	reason := fmt.Sprintf("%s 景气/价格代理涨跌 %.2f%%。", best.Name, best.ChangePercent)
	if best.ChangePercent >= 2 {
		signal = "strong_cycle"
		reason = fmt.Sprintf("%s 景气/价格代理强势上涨 %.2f%%，行业仍处在交易热区。", best.Name, best.ChangePercent)
	}
	if best.ChangePercent <= -2 {
		signal = "weak_cycle"
		reason = fmt.Sprintf("%s 景气/价格代理下跌 %.2f%%，行业风险已有价格层面反应。", best.Name, best.ChangePercent)
	}
	return FundDailyIndustryExpectationEvidence{
		Category: "industry_cycle_proxy",
		Status:   "proxy",
		Signal:   signal,
		Value:    best.ChangePercent,
		Weight:   0.7,
		Reason:   reason,
	}, true
}

func findFundDailyIndustryQuoteForTheme(theme string, quotes []FundDailyMarketQuote) (FundDailyMarketQuote, bool) {
	for _, quote := range quotes {
		for _, quoteTheme := range inferMarketThemesFromIndustry(quote.Name) {
			if quoteTheme == theme {
				return quote, true
			}
		}
	}
	return FundDailyMarketQuote{}, false
}

func flowEvidenceRiskDelta(evidence FundDailyIndustryExpectationEvidence) float64 {
	switch evidence.Signal {
	case "crowded_inflow_proxy":
		return 9
	case "derisking_proxy":
		return 6
	default:
		return 2
	}
}

func flowEvidencePricingDelta(evidence FundDailyIndustryExpectationEvidence) float64 {
	switch evidence.Signal {
	case "crowded_inflow_proxy":
		return -14
	case "derisking_proxy":
		return 16
	default:
		return 2
	}
}

func industryCycleRiskDelta(evidence FundDailyIndustryExpectationEvidence) float64 {
	switch evidence.Signal {
	case "strong_cycle":
		return 8
	case "weak_cycle":
		return 6
	default:
		return 2
	}
}

func industryCyclePricingDelta(evidence FundDailyIndustryExpectationEvidence) float64 {
	switch evidence.Signal {
	case "strong_cycle":
		return -12
	case "weak_cycle":
		return 16
	default:
		return 2
	}
}

func fundDailyIndustryExternalRiskDelta(evidence FundDailyIndustryExpectationEvidence) float64 {
	switch evidence.Category {
	case "valuation_percentile":
		switch evidence.Signal {
		case "extreme_expensive":
			return 24
		case "expensive":
			return 16
		case "elevated":
			return 8
		case "cheap":
			return 4
		default:
			return 2
		}
	case "earnings_revision":
		switch evidence.Signal {
		case "downward_revision":
			return 18
		case "upward_revision":
			return -4
		case "stable":
			return 2
		default:
			return 0
		}
	default:
		return 0
	}
}

func fundDailyIndustryExternalPricingDelta(evidence FundDailyIndustryExpectationEvidence) float64 {
	switch evidence.Category {
	case "valuation_percentile":
		switch evidence.Signal {
		case "extreme_expensive":
			return -28
		case "expensive":
			return -18
		case "elevated":
			return -8
		case "cheap":
			return 20
		default:
			return 4
		}
	case "earnings_revision":
		switch evidence.Signal {
		case "downward_revision":
			return -16
		case "upward_revision":
			return 14
		case "stable":
			return 6
		default:
			return 0
		}
	default:
		return 0
	}
}

func fundDailyIndustryExpectationConfidence(availableEvidence int, coreMissing int, riskScore float64) float64 {
	confidence := 30.0 + float64(availableEvidence)*8
	confidence -= float64(coreMissing) * 6
	if riskScore >= 45 {
		confidence += 6
	}
	return clampFloat(confidence, 20, 78)
}

func fundDailyIndustryPricingState(riskScore float64, pricingScore float64, availableEvidence int) string {
	if availableEvidence < 3 {
		return FundDailyIndustryPricingInsufficientData
	}
	if riskScore >= 35 && pricingScore <= -10 {
		return FundDailyIndustryPricingNotFullyPriced
	}
	if pricingScore >= 35 {
		return FundDailyIndustryPricingMostlyPriced
	}
	if riskScore >= 25 {
		return FundDailyIndustryPricingPartiallyPriced
	}
	return FundDailyIndustryPricingInsufficientData
}

func fundDailyIndustryRiskPressure(riskScore float64) string {
	switch {
	case riskScore >= 55:
		return "high"
	case riskScore >= 30:
		return "medium"
	default:
		return "low"
	}
}

func fundDailyIndustryExpectationEffect(riskPressure string, pricingState string) string {
	switch pricingState {
	case FundDailyIndustryPricingNotFullyPriced:
		return "降低同主题加仓，避免追高；只允许分散化小额试探。"
	case FundDailyIndustryPricingPartiallyPriced:
		return "维持谨慎，买入需要更高分散度或更低相关性。"
	case FundDailyIndustryPricingMostlyPriced:
		return "风险已有较多价格反应，可继续观察企稳质量。"
	default:
		if riskPressure == "high" {
			return "证据不足但风险压力高，先按保守处理。"
		}
		return "证据不足，暂不作为强交易约束。"
	}
}

func fundDailyIndustryThemeReasons(theme string, riskPressure string, pricingState string, evidence []FundDailyIndustryExpectationEvidence) []string {
	reasons := []string{fmt.Sprintf("%s：风险压力 %s，市场定价状态 %s。", theme, riskPressure, pricingState)}
	for _, item := range evidence {
		if item.Status == "ready" || item.Status == "proxy" {
			reasons = append(reasons, item.Reason)
		}
	}
	return reasons
}

func fundDailyIndustryThemeWarnings(evidence []FundDailyIndustryExpectationEvidence) []string {
	warnings := []string{}
	for _, item := range evidence {
		if item.Status == "missing" || item.Status == "partial" {
			warnings = append(warnings, item.Reason)
		}
	}
	return compactDailyReasons(warnings, 3)
}

func hasFundDailyIndustryEvidenceCategory(evidence []FundDailyIndustryExpectationEvidence, category string) bool {
	for _, item := range evidence {
		if item.Category == category {
			return true
		}
	}
	return false
}

func fundDailyIndustryExpectationReasons(themes []FundDailyIndustryExpectationTheme) []string {
	reasons := []string{}
	for _, theme := range themes {
		if len(theme.Reasons) > 0 {
			reasons = append(reasons, theme.Reasons[0])
		}
	}
	return reasons
}

func fundDailyIndustryExpectationWarnings(themes []FundDailyIndustryExpectationTheme) []string {
	warnings := []string{}
	for _, theme := range themes {
		warnings = append(warnings, theme.Warnings...)
	}
	return compactDailyReasons(warnings, 4)
}

func fundDailyIndustryExpectationStatus(themes []FundDailyIndustryExpectationTheme) string {
	for _, theme := range themes {
		if theme.PricingState != FundDailyIndustryPricingInsufficientData && theme.Confidence >= 35 {
			return "ready"
		}
	}
	return "unavailable"
}

func fundDailyIndustryExpectationBudgetMultiplier(themes []FundDailyIndustryExpectationTheme) float64 {
	strongestPenalty := 1.0
	for _, theme := range themes {
		if theme.ExposureWeight <= 0 {
			continue
		}
		penalty := 1.0
		switch theme.PricingState {
		case FundDailyIndustryPricingNotFullyPriced:
			if theme.RiskPressure == "high" {
				penalty = 0.88
			} else {
				penalty = 0.93
			}
		case FundDailyIndustryPricingPartiallyPriced:
			if theme.RiskPressure == "high" {
				penalty = 0.94
			} else {
				penalty = 0.97
			}
		case FundDailyIndustryPricingInsufficientData:
			if theme.RiskPressure == "high" {
				penalty = 0.95
			}
		}
		if penalty < strongestPenalty {
			strongestPenalty = penalty
		}
	}
	return roundFloat(clampFloat(strongestPenalty, 0.82, 1.05), 2)
}

func fundDailyIndustryExpectationSummary(contextData FundDailyIndustryExpectationContext) string {
	if contextData.Status != "ready" || len(contextData.Themes) == 0 {
		return "行业风险定价数据暂不可用，本次不调整行业风险因子。"
	}
	top := contextData.Themes[0]
	return fmt.Sprintf("行业风险定价参考：%s 为 %s，风险压力 %s，置信度 %.0f，预算倍率 %.2f。", top.Theme, top.PricingState, top.RiskPressure, top.Confidence, contextData.BudgetMultiplier)
}

func fundDailyIndustryExpectationTiltScoreForFund(fund FundDailyAIFund, contextData FundDailyIndustryExpectationContext) float64 {
	if contextData.Status != "ready" || len(contextData.Themes) == 0 {
		return 0
	}
	themes := fundDailyIndustryThemesForFund(fund)
	if len(themes) == 0 {
		return 0
	}
	score := 0.0
	for _, fundTheme := range themes {
		for _, contextTheme := range contextData.Themes {
			if fundTheme != contextTheme.Theme {
				continue
			}
			switch contextTheme.PricingState {
			case FundDailyIndustryPricingNotFullyPriced:
				if contextTheme.RiskPressure == "high" {
					score -= 12
				} else {
					score -= 8
				}
			case FundDailyIndustryPricingPartiallyPriced:
				score -= 4
			case FundDailyIndustryPricingMostlyPriced:
				score += 2
			case FundDailyIndustryPricingInsufficientData:
				if contextTheme.RiskPressure == "high" {
					score -= 5
				}
			}
		}
	}
	return clampFloat(score, -18, 6)
}

func roundFloat(value float64, digits int) float64 {
	scale := math.Pow10(digits)
	return math.Round(value*scale) / scale
}

func maxFloat(values ...float64) float64 {
	if len(values) == 0 {
		return 0
	}
	max := values[0]
	for _, value := range values[1:] {
		if value > max {
			max = value
		}
	}
	return max
}

func minFloat(values ...float64) float64 {
	if len(values) == 0 {
		return 0
	}
	min := values[0]
	for _, value := range values[1:] {
		if value < min {
			min = value
		}
	}
	return min
}
