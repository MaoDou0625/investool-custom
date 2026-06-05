package routes

import (
	"github.com/axiaoxin-com/investool/core"
	"github.com/axiaoxin-com/investool/models"
)

type fundPortfolioRiskReturnChartGroups struct {
	Owned []fundPortfolioRiskReturnChartPoint `json:"owned"`
	Watch []fundPortfolioRiskReturnChartPoint `json:"watch"`
	All   []fundPortfolioRiskReturnChartPoint `json:"all"`
}

type fundPortfolioCorrelationChartGroups struct {
	Owned fundPortfolioCorrelationChartData `json:"owned"`
	All   fundPortfolioCorrelationChartData `json:"all"`
}

type fundPortfolioCorrelationRefreshGroups struct {
	Owned fundPortfolioCorrelationRefreshData `json:"owned"`
	All   fundPortfolioCorrelationRefreshData `json:"all"`
}

func buildGroupedFundPortfolioChartData(
	report core.FundPortfolioExposureReport,
	ownedAdvices []core.FundPortfolioAdvice,
	watchAdvices []core.FundPortfolioAdvice,
	allAdvices []core.FundPortfolioAdvice,
	history models.FundPortfolioHistory,
	ownedCorrelationSources []fundPortfolioCorrelationSource,
	allCorrelationSources []fundPortfolioCorrelationSource,
	ownedCorrelationRefresh fundPortfolioCorrelationRefreshData,
	allCorrelationRefresh fundPortfolioCorrelationRefreshData,
) fundPortfolioChartData {
	payload := buildFundPortfolioChartData(report, ownedAdvices, history, ownedCorrelationSources, ownedCorrelationRefresh)
	payload.RiskReturnGroups = buildRiskReturnChartGroups(ownedAdvices, watchAdvices, allAdvices)
	payload.RiskReturns = payload.RiskReturnGroups.Owned
	payload.CorrelationGroups = fundPortfolioCorrelationChartGroups{
		Owned: buildCorrelationChartData(ownedCorrelationSources),
		All:   buildCorrelationChartData(allCorrelationSources),
	}
	payload.Correlation = payload.CorrelationGroups.Owned
	payload.CorrelationRefreshGroups = fundPortfolioCorrelationRefreshGroups{
		Owned: ownedCorrelationRefresh,
		All:   allCorrelationRefresh,
	}
	payload.CorrelationRefresh = payload.CorrelationRefreshGroups.Owned
	payload.Comparisons = buildComparisonChartPoints(allAdvices)
	return payload
}

func buildRiskReturnChartGroups(
	ownedAdvices []core.FundPortfolioAdvice,
	watchAdvices []core.FundPortfolioAdvice,
	allAdvices []core.FundPortfolioAdvice,
) fundPortfolioRiskReturnChartGroups {
	return fundPortfolioRiskReturnChartGroups{
		Owned: buildRiskReturnChartPoints(ownedAdvices),
		Watch: buildRiskReturnChartPoints(watchAdvices),
		All:   buildRiskReturnChartPoints(allAdvices),
	}
}
