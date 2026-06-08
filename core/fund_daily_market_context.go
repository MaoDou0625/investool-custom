package core

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

const (
	fundDailyMarketContextTimeout = 18 * time.Second
)

type FundDailyMarketContext struct {
	Status           string                     `json:"status"`
	Provider         string                     `json:"provider,omitempty"`
	GeneratedAt      string                     `json:"generated_at,omitempty"`
	Summary          string                     `json:"summary,omitempty"`
	RiskLevel        string                     `json:"risk_level,omitempty"`
	RiskScore        float64                    `json:"risk_score"`
	BudgetMultiplier float64                    `json:"budget_multiplier"`
	IndexQuotes      []FundDailyMarketQuote     `json:"index_quotes,omitempty"`
	IndustryGainers  []FundDailyMarketQuote     `json:"industry_gainers,omitempty"`
	IndustryLosers   []FundDailyMarketQuote     `json:"industry_losers,omitempty"`
	CrossAssets      []FundDailyMarketQuote     `json:"cross_assets,omitempty"`
	ThemeTilts       []FundDailyMarketThemeTilt `json:"theme_tilts,omitempty"`
	Reasons          []string                   `json:"reasons,omitempty"`
	Warnings         []string                   `json:"warnings,omitempty"`
}

type FundDailyMarketQuote struct {
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	Category      string  `json:"category,omitempty"`
	Source        string  `json:"source,omitempty"`
	Price         float64 `json:"price"`
	ChangePercent float64 `json:"change_percent"`
	Turnover      float64 `json:"turnover,omitempty"`
	AsOf          string  `json:"as_of,omitempty"`
}

type FundDailyMarketThemeTilt struct {
	Theme  string  `json:"theme"`
	Score  float64 `json:"score"`
	Reason string  `json:"reason"`
}

// BuildFundDailyMarketContext fetches and analyzes broad market signals used by daily fund advice.
func BuildFundDailyMarketContext(ctx context.Context) FundDailyMarketContext {
	queryCtx, cancel := context.WithTimeout(ctx, fundDailyMarketContextTimeout)
	defer cancel()
	return buildFundDailyMarketContext(queryCtx, newLiveFundDailyMarketDataProvider(), time.Now())
}

func buildFundDailyMarketContext(ctx context.Context, provider fundDailyMarketDataProvider, now time.Time) FundDailyMarketContext {
	contextData := FundDailyMarketContext{
		Status:           "unavailable",
		Provider:         fundDailyMarketProvider,
		GeneratedAt:      now.Format("2006-01-02 15:04:05"),
		BudgetMultiplier: 1,
		Warnings:         []string{},
	}

	indexQuotes := provider.QueryIndexQuotes(ctx)
	contextData.IndexQuotes = indexQuotes.Quotes
	contextData.Warnings = append(contextData.Warnings, indexQuotes.Warnings...)

	gainers := provider.QueryIndustryGainers(ctx, fundDailyMarketIndustryItemSize)
	contextData.IndustryGainers = gainers.Quotes
	contextData.Warnings = append(contextData.Warnings, gainers.Warnings...)

	losers := provider.QueryIndustryLosers(ctx, fundDailyMarketIndustryItemSize)
	contextData.IndustryLosers = losers.Quotes
	contextData.Warnings = append(contextData.Warnings, losers.Warnings...)

	crossAssets := provider.QueryCrossAssetQuotes(ctx)
	contextData.CrossAssets = crossAssets.Quotes
	contextData.Warnings = append(contextData.Warnings, crossAssets.Warnings...)

	return analyzeFundDailyMarketContext(contextData)
}

