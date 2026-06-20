package core

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/axiaoxin-com/investool/models"
)

type FundDailyAdviceConfig struct {
	TargetAnnualReturn               float64
	MaxTotalAmount                   float64
	CashBufferWeight                 float64
	MaxSingleFundWeight              float64
	TacticalWeight                   float64
	MaxDailyBuyWeight                float64
	CandidateCount                   int
	AICandidateCount                 int
	MinCandidateScore                int
	MinCoreCandidateScore            int
	DisableBuyOnNonWorkday           bool
	DisableBuyOnNonWorkdayConfigured bool
	NonWorkdayDates                  []string
	WorkdayDates                     []string
	Now                              time.Time
}

type FundDailyAdviceReport struct {
	GeneratedAt                time.Time
	Config                     FundDailyAdviceConfig
	WorkdayGuard               FundDailyWorkdayGuard
	CurrentAmount              float64
	InvestableAmount           float64
	CashRoom                   float64
	CashBufferAmount           float64
	DailyBuyBudget             float64
	DailyBuyBudgetReasons      []string
	DecisionPortfolioActions   []FundDailyAction
	DecisionCandidateActions   []FundDailyAction
	PortfolioActions           []FundDailyAction
	CandidateActions           []FundDailyAction
	AIContext                  FundDailyAIContext
	AIDecision                 FundDailyAIDecision
	MarketContext              FundDailyMarketContext
	NewsContext                FundDailyNewsContext
	IndustryExpectationSignals []FundDailyIndustryExpectationSignal
	IndustryExpectationContext FundDailyIndustryExpectationContext
	Warnings                   []string
}

type FundDailyAction struct {
	Code                      string
	Name                      string
	FundType                  string
	Source                    string
	IndexName                 string
	TargetETFCode             string
	TargetETFName             string
	Action                    string
	ActionLevel               string
	StrategyScore             float64
	StrategyTheme             string
	TrendScore                float64
	ThemeScore                float64
	CorrelationScore          float64
	MaxCorrelation            float64
	HasCorrelation            bool
	InstitutionalHoldingRatio float64
	InternalHoldingRatio      float64
	SuggestedAmount           float64
	SuggestedWeight           float64
	CurrentAmount             float64
	CurrentWeight             float64
	ProfitRatio               float64
	ProfitAmount              float64
	Score                     int
	RiskLevel                 string
	ExpectedAnnualReturn      float64
	HasExpectedReturn         bool
	Drawdown                  float64
	Stddev                    float64
	Sharp                     float64
	NetAssetsScaleYi          float64
	UnitNAV                   float64
	DailyProfitRatio          float64
	SubscriptionStatus        string
	CanSubscribe              bool
	Month1Return              float64
	Month3Return              float64
	Month6Return              float64
	ThisYearReturn            float64
	Year1Return               float64
	Year3Return               float64
	Year5Return               float64
	Month1RankRatio           float64
	Month3RankRatio           float64
	Month6RankRatio           float64
	ThisYearRankRatio         float64
	Year1RankRatio            float64
	ManagerName               string
	ManagerWorkingYears       float64
	ManagerManageYears        float64
	ManagerManageReturn       float64
	ManagerAnnualReturn       float64
	AssetStock                string
	AssetBond                 string
	AssetCash                 string
	TopStocks                 []FundDailyTopHolding
	Reasons                   []string
	Warnings                  []string
}

type FundDailyTopHolding struct {
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	Industry    string  `json:"industry,omitempty"`
	HoldRatio   float64 `json:"hold_ratio"`
	AdjustRatio float64 `json:"adjust_ratio"`
}

func DefaultFundDailyAdviceConfig() FundDailyAdviceConfig {
	return FundDailyAdviceConfig{
		TargetAnnualReturn:               5,
		MaxTotalAmount:                   5000,
		CashBufferWeight:                 10,
		MaxSingleFundWeight:              35,
		TacticalWeight:                   20,
		MaxDailyBuyWeight:                10,
		CandidateCount:                   8,
		AICandidateCount:                 50,
		MinCandidateScore:                68,
		MinCoreCandidateScore:            75,
		DisableBuyOnNonWorkday:           true,
		DisableBuyOnNonWorkdayConfigured: true,
		NonWorkdayDates:                  DefaultFundDailyWorkdayCalendar().NonWorkdayDates,
		WorkdayDates:                     DefaultFundDailyWorkdayCalendar().WorkdayDates,
	}
}

