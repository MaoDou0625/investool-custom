package routes

import (
	"math"
	"sort"

	"github.com/axiaoxin-com/investool/datacenter/eastmoney"
)

const minFundCorrelationReturnPairs = 8

func buildCorrelationChartData(sources []fundPortfolioCorrelationSource) fundPortfolioCorrelationChartData {
	data := fundPortfolioCorrelationChartData{}
	if len(sources) < 2 {
		return data
	}

	usableSources := make([]fundPortfolioCorrelationSource, 0, len(sources))
	returnsByCode := map[string]map[string]float64{}
	for _, source := range sources {
		returns := fundNAVDailyReturns(source.History)
		if len(returns) < minFundCorrelationReturnPairs {
			continue
		}
		usableSources = append(usableSources, source)
		returnsByCode[source.Code] = returns
	}
	sort.SliceStable(usableSources, func(i, j int) bool {
		return usableSources[i].Code < usableSources[j].Code
	})
	if len(usableSources) < 2 {
		return data
	}

	for _, source := range usableSources {
		data.Labels = append(data.Labels, shortFundLabel(source.Code, source.Name))
	}
	for y, ySource := range usableSources {
		for x, xSource := range usableSources {
			value := 1.0
			if xSource.Code != ySource.Code {
				xReturns, yReturns := alignFundNAVReturns(returnsByCode[xSource.Code], returnsByCode[ySource.Code])
				if len(xReturns) < minFundCorrelationReturnPairs {
					value = 0
				} else {
					value = pearsonCorrelation(xReturns, yReturns)
				}
			}
			data.Points = append(data.Points, fundPortfolioCorrelationHeatmapPoint{
				X:     x,
				Y:     y,
				Value: value,
			})
		}
	}
	return data
}

func fundNAVDailyReturns(history eastmoney.FundNAVHistory) map[string]float64 {
	points := make(eastmoney.FundNAVHistory, 0, len(history))
	for _, point := range history {
		if point.Date == "" || point.UnitNAV <= 0 {
			continue
		}
		points = append(points, point)
	}
	sort.SliceStable(points, func(i, j int) bool {
		return points[i].Date < points[j].Date
	})

	returns := map[string]float64{}
	for idx := 1; idx < len(points); idx++ {
		prev := points[idx-1].UnitNAV
		cur := points[idx].UnitNAV
		if prev <= 0 || cur <= 0 {
			continue
		}
		returns[points[idx].Date] = cur/prev - 1
	}
	return returns
}

func alignFundNAVReturns(a map[string]float64, b map[string]float64) ([]float64, []float64) {
	dates := make([]string, 0, len(a))
	for date := range a {
		if _, ok := b[date]; ok {
			dates = append(dates, date)
		}
	}
	sort.Strings(dates)

	alignedA := make([]float64, 0, len(dates))
	alignedB := make([]float64, 0, len(dates))
	for _, date := range dates {
		alignedA = append(alignedA, a[date])
		alignedB = append(alignedB, b[date])
	}
	return alignedA, alignedB
}

func pearsonCorrelation(a []float64, b []float64) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	if n == 0 {
		return 0
	}
	meanA := meanFloat64(a[:n])
	meanB := meanFloat64(b[:n])
	var numerator, sumA, sumB float64
	for idx := 0; idx < n; idx++ {
		da := a[idx] - meanA
		db := b[idx] - meanB
		numerator += da * db
		sumA += da * da
		sumB += db * db
	}
	denominator := math.Sqrt(sumA * sumB)
	if denominator == 0 {
		return 0
	}
	return numerator / denominator
}

func meanFloat64(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, value := range values {
		sum += value
	}
	return sum / float64(len(values))
}
