package core

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseFundPortfolioTianTianTextFromDetailText(t *testing.T) {
	raw := `前海开源金银珠宝混合C
002207 中高风险
金额(元) 1,256.30
持有金额 1,246.30
持仓成本价 2.8637
持有份额 460.91
基金净值 2.7040(05-25)`

	report, err := ParseFundPortfolioTianTianText(raw)

	require.NoError(t, err)
	require.Len(t, report.Drafts, 1)
	draft := report.Drafts[0]
	require.Equal(t, "002207", draft.Item.Code)
	require.Equal(t, "前海开源金银珠宝混合C", draft.Name)
	require.Equal(t, 1246.30, draft.Item.CurrentAmount)
	require.Equal(t, 2.8637, draft.Item.CostNav)
	require.Equal(t, 460.91, draft.Item.HoldingShares)
	require.Equal(t, 2.7040, draft.UnitNAV)
	require.True(t, draft.HasImportableData())
}

func TestParseFundPortfolioTianTianTextMultipleFunds(t *testing.T) {
	raw := `易方达创业板ETF联接A 110026 持有金额 100.50 持有份额 99.00 成本净值 1.2345
华夏上证50ETF联接A 001051 持有金额 200.00 持有份额 80.00 成本价 2.5000`

	report, err := ParseFundPortfolioTianTianText(raw)

	require.NoError(t, err)
	require.Len(t, report.Drafts, 2)
	require.Equal(t, "110026", report.Drafts[0].Item.Code)
	require.Equal(t, 100.50, report.Drafts[0].Item.CurrentAmount)
	require.Equal(t, "001051", report.Drafts[1].Item.Code)
	require.Equal(t, 200.00, report.Drafts[1].Item.CurrentAmount)
}

func TestParseFundPortfolioTianTianTextRequiresCode(t *testing.T) {
	report, err := ParseFundPortfolioTianTianText("持有金额 100.00")

	require.Error(t, err)
	require.Empty(t, report.Drafts)
}