func BuildFundDailyAdviceReport(
	ctx context.Context,
	portfolioAdvices []FundPortfolioAdvice,
	candidateFunds models.FundList,
	config FundDailyAdviceConfig,
) FundDailyAdviceReport {
	return BuildFundDailyAdviceReportWithEvidence(ctx, portfolioAdvices, candidateFunds, nil, config)
}

func BuildFundDailyAdviceReportWithEvidence(
	ctx context.Context,
	portfolioAdvices []FundPortfolioAdvice,
	candidateFunds models.FundList,
	candidateEvidence map[string]FundDailyCandidateEvidence,
	config FundDailyAdviceConfig,
) FundDailyAdviceReport {
	config = normalizeFundDailyAdviceConfig(config)
	generatedAt := time.Now()
	if !config.Now.IsZero() {
		generatedAt = config.Now
	}
	report := FundDailyAdviceReport{
		GeneratedAt: generatedAt,
		Config:      config,
		WorkdayGuard: BuildFundDailyWorkdayGuard(generatedAt, config.DisableBuyOnNonWorkday, FundDailyWorkdayCalendar{
			NonWorkdayDates: config.NonWorkdayDates,
			WorkdayDates:    config.WorkdayDates,
		}),
		Warnings: []string{"This report is an investment aid only and does not guarantee target returns."},
	}
	report.CurrentAmount = currentAmountFromAdvices(portfolioAdvices)
	report.InvestableAmount = config.MaxTotalAmount
	if report.InvestableAmount <= 0 {
		report.InvestableAmount = report.CurrentAmount
	}
	report.CashBufferAmount = report.InvestableAmount * config.CashBufferWeight / 100
	report.CashRoom = report.InvestableAmount - report.CurrentAmount - report.CashBufferAmount
	if report.CashRoom < 0 {
		report.CashRoom = 0
	}

	report.DecisionPortfolioActions = buildDailyPortfolioActions(portfolioAdvices, report)
	report.DecisionCandidateActions = buildDailyCandidateActions(ctx, candidateFunds, portfolioAdvices, candidateEvidence, report)
	if report.WorkdayGuard.BlocksBuy() {
		report.DecisionPortfolioActions, report.DecisionCandidateActions = applyFundDailyWorkdayGuard(
			report.DecisionPortfolioActions,
			report.DecisionCandidateActions,
			report.WorkdayGuard,
		)
	}
	decision := chooseDailyBuyBudget(report.DecisionPortfolioActions, report.DecisionCandidateActions, report)
	if report.WorkdayGuard.BlocksBuy() {
		decision.Budget = 0
		decision.Confidence = 0
		decision.Reasons = prependUniqueDailyReason(decision.Reasons, report.WorkdayGuard.Reason)
		report.Warnings = append(report.Warnings, report.WorkdayGuard.Reason)
	}
	report.DailyBuyBudget = decision.Budget
	report.DailyBuyBudgetReasons = decision.Reasons
	report.PortfolioActions, report.CandidateActions = applyDailyBuyBudget(
		cloneFundDailyActions(report.DecisionPortfolioActions),
		cloneFundDailyActions(report.DecisionCandidateActions),
		report,
	)
	report.CandidateActions = limitDailyCandidateActionsForDisplay(report.CandidateActions, report.Config.CandidateCount)
	if report.CurrentAmount > config.MaxTotalAmount {
		report.Warnings = append(report.Warnings, fmt.Sprintf("Current holding value %.2f exceeds configured max %.2f; new buys are disabled until exposure is reduced.", report.CurrentAmount, config.MaxTotalAmount))
	}
	if len(report.CandidateActions) == 0 {
		report.Warnings = append(report.Warnings, "No candidate fund passed the current score and risk filters. Refresh the local fund cache or lower candidate thresholds if this persists.")
	}
	return report
}

func buildDailyPortfolioActions(advices []FundPortfolioAdvice, report FundDailyAdviceReport) []FundDailyAction {
	actions := make([]FundDailyAction, 0, len(advices))
	for _, advice := range advices {
		if !advice.Item.IsOwned() {
			continue
		}
		action := dailyActionFromAdvice(advice, "持有基金", report)
		actions = append(actions, action)
	}
	sort.SliceStable(actions, func(i, j int) bool {
		return dailyActionPriority(actions[i]) > dailyActionPriority(actions[j])
	})
	return actions
}

