package core

import (
	"context"
	"fmt"
	"math"

	"github.com/axiaoxin-com/investool/datacenter/eastmoney"
	"github.com/axiaoxin-com/investool/models"
)

type FundPortfolioAdvice struct {
	Item            models.FundPortfolioItem            `json:"item"`
	Fund            *models.Fund                        `json:"fund,omitempty"`
	HolderStructure eastmoney.FundHolderStructureResult `json:"holder_structure"`
	Score           int                                 `json:"score"`
	Grade           string                              `json:"grade"`
	Action          string                              `json:"action"`
	RiskLevel       string                              `json:"risk_level"`
	ProfitRatio     float64                             `json:"profit_ratio"`
	ProfitAmount    float64                             `json:"profit_amount"`
	Reasons         []string                            `json:"reasons"`
	Warnings        []string                            `json:"warnings"`
}

func EvaluateFundPortfolio(
	ctx context.Context,
	items []models.FundPortfolioItem,
	funds map[string]*models.Fund,
	holderStructures map[string]eastmoney.FundHolderStructureResult,
) []FundPortfolioAdvice {
	results := make([]FundPortfolioAdvice, 0, len(items))
	for _, item := range items {
		results = append(results, EvaluateFundPortfolioItem(ctx, item, funds[item.Code], holderStructures[item.Code]))
	}
	return results
}

func EvaluateFundPortfolioItem(
	ctx context.Context,
	item models.FundPortfolioItem,
	fund *models.Fund,
	holderStructure eastmoney.FundHolderStructureResult,
) FundPortfolioAdvice {
	advice := FundPortfolioAdvice{
		Item:            item,
		Fund:            fund,
		HolderStructure: holderStructure,
		Score:           60,
		Reasons:         []string{},
		Warnings:        []string{},
	}

	if fund == nil {
		advice.Score = 0
		advice.Grade = "无法评估"
		advice.Action = "检查基金代码或稍后重试"
		advice.RiskLevel = "未知"
		advice.Warnings = append(advice.Warnings, "未能获取基金实时详情")
		return advice
	}

	advice.evaluateQuality(ctx, fund)
	advice.evaluateRisk(fund)
	advice.evaluateManager(fund)
	advice.evaluateScale(fund)
	advice.evaluateHolderStructure(holderStructure)
	advice.evaluatePosition(fund)

	advice.Score = clampInt(advice.Score, 0, 100)
	advice.Grade = gradeForScore(advice.Score)
	advice.RiskLevel = riskLevelForFund(fund)
	advice.Action = actionForScore(item.Status, advice.Score)

	if len(advice.Reasons) == 0 {
		advice.Reasons = append(advice.Reasons, "可用数据不足，建议先补充观察")
	}
	return advice
}

func (a FundPortfolioAdvice) ScoreBadgeClass() string {
	switch {
	case a.Score >= 80:
		return "green"
	case a.Score >= 65:
		return "blue"
	case a.Score >= 50:
		return "orange"
	default:
		return "red"
	}
}

func (a FundPortfolioAdvice) ActionBadgeClass() string {
	switch {
	case a.Score >= 80:
		return "green lighten-1"
	case a.Score >= 65:
		return "blue lighten-1"
	case a.Score >= 50:
		return "orange lighten-1"
	default:
		return "red lighten-1"
	}
}

func (a FundPortfolioAdvice) ProfitRatioText() string {
	if a.ProfitRatio == 0 {
		return "--"
	}
	return fmt.Sprintf("%.2f%%", a.ProfitRatio)
}

func (a FundPortfolioAdvice) ProfitAmountText() string {
	if a.ProfitAmount == 0 {
		return "--"
	}
	return fmt.Sprintf("%.2f", a.ProfitAmount)
}

func (a *FundPortfolioAdvice) evaluateQuality(ctx context.Context, fund *models.Fund) {
	if fund.Is4433(ctx) {
		a.addScore(16, "满足 4433 选基规则，长期和中短期排名较强")
	} else {
		a.addScore(-8, "未完全满足 4433 规则，需要结合其他指标观察")
	}

	strongLongTerm := 0
	for _, ratio := range []float64{
		fund.Performance.Year1RankRatio,
		fund.Performance.Year2RankRatio,
		fund.Performance.Year3RankRatio,
		fund.Performance.Year5RankRatio,
	} {
		if ratio > 0 && ratio <= 25 {
			strongLongTerm++
		}
	}
	if strongLongTerm >= 3 {
		a.addScore(8, "1/2/3/5 年排名多数位于同类前 25%")
	} else if strongLongTerm == 0 {
		a.addScore(-8, "长期排名优势不明显")
	}

	if fund.Performance.Month3RankRatio > 0 && fund.Performance.Month6RankRatio > 0 {
		if fund.Performance.Month3RankRatio <= 33.33 && fund.Performance.Month6RankRatio <= 33.33 {
			a.addScore(7, "近 3 月和近 6 月排名保持在同类前 1/3")
		} else if fund.Performance.Month3RankRatio > 60 && fund.Performance.Month6RankRatio > 60 {
			a.addScore(-8, "近 3 月和近 6 月排名均偏后，短期趋势走弱")
		}
	}
}

