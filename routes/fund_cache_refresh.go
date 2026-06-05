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

type fundCacheRefreshMeta struct {
	Refreshing          bool
	Stage               string
	Mode                string
	StageText           string
	StartedAt           string
	UpdatedAt           string
	FinishedAt          string
	RawFundCount        int
	Planned             int
	Total               int
	Processed           int
	Succeeded           int
	Failed              int
	Priority4433Count   int
	MissingCount        int
	StaleOtherCount     int
	DeferredCount       int
	SkippedFreshCount   int
	FundCount           int
	Fund4433Count       int
	RecommendationCount int
	TypeCount           int
	Percent             int
	Error               string
}

var (
	fundCacheRefreshMu         sync.Mutex
	fundCacheRefreshRefreshing bool
	fundCacheRefreshStartedAt  time.Time
	fundCacheRefreshUpdatedAt  time.Time
	fundCacheRefreshFinishedAt time.Time
	fundCacheRefreshProgress   core.FundCacheRefreshProgress
	fundCacheRefreshLastError  string

	fundCacheIdleRefreshMu          sync.Mutex
	fundCacheIdleRefreshLastAttempt time.Time
)

func startFundCacheRefresh(ctx context.Context) bool {
	return startFundCacheRefreshWithOptions(ctx, core.DefaultFundCacheRefreshOptions())
}

func startFundCacheRefreshWithOptions(ctx context.Context, options core.FundCacheRefreshOptions) bool {
	if options.Mode == "" {
		options.Mode = core.DefaultFundCacheRefreshOptions().Mode
	}
	fundCacheRefreshMu.Lock()
	if fundCacheRefreshRefreshing {
		fundCacheRefreshMu.Unlock()
		return false
	}
	fundCacheRefreshRefreshing = true
	fundCacheRefreshStartedAt = time.Now()
	fundCacheRefreshUpdatedAt = fundCacheRefreshStartedAt
	fundCacheRefreshFinishedAt = time.Time{}
	fundCacheRefreshLastError = ""
	fundCacheRefreshProgress = core.FundCacheRefreshProgress{
		Stage: core.FundCacheRefreshStageQueryList,
		Mode:  options.Mode,
	}
	fundCacheRefreshMu.Unlock()

	go func() {
		finalProgress, err := core.RefreshFundCache(ctx, options, updateFundCacheRefreshProgress)

		fundCacheRefreshMu.Lock()
		defer fundCacheRefreshMu.Unlock()
		fundCacheRefreshProgress = finalProgress
		fundCacheRefreshFinishedAt = time.Now()
		fundCacheRefreshUpdatedAt = fundCacheRefreshFinishedAt
		fundCacheRefreshRefreshing = false
		if err != nil {
			fundCacheRefreshLastError = err.Error()
			if fundCacheRefreshProgress.Error == "" {
				fundCacheRefreshProgress.Error = err.Error()
			}
			fundCacheRefreshProgress.Stage = core.FundCacheRefreshStageError
		}
	}()
	return true
}

func updateFundCacheRefreshProgress(progress core.FundCacheRefreshProgress) {
	fundCacheRefreshMu.Lock()
	defer fundCacheRefreshMu.Unlock()
	fundCacheRefreshProgress = progress
	fundCacheRefreshUpdatedAt = time.Now()
	if progress.Error != "" {
		fundCacheRefreshLastError = progress.Error
	}
}

func currentFundCacheRefreshMeta() fundCacheRefreshMeta {
	fundCacheRefreshMu.Lock()
	defer fundCacheRefreshMu.Unlock()

	fundCount := fundCacheRefreshProgress.FundCount
	fund4433Count := fundCacheRefreshProgress.Fund4433Count
	recommendationCount := fundCacheRefreshProgress.RecommendationCount
	typeCount := fundCacheRefreshProgress.TypeCount
	if !fundCacheRefreshRefreshing && fundCount == 0 {
		fundCount = len(models.FundAllList)
		fund4433Count = len(models.Fund4433List)
		recommendationCount = len(models.Fund4433RecommendationList)
		typeCount = len(models.FundTypeList)
	}

	return fundCacheRefreshMeta{
		Refreshing:          fundCacheRefreshRefreshing,
		Stage:               fundCacheRefreshProgress.Stage,
		Mode:                fundCacheRefreshProgress.Mode,
		StageText:           fundCacheRefreshStageText(fundCacheRefreshProgress.Stage),
		StartedAt:           formatFund4433RecommendationTime(fundCacheRefreshStartedAt),
		UpdatedAt:           formatFund4433RecommendationTime(fundCacheRefreshUpdatedAt),
		FinishedAt:          formatFund4433RecommendationTime(fundCacheRefreshFinishedAt),
		RawFundCount:        fundCacheRefreshProgress.RawFundCount,
		Planned:             fundCacheRefreshProgress.Planned,
		Total:               fundCacheRefreshProgress.Total,
		Processed:           fundCacheRefreshProgress.Processed,
		Succeeded:           fundCacheRefreshProgress.Succeeded,
		Failed:              fundCacheRefreshProgress.Failed,
		Priority4433Count:   fundCacheRefreshProgress.Priority4433Count,
		MissingCount:        fundCacheRefreshProgress.MissingCount,
		StaleOtherCount:     fundCacheRefreshProgress.StaleOtherCount,
		DeferredCount:       fundCacheRefreshProgress.DeferredCount,
		SkippedFreshCount:   fundCacheRefreshProgress.SkippedFreshCount,
		FundCount:           fundCount,
		Fund4433Count:       fund4433Count,
		RecommendationCount: recommendationCount,
		TypeCount:           typeCount,
		Percent:             fundCacheRefreshPercent(fundCacheRefreshProgress),
		Error:               fundCacheRefreshLastError,
	}
}

