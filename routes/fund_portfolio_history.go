package routes

import (
	"fmt"
	"time"

	"github.com/axiaoxin-com/investool/core"
	"github.com/axiaoxin-com/investool/models"
	"github.com/spf13/viper"
)

const defaultFundPortfolioHistoryFilename = "./fund_portfolio_history.json"

func loadAndUpdateFundPortfolioHistory(advices []core.FundPortfolioAdvice) (models.FundPortfolioHistory, []string) {
	warnings := []string{}
	store := models.NewFundPortfolioHistoryStore(fundPortfolioHistoryFilename())
	snapshot, ok := buildFundPortfolioSnapshot(advices, time.Now())
	if ok {
		if err := store.UpsertSnapshot(snapshot); err != nil {
			warnings = append(warnings, fmt.Sprintf("组合历史快照保存失败：%v", err))
		}
	}
	history, err := store.Load()
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("组合历史快照读取失败：%v", err))
	}
	return history, warnings
}

func buildFundPortfolioSnapshot(advices []core.FundPortfolioAdvice, now time.Time) (models.FundPortfolioSnapshot, bool) {
	snapshot := models.FundPortfolioSnapshot{
		Date:      now.Format("2006-01-02"),
		Timestamp: now.Format("2006-01-02 15:04:05"),
		Items:     []models.FundPortfolioSnapshotItem{},
	}
	for _, advice := range advices {
		if !advice.Item.IsOwned() || !advice.HasPosition {
			continue
		}
		snapshot.TotalCurrentAmount += advice.CurrentAmount
		snapshot.CostAmount += advice.CostAmount
		snapshot.Items = append(snapshot.Items, models.FundPortfolioSnapshotItem{
			Code:          advice.Item.Code,
			Name:          fundNameForAdvice(advice),
			UnitNav:       fundUnitNavForAdvice(advice),
			CurrentAmount: advice.CurrentAmount,
			CurrentWeight: advice.CurrentWeight,
			ProfitAmount:  advice.ProfitAmount,
			ProfitRatio:   advice.ProfitRatio,
		})
	}
	if snapshot.TotalCurrentAmount <= 0 || len(snapshot.Items) == 0 {
		return snapshot, false
	}
	snapshot.ProfitAmount = snapshot.TotalCurrentAmount - snapshot.CostAmount
	if snapshot.CostAmount > 0 {
		snapshot.ProfitRatio = snapshot.ProfitAmount / snapshot.CostAmount * 100
	}
	return snapshot, true
}

func fundNameForAdvice(advice core.FundPortfolioAdvice) string {
	if advice.Fund != nil && advice.Fund.Name != "" {
		return advice.Fund.Name
	}
	return advice.Item.Code
}

func fundUnitNavForAdvice(advice core.FundPortfolioAdvice) float64 {
	if advice.Fund == nil {
		return 0
	}
	return advice.Fund.UnitNav
}

func fundPortfolioHistoryFilename() string {
	filename := viper.GetString("fund_portfolio.history_filename")
	if filename == "" {
		return defaultFundPortfolioHistoryFilename
	}
	return filename
}
