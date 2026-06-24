package core

import (
	"path/filepath"
	"testing"
	"time"
)

func TestProfitTakingStateCapsSameEpisodeAgainstBaseline(t *testing.T) {
	fund := fundDailyProfitTakingStateTestFund()
	signals := fundDailyProfitTakingStateTestSignals()
	start := time.Date(2026, 6, 24, 11, 0, 0, 0, time.Local)

	actions, state := buildFundDailyLocalProfitTakingActionsWithState([]FundDailyAIFund{fund}, signals, nil, true, emptyFundDailyProfitTakingState(), start)

	if len(actions) != 1 {
		t.Fatalf("expected first trim action, got %+v", actions)
	}
	if actions[0].Amount != 70 {
		t.Fatalf("expected first trim capped to 70.00 by 15%% baseline target, got %.2f; action=%+v", actions[0].Amount, actions[0])
	}
	record := state.Items[fund.Code]
	if record.BaselineAmount != 500 || record.TargetAmount != 75 || record.AdvisedAmount != 70 {
		t.Fatalf("unexpected first state record: %+v", record)
	}

	actions, state = buildFundDailyLocalProfitTakingActionsWithState([]FundDailyAIFund{fund}, signals, nil, true, state, start)

	if len(actions) != 1 {
		t.Fatalf("expected same-day refresh to keep visible trim action, got %+v", actions)
	}
	if actions[0].Amount != 70 {
		t.Fatalf("expected same-day refresh to reuse 70.00 recommendation, got %.2f", actions[0].Amount)
	}
	if state.Items[fund.Code].AdvisedAmount != 70 {
		t.Fatalf("expected same-day refresh not to double cumulative amount, got %+v", state.Items[fund.Code])
	}

	nextDay := start.Add(24 * time.Hour)
	actions, state = buildFundDailyLocalProfitTakingActionsWithState([]FundDailyAIFund{fund}, signals, nil, true, state, nextDay)

	if len(actions) != 0 {
		t.Fatalf("expected no next-day trim after baseline target is effectively reached, got %+v", actions)
	}
}

func TestProfitTakingStateAllowsIncrementWhenRiskTierUpgrades(t *testing.T) {
	fund := fundDailyProfitTakingStateTestFund()
	signals := fundDailyProfitTakingStateTestSignals()
	start := time.Date(2026, 6, 24, 11, 0, 0, 0, time.Local)

	_, state := buildFundDailyLocalProfitTakingActionsWithState([]FundDailyAIFund{fund}, signals, nil, true, emptyFundDailyProfitTakingState(), start)

	fund.ProfitRatio = 30
	fund.ProfitAmount = 160
	fund.Drawdown = 42
	fund.Stddev = 42
	fund.RecentReturns = FundDailyAIReturns{Month1: 31, Month3: 72, Month6: 94}
	actions, state := buildFundDailyLocalProfitTakingActionsWithState([]FundDailyAIFund{fund}, signals, nil, true, state, start.Add(24*time.Hour))

	if len(actions) != 1 {
		t.Fatalf("expected risk upgrade to add trim action, got %+v", actions)
	}
	if actions[0].Amount != 30 {
		t.Fatalf("expected risk upgrade to add only remaining 30.00 toward 20%% baseline target, got %.2f; action=%+v", actions[0].Amount, actions[0])
	}
	record := state.Items[fund.Code]
	if record.TargetRatio != 0.20 || record.TargetAmount != 100 || record.AdvisedAmount != 100 {
		t.Fatalf("unexpected upgraded state record: %+v", record)
	}
}

func TestProfitTakingStateUsesExplicitTrimAmountAsBaselineTarget(t *testing.T) {
	fund := fundDailyProfitTakingStateTestFund()
	fund.SuggestedSellAmount = 30
	signals := fundDailyProfitTakingStateTestSignals()
	start := time.Date(2026, 6, 24, 11, 0, 0, 0, time.Local)

	actions, state := buildFundDailyLocalProfitTakingActionsWithState([]FundDailyAIFund{fund}, signals, nil, true, emptyFundDailyProfitTakingState(), start)

	if len(actions) != 1 {
		t.Fatalf("expected explicit trim action, got %+v", actions)
	}
	if actions[0].Amount != 30 {
		t.Fatalf("expected explicit trim amount 30.00, got %.2f", actions[0].Amount)
	}
	record := state.Items[fund.Code]
	if record.TargetAmount != 30 || record.AdvisedAmount != 30 {
		t.Fatalf("expected explicit amount to define this episode target, got %+v", record)
	}

	actions, _ = buildFundDailyLocalProfitTakingActionsWithState([]FundDailyAIFund{fund}, signals, nil, true, state, start.Add(24*time.Hour))
	if len(actions) != 0 {
		t.Fatalf("expected no next-day top-up beyond explicit baseline target, got %+v", actions)
	}
}

func TestFundDailyProfitTakingStateStoreRoundTrips(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "profit_taking_state.json")
	store := NewFundDailyProfitTakingStateStore(filename)
	state := emptyFundDailyProfitTakingState()
	state.Items["001407"] = FundDailyProfitTakingRecord{
		Code:           "001407",
		Name:           "trimmed fund",
		UpsideRoom:     string(fundDailyUpsideRoomLimited),
		BaselineDate:   "2026-06-24",
		BaselineAmount: 500,
		TargetRatio:    0.15,
		TargetAmount:   75,
		AdvisedAmount:  70,
	}

	if err := store.Save(state); err != nil {
		t.Fatalf("save state: %v", err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}

	if loaded.Items["001407"].AdvisedAmount != 70 {
		t.Fatalf("expected round-tripped advised amount 70, got %+v", loaded.Items["001407"])
	}
}

func fundDailyProfitTakingStateTestFund() FundDailyAIFund {
	expectedReturn := 35.0
	return FundDailyAIFund{
		Code:                 "001407",
		Name:                 "limited upside holding C",
		FundType:             "equity",
		CurrentAmount:        500,
		HoldingShares:        250,
		CurrentWeight:        22,
		ProfitRatio:          12,
		ProfitAmount:         40,
		Score:                100,
		StrategyScore:        100,
		ExpectedAnnualReturn: &expectedReturn,
		Drawdown:             22,
		Stddev:               33,
		Sharp:                1.7,
		RecentReturns:        FundDailyAIReturns{Month1: 22, Month3: 78, Month6: 92},
		RankRatios:           FundDailyAIRankRatios{Month1: 10, Month3: 10, Month6: 12, ThisYear: 12, Year1: 8},
		TopStocks:            []FundDailyTopHolding{{Name: "SharedAI", HoldRatio: 9}},
	}
}

func fundDailyProfitTakingStateTestSignals() fundDailyLocalSignals {
	return fundDailyLocalSignals{
		PortfolioTopStocks: map[string]float64{"SharedAI": 18},
		TechConcentrated:   true,
	}
}
