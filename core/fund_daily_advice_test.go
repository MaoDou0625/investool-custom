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
