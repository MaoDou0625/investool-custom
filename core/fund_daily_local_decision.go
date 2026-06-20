package core

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	fundDailyDecisionStatusLocal = "local_first_pass"
	fundDailyDecisionProvider    = "local-structured-decision"
	fundDailyDecisionModel       = "fund-daily-local-v1"
)

type fundDailyLocalSignals struct {
	PortfolioTopStocks         map[string]float64
	TechConcentrated           bool
	PortfolioRecentRunup       bool
	PortfolioHighVolatility    bool
	PortfolioConcentrationText string
}

type fundDailyLocalCandidateScore struct {
	fund  FundDailyAIFund
	score float64
}

func BuildFundDailyLocalDecision(contextData FundDailyAIContext) FundDailyAIDecision {
	signals := summarizeFundDailyLocalSignals(contextData)
	budget := chooseFundDailyLocalBudget(contextData, signals)
	actions := []FundDailyAIOutputAction{}
	reasons := []string{
		fmt.Sprintf("程序参考预算 %.2f 元，硬上限 %.2f 元；本地结构化建议会根据集中度、短期涨幅和候选分散度再次压缩。", contextData.BudgetInput.ProgramBudget, contextData.Constraints.MaxDailyBuyAmount),
	}
	riskNotes := []string{
		"该建议来自本地结构化数据规则，不是远程模型实时调用，也不构成收益承诺。",
		"实际交易前仍需结合当天估值、交易费率、赎回规则和个人现金安排。",
	}
	if signals.PortfolioConcentrationText != "" {
		reasons = append(reasons, signals.PortfolioConcentrationText)
	}
	if contextData.MarketContext.Status == "ready" {
		reasons = append(reasons, contextData.MarketContext.Summary)
		reasons = append(reasons, compactDailyReasons(contextData.MarketContext.Reasons, 2)...)
	}
	if contextData.NewsContext.Status == "ready" {
		reasons = append(reasons, contextData.NewsContext.Summary)
		reasons = append(reasons, compactDailyReasons(contextData.NewsContext.Reasons, 2)...)
	}
	if contextData.IndustryExpectationContext.Status == "ready" {
		reasons = append(reasons, contextData.IndustryExpectationContext.Summary)
		reasons = append(reasons, compactDailyReasons(contextData.IndustryExpectationContext.Reasons, 2)...)
		if contextData.IndustryExpectationContext.BudgetMultiplier < 1 {
			riskNotes = append(riskNotes, "行业风险定价模块显示部分风险尚未充分反映，今日买入预算按软约束压缩。")
		}
	}
	if signals.PortfolioRecentRunup {
		reasons = append(reasons, "持有基金近 1/3/6 月涨幅较高，今天优先控制同方向加仓节奏。")
	}
	if signals.PortfolioHighVolatility {
		riskNotes = append(riskNotes, "当前持有里存在回撤或波动偏高的基金，单日加仓金额应明显低于程序硬上限。")
	}

	if contextData.WorkdayGuard.BlocksBuy() {
		reasons = prependUniqueDailyReason(reasons, contextData.WorkdayGuard.Reason)
		riskNotes = prependUniqueDailyReason(riskNotes, contextData.WorkdayGuard.Reason)
	}

	portfolioActions := []FundDailyAIOutputAction{}
	for _, fund := range contextData.Portfolio {
		portfolioActions = append(portfolioActions, fundDailyLocalHoldAction(fund, signals))
	}
	actions = append(actions, portfolioActions...)

	candidateScores := scoreFundDailyLocalCandidates(contextData.Candidates, contextData.Portfolio, signals, contextData.MarketContext, contextData.NewsContext, contextData.IndustryExpectationContext)
	buyActions, usedBudget := buildFundDailyLocalRebalanceBuyActions(contextData, candidateScores, budget, signals)
	actions = mergeFundDailyLocalBuyActions(actions, buyActions)
	trimActions := buildFundDailyLocalProfitTakingActions(contextData.Portfolio, signals, buyActions, !contextData.WorkdayGuard.BlocksBuy())
	actions = mergeFundDailyLocalTrimActions(actions, trimActions)
	watchAction, ok := buildFundDailyLocalWatchAction(contextData.Candidates)
	if ok {
		actions = append(actions, watchAction)
	}
	if len(trimActions) > 0 {
		reasons = append(reasons, "部分已持基金已有浮盈和短期涨幅，今天可以在买入分散仓位的同时小比例止盈。")
		riskNotes = append(riskNotes, "止盈金额仅用于降低单方向暴露，不自动转成新的买入预算。")
	}

	summary := fundDailyLocalDecisionSummary(usedBudget, len(buyActions), len(trimActions), signals)
	if contextData.WorkdayGuard.BlocksBuy() {
		if len(trimActions) > 0 {
			summary = fmt.Sprintf("今天是非工作日，不安排基金买入；保留 %d 只持仓的减仓/止盈参考。", len(trimActions))
		} else {
			summary = "今天是非工作日，不安排基金买入；仅保留持有和观察。"
		}
	}

	decision := FundDailyAIDecision{
		Status:         fundDailyDecisionStatusLocal,
		Provider:       fundDailyDecisionProvider,
		Model:          fundDailyDecisionModel,
		GeneratedAt:    time.Now().Format("2006-01-02 15:04:05"),
		Summary:        summary,
		DailyBuyBudget: usedBudget,
		Actions:        actions,
		Reasons:        reasons,
		RiskNotes:      riskNotes,
		Warnings:       []string{},
	}
	decision = validateFundDailyLocalDecision(decision, contextData)
	return decision
}

