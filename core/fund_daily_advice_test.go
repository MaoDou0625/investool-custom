package core

import (
	"context"
	"testing"
	"time"

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
		Now:                time.Date(2026, 6, 15, 11, 0, 0, 0, time.Local),
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
		Now:                   time.Date(2026, 6, 15, 11, 0, 0, 0, time.Local),
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
	if total := totalPositiveDailyBuyAmount(report.CandidateActions); total > report.DailyBuyBudget {
		t.Fatalf("candidate buy total %.2f exceeds daily buy budget %.2f", total, report.DailyBuyBudget)
	}
	if report.CandidateActions[2].SuggestedAmount != 0 {
		t.Fatalf("expected last candidate to be observation-only after adaptive budget cap, got %.2f", report.CandidateActions[2].SuggestedAmount)
	}
}

func TestBuildFundDailyAdviceReportCapsPortfolioAndCandidateBuysTogether(t *testing.T) {
	report := BuildFundDailyAdviceReport(context.Background(), []FundPortfolioAdvice{
		{
			Item:                    models.FundPortfolioItem{Code: "000001", Status: models.FundPortfolioStatusOwned},
			Score:                   86,
			RiskLevel:               "中等",
			HasPosition:             true,
			CurrentAmount:           100,
			CurrentWeight:           2,
			RecommendedWeight:       35,
			HasExpectedAnnualReturn: true,
			ExpectedAnnualReturn:    8,
		},
	}, models.FundList{
		buildRecommendationFund("100001", "candidate one", 12),
	}, FundDailyAdviceConfig{
		TargetAnnualReturn:    5,
		MaxTotalAmount:        5000,
		CashBufferWeight:      10,
		MaxSingleFundWeight:   35,
		TacticalWeight:        20,
		MaxDailyBuyWeight:     2,
		CandidateCount:        1,
		MinCandidateScore:     1,
		MinCoreCandidateScore: 75,
		Now:                   time.Date(2026, 6, 15, 11, 0, 0, 0, time.Local),
	})

	if report.DailyBuyBudget <= 0 || report.DailyBuyBudget > 100 {
		t.Fatalf("expected adaptive daily buy budget in (0, 100], got %.2f", report.DailyBuyBudget)
	}
	total := totalPositiveDailyBuyAmount(report.PortfolioActions, report.CandidateActions)
	if total > report.DailyBuyBudget {
		t.Fatalf("combined buy total %.2f exceeds daily buy budget %.2f", total, report.DailyBuyBudget)
	}
	if report.PortfolioActions[0].SuggestedAmount > report.DailyBuyBudget {
		t.Fatalf("portfolio buy %.2f exceeds daily buy budget %.2f", report.PortfolioActions[0].SuggestedAmount, report.DailyBuyBudget)
	}
}

func TestBuildFundDailyAdviceReportDisablesBuysOnNonWorkday(t *testing.T) {
	report := BuildFundDailyAdviceReport(context.Background(), []FundPortfolioAdvice{
		{
			Item:                    models.FundPortfolioItem{Code: "000001", Status: models.FundPortfolioStatusOwned},
			Score:                   86,
			RiskLevel:               "中等",
			HasPosition:             true,
			CurrentAmount:           100,
			CurrentWeight:           2,
			RecommendedWeight:       35,
			HasExpectedAnnualReturn: true,
			ExpectedAnnualReturn:    8,
		},
	}, models.FundList{
		buildRecommendationFund("100001", "candidate one", 12),
	}, FundDailyAdviceConfig{
		TargetAnnualReturn:    5,
		MaxTotalAmount:        5000,
		CashBufferWeight:      10,
		MaxSingleFundWeight:   35,
		TacticalWeight:        20,
		MaxDailyBuyWeight:     10,
		CandidateCount:        1,
		MinCandidateScore:     1,
		MinCoreCandidateScore: 75,
		Now:                   time.Date(2026, 6, 14, 11, 0, 0, 0, time.Local),
	})

	if !report.WorkdayGuard.BlocksBuy() {
		t.Fatalf("expected non-workday guard to block buys")
	}
	if report.DailyBuyBudget != 0 {
		t.Fatalf("expected zero daily buy budget on non-workday, got %.2f", report.DailyBuyBudget)
	}
	if total := totalPositiveDailyBuyAmount(report.DecisionPortfolioActions, report.DecisionCandidateActions, report.PortfolioActions, report.CandidateActions); total != 0 {
		t.Fatalf("expected no positive buy actions on non-workday, got %.2f", total)
	}
	contextData := BuildFundDailyAIContext(report)
	if contextData.Constraints.MaxDailyBuyAmount != 0 {
		t.Fatalf("expected AI max daily buy amount to be zero on non-workday, got %.2f", contextData.Constraints.MaxDailyBuyAmount)
	}
	if contextData.PortfolioSummary.BuyableItemCount != 0 {
		t.Fatalf("expected zero buyable items on non-workday, got %d", contextData.PortfolioSummary.BuyableItemCount)
	}
}

