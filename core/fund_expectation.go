package core

import (
	"math"

	"github.com/axiaoxin-com/investool/models"
)

type weightedAnnualReturn struct {
	totalReturn float64
	years       float64
	weight      float64
}

func EstimateFundAnnualReturn(fund *models.Fund) (float64, bool) {
	if fund == nil {
		return 0, false
	}

	samples := []weightedAnnualReturn{
		{totalReturn: fund.Performance.Year1ProfitRatio, years: 1, weight: 0.25},
		{totalReturn: fund.Performance.Year2ProfitRatio, years: 2, weight: 0.20},
		{totalReturn: fund.Performance.Year3ProfitRatio, years: 3, weight: 0.30},
		{totalReturn: fund.Performance.Year5ProfitRatio, years: 5, weight: 0.25},
	}
	if fund.Performance.Month6ProfitRatio != 0 {
		samples = append(samples, weightedAnnualReturn{
			totalReturn: fund.Performance.Month6ProfitRatio,
			years:       0.5,
			weight:      0.10,
		})
	}

	weightedReturn := 0.0
	totalWeight := 0.0
	for _, sample := range samples {
		if sample.totalReturn <= -99.9 || sample.years <= 0 || sample.weight <= 0 {
			continue
		}
		if sample.totalReturn == 0 {
			continue
		}
		weightedReturn += annualizeReturn(sample.totalReturn, sample.years) * sample.weight
		totalWeight += sample.weight
	}
	if totalWeight == 0 {
		return 0, false
	}

	estimate := weightedReturn / totalWeight
	estimate += expectedReturnRiskAdjustment(fund)
	return clampFloat(estimate, -20, 35), true
}

func annualizeReturn(totalReturn float64, years float64) float64 {
	return (math.Pow(1+totalReturn/100, 1/years) - 1) * 100
}

func expectedReturnRiskAdjustment(fund *models.Fund) float64 {
	adjustment := 0.0
	switch {
	case fund.Sharp.Avg135 >= 1.2:
		adjustment += 2
	case fund.Sharp.Avg135 > 0 && fund.Sharp.Avg135 < 0.6:
		adjustment -= 2
	}

	if fund.MaxRetracement.Avg135 > 30 {
		adjustment -= 3
	}
	if fund.Stddev.Avg135 > 28 {
		adjustment -= 2
	}
	return adjustment
}

func clampFloat(n, min, max float64) float64 {
	return math.Max(min, math.Min(max, n))
}
