package core

import "fmt"

type fundDailyBudgetDecision struct {
	Budget     float64
	MaxBudget  float64
	Confidence float64
	Reasons    []string
}

func maxDailyCandidateBuyBudget(report FundDailyAdviceReport) float64 {
	if report.CashRoom <= 0 || report.InvestableAmount <= 0 {
		return 0
	}
	budget := report.InvestableAmount * report.Config.MaxDailyBuyWeight / 100
	if budget > report.CashRoom {
		budget = report.CashRoom
	}
	return floorDailyAmount(budget)
}

func chooseDailyCandidateBuyBudget(actions []FundDailyAction, report FundDailyAdviceReport) fundDailyBudgetDecision {
	maxBudget := maxDailyCandidateBuyBudget(report)
	decision := fundDailyBudgetDecision{
		MaxBudget: maxBudget,
		Reasons:   []string{},
	}
	if maxBudget <= 0 {
		decision.Reasons = append(decision.Reasons, "今日可用加仓空间不足，候选基金只保留观察。")
		return decision
	}

	signals := summarizeDailyBudgetSignals(actions, report)
	if signals.buyableCount == 0 {
		decision.Reasons = append(decision.Reasons, "候选基金没有形成有效买入信号，今日不安排新增买入。")
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
		fmt.Sprintf("AI预算：最高可买 %.2f 元，数据信心 %.0f%%，今日建议买入 %.2f 元。", maxBudget, decision.Confidence*100, decision.Budget),
		fmt.Sprintf("趋势/质量：候选平均数据评分 %.1f，综合评分 %.1f，趋势分 %.1f。", signals.avgStrategyScore, signals.avgScore, signals.avgTrendScore),
		fmt.Sprintf("风险/分散：平均回撤 %.1f%%，波动 %.1f%%，分散化分 %.1f。", signals.avgDrawdown, signals.avgStddev, signals.avgCorrelationScore),
		fmt.Sprintf("仓位：当前已投入约 %.1f%%，避免一次性把剩余仓位买满。", signals.currentWeight),
	)
	return decision
}

func applyDailyCandidateBuyBudget(actions []FundDailyAction, report FundDailyAdviceReport) []FundDailyAction {
	if len(actions) == 0 {
		return actions
	}

	remaining := report.DailyBuyBudget
	for idx := range actions {
		if actions[idx].SuggestedAmount <= 0 {
			continue
		}
		if remaining <= 0 {
			markDailyCandidateBudgetExhausted(&actions[idx], report)
			continue
		}

		original := actions[idx].SuggestedAmount
		allowed := original
		if allowed > remaining {
			allowed = floorDailyAmount(remaining)
		}
		if allowed <= 0 {
			markDailyCandidateBudgetExhausted(&actions[idx], report)
			continue
		}

		actions[idx].SuggestedAmount = allowed
		actions[idx].SuggestedWeight = amountToWeight(allowed, report.InvestableAmount)
		remaining -= allowed
		if allowed < original {
			actions[idx].Reasons = prependUniqueDailyReason(actions[idx].Reasons, fmt.Sprintf("单日候选买入预算 %.2f 元，本基金本次压缩到 %.2f 元。", report.DailyBuyBudget, allowed))
		}
	}
	return actions
}

func markDailyCandidateBudgetExhausted(action *FundDailyAction, report FundDailyAdviceReport) {
	action.SuggestedAmount = 0
	action.SuggestedWeight = 0
	action.Action = "观察，暂不买入"
	action.ActionLevel = "watch"
	action.Reasons = prependUniqueDailyReason(action.Reasons, fmt.Sprintf("单日候选买入预算 %.2f 元已用完，本基金仅保留观察。", report.DailyBuyBudget))
}

type fundDailyBudgetSignals struct {
	buyableCount        int
	avgScore            float64
	avgStrategyScore    float64
	avgTrendScore       float64
	avgCorrelationScore float64
	avgDrawdown         float64
	avgStddev           float64
	avgSharp            float64
	currentWeight       float64
}

func summarizeDailyBudgetSignals(actions []FundDailyAction, report FundDailyAdviceReport) fundDailyBudgetSignals {
	signals := fundDailyBudgetSignals{}
	if report.InvestableAmount > 0 {
		signals.currentWeight = report.CurrentAmount / report.InvestableAmount * 100
	}
	for _, action := range actions {
		if action.SuggestedAmount <= 0 {
			continue
		}
		signals.buyableCount++
		signals.avgScore += float64(action.Score)
		signals.avgStrategyScore += action.StrategyScore
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

func totalPositiveDailyCandidateAmount(actions []FundDailyAction) float64 {
	total := 0.0
	for _, action := range actions {
		if action.SuggestedAmount > 0 {
			total += action.SuggestedAmount
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
