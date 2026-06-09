package core

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/axiaoxin-com/investool/datacenter/news"
)

const (
	fundDailyNewsContextTimeout = 10 * time.Second
	fundDailyNewsLookback       = 72 * time.Hour
	fundDailyNewsProvider       = "rss-news-context"
	fundDailyNewsItemLimit      = 8
)

var fundDailyNewsFeeds = []news.RSSFeed{
	{Name: "BBC World", URL: "https://feeds.bbci.co.uk/news/world/rss.xml", Region: "global", Topic: "world"},
	{Name: "BBC Business", URL: "https://feeds.bbci.co.uk/news/business/rss.xml", Region: "global", Topic: "business"},
	{Name: "CNBC Top News", URL: "https://www.cnbc.com/id/100003114/device/rss/rss.html", Region: "global", Topic: "markets"},
	{Name: "CNBC World News", URL: "https://www.cnbc.com/id/10001147/device/rss/rss.html", Region: "global", Topic: "world"},
}

type FundDailyNewsContext struct {
	Status               string                     `json:"status"`
	Provider             string                     `json:"provider,omitempty"`
	GeneratedAt          string                     `json:"generated_at,omitempty"`
	Summary              string                     `json:"summary,omitempty"`
	ArticleCount         int                        `json:"article_count"`
	RelevantArticleCount int                        `json:"relevant_article_count"`
	RiskDelta            float64                    `json:"risk_delta"`
	BudgetMultiplier     float64                    `json:"budget_multiplier"`
	Articles             []FundDailyNewsArticle     `json:"articles,omitempty"`
	ThemeTilts           []FundDailyMarketThemeTilt `json:"theme_tilts,omitempty"`
	Reasons              []string                   `json:"reasons,omitempty"`
	Warnings             []string                   `json:"warnings,omitempty"`
}

type FundDailyNewsArticle struct {
	Source      string                     `json:"source"`
	Title       string                     `json:"title"`
	Link        string                     `json:"link,omitempty"`
	PublishedAt string                     `json:"published_at,omitempty"`
	Category    string                     `json:"category,omitempty"`
	RiskDelta   float64                    `json:"risk_delta"`
	Confidence  float64                    `json:"confidence"`
	Reason      string                     `json:"reason,omitempty"`
	ThemeTilts  []FundDailyMarketThemeTilt `json:"theme_tilts,omitempty"`
}

type fundDailyNewsArticleResult struct {
	Articles []news.Article
	Warnings []string
}

type fundDailyNewsDataProvider interface {
	QueryArticles(ctx context.Context) fundDailyNewsArticleResult
}

type liveFundDailyNewsDataProvider struct {
	client news.News
	feeds  []news.RSSFeed
}

func newLiveFundDailyNewsDataProvider() liveFundDailyNewsDataProvider {
	return liveFundDailyNewsDataProvider{
		client: news.NewNews(),
		feeds:  fundDailyNewsFeeds,
	}
}

func (p liveFundDailyNewsDataProvider) QueryArticles(ctx context.Context) fundDailyNewsArticleResult {
	articles, err := p.client.QueryRSSFeeds(ctx, p.feeds, 10)
	result := fundDailyNewsArticleResult{Articles: articles}
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("新闻RSS读取部分失败：%s", compactFundDailyMarketError(err)))
	}
	return result
}

func BuildFundDailyNewsContext(ctx context.Context) FundDailyNewsContext {
	queryCtx, cancel := context.WithTimeout(ctx, fundDailyNewsContextTimeout)
	defer cancel()
	return buildFundDailyNewsContext(queryCtx, newLiveFundDailyNewsDataProvider(), time.Now())
}

func buildFundDailyNewsContext(ctx context.Context, provider fundDailyNewsDataProvider, now time.Time) FundDailyNewsContext {
	contextData := FundDailyNewsContext{
		Status:           "unavailable",
		Provider:         fundDailyNewsProvider,
		GeneratedAt:      now.Format("2006-01-02 15:04:05"),
		BudgetMultiplier: 1,
		Warnings:         []string{},
	}
	result := provider.QueryArticles(ctx)
	contextData.ArticleCount = len(result.Articles)
	contextData.Warnings = append(contextData.Warnings, result.Warnings...)
	return analyzeFundDailyNewsContext(contextData, result.Articles, now)
}

