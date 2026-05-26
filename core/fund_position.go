package core

import "github.com/axiaoxin-com/investool/models"

const (
	positionCurrentAmountSourceInput = "input_current_amount"
	positionCurrentAmountSourceNAV   = "estimated_from_nav"
)

type FundPositionMetrics struct {
	CostAmount    float64
	CurrentAmount float64
	ProfitRatio   float64
	ProfitAmount  float64
	CurrentSource string
}

func CalculateFundPositionMetrics(item models.FundPortfolioItem, fund *models.Fund) (FundPositionMetrics, bool, []string) {
	if !item.IsOwned() {
		return FundPositionMetrics{}, false, nil
	}

	warnings := []string{}
	if item.CostNav <= 0 || item.HoldingShares <= 0 {
		return FundPositionMetrics{}, false, append(warnings, "未填写成本净值或持有份额，无法估算持仓收益")
	}

	costAmount := item.CostNav * item.HoldingShares
	if costAmount <= 0 {
		return FundPositionMetrics{}, false, append(warnings, "成本金额无效，无法估算持仓收益")
	}

	currentAmount := item.HoldingAmount
	currentSource := positionCurrentAmountSourceInput
	if currentAmount <= 0 {
		if fund == nil || fund.UnitNav <= 0 {
			return FundPositionMetrics{}, false, append(warnings, "未填写当前总值且缺少当前净值，无法估算持仓收益")
		}
		currentAmount = fund.UnitNav * item.HoldingShares
		currentSource = positionCurrentAmountSourceNAV
		warnings = append(warnings, "未填写当前总值，已按当前净值 × 份额估算持仓总值")
	}

	profitAmount := currentAmount - costAmount
	return FundPositionMetrics{
		CostAmount:    costAmount,
		CurrentAmount: currentAmount,
		ProfitRatio:   profitAmount / costAmount * 100,
		ProfitAmount:  profitAmount,
		CurrentSource: currentSource,
	}, true, warnings
}
