package core

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/axiaoxin-com/investool/datacenter/eastmoney"
	"github.com/axiaoxin-com/investool/models"
)

const fundDailyCorrelationMinPairs = 10

type FundDailyCandidateSelection struct {
	Funds       models.FundList
	Evidence    map[string]FundDailyCandidateEvidence
	Warnings    []string
	SourceName  string
	SourceCount int
}

type FundDailyCandidateEvidence struct {
	Code             string
	Theme            string
	StrategyScore    float64
	TrendScore       float64
	ThemeScore       float64
	CorrelationScore float64
	MaxCorrelation   float64
	HasCorrelation   bool
	ThemeOverlap     float64
	Reasons          []string
	Warnings         []string
}

type fundDailyStrategyCandidateScore struct {
	fund     *models.Fund
	evidence FundDailyCandidateEvidence
}

type fundDailyThemeTrend struct {
	theme  string
	count  int
	month1 float64
	month3 float64
	month6 float64
	rank   float64
	score  float64
}

type fundDailyThemeTrendAccumulator struct {
	theme       string
	count       int
	month1Total float64
	month1Count int
	month3Total float64
	month3Count int
	month6Total float64
	month6Count int
	rankTotal   float64
	rankCount   int
}

type fundDailyPortfolioProfile struct {
	owned         []fundDailyOwnedProfile
	themeWeights  map[string]float64
	typeWeights   map[string]float64
	hasNAVHistory bool
}

type fundDailyOwnedProfile struct {
	code     string
	name     string
	theme    string
	fundType string
	weight   float64
	returns  map[string]float64
}

func SelectDailyAdviceCandidatesWithStrategy(
	ctx context.Context,
	allFunds models.FundList,
	fallback models.FundList,
	portfolioAdvices []FundPortfolioAdvice,
	navCache models.FundNAVHistoryCache,
	maxCount int,
) FundDailyCandidateSelection {
	if maxCount <= 0 {
		maxCount = 80
	}

	source := allFunds
	sourceName := "本地全量基金缓存 + 趋势/相关性评分"
	if len(source) == 0 {
		source = fallback
		sourceName = "每日候选基金兜底 + 趋势/相关性评分"
	}

	selection := FundDailyCandidateSelection{
		Evidence:    map[string]FundDailyCandidateEvidence{},
		Warnings:    []string{},
		SourceName:  sourceName,
		SourceCount: len(source),
	}
	if len(source) == 0 {
		selection.Warnings = append(selection.Warnings, "没有可用于每日推荐的基金缓存，请先刷新本地全量基金缓存或每日候选。")
		return selection
	}
	if len(allFunds) == 0 && len(fallback) > 0 {
		selection.Warnings = append(selection.Warnings, "本地全量基金缓存为空，本次只能使用每日候选池兜底；建议先生成全量缓存。")
	}

	profile := buildFundDailyPortfolioProfile(portfolioAdvices, navCache)
	if len(profile.owned) > 0 && !profile.hasNAVHistory {
		selection.Warnings = append(selection.Warnings, "当前已持有基金缺少历史净值缓存，相关性将主要使用主题和基金类型重叠做代理。")
	}

	themeTrends := buildFundDailyThemeTrends(source)
	scored, subscriptionWarnings := scoreFundDailyStrategyCandidates(ctx, source, portfolioAdvices, navCache, profile, themeTrends)
	if len(subscriptionWarnings) > 0 {
		selection.Warnings = append(selection.Warnings, subscriptionWarnings...)
	}
	selected := selectDiverseFundDailyCandidates(scored, maxCount)
	if len(selected) == 0 && len(allFunds) > 0 && len(fallback) > 0 {
		selection.Warnings = append(selection.Warnings, "全量基金缓存没有生成可用候选，本次退回每日候选池重新评分。")
		selection.SourceName = "每日候选基金兜底 + 趋势/相关性评分"
		selection.SourceCount = len(fallback)
		themeTrends = buildFundDailyThemeTrends(fallback)
		scored, subscriptionWarnings = scoreFundDailyStrategyCandidates(ctx, fallback, portfolioAdvices, navCache, profile, themeTrends)
		if len(subscriptionWarnings) > 0 {
			selection.Warnings = append(selection.Warnings, subscriptionWarnings...)
		}
		selected = selectDiverseFundDailyCandidates(scored, maxCount)
	}

	for _, item := range selected {
		selection.Funds = append(selection.Funds, item.fund)
		selection.Evidence[item.fund.Code] = item.evidence
	}
	if len(selection.Funds) == 0 {
		selection.Warnings = append(selection.Warnings, "趋势、风控和相关性评分后没有可用候选，请检查基金缓存是否包含近期收益、排名和风险指标。")
	}
	return selection
}