func analyzeFundDailyNewsContext(contextData FundDailyNewsContext, articles []news.Article, now time.Time) FundDailyNewsContext {
	reasons := []string{}
	themeTilts := map[string]FundDailyMarketThemeTilt{}
	scoredArticles := []FundDailyNewsArticle{}
	riskDelta := 0.0

	for _, article := range articles {
		if isStaleFundDailyNewsArticle(article, now) {
			continue
		}
		scored, relevant := scoreFundDailyNewsArticle(article)
		if !relevant {
			continue
		}
		scoredArticles = append(scoredArticles, scored)
		riskDelta += scored.RiskDelta * scored.Confidence
		if scored.Reason != "" {
			reasons = append(reasons, scored.Reason)
		}
		for _, tilt := range scored.ThemeTilts {
			addFundDailyNewsThemeTilt(themeTilts, tilt.Theme, tilt.Score*scored.Confidence, tilt.Reason)
		}
	}

	sort.SliceStable(scoredArticles, func(i, j int) bool {
		left := math.Abs(scoredArticles[i].RiskDelta * scoredArticles[i].Confidence)
		right := math.Abs(scoredArticles[j].RiskDelta * scoredArticles[j].Confidence)
		if math.Abs(left-right) > 0.001 {
			return left > right
		}
		return scoredArticles[i].Title < scoredArticles[j].Title
	})
	contextData.RelevantArticleCount = len(scoredArticles)
	if len(scoredArticles) > fundDailyNewsItemLimit {
		scoredArticles = scoredArticles[:fundDailyNewsItemLimit]
	}

	contextData.RiskDelta = clampFloat(riskDelta, -15, 20)
	contextData.BudgetMultiplier = fundDailyNewsBudgetMultiplier(contextData.RiskDelta)
	contextData.Articles = scoredArticles
	contextData.ThemeTilts = fundDailyNewsThemeTilts(themeTilts)
	contextData.Reasons = compactDailyReasons(reasons, 6)
	if len(articles) > 0 {
		contextData.Status = "ready"
	}
	if len(articles) == 0 && len(contextData.Warnings) == 0 {
		contextData.Warnings = append(contextData.Warnings, "新闻RSS未返回可用条目。")
	}
	contextData.Summary = fundDailyNewsSummary(contextData)
	return contextData
}

func isStaleFundDailyNewsArticle(article news.Article, now time.Time) bool {
	if article.PublishedAt.IsZero() {
		return true
	}
	return now.Sub(article.PublishedAt) > fundDailyNewsLookback
}

