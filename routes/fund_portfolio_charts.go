package routes

import (
	"encoding/json"
	"html/template"

	"github.com/axiaoxin-com/investool/core"
	"github.com/axiaoxin-com/investool/models"
)

type fundPortfolioChartData struct {
	Themes             []fundPortfolioThemeChartPoint        `json:"themes"`
	ThemeSources       []fundPortfolioThemeSourceChartPoint  `json:"themeSources"`
	Stocks             []fundPortfolioStockChartPoint        `json:"stocks"`
	StockConcentration fundPortfolioStockConcentration       `json:"stockConcentration"`
	RiskReturns        []fundPortfolioRiskReturnChartPoint   `json:"riskReturns"`
	History            []fundPortfolioHistoryChartPoint      `json:"history"`
	Correlation        fundPortfolioCorrelationChartData     `json:"correlation"`
	CorrelationRefresh fundPortfolioCorrelationRefreshData   `json:"correlationRefresh"`
	Comparisons        []fundPortfolioComparisonChartPoint   `json:"comparisons"`
	ComparisonMetrics  []fundPortfolioComparisonMetricConfig `json:"comparisonMetrics"`
}

type fundPortfolioThemeChartPoint struct {
	Name    string  `json:"name"`
	Amount  float64 `json:"amount"`
	Weight  float64 `json:"weight"`
	Sources string  `json:"sources"`
}

type fundPortfolioThemeSourceChartPoint struct {
	Theme  string  `json:"theme"`
	Source string  `json:"source"`
	Amount float64 `json:"amount"`
	Weight float64 `json:"weight"`
}

type fundPortfolioStockChartPoint struct {
	Name     string  `json:"name"`
	Code     string  `json:"code"`
	Theme    string  `json:"theme"`
	Industry string  `json:"industry"`
	Amount   float64 `json:"amount"`
	Weight   float64 `json:"weight"`
	Source   string  `json:"source"`
}

type fundPortfolioStockConcentration struct {
	Top10Amount   float64 `json:"top10Amount"`
	Top10Weight   float64 `json:"top10Weight"`
	Top20Amount   float64 `json:"top20Amount"`
	Top20Weight   float64 `json:"top20Weight"`
	LargestName   string  `json:"largestName"`
	LargestCode   string  `json:"largestCode"`
	LargestWeight float64 `json:"largestWeight"`
}

type fundPortfolioRiskReturnChartPoint struct {
	Code           string  `json:"code"`
	Name           string  `json:"name"`
	Status         string  `json:"status"`
	Action         string  `json:"action"`
	Score          int     `json:"score"`
	CurrentAmount  float64 `json:"currentAmount"`
	CurrentWeight  float64 `json:"currentWeight"`
	ExpectedReturn float64 `json:"expectedReturn"`
	ReturnLabel    string  `json:"returnLabel"`
	Risk           float64 `json:"risk"`
	Drawdown       float64 `json:"drawdown"`
	Stddev         float64 `json:"stddev"`
}

type fundPortfolioHistoryChartPoint struct {
	Date        string  `json:"date"`
	Amount      float64 `json:"amount"`
	ProfitRatio float64 `json:"profitRatio"`
	Profit      float64 `json:"profit"`
}

type fundPortfolioCorrelationChartData struct {
	Labels []string                               `json:"labels"`
	Points []fundPortfolioCorrelationHeatmapPoint `json:"points"`
}

type fundPortfolioCorrelationHeatmapPoint struct {
	X     int     `json:"x"`
	Y     int     `json:"y"`
	Value float64 `json:"value"`
}

type fundPortfolioComparisonMetricConfig struct {
	Name string `json:"name"`
	Max  int    `json:"max"`
}

type fundPortfolioComparisonChartPoint struct {
	Code   string    `json:"code"`
	Name   string    `json:"name"`
	Status string    `json:"status"`
	Values []float64 `json:"values"`
}

func buildFundPortfolioChartDataJSON(
	report core.FundPortfolioExposureReport,
	advices []core.FundPortfolioAdvice,
	history models.FundPortfolioHistory,
	correlationSources []fundPortfolioCorrelationSource,
	correlationRefresh fundPortfolioCorrelationRefreshData,
) template.JS {
	payload := fundPortfolioChartData{
		StockConcentration: buildStockConcentration(report.StockExposures),
		History:            buildHistoryChartPoints(history),
		Correlation:        buildCorrelationChartData(correlationSources),
		CorrelationRefresh: correlationRefresh,
		ComparisonMetrics: []fundPortfolioComparisonMetricConfig{
			{Name: "评分", Max: 100},
			{Name: "预期收益", Max: 100},
			{Name: "夏普", Max: 100},
			{Name: "回撤控制", Max: 100},
			{Name: "机构持有", Max: 100},
			{Name: "规模适中", Max: 100},
		},
	}
	for _, exposure := range report.ThemeExposures {
		if exposure.Weight <= 0 {
			continue
		}
		payload.Themes = append(payload.Themes, fundPortfolioThemeChartPoint{
			Name:    exposure.Theme,
			Amount:  exposure.Amount,
			Weight:  exposure.Weight,
			Sources: exposure.SourceFunds,
		})
		for _, source := range exposure.SourceBreakdowns {
			if source.Weight <= 0 {
				continue
			}
			payload.ThemeSources = append(payload.ThemeSources, fundPortfolioThemeSourceChartPoint{
				Theme:  exposure.Theme,
				Source: source.Name,
				Amount: source.Amount,
				Weight: source.Weight,
			})
		}
	}
	for _, exposure := range report.StockExposures {
		if exposure.Weight <= 0 {
			continue
		}
		payload.Stocks = append(payload.Stocks, fundPortfolioStockChartPoint{
			Name:     exposure.StockName,
			Code:     exposure.StockCode,
			Theme:    exposure.Theme,
			Industry: exposure.Industry,
			Amount:   exposure.Amount,
			Weight:   exposure.Weight,
			Source:   exposure.ExposureSummary,
		})
	}
	payload.RiskReturns = buildRiskReturnChartPoints(advices)
	payload.Comparisons = buildComparisonChartPoints(advices)

	raw, err := json.Marshal(payload)
	if err != nil {
		return template.JS("{}")
	}
	return template.JS(raw)
}
