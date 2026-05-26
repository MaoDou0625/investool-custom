package core

import "math"

func ApplyFundPortfolioAllocationMetrics(advices []FundPortfolioAdvice, totalCurrentAmount float64) {
	for idx := range advices {
		advice := &advices[idx]
		if advice.HasPosition && totalCurrentAmount > 0 {
			advice.CurrentWeight = advice.CurrentAmount / totalCurrentAmount * 100
		}
		advice.RecommendedWeight = RecommendFundWeight(
			advice.Score,
			advice.RiskLevel,
			advice.ExpectedAnnualReturn,
			advice.HasExpectedAnnualReturn,
		)
		if advice.HasPosition {
			advice.AllocationGap = advice.RecommendedWeight - advice.CurrentWeight
		}
	}
}

func RecommendFundWeight(score int, riskLevel string, expectedAnnualReturn float64, hasExpectedReturn bool) float64 {
	recommended := baseRecommendedWeight(score)
	if hasExpectedReturn {
		switch {
		case expectedAnnualReturn >= 20:
			recommended += 2
		case expectedAnnualReturn < 0:
			recommended -= 3
		case expectedAnnualReturn < 5:
			recommended -= 1
		}
	}

	recommended = math.Min(recommended, riskWeightCap(score, riskLevel))
	return roundToOneDecimal(clampFloat(recommended, 0, 25))
}

func baseRecommendedWeight(score int) float64 {
	switch {
	case score >= 85:
		return 18
	case score >= 75:
		return 14
	case score >= 65:
		return 10
	case score >= 50:
		return 5
	default:
		return 2
	}
}

func riskWeightCap(score int, riskLevel string) float64 {
	if score < 50 {
		return 3
	}
	switch riskLevel {
	case "偏高":
		return 8
	case "中等":
		return 12
	case "中低":
		return 18
	default:
		return 10
	}
}

func roundToOneDecimal(n float64) float64 {
	return math.Round(n*10) / 10
}