func scoreFundDailyNewsArticle(article news.Article) (FundDailyNewsArticle, bool) {
	text := strings.ToLower(strings.Join([]string{
		article.Title,
		article.Description,
		strings.Join(article.Categories, " "),
		article.Topic,
	}, " "))
	scored := FundDailyNewsArticle{
		Source:      firstNonEmptyNewsString(article.Source, article.FeedName),
		Title:       article.Title,
		Link:        article.Link,
		PublishedAt: formatFundDailyNewsTime(article.PublishedAt),
		Confidence:  0.65,
	}
	reasons := []string{}
	tilts := []FundDailyMarketThemeTilt{}
	riskDelta := 0.0
	category := ""

	addSignal := func(cat string, delta float64, confidenceBoost float64, reason string, articleTilts ...FundDailyMarketThemeTilt) {
		riskDelta += delta
		scored.Confidence += confidenceBoost
		if category == "" || math.Abs(delta) > math.Abs(scored.RiskDelta) {
			category = cat
		}
		reasons = append(reasons, reason)
		tilts = append(tilts, articleTilts...)
		scored.RiskDelta = delta
	}

	if containsAnyFundDailyNews(text, "ceasefire", "truce", "peace deal", "de-escalation", "deescalation") {
		addSignal("geopolitical_relief", -5, 0.08, "地缘冲突缓和类新闻降低避险压力。",
			FundDailyMarketThemeTilt{Theme: "黄金/贵金属", Score: -4, Reason: "地缘风险缓和"},
			FundDailyMarketThemeTilt{Theme: "油气/能源", Score: -3, Reason: "地缘风险缓和"},
		)
	}
	geoArea := containsAnyFundDailyNews(text, "iran", "israel", "gaza", "ukraine", "russia", "red sea", "south china sea", "middle east")
	geoConflict := containsAnyFundDailyNews(text, "war", "conflict", "missile", "military", "troops", "invasion")
	geoAttack := containsAnyFundDailyNews(text, "attack", "attacks") && geoArea
	geoTension := containsAnyFundDailyNews(text, "tension", "tensions", "flare-up", "strike", "strikes", "clash", "escalation") && geoArea
	if geoConflict || geoAttack || geoTension {
		addSignal("geopolitical_risk", 8, 0.10, "地缘冲突或军事风险新闻提高组合风险。",
			FundDailyMarketThemeTilt{Theme: "黄金/贵金属", Score: 5, Reason: "地缘风险升温"},
			FundDailyMarketThemeTilt{Theme: "油气/能源", Score: 4, Reason: "地缘风险升温"},
			FundDailyMarketThemeTilt{Theme: "成长/科技", Score: -4, Reason: "地缘风险升温"},
		)
	}
	if containsAnyFundDailyNews(text, "tariff", "sanction", "export control", "export controls", "chip ban", "semiconductor restriction", "technology restriction", "trade war") {
		addSignal("trade_tech_restriction", 7, 0.10, "贸易、制裁或科技限制新闻压制科技成长风险偏好。",
			FundDailyMarketThemeTilt{Theme: "AI/光模块", Score: -8, Reason: "科技限制风险"},
			FundDailyMarketThemeTilt{Theme: "成长/科技", Score: -6, Reason: "科技限制风险"},
			FundDailyMarketThemeTilt{Theme: "美股科技/QDII", Score: -5, Reason: "科技限制风险"},
		)
	}
	if containsAnyFundDailyNews(text, "fed", "federal reserve", "powell", "inflation", "cpi", "pce", "rate", "rates", "treasury yield", "yields") {
		if containsAnyFundDailyNews(text, "hawkish", "higher", "sticky", "hot", "rise", "rises", "surge", "jumps", "above forecast") {
			addSignal("rate_inflation_pressure", 6, 0.08, "通胀、央行或利率偏鹰新闻提高成长资产估值压力。",
				FundDailyMarketThemeTilt{Theme: "成长/科技", Score: -5, Reason: "利率/通胀压力"},
				FundDailyMarketThemeTilt{Theme: "美股科技/QDII", Score: -6, Reason: "利率/通胀压力"},
				FundDailyMarketThemeTilt{Theme: "债券/固收", Score: -4, Reason: "利率/通胀压力"},
			)
		}
		if containsAnyFundDailyNews(text, "cut", "cuts", "cooling", "cool", "easing", "lower", "below forecast", "dovish") {
			addSignal("rate_inflation_relief", -4, 0.06, "通胀降温或降息预期新闻缓和估值压力。",
				FundDailyMarketThemeTilt{Theme: "成长/科技", Score: 4, Reason: "利率/通胀压力缓和"},
				FundDailyMarketThemeTilt{Theme: "美股科技/QDII", Score: 4, Reason: "利率/通胀压力缓和"},
				FundDailyMarketThemeTilt{Theme: "债券/固收", Score: 3, Reason: "利率/通胀压力缓和"},
			)
		}
	}
	if containsAnyFundDailyNews(text, "oil", "crude", "opec", "shipping", "red sea") {
		if containsAnyFundDailyNews(text, "surge", "rally", "rise", "rises", "supply", "attack", "disruption") {
			addSignal("energy_supply_risk", 6, 0.08, "能源或航运扰动新闻提高通胀和风险资产压力。",
				FundDailyMarketThemeTilt{Theme: "油气/能源", Score: 7, Reason: "能源供给扰动"},
				FundDailyMarketThemeTilt{Theme: "成长/科技", Score: -3, Reason: "能源供给扰动"},
			)
		}
		if containsAnyFundDailyNews(text, "fall", "falls", "drop", "decline", "slides") {
			addSignal("energy_pressure_relief", -2, 0.03, "油价回落新闻边际缓和通胀压力。",
				FundDailyMarketThemeTilt{Theme: "油气/能源", Score: -3, Reason: "油价回落"},
			)
		}
	}
	if containsAnyFundDailyNews(text, "recession", "banking crisis", "debt crisis", "default", "credit crunch", "financial crisis") {
		addSignal("systemic_risk", 8, 0.10, "衰退、债务或金融系统风险新闻提高防守需求。",
			FundDailyMarketThemeTilt{Theme: "成长/科技", Score: -5, Reason: "系统性风险升温"},
			FundDailyMarketThemeTilt{Theme: "黄金/贵金属", Score: 4, Reason: "系统性风险升温"},
		)
	}

	if len(reasons) == 0 && containsAnyFundDailyNews(text, "market", "stocks", "shares", "economy", "global") {
		scored.RiskDelta = 0
		scored.Confidence = 0.45
		scored.Category = "market_watch"
		scored.Reason = "普通市场新闻，未触发明确风险阈值。"
		return scored, false
	}
	if len(reasons) == 0 {
		return FundDailyNewsArticle{}, false
	}
	scored.RiskDelta = clampFloat(riskDelta, -10, 12)
	scored.Confidence = clampFloat(scored.Confidence, 0.45, 0.95)
	scored.Category = category
	scored.Reason = strings.Join(uniqueStrings(reasons), "；")
	scored.ThemeTilts = compactFundDailyNewsTilts(tilts)
	return scored, true
}

