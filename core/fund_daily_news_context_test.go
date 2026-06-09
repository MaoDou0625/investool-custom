package core

import (
	"context"
	"testing"
	"time"

	"github.com/axiaoxin-com/investool/datacenter/news"
)

type fakeFundDailyNewsDataProvider struct {
	articles []news.Article
	warnings []string
}

func (p fakeFundDailyNewsDataProvider) QueryArticles(ctx context.Context) fundDailyNewsArticleResult {
	return fundDailyNewsArticleResult{Articles: p.articles, Warnings: p.warnings}
}

func TestBuildFundDailyNewsContextScoresRelevantNews(t *testing.T) {
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	provider := fakeFundDailyNewsDataProvider{
		articles: []news.Article{
			{
				Source:      "Example",
				Title:       "Fed signals higher rates as inflation stays hot",
				Description: "Technology shares watch higher treasury yields.",
				Link:        "https://example.com/fed",
				PublishedAt: now.Add(-2 * time.Hour),
			},
			{
				Source:      "Example",
				Title:       "Ceasefire deal lowers oil supply fears",
				Link:        "https://example.com/ceasefire",
				PublishedAt: now.Add(-3 * time.Hour),
			},
			{
				Source:      "Example",
				Title:       "Old conflict headline",
				Link:        "https://example.com/old",
				PublishedAt: now.Add(-100 * time.Hour),
			},
		},
	}

	contextData := buildFundDailyNewsContext(context.Background(), provider, now)

	if contextData.Status != "ready" {
		t.Fatalf("expected ready news context, got %s", contextData.Status)
	}
	if contextData.ArticleCount != 3 {
		t.Fatalf("expected article count 3, got %d", contextData.ArticleCount)
	}
	if contextData.RelevantArticleCount != 2 {
		t.Fatalf("expected 2 relevant fresh articles, got %d", contextData.RelevantArticleCount)
	}
	if contextData.BudgetMultiplier <= 0 || contextData.BudgetMultiplier > 1.05 {
		t.Fatalf("unexpected budget multiplier %.2f", contextData.BudgetMultiplier)
	}
	if !hasMarketThemeTilt(contextData.ThemeTilts, "成长/科技", -1) {
		t.Fatalf("expected negative growth/tech news tilt, got %+v", contextData.ThemeTilts)
	}
	if len(contextData.Articles) != 2 {
		t.Fatalf("expected relevant articles to be exposed, got %d", len(contextData.Articles))
	}
}

func TestFundDailyNewsTiltScoreForFund(t *testing.T) {
	contextData := FundDailyNewsContext{
		Status: "ready",
		ThemeTilts: []FundDailyMarketThemeTilt{
			{Theme: "成长/科技", Score: -10},
			{Theme: "黄金/贵金属", Score: 6},
		},
	}

	techScore := fundDailyNewsTiltScoreForFund(FundDailyAIFund{Name: "人工智能ETF联接C"}, contextData)
	if techScore >= 0 {
		t.Fatalf("expected negative news tilt for tech fund, got %.2f", techScore)
	}
	goldScore := fundDailyNewsTiltScoreForFund(FundDailyAIFund{Name: "黄金ETF联接A"}, contextData)
	if goldScore <= 0 {
		t.Fatalf("expected positive news tilt for gold fund, got %.2f", goldScore)
	}
}

func TestScoreFundDailyNewsArticleDoesNotTreatHeartAttackAsGeopoliticalRisk(t *testing.T) {
	article := news.Article{
		Source:      "Example",
		Title:       "Player doing well after heart attack",
		Description: "A sports update about recovery after a medical emergency.",
		Link:        "https://example.com/sports",
		PublishedAt: time.Date(2026, 6, 9, 10, 0, 0, 0, time.UTC),
	}

	_, relevant := scoreFundDailyNewsArticle(article)

	if relevant {
		t.Fatalf("expected non-geopolitical attack article to be ignored")
	}
}

func TestScoreFundDailyNewsArticleUsesWordBoundaries(t *testing.T) {
	for _, title := range []string{
		"Warner Bros shares rise after studio update",
		"Corporate profits rise as company cuts costs",
	} {
		_, relevant := scoreFundDailyNewsArticle(news.Article{
			Source:      "Example",
			Title:       title,
			Description: "Ordinary company news.",
			Link:        "https://example.com/company",
			PublishedAt: time.Date(2026, 6, 9, 10, 0, 0, 0, time.UTC),
		})
		if relevant {
			t.Fatalf("expected title %q to avoid substring keyword false positives", title)
		}
	}
}

func TestBuildFundDailyNewsContextSkipsMissingPublishedAtForRisk(t *testing.T) {
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	contextData := buildFundDailyNewsContext(context.Background(), fakeFundDailyNewsDataProvider{
		articles: []news.Article{
			{
				Source: "Example",
				Title:  "Fed signals higher rates as inflation stays hot",
				Link:   "https://example.com/no-date",
			},
		},
	}, now)

	if contextData.Status != "ready" {
		t.Fatalf("expected ready status when feed returned articles")
	}
	if contextData.RelevantArticleCount != 0 || contextData.RiskDelta != 0 || contextData.BudgetMultiplier != 1 {
		t.Fatalf("expected missing-date article to be display-only, got relevant=%d risk=%.2f multiplier=%.2f", contextData.RelevantArticleCount, contextData.RiskDelta, contextData.BudgetMultiplier)
	}
}

func TestBuildFundDailyNewsContextCountsRelevantArticlesBeforeDisplayCap(t *testing.T) {
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	articles := []news.Article{}
	for idx := 0; idx < fundDailyNewsItemLimit+3; idx++ {
		articles = append(articles, news.Article{
			Source:      "Example",
			Title:       "Fed signals higher rates as inflation stays hot",
			Link:        "https://example.com/fed-" + string(rune('a'+idx)),
			PublishedAt: now.Add(-time.Duration(idx) * time.Hour),
		})
	}

	contextData := buildFundDailyNewsContext(context.Background(), fakeFundDailyNewsDataProvider{articles: articles}, now)

	if contextData.RelevantArticleCount != fundDailyNewsItemLimit+3 {
		t.Fatalf("expected relevant count before cap, got %d", contextData.RelevantArticleCount)
	}
	if len(contextData.Articles) != fundDailyNewsItemLimit {
		t.Fatalf("expected displayed articles capped at %d, got %d", fundDailyNewsItemLimit, len(contextData.Articles))
	}
}
