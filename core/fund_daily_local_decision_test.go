package core

import "testing"

func TestBuildFundDailyLocalDecisionShowsConversationStyleAdvice(t *testing.T) {
	contextData := FundDailyAIContext{
		Constraints: FundDailyAIConstraints{
			MaxDailyBuyAmount: 500,
			CashRoom:          4266.5,
		},
		BudgetInput: FundDailyAIBudgetInput{
			ProgramBudget: 460,
		},
		Portfolio: []FundDailyAIFund{
			{
				Code:      "001407",
				Name:      "景顺长城稳健回报混合C",
				FundType:  "混合型-灵活",
				Score:     96,
				RiskLevel: "中等",
				Drawdown:  22.5,
				Stddev:    31.6,
				TopStocks: []FundDailyTopHolding{{Name: "中际旭创", HoldRatio: 9.39}, {Name: "新易盛", HoldRatio: 9.33}},
				RecentReturns: FundDailyAIReturns{
					Month1: 20.47,
					Month3: 53.82,
					Month6: 74.5,
				},
			},
			{
				Code:      "009239",
				Name:      "融通人工智能指数(LOF)C",
				FundType:  "指数型-股票",
				Score:     89,
				RiskLevel: "偏高",
				Drawdown:  33.3,
				Stddev:    31.5,
				TopStocks: []FundDailyTopHolding{{Name: "新易盛", HoldRatio: 10.86}, {Name: "中际旭创", HoldRatio: 9.91}},
				RecentReturns: FundDailyAIReturns{
					Month1: 13.53,
					Month3: 23.41,
					Month6: 38.6,
				},
			},
			{
				Code:      "012351",
				Name:      "万家元贞量化选股股票C",
				FundType:  "股票型",
				Score:     93,
				RiskLevel: "中低",
				Drawdown:  14.9,
				Stddev:    14.4,
				RecentReturns: FundDailyAIReturns{
					Month1: 7.31,
					Month3: 13.08,
					Month6: 38.33,
				},
			},
		},
		Candidates: []FundDailyAIFund{
			{
				Code:                "017091",
				Name:                "景顺长城纳斯达克科技ETF联接(QDII)A人民币",
				FundType:            "指数型-海外股票",
				SuggestedBuyCeiling: 300,
				Score:               93,
				StrategyScore:       100,
				StrategyTheme:       "海外科技",
				NetAssetsScaleYi:    8,
				RecentReturns:       FundDailyAIReturns{Month1: 12.5, Month3: 25.5, Month6: 19.5},
			},
			{
				Code:                "017093",
				Name:                "景顺长城纳斯达克科技ETF联接(QDII)C人民币",
				FundType:            "指数型-海外股票",
				ProgramActionLevel:  "watch",
				SuggestedBuyCeiling: 0,
				Score:               93,
				StrategyScore:       100,
				StrategyTheme:       "海外科技",
				NetAssetsScaleYi:    8,
				RecentReturns:       FundDailyAIReturns{Month1: 12.45, Month3: 25.43, Month6: 19.24},
			},
			{
				Code:                "012350",
				Name:                "万家元贞量化选股股票A",
				FundType:            "股票型",
				SuggestedBuyCeiling: 300,
				Score:               93,
				StrategyScore:       100,
				StrategyTheme:       "量化多因子",
				NetAssetsScaleYi:    11,
				RecentReturns:       FundDailyAIReturns{Month1: 7.31, Month3: 13.08, Month6: 38.33},
				TopStocks:           []FundDailyTopHolding{{Name: "新易盛", HoldRatio: 1.28}, {Name: "中际旭创", HoldRatio: 1.17}},
			},
			{
				Code:                "010637",
				Name:                "财通安盈混合C",
				FundType:            "混合型",
				SuggestedBuyCeiling: 300,
				Score:               90,
				StrategyScore:       96,
				StrategyTheme:       "稳健分散",
				NetAssetsScaleYi:    2,
				RecentReturns:       FundDailyAIReturns{Month1: 7.0, Month3: 22.8, Month6: 30.0},
			},
			{
				Code:                "014661",
				Name:                "天弘黄金ETF联接A",
				FundType:            "指数型-商品",
				SuggestedBuyCeiling: 0,
				Score:               70,
				StrategyScore:       46,
				StrategyTheme:       "黄金/贵金属",
				RecentReturns:       FundDailyAIReturns{Month1: -5.46, Month3: -14.27, Month6: 4.12},
			},
		},
	}

	decision := BuildFundDailyLocalDecision(contextData)

	if !decision.Validated {
		t.Fatalf("expected decision to be validated")
	}
	if decision.DailyBuyBudget != 120 {
		t.Fatalf("expected 120 local buy budget, got %.2f", decision.DailyBuyBudget)
	}
	assertDailyDecisionAction(t, decision, "001407", "hold", 0)
	assertDailyDecisionAction(t, decision, "009239", "hold", 0)
	assertDailyDecisionAction(t, decision, "012351", "hold", 0)
	assertDailyDecisionAction(t, decision, "017091", "buy", 60)
	assertDailyDecisionAction(t, decision, "010637", "buy", 60)
	assertDailyDecisionAction(t, decision, "014661", "watch", 0)
	if hasDailyDecisionBuy(decision, "012350") {
		t.Fatalf("expected class A candidate to be suppressed when same fund class C is already held")
	}
	if hasDailyDecisionBuy(decision, "017093") {
		t.Fatalf("expected non-subscribable candidate to be unavailable for local buy decisions")
	}
}

func assertDailyDecisionAction(t *testing.T, decision FundDailyAIDecision, code string, action string, amount float64) {
	t.Helper()
	for _, item := range decision.Actions {
		if item.Code != code {
			continue
		}
		if item.Action != action || item.Amount != amount {
			t.Fatalf("expected %s %s %.2f, got %s %.2f", code, action, amount, item.Action, item.Amount)
		}
		return
	}
	t.Fatalf("missing action for %s", code)
}

func hasDailyDecisionBuy(decision FundDailyAIDecision, code string) bool {
	for _, item := range decision.Actions {
		if item.Code == code && item.Action == "buy" && item.Amount > 0 {
			return true
		}
	}
	return false
}
