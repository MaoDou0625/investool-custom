package core

import "testing"

func TestAssessFundDailyUpsideRoomDefersConstructiveAutomaticTrim(t *testing.T) {
	expectedReturn := 16.0
	fund := FundDailyAIFund{
		Code:                 "400001",
		Name:                 "AI constructive holding C",
		FundType:             "equity",
		CurrentAmount:        500,
		CurrentWeight:        10,
		ProfitRatio:          10,
		ProfitAmount:         50,
		Score:                90,
		StrategyScore:        90,
		ExpectedAnnualReturn: &expectedReturn,
		Drawdown:             14,
		Stddev:               14,
		Sharp:                1.1,
		RecentReturns:        FundDailyAIReturns{Month1: 13, Month3: 28, Month6: 38},
		RankRatios:           FundDailyAIRankRatios{Month1: 20, Month3: 22, Month6: 24, ThisYear: 21, Year1: 23},
		TopStocks:            []FundDailyTopHolding{{Name: "SharedGrowth", HoldRatio: 6}},
	}
	signals := fundDailyLocalSignals{
		PortfolioTopStocks: map[string]float64{"SharedGrowth": 6},
		TechConcentrated:   true,
	}

	assessment := assessFundDailyUpsideRoom(fund, signals)

	if assessment.room != fundDailyUpsideRoomConstructive {
		t.Fatalf("expected constructive upside room, got %s; assessment=%+v", assessment.room, assessment)
	}
	if assessment.allowAutomaticTrim {
		t.Fatalf("expected constructive upside to defer automatic trim, got %+v", assessment)
	}
	if assessment.trimRatio != 0 {
		t.Fatalf("expected zero trim ratio, got %.2f; assessment=%+v", assessment.trimRatio, assessment)
	}
}

func TestAssessFundDailyUpsideRoomSetsStretchedTrimRatio(t *testing.T) {
	fund := FundDailyAIFund{
		Code:          "400002",
		Name:          "stretched holding C",
		FundType:      "equity",
		CurrentAmount: 500,
		CurrentWeight: 12,
		ProfitRatio:   8,
		ProfitAmount:  40,
		Score:         88,
		StrategyScore: 80,
		Drawdown:      14,
		Stddev:        25,
		RecentReturns: FundDailyAIReturns{Month1: 20, Month3: 25, Month6: 30},
		TopStocks:     []FundDailyTopHolding{{Name: "SharedStock", HoldRatio: 6}},
	}
	signals := fundDailyLocalSignals{
		PortfolioTopStocks: map[string]float64{"SharedStock": 10},
	}

	assessment := assessFundDailyUpsideRoom(fund, signals)

	if assessment.room != fundDailyUpsideRoomStretched {
		t.Fatalf("expected stretched upside room, got %s; assessment=%+v", assessment.room, assessment)
	}
	if assessment.trimRatio != 0.10 {
		t.Fatalf("expected 10%% trim ratio, got %.2f; assessment=%+v", assessment.trimRatio, assessment)
	}
}