func SelectDailyAdviceCandidates(ctx context.Context, allFunds models.FundList, fallback models.FundList, maxCount int) models.FundList {
	selection := SelectDailyAdviceCandidatesWithStrategy(ctx, allFunds, fallback, nil, models.FundNAVHistoryCache{}, maxCount)
	return selection.Funds
}

func scoreFundDailyStrategyCandidates(
	ctx context.Context,
	funds models.FundList,
	portfolioAdvices []FundPortfolioAdvice,
	navCache models.FundNAVHistoryCache,
	profile fundDailyPortfolioProfile,
	themeTrends map[string]fundDailyThemeTrend,
) ([]fundDailyStrategyCandidateScore, []string) {
	portfolioCodes := map[string]struct{}{}
	for _, advice := range portfolioAdvices {
		code := strings.TrimSpace(advice.Item.Code)
		if code != "" {
			portfolioCodes[code] = struct{}{}
		}
	}
	subscriptionBlocked := map[string]string{}

	scored := make([]fundDailyStrategyCandidateScore, 0, len(funds))
	for _, fund := range funds {
		if fund == nil || strings.TrimSpace(fund.Code) == "" {
			continue
		}
		if _, exists := portfolioCodes[fund.Code]; exists {
			continue
		}
		if !fund.CanSubscribe() {
			subscriptionBlocked[fund.Code] = strings.TrimSpace(fund.SubscriptionStatus)
		}
		evidence, ok := scoreFundDailyStrategyCandidate(ctx, fund, navCache, profile, themeTrends)
		if !ok {
			continue
		}
		scored = append(scored, fundDailyStrategyCandidateScore{
			fund:     fund,
			evidence: evidence,
		})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		if math.Abs(scored[i].evidence.StrategyScore-scored[j].evidence.StrategyScore) > 0.001 {
			return scored[i].evidence.StrategyScore > scored[j].evidence.StrategyScore
		}
		if scored[i].fund.Performance.Month3ProfitRatio != scored[j].fund.Performance.Month3ProfitRatio {
			return scored[i].fund.Performance.Month3ProfitRatio > scored[j].fund.Performance.Month3ProfitRatio
		}
		return scored[i].fund.Code < scored[j].fund.Code
	})
	return scored, buildFundDailySubscriptionWarningMessages(subscriptionBlocked)
}

func buildFundDailySubscriptionWarningMessages(blocked map[string]string) []string {
	if len(blocked) == 0 {
		return nil
	}
	items := make([]string, 0, len(blocked))
	for code, status := range blocked {
		if strings.TrimSpace(status) == "" {
			status = "未读取到申购状态"
		}
		items = append(items, fmt.Sprintf("%s（%s）", code, status))
	}
	sort.Strings(items)
	return []string{
		fmt.Sprintf("以下候选基金当前非开放申购，今日建议仅观察不加仓：%s", strings.Join(items, "；")),
	}
}

