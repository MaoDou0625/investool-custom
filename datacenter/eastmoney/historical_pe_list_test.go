package eastmoney

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPEGetMidValue(t *testing.T) {
	d := HistoricalPEList{
		HistoricalPE{Date: "1", Value: 6.0},
		HistoricalPE{Date: "1", Value: 1.0},
		HistoricalPE{Date: "1", Value: 5.0},
		HistoricalPE{Date: "1", Value: 2.0},
		HistoricalPE{Date: "1", Value: 4.0},
		HistoricalPE{Date: "1", Value: 3.0},
	}
	m, err := d.GetMidValue(_ctx)
	require.Nil(t, err)
	require.Equal(t, 3.5, m)
}

func TestParseHistoricalPEResponseSkipsMissingPE(t *testing.T) {
	pe := 12.34
	resp := RespHistoricalPE{Success: true}
	resp.Result.Data = append(resp.Result.Data, struct {
		TradeDate string   `json:"TRADE_DATE"`
		PETTM     *float64 `json:"PE_TTM"`
	}{
		TradeDate: "2026-05-25 00:00:00",
		PETTM:     nil,
	})
	resp.Result.Data = append(resp.Result.Data, struct {
		TradeDate string   `json:"TRADE_DATE"`
		PETTM     *float64 `json:"PE_TTM"`
	}{
		TradeDate: "2026-05-26 00:00:00",
		PETTM:     &pe,
	})

	d, err := parseHistoricalPEResponse(resp)
	require.Nil(t, err)
	require.Equal(t, HistoricalPEList{{Date: "2026-05-26", Value: 12.34}}, d)
}

func TestQueryHistoricalPEList(t *testing.T) {
	d, err := _em.QueryHistoricalPEList(_ctx, "600149.sh")
	require.Nil(t, err)
	t.Log(d)
}
