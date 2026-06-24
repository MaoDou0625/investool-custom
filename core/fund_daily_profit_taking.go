package core

import (
	"fmt"
	"sort"
	"time"
)

const (
	fundDailyLocalMaxProfitTakingActions = 2
	fundDailyLocalMinProfitTakingAmount  = 20
	fundDailyLocalMaxProfitTakingAmount  = 120
)

type fundDailyLocalProfitTakingOption struct {
	fund        FundDailyAIFund
	upside      fundDailyUpsideAssessment
	score       float64
	amount      float64
	isExplicit  bool
	action      string
	runup       bool
	profit      bool
	riskControl bool
}

func buildFundDailyLocalProfitTakingActions(portfolio []FundDailyAIFund, signals fundDailyLocalSignals, buyActions []FundDailyAIOutputAction, allowAutomatic bool) []FundDailyAIOutputAction {
	actions, _ := buildFundDailyLocalProfitTakingActionsWithState(portfolio, signals, buyActions, allowAutomatic, emptyFundDailyProfitTakingState(), time.Now())
	return actions
}

func buildFundDailyLocalProfitTakingActionsWithState(
	portfolio []FundDailyAIFund,
	signals fundDailyLocalSignals,
	buyActions []FundDailyAIOutputAction,
	allowAutomatic bool,
	state FundDailyProfitTakingState,
	generatedAt time.Time,
) ([]FundDailyAIOutputAction, FundDailyProfitTakingState) {
	bought := fundDailyLocalActionCodeSet(buyActions)
	options := make([]fundDailyLocalProfitTakingOption, 0, len(portfolio))
	for _, fund := range portfolio {
		if bought[fund.Code] {
			continue
		}
		option, ok := fundDailyLocalProfitTakingOptionForFund(fund, signals)
		if ok {
			if !allowAutomatic && !option.isExplicit {
				continue
			}
			options = append(options, option)
		}
	}
	sort.SliceStable(options, func(i, j int) bool {
		if options[i].score != options[j].score {
			return options[i].score > options[j].score
		}
		return options[i].fund.Code < options[j].fund.Code
	})

	actions := make([]FundDailyAIOutputAction, 0, minInt(len(options), fundDailyLocalMaxProfitTakingActions))
	nextState := state
	for _, option := range options {
		if len(actions) >= fundDailyLocalMaxProfitTakingActions {
			break
		}
		adjusted, updatedState, ok, note := applyFundDailyProfitTakingState(option, nextState, generatedAt)
		if !ok {
			nextState = updatedState
			continue
		}
		nextState = updatedState
		action := fundDailyLocalProfitTakingActionFromOption(adjusted)
		if note != "" {
			action.RiskNote += " " + note
		}
		actions = append(actions, action)
	}
	return actions, nextState
}

func fundDailyLocalProfitTakingOptionForFund(fund FundDailyAIFund, signals fundDailyLocalSignals) (fundDailyLocalProfitTakingOption, bool) {
	if fund.CurrentAmount < fundDailyLocalMinProfitTakingAmount {
		return fundDailyLocalProfitTakingOption{}, false
	}

	explicitSell := fund.SuggestedSellAmount >= fundDailyLocalMinProfitTakingAmount
	explicitFullSell := explicitSell && (fund.ProgramActionLevel == "sell" || fund.ProgramAction == "sell")
	runup := fund.RecentReturns.Month1 >= 12 || fund.RecentReturns.Month3 >= 25 || fund.RecentReturns.Month6 >= 45
	profit := fund.ProfitRatio >= 15 || fund.ProfitAmount >= fundDailyLocalMinProfitTakingAmount
	riskControl := fund.Stddev >= 25 || fund.Drawdown >= 25 || (signals.TechConcentrated && isFundDailyTechExposure(fund)) || hasFundDailyLocalPortfolioTopOverlap(fund, signals)

	upside := assessFundDailyUpsideRoom(fund, signals)
	automaticTrim := profit && riskControl && (runup || upside.allowAutomaticTrim)
	if !explicitSell && !automaticTrim {
		return fundDailyLocalProfitTakingOption{}, false
	}
	if !explicitSell && !upside.allowAutomaticTrim {
		return fundDailyLocalProfitTakingOption{}, false
	}
	if explicitSell && !explicitFullSell && !upside.allowAutomaticTrim {
		return fundDailyLocalProfitTakingOption{}, false
	}

	ratio := upside.trimRatio
	if explicitSell {
		ratio = 0.20
	}
	action := "trim"
	amount := fund.CurrentAmount * ratio
	if explicitSell {
		if explicitFullSell {
			action = "sell"
			amount = minPositive(fund.SuggestedSellAmount, fund.CurrentAmount)
		} else {
			amount = minPositive(fund.SuggestedSellAmount, fund.CurrentAmount*upside.trimRatio, fund.CurrentAmount*0.30)
		}
	}
	if action == "sell" {
		amount = minPositive(amount, fund.CurrentAmount)
	} else {
		amount = minPositive(amount, fundDailyLocalMaxProfitTakingAmount, fund.CurrentAmount*0.30)
	}
	amount = roundDailyAmount(amount)
	if amount < fundDailyLocalMinProfitTakingAmount {
		return fundDailyLocalProfitTakingOption{}, false
	}

	score := fund.ProfitRatio + fund.RecentReturns.Month1*0.7 + fund.RecentReturns.Month3*0.4 + fund.RecentReturns.Month6*0.2
	if explicitSell {
		score += 20
	}
	if riskControl {
		score += 10
	}
	if signals.TechConcentrated && isFundDailyTechExposure(fund) {
		score += 8
	}
	if fund.Stddev > 0 && fund.Stddev <= 18 && fund.Drawdown > 0 && fund.Drawdown <= 18 {
		score -= 8
	}

	return fundDailyLocalProfitTakingOption{
		fund:        fund,
		upside:      upside,
		score:       score,
		amount:      amount,
		isExplicit:  explicitSell,
		action:      action,
		runup:       runup,
		profit:      profit,
		riskControl: riskControl,
	}, true
}

