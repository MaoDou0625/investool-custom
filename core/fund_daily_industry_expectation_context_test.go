package core

import (
	"strings"
	"testing"
	"time"
)

func TestBuildFundDailyIndustryExpectationContextFlagsNotFullyPricedRisk(t *testing.T) {
	portfolioAction := FundDailyAction{
		Code:          "300001",
		Name:          "AI growth holding C",
		FundType:      "index equity",
		CurrentAmount: 1500,
		CurrentWeight: 80,
		Month1Return:  18,
		Month3Return:  36,
		Month6Return:  58,
	}
	theme := fundDailyIndustryThemesForFund(fundDailyAIFundFromAction(portfolioAction, fundDailyBudgetSourcePortfolio))[0]

	report := FundDailyAdviceReport{
		GeneratedAt: time.Date(2026, 6, 20, 11, 0, 0, 0, time.Local),
		Config:      DefaultFundDailyAdviceConfig(),
		DecisionPortfolioActions: []FundDailyAction{
			portfolioAction,
		},
		MarketContext: FundDailyMarketContext{
			Status: "ready",
			ThemeTilts: []FundDailyMarketThemeTilt{
				{Theme: theme, Score: 12, Reason: "technology industry is still rising"},
			},
		},
		NewsContext: FundDailyNewsContext{
			Status: "ready",
			ThemeTilts: []FundDailyMarketThemeTilt{
				{Theme: theme, Score: -10, Reason: "rate pressure hurts growth valuation"},
			},
		},
	}

	contextData := BuildFundDailyIndustryExpectationContext(report)

	if contextData.Status != "ready" {
		t.Fatalf("expected ready context, got %s warnings=%v", contextData.Status, contextData.Warnings)
	}
	if len(contextData.Themes) == 0 {
		t.Fatalf("expected at least one theme")
	}
	top := contextData.Themes[0]
	if top.PricingState != FundDailyIndustryPricingNotFullyPriced {
		t.Fatalf("expected not fully priced state, got %+v", top)
	}
	if top.RiskPressure != "high" {
		t.Fatalf("expected high risk pressure, got %+v", top)
	}
	if contextData.BudgetMultiplier >= 1 {
		t.Fatalf("expected budget multiplier below 1, got %.2f", contextData.BudgetMultiplier)
	}
	if !strings.Contains(contextData.Summary, top.Theme) {
		t.Fatalf("expected summary to mention top theme, got %q", contextData.Summary)
	}
}

func TestFundDailyIndustryExpectationTiltScorePenalizesSameThemeCandidate(t *testing.T) {
	fund := FundDailyAIFund{Name: "AI candidate C", FundType: "index equity"}
	theme := fundDailyIndustryThemesForFund(fund)[0]
	contextData := FundDailyIndustryExpectationContext{
		Status: "ready",
		Themes: []FundDailyIndustryExpectationTheme{
			{Theme: theme, PricingState: FundDailyIndustryPricingNotFullyPriced, RiskPressure: "high"},
		},
	}

	score := fundDailyIndustryExpectationTiltScoreForFund(fund, contextData)

	if score >= 0 {
		t.Fatalf("expected same-theme candidate penalty, got %.2f", score)
	}
}

func TestBuildFundDailyIndustryExpectationContextSplitsMultiThemeExposure(t *testing.T) {
	report := FundDailyAdviceReport{
		GeneratedAt:      time.Date(2026, 6, 20, 11, 0, 0, 0, time.Local),
		InvestableAmount: 5000,
		Config:           DefaultFundDailyAdviceConfig(),
		MarketContext:    FundDailyMarketContext{Status: "ready"},
		NewsContext:      FundDailyNewsContext{Status: "ready"},
		DecisionPortfolioActions: []FundDailyAction{
			{
				Code:          "300001",
				Name:          "AI growth holding C",
				FundType:      "index equity",
				CurrentAmount: 1000,
				Month1Return:  18,
				Month3Return:  36,
				Month6Return:  58,
			},
		},
	}

	contextData := BuildFundDailyIndustryExpectationContext(report)

	totalExposure := 0.0
	for _, theme := range contextData.Themes {
		totalExposure += theme.ExposureWeight
	}
	if totalExposure > 20.1 {
		t.Fatalf("expected one fund exposure to be split across themes, got total %.2f themes=%+v", totalExposure, contextData.Themes)
	}
	if contextData.BudgetMultiplier < 0.88 {
		t.Fatalf("expected duplicate themes not to compound budget below 0.88, got %.2f", contextData.BudgetMultiplier)
	}
}

func TestBuildFundDailyIndustryExpectationContextUnavailableWithOnlyMissingEvidence(t *testing.T) {
	report := FundDailyAdviceReport{
		GeneratedAt: time.Date(2026, 6, 20, 11, 0, 0, 0, time.Local),
		Config:      DefaultFundDailyAdviceConfig(),
		DecisionCandidateActions: []FundDailyAction{
			{
				Code:               "100001",
				Name:               "Balanced candidate C",
				FundType:           "mixed",
				StrategyTheme:      "balanced",
				SuggestedAmount:    100,
				CanSubscribe:       true,
				SubscriptionStatus: "开放申购",
			},
		},
	}

	contextData := BuildFundDailyIndustryExpectationContext(report)

	if contextData.Status == "ready" {
		t.Fatalf("expected unavailable status when only missing/weak evidence exists, got %+v", contextData)
	}
	if contextData.BudgetMultiplier != 1 {
		t.Fatalf("expected no budget adjustment with unavailable context, got %.2f", contextData.BudgetMultiplier)
	}
}
