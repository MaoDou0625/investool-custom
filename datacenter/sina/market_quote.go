package sina

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/axiaoxin-com/goutils"
	"github.com/axiaoxin-com/logging"
	"github.com/corpix/uarand"
	"go.uber.org/zap"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

const (
	MarketQuoteCategoryCNIndex   = "cn_index"
	MarketQuoteCategoryUSIndex   = "us_index"
	MarketQuoteCategoryHKIndex   = "hk_index"
	MarketQuoteCategoryGlobal    = "global_index"
	MarketQuoteCategoryFX        = "fx"
	MarketQuoteCategoryCommodity = "commodity"
)

// MarketQuote is a normalized quote parsed from Sina hq.sinajs.cn responses.
type MarketQuote struct {
	Symbol        string  `json:"symbol"`
	Name          string  `json:"name"`
	Category      string  `json:"category"`
	Price         float64 `json:"price"`
	ChangePercent float64 `json:"change_percent"`
	ChangeAmount  float64 `json:"change_amount"`
	High          float64 `json:"high,omitempty"`
	Low           float64 `json:"low,omitempty"`
	Turnover      float64 `json:"turnover,omitempty"`
	AsOf          string  `json:"as_of,omitempty"`
}

// QueryMarketQuotes queries Sina real-time quotes for index, FX, and commodity symbols.
func (s Sina) QueryMarketQuotes(ctx context.Context, symbols []string) ([]MarketQuote, error) {
	normalized := compactSinaMarketSymbols(symbols)
	if len(normalized) == 0 {
		return nil, nil
	}
	apiurl := fmt.Sprintf("https://hq.sinajs.cn/rn=%d&list=%s", time.Now().UnixMilli(), strings.Join(normalized, ","))
	logging.Debug(ctx, "Sina QueryMarketQuotes "+apiurl+" begin")
	beginTime := time.Now()
	header := map[string]string{
		"User-Agent": uarand.GetRandom(),
		"Referer":    "https://finance.sina.com.cn/",
	}
	resp, err := goutils.HTTPGETRaw(ctx, s.HTTPClient, apiurl, header)
	latency := time.Since(beginTime).Milliseconds()
	logging.Debug(ctx, "Sina QueryMarketQuotes "+apiurl+" end", zap.Int64("latency(ms)", latency))
	if err != nil {
		return nil, err
	}
	trans := transform.NewReader(bytes.NewReader(resp), simplifiedchinese.GBK.NewDecoder())
	utf8resp, err := ioutil.ReadAll(trans)
	if err != nil {
		return nil, err
	}
	quotes := []MarketQuote{}
	for _, line := range strings.Split(string(utf8resp), "\n") {
		quote, ok := parseSinaMarketQuoteLine(line)
		if ok {
			quotes = append(quotes, quote)
		}
	}
	return quotes, nil
}

func parseSinaMarketQuoteLine(line string) (MarketQuote, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "var hq_str_") {
		return MarketQuote{}, false
	}
	eq := strings.Index(line, "=")
	if eq <= len("var hq_str_") {
		return MarketQuote{}, false
	}
	symbol := strings.TrimSpace(strings.TrimPrefix(line[:eq], "var hq_str_"))
	payload := strings.TrimSpace(line[eq+1:])
	payload = strings.TrimSuffix(payload, ";")
	payload = strings.Trim(payload, "\"")
	if symbol == "" || payload == "" {
		return MarketQuote{}, false
	}
	fields := strings.Split(payload, ",")
	switch {
	case strings.HasPrefix(symbol, "sh") || strings.HasPrefix(symbol, "sz"):
		return parseSinaCNIndexQuote(symbol, fields)
	case strings.HasPrefix(symbol, "gb_"):
		return parseSinaUSIndexQuote(symbol, fields)
	case strings.HasPrefix(symbol, "hk"):
		return parseSinaHKIndexQuote(symbol, fields)
	case strings.HasPrefix(symbol, "b_"):
		return parseSinaGlobalIndexQuote(symbol, fields)
	case strings.HasPrefix(symbol, "fx_"):
		return parseSinaFXQuote(symbol, fields)
	case strings.HasPrefix(symbol, "hf_"):
		return parseSinaCommodityQuote(symbol, fields)
	case symbol == "DINIW" || symbol == "DINIW2":
		return parseSinaDollarIndexQuote(symbol, fields)
	default:
		return MarketQuote{}, false
	}
}

func parseSinaCNIndexQuote(symbol string, fields []string) (MarketQuote, bool) {
	if len(fields) < 6 {
		return MarketQuote{}, false
	}
	price := parseSinaFloat(fields[3])
	prevClose := parseSinaFloat(fields[2])
	change := price - prevClose
	asOf := joinSinaDateTime(fields, 30, 31)
	return MarketQuote{
		Symbol:        symbol,
		Name:          fields[0],
		Category:      MarketQuoteCategoryCNIndex,
		Price:         price,
		ChangeAmount:  roundSinaFloat(change),
		ChangePercent: percentChange(price, prevClose),
		High:          parseSinaFloat(fields[4]),
		Low:           parseSinaFloat(fields[5]),
		Turnover:      parseSinaFloatAt(fields, 9),
		AsOf:          asOf,
	}, strings.TrimSpace(fields[0]) != ""
}

