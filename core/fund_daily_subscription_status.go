package core

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/axiaoxin-com/investool/datacenter/eastmoney"
	"github.com/axiaoxin-com/investool/models"
)

const (
	fundDailySubscriptionRefreshWorkers = 6
	fundDailySubscriptionRefreshTimeout = 12 * time.Second
)

type fundDailySubscriptionInfoProvider interface {
	QueryFundInfo(ctx context.Context, fundCode string) (*eastmoney.RespFundInfo, error)
}

type fundDailySubscriptionRefreshResult struct {
	code                  string
	subscriptionStatus    string
	fixedInvestmentStatus string
	err                   error
}

// RefreshFundDailySubscriptionStatuses refreshes buyability for the small daily-advice
// working set. It deliberately avoids refreshing the full local cache on page load.
func RefreshFundDailySubscriptionStatuses(ctx context.Context, candidateFunds models.FundList, portfolioAdvices []FundPortfolioAdvice) []string {
	return refreshFundDailySubscriptionStatuses(ctx, eastmoney.NewEastMoney(), candidateFunds, portfolioAdvices)
}

func refreshFundDailySubscriptionStatuses(ctx context.Context, provider fundDailySubscriptionInfoProvider, candidateFunds models.FundList, portfolioAdvices []FundPortfolioAdvice) []string {
	targets := collectFundDailySubscriptionTargets(candidateFunds, portfolioAdvices)
	if len(targets) == 0 {
		return nil
	}

	codes := make([]string, 0, len(targets))
	for code := range targets {
		codes = append(codes, code)
	}
	sort.Strings(codes)

	jobs := make(chan string)
	results := make(chan fundDailySubscriptionRefreshResult, len(codes))
	workerCount := minInt(fundDailySubscriptionRefreshWorkers, len(codes))
	var wg sync.WaitGroup
	for idx := 0; idx < workerCount; idx++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for code := range jobs {
				results <- queryFundDailySubscriptionStatus(ctx, provider, code)
			}
		}()
	}

	go func() {
		for _, code := range codes {
			jobs <- code
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	failed := []string{}
	for result := range results {
		if result.err != nil {
			failed = append(failed, result.code)
			applyFundDailySubscriptionStatus(targets[result.code], "未读取到申购状态", "")
			continue
		}
		applyFundDailySubscriptionStatus(targets[result.code], result.subscriptionStatus, result.fixedInvestmentStatus)
	}
	sort.Strings(failed)

	if len(failed) == 0 {
		return nil
	}
	return []string{fmt.Sprintf("申购状态实时刷新失败：%s；这些基金按不可申购处理。", strings.Join(limitFundDailySubscriptionWarningCodes(failed), "；"))}
}

func collectFundDailySubscriptionTargets(candidateFunds models.FundList, portfolioAdvices []FundPortfolioAdvice) map[string][]*models.Fund {
	targets := map[string][]*models.Fund{}
	for _, fund := range candidateFunds {
		addFundDailySubscriptionTarget(targets, fund)
	}
	for _, advice := range portfolioAdvices {
		addFundDailySubscriptionTarget(targets, advice.Fund)
	}
	return targets
}

func addFundDailySubscriptionTarget(targets map[string][]*models.Fund, fund *models.Fund) {
	if fund == nil {
		return
	}
	code := strings.TrimSpace(fund.Code)
	if code == "" {
		return
	}
	targets[code] = append(targets[code], fund)
}

func queryFundDailySubscriptionStatus(ctx context.Context, provider fundDailySubscriptionInfoProvider, code string) fundDailySubscriptionRefreshResult {
	queryCtx, cancel := context.WithTimeout(ctx, fundDailySubscriptionRefreshTimeout)
	defer cancel()

	info, err := provider.QueryFundInfo(queryCtx, code)
	if err != nil {
		return fundDailySubscriptionRefreshResult{code: code, err: err}
	}
	fund := models.NewFund(ctx, info)
	if fund == nil || strings.TrimSpace(fund.Code) == "" {
		return fundDailySubscriptionRefreshResult{code: code, err: fmt.Errorf("empty fund detail")}
	}
	return fundDailySubscriptionRefreshResult{
		code:                  code,
		subscriptionStatus:    strings.TrimSpace(fund.SubscriptionStatus),
		fixedInvestmentStatus: strings.TrimSpace(fund.FixedInvestmentStatus),
	}
}

func applyFundDailySubscriptionStatus(funds []*models.Fund, subscriptionStatus string, fixedInvestmentStatus string) {
	for _, fund := range funds {
		if fund == nil {
			continue
		}
		fund.SubscriptionStatus = strings.TrimSpace(subscriptionStatus)
		fund.FixedInvestmentStatus = strings.TrimSpace(fixedInvestmentStatus)
	}
}

func limitFundDailySubscriptionWarningCodes(codes []string) []string {
	const limit = 8
	if len(codes) <= limit {
		return codes
	}
	limited := append([]string{}, codes[:limit]...)
	limited = append(limited, fmt.Sprintf("另%d只", len(codes)-limit))
	return limited
}
