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

func TestBuildFundDailyMarketContextAnalyzesRiskAndThemes(t *testing.T) {
	provider := fakeFundDailyMarketDataProvider{
		indexQuotes: []FundDailyMarketQuote{
			{Code: "000001", Name: "上证指数", Category: "cn_index", ChangePercent: -1.2},
			{Code: "399001", Name: "深证成指", Category: "cn_index", ChangePercent: -2.3},
			{Code: "399006", Name: "创业板指", Category: "cn_index", ChangePercent: -3.1},
			{Code: "gb_ixic", Name: "纳斯达克", Category: "us_index", ChangePercent: -2.4},
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
}

func TestFundDailyMarketTiltScoreForFund(t *testing.T) {
	market := FundDailyMarketContext{
		Status: "ready",
		ThemeTilts: []FundDailyMarketThemeTilt{
			{Theme: "黄金/贵金属", Score: -15},
			{Theme: "成长/科技", Score: 8},
			{Theme: "美股科技/QDII", Score: -12},
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
