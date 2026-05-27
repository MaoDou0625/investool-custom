package routes

import (
	"context"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/axiaoxin-com/investool/core"
	"github.com/axiaoxin-com/investool/models"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

type fund4433DisplayData struct {
	List models.FundList
	Meta fund4433RecommendationMeta
}

type fund4433RecommendationMeta struct {
	UsingRecommendation bool
	Refreshing          bool
	RefreshStartedAt    string
	UpdatedAt           string
	Source              string
	SourceCount         int
	Error               string
}

var (
	fund4433RecommendationRefreshMu        sync.Mutex
	fund4433RecommendationRefreshing       bool
	fund4433RecommendationRefreshStartedAt time.Time
	fund4433RecommendationLastError        string
)

func resolveFund4433Display(ctx context.Context, forceRefresh bool) fund4433DisplayData {
	if len(models.Fund4433List) > 0 && !forceRefresh {
		return fund4433DisplayData{
			List: models.Fund4433List,
			Meta: fund4433RecommendationMeta{
				Refreshing: fund4433RecommendationRefreshingNow(),
			},
		}
	}

	if forceRefresh || !isFund4433RecommendationFresh(models.Fund4433RecommendationUpdatedAt) {
		startFund4433RecommendationRefresh(context.Background())
	}

	return fund4433DisplayData{
		List: models.Fund4433RecommendationList,
		Meta: currentFund4433RecommendationMeta(),
	}
}

func startFund4433RecommendationRefresh(ctx context.Context) bool {
	fund4433RecommendationRefreshMu.Lock()
	if fund4433RecommendationRefreshing {
		fund4433RecommendationRefreshMu.Unlock()
		return false
	}
	fund4433RecommendationRefreshing = true
	fund4433RecommendationRefreshStartedAt = time.Now()
	fund4433RecommendationLastError = ""
	fund4433RecommendationRefreshMu.Unlock()

	go func() {
		list, source, sourceCount, err := core.RefreshFund4433Recommendations(ctx, models.FundAllList, core.DefaultFund4433RecommendationOptions())
		fund4433RecommendationRefreshMu.Lock()
		defer fund4433RecommendationRefreshMu.Unlock()
		defer func() {
			fund4433RecommendationRefreshing = false
		}()
		if err != nil {
			fund4433RecommendationLastError = err.Error()
			return
		}
		cache := models.Fund4433RecommendationCache{
			UpdatedAt:   time.Now(),
			Source:      source,
			SourceCount: sourceCount,
			Items:       list,
		}
		if err := models.SaveFund4433RecommendationCache(cache); err != nil {
			fund4433RecommendationLastError = err.Error()
		}
	}()
	return true
}

func currentFund4433RecommendationMeta() fund4433RecommendationMeta {
	fund4433RecommendationRefreshMu.Lock()
	defer fund4433RecommendationRefreshMu.Unlock()

	return fund4433RecommendationMeta{
		UsingRecommendation: len(models.Fund4433RecommendationList) > 0,
		Refreshing:          fund4433RecommendationRefreshing,
		RefreshStartedAt:    formatFund4433RecommendationTime(fund4433RecommendationRefreshStartedAt),
		UpdatedAt:           formatFund4433RecommendationTime(models.Fund4433RecommendationUpdatedAt),
		Source:              models.Fund4433RecommendationSource,
		SourceCount:         models.Fund4433RecommendationSourceCount,
		Error:               fund4433RecommendationLastError,
	}
}

func fund4433RecommendationRefreshingNow() bool {
	fund4433RecommendationRefreshMu.Lock()
	defer fund4433RecommendationRefreshMu.Unlock()
	return fund4433RecommendationRefreshing
}

func isFund4433RecommendationFresh(updatedAt time.Time) bool {
	if updatedAt.IsZero() {
		return false
	}
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		location = time.Local
	}
	return time.Now().In(location).Format("2006-01-02") == updatedAt.In(location).Format("2006-01-02")
}

func formatFund4433RecommendationTime(t time.Time) string {
	if t.IsZero() {
		return "--"
	}
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		location = time.Local
	}
	return t.In(location).Format("2006-01-02 15:04:05")
}

// Fund4433RecommendationRefresh 手动触发 4433 每日候选刷新。
func Fund4433RecommendationRefresh(c *gin.Context) {
	startFund4433RecommendationRefresh(context.Background())
	message := url.QueryEscape("已开始后台刷新每日候选基金")
	c.Redirect(http.StatusFound, viper.GetString("server.host_url")+"/fund?message="+message+"#4433")
}

// Fund4433RecommendationStatus 返回 4433 每日候选刷新状态。
func Fund4433RecommendationStatus(c *gin.Context) {
	meta := currentFund4433RecommendationMeta()
	c.JSON(http.StatusOK, gin.H{
		"using_recommendation": meta.UsingRecommendation,
		"refreshing":           meta.Refreshing,
		"refresh_started_at":   meta.RefreshStartedAt,
		"updated_at":           meta.UpdatedAt,
		"source":               meta.Source,
		"source_count":         meta.SourceCount,
		"count":                len(models.Fund4433RecommendationList),
		"error":                meta.Error,
	})
}
