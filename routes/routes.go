// 在这个文件中注册 URL handler

package routes

import "github.com/gin-gonic/gin"

// Routes 注册 API URL 路由
func Routes(app *gin.Engine) {
	app.GET("/", FundIndex)
	app.GET("/stock", StockIndex)
	app.POST("/selector", StockSelector)
	app.POST("/checker", StockChecker)
	app.GET("/fund", FundIndex)
	app.GET("/fund/filter", FundFilter)
	app.POST("/fund/check", FundCheck)
	app.GET("/fund/portfolio", FundPortfolioPage)
	app.GET("/fund/portfolio/tiantian", FundPortfolioTianTianLogin)
	app.GET("/fund/portfolio/tiantian/open", FundPortfolioTianTianOpenDefaultBrowser)
	app.POST("/fund/portfolio/tiantian/import", FundPortfolioTianTianImportText)
	app.POST("/fund/portfolio/tiantian/import/xlsx", FundPortfolioTianTianImportXLSX)
	app.GET("/fund/portfolio/tiantian/continue", FundPortfolioTianTianContinue)
	app.POST("/fund/portfolio/correlation/refresh", FundPortfolioCorrelationRefresh)
	app.POST("/fund/portfolio/recognize", FundPortfolioRecognizeScreenshot)
	app.POST("/fund/portfolio/save", FundPortfolioSave)
	app.POST("/fund/portfolio/delete", FundPortfolioDelete)
	app.GET("/about", About)
	app.GET("/comment", Comment)
	app.GET("/fund/similarity", FundSimilarity)
	app.GET("/materials", Materials)
	app.POST("/fund/query_by_stock", QueryFundByStock)
	app.GET("/fund/managers", FundManagers)
}
