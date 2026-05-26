// Tiantian Fund historical NAV.
package eastmoney

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/axiaoxin-com/goutils"
	"github.com/axiaoxin-com/logging"
	"github.com/corpix/uarand"
	"go.uber.org/zap"
)

const (
	defaultFundNAVHistoryPageSize = 90
	maxFundNAVHistoryPageSize     = 366
)

// FundNAVHistoryPoint is one historical unit-NAV observation.
type FundNAVHistoryPoint struct {
	Date    string  `json:"date"`
	UnitNAV float64 `json:"unit_nav"`
}

// FundNAVHistory is sorted oldest-to-newest by Date.
type FundNAVHistory []FundNAVHistoryPoint

type respFundNAVHistory struct {
	Data struct {
		LSJZList []fundNAVHistoryRawPoint `json:"LSJZList"`
	} `json:"Data"`
	ErrCode int    `json:"ErrCode"`
	ErrMsg  string `json:"ErrMsg"`
}

type fundNAVHistoryRawPoint struct {
	FSRQ string `json:"FSRQ"`
	DWJZ string `json:"DWJZ"`
}

// QueryFundNAVHistory queries recent fund unit-NAV history from Tiantian Fund.
func (e EastMoney) QueryFundNAVHistory(ctx context.Context, fundCode string, pageSize int) (FundNAVHistory, error) {
	fundCode = strings.TrimSpace(fundCode)
	if fundCode == "" {
		return nil, fmt.Errorf("fund code is empty")
	}
	if pageSize <= 0 {
		pageSize = defaultFundNAVHistoryPageSize
	}
	if pageSize > maxFundNAVHistoryPageSize {
		pageSize = maxFundNAVHistoryPageSize
	}

	apiurl := "https://api.fund.eastmoney.com/f10/lsjz"
	params := map[string]string{
		"fundCode":  fundCode,
		"pageIndex": "1",
		"pageSize":  strconv.Itoa(pageSize),
	}
	logging.Debug(ctx, "EastMoney QueryFundNAVHistory "+apiurl+" begin", zap.Any("params", params))
	beginTime := time.Now()
	apiurl, err := goutils.NewHTTPGetURLWithQueryString(ctx, apiurl, params)
	if err != nil {
		return nil, err
	}

	header := map[string]string{
		"Accept":     "application/json, text/plain, */*",
		"Referer":    "https://fundf10.eastmoney.com/",
		"user-agent": uarand.GetRandom(),
	}
	resp := respFundNAVHistory{}
	err = goutils.HTTPGET(ctx, e.HTTPClient, apiurl, header, &resp)
	latency := time.Now().Sub(beginTime).Milliseconds()
	logging.Debug(ctx, "EastMoney QueryFundNAVHistory "+apiurl+" end", zap.Int64("latency(ms)", latency))
	if err != nil {
		return nil, err
	}
	if resp.ErrCode != 0 {
		if resp.ErrMsg != "" {
			return nil, fmt.Errorf("fund nav history api failed: %s", resp.ErrMsg)
		}
		return nil, fmt.Errorf("fund nav history api failed: errcode %d", resp.ErrCode)
	}
	return parseFundNAVHistoryItems(resp.Data.LSJZList), nil
}

func parseFundNAVHistoryItems(items []fundNAVHistoryRawPoint) FundNAVHistory {
	history := make(FundNAVHistory, 0, len(items))
	for _, item := range items {
		date := strings.TrimSpace(item.FSRQ)
		if _, err := time.Parse("2006-01-02", date); err != nil {
			continue
		}

		unitNAV, err := strconv.ParseFloat(strings.TrimSpace(item.DWJZ), 64)
		if err != nil || unitNAV <= 0 {
			continue
		}
		history = append(history, FundNAVHistoryPoint{
			Date:    date,
			UnitNAV: unitNAV,
		})
	}
	sort.SliceStable(history, func(i, j int) bool {
		return history[i].Date < history[j].Date
	})
	return history
}
