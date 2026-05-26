package routes

import (
	"net/http"
	"net/http/httptest"
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
	require.True(t, strings.HasSuffix(target, "#portfolio-add"))
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
	require.NotContains(t, body, `type="password"`)
}
