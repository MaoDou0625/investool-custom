package routes

import (
	"testing"

	"github.com/axiaoxin-com/investool/datacenter/eastmoney"
	"github.com/stretchr/testify/require"
)

func TestBuildCorrelationChartDataAlignsByFundNAVDates(t *testing.T) {
	sources := []fundPortfolioCorrelationSource{
		{
			Code: "000002",
			Name: "Fund B",
			History: eastmoney.FundNAVHistory{
				{Date: "2026-05-01", UnitNAV: 1.00},
				{Date: "2026-05-02", UnitNAV: 1.02},
				{Date: "2026-05-03", UnitNAV: 1.01},
				{Date: "2026-05-04", UnitNAV: 1.04},
				{Date: "2026-05-05", UnitNAV: 1.03},
				{Date: "2026-05-06", UnitNAV: 1.06},
				{Date: "2026-05-07", UnitNAV: 1.05},
				{Date: "2026-05-08", UnitNAV: 1.08},
				{Date: "2026-05-09", UnitNAV: 1.07},
				{Date: "2026-05-10", UnitNAV: 1.10},
			},
		},
		{
			Code: "000001",
			Name: "Fund A",
			History: eastmoney.FundNAVHistory{
				{Date: "2026-05-01", UnitNAV: 2.00},
				{Date: "2026-05-02", UnitNAV: 2.04},
				{Date: "2026-05-03", UnitNAV: 2.02},
				{Date: "2026-05-04", UnitNAV: 2.08},
				{Date: "2026-05-05", UnitNAV: 2.06},
				{Date: "2026-05-06", UnitNAV: 2.12},
				{Date: "2026-05-07", UnitNAV: 2.10},
				{Date: "2026-05-08", UnitNAV: 2.16},
				{Date: "2026-05-09", UnitNAV: 2.14},
				{Date: "2026-05-10", UnitNAV: 2.20},
			},
		},
	}

	data := buildCorrelationChartData(sources)

	require.Equal(t, []string{"000001 Fund A", "000002 Fund B"}, data.Labels)
	require.Len(t, data.Points, 4)
	require.InDelta(t, 1, data.Points[0].Value, 0.00001)
	require.Greater(t, data.Points[1].Value, 0.99)
}

func TestBuildCorrelationChartDataRequiresEnoughFundNAVReturns(t *testing.T) {
	sources := []fundPortfolioCorrelationSource{
		{
			Code: "000001",
			Name: "Fund A",
			History: eastmoney.FundNAVHistory{
				{Date: "2026-05-01", UnitNAV: 1.00},
				{Date: "2026-05-02", UnitNAV: 1.01},
			},
		},
		{
			Code: "000002",
			Name: "Fund B",
			History: eastmoney.FundNAVHistory{
				{Date: "2026-05-01", UnitNAV: 1.00},
				{Date: "2026-05-02", UnitNAV: 1.02},
			},
		},
	}

	data := buildCorrelationChartData(sources)

	require.Empty(t, data.Labels)
	require.Empty(t, data.Points)
}
