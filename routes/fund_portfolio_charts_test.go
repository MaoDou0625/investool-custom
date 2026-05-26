package routes

import (
	"encoding/json"
	"testing"

	"github.com/axiaoxin-com/investool/core"
	"github.com/stretchr/testify/require"
)

func TestBuildFundPortfolioChartDataJSON(t *testing.T) {
	report := core.FundPortfolioExposureReport{
		ThemeExposures: []core.FundPortfolioThemeExposure{
			{Theme: "黄金/贵金属", Amount: 1200, Weight: 42.5, SourceFunds: "002207 42.5%"},
		},
		ETFLookThroughs: []core.FundPortfolioETFLookThrough{
			{
				FundCode:      "017057",
				FundName:      "嘉实国证绿色电力ETF发起联接C",
				CurrentAmount: 300,
				CurrentWeight: 10,
				TargetETFCode: "159625",
				TargetETFName: "绿色电力ETF嘉实",
				Status:        "已按目标ETF穿透",
			},
		},
		StockExposures: []core.FundPortfolioStockExposure{
			{
				StockCode:       "600988",
				StockName:       "赤峰黄金",
				Theme:           "黄金/贵金属",
				Industry:        "有色金属",
				Amount:          123,
				Weight:          4.6,
				ExposureSummary: "002207 持仓 9.87%",
			},
		},
	}

	raw := []byte(buildFundPortfolioChartDataJSON(report))
	payload := fundPortfolioChartData{}
	require.NoError(t, json.Unmarshal(raw, &payload))

	require.Len(t, payload.Themes, 1)
	require.Equal(t, "黄金/贵金属", payload.Themes[0].Name)
	require.Len(t, payload.ETFLinks, 1)
	require.Equal(t, "017057 嘉实国证绿色电力ETF发起联接C", payload.ETFLinks[0].SourceName)
	require.Equal(t, "159625 绿色电力ETF嘉实", payload.ETFLinks[0].TargetName)
	require.Len(t, payload.Stocks, 1)
	require.Equal(t, "赤峰黄金", payload.Stocks[0].Name)
}
