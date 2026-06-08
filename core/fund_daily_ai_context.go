package core

import "fmt"

const fundDailyAIContextSchemaVersion = "fund_daily_advice_context.v1"

type FundDailyAIContext struct {
	SchemaVersion    string                      `json:"schema_version"`
	GeneratedAt      string                      `json:"generated_at"`
	Goal             FundDailyAIGoal             `json:"goal"`
	Constraints      FundDailyAIConstraints      `json:"constraints"`
	PortfolioSummary FundDailyAIPortfolioSummary `json:"portfolio_summary"`
	BudgetInput      FundDailyAIBudgetInput      `json:"budget_input"`
	MarketContext    FundDailyMarketContext      `json:"market_context"`
	Portfolio        []FundDailyAIFund           `json:"portfolio"`
	Candidates       []FundDailyAIFund           `json:"candidates"`
	OutputContract   FundDailyAIOutputContract   `json:"output_contract"`
	ValidationRules  []string                    `json:"validation_rules"`
	Warnings         []string                    `json:"warnings,omitempty"`
}

type FundDailyAIGoal struct {
	TargetAnnualReturn  float64 `json:"target_annual_return_percent"`
	MaxTotalAmount      float64 `json:"max_total_amount"`
	CanAcceptVolatility bool    `json:"can_accept_short_term_volatility"`
	PrimaryObjective    string  `json:"primary_objective"`
}

type FundDailyAIConstraints struct {
	MaxTotalAmount        float64 `json:"max_total_amount"`
	CurrentAmount         float64 `json:"current_amount"`
	InvestableAmount      float64 `json:"investable_amount"`
	CashBufferAmount      float64 `json:"cash_buffer_amount"`
	CashRoom              float64 `json:"cash_room"`
	MaxSingleFundWeight   float64 `json:"max_single_fund_weight_percent"`
	MaxDailyBuyWeight     float64 `json:"max_daily_buy_weight_percent"`
	MaxDailyBuyAmount     float64 `json:"max_daily_buy_amount"`
	DisplayCandidateCount int     `json:"display_candidate_count"`
	AICandidateCount      int     `json:"ai_candidate_count"`
}

type FundDailyAIPortfolioSummary struct {
	OwnedFundCount   int     `json:"owned_fund_count"`
	CandidateCount   int     `json:"candidate_count"`
	CurrentWeight    float64 `json:"current_weight_percent"`
	BuyableItemCount int     `json:"buyable_item_count"`
}

type FundDailyAIBudgetInput struct {
	ProgramBudget           float64  `json:"program_budget"`
	ProgramBudgetReasons    []string `json:"program_budget_reasons"`
	MaxProgramAllowedBudget float64  `json:"max_program_allowed_budget"`
	Notes                   []string `json:"notes"`
}