func summarizeFundDailyLocalSignals(contextData FundDailyAIContext) fundDailyLocalSignals {
	signals := fundDailyLocalSignals{
		PortfolioTopStocks: map[string]float64{},
	}
	techCount := 0
	recentRunupCount := 0
	highVolatilityCount := 0
	for _, fund := range contextData.Portfolio {
		if isFundDailyTechExposure(fund) {
			techCount++
		}
		if fund.RecentReturns.Month1 >= 10 || fund.RecentReturns.Month3 >= 20 || fund.RecentReturns.Month6 >= 35 {
			recentRunupCount++
		}
		if fund.Stddev >= 28 || fund.Drawdown >= 30 {
			highVolatilityCount++
		}
		for _, stock := range fund.TopStocks {
			if strings.TrimSpace(stock.Name) == "" || stock.HoldRatio < 5 {
				continue
			}
			signals.PortfolioTopStocks[stock.Name] += stock.HoldRatio
		}
	}
	signals.TechConcentrated = len(contextData.Portfolio) > 0 && techCount*2 >= len(contextData.Portfolio)
	signals.PortfolioRecentRunup = recentRunupCount > 0
	signals.PortfolioHighVolatility = highVolatilityCount > 0

	repeated := []string{}
	for stock, weight := range signals.PortfolioTopStocks {
		if weight >= 10 {
			repeated = append(repeated, stock)
		}
	}
	sort.Strings(repeated)
	if signals.TechConcentrated || len(repeated) > 0 {
		if len(repeated) > 0 {
			signals.PortfolioConcentrationText = fmt.Sprintf("当前持有集中在 AI/光模块/科技链条，重仓股重叠明显：%s。", strings.Join(repeated, "、"))
		} else {
			signals.PortfolioConcentrationText = "当前持有集中在 AI/科技链条，今天不宜继续把预算集中到同方向。"
		}
	}
	return signals
}

func chooseFundDailyLocalBudget(contextData FundDailyAIContext, signals fundDailyLocalSignals) float64 {
	if contextData.WorkdayGuard.BlocksBuy() || contextData.BudgetInput.ProgramBudget <= 0 {
		return 0
	}
	if contextData.Constraints.MaxDailyBuyAmount <= 0 || contextData.Constraints.CashRoom <= 0 {
		return 0
	}
	base := minPositive(contextData.BudgetInput.ProgramBudget, contextData.Constraints.MaxDailyBuyAmount, contextData.Constraints.CashRoom)
	if base <= 0 {
		return 0
	}
	factor := 0.35
	if signals.TechConcentrated {
		factor -= 0.06
	}
	if signals.PortfolioRecentRunup {
		factor -= 0.03
	}
	budget := roundDailyAmount(base * factor)
	if signals.TechConcentrated && budget > 150 {
		budget = 150
	}
	if contextData.MarketContext.Status == "ready" && contextData.MarketContext.BudgetMultiplier > 0 {
		budget = roundDailyAmount(budget * contextData.MarketContext.BudgetMultiplier)
	}
	if contextData.NewsContext.Status == "ready" && contextData.NewsContext.BudgetMultiplier > 0 {
		budget = roundDailyAmount(budget * contextData.NewsContext.BudgetMultiplier)
	}
	if contextData.IndustryExpectationContext.Status == "ready" && contextData.IndustryExpectationContext.BudgetMultiplier > 0 {
		budget = roundDailyAmount(budget * contextData.IndustryExpectationContext.BudgetMultiplier)
	}
	if budget > contextData.Constraints.MaxDailyBuyAmount {
		budget = floorDailyAmount(contextData.Constraints.MaxDailyBuyAmount)
	}
	if budget < 0 {
		return 0
	}
	return budget
}

