package core

import (
	"fmt"
	"sort"
)

const (
	fundDailyBudgetSourcePortfolio = "portfolio"
	fundDailyBudgetSourceCandidate = "candidate"
)

type fundDailyBudgetDecision struct {
	Budget     float64
	MaxBudget  float64
	Confidence float64
	Reasons    []string
}

type fundDailyBudgetActionRef struct {
	source string
	action *FundDailyAction
}

func maxDailyBuyBudget(report FundDailyAdviceReport) float64 {
	if report.CashRoom <= 0 || report.InvestableAmount <= 0 {
		return 0
	}
	budget := report.InvestableAmount * report.Config.MaxDailyBuyWeight / 100
	if budget > report.CashRoom {
		budget = report.CashRoom
	}
	return floorDailyAmount(budget)
}

func chooseDailyBuyBudget(portfolioActions []FundDailyAction, candidateActions []FundDailyAction, report FundDailyAdviceReport) fundDailyBudgetDecision {
	maxBudget := maxDailyBuyBudget(report)
	decision := fundDailyBudgetDecision{
		MaxBudget: maxBudget,
		Reasons:   []string{},
	}
	if maxBudget <= 0 {
		decision.Reasons = append(decision.Reasons, "今日可用加仓空间不足，所有买入操作只保留观察。")
		return decision
	}

	refs := collectDailyBuyBudgetActionRefs(portfolioActions, candidateActions)
	signals := summarizeDailyBudgetSignals(refs, report)
	if signals.buyableCount == 0 {
		decision.Reasons = append(decision.Reasons, "持有基金和候选基金都没有形成有效买入信号，今日不安排新增买入。")
		return decision
	}

	confidence := 0.30
	switch {
	case signals.avgStrategyScore >= 90:
		confidence += 0.22
	case signals.avgStrategyScore >= 80:
		confidence += 0.15
	case signals.avgStrategyScore >= 70:
		confidence += 0.08
	case signals.avgStrategyScore > 0:
		confidence -= 0.08
	}
	switch {
	case signals.avgScore >= 88:
		confidence += 0.15
	case signals.avgScore >= 78:
		confidence += 0.08
	case signals.avgScore < 68:
		confidence -= 0.10
	}
	switch {
	case signals.avgTrendScore >= 16:
		confidence += 0.18
	case signals.avgTrendScore >= 8:
		confidence += 0.10
	case signals.avgTrendScore < 0:
		confidence -= 0.15
	}
	switch {
	case signals.avgCorrelationScore >= 5:
		confidence += 0.08
	case signals.avgCorrelationScore < 0:
		confidence -= 0.12
	}
	switch {
	case signals.avgDrawdown > 30 || signals.avgStddev > 30:
		confidence -= 0.22
	case signals.avgDrawdown > 25 || signals.avgStddev > 25:
		confidence -= 0.10
	case signals.avgSharp >= 1.2:
		confidence += 0.06
	}
	switch {
	case signals.currentWeight >= 70:
		confidence -= 0.25
	case signals.currentWeight >= 50:
		confidence -= 0.12
	case signals.currentWeight <= 20:
		confidence += 0.08
	}

	decision.Confidence = clampFloat(confidence, 0.12, 1)
	decision.Budget = floorDailyAmount(maxBudget * decision.Confidence)
	decision.Reasons = append(decision.Reasons,
		fmt.Sprintf("AI预算：最高可买 %.2f 元，数据信心 %.0f%%，今日总买入预算 %.2f 元。", maxBudget, decision.Confidence*100, decision.Budget),
		fmt.Sprintf("买入信号：持有基金 %d 个，候选基金 %d 个，统一竞争今日预算。", signals.portfolioBuyCount, signals.candidateBuyCount),
		fmt.Sprintf("趋势/质量：买入项平均数据评分 %.1f，综合评分 %.1f，趋势分 %.1f。", signals.avgStrategyScore, signals.avgScore, signals.avgTrendScore),
		fmt.Sprintf("风险/分散：平均回撤 %.1f%%，波动 %.1f%%，分散化分 %.1f。", signals.avgDrawdown, signals.avgStddev, signals.avgCorrelationScore),
		fmt.Sprintf("仓位：当前已投入约 %.1f%%，避免持有基金和候选基金合计一天买满。", signals.currentWeight),
	)
	return decision
}

func applyDailyBuyBudget(portfolioActions []FundDailyAction, candidateActions []FundDailyAction, report FundDailyAdviceReport) ([]FundDailyAction, []FundDailyAction) {
	refs := collectDailyBuyBudgetActionRefs(portfolioActions, candidateActions)
	if len(refs) == 0 {
		return portfolioActions, candidateActions
	}
	sort.SliceStable(refs, func(i, j int) bool {
		left := dailyBuyBudgetPriority(refs[i])
		right := dailyBuyBudgetPriority(refs[j])
		if left != right {
			return left > right
		}
		return refs[i].action.Code < refs[j].action.Code
	})

	remaining := report.DailyBuyBudget
	for _, ref := range refs {
		if ref.action.SuggestedAmount <= 0 {
			continue
		}
		if remaining <= 0 {
			markDailyBuyBudgetExhausted(ref.action, ref.source, report)
			continue
		}

		original := ref.action.SuggestedAmount
		allowed := original
		if allowed > remaining {
			allowed = floorDailyAmount(remaining)
		}
		if allowed <= 0 {
			markDailyBuyBudgetExhausted(ref.action, ref.source, report)
			continue
		}

		ref.action.SuggestedAmount = allowed
		ref.action.SuggestedWeight = amountToWeight(allowed, report.InvestableAmount)
		remaining -= allowed
		if allowed < original {
			ref.action.Reasons = prependUniqueDailyReason(ref.action.Reasons, fmt.Sprintf("今日总买入预算 %.2f 元，本基金本次压缩到 %.2f 元。", report.DailyBuyBudget, allowed))
		}
	}
	return portfolioActions, candidateActions
}

