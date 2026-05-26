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
	require.Len(t, payload.Stocks, 1)
	require.Equal(t, "赤峰黄金", payload.Stocks[0].Name)
}
