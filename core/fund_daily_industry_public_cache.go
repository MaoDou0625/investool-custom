package core

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/axiaoxin-com/investool/datacenter/eastmoney"
	"github.com/axiaoxin-com/investool/models"
)

func fundDailyIndustrySignalCacheItem(
	ctx context.Context,
	provider fundDailyIndustryPublicDataProvider,
	cache models.FundDailyIndustrySignalCache,
	target fundDailyIndustryStockTarget,
	now time.Time,
) (
	models.FundDailyIndustrySignalCacheItem,
	models.FundDailyIndustrySignalCacheItem,
	[]models.FundDailyIndustryProfitPredictCachePoint,
	string,
	bool,
	[]string,
) {
	current := fundDailyIndustryNormalizeComponentTimestamps(cache.Items[target.Code])
	if fundDailyIndustrySignalCacheItemFresh(current, now) && fundDailyIndustrySignalCacheItemHasFullData(current, now) {
		previous, previousUpdatedAt, hadPrevious := fundDailyIndustryProfitBaseline(current, now)
		return fundDailyIndustryScoreItemFromCache(current, now), current, previous, previousUpdatedAt, hadPrevious, nil
	}

	updated := current
	updated.Code = target.Code
	updated.Name = target.Name
	warnings := []string{}
	didRefresh := false
	secuCode := fundDailyEastMoneySecuCode(target.Code)

	peList, err := provider.QueryHistoricalPEList(ctx, secuCode)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("%s 估值分位公开源读取失败：%s", target.Name, compactFundDailyMarketError(err)))
	} else {
		updated.HistoricalPE = fundDailyIndustryPECachePoints(peList)
		updated.HistoricalPEUpdatedAt = now.Format("2006-01-02 15:04:05")
		didRefresh = true
	}

	predicts, err := provider.QueryProfitPredict(ctx, secuCode)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("%s 盈利预测公开源读取失败：%s", target.Name, compactFundDailyMarketError(err)))
	} else {
		if !fundDailyIndustrySignalComponentFresh(updated.ProfitPredictsUpdatedAt, updated.UpdatedAt, now) && len(updated.ProfitPredicts) > 0 {
			updated.PreviousProfitPredicts = updated.ProfitPredicts
			updated.PreviousProfitPredictsUpdatedAt = updated.ProfitPredictsUpdatedAt
			if updated.PreviousProfitPredictsUpdatedAt == "" {
				updated.PreviousProfitPredictsUpdatedAt = updated.UpdatedAt
			}
		}
		updated.ProfitPredicts = fundDailyIndustryProfitPredictCachePoints(predicts)
		updated.ProfitPredictsUpdatedAt = now.Format("2006-01-02 15:04:05")
		didRefresh = true
	}

	if didRefresh {
		updated.UpdatedAt = now.Format("2006-01-02 15:04:05")
	}
	previous, previousUpdatedAt, hadPrevious := fundDailyIndustryProfitBaseline(updated, now)
	return fundDailyIndustryScoreItemFromCache(updated, now), updated, previous, previousUpdatedAt, hadPrevious, warnings
}

func fundDailyIndustrySignalCacheItemFresh(item models.FundDailyIndustrySignalCacheItem, now time.Time) bool {
	updatedAt, err := time.ParseInLocation("2006-01-02 15:04:05", item.UpdatedAt, time.Local)
	if err != nil {
		return false
	}
	return updatedAt.Format("2006-01-02") == now.Format("2006-01-02")
}

func fundDailyIndustrySignalCacheItemHasFullData(item models.FundDailyIndustrySignalCacheItem, now time.Time) bool {
	return len(item.HistoricalPE) > 0 &&
		len(item.ProfitPredicts) > 0 &&
		fundDailyIndustrySignalComponentFresh(item.HistoricalPEUpdatedAt, item.UpdatedAt, now) &&
		fundDailyIndustrySignalComponentFresh(item.ProfitPredictsUpdatedAt, item.UpdatedAt, now)
}

func fundDailyIndustrySignalComponentFresh(componentUpdatedAt string, fallbackUpdatedAt string, now time.Time) bool {
	updatedAt := strings.TrimSpace(componentUpdatedAt)
	if updatedAt == "" {
		updatedAt = strings.TrimSpace(fallbackUpdatedAt)
	}
	parsed, err := time.ParseInLocation("2006-01-02 15:04:05", updatedAt, time.Local)
	if err != nil {
		return false
	}
	return parsed.Format("2006-01-02") == now.Format("2006-01-02")
}

