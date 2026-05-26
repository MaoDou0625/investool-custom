package eastmoney

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseFundNAVHistoryItems(t *testing.T) {
	items := []fundNAVHistoryRawPoint{
		{FSRQ: "2026-05-26", DWJZ: "2.8020"},
		{FSRQ: "2026-05-24", DWJZ: "--"},
		{FSRQ: "bad-date", DWJZ: "2.7000"},
		{FSRQ: "2026-05-25", DWJZ: "2.7040"},
	}

	history := parseFundNAVHistoryItems(items)

	require.Len(t, history, 2)
	require.Equal(t, "2026-05-25", history[0].Date)
	require.InDelta(t, 2.7040, history[0].UnitNAV, 0.00001)
	require.Equal(t, "2026-05-26", history[1].Date)
	require.InDelta(t, 2.8020, history[1].UnitNAV, 0.00001)
}