func TestAssessFundDailyUpsideRoomSetsLimitedRiskTrimRatio(t *testing.T) {
	expectedReturn := 35.0
	fund := FundDailyAIFund{
		Code:                 "001407",
		Name:                 "AI stretched holding C",
		FundType:             "equity",
		CurrentAmount:        500,
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
	signals := fundDailyLocalSignals{
		PortfolioTopStocks: map[string]float64{"SharedAI": 18},
		TechConcentrated:   true,
	}

	assessment := assessFundDailyUpsideRoom(fund, signals)

	if !assessment.allowAutomaticTrim {
		t.Fatalf("expected limited upside to allow automatic trim, got %+v", assessment)
	}
	if assessment.room != fundDailyUpsideRoomLimited {
		t.Fatalf("expected limited upside room, got %s; assessment=%+v", assessment.room, assessment)
	}
	if assessment.trimRatio != 0.15 {
		t.Fatalf("expected 15%% trim ratio, got %.2f; assessment=%+v", assessment.trimRatio, assessment)
	}
}

func TestAssessFundDailyUpsideRoomSetsExhaustedTrimRatio(t *testing.T) {
	fund := FundDailyAIFund{
		Code:          "400003",
		Name:          "exhausted holding C",
		FundType:      "equity",
		CurrentAmount: 500,
		CurrentWeight: 30,
		ProfitRatio:   30,
		ProfitAmount:  160,
		Score:         90,
		StrategyScore: 90,
		Drawdown:      42,
		Stddev:        42,
		RecentReturns: FundDailyAIReturns{Month1: 31, Month3: 72, Month6: 94},
		TopStocks:     []FundDailyTopHolding{{Name: "SharedAI", HoldRatio: 9}},
	}
	signals := fundDailyLocalSignals{
		PortfolioTopStocks: map[string]float64{"SharedAI": 18},
		TechConcentrated:   true,
	}

	assessment := assessFundDailyUpsideRoom(fund, signals)

	if assessment.room != fundDailyUpsideRoomExhausted {
		t.Fatalf("expected exhausted upside room, got %s; assessment=%+v", assessment.room, assessment)
	}
	if assessment.trimRatio != 0.20 {
		t.Fatalf("expected 20%% trim ratio, got %.2f; assessment=%+v", assessment.trimRatio, assessment)
	}
}

func TestProfitTakingDefersExplicitTrimWhenUpsideIsConstructive(t *testing.T) {
	expectedReturn := 16.0
	fund := FundDailyAIFund{
		Code:                 "400004",
		Name:                 "constructive explicit trim C",
		FundType:             "equity",
		ProgramActionLevel:   "trim",
		SuggestedSellAmount:  150,
		CurrentAmount:        500,
		CurrentWeight:        10,
		ProfitRatio:          10,
		ProfitAmount:         50,
		Score:                90,
		StrategyScore:        90,
		ExpectedAnnualReturn: &expectedReturn,
		Drawdown:             14,
		Stddev:               14,
		Sharp:                1.1,
		RecentReturns:        FundDailyAIReturns{Month1: 13, Month3: 28, Month6: 38},
		RankRatios:           FundDailyAIRankRatios{Month1: 20, Month3: 22, Month6: 24, ThisYear: 21, Year1: 23},
		TopStocks:            []FundDailyTopHolding{{Name: "SharedGrowth", HoldRatio: 6}},
	}
	signals := fundDailyLocalSignals{
		PortfolioTopStocks: map[string]float64{"SharedGrowth": 6},
		TechConcentrated:   true,
	}

	_, ok := fundDailyLocalProfitTakingOptionForFund(fund, signals)

	if ok {
		t.Fatalf("expected explicit trim to defer when upside remains constructive")
	}
}

func TestProfitTakingAssessesLimitedUpsideWithoutLegacyRunup(t *testing.T) {
	fund := FundDailyAIFund{
		Code:          "400006",
		Name:          "limited upside high risk C",
		FundType:      "equity",
		CurrentAmount: 500,
		CurrentWeight: 20,
		ProfitRatio:   36,
		ProfitAmount:  160,
		Drawdown:      34,
		Stddev:        34,
		RecentReturns: FundDailyAIReturns{Month1: 5, Month3: 8, Month6: 12},
		TopStocks:     []FundDailyTopHolding{{Name: "SharedRisk", HoldRatio: 7}},
	}
	signals := fundDailyLocalSignals{
		PortfolioTopStocks: map[string]float64{"SharedRisk": 11},
	}

	option, ok := fundDailyLocalProfitTakingOptionForFund(fund, signals)

	if !ok {
		t.Fatalf("expected limited upside and high risk to trigger trim even without legacy runup")
	}
	if option.upside.room != fundDailyUpsideRoomLimited {
		t.Fatalf("expected limited upside room, got %s; option=%+v", option.upside.room, option)
	}
	if option.amount != 80 {
		t.Fatalf("expected 15%% trim rounded to 80.00, got %.2f; option=%+v", option.amount, option)
	}
}

func TestProfitTakingCapsExplicitTrimByExhaustedUpsideRatio(t *testing.T) {
	fund := FundDailyAIFund{
		Code:                "400005",
		Name:                "exhausted explicit trim C",
		FundType:            "equity",
		ProgramActionLevel:  "trim",
		SuggestedSellAmount: 250,
		CurrentAmount:       500,
		CurrentWeight:       30,
		ProfitRatio:         30,
		ProfitAmount:        160,
		Score:               90,
		StrategyScore:       90,
		Drawdown:            42,
		Stddev:              42,
		RecentReturns:       FundDailyAIReturns{Month1: 31, Month3: 72, Month6: 94},
		TopStocks:           []FundDailyTopHolding{{Name: "SharedAI", HoldRatio: 9}},
	}
	signals := fundDailyLocalSignals{
		PortfolioTopStocks: map[string]float64{"SharedAI": 18},
		TechConcentrated:   true,
	}

	option, ok := fundDailyLocalProfitTakingOptionForFund(fund, signals)

	if !ok {
		t.Fatalf("expected exhausted explicit trim to remain actionable")
	}
	if option.amount != 100 {
		t.Fatalf("expected explicit trim capped by 20%% upside ratio to 100.00, got %.2f; option=%+v", option.amount, option)
	}
}