type FundDailyAIFund struct {
	Code                      string                `json:"code"`
	Name                      string                `json:"name"`
	Source                    string                `json:"source"`
	FundType                  string                `json:"fund_type"`
	ProgramAction             string                `json:"program_action"`
	ProgramActionLevel        string                `json:"program_action_level"`
	SuggestedBuyCeiling       float64               `json:"suggested_buy_ceiling"`
	SuggestedSellAmount       float64               `json:"suggested_sell_amount"`
	CurrentAmount             float64               `json:"current_amount"`
	CurrentWeight             float64               `json:"current_weight_percent"`
	SuggestedWeight           float64               `json:"suggested_weight_percent"`
	Score                     int                   `json:"score"`
	RiskLevel                 string                `json:"risk_level"`
	ExpectedAnnualReturn      *float64              `json:"expected_annual_return_percent,omitempty"`
	StrategyScore             float64               `json:"strategy_score"`
	StrategyTheme             string                `json:"strategy_theme,omitempty"`
	TrendScore                float64               `json:"trend_score"`
	ThemeScore                float64               `json:"theme_score"`
	CorrelationScore          float64               `json:"correlation_score"`
	MaxCorrelation            *float64              `json:"max_correlation,omitempty"`
	InstitutionalHoldingRatio float64               `json:"institutional_holding_ratio"`
	InternalHoldingRatio      float64               `json:"internal_holding_ratio"`
	Drawdown                  float64               `json:"drawdown_percent"`
	Stddev                    float64               `json:"stddev_percent"`
	Sharp                     float64               `json:"sharp"`
	NetAssetsScaleYi          float64               `json:"net_assets_scale_yi"`
	UnitNAV                   float64               `json:"unit_nav"`
	DailyProfitRatio          float64               `json:"daily_profit_ratio_percent"`
	SubscriptionStatus        string                `json:"subscription_status"`
	CanSubscribe              bool                  `json:"can_subscribe"`
	RecentReturns             FundDailyAIReturns    `json:"recent_returns"`
	RankRatios                FundDailyAIRankRatios `json:"rank_ratios"`
	Manager                   FundDailyAIManager    `json:"manager"`
	IndexName                 string                `json:"index_name,omitempty"`
	TargetETFCode             string                `json:"target_etf_code,omitempty"`
	TargetETFName             string                `json:"target_etf_name,omitempty"`
	Assets                    FundDailyAIAssets     `json:"assets"`
	TopStocks                 []FundDailyTopHolding `json:"top_stocks,omitempty"`
	Reasons                   []string              `json:"reasons,omitempty"`
	Warnings                  []string              `json:"warnings,omitempty"`
}

type FundDailyAIReturns struct {
	Month1   float64 `json:"month_1_percent"`
	Month3   float64 `json:"month_3_percent"`
	Month6   float64 `json:"month_6_percent"`
	ThisYear float64 `json:"this_year_percent"`
	Year1    float64 `json:"year_1_percent"`
	Year3    float64 `json:"year_3_percent"`
	Year5    float64 `json:"year_5_percent"`
}

type FundDailyAIRankRatios struct {
	Month1   float64 `json:"month_1_percentile"`
	Month3   float64 `json:"month_3_percentile"`
	Month6   float64 `json:"month_6_percentile"`
	ThisYear float64 `json:"this_year_percentile"`
	Year1    float64 `json:"year_1_percentile"`
}

type FundDailyAIManager struct {
	Name         string  `json:"name,omitempty"`
	WorkingYears float64 `json:"working_years"`
	ManageYears  float64 `json:"manage_years"`
	ManageReturn float64 `json:"manage_return_percent"`
	AnnualReturn float64 `json:"annual_return_percent"`
}

type FundDailyAIAssets struct {
	Stock string `json:"stock_percent,omitempty"`
	Bond  string `json:"bond_percent,omitempty"`
	Cash  string `json:"cash_percent,omitempty"`
}

type FundDailyAIOutputContract struct {
	RequiredFormat string   `json:"required_format"`
	RequiredFields []string `json:"required_fields"`
	ActionFields   []string `json:"action_fields"`
	AllowedActions []string `json:"allowed_actions"`
}

type FundDailyAIDecision struct {
	Status         string                    `json:"status"`
	Provider       string                    `json:"provider,omitempty"`
	Model          string                    `json:"model,omitempty"`
	GeneratedAt    string                    `json:"generated_at,omitempty"`
	Summary        string                    `json:"summary,omitempty"`
	DailyBuyBudget float64                   `json:"daily_buy_budget"`
	Actions        []FundDailyAIOutputAction `json:"actions,omitempty"`
	Reasons        []string                  `json:"reasons,omitempty"`
	RiskNotes      []string                  `json:"risk_notes,omitempty"`
	Warnings       []string                  `json:"warnings,omitempty"`
	Validated      bool                      `json:"validated"`
}

type FundDailyAIOutputAction struct {
	Code     string  `json:"code"`
	Name     string  `json:"name,omitempty"`
	FundType string  `json:"fund_type,omitempty"`
	Source   string  `json:"source"`
	Action   string  `json:"action"`
	Amount   float64 `json:"amount"`
	Reason   string  `json:"reason"`
	RiskNote string  `json:"risk_note"`
}

