package yahoo

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"time"

	"github.com/axiaoxin-com/goutils"
	"github.com/axiaoxin-com/logging"
	"github.com/corpix/uarand"
	"go.uber.org/zap"
)

const (
	MarketQuoteCategoryVolatility = "volatility"
	MarketQuoteCategoryRate       = "rate"
	MarketQuoteCategoryBondETF    = "bond_etf"
)

// MarketQuote is a normalized Yahoo Finance chart quote.
type MarketQuote struct {
	Symbol        string  `json:"symbol"`
	Name          string  `json:"name"`
	Category      string  `json:"category"`
	Price         float64 `json:"price"`
	ChangePercent float64 `json:"change_percent"`
	ChangeAmount  float64 `json:"change_amount"`
	PreviousClose float64 `json:"previous_close"`
	AsOf          string  `json:"as_of,omitempty"`
}

type respYahooChart struct {
	Chart struct {
		Result []struct {
			Meta struct {
				Symbol             string  `json:"symbol"`
				RegularMarketPrice float64 `json:"regularMarketPrice"`
				RegularMarketTime  int64   `json:"regularMarketTime"`
			} `json:"meta"`
			Timestamp  []int64 `json:"timestamp"`
			Indicators struct {
				Quote []struct {
					Close []*float64 `json:"close"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
		Error interface{} `json:"error"`
	} `json:"chart"`
}

// QueryMarketQuotes queries Yahoo Finance chart quotes by symbol.
func (y Yahoo) QueryMarketQuotes(ctx context.Context, symbols []string) ([]MarketQuote, error) {
	result := []MarketQuote{}
	for _, symbol := range compactYahooMarketSymbols(symbols) {
		quote, err := y.QueryMarketQuote(ctx, symbol)
		if err != nil {
			return result, err
		}
		result = append(result, quote)
	}
	return result, nil
}

func (y Yahoo) QueryMarketQuote(ctx context.Context, symbol string) (MarketQuote, error) {
	apiurl := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?range=5d&interval=1d", url.PathEscape(symbol))
	logging.Debug(ctx, "Yahoo QueryMarketQuote "+apiurl+" begin")
	beginTime := time.Now()
	header := map[string]string{"User-Agent": uarand.GetRandom()}
	resp := respYahooChart{}
	if err := goutils.HTTPGET(ctx, y.HTTPClient, apiurl, header, &resp); err != nil {
		return MarketQuote{}, err
	}
	logging.Debug(ctx, "Yahoo QueryMarketQuote "+apiurl+" end", zap.Int64("latency(ms)", time.Since(beginTime).Milliseconds()))
	return parseYahooChartQuote(resp, symbol)
}

func parseYahooChartQuote(resp respYahooChart, requestedSymbol string) (MarketQuote, error) {
	if resp.Chart.Error != nil {
		return MarketQuote{}, fmt.Errorf("yahoo chart error: %v", resp.Chart.Error)
	}
	if len(resp.Chart.Result) == 0 {
		return MarketQuote{}, fmt.Errorf("yahoo chart result empty: %s", requestedSymbol)
	}
	item := resp.Chart.Result[0]
	if len(item.Indicators.Quote) == 0 {
		return MarketQuote{}, fmt.Errorf("yahoo chart quote empty: %s", requestedSymbol)
	}
	latestClose, previousClose := lastTwoYahooCloses(item.Indicators.Quote[0].Close)
	price := item.Meta.RegularMarketPrice
	if price <= 0 {
		price = latestClose
	}
	change := roundYahooFloat(price - previousClose)
	return MarketQuote{
		Symbol:        firstNonEmptyYahoo(item.Meta.Symbol, requestedSymbol),
		Name:          yahooMarketQuoteName(firstNonEmptyYahoo(item.Meta.Symbol, requestedSymbol)),
		Category:      yahooMarketQuoteCategory(firstNonEmptyYahoo(item.Meta.Symbol, requestedSymbol)),
		Price:         price,
		ChangeAmount:  change,
		ChangePercent: yahooPercentChange(price, previousClose),
		PreviousClose: previousClose,
		AsOf:          yahooQuoteAsOf(item.Meta.RegularMarketTime, item.Timestamp),
	}, nil
}

func lastTwoYahooCloses(closes []*float64) (float64, float64) {
	values := []float64{}
	for _, closeValue := range closes {
		if closeValue == nil || math.IsNaN(*closeValue) || math.IsInf(*closeValue, 0) {
			continue
		}
		values = append(values, *closeValue)
	}
	if len(values) == 0 {
		return 0, 0
	}
	if len(values) == 1 {
		return values[0], values[0]
	}
	return values[len(values)-1], values[len(values)-2]
}

func yahooMarketQuoteName(symbol string) string {
	switch symbol {
	case "^VIX":
		return "VIX恐慌指数"
	case "^TNX":
		return "美国10年期国债收益率"
	case "^TYX":
		return "美国30年期国债收益率"
	case "TLT":
		return "20年期以上美债ETF"
	default:
		return symbol
	}
}

func yahooMarketQuoteCategory(symbol string) string {
	switch symbol {
	case "^VIX":
		return MarketQuoteCategoryVolatility
	case "^TNX", "^TYX":
		return MarketQuoteCategoryRate
	case "TLT":
		return MarketQuoteCategoryBondETF
	default:
		return ""
	}
}

func yahooQuoteAsOf(regularMarketTime int64, timestamps []int64) string {
	if regularMarketTime > 0 {
		return time.Unix(regularMarketTime, 0).UTC().Format("2006-01-02 15:04:05 UTC")
	}
	if len(timestamps) > 0 && timestamps[len(timestamps)-1] > 0 {
		return time.Unix(timestamps[len(timestamps)-1], 0).UTC().Format("2006-01-02 15:04:05 UTC")
	}
	return ""
}

func yahooPercentChange(price float64, previous float64) float64 {
	if previous == 0 {
		return 0
	}
	return roundYahooFloat((price - previous) / previous * 100)
}

func roundYahooFloat(value float64) float64 {
	return math.Round(value*10000) / 10000
}

func compactYahooMarketSymbols(symbols []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, symbol := range symbols {
		if symbol == "" || seen[symbol] {
			continue
		}
		seen[symbol] = true
		result = append(result, symbol)
	}
	return result
}

func firstNonEmptyYahoo(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
