package core

import (
	"fmt"
	"sort"
	"strings"
)

const (
	fundDailyLocalMaxRebalanceBuys   = 2
	fundDailyLocalNewFundScoreMargin = 8
)

type fundDailyLocalBuyOption struct {
	fund            FundDailyAIFund
	source          string
	role            string
	score           float64
	diversifyScore  float64
	topStockOverlap float64
	sameRoleOverlap bool
}

func buildFundDailyLocalRebalanceBuyActions(contextData FundDailyAIContext, candidateScores []fundDailyLocalCandidateScore, budget float64, signals fundDailyLocalSignals) ([]FundDailyAIOutputAction, float64) {
	if budget <= 0 {
		return nil, 0
	}

	portfolioOptions := buildFundDailyLocalPortfolioBuyOptions(contextData.Portfolio, signals)
	candidateOptions := buildFundDailyLocalCandidateBuyOptions(candidateScores, contextData.Portfolio, portfolioOptions, signals)
	options := append(portfolioOptions, candidateOptions...)
	sort.SliceStable(options, func(i, j int) bool {
		if options[i].score != options[j].score {
			return options[i].score > options[j].score
		}
		if options[i].source != options[j].source {
			return options[i].source == fundDailyBudgetSourcePortfolio
		}
		return options[i].fund.Code < options[j].fund.Code
	})

	actions := []FundDailyAIOutputAction{}
	remaining := budget
	selectedRoles := map[string]bool{}
	for _, option := range options {
		if remaining < 10 || len(actions) >= fundDailyLocalMaxRebalanceBuys {
			break
		}
		if selectedRoles[option.role] && len(actions) > 0 {
			continue
		}
		amount := 60.0
		if budget >= 180 {
			amount = 80
		}
		amount = minPositive(amount, option.fund.SuggestedBuyCeiling, remaining)
		amount = roundDailyAmount(amount)
		if amount <= 0 {
			continue
		}
		actions = append(actions, fundDailyLocalBuyActionFromOption(option, amount))
		selectedRoles[option.role] = true
		remaining -= amount
	}
	return actions, budget - remaining
}

func buildFundDailyLocalPortfolioBuyOptions(portfolio []FundDailyAIFund, signals fundDailyLocalSignals) []fundDailyLocalBuyOption {
	options := []fundDailyLocalBuyOption{}
	for _, fund := range portfolio {
		if !fund.CanSubscribe || fund.SuggestedBuyCeiling <= 0 || fund.CurrentAmount <= 0 {
			continue
		}
		if fund.CurrentWeight >= 30 {
			continue
		}
		if fund.Drawdown >= 30 || fund.Stddev >= 30 {
			continue
		}
		if signals.TechConcentrated && isFundDailyTechExposure(fund) {
			continue
		}
		option := fundDailyLocalBuyOptionForFund(fund, fundDailyBudgetSourcePortfolio, portfolio)
		option.score += 10
		if fund.CurrentWeight > 0 && fund.CurrentWeight <= 10 {
			option.score += 6
		}
		if fund.RecentReturns.Month1 >= 15 || fund.RecentReturns.Month3 >= 35 || fund.RecentReturns.Month6 >= 60 {
			option.score -= 10
		}
		if option.score < 62 {
			continue
		}
		options = append(options, option)
	}
	return options
}

func buildFundDailyLocalCandidateBuyOptions(candidateScores []fundDailyLocalCandidateScore, portfolio []FundDailyAIFund, portfolioOptions []fundDailyLocalBuyOption, signals fundDailyLocalSignals) []fundDailyLocalBuyOption {
	options := []fundDailyLocalBuyOption{}
	for _, item := range candidateScores {
		option := fundDailyLocalBuyOptionForFund(item.fund, fundDailyBudgetSourceCandidate, portfolio)
		option.score = option.score*0.60 + item.score*0.40
		if signals.TechConcentrated && isFundDailyTechExposure(item.fund) {
			option.score -= 8
		}
		if existing, ok := fundDailyLocalExistingAlternative(option, portfolioOptions); ok {
			if existing.score+fundDailyLocalNewFundScoreMargin >= option.score {
				continue
			}
		}
		if option.score < 65 {
			continue
		}
		options = append(options, option)
	}
	return options
}

func fundDailyLocalBuyOptionForFund(fund FundDailyAIFund, source string, portfolio []FundDailyAIFund) fundDailyLocalBuyOption {
	diversifyScore, topStockOverlap, sameRoleOverlap := fundDailyLocalPairwiseDiversification(fund, portfolio)
	score := fund.StrategyScore*0.45 + float64(fund.Score)*0.30 + diversifyScore
	if fund.NetAssetsScaleYi >= 1 {
		score += 4
	}
	if fund.Stddev > 0 && fund.Stddev <= 18 {
		score += 4
	}
	if fund.Drawdown > 0 && fund.Drawdown <= 18 {
		score += 4
	}
	if fund.Stddev >= 25 || fund.Drawdown >= 25 {
		score -= 8
	}
	if fund.RecentReturns.Month1 >= 20 || fund.RecentReturns.Month3 >= 40 || fund.RecentReturns.Month6 >= 70 {
		score -= 10
	}
	return fundDailyLocalBuyOption{
		fund:            fund,
		source:          source,
		role:            fundDailyLocalDiversificationRole(fund),
		score:           score,
		diversifyScore:  diversifyScore,
		topStockOverlap: topStockOverlap,
		sameRoleOverlap: sameRoleOverlap,
	}
}

