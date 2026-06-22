package core

import (
	"fmt"
	"math"
)

func fundDailyRedeemShareAmount(amount float64, fund FundDailyAIFund) float64 {
	return fundDailyRedeemShareAmountFrom(amount, fund.CurrentAmount, fund.HoldingShares, fund.UnitNAV)
}

func fundDailyRedeemShareAmountFrom(amount float64, currentAmount float64, holdingShares float64, unitNAV float64) float64 {
	if amount <= 0 {
		return 0
	}
	if holdingShares > 0 && currentAmount > 0 {
		return roundFundDailyRedeemShares(amount / currentAmount * holdingShares)
	}
	if unitNAV > 0 {
		return roundFundDailyRedeemShares(amount / unitNAV)
	}
	return 0
}

func fundDailyRedeemShareBasisText(fund FundDailyAIFund) string {
	if fund.HoldingShares > 0 && fund.CurrentAmount > 0 {
		return fmt.Sprintf("按当前持仓 %.2f 元 / %.2f 份折算", fund.CurrentAmount, fund.HoldingShares)
	}
	if fund.UnitNAV > 0 {
		return fmt.Sprintf("按单位净值 %.4f 元折算", fund.UnitNAV)
	}
	return ""
}

func roundFundDailyRedeemShares(shares float64) float64 {
	return math.Round(shares*100) / 100
}
