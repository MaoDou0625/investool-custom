package routes

import (
	"encoding/json"
	"html/template"

	"github.com/axiaoxin-com/investool/core"
)

type fundPortfolioChartData struct {
	Themes   []fundPortfolioThemeChartPoint `json:"themes"`
	ETFLinks []fundPortfolioETFChartLink    `json:"etfLinks"`
	Stocks   []fundPortfolioStockChartPoint `json:"stocks"`
}

type fundPortfolioThemeChartPoint struct {
	Name    string  `json:"name"`
	Amount  float64 `json:"amount"`
	Weight  float64 `json:"weight"`
	Sources string  `json:"sources"`
}

type fundPortfolioETFChartLink struct {
	SourceName string  `json:"sourceName"`
	TargetName string  `json:"targetName"`
	Amount     float64 `json:"amount"`
	Weight     float64 `json:"weight"`
	Status     string  `json:"status"`
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

func buildFundPortfolioChartDataJSON(report core.FundPortfolioExposureReport) template.JS {
	payload := fundPortfolioChartData{}
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
	}
	for _, lookThrough := range report.ETFLookThroughs {
		if lookThrough.CurrentWeight <= 0 {
			continue
		}
		payload.ETFLinks = append(payload.ETFLinks, fundPortfolioETFChartLink{
			SourceName: lookThrough.FundCode + " " + lookThrough.FundName,
			TargetName: lookThrough.TargetETFCode + " " + lookThrough.TargetETFName,
			Amount:     lookThrough.CurrentAmount,
			Weight:     lookThrough.CurrentWeight,
			Status:     lookThrough.Status,
		})
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

	raw, err := json.Marshal(payload)
	if err != nil {
		return template.JS("{}")
	}
	return template.JS(raw)
}
