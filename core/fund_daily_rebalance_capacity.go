package core

const (
	fundDailyLocalMinRebalanceBuyAmount     = 20
	fundDailyLocalDefaultRebalanceBuyAmount = 60
	fundDailyLocalMediumRebalanceBuyAmount  = 80
	fundDailyLocalLargeRebalanceBuyAmount   = 100
)

func fundDailyLocalHasMeaningfulBuyRoom(remaining float64) bool {
	return remaining >= fundDailyLocalMinRebalanceBuyAmount
}

func fundDailyLocalRebalanceBaseBuyAmount(budget float64) float64 {
	switch {
	case budget >= 360:
		return fundDailyLocalLargeRebalanceBuyAmount
	case budget >= 180:
		return fundDailyLocalMediumRebalanceBuyAmount
	default:
		return fundDailyLocalDefaultRebalanceBuyAmount
	}
}

func fundDailyLocalRebalanceBuyAmount(fund FundDailyAIFund, budget float64, remaining float64) float64 {
	amount := minPositive(fundDailyLocalRebalanceBaseBuyAmount(budget), fund.SuggestedBuyCeiling, remaining)
	amount = floorDailyAmount(amount)
	if amount < fundDailyLocalMinRebalanceBuyAmount {
		return 0
	}
	return amount
}
