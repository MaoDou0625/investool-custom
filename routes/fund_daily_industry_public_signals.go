package routes

import (
	"context"
	"fmt"

	"github.com/axiaoxin-com/investool/core"
	"github.com/axiaoxin-com/investool/models"
	"github.com/spf13/viper"
)

const defaultFundDailyIndustrySignalCacheFilename = "./fund_daily_industry_signal_cache.json"

func loadFundDailyIndustryPublicSignals(
	ctx context.Context,
	report core.FundDailyAdviceReport,
) ([]core.FundDailyIndustryExpectationSignal, []string) {
	store := newFundDailyIndustrySignalCacheStore()
	cache, err := store.Load()
	warnings := []string{}
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("行业公开源缓存读取失败，盈利预期修正首日将降级为快照：%v", err))
		cache = models.FundDailyIndustrySignalCache{Items: map[string]models.FundDailyIndustrySignalCacheItem{}}
	}

	signals, updatedCache, signalWarnings := core.BuildFundDailyIndustryPublicSignals(ctx, report, cache)
	warnings = append(warnings, signalWarnings...)
	if len(updatedCache.Items) > 0 {
		if err := store.Save(updatedCache); err != nil {
			warnings = append(warnings, fmt.Sprintf("行业公开源缓存写入失败，后续盈利预期修正无法与本次快照比较：%v", err))
		}
	}
	return signals, warnings
}

func newFundDailyIndustrySignalCacheStore() *models.FundDailyIndustrySignalCacheStore {
	return models.NewFundDailyIndustrySignalCacheStore(fundDailyIndustrySignalCacheFilename())
}

func fundDailyIndustrySignalCacheFilename() string {
	filename := viper.GetString("fund_advice.industry_signal_cache_filename")
	if filename == "" {
		return defaultFundDailyIndustrySignalCacheFilename
	}
	return filename
}
