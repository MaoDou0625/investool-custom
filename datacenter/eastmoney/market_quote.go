package eastmoney

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/axiaoxin-com/goutils"
	"github.com/axiaoxin-com/logging"
	"github.com/corpix/uarand"
	"go.uber.org/zap"
)

// MarketQuote is a normalized quote from EastMoney push2 APIs.
type MarketQuote struct {
	SecID         string  `json:"secid"`
	Code          string  `json:"code"`
	Market        int     `json:"market"`
	Name          string  `json:"name"`
	Price         float64 `json:"price"`
	ChangePercent float64 `json:"change_percent"`
	ChangeAmount  float64 `json:"change_amount"`
	Turnover      float64 `json:"turnover"`
	High          float64 `json:"high"`
	Low           float64 `json:"low"`
	Open          float64 `json:"open"`
	PreviousClose float64 `json:"previous_close"`
}

type respMarketQuoteList struct {
	RC   int `json:"rc"`
	Data struct {
		Total int `json:"total"`
		Diff  []struct {
			Price         float64 `json:"f2"`
			ChangePercent float64 `json:"f3"`
			ChangeAmount  float64 `json:"f4"`
			Turnover      float64 `json:"f6"`
			Code          string  `json:"f12"`
			Market        int     `json:"f13"`
			Name          string  `json:"f14"`
			High          float64 `json:"f15"`
			Low           float64 `json:"f16"`
			Open          float64 `json:"f17"`
			PreviousClose float64 `json:"f18"`
		} `json:"diff"`
	} `json:"data"`
}

// QueryMarketQuotes queries index or asset quotes by EastMoney secids.
func (e EastMoney) QueryMarketQuotes(ctx context.Context, secids []string) ([]MarketQuote, error) {
	normalized := compactMarketSecIDs(secids)
	if len(normalized) == 0 {
		return nil, nil
	}
	apiurl := "https://push2.eastmoney.com/api/qt/ulist.np/get"
	params := map[string]string{
		"fltt":   "2",
		"invt":   "2",
		"fields": "f12,f13,f14,f2,f3,f4,f6,f15,f16,f17,f18",
		"secids": strings.Join(normalized, ","),
	}
	logging.Debug(ctx, "EastMoney QueryMarketQuotes "+apiurl+" begin", zap.Any("params", params))
	beginTime := time.Now()
	apiurl, err := goutils.NewHTTPGetURLWithQueryString(ctx, apiurl, params)
	if err != nil {
		return nil, err
	}
	header := map[string]string{"user-agent": uarand.GetRandom()}
	resp := respMarketQuoteList{}
	if err := goutils.HTTPGET(ctx, e.HTTPClient, apiurl, header, &resp); err != nil {
		return nil, err
	}
	logging.Debug(ctx, "EastMoney QueryMarketQuotes "+apiurl+" end", zap.Int64("latency(ms)", time.Since(beginTime).Milliseconds()))
	if resp.RC != 0 {
		return nil, fmt.Errorf("QueryMarketQuotes rc=%d", resp.RC)
	}
	quotes := make([]MarketQuote, 0, len(resp.Data.Diff))
	for _, item := range resp.Data.Diff {
		quotes = append(quotes, MarketQuote{
			SecID:         fmt.Sprintf("%d.%s", item.Market, item.Code),
			Code:          item.Code,
			Market:        item.Market,
			Name:          item.Name,
			Price:         item.Price,
			ChangePercent: item.ChangePercent,
			ChangeAmount:  item.ChangeAmount,
			Turnover:      item.Turnover,
			High:          item.High,
			Low:           item.Low,
			Open:          item.Open,
			PreviousClose: item.PreviousClose,
		})
	}
	return quotes, nil
}

// QueryIndustryBoardMovers queries EastMoney industry board movers.
func (e EastMoney) QueryIndustryBoardMovers(ctx context.Context, limit int, ascending bool) ([]MarketQuote, error) {
	if limit <= 0 {
		limit = 10
	}
	order := "1"
	if ascending {
		order = "0"
	}
	apiurl := "https://push2.eastmoney.com/api/qt/clist/get"
	params := map[string]string{
		"pn":     "1",
		"pz":     fmt.Sprintf("%d", limit),
		"po":     order,
		"np":     "1",
		"fltt":   "2",
		"invt":   "2",
		"fid":    "f3",
		"fs":     "m:90+t:2",
		"fields": "f12,f14,f2,f3,f4,f6",
	}
	logging.Debug(ctx, "EastMoney QueryIndustryBoardMovers "+apiurl+" begin", zap.Any("params", params))
	beginTime := time.Now()
	apiurl, err := goutils.NewHTTPGetURLWithQueryString(ctx, apiurl, params)
	if err != nil {
		return nil, err
	}
	header := map[string]string{"user-agent": uarand.GetRandom()}
	resp := respMarketQuoteList{}
	if err := goutils.HTTPGET(ctx, e.HTTPClient, apiurl, header, &resp); err != nil {
		return nil, err
	}
	logging.Debug(ctx, "EastMoney QueryIndustryBoardMovers "+apiurl+" end", zap.Int64("latency(ms)", time.Since(beginTime).Milliseconds()))
	if resp.RC != 0 {
		return nil, fmt.Errorf("QueryIndustryBoardMovers rc=%d", resp.RC)
	}
	quotes := make([]MarketQuote, 0, len(resp.Data.Diff))
	for _, item := range resp.Data.Diff {
		quotes = append(quotes, MarketQuote{
			Code:          item.Code,
			Name:          item.Name,
			Price:         item.Price,
			ChangePercent: item.ChangePercent,
			ChangeAmount:  item.ChangeAmount,
			Turnover:      item.Turnover,
		})
	}
	return quotes, nil
}

func compactMarketSecIDs(secids []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, secid := range secids {
		secid = strings.TrimSpace(secid)
		if secid == "" || seen[secid] {
			continue
		}
		seen[secid] = true
		result = append(result, secid)
	}
	return result
}
