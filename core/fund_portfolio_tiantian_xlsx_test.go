package core

import (
	"bytes"
	"testing"

	"github.com/360EntSecGroup-Skylar/excelize/v2"
	"github.com/stretchr/testify/require"
)

func TestParseFundPortfolioTianTianXLSX(t *testing.T) {
	var buf bytes.Buffer
	file := excelize.NewFile()
	sheet := file.GetSheetName(file.GetActiveSheetIndex())
	rows := [][]interface{}{
		{"产品代码", "产品名称", "产品类型", "最新净值", "净值日期（年-月-日）", "金额（元）", "持仓收益（元）", "持仓收益（%）"},
		{"009239", "融通人工智能指数(LOF)C", "指数型", "3.1095", "2026-05-26", "20.00", "2.00", "11.11%"},
		{"001407", "景顺长城稳健回报混合C", "混合型", "6.8160", "2026-05-26", "20.00", "--", "--"},
		{"持仓收益(元)：0.00", "", "", "", "累计收益(元)：0.00", "", "", ""},
	}
	for r, row := range rows {
		for c, value := range row {
			cell, err := excelize.CoordinatesToCellName(c+1, r+1)
			require.NoError(t, err)
			require.NoError(t, file.SetCellValue(sheet, cell, value))
		}
	}
	require.NoError(t, file.Write(&buf))

	report, err := ParseFundPortfolioTianTianXLSX(bytes.NewReader(buf.Bytes()))

	require.NoError(t, err)
	require.Len(t, report.Drafts, 2)
	first := report.Drafts[0]
	require.Equal(t, "009239", first.Item.Code)
	require.Equal(t, "融通人工智能指数(LOF)C", first.Name)
	require.InDelta(t, 20, first.Item.CurrentAmount, 0.001)
	require.InDelta(t, 20/3.1095, first.Item.HoldingShares, 0.001)
	require.InDelta(t, 18/(20/3.1095), first.Item.CostNav, 0.001)
	require.Contains(t, first.Item.Note, "profit 2.00")

	second := report.Drafts[1]
	require.Equal(t, "001407", second.Item.Code)
	require.InDelta(t, 20, second.Item.CurrentAmount, 0.001)
	require.Zero(t, second.Item.CostNav)
	require.NotEmpty(t, second.Warnings)
}

func TestParseFundPortfolioTianTianXLSXRequiresHoldingRows(t *testing.T) {
	var buf bytes.Buffer
	file := excelize.NewFile()
	require.NoError(t, file.SetCellValue("Sheet1", "A1", "无关内容"))
	require.NoError(t, file.Write(&buf))

	report, err := ParseFundPortfolioTianTianXLSX(bytes.NewReader(buf.Bytes()))

	require.Error(t, err)
	require.Empty(t, report.Drafts)
}