func fundDailyLocalHoldAction(fund FundDailyAIFund, signals fundDailyLocalSignals) FundDailyAIOutputAction {
	reason := "保留持有，今天不新增同方向仓位。"
	if isFundDailyTechExposure(fund) || hasFundDailyLocalPortfolioTopOverlap(fund, signals) {
		reason = "该基金与当前 AI/光模块链条暴露较高，今天先持有观察，不继续加同方向仓位。"
	}
	if fund.RecentReturns.Month1 >= 10 || fund.RecentReturns.Month3 >= 20 || fund.RecentReturns.Month6 >= 35 {
		reason += fmt.Sprintf(" 近 1/3/6 月收益 %.1f%% / %.1f%% / %.1f%%，追高风险需要控制。", fund.RecentReturns.Month1, fund.RecentReturns.Month3, fund.RecentReturns.Month6)
	}
	return FundDailyAIOutputAction{
		Code:     fund.Code,
		Name:     fund.Name,
		FundType: fund.FundType,
		Source:   fundDailyBudgetSourcePortfolio,
		Action:   "hold",
		Amount:   0,
		Reason:   reason,
		RiskNote: fmt.Sprintf("回撤 %.1f%%，波动 %.1f%%，风险等级 %s。", fund.Drawdown, fund.Stddev, fund.RiskLevel),
	}
}

func scoreFundDailyLocalCandidates(candidates []FundDailyAIFund, portfolio []FundDailyAIFund, signals fundDailyLocalSignals, market FundDailyMarketContext, newsContext FundDailyNewsContext, industryContext FundDailyIndustryExpectationContext) []fundDailyLocalCandidateScore {
	filtered := make([]FundDailyAIFund, 0, len(candidates))
	for _, fund := range candidates {
		if !fund.CanSubscribe || fund.SuggestedBuyCeiling <= 0 || fund.Score < 65 {
			continue
		}
		filtered = append(filtered, fund)
	}
	filtered = filterOwnedShareClassFundDailyAIFunds(preferClassCFundDailyAIFunds(filtered), portfolio)
	scores := make([]fundDailyLocalCandidateScore, 0, len(filtered))
	for _, fund := range filtered {
		score := fund.StrategyScore*0.55 + float64(fund.Score)*0.35
		if strings.Contains(fund.StrategyTheme, "量化") {
			score += 16
		}
		if strings.Contains(fund.StrategyTheme, "海外") || strings.Contains(fund.Name, "纳斯达克") || strings.Contains(fund.Name, "QDII") {
			score += 12
		}
		if strings.Contains(fund.Name, "纳斯达克科技") {
			score += 8
		}
		if fund.Score >= 90 && fund.NetAssetsScaleYi >= 1 {
			score += 10
		}
		if signals.TechConcentrated && isFundDailyTechExposure(fund) {
			score -= 18
		}
		if hasFundDailyLocalPortfolioTopOverlap(fund, signals) {
			score -= 18
		}
		if fund.RecentReturns.Month1 >= 20 || fund.RecentReturns.Month3 >= 40 || fund.RecentReturns.Month6 >= 70 {
			score -= 10
		}
		if fund.Stddev >= 25 || fund.Drawdown >= 25 {
			score -= 8
		}
		if strings.Contains(fund.StrategyTheme, "黄金") && (fund.RecentReturns.Month1 < 0 || fund.RecentReturns.Month3 < 0) {
			score -= 25
		}
		score += fundDailyMarketTiltScoreForFund(fund, market)
		score += fundDailyNewsTiltScoreForFund(fund, newsContext)
		score += fundDailyIndustryExpectationTiltScoreForFund(fund, industryContext)
		scores = append(scores, fundDailyLocalCandidateScore{fund: fund, score: score})
	}
	sort.SliceStable(scores, func(i, j int) bool {
		if scores[i].score != scores[j].score {
			return scores[i].score > scores[j].score
		}
		if scores[i].fund.Score != scores[j].fund.Score {
			return scores[i].fund.Score > scores[j].fund.Score
		}
		return scores[i].fund.Code < scores[j].fund.Code
	})
	return scores
}