func (a *FundPortfolioAdvice) evaluateRisk(fund *models.Fund) {
	switch {
	case fund.Sharp.Avg135 >= 1.2:
		a.addScore(8, "1/3/5 年平均夏普较好")
	case fund.Sharp.Avg135 > 0 && fund.Sharp.Avg135 < 0.6:
		a.addScore(-7, "1/3/5 年平均夏普偏低")
	}

	switch {
	case fund.MaxRetracement.Avg135 > 0 && fund.MaxRetracement.Avg135 <= 20:
		a.addScore(5, "平均最大回撤可控")
	case fund.MaxRetracement.Avg135 > 30:
		a.addScore(-9, "平均最大回撤偏高")
	}

	switch {
	case fund.Stddev.Avg135 > 0 && fund.Stddev.Avg135 <= 20:
		a.addScore(4, "平均波动率相对可控")
	case fund.Stddev.Avg135 > 28:
		a.addScore(-6, "平均波动率偏高")
	}
}

func (a *FundPortfolioAdvice) evaluateManager(fund *models.Fund) {
	manageYears := fund.Manager.ManageDays / 365.0
	switch {
	case manageYears >= 5:
		a.addScore(7, fmt.Sprintf("基金经理管理本基金 %.1f 年，任期较稳定", manageYears))
	case manageYears > 0 && manageYears < 2:
		a.addScore(-6, fmt.Sprintf("基金经理管理本基金 %.1f 年，任期偏短", manageYears))
	}

	if fund.Manager.ManageRepay > 0 {
		a.addScore(4, fmt.Sprintf("基金经理任职回报 %.2f%%", fund.Manager.ManageRepay))
	} else if fund.Manager.ManageRepay < 0 {
		a.addScore(-5, fmt.Sprintf("基金经理任职回报 %.2f%%", fund.Manager.ManageRepay))
	}
}

func (a *FundPortfolioAdvice) evaluateScale(fund *models.Fund) {
	scaleYi := fund.NetAssetsScale / 100000000.0
	switch {
	case scaleYi >= 2 && scaleYi <= 80:
		a.addScore(5, fmt.Sprintf("基金规模 %.2f 亿，处于较可管理区间", scaleYi))
	case scaleYi > 0 && scaleYi < 2:
		a.addScore(-6, fmt.Sprintf("基金规模 %.2f 亿，规模偏小", scaleYi))
	case scaleYi > 150:
		a.addScore(-4, fmt.Sprintf("基金规模 %.2f 亿，规模偏大", scaleYi))
	}
}

func (a *FundPortfolioAdvice) evaluateHolderStructure(holderStructure eastmoney.FundHolderStructureResult) {
	if holderStructure.Latest == nil {
		a.Warnings = append(a.Warnings, "暂无持有人结构数据")
		return
	}

	ratio := holderStructure.Latest.InstitutionalHoldingRatio
	switch {
	case ratio >= 35 && ratio <= 75:
		a.addScore(5, fmt.Sprintf("机构持有比例 %.2f%%，有一定机构认可度", ratio))
	case ratio > 90:
		a.Warnings = append(a.Warnings, fmt.Sprintf("机构持有比例 %.2f%%，需关注大额赎回波动", ratio))
	case ratio > 0 && ratio < 10:
		a.addScore(-3, fmt.Sprintf("机构持有比例 %.2f%%，机构参与度较低", ratio))
	}
}

func (a *FundPortfolioAdvice) evaluatePosition(fund *models.Fund) {
	if !a.Item.IsOwned() {
		return
	}
	if a.Item.CostNav <= 0 || fund.UnitNav <= 0 {
		a.Warnings = append(a.Warnings, "未填写成本净值或缺少当前净值，无法估算持仓收益")
		return
	}

	a.ProfitRatio = (fund.UnitNav - a.Item.CostNav) / a.Item.CostNav * 100
	if a.Item.HoldingShares > 0 {
		a.ProfitAmount = (fund.UnitNav - a.Item.CostNav) * a.Item.HoldingShares
	}

	switch {
	case a.ProfitRatio <= -15 && a.Score >= 70:
		a.Reasons = append(a.Reasons, "当前浮亏较大但质量分不低，优先复核仓位和持有周期，避免只按短期亏损决策")
	case a.ProfitRatio >= 25 && a.Score < 65:
		a.Reasons = append(a.Reasons, "已有较高浮盈但质量分一般，可考虑降低后续加仓强度")
	}
}

func (a *FundPortfolioAdvice) addScore(delta int, reason string) {
	a.Score += delta
	a.Reasons = append(a.Reasons, reason)
}

func gradeForScore(score int) string {
	switch {
	case score >= 85:
		return "优秀"
	case score >= 75:
		return "较好"
	case score >= 65:
		return "可观察"
	case score >= 50:
		return "偏弱"
	default:
		return "风险较高"
	}
}

func riskLevelForFund(fund *models.Fund) string {
	riskScore := 0
	if fund.Stddev.Avg135 > 25 {
		riskScore++
	}
	if fund.MaxRetracement.Avg135 > 30 {
		riskScore++
	}
	if fund.Sharp.Avg135 > 0 && fund.Sharp.Avg135 < 0.6 {
		riskScore++
	}
	switch riskScore {
	case 0:
		return "中低"
	case 1:
		return "中等"
	default:
		return "偏高"
	}
}

func actionForScore(status string, score int) string {
	if status == models.FundPortfolioStatusOwned {
		switch {
		case score >= 80:
			return "继续持有，可按目标仓位定投"
		case score >= 65:
			return "继续持有，控制加仓节奏"
		case score >= 50:
			return "观察，暂停主动加仓"
		default:
			return "降低仓位或寻找替代基金"
		}
	}

	switch {
	case score >= 80:
		return "可考虑小额建仓"
	case score >= 65:
		return "继续观察，等待回撤或确认"
	case score >= 50:
		return "暂缓买入"
	default:
		return "不建议纳入"
	}
}

func clampInt(n, min, max int) int {
	return int(math.Max(float64(min), math.Min(float64(max), float64(n))))
}
