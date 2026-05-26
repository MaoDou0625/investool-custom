package routes

import (
	"math"
	"sort"

	"github.com/axiaoxin-com/investool/models"
)

func buildHistoryChartPoints(history models.FundPortfolioHistory) []fundPortfolioHistoryChartPoint {
	points := make([]fundPortfolioHistoryChartPoint, 0, len(history.Snapshots))
	for _, snapshot := range history.Snapshots {
		if snapshot.Date == "" || snapshot.TotalCurrentAmount <= 0 {
			continue
		}
		points = append(points, fundPortfolioHistoryChartPoint{
			Date:        snapshot.Date,
			Amount:      snapshot.TotalCurrentAmount,
			ProfitRatio: snapshot.ProfitRatio,
			Profit:      snapshot.ProfitAmount,
		})
	}
	return points
}

func buildCorrelationChartData(history models.FundPortfolioHistory) fundPortfolioCorrelationChartData {
	data := fundPortfolioCorrelationChartData{}
	if len(history.Snapshots) < 2 {
		return data
	}

	valuesByCode := map[string][]fundPortfolioSnapshotValue{}
	namesByCode := map[string]string{}
	for _, snapshot := range history.Snapshots {
		for _, item := range snapshot.Items {
			value := item.UnitNav
			if value <= 0 {
				value = item.CurrentAmount
			}
			if value <= 0 {
				continue
			}
			valuesByCode[item.Code] = append(valuesByCode[item.Code], fundPortfolioSnapshotValue{
				date:  snapshot.Date,
				value: value,
			})
			if item.Name != "" {
				namesByCode[item.Code] = item.Name
			}
		}
	}

	codes := []string{}
	returnsByCode := map[string][]float64{}
	for code, values := range valuesByCode {
		returns := snapshotReturns(values)
		if len(returns) < 2 {
			continue
		}
		codes = append(codes, code)
		returnsByCode[code] = returns
	}
	sort.Strings(codes)
	if len(codes) < 2 {
		return data
	}

	for _, code := range codes {
		data.Labels = append(data.Labels, shortFundLabel(code, namesByCode[code]))
	}
	for y, yCode := range codes {
		for x, xCode := range codes {
			data.Points = append(data.Points, fundPortfolioCorrelationHeatmapPoint{
				X:     x,
				Y:     y,
				Value: pearsonCorrelation(returnsByCode[xCode], returnsByCode[yCode]),
			})
		}
	}
	return data
}

type fundPortfolioSnapshotValue struct {
	date  string
	value float64
}

func snapshotReturns(values []fundPortfolioSnapshotValue) []float64 {
	sort.SliceStable(values, func(i, j int) bool {
		return values[i].date < values[j].date
	})
	returns := []float64{}
	for idx := 1; idx < len(values); idx++ {
		prev := values[idx-1].value
		cur := values[idx].value
		if prev <= 0 || cur <= 0 {
			continue
		}
		returns = append(returns, cur/prev-1)
	}
	return returns
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
