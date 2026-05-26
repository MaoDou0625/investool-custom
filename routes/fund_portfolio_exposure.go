package routes

import (
	"fmt"

	"github.com/axiaoxin-com/investool/core"
	"github.com/axiaoxin-com/investool/models"
	"github.com/gin-gonic/gin"
)

func buildFundPortfolioExposureReport(
	c *gin.Context,
	portfolio models.FundPortfolio,
	funds map[string]*models.Fund,
	pageErr string,
) core.FundPortfolioExposureReport {
	if pageErr != "" || len(funds) == 0 {
		return core.FundPortfolioExposureReport{}
	}

	targetFunds := map[string]*models.Fund{}
	warnings := []string{}
	targetCodes := core.FundPortfolioTargetETFCodes(portfolio.Items, funds)
	if len(targetCodes) > 0 {
		searcher := core.NewSearcher(c)
		var err error
		targetFunds, err = searcher.SearchFunds(c, targetCodes)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("目标ETF详情查询失败：%v", err))
		}
	}

	report := core.BuildFundPortfolioExposureReport(portfolio.Items, funds, targetFunds)
	report.Warnings = append(warnings, report.Warnings...)
	return report
}
