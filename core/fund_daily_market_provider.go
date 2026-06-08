package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/axiaoxin-com/investool/datacenter/eastmoney"
	"github.com/axiaoxin-com/investool/datacenter/sina"
	"github.com/axiaoxin-com/investool/datacenter/yahoo"
)

const (
	fundDailyMarketProvider         = "sina-eastmoney-market-context"
	fundDailyMarketIndustryItemSize = 10
	fundDailyMarketQuoteSourceSina  = "sina"
	fundDailyMarketQuoteSourceEM    = "eastmoney"
)

var fundDailyMarketIndexSecIDs = []string{
	"1.000001",
	"0.399001",
	"0.399006",
}

var fundDailyMarketSinaIndexSymbols = []string{
	"sh000001",
	"sz399001",
	"sz399006",
	"gb_dji",
	"gb_inx",
	"gb_ixic",
	"gb_ndx",
	"hkHSI",
	"hkHSTECH",
	"hkHSCEI",
	"b_NKY",
	"b_DAX",
	"b_FTSE",
	"b_CAC",
}

var fundDailyMarketSinaCrossAssetSymbols = []string{
	"fx_susdcnh",
	"fx_susdcny",
	"fx_seurusd",
	"fx_susdjpy",
	"fx_sgbpusd",
	"fx_saudusd",
	"fx_susdhkd",
	"DINIW",
	"hf_XAU",
	"hf_GC",
	"hf_SI",
	"hf_CL",
	"hf_OIL",
	"hf_HG",
	"hf_CAD",
	"hf_NID",
}

var fundDailyMarketYahooRiskSymbols = []string{
	"^VIX",
	"^TNX",
	"^TYX",
	"TLT",
}

type fundDailyMarketQuoteResult struct {
	Quotes   []FundDailyMarketQuote
	Warnings []string
}

type fundDailyMarketDataProvider interface {
	QueryIndexQuotes(ctx context.Context) fundDailyMarketQuoteResult
	QueryIndustryGainers(ctx context.Context, limit int) fundDailyMarketQuoteResult
	QueryIndustryLosers(ctx context.Context, limit int) fundDailyMarketQuoteResult
	QueryCrossAssetQuotes(ctx context.Context) fundDailyMarketQuoteResult
	QueryRiskAssetQuotes(ctx context.Context) fundDailyMarketQuoteResult
}

type liveFundDailyMarketDataProvider struct {
	eastMoney eastmoney.EastMoney
	sina      sina.Sina
	yahoo     yahoo.Yahoo
}

func newLiveFundDailyMarketDataProvider() liveFundDailyMarketDataProvider {
	return liveFundDailyMarketDataProvider{
		eastMoney: eastmoney.NewEastMoney(),
		sina:      sina.NewSina(),
		yahoo:     yahoo.NewYahoo(),
	}
}

func (p liveFundDailyMarketDataProvider) QueryIndexQuotes(ctx context.Context) fundDailyMarketQuoteResult {
	sinaQuotes, err := p.sina.QueryMarketQuotes(ctx, fundDailyMarketSinaIndexSymbols)
	if err == nil && len(sinaQuotes) > 0 {
		return fundDailyMarketQuoteResult{Quotes: normalizeFundDailySinaMarketQuotes(sinaQuotes)}
	}
	result := fundDailyMarketQuoteResult{}
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Sina主要指数行情读取失败：%s", compactFundDailyMarketError(err)))
	} else {
		result.Warnings = append(result.Warnings, "Sina主要指数行情为空")
	}

	eastMoneyQuotes, err := p.eastMoney.QueryMarketQuotes(ctx, fundDailyMarketIndexSecIDs)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("东方财富主要指数行情读取失败：%s", compactFundDailyMarketError(err)))
		return result
	}
	result.Quotes = normalizeFundDailyEastMoneyMarketQuotes(eastMoneyQuotes, "cn_index")
	return result
}

func (p liveFundDailyMarketDataProvider) QueryIndustryGainers(ctx context.Context, limit int) fundDailyMarketQuoteResult {
	quotes, err := p.eastMoney.QueryIndustryBoardMovers(ctx, limit, false)
	if err != nil {
		return fundDailyMarketQuoteResult{Warnings: []string{fmt.Sprintf("行业上涨榜读取失败：%s", compactFundDailyMarketError(err))}}
	}
	return fundDailyMarketQuoteResult{Quotes: normalizeFundDailyEastMoneyMarketQuotes(quotes, "industry")}
}