func BuildFundDailyAIContext(report FundDailyAdviceReport) FundDailyAIContext {
	portfolioActions := report.DecisionPortfolioActions
	if len(portfolioActions) == 0 {
		portfolioActions = report.PortfolioActions
	}
	candidateActions := report.DecisionCandidateActions
	if len(candidateActions) == 0 {
		candidateActions = report.CandidateActions
	}
	candidateTotal := len(candidateActions)
	if report.Config.AICandidateCount > 0 && len(candidateActions) > report.Config.AICandidateCount {
		candidateActions = candidateActions[:report.Config.AICandidateCount]
	}

	maxBudget := maxDailyBuyBudget(report)
	buyableCount := 0
	for _, action := range append(cloneFundDailyActions(portfolioActions), candidateActions...) {
		if action.SuggestedAmount > 0 {
			buyableCount++
		}
	}

	currentWeight := 0.0
	if report.InvestableAmount > 0 {
		currentWeight = report.CurrentAmount / report.InvestableAmount * 100
	}

	return FundDailyAIContext{
		SchemaVersion: fundDailyAIContextSchemaVersion,
		GeneratedAt:   report.GeneratedAt.Format("2006-01-02 15:04:05"),
		Goal: FundDailyAIGoal{
			TargetAnnualReturn:  report.Config.TargetAnnualReturn,
			MaxTotalAmount:      report.Config.MaxTotalAmount,
			CanAcceptVolatility: true,
			PrimaryObjective:    "Seek a relatively stable fund portfolio with annual return above target while keeping total investment within the configured cap.",
		},
		Constraints: FundDailyAIConstraints{
			MaxTotalAmount:        report.Config.MaxTotalAmount,
			CurrentAmount:         report.CurrentAmount,
			InvestableAmount:      report.InvestableAmount,
			CashBufferAmount:      report.CashBufferAmount,
			CashRoom:              report.CashRoom,
			MaxSingleFundWeight:   report.Config.MaxSingleFundWeight,
			MaxDailyBuyWeight:     report.Config.MaxDailyBuyWeight,
			MaxDailyBuyAmount:     maxBudget,
			DisplayCandidateCount: report.Config.CandidateCount,
			AICandidateCount:      report.Config.AICandidateCount,
		},
		PortfolioSummary: FundDailyAIPortfolioSummary{
			OwnedFundCount:   len(portfolioActions),
			CandidateCount:   candidateTotal,
			CurrentWeight:    currentWeight,
			BuyableItemCount: buyableCount,
		},
		BudgetInput: FundDailyAIBudgetInput{
			ProgramBudget:           report.DailyBuyBudget,
			ProgramBudgetReasons:    report.DailyBuyBudgetReasons,
			MaxProgramAllowedBudget: maxBudget,
			Notes: []string{
				"The model may recommend daily_buy_budget = 0 when market signals are weak or current exposure is already high.",
				"ProgramBudget is a deterministic reference, not a required buy amount.",
			},
		},
		MarketContext:   report.MarketContext,
		Portfolio:       buildFundDailyAIFunds(portfolioActions, fundDailyBudgetSourcePortfolio),
		Candidates:      buildFundDailyAIFunds(candidateActions, fundDailyBudgetSourceCandidate),
		OutputContract:  fundDailyAIOutputContract(),
		ValidationRules: fundDailyAIValidationRules(maxBudget),
		Warnings:        report.Warnings,
	}
}

func buildFundDailyAIFunds(actions []FundDailyAction, source string) []FundDailyAIFund {
	funds := make([]FundDailyAIFund, 0, len(actions))
	for _, action := range actions {
		funds = append(funds, fundDailyAIFundFromAction(action, source))
	}
	return funds
}