func analyzeFundDailyMarketContext(contextData FundDailyMarketContext) FundDailyMarketContext {
	riskScore := 45.0
	reasons := []string{}

	domesticIndexQuotes := marketQuotesByCategory(contextData.IndexQuotes, "cn_index")
	if len(domesticIndexQuotes) == 0 && !hasMarketQuoteCategory(contextData.IndexQuotes) {
		domesticIndexQuotes = contextData.IndexQuotes
	}
	if len(domesticIndexQuotes) > 0 {
		indexAvg := averageMarketChange(domesticIndexQuotes)
		reasons = append(reasons, fmt.Sprintf("A股主要指数平均涨跌 %.2f%%。", indexAvg))
		switch {
		case indexAvg <= -3:
			riskScore += 30
		case indexAvg <= -2:
			riskScore += 22
		case indexAvg <= -1:
			riskScore += 12
		case indexAvg >= 2:
			riskScore -= 16
		case indexAvg >= 1:
			riskScore -= 8
		}
		for _, quote := range domesticIndexQuotes {
			if quote.ChangePercent <= -3 {
				riskScore += 8
				reasons = append(reasons, fmt.Sprintf("%s 跌幅 %.2f%%，成长/权益仓位需要降速。", quote.Name, quote.ChangePercent))
			}
		}
	}

	overseasIndexQuotes := marketQuotesByCategory(contextData.IndexQuotes, "us_index", "hk_index")
	if len(overseasIndexQuotes) > 0 {
		overseasAvg := averageMarketChange(overseasIndexQuotes)
		reasons = append(reasons, fmt.Sprintf("海外/港股指数平均涨跌 %.2f%%。", overseasAvg))
		switch {
		case overseasAvg <= -2:
			riskScore += 12
		case overseasAvg <= -1:
			riskScore += 6
		case overseasAvg >= 1.5:
			riskScore -= 5
		}
		for _, quote := range overseasIndexQuotes {
			if quote.ChangePercent <= -3 {
				riskScore += 6
				reasons = append(reasons, fmt.Sprintf("%s 跌幅 %.2f%%，QDII/海外科技方向需要谨慎。", quote.Name, quote.ChangePercent))
			}
		}
	}

	if len(contextData.IndustryLosers) > 0 {
		loserAvg := averageMarketChange(contextData.IndustryLosers)
		if loserAvg <= -5 {
			riskScore += 12
			reasons = append(reasons, fmt.Sprintf("跌幅靠前行业平均 %.2f%%，市场分化和回撤风险偏高。", loserAvg))
		}
	}
	if len(contextData.IndustryGainers) > 0 {
		gainerAvg := averageMarketChange(contextData.IndustryGainers)
		if gainerAvg >= 2 {
			riskScore -= 4
			reasons = append(reasons, fmt.Sprintf("上涨靠前行业平均 %.2f%%，仍有局部结构性机会。", gainerAvg))
		}
	}
	if len(contextData.CrossAssets) > 0 {
		crossAssetRisk, crossAssetReasons := analyzeFundDailyMarketCrossAssets(contextData.CrossAssets)
		riskScore += crossAssetRisk
		reasons = append(reasons, crossAssetReasons...)
	}

	contextData.RiskScore = clampFloat(riskScore, 0, 100)
	contextData.RiskLevel = fundDailyMarketRiskLevel(contextData.RiskScore)
	contextData.BudgetMultiplier = fundDailyMarketBudgetMultiplier(contextData.RiskScore)
	contextData.ThemeTilts = buildFundDailyMarketThemeTilts(contextData)
	contextData.Reasons = compactDailyReasons(reasons, 6)
	if len(contextData.IndexQuotes) > 0 || len(contextData.IndustryGainers) > 0 || len(contextData.IndustryLosers) > 0 || len(contextData.CrossAssets) > 0 {
		contextData.Status = "ready"
	}
	contextData.Summary = fundDailyMarketSummary(contextData)
	return contextData
}

func marketQuotesByCategory(quotes []FundDailyMarketQuote, categories ...string) []FundDailyMarketQuote {
	wanted := map[string]bool{}
	for _, category := range categories {
		wanted[category] = true
	}
	result := []FundDailyMarketQuote{}
	for _, quote := range quotes {
		if wanted[quote.Category] {
			result = append(result, quote)
		}
	}
	return result
}