func scoreFundDailyStrategyCandidate(
	ctx context.Context,
	fund *models.Fund,
	navCache models.FundNAVHistoryCache,
	profile fundDailyPortfolioProfile,
	themeTrends map[string]fundDailyThemeTrend,
) (FundDailyCandidateEvidence, bool) {
	if shouldSkipFund4433Recommendation(fund) {
		return FundDailyCandidateEvidence{}, false
	}

	advice := EvaluateFundPortfolioItem(ctx, models.FundPortfolioItem{
		Code:   fund.Code,
		Status: models.FundPortfolioStatusWatch,
	}, fund, eastmoney.FundHolderStructureResult{})
	qualityScore := float64(advice.Score)
	qualityReason := fmt.Sprintf("质量：综合评分 %d，4433 不是硬门槛", advice.Score)
	if score, rankFields, ok := scoreFund4433Recommendation(ctx, fund); ok {
		qualityScore = qualityScore*0.70 + clampFloat(score, 0, 110)*0.30
		qualityReason = fmt.Sprintf("质量：综合评分 %d，近 4433 质量分 %.1f，%d 个排名字段可用", advice.Score, score, rankFields)
	}

	trendScore, trendReason := fundDailyMomentumScore(fund)
	theme := inferFundTheme(fund, "", "")
	themeScore := 0.0
	themeReason := "主题：缺少同主题趋势样本，按基金自身趋势为主"
	if trend, ok := themeTrends[theme]; ok {
		themeScore = trend.score
		themeReason = fmt.Sprintf("主题：%s 样本 %d 只，近1/3/6月均值 %.1f%% / %.1f%% / %.1f%%", trend.theme, trend.count, trend.month1, trend.month3, trend.month6)
	}

	correlationScore, maxCorrelation, hasCorrelation, themeOverlap, correlationReason, correlationWarnings := fundDailyCorrelationSignal(fund, navCache, profile, theme)
	riskScore, riskReason := fundDailyRiskAdjustment(fund)
	strategyScore := clampFloat(qualityScore*0.55+trendScore+themeScore+correlationScore+riskScore+8, 0, 100)

	if strategyScore <= 0 {
		return FundDailyCandidateEvidence{}, false
	}
	return FundDailyCandidateEvidence{
		Code:             fund.Code,
		Theme:            theme,
		StrategyScore:    strategyScore,
		TrendScore:       trendScore,
		ThemeScore:       themeScore,
		CorrelationScore: correlationScore,
		MaxCorrelation:   maxCorrelation,
		HasCorrelation:   hasCorrelation,
		ThemeOverlap:     themeOverlap,
		Reasons: []string{
			fmt.Sprintf("数据评分：%.1f（趋势 %.1f，主题 %.1f，分散化 %.1f）", strategyScore, trendScore, themeScore, correlationScore),
			trendReason,
			correlationReason,
			themeReason,
			qualityReason,
			riskReason,
		},
		Warnings: correlationWarnings,
	}, true
}

func buildFundDailyThemeTrends(funds models.FundList) map[string]fundDailyThemeTrend {
	accumulators := map[string]*fundDailyThemeTrendAccumulator{}
	for _, fund := range funds {
		if fund == nil || shouldSkipFund4433Recommendation(fund) {
			continue
		}
		theme := inferFundTheme(fund, "", "")
		if strings.TrimSpace(theme) == "" {
			continue
		}
		accumulator := accumulators[theme]
		if accumulator == nil {
			accumulator = &fundDailyThemeTrendAccumulator{theme: theme}
			accumulators[theme] = accumulator
		}
		accumulator.add(fund)
	}

	trends := map[string]fundDailyThemeTrend{}
	for theme, accumulator := range accumulators {
		trend := accumulator.trend()
		if trend.count == 0 {
			continue
		}
		trends[theme] = trend
	}
	return trends
}

