package core

import (
	"math"
	"sort"
	"strings"
)

func fundDailyIndustryStockTargets(report FundDailyAdviceReport) []fundDailyIndustryStockTarget {
	portfolioActions := report.DecisionPortfolioActions
	if len(portfolioActions) == 0 {
		portfolioActions = report.PortfolioActions
	}
	candidateActions := report.DecisionCandidateActions
	if len(candidateActions) == 0 {
		candidateActions = report.CandidateActions
	}

	combined := map[string]fundDailyIndustryStockTarget{}
	addTargets := func(actions []FundDailyAction, source string) {
		for _, action := range actions {
			fund := fundDailyAIFundFromAction(action, source)
			themes := fundDailyIndustryThemesForFund(fund)
			if len(themes) == 0 || len(action.TopStocks) == 0 {
				continue
			}
			fundWeight := fundDailyIndustryExposureWeight(fund, report)
			if fundWeight <= 0 && source == fundDailyBudgetSourceCandidate {
				fundWeight = amountToWeight(action.SuggestedAmount, report.InvestableAmount)
			}
			for _, stock := range action.TopStocks {
				code, ok := normalizeFundDailyAStockCode(stock.Code)
				if !ok {
					continue
				}
				name := strings.TrimSpace(stock.Name)
				if name == "" {
					name = code
				}
				stockWeight := fundWeight * stock.HoldRatio / 100
				if stockWeight <= 0 {
					stockWeight = stock.HoldRatio / 100
				}
				stockWeight = stockWeight / float64(len(themes))
				for _, theme := range themes {
					key := code + "|" + theme
					current := combined[key]
					current.Code = code
					current.Name = name
					current.Theme = theme
					current.Weight += stockWeight
					combined[key] = current
				}
			}
		}
	}
	addTargets(portfolioActions, fundDailyBudgetSourcePortfolio)
	addTargets(candidateActions, fundDailyBudgetSourceCandidate)

	targets := make([]fundDailyIndustryStockTarget, 0, len(combined))
	for _, target := range combined {
		targets = append(targets, target)
	}
	sort.SliceStable(targets, func(i, j int) bool {
		if math.Abs(targets[i].Weight-targets[j].Weight) > 0.0001 {
			return targets[i].Weight > targets[j].Weight
		}
		if targets[i].Code != targets[j].Code {
			return targets[i].Code < targets[j].Code
		}
		return targets[i].Theme < targets[j].Theme
	})
	return targets
}

func fundDailyIndustrySelectedStockCodes(targets []fundDailyIndustryStockTarget, limit int) []string {
	if limit <= 0 {
		return nil
	}
	weights := map[string]float64{}
	for _, target := range targets {
		weights[target.Code] += target.Weight
	}
	codes := make([]string, 0, len(weights))
	for code := range weights {
		codes = append(codes, code)
	}
	sort.SliceStable(codes, func(i, j int) bool {
		if math.Abs(weights[codes[i]]-weights[codes[j]]) > 0.0001 {
			return weights[codes[i]] > weights[codes[j]]
		}
		return codes[i] < codes[j]
	})
	if len(codes) > limit {
		codes = codes[:limit]
	}
	return codes
}

func normalizeFundDailyAStockCode(raw string) (string, bool) {
	code := strings.ToUpper(strings.TrimSpace(raw))
	code = strings.TrimPrefix(code, "SH")
	code = strings.TrimPrefix(code, "SZ")
	code = strings.TrimSuffix(code, ".SH")
	code = strings.TrimSuffix(code, ".SZ")
	if len(code) != 6 {
		return "", false
	}
	for _, ch := range code {
		if ch < '0' || ch > '9' {
			return "", false
		}
	}
	if code[0] != '0' && code[0] != '3' && code[0] != '6' {
		return "", false
	}
	return code, true
}

func fundDailyEastMoneySecuCode(code string) string {
	if strings.HasPrefix(code, "6") {
		return code + ".SH"
	}
	return code + ".SZ"
}
