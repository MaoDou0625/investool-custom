package core

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/axiaoxin-com/investool/datacenter/eastmoney"
	"github.com/axiaoxin-com/investool/models"
)

type FundDailyAdviceConfig struct {
	TargetAnnualReturn    float64
	MaxTotalAmount        float64
	CashBufferWeight      float64
	MaxSingleFundWeight   float64
	TacticalWeight        float64
	CandidateCount        int
	MinCandidateScore     int
	MinCoreCandidateScore int
}

type FundDailyAdviceReport struct {
	GeneratedAt      time.Time
	Config           FundDailyAdviceConfig
	CurrentAmount    float64
	InvestableAmount float64
	CashRoom         float64
	CashBufferAmount float64
	PortfolioActions []FundDailyAction
	CandidateActions []FundDailyAction
	Warnings         []string
}

type FundDailyAction struct {
	Code                 string
	Name                 string
	FundType             string
	Source               string
	Action               string
	ActionLevel          string
	SuggestedAmount      float64
	SuggestedWeight      float64
	CurrentAmount        float64
	CurrentWeight        float64
	Score                int
	RiskLevel            string
	ExpectedAnnualReturn float64
	HasExpectedReturn    bool
	Drawdown             float64
	Stddev               float64
	Sharp                float64
	Reasons              []string
	Warnings             []string
}

func DefaultFundDailyAdviceConfig() FundDailyAdviceConfig {
	return FundDailyAdviceConfig{
		TargetAnnualReturn:    5,
		MaxTotalAmount:        5000,
		CashBufferWeight:      10,
		MaxSingleFundWeight:   35,
		TacticalWeight:        20,
		CandidateCount:        8,
		MinCandidateScore:     68,
		MinCoreCandidateScore: 75,
	}
}

func BuildFundDailyAdviceReport(
	ctx context.Context,
	portfolioAdvices []FundPortfolioAdvice,
	candidateFunds models.FundList,
	config FundDailyAdviceConfig,
) FundDailyAdviceReport {
	config = normalizeFundDailyAdviceConfig(config)
	report := FundDailyAdviceReport{
		GeneratedAt: time.Now(),
		Config:      config,
		Warnings:    []string{"This report is an investment aid only and does not guarantee target returns."},
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

	report.PortfolioActions = buildDailyPortfolioActions(portfolioAdvices, report)
	report.CandidateActions = buildDailyCandidateActions(ctx, candidateFunds, portfolioAdvices, report)
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

func buildDailyCandidateActions(ctx context.Context, funds models.FundList, portfolioAdvices []FundPortfolioAdvice, report FundDailyAdviceReport) []FundDailyAction {
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
		}, fund, eastmoney.FundHolderStructureResult{})
		if advice.Score < report.Config.MinCandidateScore {
			continue
		}
		if advice.HasExpectedAnnualReturn && advice.ExpectedAnnualReturn < report.Config.TargetAnnualReturn-1 {
			continue
		}
		if fund.MaxRetracement.Avg135 > 35 || fund.Stddev.Avg135 > 35 {
			continue
		}
		action := dailyActionFromAdvice(advice, "候选基金", report)
		if advice.Score >= report.Config.MinCoreCandidateScore && fund.MaxRetracement.Avg135 <= 25 {
			action.Action = "候选核心买入"
			action.ActionLevel = "buy"
		} else {
			action.Action = "小额观察买入"
			action.ActionLevel = "watch"
		}
		action.SuggestedAmount = candidateSuggestedAmount(action, report)
		action.SuggestedWeight = amountToWeight(action.SuggestedAmount, report.InvestableAmount)
		actions = append(actions, action)
	}
	actions = preferClassCFundDailyActions(actions)
	sort.SliceStable(actions, func(i, j int) bool {
		if actions[i].Score != actions[j].Score {
			return actions[i].Score > actions[j].Score
		}
		if actions[i].ExpectedAnnualReturn != actions[j].ExpectedAnnualReturn {
			return actions[i].ExpectedAnnualReturn > actions[j].ExpectedAnnualReturn
		}
		return actions[i].Code < actions[j].Code
	})
	if len(actions) > report.Config.CandidateCount {
		actions = actions[:report.Config.CandidateCount]
	}
	return actions
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
		Reasons:              compactDailyReasons(advice.Reasons, 3),
		Warnings:             advice.Warnings,
	}
	if advice.Fund != nil {
		action.Name = advice.Fund.Name
		action.FundType = advice.Fund.Type
		action.Drawdown = advice.Fund.MaxRetracement.Avg135
		action.Stddev = advice.Fund.Stddev.Avg135
		action.Sharp = advice.Fund.Sharp.Avg135
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

func fillOwnedDailyAction(action *FundDailyAction, advice FundPortfolioAdvice, report FundDailyAdviceReport) {
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
	base := 200.0
	if action.ActionLevel == "buy" {
		base = 400
	}
	if action.Score >= 85 {
		base += 200
	}
	amount := minPositive(base, report.CashRoom, maxTacticalAmount)
	return roundDailyAmount(amount)
}

func SelectDailyAdviceCandidates(ctx context.Context, allFunds models.FundList, fallback models.FundList, maxCount int) models.FundList {
	if maxCount <= 0 {
		maxCount = 80
	}
	source := allFunds
	if len(source) == 0 {
		source = fallback
	}
	if len(source) == 0 {
		return nil
	}
	options := DefaultFund4433RecommendationOptions()
	options.MaxCount = maxCount
	options.MinRankFields = 2
	candidates := BuildFund4433Recommendations(ctx, source, options)
	if len(candidates) == 0 && len(fallback) > 0 {
		candidates = BuildFund4433Recommendations(ctx, fallback, options)
	}
	return candidates
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
	if config.CandidateCount <= 0 {
		config.CandidateCount = defaults.CandidateCount
	}
	if config.MinCandidateScore <= 0 {
		config.MinCandidateScore = defaults.MinCandidateScore
	}
	if config.MinCoreCandidateScore <= 0 {
		config.MinCoreCandidateScore = defaults.MinCoreCandidateScore
	}
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