func filterOwnedShareClassFundDailyAIFunds(candidates []FundDailyAIFund, portfolio []FundDailyAIFund) []FundDailyAIFund {
	if len(candidates) == 0 || len(portfolio) == 0 {
		return candidates
	}

	ownedGroups := map[string]bool{}
	for _, fund := range portfolio {
		group := fundDailyShareClassGroup(fund.Name, fund.Code)
		if group != "" {
			ownedGroups[group] = true
		}
	}
	if len(ownedGroups) == 0 {
		return candidates
	}

	filtered := make([]FundDailyAIFund, 0, len(candidates))
	for _, fund := range candidates {
		if ownedGroups[fundDailyShareClassGroup(fund.Name, fund.Code)] {
			continue
		}
		filtered = append(filtered, fund)
	}
	return filtered
}

func fundDailyShareClassGroup(name string, code string) string {
	base, _ := splitFundShareClass(name)
	if base != "" {
		return base
	}
	return code
}

func buildFundDailyLocalBuyActions(scores []fundDailyLocalCandidateScore, budget float64) ([]FundDailyAIOutputAction, float64) {
	if budget <= 0 {
		return nil, 0
	}
	actions := []FundDailyAIOutputAction{}
	remaining := budget
	selectedThemes := map[string]bool{}
	for _, item := range scores {
		if remaining < 10 || len(actions) >= 2 {
			break
		}
		fund := item.fund
		themeKey := fundDailyLocalThemeKey(fund)
		if selectedThemes[themeKey] && len(actions) > 0 {
			continue
		}
		amount := 60.0
		if budget >= 180 {
			amount = 80
		}
		amount = minPositive(amount, fund.SuggestedBuyCeiling, remaining)
		amount = roundDailyAmount(amount)
		if amount <= 0 {
			continue
		}
		actions = append(actions, FundDailyAIOutputAction{
			Code:     fund.Code,
			Name:     fund.Name,
			FundType: fund.FundType,
			Source:   fundDailyBudgetSourceCandidate,
			Action:   "buy",
			Amount:   amount,
			Reason:   fundDailyLocalBuyReason(fund),
			RiskNote: fmt.Sprintf("回撤 %.1f%%，波动 %.1f%%，近 1/3/6 月收益 %.1f%% / %.1f%% / %.1f%%。", fund.Drawdown, fund.Stddev, fund.RecentReturns.Month1, fund.RecentReturns.Month3, fund.RecentReturns.Month6),
		})
		selectedThemes[themeKey] = true
		remaining -= amount
	}
	return actions, budget - remaining
}

func buildFundDailyLocalWatchAction(candidates []FundDailyAIFund) (FundDailyAIOutputAction, bool) {
	var best *FundDailyAIFund
	for idx := range candidates {
		fund := candidates[idx]
		if !strings.Contains(fund.StrategyTheme, "黄金") && !strings.Contains(fund.StrategyTheme, "贵金属") {
			continue
		}
		if best == nil || fund.Score > best.Score {
			best = &fund
		}
	}
	if best == nil {
		return FundDailyAIOutputAction{}, false
	}
	return FundDailyAIOutputAction{
		Code:     best.Code,
		Name:     best.Name,
		FundType: best.FundType,
		Source:   fundDailyBudgetSourceCandidate,
		Action:   "watch",
		Amount:   0,
		Reason:   "黄金/贵金属与科技链条相关性较低，有分散价值；但近 1/3 月收益偏弱，今天先观察，不急于买入。",
		RiskNote: fmt.Sprintf("近 1/3 月收益 %.1f%% / %.1f%%，趋势尚未恢复。", best.RecentReturns.Month1, best.RecentReturns.Month3),
	}, true
}

type fundDailyDecisionBuyLimit struct {
	buyAmount     float64
	currentAmount float64
	canSubscribe  bool
}