func buildDailyCandidateActions(ctx context.Context, funds models.FundList, portfolioAdvices []FundPortfolioAdvice, candidateEvidence map[string]FundDailyCandidateEvidence, report FundDailyAdviceReport) []FundDailyAction {
	ownedOrWatched := map[string]struct{}{}
	for _, advice := range portfolioAdvices {
		ownedOrWatched[advice.Item.Code] = struct{}{}
	}
	actions := []FundDailyAction{}
	for _, fund := range funds {
		if fund == nil {
			continue
		}
		if _, exists := ownedOrWatched[fund.Code]; exists {
			continue
		}
		advice := EvaluateFundPortfolioItem(ctx, models.FundPortfolioItem{
			Code:   fund.Code,
			Status: models.FundPortfolioStatusWatch,
		}, fund, fundHolderStructureFromFund(fund))
		status := strings.TrimSpace(fund.SubscriptionStatus)
		action := dailyActionFromAdvice(advice, "候选池", report)
		if !fund.CanSubscribe() {
			action.Action = "暂停申购，观察"
			action.ActionLevel = "watch"
			action.SuggestedAmount = 0
			action.SuggestedWeight = 0
			if status == "" {
				status = "未读取到申购状态"
			}
			action.Warnings = append(action.Warnings, fmt.Sprintf("申购状态为“%s”，当前暂停申购，今日不建议加仓。", status))
			if evidence, ok := candidateEvidence[fund.Code]; ok {
				applyFundDailyCandidateEvidence(&action, evidence)
			}
			actions = append(actions, action)
			continue
		}
		if advice.Score < report.Config.MinCandidateScore {
			continue
		}
		if advice.HasExpectedAnnualReturn && advice.ExpectedAnnualReturn < report.Config.TargetAnnualReturn-1 {
			continue
		}
		if fund.MaxRetracement.Avg135 > 35 || fund.Stddev.Avg135 > 35 {
			continue
		}
		action = dailyActionFromAdvice(advice, "候选基金", report)
		if advice.Score >= report.Config.MinCoreCandidateScore && fund.MaxRetracement.Avg135 <= 25 {
			action.Action = "候选核心买入"
			action.ActionLevel = "buy"
		} else {
			action.Action = "小额观察买入"
			action.ActionLevel = "watch"
		}
		if evidence, ok := candidateEvidence[fund.Code]; ok {
			applyFundDailyCandidateEvidence(&action, evidence)
		}
		action.SuggestedAmount = candidateSuggestedAmount(action, report)
		action.SuggestedWeight = amountToWeight(action.SuggestedAmount, report.InvestableAmount)
		actions = append(actions, action)
	}
	actions = preferClassCFundDailyActions(actions)
	sort.SliceStable(actions, func(i, j int) bool {
		if actions[i].StrategyScore != actions[j].StrategyScore {
			return actions[i].StrategyScore > actions[j].StrategyScore
		}
		if actions[i].Score != actions[j].Score {
			return actions[i].Score > actions[j].Score
		}
		if actions[i].ExpectedAnnualReturn != actions[j].ExpectedAnnualReturn {
			return actions[i].ExpectedAnnualReturn > actions[j].ExpectedAnnualReturn
		}
		return actions[i].Code < actions[j].Code
	})
	return actions
}

func cloneFundDailyActions(actions []FundDailyAction) []FundDailyAction {
	if len(actions) == 0 {
		return nil
	}
	cloned := make([]FundDailyAction, len(actions))
	copy(cloned, actions)
	return cloned
}

func limitDailyCandidateActionsForDisplay(actions []FundDailyAction, limit int) []FundDailyAction {
	if limit <= 0 || len(actions) <= limit {
		return actions
	}
	display := cloneFundDailyActions(actions)
	sort.SliceStable(display, func(i, j int) bool {
		leftBuy := display[i].SuggestedAmount > 0
		rightBuy := display[j].SuggestedAmount > 0
		if leftBuy != rightBuy {
			return leftBuy
		}
		if display[i].StrategyScore != display[j].StrategyScore {
			return display[i].StrategyScore > display[j].StrategyScore
		}
		if display[i].Score != display[j].Score {
			return display[i].Score > display[j].Score
		}
		return display[i].Code < display[j].Code
	})
	return display[:limit]
}

