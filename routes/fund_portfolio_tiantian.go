package routes

import (
	"net/http"
	"net/url"

	"github.com/axiaoxin-com/investool/version"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

const tiantianFundHomeURL = "https://www.1234567.com.cn/"

type fundPortfolioTianTianViewData struct {
	Env              string
	HostURL          string
	Version          string
	PageTitle        string
	Error            string
	FundPortfolioURL string
	TianTianFundURL  string
	ContinueURL      string
}

func FundPortfolioTianTianLogin(c *gin.Context) {
	c.HTML(http.StatusOK, "fund_portfolio_tiantian.html", fundPortfolioTianTianViewData{
		Env:              viper.GetString("env"),
		HostURL:          viper.GetString("server.host_url"),
		Version:          version.Version,
		PageTitle:        "InvesTool | 天天基金迁移",
		FundPortfolioURL: fundPortfolioBaseURL(),
		TianTianFundURL:  tiantianFundHomeURL,
		ContinueURL:      fundPortfolioTianTianContinueURL(),
	})
}

func FundPortfolioTianTianContinue(c *gin.Context) {
	c.Redirect(http.StatusFound, fundPortfolioTianTianContinueURL())
}

func fundPortfolioTianTianContinueURL() string {
	values := url.Values{}
	values.Set("message", "已返回本地导入流程，可上传持仓截图或手动录入基金")
	return fundPortfolioBaseURL() + "?" + values.Encode() + "#portfolio-add"
}