func fundDailyIndustryNormalizeComponentTimestamps(item models.FundDailyIndustrySignalCacheItem) models.FundDailyIndustrySignalCacheItem {
	if item.UpdatedAt == "" {
		return item
	}
	if item.HistoricalPEUpdatedAt == "" && len(item.HistoricalPE) > 0 {
		item.HistoricalPEUpdatedAt = item.UpdatedAt
	}
	if item.ProfitPredictsUpdatedAt == "" && len(item.ProfitPredicts) > 0 {
		item.ProfitPredictsUpdatedAt = item.UpdatedAt
	}
	if item.PreviousProfitPredictsUpdatedAt == "" && len(item.PreviousProfitPredicts) > 0 {
		item.PreviousProfitPredictsUpdatedAt = item.UpdatedAt
	}
	return item
}

func fundDailyIndustryScoreItemFromCache(item models.FundDailyIndustrySignalCacheItem, now time.Time) models.FundDailyIndustrySignalCacheItem {
	scoreItem := models.FundDailyIndustrySignalCacheItem{
		Code:      item.Code,
		Name:      item.Name,
		UpdatedAt: item.UpdatedAt,
	}
	if fundDailyIndustrySignalComponentFresh(item.HistoricalPEUpdatedAt, item.UpdatedAt, now) {
		scoreItem.HistoricalPE = item.HistoricalPE
		scoreItem.HistoricalPEUpdatedAt = item.HistoricalPEUpdatedAt
	}
	if fundDailyIndustrySignalComponentFresh(item.ProfitPredictsUpdatedAt, item.UpdatedAt, now) {
		scoreItem.ProfitPredicts = item.ProfitPredicts
		scoreItem.ProfitPredictsUpdatedAt = item.ProfitPredictsUpdatedAt
	}
	return scoreItem
}

func fundDailyIndustryProfitBaseline(
	item models.FundDailyIndustrySignalCacheItem,
	now time.Time,
) ([]models.FundDailyIndustryProfitPredictCachePoint, string, bool) {
	if len(item.PreviousProfitPredicts) > 0 && !fundDailyIndustrySignalComponentFresh(item.PreviousProfitPredictsUpdatedAt, "", now) {
		return item.PreviousProfitPredicts, item.PreviousProfitPredictsUpdatedAt, true
	}
	if len(item.ProfitPredicts) > 0 && !fundDailyIndustrySignalComponentFresh(item.ProfitPredictsUpdatedAt, item.UpdatedAt, now) {
		updatedAt := item.ProfitPredictsUpdatedAt
		if updatedAt == "" {
			updatedAt = item.UpdatedAt
		}
		return item.ProfitPredicts, updatedAt, true
	}
	return nil, "", false
}

func fundDailyIndustryPECachePoints(peList eastmoney.HistoricalPEList) []models.FundDailyIndustryPEPoint {
	points := make([]models.FundDailyIndustryPEPoint, 0, len(peList))
	for _, point := range peList {
		if point.Value <= 0 || strings.TrimSpace(point.Date) == "" {
			continue
		}
		points = append(points, models.FundDailyIndustryPEPoint{
			Date: point.Date,
			PE:   point.Value,
		})
	}
	sort.SliceStable(points, func(i, j int) bool {
		return points[i].Date < points[j].Date
	})
	return points
}

func fundDailyIndustryProfitPredictCachePoints(predicts eastmoney.ProfitPredictList) []models.FundDailyIndustryProfitPredictCachePoint {
	points := make([]models.FundDailyIndustryProfitPredictCachePoint, 0, len(predicts))
	for _, point := range predicts {
		if point.PredictYear <= 0 || point.Eps == 0 {
			continue
		}
		points = append(points, models.FundDailyIndustryProfitPredictCachePoint{
			Year: point.PredictYear,
			EPS:  point.Eps,
			PE:   point.Pe,
		})
	}
	sort.SliceStable(points, func(i, j int) bool {
		return points[i].Year < points[j].Year
	})
	return points
}
