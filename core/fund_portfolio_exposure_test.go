package core

import (
	"encoding/json"
	"testing"

	"github.com/axiaoxin-com/investool/models"
	"github.com/stretchr/testify/require"
)

func TestFundPortfolioTargetETFCodesOwnedOnly(t *testing.T) {
	funds := map[string]*models.Fund{
		"017057": exposureTestFund(t, `{
			"code": "017057",
			"name": "嘉实国证绿色电力ETF发起联接C",
			"target_etf_code": "159625"
		}`),
		"020105": exposureTestFund(t, `{
			"code": "020105",
			"name": "易方达石油化工ETF联接C",
			"target_etf_code": "516570"
		}`),
	}
	items := []models.FundPortfolioItem{
		{Code: "017057", Status: models.FundPortfolioStatusOwned, CurrentAmount: 300},
		{Code: "020105", Status: models.FundPortfolioStatusWatch, CurrentAmount: 100},
	}

	codes := FundPortfolioTargetETFCodes(items, funds)

	require.Equal(t, []string{"159625"}, codes)
}

func TestBuildFundPortfolioExposureReportLooksThroughTargetETF(t *testing.T) {
	items := []models.FundPortfolioItem{
		{Code: "017057", Status: models.FundPortfolioStatusOwned, CurrentAmount: 1000},
	}
	funds := map[string]*models.Fund{
		"017057": exposureTestFund(t, `{
			"code": "017057",
			"name": "嘉实国证绿色电力ETF发起联接C",
			"target_etf_code": "159625",
			"target_etf_name": "绿色电力ETF嘉实"
		}`),
	}
	targetFunds := map[string]*models.Fund{
		"159625": exposureTestFund(t, `{
			"code": "159625",
			"name": "绿色电力ETF嘉实",
			"assets_proportion": {"stock": "99.39%"},
			"industry_proportions": [
				{"industry": "电力、热力、燃气及水生产和供应业", "prop": "94.80"},
				{"industry": "制造业", "prop": "4.58"},
				{"industry": "合计", "prop": "99.38"}
			],
			"stocks": [
				{"code": "601985", "name": "中国核电", "industry": "公用事业", "hold_ratio": 9.49}
			]
		}`),
	}

	report := BuildFundPortfolioExposureReport(items, funds, targetFunds)

	require.True(t, report.HasData())
	require.InDelta(t, 1000, report.TotalCurrentAmount, 0.01)
	require.Len(t, report.ETFLookThroughs, 1)
	require.True(t, report.ETFLookThroughs[0].LookedThrough)
	require.Equal(t, "159625", report.ETFLookThroughs[0].TargetETFCode)
	require.Equal(t, "已按目标ETF穿透", report.ETFLookThroughs[0].Status)
	require.NotEmpty(t, report.ThemeExposures)
	require.Equal(t, "绿色电力/公用事业", report.ThemeExposures[0].Theme)
	require.InDelta(t, 99.38, report.ThemeExposures[0].Weight, 0.01)
	require.Len(t, report.StockExposures, 1)
	require.True(t, report.StockExposures[0].LookedThrough)
	require.Equal(t, "017057 -> 159625 持仓 9.49%", report.StockExposures[0].ExposureSummary)
}

func TestBuildFundPortfolioExposureReportFallsBackToFundTheme(t *testing.T) {
	items := []models.FundPortfolioItem{
		{Code: "002207", Status: models.FundPortfolioStatusOwned, CurrentAmount: 500},
	}
	funds := map[string]*models.Fund{
		"002207": exposureTestFund(t, `{
			"code": "002207",
			"name": "前海开源金银珠宝混合C"
		}`),
	}

	report := BuildFundPortfolioExposureReport(items, funds, nil)

	require.Len(t, report.ThemeExposures, 1)
	require.Equal(t, "黄金/贵金属", report.ThemeExposures[0].Theme)
	require.Equal(t, "002207 缺少行业持仓数据，已按基金名称归类为黄金/贵金属", report.Warnings[0])
}

func TestInferFundThemeKeepsPowerEquipmentAsNewEnergy(t *testing.T) {
	require.Equal(t, "新能源/电力设备", inferFundTheme(&models.Fund{}, "电力设备", "宁德时代"))
	require.Equal(t, "绿色电力/公用事业", inferFundTheme(&models.Fund{}, "电力、热力、燃气及水生产和供应业", "长江电力"))
}

func exposureTestFund(t *testing.T, raw string) *models.Fund {
	t.Helper()

	fund := models.Fund{}
	require.NoError(t, json.Unmarshal([]byte(raw), &fund))
	return &fund
}