func hasMarketQuoteCategory(quotes []FundDailyMarketQuote) bool {
	for _, quote := range quotes {
		if strings.TrimSpace(quote.Category) != "" {
			return true
		}
	}
	return false
}

func analyzeFundDailyMarketCrossAssets(quotes []FundDailyMarketQuote) (float64, []string) {
	riskDelta := 0.0
	reasons := []string{}
	for _, quote := range quotes {
		name := quote.Name + " " + quote.Code
		switch {
		case strings.Contains(name, "人民币"):
			if quote.ChangePercent >= 0.3 {
				riskDelta += 4
				reasons = append(reasons, fmt.Sprintf("%s 上行 %.2f%%，人民币偏弱会抬升海外资产和风险资产波动。", quote.Name, quote.ChangePercent))
			} else if quote.ChangePercent <= -0.3 {
				riskDelta -= 2
				reasons = append(reasons, fmt.Sprintf("%s 下行 %.2f%%，人民币偏强对风险偏好略有支撑。", quote.Name, quote.ChangePercent))
			}
		case strings.Contains(name, "黄金"):
			if quote.ChangePercent >= 1 {
				riskDelta += 4
				reasons = append(reasons, fmt.Sprintf("%s 上涨 %.2f%%，避险资产走强，权益买入节奏需保守。", quote.Name, quote.ChangePercent))
			} else if quote.ChangePercent <= -1 {
				riskDelta -= 2
				reasons = append(reasons, fmt.Sprintf("%s 下跌 %.2f%%，避险需求边际回落。", quote.Name, quote.ChangePercent))
			}
		case strings.Contains(name, "原油"):
			if quote.ChangePercent >= 2 {
				riskDelta += 5
				reasons = append(reasons, fmt.Sprintf("%s 上涨 %.2f%%，通胀和能源价格扰动偏强。", quote.Name, quote.ChangePercent))
			} else if quote.ChangePercent <= -3 {
				riskDelta -= 2
				reasons = append(reasons, fmt.Sprintf("%s 下跌 %.2f%%，能源价格压力边际下降。", quote.Name, quote.ChangePercent))
			}
		}
	}
	return riskDelta, compactDailyReasons(reasons, 4)
}

func averageMarketChange(quotes []FundDailyMarketQuote) float64 {
	if len(quotes) == 0 {
		return 0
	}
	total := 0.0
	count := 0
	for _, quote := range quotes {
		if math.IsNaN(quote.ChangePercent) || math.IsInf(quote.ChangePercent, 0) {
			continue
		}
		total += quote.ChangePercent
		count++
	}
	if count == 0 {
		return 0
	}
	return total / float64(count)
}

func fundDailyMarketRiskLevel(score float64) string {
	switch {
	case score >= 75:
		return "high"
	case score >= 60:
		return "cautious"
	case score >= 40:
		return "neutral"
	default:
		return "constructive"
	}
}

func fundDailyMarketBudgetMultiplier(score float64) float64 {
	switch {
	case score >= 75:
		return 0.45
	case score >= 60:
		return 0.65
	case score >= 40:
		return 0.9
	default:
		return 1.08
	}
}