func fundCacheRefreshStageText(stage string) string {
	switch stage {
	case core.FundCacheRefreshStageQueryList:
		return "正在获取基金代码列表"
	case core.FundCacheRefreshStageBuildPlan:
		return "正在生成优先刷新计划"
	case core.FundCacheRefreshStageQueryDetail:
		return "正在补全基金详情"
	case core.FundCacheRefreshStageBuildCache:
		return "正在生成 4433 与候选列表"
	case core.FundCacheRefreshStageWriteCache:
		return "正在写入本地缓存"
	case core.FundCacheRefreshStageDone:
		return "本地缓存已完成"
	case core.FundCacheRefreshStageError:
		return "刷新失败"
	default:
		return "尚未刷新"
	}
}

func fundCacheRefreshPercent(progress core.FundCacheRefreshProgress) int {
	if progress.Stage == core.FundCacheRefreshStageDone {
		return 100
	}
	if progress.Total <= 0 {
		return 0
	}
	percent := progress.Processed * 100 / progress.Total
	if percent == 0 && progress.Processed > 0 {
		return 1
	}
	if percent < 0 {
		return 0
	}
	if percent > 99 && progress.Stage != core.FundCacheRefreshStageDone {
		return 99
	}
	return percent
}

func fundCacheRefreshOptionsFromRequest(c *gin.Context) core.FundCacheRefreshOptions {
	options := core.DefaultFundCacheRefreshOptions()
	options.MaxFunds = fundCacheRefreshPriorityBatchSize(options.MaxFunds)
	if c.Query("mode") == core.FundCacheRefreshModeFull {
		options.Mode = core.FundCacheRefreshModeFull
		options.MaxFunds = 0
	}
	return options
}

func fundCacheRefreshPriorityBatchSize(defaultValue int) int {
	size := viper.GetInt("fund_cache.priority_batch_size")
	if size <= 0 {
		return defaultValue
	}
	return size
}

func maybeStartIdleFundCacheRefresh(ctx context.Context) bool {
	if !viper.GetBool("fund_cache.idle_refresh_enabled") {
		return false
	}
	fundCacheIdleRefreshMu.Lock()
	now := time.Now()
	cooldown := time.Duration(viper.GetInt("fund_cache.idle_refresh_cooldown_minutes")) * time.Minute
	if cooldown <= 0 {
		cooldown = 30 * time.Minute
	}
	if !fundCacheIdleRefreshLastAttempt.IsZero() && now.Sub(fundCacheIdleRefreshLastAttempt) < cooldown {
		fundCacheIdleRefreshMu.Unlock()
		return false
	}
	fundCacheIdleRefreshLastAttempt = now
	fundCacheIdleRefreshMu.Unlock()

	options := core.DefaultFundCacheRefreshOptions()
	options.Mode = core.FundCacheRefreshModePriority
	options.MaxFunds = fundCacheRefreshPriorityBatchSize(options.MaxFunds)
	return startFundCacheRefreshWithOptions(ctx, options)
}

// FundCacheRefresh 手动触发本地基金缓存刷新。
func FundCacheRefresh(c *gin.Context) {
	started := startFundCacheRefreshWithOptions(context.Background(), fundCacheRefreshOptionsFromRequest(c))
	message := "已开始后台刷新本地基金缓存"
	if !started {
		message = "本地基金缓存正在刷新中"
	}
	c.Redirect(http.StatusFound, viper.GetString("server.host_url")+"/fund?message="+url.QueryEscape(message)+"#4433")
}

// FundCacheRefreshStatus 返回本地全量基金缓存刷新状态。
func FundCacheRefreshStatus(c *gin.Context) {
	meta := currentFundCacheRefreshMeta()
	c.JSON(http.StatusOK, gin.H{
		"refreshing":           meta.Refreshing,
		"stage":                meta.Stage,
		"mode":                 meta.Mode,
		"stage_text":           meta.StageText,
		"started_at":           meta.StartedAt,
		"updated_at":           meta.UpdatedAt,
		"finished_at":          meta.FinishedAt,
		"raw_fund_count":       meta.RawFundCount,
		"planned":              meta.Planned,
		"total":                meta.Total,
		"processed":            meta.Processed,
		"succeeded":            meta.Succeeded,
		"failed":               meta.Failed,
		"priority_4433_count":  meta.Priority4433Count,
		"missing_count":        meta.MissingCount,
		"stale_other_count":    meta.StaleOtherCount,
		"deferred_count":       meta.DeferredCount,
		"skipped_fresh_count":  meta.SkippedFreshCount,
		"fund_count":           meta.FundCount,
		"fund_4433_count":      meta.Fund4433Count,
		"recommendation_count": meta.RecommendationCount,
		"type_count":           meta.TypeCount,
		"percent":              meta.Percent,
		"error":                meta.Error,
	})
}