func dailyActionFromAdvice(advice FundPortfolioAdvice, source string, report FundDailyAdviceReport) FundDailyAction {
	action := FundDailyAction{
		Code:                 advice.Item.Code,
		Source:               source,
		Score:                advice.Score,
		RiskLevel:            advice.RiskLevel,
		ExpectedAnnualReturn: advice.ExpectedAnnualReturn,
		HasExpectedReturn:    advice.HasExpectedAnnualReturn,
		CurrentAmount:        advice.CurrentAmount,
		CurrentWeight:        advice.CurrentWeight,
		ProfitRatio:          advice.ProfitRatio,
		ProfitAmount:         advice.ProfitAmount,
		Reasons:              compactDailyReasons(advice.Reasons, 3),
		Warnings:             advice.Warnings,
	}
	if advice.Fund != nil {
		fillDailyFundActionMetrics(&action, advice.Fund)
	}
	if advice.HolderStructure.Latest != nil {
		action.InstitutionalHoldingRatio = advice.HolderStructure.Latest.InstitutionalHoldingRatio
		action.InternalHoldingRatio = advice.HolderStructure.Latest.InternalHoldingRatio
	}
	if action.Name == "" {
		action.Name = advice.Item.Code
	}
	if advice.Item.IsOwned() {
		fillOwnedDailyAction(&action, advice, report)
		return action
	}
	action.Action = advice.Action
	action.ActionLevel = "watch"
	return action
}

func fillDailyFundActionMetrics(action *FundDailyAction, fund *models.Fund) {
	action.Name = fund.Name
	action.FundType = fund.Type
	action.IndexName = fund.IndexName
	action.TargetETFCode = fund.TargetETFCode
	action.TargetETFName = fund.TargetETFName
	action.Drawdown = fund.MaxRetracement.Avg135
	action.Stddev = fund.Stddev.Avg135
	action.Sharp = fund.Sharp.Avg135
	action.InstitutionalHoldingRatio = fund.InstitutionalHoldingRatio
	action.InternalHoldingRatio = fund.InternalHoldingRatio
	action.NetAssetsScaleYi = fund.NetAssetsScale / 100000000
	action.UnitNAV = fund.UnitNav
	action.DailyProfitRatio = fund.DailyProfitRatio
	action.SubscriptionStatus = strings.TrimSpace(fund.SubscriptionStatus)
	action.CanSubscribe = fund.CanSubscribe()
	action.Month1Return = fund.Performance.Month1ProfitRatio
	action.Month3Return = fund.Performance.Month3ProfitRatio
	action.Month6Return = fund.Performance.Month6ProfitRatio
	action.ThisYearReturn = fund.Performance.ThisYearProfitRatio
	action.Year1Return = fund.Performance.Year1ProfitRatio
	action.Year3Return = fund.Performance.Year3ProfitRatio
	action.Year5Return = fund.Performance.Year5ProfitRatio
	action.Month1RankRatio = fund.Performance.Month1RankRatio
	action.Month3RankRatio = fund.Performance.Month3RankRatio
	action.Month6RankRatio = fund.Performance.Month6RankRatio
	action.ThisYearRankRatio = fund.Performance.ThisYearRankRatio
	action.Year1RankRatio = fund.Performance.Year1RankRatio
	action.ManagerName = fund.Manager.Name
	action.ManagerWorkingYears = fund.Manager.WorkingDays / 365
	action.ManagerManageYears = fund.Manager.ManageDays / 365
	action.ManagerManageReturn = fund.Manager.ManageRepay
	action.ManagerAnnualReturn = fund.Manager.YearsAvgRepay
	action.AssetStock = fund.AssetsProportion.Stock
	action.AssetBond = fund.AssetsProportion.Bond
	action.AssetCash = fund.AssetsProportion.Cash
	action.TopStocks = make([]FundDailyTopHolding, 0, minInt(5, len(fund.Stocks)))
	for idx, stock := range fund.Stocks {
		if idx >= 5 {
			break
		}
		action.TopStocks = append(action.TopStocks, FundDailyTopHolding{
			Code:        stock.Code,
			Name:        stock.Name,
			Industry:    stock.Industry,
			HoldRatio:   stock.HoldRatio,
			AdjustRatio: stock.AdjustRatio,
		})
	}
}

func applyFundDailyCandidateEvidence(action *FundDailyAction, evidence FundDailyCandidateEvidence) {
	action.StrategyScore = evidence.StrategyScore
	action.StrategyTheme = evidence.Theme
	action.TrendScore = evidence.TrendScore
	action.ThemeScore = evidence.ThemeScore
	action.CorrelationScore = evidence.CorrelationScore
	action.MaxCorrelation = evidence.MaxCorrelation
	action.HasCorrelation = evidence.HasCorrelation
	reasons := compactDailyReasons(evidence.Reasons, 4)
	reasons = append(reasons, action.Reasons...)
	action.Reasons = compactDailyReasons(reasons, 6)
	action.Warnings = append(action.Warnings, evidence.Warnings...)
}