func fundDailyLocalProfitTakingActionFromOption(option fundDailyLocalProfitTakingOption) FundDailyAIOutputAction {
	fund := option.fund
	shareAmount := fundDailyRedeemShareAmount(option.amount, fund)
	riskNote := fmt.Sprintf("当前浮盈 %.1f%% / %.2f 元；近 1/3/6 月收益 %.1f%% / %.1f%% / %.1f%%，回撤 %.1f%%，波动 %.1f%%。", fund.ProfitRatio, fund.ProfitAmount, fund.RecentReturns.Month1, fund.RecentReturns.Month3, fund.RecentReturns.Month6, fund.Drawdown, fund.Stddev)
	if shareAmount > 0 {
		basis := fundDailyRedeemShareBasisText(fund)
		if basis != "" {
			riskNote += fmt.Sprintf(" 减仓操作按 %.2f 份填写，%.2f 元为组合风控估算值，%s。", shareAmount, option.amount, basis)
		} else {
			riskNote += fmt.Sprintf(" 减仓操作按 %.2f 份填写，%.2f 元为组合风控估算值。", shareAmount, option.amount)
		}
	} else {
		riskNote += fmt.Sprintf(" 减仓参考金额 %.2f 元；份额需在交易前按账户最新净值手动折算。", option.amount)
	}
	if option.upside.riskNote != "" {
		riskNote += " " + option.upside.riskNote
	}
	return FundDailyAIOutputAction{
		Code:        fund.Code,
		Name:        fund.Name,
		FundType:    fund.FundType,
		Source:      fundDailyBudgetSourcePortfolio,
		Action:      option.action,
		Amount:      option.amount,
		ShareAmount: shareAmount,
		Reason:      fundDailyLocalProfitTakingReason(option),
		RiskNote:    riskNote,
	}
}

func fundDailyLocalProfitTakingReason(option fundDailyLocalProfitTakingOption) string {
	if option.isExplicit {
		if option.upside.reason != "" {
			return "原始仓位规则已给出减仓信号；" + option.upside.reason
		}
		return "原始仓位规则已给出减仓信号，本地结构化建议保留小比例卖出，先释放风险，避免等到满仓后再处理。"
	}
	if option.upside.reason != "" {
		return option.upside.reason
	}
	if option.riskControl {
		return "已有浮盈且短期涨幅较高，同时组合在同方向暴露或波动偏高，建议小比例止盈，避免利润全部暴露在同一轮波动里。"
	}
	return "已有浮盈且短期涨幅较高，建议小比例止盈，给后续分散买入预留弹性。"
}

func fundDailyLocalActionCodeSet(actions []FundDailyAIOutputAction) map[string]bool {
	codes := map[string]bool{}
	for _, action := range actions {
		if action.Code != "" {
			codes[action.Code] = true
		}
	}
	return codes
}

func mergeFundDailyLocalTrimActions(actions []FundDailyAIOutputAction, trimActions []FundDailyAIOutputAction) []FundDailyAIOutputAction {
	if len(trimActions) == 0 {
		return actions
	}
	merged := append([]FundDailyAIOutputAction(nil), actions...)
	for _, trim := range trimActions {
		replaced := false
		for idx := range merged {
			if merged[idx].Source == fundDailyBudgetSourcePortfolio && merged[idx].Code == trim.Code {
				merged[idx] = trim
				replaced = true
				break
			}
		}
		if !replaced {
			merged = append(merged, trim)
		}
	}
	return merged
}