func parseSinaUSIndexQuote(symbol string, fields []string) (MarketQuote, bool) {
	if len(fields) < 5 {
		return MarketQuote{}, false
	}
	return MarketQuote{
		Symbol:        symbol,
		Name:          fields[0],
		Category:      MarketQuoteCategoryUSIndex,
		Price:         parseSinaFloat(fields[1]),
		ChangePercent: parseSinaFloat(fields[2]),
		ChangeAmount:  parseSinaFloat(fields[4]),
		High:          parseSinaFloatAt(fields, 6),
		Low:           parseSinaFloatAt(fields, 7),
		AsOf:          strings.TrimSpace(fields[3]),
	}, strings.TrimSpace(fields[0]) != ""
}

func parseSinaHKIndexQuote(symbol string, fields []string) (MarketQuote, bool) {
	if len(fields) < 9 {
		return MarketQuote{}, false
	}
	return MarketQuote{
		Symbol:        symbol,
		Name:          fields[1],
		Category:      MarketQuoteCategoryHKIndex,
		Price:         parseSinaFloat(fields[2]),
		ChangeAmount:  parseSinaFloat(fields[7]),
		ChangePercent: parseSinaFloat(fields[8]),
		High:          parseSinaFloatAt(fields, 3),
		Low:           parseSinaFloatAt(fields, 5),
		Turnover:      parseSinaFloatAt(fields, 11),
		AsOf:          joinSinaDateTime(fields, 17, 18),
	}, strings.TrimSpace(fields[1]) != ""
}

func parseSinaGlobalIndexQuote(symbol string, fields []string) (MarketQuote, bool) {
	if len(fields) < 4 {
		return MarketQuote{}, false
	}
	return MarketQuote{
		Symbol:        symbol,
		Name:          fields[0],
		Category:      MarketQuoteCategoryGlobal,
		Price:         parseSinaFloat(fields[1]),
		ChangeAmount:  parseSinaFloat(fields[2]),
		ChangePercent: parseSinaFloat(fields[3]),
		High:          parseSinaFloatAt(fields, 10),
		Low:           parseSinaFloatAt(fields, 11),
		Turnover:      parseSinaFloatAt(fields, 12),
		AsOf:          joinSinaDateTime(fields, 6, 7),
	}, strings.TrimSpace(fields[0]) != ""
}

func parseSinaFXQuote(symbol string, fields []string) (MarketQuote, bool) {
	if len(fields) < 12 {
		return MarketQuote{}, false
	}
	return MarketQuote{
		Symbol:        symbol,
		Name:          fields[9],
		Category:      MarketQuoteCategoryFX,
		Price:         parseSinaFloat(fields[1]),
		ChangePercent: parseSinaFloat(fields[10]),
		ChangeAmount:  parseSinaFloat(fields[11]),
		High:          parseSinaFloatAt(fields, 6),
		Low:           parseSinaFloatAt(fields, 7),
		AsOf:          joinSinaDateTime(fields, 17, 0),
	}, strings.TrimSpace(fields[9]) != ""
}

func parseSinaCommodityQuote(symbol string, fields []string) (MarketQuote, bool) {
	if len(fields) < 14 {
		return MarketQuote{}, false
	}
	price := parseSinaFloat(fields[0])
	prev := parseSinaFloat(fields[7])
	return MarketQuote{
		Symbol:        symbol,
		Name:          fields[13],
		Category:      MarketQuoteCategoryCommodity,
		Price:         price,
		ChangeAmount:  roundSinaFloat(price - prev),
		ChangePercent: percentChange(price, prev),
		High:          parseSinaFloatAt(fields, 4),
		Low:           parseSinaFloatAt(fields, 5),
		AsOf:          joinSinaDateTime(fields, 12, 6),
	}, strings.TrimSpace(fields[13]) != ""
}

func parseSinaDollarIndexQuote(symbol string, fields []string) (MarketQuote, bool) {
	if len(fields) < 10 {
		return MarketQuote{}, false
	}
	return MarketQuote{
		Symbol:   symbol,
		Name:     fields[9],
		Category: MarketQuoteCategoryFX,
		Price:    parseSinaFloat(fields[1]),
		High:     parseSinaFloatAt(fields, 6),
		Low:      parseSinaFloatAt(fields, 7),
		AsOf:     joinSinaDateTime(fields, 10, 0),
	}, strings.TrimSpace(fields[9]) != ""
}

func compactSinaMarketSymbols(symbols []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, symbol := range symbols {
		symbol = strings.TrimSpace(symbol)
		if symbol == "" || seen[symbol] {
			continue
		}
		seen[symbol] = true
		result = append(result, symbol)
	}
	return result
}

func parseSinaFloatAt(fields []string, index int) float64 {
	if index < 0 || index >= len(fields) {
		return 0
	}
	return parseSinaFloat(fields[index])
}

func parseSinaFloat(value string) float64 {
	value = strings.TrimSpace(value)
	if value == "" || value == "--" {
		return 0
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return 0
	}
	return parsed
}

func percentChange(price float64, previous float64) float64 {
	if previous == 0 {
		return 0
	}
	return roundSinaFloat((price - previous) / previous * 100)
}

func roundSinaFloat(value float64) float64 {
	return math.Round(value*10000) / 10000
}

func joinSinaDateTime(fields []string, dateIndex int, timeIndex int) string {
	if dateIndex < 0 || dateIndex >= len(fields) {
		return ""
	}
	date := strings.TrimSpace(fields[dateIndex])
	if timeIndex < 0 || timeIndex >= len(fields) {
		return date
	}
	tm := strings.TrimSpace(fields[timeIndex])
	if date == "" {
		return tm
	}
	if tm == "" {
		return date
	}
	return date + " " + tm
}
