package core

import "testing"

func TestFundDailyLocalRebalanceBuyAmountAllowsTwentyYuanRemainder(t *testing.T) {
	fund := FundDailyAIFund{SuggestedBuyCeiling: 300}

	if !fundDailyLocalHasMeaningfulBuyRoom(20) {
		t.Fatalf("expected 20 yuan to be meaningful buy room")
	}
	if fundDailyLocalHasMeaningfulBuyRoom(10) {
		t.Fatalf("expected 10 yuan to be below meaningful buy room")
	}
	if got := fundDailyLocalRebalanceBuyAmount(fund, 80, 20); got != 20 {
		t.Fatalf("expected 20 yuan remainder to be allocatable, got %.2f", got)
	}
}
