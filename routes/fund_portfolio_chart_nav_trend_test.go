package routes

import (
	"fmt"
	"testing"

	"github.com/axiaoxin-com/investool/datacenter/eastmoney"
	"github.com/stretchr/testify/require"
)

func TestBuildFundNAVTrendChartDataNormalizesReturnFromFirstPoint(t *testing.T) {
	data := buildFundNAVTrendChartData([]fundPortfolioCorrelationSource{
		{
			Code: "002207",
			Name: "Fund A",
			History: eastmoney.FundNAVHistory{
				{Date: "2026-05-01", UnitNAV: 1.00},
				{Date: "2026-05-02", UnitNAV: 1.02},
				{Date: "2026-05-03", UnitNAV: 0},
				{Date: "2026-05-04", UnitNAV: 1.05},
			},
		},
	})

	require.Len(t, data.Series, 1)
	require.Equal(t, "002207 Fund A", data.Series[0].Name)
	require.Len(t, data.Series[0].Points, 3)
	require.Equal(t, "2026-05-01", data.Series[0].Points[0].Date)
	require.InDelta(t, 0, data.Series[0].Points[0].ReturnRatio, 0.00001)
	require.InDelta(t, 5, data.Series[0].Points[2].ReturnRatio, 0.00001)
}

func TestFundNAVTrendPointsKeepsRecentWindow(t *testing.T) {
	history := make(eastmoney.FundNAVHistory, 0, maxFundNAVTrendPoints+2)
	for idx := 0; idx < maxFundNAVTrendPoints+2; idx++ {
		history = append(history, eastmoney.FundNAVHistoryPoint{
			Date:    "2026-05-" + twoDigitDay(idx+1),
			UnitNAV: 1 + float64(idx)/100,
		})
	}

	points := fundNAVTrendPoints(history)

	require.Len(t, points, maxFundNAVTrendPoints)
	require.Equal(t, "2026-05-03", points[0].Date)
}

func twoDigitDay(day int) string {
	return fmt.Sprintf("%02d", day)
}
