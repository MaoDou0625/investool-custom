// Package cron 定时任务
package cron

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"time"

	"github.com/axiaoxin-com/goutils"
	"github.com/axiaoxin-com/investool/core"
	"github.com/axiaoxin-com/investool/models"
	"github.com/axiaoxin-com/logging"
)

// SyncFund 同步基金数据
func SyncFund() {
	if !goutils.IsTradingDay() {
		return
	}
	ctx := context.Background()
	logging.Info(ctx, "SyncFund request start...")
	if _, err := core.RefreshFundCache(ctx, core.DefaultFundCacheRefreshOptions(), nil); err != nil {
		logging.Error(ctx, "SyncFund RefreshFundCache error:"+err.Error())
		promSyncError.WithLabelValues("SyncFund").Inc()
		return
	}
}

// Update4433 更新4433检测结果
func Update4433() {
	ctx := context.Background()
	fundlist := core.BuildFund4433List(ctx, models.FundAllList)
	// 更新 models 变量
	models.Fund4433List = fundlist

	// 更新文件
	b, err := json.Marshal(fundlist)
	if err != nil {
		logging.Errorf(ctx, "Update4433 json marshal error:", err)
		promSyncError.WithLabelValues("Update4433").Inc()
		return
	} else if err := ioutil.WriteFile(models.Fund4433ListFilename, b, 0666); err != nil {
		logging.Errorf(ctx, "Update4433 WriteFile error:", err)
		promSyncError.WithLabelValues("Update4433").Inc()
		return
	}
}

// Update4433Recommendation 更新4433为空时展示的每日候选基金。
func Update4433Recommendation() {
	ctx := context.Background()
	fundlist, source, sourceCount, err := core.RefreshFund4433Recommendations(ctx, models.FundAllList, core.DefaultFund4433RecommendationOptions())
	if err != nil {
		logging.Errorf(ctx, "Update4433Recommendation refresh error:%v", err)
		promSyncError.WithLabelValues("Update4433Recommendation").Inc()
		return
	}
	cache := models.Fund4433RecommendationCache{
		UpdatedAt:   time.Now(),
		Source:      source,
		SourceCount: sourceCount,
		Items:       fundlist,
	}
	if err := models.SaveFund4433RecommendationCache(cache); err != nil {
		logging.Errorf(ctx, "Update4433Recommendation WriteFile error:%v", err)
		promSyncError.WithLabelValues("Update4433Recommendation").Inc()
		return
	}
}