func (a *fundDailyThemeTrendAccumulator) add(fund *models.Fund) {
	a.count++
	if fund.Performance.Month1ProfitRatio != 0 {
		a.month1Total += fund.Performance.Month1ProfitRatio
		a.month1Count++
	}
	if fund.Performance.Month3ProfitRatio != 0 {
		a.month3Total += fund.Performance.Month3ProfitRatio
		a.month3Count++
	}
	if fund.Performance.Month6ProfitRatio != 0 {
		a.month6Total += fund.Performance.Month6ProfitRatio
		a.month6Count++
	}
	for _, ratio := range []float64{
		fund.Performance.Month3RankRatio,
		fund.Performance.Month6RankRatio,
		fund.Performance.ThisYearRankRatio,
	} {
		if ratio > 0 {
			a.rankTotal += ratio
			a.rankCount++
		}
	}
}

func (a fundDailyThemeTrendAccumulator) trend() fundDailyThemeTrend {
	trend := fundDailyThemeTrend{
		theme: a.theme,
		count: a.count,
	}
	trend.month1 = averageOrZero(a.month1Total, a.month1Count)
	trend.month3 = averageOrZero(a.month3Total, a.month3Count)
	trend.month6 = averageOrZero(a.month6Total, a.month6Count)
	trend.rank = averageOrZero(a.rankTotal, a.rankCount)

	score := 0.0
	score += clampFloat(trend.month1/2.0, -4, 6)
	score += clampFloat(trend.month3/3.0, -6, 8)
	score += clampFloat(trend.month6/6.0, -4, 6)
	switch {
	case trend.rank > 0 && trend.rank <= 25:
		score += 4
	case trend.rank > 0 && trend.rank <= 40:
		score += 2
	case trend.rank > 60:
		score -= 4
	}
	if trend.count < 3 {
		score *= 0.65
	}
	trend.score = clampFloat(score, -10, 15)
	return trend
}

func fundDailyMomentumScore(fund *models.Fund) (float64, string) {
	score := 0.0
	score += clampFloat(fund.Performance.Month1ProfitRatio/1.8, -6, 7)
	score += clampFloat(fund.Performance.Month3ProfitRatio/3.0, -8, 9)
	score += clampFloat(fund.Performance.Month6ProfitRatio/5.0, -6, 7)
	score += clampFloat(fund.Performance.ThisYearProfitRatio/5.0, -4, 5)
	for _, ratio := range []float64{
		fund.Performance.Month3RankRatio,
		fund.Performance.Month6RankRatio,
		fund.Performance.ThisYearRankRatio,
	} {
		switch {
		case ratio > 0 && ratio <= 25:
			score += 2.5
		case ratio > 0 && ratio <= 40:
			score += 1.2
		case ratio > 65:
			score -= 2.5
		}
	}
	score = clampFloat(score, -18, 24)
	return score, fmt.Sprintf("趋势：近1/3/6月收益 %.1f%% / %.1f%% / %.1f%%，今年 %.1f%%", fund.Performance.Month1ProfitRatio, fund.Performance.Month3ProfitRatio, fund.Performance.Month6ProfitRatio, fund.Performance.ThisYearProfitRatio)
}

func buildFundDailyPortfolioProfile(
	advices []FundPortfolioAdvice,
	navCache models.FundNAVHistoryCache,
) fundDailyPortfolioProfile {
	profile := fundDailyPortfolioProfile{
		owned:        []fundDailyOwnedProfile{},
		themeWeights: map[string]float64{},
		typeWeights:  map[string]float64{},
	}

	totalAmount := currentAmountFromAdvices(advices)
	for _, advice := range advices {
		if !advice.Item.IsOwned() || advice.Fund == nil {
			continue
		}
		weight := advice.CurrentWeight
		if weight <= 0 && totalAmount > 0 && advice.CurrentAmount > 0 {
			weight = advice.CurrentAmount / totalAmount * 100
		}
		theme := inferFundTheme(advice.Fund, "", "")
		owned := fundDailyOwnedProfile{
			code:     advice.Item.Code,
			name:     advice.Fund.Name,
			theme:    theme,
			fundType: advice.Fund.Type,
			weight:   weight,
		}
		if item, ok := navCache.Items[advice.Item.Code]; ok {
			returns := fundDailyNAVReturns(item.Points, 90)
			if len(returns) >= fundDailyCorrelationMinPairs {
				owned.returns = returns
				profile.hasNAVHistory = true
			}
		}
		profile.owned = append(profile.owned, owned)
	}

	if len(profile.owned) == 0 {
		return profile
	}
	equalWeight := 100.0 / float64(len(profile.owned))
	for i := range profile.owned {
		if profile.owned[i].weight <= 0 {
			profile.owned[i].weight = equalWeight
		}
		profile.themeWeights[profile.owned[i].theme] += profile.owned[i].weight
		if strings.TrimSpace(profile.owned[i].fundType) != "" {
			profile.typeWeights[profile.owned[i].fundType] += profile.owned[i].weight
		}
	}
	return profile
}