func fillOwnedDailyAction(action *FundDailyAction, advice FundPortfolioAdvice, report FundDailyAdviceReport) {
	if advice.Fund != nil && !advice.Fund.CanSubscribe() {
		status := strings.TrimSpace(advice.Fund.SubscriptionStatus)
		if status == "" {
			status = "未读取到申购状态"
		}
		action.Action = "暂停申购，暂不加仓"
		action.ActionLevel = "watch"
		action.SuggestedAmount = 0
		action.SuggestedWeight = 0
		action.Warnings = append(action.Warnings, fmt.Sprintf("持仓基金申购状态为“%s”，当前暂停申购。", status))
		return
	}

	maxSingleAmount := report.InvestableAmount * report.Config.MaxSingleFundWeight / 100
	recommendedAmount := report.InvestableAmount * advice.RecommendedWeight / 100
	if recommendedAmount > maxSingleAmount {
		recommendedAmount = maxSingleAmount
	}
	gapAmount := recommendedAmount - advice.CurrentAmount

	switch {
	case advice.Score < 50:
		action.Action = "减仓或放弃"
		action.ActionLevel = "sell"
		action.SuggestedAmount = -roundDailyAmount(minPositive(advice.CurrentAmount*0.5, advice.CurrentAmount))
	case advice.Score < 65:
		action.Action = "停止加仓"
		action.ActionLevel = "hold"
		action.SuggestedAmount = 0
	case gapAmount > 50 && report.CashRoom > 0 && advice.Score >= 75:
		action.Action = "可加仓"
		action.ActionLevel = "buy"
		action.SuggestedAmount = roundDailyAmount(minPositive(gapAmount, report.CashRoom, 500))
	case gapAmount < -50:
		action.Action = "适度降仓"
		action.ActionLevel = "trim"
		action.SuggestedAmount = -roundDailyAmount(minPositive(-gapAmount, advice.CurrentAmount*0.3))
	default:
		action.Action = "继续持有"
		action.ActionLevel = "hold"
		action.SuggestedAmount = 0
	}
	action.SuggestedWeight = amountToWeight(action.SuggestedAmount, report.InvestableAmount)
	if advice.HasExpectedAnnualReturn && advice.ExpectedAnnualReturn < report.Config.TargetAnnualReturn && advice.Score < 75 {
		action.Reasons = append([]string{"估算年化低于目标且评分未达到核心加仓线。"}, action.Reasons...)
	}
}

func candidateSuggestedAmount(action FundDailyAction, report FundDailyAdviceReport) float64 {
	if report.CashRoom <= 0 {
		return 0
	}
	maxTacticalAmount := report.InvestableAmount * report.Config.TacticalWeight / 100
	base := 100.0
	if action.ActionLevel == "buy" {
		base = 200
	}
	if action.Score >= 85 {
		base += 100
	}
	amount := minPositive(base, report.CashRoom, maxTacticalAmount)
	return roundDailyAmount(amount)
}

func normalizeFundDailyAdviceConfig(config FundDailyAdviceConfig) FundDailyAdviceConfig {
	defaults := DefaultFundDailyAdviceConfig()
	if config.TargetAnnualReturn <= 0 {
		config.TargetAnnualReturn = defaults.TargetAnnualReturn
	}
	if config.MaxTotalAmount <= 0 {
		config.MaxTotalAmount = defaults.MaxTotalAmount
	}
	if config.CashBufferWeight < 0 {
		config.CashBufferWeight = defaults.CashBufferWeight
	}
	if config.CashBufferWeight > 50 {
		config.CashBufferWeight = 50
	}
	if config.MaxSingleFundWeight <= 0 {
		config.MaxSingleFundWeight = defaults.MaxSingleFundWeight
	}
	if config.TacticalWeight <= 0 {
		config.TacticalWeight = defaults.TacticalWeight
	}
	if config.MaxDailyBuyWeight <= 0 {
		config.MaxDailyBuyWeight = defaults.MaxDailyBuyWeight
	}
	if config.MaxDailyBuyWeight > 50 {
		config.MaxDailyBuyWeight = 50
	}
	if config.CandidateCount <= 0 {
		config.CandidateCount = defaults.CandidateCount
	}
	if config.AICandidateCount <= 0 {
		config.AICandidateCount = defaults.AICandidateCount
	}
	if config.AICandidateCount < config.CandidateCount {
		config.AICandidateCount = config.CandidateCount
	}
	if config.MinCandidateScore <= 0 {
		config.MinCandidateScore = defaults.MinCandidateScore
	}
	if config.MinCoreCandidateScore <= 0 {
		config.MinCoreCandidateScore = defaults.MinCoreCandidateScore
	}
	if !config.DisableBuyOnNonWorkdayConfigured {
		config.DisableBuyOnNonWorkday = defaults.DisableBuyOnNonWorkday
	}
	if len(config.NonWorkdayDates) == 0 {
		config.NonWorkdayDates = defaults.NonWorkdayDates
	}
	if len(config.WorkdayDates) == 0 {
		config.WorkdayDates = defaults.WorkdayDates
	}
	config.NonWorkdayDates = normalizeFundDailyCalendarDates(config.NonWorkdayDates)
	config.WorkdayDates = normalizeFundDailyCalendarDates(config.WorkdayDates)
	return config
}

