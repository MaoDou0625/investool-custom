package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/axiaoxin-com/investool/core"
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
	require.Contains(t, body, "当前显示")
	require.Contains(t, body, "每日候选基金")
	require.Contains(t, body, "测试每日候选基金")
	require.Contains(t, body, "测试缓存")
}

func TestFundIndexShowsCacheRefreshEntry(t *testing.T) {
	oldAllList := models.FundAllList
	old4433List := models.Fund4433List
	oldRecommendationList := models.Fund4433RecommendationList
	oldRecommendationUpdatedAt := models.Fund4433RecommendationUpdatedAt
	defer func() {
		models.FundAllList = oldAllList
		models.Fund4433List = old4433List
		models.Fund4433RecommendationList = oldRecommendationList
		models.Fund4433RecommendationUpdatedAt = oldRecommendationUpdatedAt
		resetFundCacheRefreshStateForTest()
	}()
	resetFundCacheRefreshStateForTest()

	models.FundAllList = models.FundList{buildFundIndexRecommendationFund("000001", "cache entry fund")}
	models.Fund4433List = models.FundList{buildFundIndexRecommendationFund("000001", "cache entry fund")}
	models.Fund4433RecommendationList = nil
	models.Fund4433RecommendationUpdatedAt = time.Now()

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
	require.Contains(t, body, `id="fund-cache-refresh-status"`)
	require.Contains(t, body, `/fund/cache/refresh`)
	require.Contains(t, body, `/fund/cache/status`)
}

func TestFundCacheRefreshStatusUsesCurrentCacheWhenIdle(t *testing.T) {
	oldAllList := models.FundAllList
	old4433List := models.Fund4433List
	oldRecommendationList := models.Fund4433RecommendationList
	defer func() {
		models.FundAllList = oldAllList
		models.Fund4433List = old4433List
		models.Fund4433RecommendationList = oldRecommendationList
		resetFundCacheRefreshStateForTest()
	}()
	resetFundCacheRefreshStateForTest()

	models.FundAllList = models.FundList{buildFundIndexRecommendationFund("000001", "cache status fund")}
	models.Fund4433List = models.FundList{buildFundIndexRecommendationFund("000001", "cache status fund")}
	models.Fund4433RecommendationList = models.FundList{buildFundIndexRecommendationFund("000002", "recommendation fund")}

	viper.Set("server.mode", "release")
	viper.Set("server.host_url", "")
	viper.Set("statics.tmpl_path", "html/*")
	viper.Set("statics.url", "/statics")
	defer viper.Reset()

	app := webserver.NewGinEngine()
	Routes(app)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/fund/cache/status", nil)
	require.NoError(t, err)
	app.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	require.Contains(t, body, `"refreshing":false`)
	require.Contains(t, body, `"fund_count":1`)
	require.Contains(t, body, `"fund_4433_count":1`)
	require.Contains(t, body, `"recommendation_count":1`)
}

func resetFundCacheRefreshStateForTest() {
	fundCacheRefreshMu.Lock()
	defer fundCacheRefreshMu.Unlock()
	fundCacheRefreshRefreshing = false
	fundCacheRefreshStartedAt = time.Time{}
	fundCacheRefreshUpdatedAt = time.Time{}
	fundCacheRefreshFinishedAt = time.Time{}
	fundCacheRefreshProgress = core.FundCacheRefreshProgress{}
	fundCacheRefreshLastError = ""
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