func fundDailyCorrelationSignal(
	fund *models.Fund,
	navCache models.FundNAVHistoryCache,
	profile fundDailyPortfolioProfile,
	theme string,
) (float64, float64, bool, float64, string, []string) {
	if len(profile.owned) == 0 {
		return 4, 0, false, 0, "分散化：当前没有持仓，候选不受组合相关性约束", nil
	}

	themeOverlap := profile.themeWeights[theme]
	candidateReturns := map[string]float64{}
	if item, ok := navCache.Items[fund.Code]; ok {
		candidateReturns = fundDailyNAVReturns(item.Points, 90)
	}

	maxCorrelation := -2.0
	hasCorrelation := false
	for _, owned := range profile.owned {
		if len(candidateReturns) == 0 || len(owned.returns) == 0 {
			continue
		}
		correlation, ok := fundDailyPearsonCorrelation(candidateReturns, owned.returns)
		if !ok {
			continue
		}
		hasCorrelation = true
		if correlation > maxCorrelation {
			maxCorrelation = correlation
		}
	}

	if hasCorrelation {
		switch {
		case maxCorrelation >= 0.85:
			return -10, maxCorrelation, true, themeOverlap, fmt.Sprintf("相关性：与已持有基金近净值最大相关 %.2f，重叠偏高", maxCorrelation), nil
		case maxCorrelation >= 0.65:
			return -5, maxCorrelation, true, themeOverlap, fmt.Sprintf("相关性：与已持有基金近净值最大相关 %.2f，需要控制仓位", maxCorrelation), nil
		case maxCorrelation <= 0.30:
			return 8, maxCorrelation, true, themeOverlap, fmt.Sprintf("相关性：与已持有基金近净值最大相关 %.2f，分散化价值较高", maxCorrelation), nil
		case maxCorrelation <= 0.50:
			return 4, maxCorrelation, true, themeOverlap, fmt.Sprintf("相关性：与已持有基金近净值最大相关 %.2f，相关性较低", maxCorrelation), nil
		default:
			return 1, maxCorrelation, true, themeOverlap, fmt.Sprintf("相关性：与已持有基金近净值最大相关 %.2f，中等相关", maxCorrelation), nil
		}
	}

	warnings := []string{}
	if profile.hasNAVHistory {
		warnings = append(warnings, "候选基金缺少本地历史净值，相关性使用主题重叠估算。")
	}
	switch {
	case themeOverlap >= 35:
		return -8, 0, false, themeOverlap, fmt.Sprintf("分散化：当前组合中 %s 已约 %.1f%%，不宜继续集中", theme, themeOverlap), warnings
	case themeOverlap >= 18:
		return -3, 0, false, themeOverlap, fmt.Sprintf("分散化：当前组合中 %s 已约 %.1f%%，仓位需克制", theme, themeOverlap), warnings
	default:
		typeOverlap := profile.typeWeights[fund.Type]
		if typeOverlap >= 60 {
			return -2, 0, false, themeOverlap, fmt.Sprintf("分散化：主题重叠低，但同类型基金仓位约 %.1f%%", typeOverlap), warnings
		}
		return 5, 0, false, themeOverlap, fmt.Sprintf("分散化：与当前持仓主题重叠约 %.1f%%，可提升组合分散度", themeOverlap), warnings
	}
}

