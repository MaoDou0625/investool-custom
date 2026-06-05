package core

import (
	"context"
	"testing"

	"github.com/axiaoxin-com/investool/models"
)

func TestBuildFundDailyAdviceReportRespectsCashRoom(t *testing.T) {
	report := BuildFundDailyAdviceReport(context.Background(), []FundPortfolioAdvice{
		{
			Item:                    models.FundPortfolioItem{Code: "000001", Status: models.FundPortfolioStatusOwned},
			Score:                   82,
			RiskLevel:               "中等",
			HasPosition:             true,
			CurrentAmount:           1000,
			CurrentWeight:           20,
			RecommendedWeight:       30,
			HasExpectedAnnualReturn: true,
			ExpectedAnnualReturn:    8,
		},
	}, nil, FundDailyAdviceConfig{
		TargetAnnualReturn: 5,
		MaxTotalAmount:     1200,
		CashBufferWeight:   10,
	})

	if report.CashRoom != 80 {
		t.Fatalf("expected cash room 80, got %.2f", report.CashRoom)
	}
	if len(report.PortfolioActions) != 1 {
		t.Fatalf("expected 1 portfolio action, got %d", len(report.PortfolioActions))
	}
	if report.PortfolioActions[0].SuggestedAmount > 80 {
		t.Fatalf("suggested amount exceeds cash room: %.2f", report.PortfolioActions[0].SuggestedAmount)
	}
}

func TestBuildFundDailyAdviceReportFlagsWeakHolding(t *testing.T) {
	report := BuildFundDailyAdviceReport(context.Background(), []FundPortfolioAdvice{
		{
			Item:          models.FundPortfolioItem{Code: "000001", Status: models.FundPortfolioStatusOwned},
			Score:         40,
			RiskLevel:     "偏高",
			HasPosition:   true,
			CurrentAmount: 1000,
		},
	}, nil, DefaultFundDailyAdviceConfig())

	if len(report.PortfolioActions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(report.PortfolioActions))
	}
	if report.PortfolioActions[0].ActionLevel != "sell" {
		t.Fatalf("expected sell action, got %s", report.PortfolioActions[0].ActionLevel)
	}
	if report.PortfolioActions[0].SuggestedAmount >= 0 {
		t.Fatalf("expected negative sell amount, got %.2f", report.PortfolioActions[0].SuggestedAmount)
	}
}

func TestBuildFundDailyAdviceReportCapsCandidateDailyBuyBudget(t *testing.T) {
	report := BuildFundDailyAdviceReport(context.Background(), nil, models.FundList{
		buildRecommendationFund("100001", "candidate one", 12),
		buildRecommendationFund("100002", "candidate two", 12),
		buildRecommendationFund("100003", "candidate three", 12),
	}, FundDailyAdviceConfig{
		TargetAnnualReturn:    5,
		MaxTotalAmount:        5000,
		CashBufferWeight:      10,
		MaxSingleFundWeight:   35,
		TacticalWeight:        20,
		MaxDailyBuyWeight:     10,
		CandidateCount:        3,
		MinCandidateScore:     1,
		MinCoreCandidateScore: 75,
	})

	if report.DailyBuyBudget <= 0 || report.DailyBuyBudget > 500 {
		t.Fatalf("expected adaptive daily buy budget in (0, 500], got %.2f", report.DailyBuyBudget)
	}
	if len(report.CandidateActions) != 3 {
		t.Fatalf("expected 3 candidate actions, got %d", len(report.CandidateActions))
	}
	if len(report.DailyBuyBudgetReasons) == 0 {
		t.Fatalf("expected adaptive budget reasons")
	}
	if total := totalPositiveDailyCandidateAmount(report.CandidateActions); total > report.DailyBuyBudget {
		t.Fatalf("candidate buy total %.2f exceeds daily buy budget %.2f", total, report.DailyBuyBudget)
	}
	if report.CandidateActions[1].SuggestedAmount != 0 || report.CandidateActions[2].SuggestedAmount != 0 {
		t.Fatalf("expected later candidates to be observation-only after adaptive budget cap, got %.2f and %.2f", report.CandidateActions[1].SuggestedAmount, report.CandidateActions[2].SuggestedAmount)
	}
}
