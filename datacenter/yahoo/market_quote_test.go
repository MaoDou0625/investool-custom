package yahoo

import "testing"

func TestParseYahooChartQuote(t *testing.T) {
	prev := 16.05
	latest := 19.45
	resp := respYahooChart{}
	resp.Chart.Result = append(resp.Chart.Result, struct {
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
	}{})
	item := &resp.Chart.Result[0]
	item.Meta.Symbol = "^VIX"
	item.Meta.RegularMarketPrice = 19.46
	item.Meta.RegularMarketTime = 1780916866
	item.Indicators.Quote = append(item.Indicators.Quote, struct {
		Close []*float64 `json:"close"`
	}{Close: []*float64{&prev, nil, &latest}})

	quote, err := parseYahooChartQuote(resp, "^VIX")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if quote.Name != "VIX恐慌指数" || quote.Category != MarketQuoteCategoryVolatility {
		t.Fatalf("unexpected quote metadata: %+v", quote)
	}
	if quote.Price != 19.46 || quote.PreviousClose != prev {
		t.Fatalf("unexpected price fields: %+v", quote)
	}
	if quote.ChangePercent <= 0 || quote.ChangeAmount <= 0 {
		t.Fatalf("expected positive change: %+v", quote)
	}
}
