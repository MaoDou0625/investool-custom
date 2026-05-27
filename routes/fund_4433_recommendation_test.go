package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/axiaoxin-com/investool/models"
	"github.com/axiaoxin-com/investool/webserver"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestFundIndexUsesRecommendationWhen4433Empty(t *testing.T) {
	old4433List := models.Fund4433List
	old4433Types := models.Fund4433TypeList
	oldAllList := models.FundAllList
	oldRecommendationList := models.Fund4433RecommendationList
	oldRecommendationUpdatedAt := models.Fund4433RecommendationUpdatedAt
	oldRecommendationSource := models.Fund4433RecommendationSource
	oldRecommendationSourceCount := models.Fund4433RecommendationSourceCount
	defer func() {
		models.Fund4433List = old4433List
		models.Fund4433TypeList = old4433Types
		models.FundAllList = oldAllList
		models.Fund4433RecommendationList = oldRecommendationList
		models.Fund4433RecommendationUpdatedAt = oldRecommendationUpdatedAt
		models.Fund4433RecommendationSource = oldRecommendationSource
		models.Fund4433RecommendationSourceCount = oldRecommendationSourceCount
	}()

	fund4433RecommendationRefreshMu.Lock()
	fund4433RecommendationRefreshing = false
	fund4433RecommendationLastError = ""
	fund4433RecommendationRefreshMu.Unlock()

	models.Fund4433List = nil
	models.Fund4433TypeList = nil
	models.FundAllList = nil
	models.ApplyFund4433RecommendationCache(models.Fund4433RecommendationCache{
		UpdatedAt:   time.Now(),
		Source:      "测试缓存",
		SourceCount: 1,
		Items: models.FundList{
			buildFundIndexRecommendationFund("000001", "测试每日候选基金"),
		},
	})

	viper.Set("server.mode", "release")
	viper.Set("server.host_url", "")
	viper.Set("statics.tmpl_path", "html/*")
	viper.Set("statics.url", "/statics")
	defer viper.Reset()

	app := webserver.NewGinEngine()
	Routes(app)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/fund", nil)
	require.NoError(t, err)
	app.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	require.Contains(t, body, "当前显示每日候选基金")
	require.Contains(t, body, "测试每日候选基金")
	require.Contains(t, body, "测试缓存")
}

func buildFundIndexRecommendationFund(code string, name string) *models.Fund {
	fund := &models.Fund{
		Code:            code,
		Name:            name,
		Type:            "混合型",
		EstablishedDate: "2018-01-01",
		NetAssetsScale:  20 * 100000000,
	}
	fund.Performance.WeekProfitRatio = 1
	fund.Performance.Month3RankRatio = 20
	fund.Performance.Month6RankRatio = 20
	fund.Performance.Year1RankRatio = 20
	return fund
}