func (p liveFundDailyMarketDataProvider) QueryIndustryLosers(ctx context.Context, limit int) fundDailyMarketQuoteResult {
	quotes, err := p.eastMoney.QueryIndustryBoardMovers(ctx, limit, true)
	if err != nil {
		return fundDailyMarketQuoteResult{Warnings: []string{fmt.Sprintf("行业下跌榜读取失败：%s", compactFundDailyMarketError(err))}}
	}
	return fundDailyMarketQuoteResult{Quotes: normalizeFundDailyEastMoneyMarketQuotes(quotes, "industry")}
}

func (p liveFundDailyMarketDataProvider) QueryCrossAssetQuotes(ctx context.Context) fundDailyMarketQuoteResult {
	quotes, err := p.sina.QueryMarketQuotes(ctx, fundDailyMarketSinaCrossAssetSymbols)
	if err != nil {
		return fundDailyMarketQuoteResult{Warnings: []string{fmt.Sprintf("跨资产行情读取失败：%s", compactFundDailyMarketError(err))}}
	}
	return fundDailyMarketQuoteResult{Quotes: normalizeFundDailySinaMarketQuotes(quotes)}
}

func (p liveFundDailyMarketDataProvider) QueryRiskAssetQuotes(ctx context.Context) fundDailyMarketQuoteResult {
	quotes, err := p.yahoo.QueryMarketQuotes(ctx, fundDailyMarketYahooRiskSymbols)
	if err != nil {
		return fundDailyMarketQuoteResult{Warnings: []string{fmt.Sprintf("波动率/利率行情读取失败：%s", compactFundDailyMarketError(err))}}
	}
	return fundDailyMarketQuoteResult{Quotes: normalizeFundDailyYahooMarketQuotes(quotes)}
}

func compactFundDailyMarketError(err error) string {
	if err == nil {
		return ""
	}
	text := err.Error()
	switch {
	case strings.Contains(text, "502 Bad Gateway"):
		return "HTTP 502 Bad Gateway"
	case strings.Contains(text, "timeout"):
		return "请求超时"
	case len(text) > 180:
		return text[:180] + "..."
	default:
		return text
	}
}

func normalizeFundDailyEastMoneyMarketQuotes(quotes []eastmoney.MarketQuote, category string) []FundDailyMarketQuote {
	result := make([]FundDailyMarketQuote, 0, len(quotes))
	for _, quote := range quotes {
		if strings.TrimSpace(quote.Code) == "" && strings.TrimSpace(quote.Name) == "" {
			continue
		}
		result = append(result, FundDailyMarketQuote{
			Code:          quote.Code,
			Name:          quote.Name,
			Category:      category,
			Source:        fundDailyMarketQuoteSourceEM,
			Price:         quote.Price,
			ChangeAmount:  quote.ChangeAmount,
			ChangePercent: quote.ChangePercent,
			Turnover:      quote.Turnover,
		})
	}
	return result
}

func normalizeFundDailySinaMarketQuotes(quotes []sina.MarketQuote) []FundDailyMarketQuote {
	result := make([]FundDailyMarketQuote, 0, len(quotes))
	for _, quote := range quotes {
		if strings.TrimSpace(quote.Symbol) == "" && strings.TrimSpace(quote.Name) == "" {
			continue
		}
		result = append(result, FundDailyMarketQuote{
			Code:          quote.Symbol,
			Name:          quote.Name,
			Category:      quote.Category,
			Source:        fundDailyMarketQuoteSourceSina,
			Price:         quote.Price,
			ChangeAmount:  quote.ChangeAmount,
			ChangePercent: quote.ChangePercent,
			Turnover:      quote.Turnover,
			AsOf:          quote.AsOf,
		})
	}
	return result
}

func normalizeFundDailyYahooMarketQuotes(quotes []yahoo.MarketQuote) []FundDailyMarketQuote {
	result := make([]FundDailyMarketQuote, 0, len(quotes))
	for _, quote := range quotes {
		if strings.TrimSpace(quote.Symbol) == "" && strings.TrimSpace(quote.Name) == "" {
			continue
		}
		result = append(result, FundDailyMarketQuote{
			Code:          quote.Symbol,
			Name:          quote.Name,
			Category:      quote.Category,
			Source:        "yahoo",
			Price:         quote.Price,
			ChangeAmount:  quote.ChangeAmount,
			ChangePercent: quote.ChangePercent,
			AsOf:          quote.AsOf,
		})
	}
	return result
}