func collectDailyBuyBudgetActionRefs(portfolioActions []FundDailyAction, candidateActions []FundDailyAction) []fundDailyBudgetActionRef {
	refs := []fundDailyBudgetActionRef{}
	for idx := range portfolioActions {
		if portfolioActions[idx].SuggestedAmount > 0 {
			refs = append(refs, fundDailyBudgetActionRef{
				source: fundDailyBudgetSourcePortfolio,
				action: &portfolioActions[idx],
			})
		}
	}
	for idx := range candidateActions {
		if candidateActions[idx].SuggestedAmount > 0 {
			refs = append(refs, fundDailyBudgetActionRef{
				source: fundDailyBudgetSourceCandidate,
				action: &candidateActions[idx],
			})
		}
	}
	return refs
}

func dailyBuyBudgetPriority(ref fundDailyBudgetActionRef) float64 {
	action := ref.action
	priority := effectiveDailyStrategyScore(*action)*0.65 + float64(action.Score)*0.35
	if action.ActionLevel == "buy" {
		priority += 5
	}
	if ref.source == fundDailyBudgetSourcePortfolio {
		priority += 8
	}
	return priority
}

func markDailyBuyBudgetExhausted(action *FundDailyAction, source string, report FundDailyAdviceReport) {
	action.SuggestedAmount = 0
	action.SuggestedWeight = 0
	if source == fundDailyBudgetSourcePortfolio {
		action.Action = "今日暂不加仓"
		action.ActionLevel = "hold"
		action.Reasons = prependUniqueDailyReason(action.Reasons, fmt.Sprintf("今日总买入预算 %.2f 元已用完，本持有基金暂不加仓。", report.DailyBuyBudget))
		return
	}
	action.Action = "观察，暂不买入"
	action.ActionLevel = "watch"
	action.Reasons = prependUniqueDailyReason(action.Reasons, fmt.Sprintf("今日总买入预算 %.2f 元已用完，本候选基金仅保留观察。", report.DailyBuyBudget))
}

type fundDailyBudgetSignals struct {
	buyableCount        int
	portfolioBuyCount   int
	candidateBuyCount   int
	avgScore            float64
	avgStrategyScore    float64
	avgTrendScore       float64
	avgCorrelationScore float64
	avgDrawdown         float64
	avgStddev           float64
	avgSharp            float64
	currentWeight       float64
}

func summarizeDailyBudgetSignals(refs []fundDailyBudgetActionRef, report FundDailyAdviceReport) fundDailyBudgetSignals {
	signals := fundDailyBudgetSignals{}
	if report.InvestableAmount > 0 {
		signals.currentWeight = report.CurrentAmount / report.InvestableAmount * 100
	}
	for _, ref := range refs {
		action := ref.action
		if action.SuggestedAmount <= 0 {
			continue
		}
		signals.buyableCount++
		if ref.source == fundDailyBudgetSourcePortfolio {
			signals.portfolioBuyCount++
		} else {
			signals.candidateBuyCount++
		}
		signals.avgScore += float64(action.Score)
		signals.avgStrategyScore += effectiveDailyStrategyScore(*action)
		signals.avgTrendScore += action.TrendScore
		signals.avgCorrelationScore += action.CorrelationScore
		signals.avgDrawdown += action.Drawdown
		signals.avgStddev += action.Stddev
		signals.avgSharp += action.Sharp
	}
	if signals.buyableCount == 0 {
		return signals
	}
	count := float64(signals.buyableCount)
	signals.avgScore /= count
	signals.avgStrategyScore /= count
	signals.avgTrendScore /= count
	signals.avgCorrelationScore /= count
	signals.avgDrawdown /= count
	signals.avgStddev /= count
	signals.avgSharp /= count
	return signals
}

func effectiveDailyStrategyScore(action FundDailyAction) float64 {
	if action.StrategyScore > 0 {
		return action.StrategyScore
	}
	return float64(action.Score)
}

func totalPositiveDailyBuyAmount(actionGroups ...[]FundDailyAction) float64 {
	total := 0.0
	for _, actions := range actionGroups {
		for _, action := range actions {
			if action.SuggestedAmount > 0 {
				total += action.SuggestedAmount
			}
		}
	}
	return total
}

func floorDailyAmount(amount float64) float64 {
	if amount < 10 {
		return 0
	}
	return float64(int(amount/10)) * 10
}