func buildFundDailyMarketThemeTilts(contextData FundDailyMarketContext) []FundDailyMarketThemeTilt {
	tilts := map[string]FundDailyMarketThemeTilt{}
	addTilt := func(theme string, score float64, reason string) {
		if strings.TrimSpace(theme) == "" {
			return
		}
		current := tilts[theme]
		current.Theme = theme
		current.Score += score
		if current.Reason == "" {
			current.Reason = reason
		} else if reason != "" {
			current.Reason += "；" + reason
		}
		tilts[theme] = current
	}

	for _, quote := range append(contextData.IndustryGainers, contextData.IndustryLosers...) {
		name := quote.Name
		score := 0.0
		switch {
		case quote.ChangePercent >= 3:
			score = 10
		case quote.ChangePercent >= 1:
			score = 5
		case quote.ChangePercent <= -5:
			score = -16
		case quote.ChangePercent <= -3:
			score = -10
		case quote.ChangePercent <= -1:
			score = -5
		}
		if score == 0 {
			continue
		}
		reason := fmt.Sprintf("%s %.2f%%", name, quote.ChangePercent)
		for _, theme := range inferMarketThemesFromIndustry(name) {
			addTilt(theme, score, reason)
		}
	}

	if quote, ok := findMarketQuote(contextData.IndexQuotes, "创业板指"); ok {
		switch {
		case quote.ChangePercent <= -2:
			addTilt("成长/科技", -10, fmt.Sprintf("创业板指 %.2f%%", quote.ChangePercent))
		case quote.ChangePercent >= 1:
			addTilt("成长/科技", 6, fmt.Sprintf("创业板指 %.2f%%", quote.ChangePercent))
		}
	}
	if quote, ok := findMarketQuoteContains(contextData.IndexQuotes, "纳斯达克"); ok {
		switch {
		case quote.ChangePercent <= -2:
			addTilt("美股科技/QDII", -14, fmt.Sprintf("纳斯达克 %.2f%%", quote.ChangePercent))
			addTilt("成长/科技", -4, fmt.Sprintf("纳斯达克 %.2f%%", quote.ChangePercent))
		case quote.ChangePercent >= 1:
			addTilt("美股科技/QDII", 8, fmt.Sprintf("纳斯达克 %.2f%%", quote.ChangePercent))
		}
	}
	if quote, ok := findMarketQuoteContains(contextData.IndexQuotes, "标普"); ok {
		switch {
		case quote.ChangePercent <= -2:
			addTilt("美股宽基/QDII", -10, fmt.Sprintf("标普500 %.2f%%", quote.ChangePercent))
		case quote.ChangePercent >= 1:
			addTilt("美股宽基/QDII", 6, fmt.Sprintf("标普500 %.2f%%", quote.ChangePercent))
		}
	}
	for _, quote := range contextData.CrossAssets {
		name := quote.Name + " " + quote.Code
		switch {
		case strings.Contains(name, "黄金"):
			if quote.ChangePercent >= 1 {
				addTilt("黄金/贵金属", 8, fmt.Sprintf("%s %.2f%%", quote.Name, quote.ChangePercent))
			} else if quote.ChangePercent <= -1 {
				addTilt("黄金/贵金属", -8, fmt.Sprintf("%s %.2f%%", quote.Name, quote.ChangePercent))
			}
		case strings.Contains(name, "原油"):
			if quote.ChangePercent >= 1 {
				addTilt("油气/能源", 6, fmt.Sprintf("%s %.2f%%", quote.Name, quote.ChangePercent))
			} else if quote.ChangePercent <= -1 {
				addTilt("油气/能源", -6, fmt.Sprintf("%s %.2f%%", quote.Name, quote.ChangePercent))
			}
		}
	}

	result := make([]FundDailyMarketThemeTilt, 0, len(tilts))
	for _, item := range tilts {
		item.Score = clampFloat(item.Score, -30, 30)
		result = append(result, item)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if math.Abs(result[i].Score-result[j].Score) > 0.001 {
			return math.Abs(result[i].Score) > math.Abs(result[j].Score)
		}
		return result[i].Theme < result[j].Theme
	})
	return result
}

