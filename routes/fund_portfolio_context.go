package routes

import (
	"github.com/axiaoxin-com/investool/core"
	"github.com/axiaoxin-com/investool/datacenter/eastmoney"
	"github.com/axiaoxin-com/investool/models"
	"github.com/gin-gonic/gin"
)

type fundPortfolioAnalysisContext struct {
	Portfolio        models.FundPortfolio
	Funds            map[string]*models.Fund
	HolderStructures map[string]eastmoney.FundHolderStructureResult
	ExposureReport   core.FundPortfolioExposureReport
	AllAdvices       []core.FundPortfolioAdvice
	OwnedAdvices     []core.FundPortfolioAdvice
	WatchAdvices     []core.FundPortfolioAdvice
	PageError        string
}

type fundPortfolioAnalysisLoadOptions struct {
	UseLocalFundCache     bool
	QueryHolderStructures bool
}

func loadFundPortfolioAnalysisContext(c *gin.Context, pageErr string) fundPortfolioAnalysisContext {
	return loadFundPortfolioAnalysisContextWithOptions(c, pageErr, fundPortfolioAnalysisLoadOptions{
		QueryHolderStructures: true,
	})
}

func loadFundPortfolioAnalysisContextWithOptions(c *gin.Context, pageErr string, options fundPortfolioAnalysisLoadOptions) fundPortfolioAnalysisContext {
	result := fundPortfolioAnalysisContext{
		Funds:            map[string]*models.Fund{},
		HolderStructures: map[string]eastmoney.FundHolderStructureResult{},
		PageError:        pageErr,
	}

	store := newFundPortfolioStore()
	portfolio, err := store.Load()
	if err != nil && result.PageError == "" {
		result.PageError = err.Error()
	}
	result.Portfolio = portfolio

	if len(portfolio.Items) > 0 && result.PageError == "" {
		funds, searchErr := loadFundPortfolioFunds(c, portfolio.Codes(), options.UseLocalFundCache)
		if searchErr != nil {
			result.PageError = searchErr.Error()
		} else {
			result.Funds = funds
		}
	}

	if options.QueryHolderStructures && len(result.Funds) > 0 && result.PageError == "" {
		result.HolderStructures = queryFundHolderStructures(c, result.Funds)
	}

	result.ExposureReport = buildFundPortfolioExposureReport(c, portfolio, result.Funds, result.PageError)
	result.AllAdvices = core.EvaluateFundPortfolio(c, portfolio.Items, result.Funds, result.HolderStructures)
	result.OwnedAdvices, result.WatchAdvices = splitFundPortfolioAdvices(result.AllAdvices)
	return result
}

func loadFundPortfolioFunds(c *gin.Context, codes []string, useLocalFundCache bool) (map[string]*models.Fund, error) {
	funds := map[string]*models.Fund{}
	missingCodes := []string{}
	if useLocalFundCache {
		localFunds := fundPortfolioLocalFundMap(models.FundAllList)
		for _, code := range codes {
			if fund := localFunds[code]; fund != nil {
				funds[code] = fund
			} else {
				missingCodes = append(missingCodes, code)
			}
		}
	} else {
		missingCodes = codes
	}
	if len(missingCodes) == 0 {
		return funds, nil
	}
	searcher := core.NewSearcher(c)
	fetched, err := searcher.SearchFunds(c, missingCodes)
	if err != nil {
		return funds, err
	}
	for code, fund := range fetched {
		funds[code] = fund
	}
	return funds, nil
}

func fundPortfolioLocalFundMap(funds models.FundList) map[string]*models.Fund {
	result := make(map[string]*models.Fund, len(funds))
	for _, fund := range funds {
		if fund == nil || fund.Code == "" {
			continue
		}
		result[fund.Code] = fund
	}
	return result
}
