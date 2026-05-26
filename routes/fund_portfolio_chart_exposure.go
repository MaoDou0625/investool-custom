package routes

import "github.com/axiaoxin-com/investool/core"

func buildStockConcentration(stocks []core.FundPortfolioStockExposure) fundPortfolioStockConcentration {
	concentration := fundPortfolioStockConcentration{}
	for idx, stock := range stocks {
		if idx == 0 {
			concentration.LargestName = stock.StockName
			concentration.LargestCode = stock.StockCode
			concentration.LargestWeight = stock.Weight
		}
		if idx < 10 {
			concentration.Top10Amount += stock.Amount
			concentration.Top10Weight += stock.Weight
		}
		if idx < 20 {
			concentration.Top20Amount += stock.Amount
			concentration.Top20Weight += stock.Weight
		}
	}
	return concentration
}