func validateFundDailyLocalDecision(decision FundDailyAIDecision, contextData FundDailyAIContext) FundDailyAIDecision {
	limits := map[string]fundDailyDecisionBuyLimit{}
	for _, fund := range contextData.Portfolio {
		limits[fundDailyDecisionActionKey(fundDailyBudgetSourcePortfolio, fund.Code)] = fundDailyDecisionBuyLimit{
			buyAmount:     fund.SuggestedBuyCeiling,
			currentAmount: fund.CurrentAmount,
			canSubscribe:  fund.CanSubscribe,
		}
	}
	for _, fund := range contextData.Candidates {
		limits[fundDailyDecisionActionKey(fundDailyBudgetSourceCandidate, fund.Code)] = fundDailyDecisionBuyLimit{
			buyAmount:     fund.SuggestedBuyCeiling,
			currentAmount: fund.CurrentAmount,
			canSubscribe:  fund.CanSubscribe,
		}
	}

	total := 0.0
	for idx := range decision.Actions {
		action := &decision.Actions[idx]
		if action.Amount <= 0 {
			continue
		}
		key := fundDailyDecisionActionKey(action.Source, action.Code)
		limit, ok := limits[key]
		if !ok {
			action.Amount = 0
			decision.Warnings = append(decision.Warnings, fmt.Sprintf("%s 不在本次持有/候选数据集中，操作金额已清零。", action.Code))
			continue
		}
		if action.Action == "trim" || action.Action == "sell" {
			if action.Source != fundDailyBudgetSourcePortfolio || limit.currentAmount <= 0 {
				action.Amount = 0
				decision.Warnings = append(decision.Warnings, fmt.Sprintf("%s 不是有效持仓，卖出金额已清零。", action.Code))
				continue
			}
			sellCap := limit.currentAmount
			if action.Action == "trim" {
				sellCap = minPositive(limit.currentAmount*0.30, limit.currentAmount)
			}
			if action.Amount > sellCap {
				action.Amount = floorDailyAmount(sellCap)
				decision.Warnings = append(decision.Warnings, fmt.Sprintf("%s 超过本次减仓上限，已压缩到 %.2f 元。", action.Code, action.Amount))
			}
			continue
		}
		if action.Action != "buy" {
			action.Amount = 0
			decision.Warnings = append(decision.Warnings, fmt.Sprintf("%s 的非买入动作不应带正金额，金额已清零。", action.Code))
			continue
		}
		if !limit.canSubscribe {
			action.Amount = 0
			decision.Warnings = append(decision.Warnings, fmt.Sprintf("%s 当前不是开放申购状态，买入金额已清零。", action.Code))
			continue
		}
		if action.Amount > limit.buyAmount {
			action.Amount = floorDailyAmount(limit.buyAmount)
			decision.Warnings = append(decision.Warnings, fmt.Sprintf("%s 超过单基金上限，已压缩到 %.2f 元。", action.Code, action.Amount))
		}
		total += action.Amount
	}
	if total > contextData.Constraints.MaxDailyBuyAmount {
		remaining := floorDailyAmount(contextData.Constraints.MaxDailyBuyAmount)
		total = 0
		for idx := range decision.Actions {
			action := &decision.Actions[idx]
			if action.Action != "buy" || action.Amount <= 0 {
				continue
			}
			if remaining <= 0 {
				action.Amount = 0
				continue
			}
			allowed := action.Amount
			if allowed > remaining {
				allowed = floorDailyAmount(remaining)
			}
			action.Amount = allowed
			remaining -= allowed
			total += allowed
		}
		decision.Warnings = append(decision.Warnings, "总买入额超过硬上限，已按顺序压缩。")
	}
	decision.DailyBuyBudget = total
	decision.Validated = true
	return decision
}

func fundDailyLocalDecisionSummary(budget float64, buyCount int, trimCount int, signals fundDailyLocalSignals) string {
	if budget <= 0 || buyCount == 0 {
		if trimCount > 0 {
			return fmt.Sprintf("今天不安排新增买入，但建议对 %d 只已有浮盈且波动偏高的持仓小比例止盈。", trimCount)
		}
		return "今天不安排新增买入，先观察持仓和候选方向的波动。"
	}
	if trimCount > 0 {
		return fmt.Sprintf("今天建议总买入 %.2f 元，同时对 %d 只已有浮盈的持仓小比例止盈。", budget, trimCount)
	}
	if signals.TechConcentrated {
		return fmt.Sprintf("今天建议总买入 %.2f 元，只做小额分散，不继续集中加 AI/光模块方向。", budget)
	}
	return fmt.Sprintf("今天建议总买入 %.2f 元，优先选择质量、趋势和分散度更均衡的基金。", budget)
}

