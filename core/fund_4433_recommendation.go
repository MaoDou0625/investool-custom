package core

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/axiaoxin-com/investool/datacenter"
	"github.com/axiaoxin-com/investool/datacenter/eastmoney"
	"github.com/axiaoxin-com/investool/models"
	"github.com/axiaoxin-com/logging"
)

// Fund4433RecommendationOptions 控制 4433 候选基金生成规模。
type Fund4433RecommendationOptions struct {
	MaxCount         int
	CandidatePerType int
	WorkerCount      int
	MinRankFields    int
	FundTypes        []eastmoney.FundType
}

// DefaultFund4433RecommendationOptions 返回本地每日候选的默认配置。
func DefaultFund4433RecommendationOptions() Fund4433RecommendationOptions {
	return Fund4433RecommendationOptions{
		MaxCount:         30,
		CandidatePerType: 16,
		WorkerCount:      4,
		MinRankFields:    3,
		FundTypes: []eastmoney.FundType{
			eastmoney.FundTypeStock,
			eastmoney.FundTypeMix,
			eastmoney.FundTypeIndex,
			eastmoney.FundTypeETF,
			eastmoney.FundTypeETFLink,
			eastmoney.FundTypeQDII,
			eastmoney.FundTypeLOF,
			eastmoney.FundTypeBond,
		},
	}
}

// RefreshFund4433Recommendations 用本地全量缓存或实时候选池生成每日 4433 候选。
func RefreshFund4433Recommendations(ctx context.Context, source models.FundList, options Fund4433RecommendationOptions) (models.FundList, string, int, error) {
	options = normalizeFund4433RecommendationOptions(options)
	if len(source) > 0 {
		recommendations := BuildFund4433Recommendations(ctx, source, options)
		if len(recommendations) > 0 {
			return recommendations, "本地全量基金缓存", len(source), nil
		}
	}

	candidates, err := FetchFund4433RecommendationCandidates(ctx, options)
	if err != nil {
		return nil, "天天基金实时候选", 0, err
	}
	recommendations := BuildFund4433Recommendations(ctx, candidates, options)
	if len(recommendations) == 0 {
		return nil, "天天基金实时候选", len(candidates), fmt.Errorf("未能从实时候选池生成可展示基金")
	}
	return recommendations, "天天基金实时候选", len(candidates), nil
}

// FetchFund4433RecommendationCandidates 从天天基金按基金类型拉取一批轻量候选，再补全详情。
func FetchFund4433RecommendationCandidates(ctx context.Context, options Fund4433RecommendationOptions) (models.FundList, error) {
	options = normalizeFund4433RecommendationOptions(options)
	codes, err := queryFund4433RecommendationCandidateCodes(ctx, options)
	if err != nil {
		return nil, err
	}
	searcher := NewSearcher(ctx)
	fundMap, err := searcher.SearchFundsWithWorkerCount(ctx, codes, options.WorkerCount)
	if err != nil {
		return nil, err
	}
	funds := make(models.FundList, 0, len(fundMap))
	for _, code := range codes {
		if fund := fundMap[code]; fund != nil {
			funds = append(funds, fund)
		}
	}
	return funds, nil
}

// BuildFund4433Recommendations 按接近 4433、风险、规模和经理稳定性生成候选基金。
func BuildFund4433Recommendations(ctx context.Context, funds models.FundList, options Fund4433RecommendationOptions) models.FundList {
	options = normalizeFund4433RecommendationOptions(options)
	scored := make([]fund4433RecommendationScore, 0, len(funds))
	for _, fund := range funds {
		score, rankFields, ok := scoreFund4433Recommendation(ctx, fund)
		if !ok || rankFields < options.MinRankFields {
			continue
		}
		scored = append(scored, fund4433RecommendationScore{
			Fund:       fund,
			Score:      score,
			RankFields: rankFields,
		})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		if math.Abs(scored[i].Score-scored[j].Score) > 0.001 {
			return scored[i].Score > scored[j].Score
		}
		if scored[i].RankFields != scored[j].RankFields {
			return scored[i].RankFields > scored[j].RankFields
		}
		if scored[i].Fund.Performance.Year1ProfitRatio != scored[j].Fund.Performance.Year1ProfitRatio {
			return scored[i].Fund.Performance.Year1ProfitRatio > scored[j].Fund.Performance.Year1ProfitRatio
		}
		return scored[i].Fund.Code < scored[j].Fund.Code
	})

	if len(scored) > options.MaxCount {
		scored = scored[:options.MaxCount]
	}
	results := make(models.FundList, 0, len(scored))
	for _, item := range scored {
		results = append(results, item.Fund)
	}
	return results
}

type fund4433RecommendationScore struct {
	Fund       *models.Fund
	Score      float64
	RankFields int
}

func normalizeFund4433RecommendationOptions(options Fund4433RecommendationOptions) Fund4433RecommendationOptions {
	if options.MaxCount <= 0 {
		options.MaxCount = 30
	}
	if options.CandidatePerType <= 0 {
		options.CandidatePerType = 16
	}
	if options.WorkerCount <= 0 {
		options.WorkerCount = 4
	}
	if options.MinRankFields <= 0 {
		options.MinRankFields = 3
	}
	if len(options.FundTypes) == 0 {
		options.FundTypes = DefaultFund4433RecommendationOptions().FundTypes
	}
	return options
}

