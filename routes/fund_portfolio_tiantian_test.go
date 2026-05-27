package routes

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/axiaoxin-com/investool/webserver"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestFundPortfolioTianTianContinueURL(t *testing.T) {
	viper.Set("server.host_url", "http://127.0.0.1:4869")
	defer viper.Reset()

	target := fundPortfolioTianTianContinueURL()

	require.True(t, strings.HasPrefix(target, "http://127.0.0.1:4869/fund/portfolio?"))
	require.Contains(t, target, "message=")
	require.Contains(t, target, "tiantian_result=manual_no_import")
	require.True(t, strings.HasSuffix(target, "#portfolio-add"))
}

func TestBuildFundPortfolioTianTianResultNoImport(t *testing.T) {
	result := buildFundPortfolioTianTianResult(fundPortfolioTianTianResultNoImport)

	require.NotNil(t, result)
	require.Equal(t, "未自动导入", result.Status)
	require.Zero(t, result.AddedCount)
	require.Zero(t, result.UpdatedCount)
	require.Zero(t, result.SkippedCount)
	require.NotEmpty(t, result.Details)
	require.NotEmpty(t, result.NextSteps)
	require.Nil(t, buildFundPortfolioTianTianResult(""))
}

func TestFundPortfolioTianTianLoginRendersManualFlow(t *testing.T) {
	viper.Set("server.mode", "release")
	viper.Set("server.host_url", "")
	viper.Set("statics.tmpl_path", "html/*")
	viper.Set("statics.url", "/statics")
	defer viper.Reset()

	app := webserver.NewGinEngine()
	Routes(app)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/fund/portfolio/tiantian", nil)
	require.NoError(t, err)
	app.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	require.Contains(t, body, "fund_portfolio_tiantian_content")
	require.Contains(t, body, "tiantian-open-link")
	require.Contains(t, body, "tiantian-open-default-link")
	require.Contains(t, body, "tiantian_holding_text")
	require.NotContains(t, body, `type="password"`)
}

func TestFundPortfolioTianTianImportTextPreview(t *testing.T) {
	viper.Set("server.mode", "release")
	viper.Set("server.host_url", "")
	viper.Set("statics.tmpl_path", "html/*")
	viper.Set("statics.url", "/statics")
	defer viper.Reset()

	app := webserver.NewGinEngine()
	Routes(app)

	form := url.Values{}
	form.Set("action", "preview")
	form.Set("holding_text", "前海开源金银珠宝混合C\n002207\n持有金额 1,246.30\n持仓成本价 2.8637\n持有份额 460.91")

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/fund/portfolio/tiantian/import", strings.NewReader(form.Encode()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	app.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	require.Contains(t, body, "识别预览")
	require.Contains(t, body, "002207")
	require.Contains(t, body, "1246.30")
	require.NotContains(t, body, `type="password"`)
}

func TestFundPortfolioTianTianImportTextSave(t *testing.T) {
	tmp, err := os.CreateTemp("", "investool-portfolio-*.json")
	require.NoError(t, err)
	tmpFilename := tmp.Name()
	require.NoError(t, tmp.Close())
	require.NoError(t, os.Remove(tmpFilename))
	defer os.Remove(tmpFilename)

	viper.Set("server.mode", "release")
	viper.Set("server.host_url", "")
	viper.Set("statics.tmpl_path", "html/*")
	viper.Set("statics.url", "/statics")
	viper.Set("fund_portfolio.filename", tmpFilename)
	defer viper.Reset()

	app := webserver.NewGinEngine()
	Routes(app)

	form := url.Values{}
	form.Set("action", "save")
	form.Set("holding_text", "前海开源金银珠宝混合C\n002207\n持有金额 1,246.30\n持仓成本价 2.8637\n持有份额 460.91")

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/fund/portfolio/tiantian/import", strings.NewReader(form.Encode()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	app.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	require.Contains(t, body, "已处理")
	require.Contains(t, body, "已新增")

	content, err := os.ReadFile(tmpFilename)
	require.NoError(t, err)
	require.Contains(t, string(content), `"code": "002207"`)
	require.Contains(t, string(content), `"current_amount": 1246.3`)
}