func fundDailyLocalBuyReason(fund FundDailyAIFund) string {
	switch {
	case strings.Contains(fund.StrategyTheme, "量化"):
		return "量化多因子方向可降低对单一 AI/光模块链条的依赖，适合小额分散配置。"
	case strings.Contains(fund.StrategyTheme, "海外") || strings.Contains(fund.Name, "纳斯达克"):
		return "海外科技暴露与当前 A 股科技持仓不完全相同，且本次优先选择可用的 C 类份额做小额试买。"
	default:
		return "综合评分、趋势和风险指标较好，但仍按小额试买处理。"
	}
}

func fundDailyLocalThemeKey(fund FundDailyAIFund) string {
	switch {
	case strings.Contains(fund.StrategyTheme, "量化"):
		return "quant"
	case strings.Contains(fund.StrategyTheme, "海外") || strings.Contains(fund.Name, "纳斯达克"):
		return "overseas"
	case strings.Contains(fund.StrategyTheme, "黄金") || strings.Contains(fund.StrategyTheme, "贵金属"):
		return "gold"
	case isFundDailyTechExposure(fund):
		return "tech"
	default:
		return fund.StrategyTheme
	}
}

func isFundDailyTechExposure(fund FundDailyAIFund) bool {
	text := fund.Name + " " + fund.StrategyTheme + " " + fund.FundType
	for _, keyword := range []string{"AI", "人工智能", "光模块", "科技", "半导体", "通信", "5G", "科创", "高端制造", "先进制造"} {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	for _, stock := range fund.TopStocks {
		if stock.HoldRatio < 3 {
			continue
		}
		if stock.Name == "中际旭创" || stock.Name == "新易盛" || stock.Name == "源杰科技" || stock.Name == "寒武纪" {
			return true
		}
	}
	return false
}

func hasFundDailyLocalPortfolioTopOverlap(fund FundDailyAIFund, signals fundDailyLocalSignals) bool {
	for _, stock := range fund.TopStocks {
		if stock.HoldRatio < 3 {
			continue
		}
		if signals.PortfolioTopStocks[stock.Name] > 0 {
			return true
		}
	}
	return false
}

func preferClassCFundDailyAIFunds(funds []FundDailyAIFund) []FundDailyAIFund {
	groupHasC := map[string]bool{}
	classes := make([]string, len(funds))
	groups := make([]string, len(funds))
	for idx, fund := range funds {
		base, class := splitFundShareClass(fund.Name)
		if base == "" {
			base = fund.Code
		}
		groups[idx] = base
		classes[idx] = class
		if class == fundShareClassC {
			groupHasC[base] = true
		}
	}
	filtered := make([]FundDailyAIFund, 0, len(funds))
	for idx, fund := range funds {
		if classes[idx] == fundShareClassA && groupHasC[groups[idx]] {
			continue
		}
		filtered = append(filtered, fund)
	}
	return filtered
}

func fundDailyDecisionActionKey(source string, code string) string {
	return source + ":" + code
}

func (d FundDailyAIDecision) DailyBuyBudgetText() string {
	return fmt.Sprintf("%.2f", d.DailyBuyBudget)
}

func (a FundDailyAIOutputAction) AmountText() string {
	if a.Amount == 0 {
		return "--"
	}
	return fmt.Sprintf("%.2f", a.Amount)
}

func (a FundDailyAIOutputAction) ActionLabel() string {
	switch a.Action {
	case "buy":
		return "买入"
	case "hold":
		return "持有"
	case "watch":
		return "观察"
	case "trim":
		return "减仓"
	case "sell":
		return "放弃/卖出"
	case "skip":
		return "跳过"
	default:
		return a.Action
	}
}

func (a FundDailyAIOutputAction) SourceLabel() string {
	switch a.Source {
	case fundDailyBudgetSourcePortfolio:
		return "已持有"
	case fundDailyBudgetSourceCandidate:
		return "候选"
	default:
		return a.Source
	}
}

func (a FundDailyAIOutputAction) BadgeClass() string {
	switch a.Action {
	case "buy":
		return "green"
	case "sell":
		return "red"
	case "trim":
		return "orange"
	case "watch":
		return "blue"
	default:
		return "grey"
	}
}
