package routes

import (
	"net/http"
	"net/url"

	"github.com/axiaoxin-com/investool/core"
	"github.com/axiaoxin-com/investool/datacenter/eastmoney"
	"github.com/axiaoxin-com/investool/models"
	"github.com/axiaoxin-com/investool/version"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

const defaultFundPortfolioFilename = "./fund_portfolio.json"

type fundPortfolioViewData struct {
	Env              string
	HostURL          string
	Version          string
	PageTitle        string
	Error            string
	Message          string
	Portfolio        models.FundPortfolio
	OwnedAdvices     []core.FundPortfolioAdvice
	WatchAdvices     []core.FundPortfolioAdvice
	AllAdvices       []core.FundPortfolioAdvice
	PortfolioFile    string
	FundPortfolioURL string
}

type fundPortfolioDeleteParam struct {
	Code string `json:"code" form:"code"`
}

func FundPortfolioPage(c *gin.Context) {
	renderFundPortfolioPage(c, c.Query("message"), c.Query("error"))
}

func FundPortfolioSave(c *gin.Context) {
	item := models.FundPortfolioItem{}
	if err := c.ShouldBind(&item); err != nil {
		redirectFundPortfolio(c, "", err.Error())
		return
	}

	if err := newFundPortfolioStore().Upsert(item); err != nil {
		redirectFundPortfolio(c, "", err.Error())
		return
	}
	redirectFundPortfolio(c, "基金已保存", "")
}

func FundPortfolioDelete(c *gin.Context) {
	p := fundPortfolioDeleteParam{}
	if err := c.ShouldBind(&p); err != nil {
		redirectFundPortfolio(c, "", err.Error())
		return
	}
	if err := newFundPortfolioStore().Delete(p.Code); err != nil {
		redirectFundPortfolio(c, "", err.Error())
		return
	}
	redirectFundPortfolio(c, "基金已删除", "")
}

func renderFundPortfolioPage(c *gin.Context, message string, pageErr string) {
	store := newFundPortfolioStore()
	portfolio, err := store.Load()
	if err != nil && pageErr == "" {
		pageErr = err.Error()
	}

	funds := map[string]*models.Fund{}
	holderStructures := map[string]eastmoney.FundHolderStructureResult{}
	if len(portfolio.Items) > 0 && pageErr == "" {
		searcher := core.NewSearcher(c)
		var searchErr error
		funds, searchErr = searcher.SearchFunds(c, portfolio.Codes())
		if searchErr != nil {
			pageErr = searchErr.Error()
		}
	}

	if len(funds) > 0 && pageErr == "" {
		holderStructures = queryFundHolderStructures(c, funds)
	}

	allAdvices := core.EvaluateFundPortfolio(c, portfolio.Items, funds, holderStructures)
	ownedAdvices, watchAdvices := splitFundPortfolioAdvices(allAdvices)

	data := fundPortfolioViewData{
		Env:              viper.GetString("env"),
		HostURL:          viper.GetString("server.host_url"),
		Version:          version.Version,
		PageTitle:        "InvesTool | 我的基金",
		Error:            pageErr,
		Message:          message,
		Portfolio:        portfolio,
		OwnedAdvices:     ownedAdvices,
		WatchAdvices:     watchAdvices,
		AllAdvices:       allAdvices,
		PortfolioFile:    fundPortfolioFilename(),
		FundPortfolioURL: viper.GetString("server.host_url") + "/fund/portfolio",
	}
	c.HTML(http.StatusOK, "fund_portfolio.html", data)
}

func newFundPortfolioStore() *models.FundPortfolioStore {
	return models.NewFundPortfolioStore(fundPortfolioFilename())
}

func fundPortfolioFilename() string {
	filename := viper.GetString("fund_portfolio.filename")
	if filename == "" {
		return defaultFundPortfolioFilename
	}
	return filename
}

func redirectFundPortfolio(c *gin.Context, message string, err string) {
	values := url.Values{}
	if message != "" {
		values.Set("message", message)
	}
	if err != "" {
		values.Set("error", err)
	}

	target := viper.GetString("server.host_url") + "/fund/portfolio"
	if encoded := values.Encode(); encoded != "" {
		target += "?" + encoded
	}
	c.Redirect(http.StatusFound, target)
}

func splitFundPortfolioAdvices(all []core.FundPortfolioAdvice) ([]core.FundPortfolioAdvice, []core.FundPortfolioAdvice) {
	owned := []core.FundPortfolioAdvice{}
	watch := []core.FundPortfolioAdvice{}
	for _, advice := range all {
		if advice.Item.IsOwned() {
			owned = append(owned, advice)
		} else {
			watch = append(watch, advice)
		}
	}
	return owned, watch
}
