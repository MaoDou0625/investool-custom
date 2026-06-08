package core

import (
	"context"
	"testing"
	"time"
)

type fakeFundDailyMarketDataProvider struct {
	indexQuotes []FundDailyMarketQuote
	gainers     []FundDailyMarketQuote
	losers      []FundDailyMarketQuote
	crossAssets []FundDailyMarketQuote
	riskAssets  []FundDailyMarketQuote
}

func (p fakeFundDailyMarketDataProvider) QueryIndexQuotes(ctx context.Context) fundDailyMarketQuoteResult {
	return fundDailyMarketQuoteResult{Quotes: p.indexQuotes}
}

func (p fakeFundDailyMarketDataProvider) QueryIndustryGainers(ctx context.Context, limit int) fundDailyMarketQuoteResult {
	return fundDailyMarketQuoteResult{Quotes: p.gainers}
}

func (p fakeFundDailyMarketDataProvider) QueryIndustryLosers(ctx context.Context, limit int) fundDailyMarketQuoteResult {
	return fundDailyMarketQuoteResult{Quotes: p.losers}
}

func (p fakeFundDailyMarketDataProvider) QueryCrossAssetQuotes(ctx context.Context) fundDailyMarketQuoteResult {
	return fundDailyMarketQuoteResult{Quotes: p.crossAssets}
}

func (p fakeFundDailyMarketDataProvider) QueryRiskAssetQuotes(ctx context.Context) fundDailyMarketQuoteResult {
	return fundDailyMarketQuoteResult{Quotes: p.riskAssets}
}

func TestBuildFundDailyMarketContextAnalyzesRiskAndThemes(t *testing.T) {
	provider := fakeFundDailyMarketDataProvider{
		indexQuotes: []FundDailyMarketQuote{
			{Code: "000001", Name: "上证指数", Category: "cn_index", ChangePercent: -1.2},
			{Code: "399001", Name: "深证成指", Category: "cn_index", ChangePercent: -2.3},
			{Code: "399006", Name: "创业板指", Category: "cn_index", ChangePercent: -3.1},
			{Code: "gb_ixic", Name: "纳斯达克", Category: "us_index", ChangePercent: -2.4},
			{Code: "hkHSTECH", Name: "恒生科技指数", Category: "hk_index", ChangePercent: -2.7},
		},
		gainers: []FundDailyMarketQuote{
			{Code: "BK1408", Name: "机器人", ChangePercent: 3.2},
		},
		losers: []FundDailyMarketQuote{
			{Code: "BK1617", Name: "黄金", ChangePercent: -6.4},
			{Code: "BK0732", Name: "贵金属", ChangePercent: -5.8},
		},
		crossAssets: []FundDailyMarketQuote{
			{Code: "hf_CL", Name: "纽约原油", Category: "commodity", ChangePercent: 3.4},
		},
		riskAssets: []FundDailyMarketQuote{
			{Code: "^VIX", Name: "VIX恐慌指数", Category: "volatility", Price: 28, ChangePercent: 12},
			{Code: "^TNX", Name: "美国10年期国债收益率", Category: "rate", Price: 4.75, ChangeAmount: 0.08},
			{Code: "TLT", Name: "20年期以上美债ETF", Category: "bond_etf", ChangePercent: -1.2},
		},
	}

	contextData := buildFundDailyMarketContext(context.Background(), provider, time.Date(2026, 6, 8, 11, 0, 0, 0, time.Local))

	if contextData.Status != "ready" {
		t.Fatalf("expected ready market context, got %s", contextData.Status)
	}
	if contextData.RiskLevel != "high" {
		t.Fatalf("expected high risk level, got %s score %.2f", contextData.RiskLevel, contextData.RiskScore)
	}
	if contextData.BudgetMultiplier >= 1 {
		t.Fatalf("expected budget multiplier below 1 in high risk, got %.2f", contextData.BudgetMultiplier)
	}
	if !hasMarketThemeTilt(contextData.ThemeTilts, "黄金/贵金属", -1) {
		t.Fatalf("expected negative gold tilt, got %+v", contextData.ThemeTilts)
	}
	if !hasMarketThemeTilt(contextData.ThemeTilts, "AI/光模块", 1) {
		t.Fatalf("expected positive tech tilt from robot gainer, got %+v", contextData.ThemeTilts)
	}
	if !hasMarketThemeTilt(contextData.ThemeTilts, "美股科技/QDII", -1) {
		t.Fatalf("expected negative QDII tilt from Nasdaq drop, got %+v", contextData.ThemeTilts)
	}
	if !hasMarketThemeTilt(contextData.ThemeTilts, "油气/能源", 1) {
		t.Fatalf("expected positive energy tilt from oil, got %+v", contextData.ThemeTilts)
	}
	if !hasMarketThemeTilt(contextData.ThemeTilts, "债券/固收", -1) {
		t.Fatalf("expected negative bond tilt from rates/TLT, got %+v", contextData.ThemeTilts)
	}
	if !hasMarketThemeTilt(contextData.ThemeTilts, "港股科技/QDII", -1) {
		t.Fatalf("expected negative HK tech tilt, got %+v", contextData.ThemeTilts)
	}
}

func TestFundDailyMarketTiltScoreForFund(t *testing.T) {
	market := FundDailyMarketContext{
		Status: "ready",
		ThemeTilts: []FundDailyMarketThemeTilt{
			{Theme: "黄金/贵金属", Score: -15},
			{Theme: "成长/科技", Score: 8},
			{Theme: "美股科技/QDII", Score: -12},
			{Theme: "债券/固收", Score: -8},
		},
	}

	goldScore := fundDailyMarketTiltScoreForFund(FundDailyAIFund{Name: "黄金ETF联接A"}, market)
	if goldScore >= 0 {
		t.Fatalf("expected negative gold tilt, got %.2f", goldScore)
	}
	techScore := fundDailyMarketTiltScoreForFund(FundDailyAIFund{Name: "人工智能ETF联接C"}, market)
	if techScore <= 0 {
		t.Fatalf("expected positive tech tilt, got %.2f", techScore)
	}
	qdiiScore := fundDailyMarketTiltScoreForFund(FundDailyAIFund{Name: "纳斯达克100ETF联接(QDII)C"}, market)
	if qdiiScore >= 0 {
		t.Fatalf("expected negative QDII tilt, got %.2f", qdiiScore)
	}
	bondScore := fundDailyMarketTiltScoreForFund(FundDailyAIFund{Name: "中长期纯债债券A"}, market)
	if bondScore >= 0 {
		t.Fatalf("expected negative bond tilt, got %.2f", bondScore)
	}
}

func hasMarketThemeTilt(tilts []FundDailyMarketThemeTilt, theme string, sign int) bool {
	for _, tilt := range tilts {
		if tilt.Theme != theme {
			continue
		}
		if sign < 0 && tilt.Score < 0 {
			return true
		}
		if sign > 0 && tilt.Score > 0 {
			return true
		}
	}
	return false
}
