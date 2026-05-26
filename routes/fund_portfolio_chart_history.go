package routes

import "github.com/axiaoxin-com/investool/models"

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
