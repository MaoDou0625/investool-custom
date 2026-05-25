package eastmoney

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildLatestPCMainForms(t *testing.T) {
	income := 49128712950.94
	ratio := 0.916593
	oldIncome := 100000000.0
	oldRatio := 1.0

	forms := buildLatestPCMainForms([]pcBusinessMainForm{
		{
			ReportDate:         "2024-12-31 00:00:00",
			MainopType:         "1",
			ItemName:           "old",
			MainBusinessIncome: &oldIncome,
			MBIRatio:           &oldRatio,
		},
		{
			ReportDate:         "2025-12-31 00:00:00",
			MainopType:         "2",
			ItemName:           "solar module",
			MainBusinessIncome: &income,
			MBIRatio:           &ratio,
		},
	})

	require.Len(t, forms, 1)
	require.Equal(t, "3", forms[0].Type)
	require.Equal(t, "solar module", forms[0].MainForm)
	require.Equal(t, "491.29亿", forms[0].MainIncome)
	require.Equal(t, "91.66%", forms[0].MainIncomeRatio)
}

func TestFormatPCSecuCode(t *testing.T) {
	require.Equal(t, "SH600149", formatPCSecuCode("600149.sh"))
	require.Equal(t, "SZ002459", formatPCSecuCode("002459.sz"))
	require.Equal(t, "SH600000", formatPCSecuCode("600000"))
	require.Equal(t, "SZ300750", formatPCSecuCode("300750"))
}
