package routes

import (
	"net/http"
	"net/url"

	"github.com/axiaoxin-com/investool/version"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

const tiantianFundHomeURL = "https://www.1234567.com.cn/"
const fundPortfolioTianTianResultNoImport = "manual_no_import"

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

type fundPortfolioTianTianImportResult struct {
	Title        string
	Status       string
	StatusClass  string
	AddedCount   int
	UpdatedCount int
	SkippedCount int
	Details      []string
	NextSteps    []string
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
	values.Set("message", "天天基金接续完成：本次没有自动新增基金")
	values.Set("tiantian_result", fundPortfolioTianTianResultNoImport)
	return fundPortfolioBaseURL() + "?" + values.Encode() + "#portfolio-add"
}

func buildFundPortfolioTianTianResult(code string) *fundPortfolioTianTianImportResult {
	if code != fundPortfolioTianTianResultNoImport {
		return nil
	}
	return &fundPortfolioTianTianImportResult{
		Title:        "天天基金接续结果",
		Status:       "未自动导入",
		StatusClass:  "is-neutral",
		AddedCount:   0,
		UpdatedCount: 0,
		SkippedCount: 0,
		Details: []string{
			"本次没有新增、更新或跳过任何基金。",
			"当前流程只负责打开天天基金并等待你手动登录，不读取网页持仓、不保存登录信息。",
		},
		NextSteps: []string{
			"在天天基金持仓页截图后，回到这里上传截图识别。",
			"也可以直接在上方表单手动录入基金代码、当前总值、成本净值和持有份额。",
		},
	}
}