func queryFund4433RecommendationCandidateCodes(ctx context.Context, options Fund4433RecommendationOptions) ([]string, error) {
	codes := make([]string, 0, len(options.FundTypes)*options.CandidatePerType)
	seen := map[string]struct{}{}
	for _, fundType := range options.FundTypes {
		resp, err := datacenter.EastMoney.QueryFundListByPage(ctx, fundType, 1)
		if err != nil {
			logging.Warnf(ctx, "QueryFundListByPage fundType:%v err:%v", fundType, err)
			continue
		}
		typeCount := 0
		for _, page := range resp.Datas {
			for _, item := range page {
				if typeCount >= options.CandidatePerType {
					break
				}
				if item.Fcode == "" {
					continue
				}
				if _, exists := seen[item.Fcode]; exists {
					continue
				}
				seen[item.Fcode] = struct{}{}
				codes = append(codes, item.Fcode)
				typeCount++
			}
		}
	}
	if len(codes) == 0 {
		return nil, fmt.Errorf("未能获取天天基金候选基金代码")
	}
	return codes, nil
}

func scoreFund4433Recommendation(ctx context.Context, fund *models.Fund) (float64, int, bool) {
	if fund == nil {
		return 0, 0, false
	}
	if shouldSkipFund4433Recommendation(fund) {
		return 0, 0, false
	}

	performance := fund.Performance
	score := 45.0
	rankFields := 0
	rankSignals := []struct {
		ratio  float64
		target float64
		weight float64
	}{
		{performance.Year1RankRatio, 25, 15},
		{performance.Year2RankRatio, 25, 10},
		{performance.Year3RankRatio, 25, 12},
		{performance.Year5RankRatio, 25, 8},
		{performance.ThisYearRankRatio, 25, 10},
		{performance.Month6RankRatio, 33.33, 9},
		{performance.Month3RankRatio, 33.33, 9},
	}
	for _, signal := range rankSignals {
		if signal.ratio <= 0 {
			continue
		}
		rankFields++
		score += rankSignalScore(signal.ratio, signal.target, signal.weight)
	}
	if rankFields == 0 {
		return 0, 0, false
	}

	if performance.Year1ProfitRatio > 0 {
		score += math.Min(8, performance.Year1ProfitRatio/5)
	}
	if performance.Month3ProfitRatio > 0 {
		score += math.Min(5, performance.Month3ProfitRatio/4)
	}
	if performance.Month6ProfitRatio < -10 {
		score -= 6
	}

	score += fundRiskScore(fund)
	score += fundScaleScore(fund)
	score += fundManagerScore(fund)
	score += fundEstablishedScore(ctx, fund)
	return score, rankFields, true
}

func shouldSkipFund4433Recommendation(fund *models.Fund) bool {
	if strings.Contains(fund.Type, "货币") {
		return true
	}
	for _, keyword := range []string{"美元", "现汇", "现钞"} {
		if strings.Contains(fund.Name, keyword) {
			return true
		}
	}
	return false
}

func rankSignalScore(ratio, target, weight float64) float64 {
	switch {
	case ratio <= target:
		return weight * (1 + (target-ratio)/target*0.5)
	case ratio <= target*1.5:
		return weight * (1 - (ratio-target)/(target*0.5)*0.5)
	case ratio <= 60:
		return -weight * 0.25
	default:
		return -weight * 0.5
	}
}

func fundRiskScore(fund *models.Fund) float64 {
	score := 0.0
	switch {
	case fund.Sharp.Avg135 >= 1.2:
		score += 8
	case fund.Sharp.Avg135 >= 0.8:
		score += 4
	case fund.Sharp.Avg135 > 0 && fund.Sharp.Avg135 < 0.5:
		score -= 5
	}
	switch {
	case fund.MaxRetracement.Avg135 > 0 && fund.MaxRetracement.Avg135 <= 20:
		score += 5
	case fund.MaxRetracement.Avg135 > 30:
		score -= 8
	}
	switch {
	case fund.Stddev.Avg135 > 0 && fund.Stddev.Avg135 <= 20:
		score += 3
	case fund.Stddev.Avg135 > 30:
		score -= 4
	}
	return score
}

func fundScaleScore(fund *models.Fund) float64 {
	scaleYi := fund.NetAssetsScale / 100000000
	switch {
	case scaleYi >= 2 && scaleYi <= 80:
		return 5
	case scaleYi >= 1 && scaleYi <= 150:
		return 2
	case scaleYi > 0 && scaleYi < 1:
		return -8
	case scaleYi > 150:
		return -4
	default:
		return 0
	}
}

func fundManagerScore(fund *models.Fund) float64 {
	manageYears := fund.Manager.ManageDays / 365
	score := 0.0
	switch {
	case manageYears >= 5:
		score += 6
	case manageYears >= 3:
		score += 4
	case manageYears > 0 && manageYears < 1:
		score -= 3
	}
	if fund.Manager.ManageRepay > 0 {
		score += math.Min(4, fund.Manager.ManageRepay/20)
	} else if fund.Manager.ManageRepay < -10 {
		score -= 4
	}
	return score
}

func fundEstablishedScore(ctx context.Context, fund *models.Fund) float64 {
	if fund.EstablishedDate == "" || fund.EstablishedDate == "--" {
		return 0
	}
	years := fund.EstabYears(ctx)
	switch {
	case years >= 5:
		return 5
	case years >= 3:
		return 3
	case years > 0 && years < 1:
		return -5
	default:
		return 0
	}
}
