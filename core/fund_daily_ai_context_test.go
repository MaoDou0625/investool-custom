package core

import (
	"context"
	"testing"

	"github.com/axiaoxin-com/investool/models"
)

func TestBuildFundDailyAIContextUsesLargerCandidatePoolThanPageDisplay(t *testing.T) {
	candidates := models.FundList{}
	for idx := 0; idx < 10; idx++ {
		candidates = append(candidates, buildRecommendationFund("90"+string(rune('0'+idx))+"001", "candidate", 12))
	}

	report := BuildFundDailyAdviceReport(context.Background(), nil, candidates, FundDailyAdviceConfig{
		TargetAnnualReturn:    5,
		MaxTotalAmount:        5000,
		CashBufferWeight:      10,
		MaxSingleFundWeight:   35,
		TacticalWeight:        20,
		MaxDailyBuyWeight:     10,
		CandidateCount:        3,
		AICandidateCount:      6,
		MinCandidateScore:     1,
		MinCoreCandidateScore: 75,
	})
	contextData := BuildFundDailyAIContext(report)

	if len(report.CandidateActions) != 3 {
		t.Fatalf("expected page display to keep 3 candidates, got %d", len(report.CandidateActions))
	}
	if len(contextData.Candidates) != 6 {
		t.Fatalf("expected AI context to keep 6 candidates, got %d", len(contextData.Candidates))
	}
	if contextData.Constraints.AICandidateCount != 6 {
		t.Fatalf("expected AI candidate count 6, got %d", contextData.Constraints.AICandidateCount)
	}
	if contextData.Candidates[0].Source != fundDailyBudgetSourceCandidate {
		t.Fatalf("expected normalized candidate source, got %s", contextData.Candidates[0].Source)
	}
	if len(contextData.ValidationRules) == 0 {
		t.Fatalf("expected validation rules in AI context")
	}
}

func TestBuildFundDailyAIContextIncludesHolderStructureRatios(t *testing.T) {
	report := FundDailyAdviceReport{
		Config: DefaultFundDailyAdviceConfig(),
		DecisionCandidateActions: []FundDailyAction{
			{
				Code:                      "000001",
				Name:                      "holder ratio fund",
				InstitutionalHoldingRatio: 45.6,
				InternalHoldingRatio:      1.2,
			},
		},
	}

	contextData := BuildFundDailyAIContext(report)

	if len(contextData.Candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(contextData.Candidates))
	}
	if contextData.Candidates[0].InstitutionalHoldingRatio != 45.6 {
		t.Fatalf("expected institutional holder ratio 45.6, got %.2f", contextData.Candidates[0].InstitutionalHoldingRatio)
	}
	if contextData.Candidates[0].InternalHoldingRatio != 1.2 {
		t.Fatalf("expected internal holder ratio 1.2, got %.2f", contextData.Candidates[0].InternalHoldingRatio)
	}
}

func TestBuildFundDailyAIContextIncludesSubscriptionStatus(t *testing.T) {
	report := FundDailyAdviceReport{
		Config: DefaultFundDailyAdviceConfig(),
		DecisionCandidateActions: []FundDailyAction{
			{
				Code:               "017093",
				Name:               "subscription gated fund",
				SubscriptionStatus: "暂停申购",
				CanSubscribe:       false,
			},
		},
	}

	contextData := BuildFundDailyAIContext(report)

	if len(contextData.Candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(contextData.Candidates))
	}
	if contextData.Candidates[0].SubscriptionStatus != "暂停申购" {
		t.Fatalf("expected subscription status to be carried into context, got %q", contextData.Candidates[0].SubscriptionStatus)
	}
	if contextData.Candidates[0].CanSubscribe {
		t.Fatalf("expected stopped fund to be marked non-subscribable")
	}
}

func TestBuildFundDailyAIContextIncludesMarketContext(t *testing.T) {
	report := FundDailyAdviceReport{
		Config: DefaultFundDailyAdviceConfig(),
		MarketContext: FundDailyMarketContext{
			Status:           "ready",
			Summary:          "market summary",
			RiskLevel:        "cautious",
			BudgetMultiplier: 0.65,
		},
	}

	contextData := BuildFundDailyAIContext(report)

	if contextData.MarketContext.Status != "ready" {
		t.Fatalf("expected market context to be carried into AI context")
	}
	if contextData.MarketContext.BudgetMultiplier != 0.65 {
		t.Fatalf("expected budget multiplier 0.65, got %.2f", contextData.MarketContext.BudgetMultiplier)
	}
}

func TestBuildFundDailyAIContextIncludesNewsContext(t *testing.T) {
	report := FundDailyAdviceReport{
		Config: DefaultFundDailyAdviceConfig(),
		NewsContext: FundDailyNewsContext{
			Status:           "ready",
			Summary:          "news summary",
			RiskDelta:        8,
			BudgetMultiplier: 0.88,
			Articles: []FundDailyNewsArticle{
				{Source: "Example", Title: "Fed keeps rates higher", RiskDelta: 6, Confidence: 0.8},
			},
		},
	}

	contextData := BuildFundDailyAIContext(report)

	if contextData.NewsContext.Status != "ready" {
		t.Fatalf("expected news context to be carried into AI context")
	}
	if contextData.NewsContext.BudgetMultiplier != 0.88 {
		t.Fatalf("expected news budget multiplier 0.88, got %.2f", contextData.NewsContext.BudgetMultiplier)
	}
	if len(contextData.NewsContext.Articles) != 1 {
		t.Fatalf("expected news article evidence in AI context")
	}
}
