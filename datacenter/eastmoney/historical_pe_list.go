package eastmoney

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/axiaoxin-com/goutils"
	"github.com/axiaoxin-com/logging"
	"go.uber.org/zap"
)

type RespHistoricalPE struct {
	Result struct {
		Data []struct {
			TradeDate string   `json:"TRADE_DATE"`
			PETTM     *float64 `json:"PE_TTM"`
		} `json:"data"`
	} `json:"result"`
	Success bool   `json:"success"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

type HistoricalPE struct {
	Value float64
	Date  string
}

type HistoricalPEList []HistoricalPE

func (h HistoricalPEList) GetMidValue(ctx context.Context) (float64, error) {
	values := []float64{}
	for _, i := range h {
		values = append(values, i.Value)
	}
	return goutils.MidValueFloat64(values)
}

func (e EastMoney) QueryHistoricalPEList(ctx context.Context, secuCode string) (HistoricalPEList, error) {
	apiurl := "https://datacenter-web.eastmoney.com/api/data/v1/get"
	params := map[string]string{
		"reportName":   "RPT_VALUEANALYSIS_DET",
		"columns":      "ALL",
		"quoteColumns": "",
		"pageNumber":   "1",
		"pageSize":     "5000",
		"sortColumns":  "TRADE_DATE",
		"sortTypes":    "1",
		"source":       "WEB",
		"client":       "WEB",
		"filter":       fmt.Sprintf(`(SECURITY_CODE="%s")`, normalizeSecurityCode(secuCode)),
	}
	logging.Debug(ctx, "EastMoney QueryHistoricalPEList "+apiurl+" begin", zap.Any("params", params))
	beginTime := time.Now()
	apiurl, err := goutils.NewHTTPGetURLWithQueryString(ctx, apiurl, params)
	if err != nil {
		return nil, err
	}
	resp := RespHistoricalPE{}
	err = goutils.HTTPGET(ctx, e.HTTPClient, apiurl, nil, &resp)
	latency := time.Now().Sub(beginTime).Milliseconds()
	logging.Debug(
		ctx,
		"EastMoney QueryHistoricalPEList "+apiurl+" end",
		zap.Int64("latency(ms)", latency),
	)
	if err != nil {
		return nil, err
	}
	return parseHistoricalPEResponse(resp)
}

func parseHistoricalPEResponse(resp RespHistoricalPE) (HistoricalPEList, error) {
	if !resp.Success {
		return nil, fmt.Errorf("historical pe request failed:%d %s", resp.Code, resp.Message)
	}
	if len(resp.Result.Data) == 0 {
		return nil, errors.New("no historical pe data")
	}

	result := HistoricalPEList{}
	for _, i := range resp.Result.Data {
		if i.PETTM == nil {
			continue
		}
		result = append(result, HistoricalPE{
			Date:  strings.Split(i.TradeDate, " ")[0],
			Value: *i.PETTM,
		})
	}
	if len(result) == 0 {
		return nil, errors.New("no historical pe data")
	}
	return result, nil
}