func inferMarketThemesFromIndustry(name string) []string {
	mapping := []struct {
		needles []string
		themes  []string
	}{
		{[]string{"机器人", "数字", "软件", "半导体", "电子", "通信", "光", "人工智能"}, []string{"成长/科技", "AI/光模块"}},
		{[]string{"新能源", "电池", "光伏", "储能", "锂", "汽车"}, []string{"新能源"}},
		{[]string{"黄金", "贵金属", "白银"}, []string{"黄金/贵金属"}},
		{[]string{"石油", "油气", "煤炭", "能源"}, []string{"油气/能源"}},
		{[]string{"医药", "医疗", "生物"}, []string{"医药"}},
		{[]string{"银行", "保险", "证券"}, []string{"金融/红利"}},
	}
	result := []string{}
	for _, item := range mapping {
		for _, needle := range item.needles {
			if strings.Contains(name, needle) {
				result = append(result, item.themes...)
				break
			}
		}
	}
	return uniqueStrings(result)
}

func findMarketQuote(quotes []FundDailyMarketQuote, name string) (FundDailyMarketQuote, bool) {
	for _, quote := range quotes {
		if quote.Name == name {
			return quote, true
		}
	}
	return FundDailyMarketQuote{}, false
}

func findMarketQuoteContains(quotes []FundDailyMarketQuote, keyword string) (FundDailyMarketQuote, bool) {
	for _, quote := range quotes {
		if strings.Contains(quote.Name, keyword) {
			return quote, true
		}
	}
	return FundDailyMarketQuote{}, false
}

func uniqueStrings(items []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		result = append(result, item)
	}
	return result
}

func fundDailyMarketSummary(contextData FundDailyMarketContext) string {
	if contextData.Status != "ready" {
		return "市场环境数据暂不可用，本次仍按基金与持仓数据生成建议。"
	}
	return fmt.Sprintf("市场风险等级 %s，风险分 %.1f，建议预算倍率 %.2f。", contextData.RiskLevel, contextData.RiskScore, contextData.BudgetMultiplier)
}

func fundDailyMarketTiltScoreForFund(fund FundDailyAIFund, market FundDailyMarketContext) float64 {
	if market.Status != "ready" || len(market.ThemeTilts) == 0 {
		return 0
	}
	themes := inferFundDailyMarketThemes(fund)
	if len(themes) == 0 {
		return 0
	}
	score := 0.0
	for _, tilt := range market.ThemeTilts {
		for _, theme := range themes {
			if tilt.Theme == theme {
				score += tilt.Score
			}
		}
	}
	return clampFloat(score, -30, 30)
}

func inferFundDailyMarketThemes(fund FundDailyAIFund) []string {
	text := strings.Join([]string{fund.Name, fund.FundType, fund.StrategyTheme, fund.IndexName, fund.TargetETFName}, " ")
	themes := []string{}
	if isFundDailyTechExposure(fund) || strings.Contains(text, "科技") || strings.Contains(text, "人工智能") || strings.Contains(text, "半导体") || strings.Contains(text, "通信") {
		themes = append(themes, "成长/科技", "AI/光模块")
	}
	if strings.Contains(text, "新能源") || strings.Contains(text, "电池") || strings.Contains(text, "制造") {
		themes = append(themes, "新能源")
	}
	if strings.Contains(text, "黄金") || strings.Contains(text, "贵金属") {
		themes = append(themes, "黄金/贵金属")
	}
	if strings.Contains(text, "石油") || strings.Contains(text, "油气") || strings.Contains(text, "能源") {
		themes = append(themes, "油气/能源")
	}
	if strings.Contains(text, "纳斯达克") || strings.Contains(text, "美股") || strings.Contains(text, "QDII") || strings.Contains(text, "全球科技") {
		themes = append(themes, "美股科技/QDII")
	}
	if strings.Contains(text, "标普") || strings.Contains(text, "美国") || strings.Contains(text, "全球") {
		themes = append(themes, "美股宽基/QDII")
	}
	if strings.Contains(text, "医药") || strings.Contains(text, "医疗") {
		themes = append(themes, "医药")
	}
	if strings.Contains(text, "银行") || strings.Contains(text, "红利") || strings.Contains(text, "金融") {
		themes = append(themes, "金融/红利")
	}
	return uniqueStrings(themes)
}
