package core

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/axiaoxin-com/investool/datacenter/eastmoney"
	"github.com/axiaoxin-com/investool/models"
	"github.com/stretchr/testify/require"
)

func TestEvaluateFundPortfolioItemOwnedStrongFund(t *testing.T) {
	ctx := context.Background()
	fund := testAdvisorFund(t)
	item := models.FundPortfolioItem{
		Code:          "260104",
		Status:        models.FundPortfolioStatusOwned,
		CostNav:       1.8,
		HoldingShares: 1000,
	}
	holderStructure := eastmoney.FundHolderStructureResult{
		Latest: &eastmoney.FundHolderStructure{
			InstitutionalHoldingRatio: 45,
		},
	}

	advice := EvaluateFundPortfolioItem(ctx, item, fund, holderStructure)

	require.GreaterOrEqual(t, advice.Score, 80)
	require.Equal(t, "继续持有，可按目标仓位定投", advice.Action)
	require.InDelta(t, 11.11, advice.ProfitRatio, 0.01)
	require.InDelta(t, 200, advice.ProfitAmount, 0.01)
	require.NotEmpty(t, advice.Reasons)
}

func TestEvaluateFundPortfolioItemMissingFund(t *testing.T) {
	advice := EvaluateFundPortfolioItem(
		context.Background(),
		models.FundPortfolioItem{Code: "000000", Status: models.FundPortfolioStatusWatch},
		nil,
		eastmoney.FundHolderStructureResult{},
	)

	require.Equal(t, 0, advice.Score)
	require.Equal(t, "检查基金代码或稍后重试", advice.Action)
	require.NotEmpty(t, advice.Warnings)
}

func testAdvisorFund(t *testing.T) *models.Fund {
	t.Helper()

	raw := `{
		"JJXQ": {
			"Datas": {
				"FCODE": "260104",
				"SHORTNAME": "测试优选基金",
				"FTYPE": "混合型",
				"ESTABDATE": "2010-01-01",
				"DWJZ": "2.0000",
				"LJJZ": "3.5000",
				"RZDF": "1.20",
				"CURRENTDAYMARK": "2026-05-25",
				"RATE": "0.15"
			}
		},
		"JDZF": {
			"Datas": [
				{"title": "3Y", "syl": "5", "avg": "1", "hs300": "1", "rank": "20", "sc": "100"},
				{"title": "6Y", "syl": "8", "avg": "1", "hs300": "1", "rank": "25", "sc": "100"},
				{"title": "1N", "syl": "12", "avg": "1", "hs300": "1", "rank": "10", "sc": "100"},
				{"title": "2N", "syl": "25", "avg": "1", "hs300": "1", "rank": "15", "sc": "100"},
				{"title": "3N", "syl": "40", "avg": "1", "hs300": "1", "rank": "18", "sc": "100"},
				{"title": "5N", "syl": "80", "avg": "1", "hs300": "1", "rank": "20", "sc": "100"},
				{"title": "JN", "syl": "6", "avg": "1", "hs300": "1", "rank": "15", "sc": "100"}
			]
		},
		"JJGM": {
			"Datas": [
				{"FSRQ": "2026-03-31", "NETNAV": "3000000000"}
			]
		},
		"TSSJ": {
			"Datas": {
				"SHARP1": "1.5",
				"SHARP3": "1.3",
				"SHARP5": "1.2",
				"MAXRETRA1": "15",
				"MAXRETRA3": "18",
				"MAXRETRA5": "20",
				"STDDEV1": "16",
				"STDDEV3": "18",
				"STDDEV5": "19"
			}
		},
		"JJJLNEW": {
			"Datas": [
				{
					"MANGER": [
						{
							"MGRID": "1",
							"MGRNAME": "测试经理",
							"TOTALDAYS": "3000",
							"DAYS": "2500",
							"PENAVGROWTH": "120",
							"YIELDSE": "15"
						}
					]
				}
			]
		}
	}`

	resp := eastmoney.RespFundInfo{}
	require.NoError(t, json.Unmarshal([]byte(raw), &resp))
	return models.NewFund(context.Background(), &resp)
}