func TestBuildFundDailyAdviceReportAllowsBuysOnWorkday(t *testing.T) {
	report := BuildFundDailyAdviceReport(context.Background(), nil, models.FundList{
		buildRecommendationFund("100001", "candidate one", 12),
	}, FundDailyAdviceConfig{
		TargetAnnualReturn:    5,
		MaxTotalAmount:        5000,
		CashBufferWeight:      10,
		MaxSingleFundWeight:   35,
		TacticalWeight:        20,
		MaxDailyBuyWeight:     10,
		CandidateCount:        1,
		MinCandidateScore:     1,
		MinCoreCandidateScore: 75,
		Now:                   time.Date(2026, 6, 15, 11, 0, 0, 0, time.Local),
	})

	if report.WorkdayGuard.BlocksBuy() {
		t.Fatalf("expected workday guard to allow buys")
	}
	if report.DailyBuyBudget <= 0 {
		t.Fatalf("expected positive daily buy budget on workday, got %.2f", report.DailyBuyBudget)
	}
	if total := totalPositiveDailyBuyAmount(report.CandidateActions); total <= 0 {
		t.Fatalf("expected positive candidate buy amount on workday")
	}
}

func TestBuildFundDailyAdviceReportDisablesBuysOnConfiguredWeekdayHoliday(t *testing.T) {
	report := BuildFundDailyAdviceReport(context.Background(), nil, models.FundList{
		buildRecommendationFund("100001", "candidate one", 12),
	}, FundDailyAdviceConfig{
		TargetAnnualReturn:    5,
		MaxTotalAmount:        5000,
		CashBufferWeight:      10,
		MaxSingleFundWeight:   35,
		TacticalWeight:        20,
		MaxDailyBuyWeight:     10,
		CandidateCount:        1,
		MinCandidateScore:     1,
		MinCoreCandidateScore: 75,
		Now:                   time.Date(2026, 5, 4, 11, 0, 0, 0, time.Local),
	})

	if !report.WorkdayGuard.BlocksBuy() {
		t.Fatalf("expected configured weekday holiday to block buys")
	}
	if report.WorkdayGuard.CalendarOverride != "configured_non_workday" {
		t.Fatalf("expected configured non-workday override, got %q", report.WorkdayGuard.CalendarOverride)
	}
	if report.DailyBuyBudget != 0 {
		t.Fatalf("expected zero budget on configured holiday, got %.2f", report.DailyBuyBudget)
	}
}

func TestBuildFundDailyAdviceReportAllowsBuysOnConfiguredWeekendWorkday(t *testing.T) {
	report := BuildFundDailyAdviceReport(context.Background(), nil, models.FundList{
		buildRecommendationFund("100001", "candidate one", 12),
	}, FundDailyAdviceConfig{
		TargetAnnualReturn:    5,
		MaxTotalAmount:        5000,
		CashBufferWeight:      10,
		MaxSingleFundWeight:   35,
		TacticalWeight:        20,
		MaxDailyBuyWeight:     10,
		CandidateCount:        1,
		MinCandidateScore:     1,
		MinCoreCandidateScore: 75,
		Now:                   time.Date(2026, 5, 9, 11, 0, 0, 0, time.Local),
	})

	if report.WorkdayGuard.BlocksBuy() {
		t.Fatalf("expected configured weekend workday to allow buys")
	}
	if report.WorkdayGuard.CalendarOverride != "configured_workday" {
		t.Fatalf("expected configured workday override, got %q", report.WorkdayGuard.CalendarOverride)
	}
	if report.DailyBuyBudget <= 0 {
		t.Fatalf("expected positive budget on configured workday, got %.2f", report.DailyBuyBudget)
	}
}

func TestBuildFundDailyAdviceReportRemovesBuyLabelsOnNonWorkdayWithNoCashRoom(t *testing.T) {
	report := BuildFundDailyAdviceReport(context.Background(), []FundPortfolioAdvice{
		{
			Item:          models.FundPortfolioItem{Code: "000001", Status: models.FundPortfolioStatusOwned},
			Score:         80,
			RiskLevel:     "中等",
			HasPosition:   true,
			CurrentAmount: 4500,
			CurrentWeight: 90,
		},
	}, models.FundList{
		buildRecommendationFund("100001", "candidate one", 12),
	}, FundDailyAdviceConfig{
		TargetAnnualReturn:    5,
		MaxTotalAmount:        5000,
		CashBufferWeight:      10,
		MaxSingleFundWeight:   35,
		TacticalWeight:        20,
		MaxDailyBuyWeight:     10,
		CandidateCount:        1,
		MinCandidateScore:     1,
		MinCoreCandidateScore: 75,
		Now:                   time.Date(2026, 6, 14, 11, 0, 0, 0, time.Local),
	})

	for _, action := range append(report.DecisionPortfolioActions, report.DecisionCandidateActions...) {
		if action.ActionLevel == "buy" || action.SuggestedAmount > 0 {
			t.Fatalf("expected non-workday guard to remove buy labels and amounts, got %+v", action)
		}
	}
	contextData := BuildFundDailyAIContext(report)
	for _, fund := range append(contextData.Portfolio, contextData.Candidates...) {
		if fund.ProgramActionLevel == "buy" || fund.SuggestedBuyCeiling > 0 {
			t.Fatalf("expected AI context to avoid buy labels and ceilings on non-workday, got %+v", fund)
		}
	}
}