func fundDailyRiskAdjustment(fund *models.Fund) (float64, string) {
	score := 0.0
	switch {
	case fund.MaxRetracement.Avg135 > 0 && fund.MaxRetracement.Avg135 <= 20:
		score += 3
	case fund.MaxRetracement.Avg135 > 40:
		score -= 12
	case fund.MaxRetracement.Avg135 > 30:
		score -= 6
	}
	switch {
	case fund.Stddev.Avg135 > 0 && fund.Stddev.Avg135 <= 20:
		score += 2
	case fund.Stddev.Avg135 > 35:
		score -= 8
	case fund.Stddev.Avg135 > 28:
		score -= 4
	}
	switch {
	case fund.Sharp.Avg135 >= 1.2:
		score += 4
	case fund.Sharp.Avg135 > 0 && fund.Sharp.Avg135 < 0.5:
		score -= 4
	}
	return score, fmt.Sprintf("风控：回撤 %.1f%%，波动 %.1f%%，夏普 %.2f", fund.MaxRetracement.Avg135, fund.Stddev.Avg135, fund.Sharp.Avg135)
}

func selectDiverseFundDailyCandidates(scored []fundDailyStrategyCandidateScore, maxCount int) []fundDailyStrategyCandidateScore {
	if maxCount <= 0 {
		maxCount = 80
	}
	maxPerTheme := fundDailyMaxPerTheme(maxCount)
	selected := []fundDailyStrategyCandidateScore{}
	themeCounts := map[string]int{}
	for _, item := range scored {
		theme := item.evidence.Theme
		if theme == "" {
			theme = "其他/未分类"
		}
		if themeCounts[theme] >= maxPerTheme {
			continue
		}
		selected = append(selected, item)
		themeCounts[theme]++
		if len(selected) >= maxCount {
			break
		}
	}
	return selected
}

func fundDailyMaxPerTheme(maxCount int) int {
	switch {
	case maxCount <= 8:
		return 2
	case maxCount <= 30:
		return 3
	default:
		return 6
	}
}

func fundDailyNAVReturns(points []models.FundNAVHistoryCachePoint, maxPairs int) map[string]float64 {
	returns := map[string]float64{}
	if len(points) < 2 {
		return returns
	}
	sorted := append([]models.FundNAVHistoryCachePoint(nil), points...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Date < sorted[j].Date
	})
	if maxPairs > 0 && len(sorted) > maxPairs+1 {
		sorted = sorted[len(sorted)-maxPairs-1:]
	}
	for i := 1; i < len(sorted); i++ {
		prev := sorted[i-1]
		curr := sorted[i]
		if prev.UnitNAV <= 0 || curr.UnitNAV <= 0 || strings.TrimSpace(curr.Date) == "" {
			continue
		}
		returns[curr.Date] = curr.UnitNAV/prev.UnitNAV - 1
	}
	return returns
}

func fundDailyPearsonCorrelation(a map[string]float64, b map[string]float64) (float64, bool) {
	xs := []float64{}
	ys := []float64{}
	for date, x := range a {
		y, ok := b[date]
		if !ok {
			continue
		}
		xs = append(xs, x)
		ys = append(ys, y)
	}
	if len(xs) < fundDailyCorrelationMinPairs {
		return 0, false
	}

	meanX := averageFloat64(xs)
	meanY := averageFloat64(ys)
	var numerator, denomX, denomY float64
	for i := range xs {
		dx := xs[i] - meanX
		dy := ys[i] - meanY
		numerator += dx * dy
		denomX += dx * dx
		denomY += dy * dy
	}
	if denomX <= 0 || denomY <= 0 {
		return 0, false
	}
	return numerator / math.Sqrt(denomX*denomY), true
}

func averageOrZero(total float64, count int) float64 {
	if count <= 0 {
		return 0
	}
	return total / float64(count)
}

func averageFloat64(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}