func fundDailyLocalPairwiseDiversification(fund FundDailyAIFund, portfolio []FundDailyAIFund) (float64, float64, bool) {
	role := fundDailyLocalDiversificationRole(fund)
	topStocks := fundDailyLocalTopStockMap(fund)
	maxTopStockOverlap := 0.0
	sameRoleOverlap := false
	for _, peer := range portfolio {
		if peer.Code == fund.Code || peer.CurrentAmount <= 0 {
			continue
		}
		if role != "" && role == fundDailyLocalDiversificationRole(peer) {
			sameRoleOverlap = true
		}
		peerStocks := fundDailyLocalTopStockMap(peer)
		overlap := 0.0
		for name, weight := range topStocks {
			if peerWeight, ok := peerStocks[name]; ok {
				overlap += minPositive(weight, peerWeight)
			}
		}
		if overlap > maxTopStockOverlap {
			maxTopStockOverlap = overlap
		}
	}

	score := 8.0
	switch {
	case maxTopStockOverlap >= 12:
		score -= 20
	case maxTopStockOverlap >= 6:
		score -= 12
	case maxTopStockOverlap >= 3:
		score -= 6
	}
	if sameRoleOverlap {
		score -= 6
	}
	if maxTopStockOverlap == 0 && !sameRoleOverlap {
		score += 6
	}
	return score, maxTopStockOverlap, sameRoleOverlap
}

func fundDailyLocalTopStockMap(fund FundDailyAIFund) map[string]float64 {
	stocks := map[string]float64{}
	for _, stock := range fund.TopStocks {
		name := strings.TrimSpace(stock.Name)
		if name == "" || stock.HoldRatio < 3 {
			continue
		}
		stocks[name] = stock.HoldRatio
	}
	return stocks
}

func fundDailyLocalExistingAlternative(candidate fundDailyLocalBuyOption, portfolioOptions []fundDailyLocalBuyOption) (fundDailyLocalBuyOption, bool) {
	var best fundDailyLocalBuyOption
	found := false
	for _, option := range portfolioOptions {
		if option.role != candidate.role {
			continue
		}
		if !found || option.score > best.score {
			best = option
			found = true
		}
	}
	return best, found
}

func fundDailyLocalBuyActionFromOption(option fundDailyLocalBuyOption, amount float64) FundDailyAIOutputAction {
	fund := option.fund
	return FundDailyAIOutputAction{
		Code:     fund.Code,
		Name:     fund.Name,
		FundType: fund.FundType,
		Source:   option.source,
		Action:   "buy",
		Amount:   amount,
		Reason:   fundDailyLocalRebalanceBuyReason(option),
		RiskNote: fmt.Sprintf("回撤 %.1f%%，波动 %.1f%%，近 1/3/6 月收益 %.1f%% / %.1f%% / %.1f%%。", fund.Drawdown, fund.Stddev, fund.RecentReturns.Month1, fund.RecentReturns.Month3, fund.RecentReturns.Month6),
	}
}

func fundDailyLocalRebalanceBuyReason(option fundDailyLocalBuyOption) string {
	if option.source == fundDailyBudgetSourcePortfolio {
		return "已持有且与其他已持基金的单基金对比重叠较低，优先增仓已有分散仓位，避免为相同分散目的新增基金。"
	}
	if option.diversifyScore > 8 {
		return "与已持有基金逐一比较后重叠较低，且没有相同角色的已持小额仓位，可小额新增做分散。"
	}
	return fundDailyLocalBuyReason(option.fund)
}

func fundDailyLocalDiversificationRole(fund FundDailyAIFund) string {
	text := strings.ToUpper(fund.Name + " " + fund.StrategyTheme + " " + fund.FundType + " " + fund.IndexName)
	switch {
	case fundDailyLocalTextContainsAny(text, "黄金", "贵金属", "黃金", "GOLD"):
		return "gold"
	case fundDailyLocalTextContainsAny(text, "偏债", "债券", "纯债", "固收", "风险预算", "BOND"):
		return "bond-balanced"
	case fundDailyLocalTextContainsAny(text, "量化", "多因子", "QUANT"):
		return "quant"
	case fundDailyLocalTextContainsAny(text, "海外", "纳斯达克", "QDII", "NASDAQ"):
		return "overseas"
	case isFundDailyTechExposure(fund):
		return "tech"
	case fundDailyLocalTextContainsAny(text, "稳健", "均衡", "分散", "低波动", "BALANCED", "DIVERS"):
		return "low-volatility-diversifier"
	case fund.Drawdown > 0 && fund.Drawdown <= 18 && fund.Stddev > 0 && fund.Stddev <= 18:
		return "low-volatility-diversifier"
	default:
		if strings.TrimSpace(fund.FundType) != "" {
			return "type:" + strings.TrimSpace(fund.FundType)
		}
		return "unknown"
	}
}

func fundDailyLocalTextContainsAny(text string, keywords ...string) bool {
	for _, keyword := range keywords {
		if strings.Contains(text, strings.ToUpper(keyword)) {
			return true
		}
	}
	return false
}

func mergeFundDailyLocalBuyActions(actions []FundDailyAIOutputAction, buyActions []FundDailyAIOutputAction) []FundDailyAIOutputAction {
	if len(buyActions) == 0 {
		return actions
	}
	merged := append([]FundDailyAIOutputAction(nil), actions...)
	for _, buy := range buyActions {
		if buy.Source != fundDailyBudgetSourcePortfolio {
			merged = append(merged, buy)
			continue
		}
		replaced := false
		for idx := range merged {
			if merged[idx].Source == fundDailyBudgetSourcePortfolio && merged[idx].Code == buy.Code {
				merged[idx] = buy
				replaced = true
				break
			}
		}
		if !replaced {
			merged = append(merged, buy)
		}
	}
	return merged
}
