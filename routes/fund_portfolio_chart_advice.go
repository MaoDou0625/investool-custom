package routes

import (
	"sort"

	"github.com/axiaoxin-com/investool/core"
)

func buildRiskReturnChartPoints(advices []core.FundPortfolioAdvice) []fundPortfolioRiskReturnChartPoint {
	points := []fundPortfolioRiskReturnChartPoint{}
	for _, advice := range advices {
		if advice.Fund == nil {
			continue
		}
		expectedReturn, label, ok := riskReturnYValue(advice)
		if !ok {
			continue
		}
		risk := advice.Fund.MaxRetracement.Avg135
		if risk <= 0 {
			risk = advice.Fund.Stddev.Avg135
		}
		if risk <= 0 {
			continue
		}
		points = append(points, fundPortfolioRiskReturnChartPoint{
			Code:           advice.Item.Code,
			Name:           advice.Fund.Name,
			Status:         advice.Item.StatusName(),
			Action:         advice.Action,
			Score:          advice.Score,
			CurrentAmount:  advice.CurrentAmount,
			CurrentWeight:  advice.CurrentWeight,
			ExpectedReturn: expectedReturn,
			ReturnLabel:    label,
			Risk:           risk,
			Drawdown:       advice.Fund.MaxRetracement.Avg135,
			Stddev:         advice.Fund.Stddev.Avg135,
		})
	}
	sort.Slice(points, func(i, j int) bool {
		return points[i].CurrentAmount > points[j].CurrentAmount
	})
	return points
}

func riskReturnYValue(advice core.FundPortfolioAdvice) (float64, string, bool) {
	if advice.HasExpectedAnnualReturn {
		return advice.ExpectedAnnualReturn, "预期年化收益", true
	}
	if advice.Fund != nil && advice.Fund.Performance.Year1ProfitRatio != 0 {
		return advice.Fund.Performance.Year1ProfitRatio, "近1年收益", true
	}
	return 0, "", false
}

func buildComparisonChartPoints(advices []core.FundPortfolioAdvice) []fundPortfolioComparisonChartPoint {
	points := []fundPortfolioComparisonChartPoint{}
	for _, advice := range advices {
		if advice.Fund == nil {
			continue
		}
		points = append(points, fundPortfolioComparisonChartPoint{
			Code:   advice.Item.Code,
			Name:   shortFundLabel(advice.Item.Code, advice.Fund.Name),
			Status: advice.Item.StatusName(),
			Values: []float64{
				float64(advice.Score),
				normalizeExpectedReturn(advice),
				clampFloat(advice.Fund.Sharp.Avg135/2*100, 0, 100),
				normalizeDrawdownControl(advice.Fund.MaxRetracement.Avg135),
				normalizeInstitutionalHolding(advice),
				normalizeFundScale(advice.Fund.NetAssetsScale),
			},
		})
	}
	sort.Slice(points, func(i, j int) bool {
		return points[i].Values[0] > points[j].Values[0]
	})
	if len(points) > 6 {
		points = points[:6]
	}
	return points
}

func normalizeExpectedReturn(advice core.FundPortfolioAdvice) float64 {
	value, _, ok := riskReturnYValue(advice)
	if !ok {
		return 0
	}
	return clampFloat((value+10)/40*100, 0, 100)
}

func normalizeDrawdownControl(drawdown float64) float64 {
	if drawdown <= 0 {
		return 0
	}
	return clampFloat(100-drawdown/60*100, 0, 100)
}

func normalizeInstitutionalHolding(advice core.FundPortfolioAdvice) float64 {
	if advice.HolderStructure.Latest == nil {
		return 0
	}
	return clampFloat(advice.HolderStructure.Latest.InstitutionalHoldingRatio, 0, 100)
}

func normalizeFundScale(netAssetsScale float64) float64 {
	scaleYi := netAssetsScale / 100000000
	switch {
	case scaleYi <= 0:
		return 0
	case scaleYi >= 2 && scaleYi <= 80:
		return 100
	case scaleYi < 2:
		return clampFloat(scaleYi/2*80, 0, 80)
	case scaleYi <= 150:
		return clampFloat(100-(scaleYi-80)/70*40, 0, 100)
	default:
		return 40
	}
}
