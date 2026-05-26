package routes

import (
	"sort"

	"github.com/axiaoxin-com/investool/datacenter/eastmoney"
)

const maxFundNAVTrendPoints = 90

func buildFundNAVTrendChartData(sources []fundPortfolioCorrelationSource) fundPortfolioNAVTrendChartData {
	data := fundPortfolioNAVTrendChartData{}
	for _, source := range sources {
		points := fundNAVTrendPoints(source.History)
		if len(points) < 2 {
			continue
		}
		data.Series = append(data.Series, fundPortfolioNAVTrendSeries{
			Code:   source.Code,
			Name:   shortFundLabel(source.Code, source.Name),
			Points: points,
		})
	}
	return data
}

func fundNAVTrendPoints(history eastmoney.FundNAVHistory) []fundPortfolioNAVTrendPoint {
	points := normalizeFundNAVHistory(history)
	if len(points) > maxFundNAVTrendPoints {
		points = points[len(points)-maxFundNAVTrendPoints:]
	}
	if len(points) < 2 {
		return nil
	}

	baseNAV := points[0].UnitNAV
	if baseNAV <= 0 {
		return nil
	}
	trendPoints := make([]fundPortfolioNAVTrendPoint, 0, len(points))
	for _, point := range points {
		trendPoints = append(trendPoints, fundPortfolioNAVTrendPoint{
			Date:        point.Date,
			UnitNAV:     point.UnitNAV,
			ReturnRatio: (point.UnitNAV/baseNAV - 1) * 100,
		})
	}
	return trendPoints
}

func normalizeFundNAVHistory(history eastmoney.FundNAVHistory) eastmoney.FundNAVHistory {
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
	return points
}