func currentAmountFromAdvices(advices []FundPortfolioAdvice) float64 {
	total := 0.0
	for _, advice := range advices {
		if advice.Item.IsOwned() && advice.CurrentAmount > 0 {
			total += advice.CurrentAmount
		}
	}
	return total
}

func compactDailyReasons(reasons []string, max int) []string {
	result := []string{}
	for _, reason := range reasons {
		reason = strings.TrimSpace(reason)
		if reason == "" {
			continue
		}
		result = append(result, reason)
		if len(result) >= max {
			break
		}
	}
	return result
}

func dailyActionPriority(action FundDailyAction) int {
	switch action.ActionLevel {
	case "buy":
		return 5
	case "sell":
		return 4
	case "trim":
		return 3
	case "watch":
		return 2
	default:
		return 1
	}
}

func minPositive(values ...float64) float64 {
	result := 0.0
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if result == 0 || value < result {
			result = value
		}
	}
	return result
}

func roundDailyAmount(amount float64) float64 {
	if amount <= 0 {
		return 0
	}
	return mathRound(amount/10) * 10
}

func amountToWeight(amount float64, total float64) float64 {
	if total <= 0 {
		return 0
	}
	return amount / total * 100
}

func mathRound(n float64) float64 {
	if n >= 0 {
		return float64(int(n + 0.5))
	}
	return float64(int(n - 0.5))
}

func (r FundDailyAdviceReport) GeneratedAtText() string {
	return r.GeneratedAt.Format("2006-01-02 15:04:05")
}

func (r FundDailyAdviceReport) CurrentAmountText() string {
	return fmt.Sprintf("%.2f", r.CurrentAmount)
}

func (r FundDailyAdviceReport) CashRoomText() string {
	return fmt.Sprintf("%.2f", r.CashRoom)
}

func (r FundDailyAdviceReport) DailyBuyBudgetText() string {
	return fmt.Sprintf("%.2f", r.DailyBuyBudget)
}

func (r FundDailyAdviceReport) InvestableAmountText() string {
	return fmt.Sprintf("%.2f", r.InvestableAmount)
}

func (a FundDailyAction) SuggestedAmountText() string {
	if a.SuggestedAmount == 0 {
		return "--"
	}
	return fmt.Sprintf("%+.2f", a.SuggestedAmount)
}

func (a FundDailyAction) ExpectedAnnualReturnText() string {
	if !a.HasExpectedReturn {
		return "--"
	}
	return fmt.Sprintf("%.2f%%", a.ExpectedAnnualReturn)
}

func (a FundDailyAction) CurrentWeightText() string {
	if a.CurrentWeight <= 0 {
		return "--"
	}
	return fmt.Sprintf("%.1f%%", a.CurrentWeight)
}

func (a FundDailyAction) SuggestedWeightText() string {
	if a.SuggestedWeight == 0 {
		return "--"
	}
	return fmt.Sprintf("%+.1f%%", a.SuggestedWeight)
}

func (a FundDailyAction) StrategyScoreText() string {
	if a.StrategyScore <= 0 {
		return "--"
	}
	return fmt.Sprintf("%.1f", a.StrategyScore)
}

func (a FundDailyAction) CorrelationText() string {
	if !a.HasCorrelation {
		return "--"
	}
	return fmt.Sprintf("%.2f", a.MaxCorrelation)
}

func (a FundDailyAction) BadgeClass() string {
	switch a.ActionLevel {
	case "buy":
		return "green"
	case "sell":
		return "red"
	case "trim":
		return "orange"
	case "watch":
		return "blue"
	default:
		return "grey"
	}
}