func containsAnyFundDailyNews(text string, needles ...string) bool {
	normalizedText := normalizeFundDailyNewsMatchText(text)
	if normalizedText == "" {
		return false
	}
	paddedText := " " + normalizedText + " "
	for _, needle := range needles {
		normalizedNeedle := normalizeFundDailyNewsMatchText(needle)
		if normalizedNeedle == "" {
			continue
		}
		if strings.Contains(paddedText, " "+normalizedNeedle+" ") {
			return true
		}
	}
	return false
}

func normalizeFundDailyNewsMatchText(text string) string {
	text = strings.ToLower(text)
	builder := strings.Builder{}
	lastSpace := true
	for _, ch := range text {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') {
			builder.WriteRune(ch)
			lastSpace = false
			continue
		}
		if !lastSpace {
			builder.WriteRune(' ')
			lastSpace = true
		}
	}
	return strings.TrimSpace(builder.String())
}

func addFundDailyNewsThemeTilt(tilts map[string]FundDailyMarketThemeTilt, theme string, score float64, reason string) {
	if strings.TrimSpace(theme) == "" {
		return
	}
	current := tilts[theme]
	current.Theme = theme
	current.Score += score
	if current.Reason == "" {
		current.Reason = reason
	} else if reason != "" && !strings.Contains(current.Reason, reason) {
		current.Reason += "；" + reason
	}
	tilts[theme] = current
}

func fundDailyNewsThemeTilts(tilts map[string]FundDailyMarketThemeTilt) []FundDailyMarketThemeTilt {
	result := make([]FundDailyMarketThemeTilt, 0, len(tilts))
	for _, tilt := range tilts {
		tilt.Score = clampFloat(tilt.Score, -18, 18)
		result = append(result, tilt)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if math.Abs(result[i].Score-result[j].Score) > 0.001 {
			return math.Abs(result[i].Score) > math.Abs(result[j].Score)
		}
		return result[i].Theme < result[j].Theme
	})
	return result
}

func compactFundDailyNewsTilts(tilts []FundDailyMarketThemeTilt) []FundDailyMarketThemeTilt {
	grouped := map[string]FundDailyMarketThemeTilt{}
	for _, tilt := range tilts {
		addFundDailyNewsThemeTilt(grouped, tilt.Theme, tilt.Score, tilt.Reason)
	}
	return fundDailyNewsThemeTilts(grouped)
}

func fundDailyNewsBudgetMultiplier(riskDelta float64) float64 {
	switch {
	case riskDelta >= 15:
		return 0.78
	case riskDelta >= 8:
		return 0.88
	case riskDelta >= 3:
		return 0.95
	case riskDelta <= -8:
		return 1.05
	case riskDelta <= -3:
		return 1.02
	default:
		return 1
	}
}

func fundDailyNewsSummary(contextData FundDailyNewsContext) string {
	if contextData.Status != "ready" {
		return "新闻/国际局势数据暂不可用，本次不调整新闻风险因子。"
	}
	if contextData.RelevantArticleCount == 0 {
		return fmt.Sprintf("新闻/国际局势未触发明确风险阈值，预算倍率 %.2f。", contextData.BudgetMultiplier)
	}
	return fmt.Sprintf("新闻/国际局势风险调整 %.1f，相关条目 %d/%d，预算倍率 %.2f。", contextData.RiskDelta, contextData.RelevantArticleCount, contextData.ArticleCount, contextData.BudgetMultiplier)
}

func formatFundDailyNewsTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format("2006-01-02 15:04:05")
}

func firstNonEmptyNewsString(items ...string) string {
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			return item
		}
	}
	return ""
}

func fundDailyNewsTiltScoreForFund(fund FundDailyAIFund, newsContext FundDailyNewsContext) float64 {
	if newsContext.Status != "ready" || len(newsContext.ThemeTilts) == 0 {
		return 0
	}
	return fundDailyThemeTiltScoreForFund(fund, newsContext.ThemeTilts)
}