func fundDailyAIFundFromAction(action FundDailyAction, source string) FundDailyAIFund {
	fund := FundDailyAIFund{
		Code:                      action.Code,
		Name:                      action.Name,
		Source:                    source,
		FundType:                  action.FundType,
		ProgramAction:             action.Action,
		ProgramActionLevel:        action.ActionLevel,
		CurrentAmount:             action.CurrentAmount,
		CurrentWeight:             action.CurrentWeight,
		SuggestedWeight:           action.SuggestedWeight,
		Score:                     action.Score,
		RiskLevel:                 action.RiskLevel,
		StrategyScore:             effectiveDailyStrategyScore(action),
		StrategyTheme:             action.StrategyTheme,
		TrendScore:                action.TrendScore,
		ThemeScore:                action.ThemeScore,
		CorrelationScore:          action.CorrelationScore,
		InstitutionalHoldingRatio: action.InstitutionalHoldingRatio,
		InternalHoldingRatio:      action.InternalHoldingRatio,
		Drawdown:                  action.Drawdown,
		Stddev:                    action.Stddev,
		Sharp:                     action.Sharp,
		NetAssetsScaleYi:          action.NetAssetsScaleYi,
		UnitNAV:                   action.UnitNAV,
		DailyProfitRatio:          action.DailyProfitRatio,
		SubscriptionStatus:        action.SubscriptionStatus,
		CanSubscribe:              action.CanSubscribe,
		RecentReturns: FundDailyAIReturns{
			Month1:   action.Month1Return,
			Month3:   action.Month3Return,
			Month6:   action.Month6Return,
			ThisYear: action.ThisYearReturn,
			Year1:    action.Year1Return,
			Year3:    action.Year3Return,
			Year5:    action.Year5Return,
		},
		RankRatios: FundDailyAIRankRatios{
			Month1:   action.Month1RankRatio,
			Month3:   action.Month3RankRatio,
			Month6:   action.Month6RankRatio,
			ThisYear: action.ThisYearRankRatio,
			Year1:    action.Year1RankRatio,
		},
		Manager: FundDailyAIManager{
			Name:         action.ManagerName,
			WorkingYears: action.ManagerWorkingYears,
			ManageYears:  action.ManagerManageYears,
			ManageReturn: action.ManagerManageReturn,
			AnnualReturn: action.ManagerAnnualReturn,
		},
		IndexName:     action.IndexName,
		TargetETFCode: action.TargetETFCode,
		TargetETFName: action.TargetETFName,
		Assets: FundDailyAIAssets{
			Stock: action.AssetStock,
			Bond:  action.AssetBond,
			Cash:  action.AssetCash,
		},
		TopStocks: action.TopStocks,
		Reasons:   action.Reasons,
		Warnings:  action.Warnings,
	}
	if action.SuggestedAmount > 0 {
		fund.SuggestedBuyCeiling = action.SuggestedAmount
	}
	if action.SuggestedAmount < 0 {
		fund.SuggestedSellAmount = -action.SuggestedAmount
	}
	if action.HasExpectedReturn {
		expected := action.ExpectedAnnualReturn
		fund.ExpectedAnnualReturn = &expected
	}
	if action.HasCorrelation {
		correlation := action.MaxCorrelation
		fund.MaxCorrelation = &correlation
	}
	return fund
}

func fundDailyAIOutputContract() FundDailyAIOutputContract {
	return FundDailyAIOutputContract{
		RequiredFormat: "strict JSON object only; no markdown and no prose outside JSON",
		RequiredFields: []string{
			"daily_buy_budget",
			"actions",
			"summary",
			"risk_notes",
		},
		ActionFields: []string{
			"code",
			"source",
			"action",
			"amount",
			"reason",
			"risk_note",
		},
		AllowedActions: []string{"buy", "hold", "watch", "trim", "sell", "skip"},
	}
}

func fundDailyAIValidationRules(maxBudget float64) []string {
	return []string{
		fmt.Sprintf("Total positive buy amount must be <= %.2f.", maxBudget),
		"Each positive buy amount must be <= that fund's suggested_buy_ceiling.",
		"Actions may only reference codes present in portfolio or candidates.",
		"Class C should be preferred over class A when both share the same fund base and both are available.",
		"daily_buy_budget may be 0; never force a buy when signals are weak.",
		"Sell/trim suggestions are advisory only and must not be converted into new buy budget.",
	}
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
