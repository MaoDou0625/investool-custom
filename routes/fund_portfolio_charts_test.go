package routes

import (
	"encoding/json"
	"testing"

	"github.com/axiaoxin-com/investool/core"
	"github.com/axiaoxin-com/investool/datacenter/eastmoney"
	"github.com/axiaoxin-com/investool/models"
	"github.com/stretchr/testify/require"
)

func TestBuildFundPortfolioChartDataJSON(t *testing.T) {
	report := core.FundPortfolioExposureReport{
		ThemeExposures: []core.FundPortfolioThemeExposure{
			{
				Theme:       "黄金/贵金属",
				Amount:      1200,
				Weight:      42.5,
				SourceFunds: "002207 42.5%",
				SourceBreakdowns: []core.FundPortfolioExposureSource{
					{Name: "002207", Amount: 900, Weight: 31.8},
					{Name: "000001", Amount: 300, Weight: 10.7},
				},
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
			{
				StockCode:       "601899",
				StockName:       "紫金矿业",
				Theme:           "黄金/贵金属",
				Industry:        "有色金属",
				Amount:          80,
				Weight:          3.0,
				ExposureSummary: "000001 持仓 4.00%",
			},
		},
	}
	advices := []core.FundPortfolioAdvice{
		buildChartAdvice("002207", "前海开源金银珠宝混合C", models.FundPortfolioStatusOwned, 70, 1200, 42.5, 12, 18, 1.1, 35),
		buildChartAdvice("000001", "候选黄金ETF联接", models.FundPortfolioStatusWatch, 82, 0, 0, 10, 16, 1.3, 45),
	}
	history := models.FundPortfolioHistory{
		Snapshots: []models.FundPortfolioSnapshot{
			buildChartSnapshot("2026-05-24", 1000, 1100, []models.FundPortfolioSnapshotItem{
				{Code: "002207", Name: "前海开源金银珠宝混合C", UnitNav: 2.50, CurrentAmount: 700},
				{Code: "000001", Name: "候选黄金ETF联接", UnitNav: 1.00, CurrentAmount: 300},
			}),
			buildChartSnapshot("2026-05-25", 1060, 1100, []models.FundPortfolioSnapshotItem{
				{Code: "002207", Name: "前海开源金银珠宝混合C", UnitNav: 2.55, CurrentAmount: 735},
				{Code: "000001", Name: "候选黄金ETF联接", UnitNav: 1.03, CurrentAmount: 325},
			}),
			buildChartSnapshot("2026-05-26", 1200, 1100, []models.FundPortfolioSnapshotItem{
				{Code: "002207", Name: "前海开源金银珠宝混合C", UnitNav: 2.70, CurrentAmount: 840},
				{Code: "000001", Name: "候选黄金ETF联接", UnitNav: 1.08, CurrentAmount: 360},
			}),
		},
	}
	correlationSources := []fundPortfolioCorrelationSource{
		buildChartCorrelationSource("002207", "鍓嶆捣寮€婧愰噾閾剁彔瀹濇贩鍚圕", []float64{1.00, 1.02, 1.01, 1.05, 1.07, 1.04, 1.08, 1.10, 1.12}),
		buildChartCorrelationSource("000001", "鍊欓€夐粍閲慐TF鑱旀帴", []float64{1.00, 1.01, 1.03, 1.06, 1.05, 1.08, 1.11, 1.10, 1.14}),
	}

	raw := []byte(buildFundPortfolioChartDataJSON(report, advices, history, correlationSources))
	payload := fundPortfolioChartData{}
	require.NoError(t, json.Unmarshal(raw, &payload))

	require.Len(t, payload.Themes, 1)
	require.Equal(t, "黄金/贵金属", payload.Themes[0].Name)
	require.Len(t, payload.ThemeSources, 2)
	require.Len(t, payload.Stocks, 2)
	require.Equal(t, "赤峰黄金", payload.Stocks[0].Name)
	require.InDelta(t, 7.6, payload.StockConcentration.Top10Weight, 0.01)
	require.Len(t, payload.RiskReturns, 2)
	require.Len(t, payload.History, 3)
	require.Len(t, payload.Correlation.Labels, 2)
	require.Len(t, payload.Correlation.Points, 4)
	require.Len(t, payload.ComparisonMetrics, 6)
	require.Len(t, payload.Comparisons, 2)
}

func buildChartAdvice(
	code string,
	name string,
	status string,
	score int,
	currentAmount float64,
	currentWeight float64,
	expectedReturn float64,
	drawdown float64,
	sharp float64,
	institutionalHolding float64,
) core.FundPortfolioAdvice {
	fund := &models.Fund{
		Code:           code,
		Name:           name,
		UnitNav:        1,
		NetAssetsScale: 20 * 100000000,
	}
	fund.Performance.Year1ProfitRatio = expectedReturn - 1
	fund.Stddev.Avg135 = drawdown + 2
	fund.MaxRetracement.Avg135 = drawdown
	fund.Sharp.Avg135 = sharp

	return core.FundPortfolioAdvice{
		Item: models.FundPortfolioItem{
			Code:   code,
			Status: status,
		},
		Fund:                    fund,
		HolderStructure:         eastmoney.FundHolderStructureResult{Latest: &eastmoney.FundHolderStructure{InstitutionalHoldingRatio: institutionalHolding}},
		Score:                   score,
		Action:                  "观察",
		HasPosition:             currentAmount > 0,
		CurrentAmount:           currentAmount,
		CurrentWeight:           currentWeight,
		HasExpectedAnnualReturn: true,
		ExpectedAnnualReturn:    expectedReturn,
	}
}

func buildChartSnapshot(date string, amount float64, cost float64, items []models.FundPortfolioSnapshotItem) models.FundPortfolioSnapshot {
	profit := amount - cost
	profitRatio := 0.0
	if cost > 0 {
		profitRatio = profit / cost * 100
	}
	return models.FundPortfolioSnapshot{
		Date:               date,
		TotalCurrentAmount: amount,
		CostAmount:         cost,
		ProfitAmount:       profit,
		ProfitRatio:        profitRatio,
		Items:              items,
	}
}

func buildChartCorrelationSource(code string, name string, unitNAVs []float64) fundPortfolioCorrelationSource {
	dates := []string{
		"2026-05-14",
		"2026-05-15",
		"2026-05-18",
		"2026-05-19",
		"2026-05-20",
		"2026-05-21",
		"2026-05-22",
		"2026-05-25",
		"2026-05-26",
	}
	history := make(eastmoney.FundNAVHistory, 0, len(unitNAVs))
	for idx, unitNAV := range unitNAVs {
		history = append(history, eastmoney.FundNAVHistoryPoint{
			Date:    dates[idx],
			UnitNAV: unitNAV,
		})
	}
	return fundPortfolioCorrelationSource{
		Code:    code,
		Name:    name,
		History: history,
	}
}
