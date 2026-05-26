package core

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseFundPortfolioScreenshotTextAlipayOCR(t *testing.T) {
	raw := `〈 夼 产 详 情 前 海 开 源 金 银 珠 宝 混 合 C 002207 中 高 风 险 金 额 （ 元 ） 1 ， 256 · 30 8 弓 详 情 持 有 益 率 O 一 5 ． 58 ％ 昨 益 （ 元 ） O + 1 3 ． 26 持 有 奎 额 1 ， 246 ． 30 持 仓 成 本 价 2 ． 8637 刂 栖 + 1 · 08 ％ 持 有 益 （ 元 ） O 一 73 ． 58 待 确 认 金 额 10 ． 00 持 有 份 额 460 ． 91 基 奎 净 位 （ 05 一 25 ） 2 ． 7040 益 明 细`

	draft, err := ParseFundPortfolioScreenshotText(raw)
	require.NoError(t, err)
	require.Equal(t, "002207", draft.Item.Code)
	require.Equal(t, "owned", draft.Item.Status)
	require.InDelta(t, 1246.30, draft.Item.CurrentAmount, 0.001)
	require.InDelta(t, 2.8637, draft.Item.CostNav, 0.00001)
	require.InDelta(t, 460.91, draft.Item.HoldingShares, 0.001)
	require.InDelta(t, 1256.30, draft.TotalAmount, 0.001)
	require.InDelta(t, 10.00, draft.PendingAmount, 0.001)
	require.InDelta(t, 2.7040, draft.UnitNAV, 0.00001)
	require.Equal(t, "05-25", draft.NAVDate)
	require.Contains(t, draft.Item.Note, "pending 10.00")
	require.Contains(t, draft.Item.Note, "NAV 2.7040 on 05-25")
}

func TestParseFundPortfolioScreenshotTextFallbackTotalMinusPending(t *testing.T) {
	raw := `资产详情 测试基金C 123456 金额（元） 1,256.30 待确认金额 10.00 持仓成本价 2.8637 持有份额 460.91`

	draft, err := ParseFundPortfolioScreenshotText(raw)
	require.NoError(t, err)
	require.Equal(t, "123456", draft.Item.Code)
	require.InDelta(t, 1246.30, draft.Item.CurrentAmount, 0.001)
	require.NotEmpty(t, draft.Warnings)
}
