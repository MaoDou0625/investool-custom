package core

import (
	"context"
	"math"
	"strings"
	"sync"

	"github.com/axiaoxin-com/investool/datacenter"
	"github.com/axiaoxin-com/investool/datacenter/eastmoney"
	"github.com/axiaoxin-com/investool/models"
	"github.com/axiaoxin-com/logging"
)

func enrichFundHolderStructureCache(ctx context.Context, funds map[string]*models.Fund, workerCount int) {
	if len(funds) == 0 {
		return
	}
	if workerCount <= 0 {
		workerCount = 1
	}
	workerCount = int(math.Min(float64(workerCount), float64(len(funds))))

	type holderStructureCacheTask struct {
		code string
		fund *models.Fund
	}
	tasks := make(chan holderStructureCacheTask)
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range tasks {
				result, err := datacenter.EastMoney.QueryFundHolderStructure(ctx, task.code)
				if err != nil || result.Latest == nil {
					if err != nil {
						logging.Warnf(ctx, "fund holder structure refresh code:%s err:%v", task.code, err)
					}
					continue
				}

				task.fund.InstitutionalHoldingRatio = result.Latest.InstitutionalHoldingRatio
				task.fund.InternalHoldingRatio = result.Latest.InternalHoldingRatio
				task.fund.HasHolderStructure = true
			}
		}()
	}
	for code, fund := range funds {
		code = strings.TrimSpace(code)
		if code == "" || fund == nil {
			continue
		}
		tasks <- holderStructureCacheTask{code: code, fund: fund}
	}
	close(tasks)
	wg.Wait()
}

func fundHolderStructureFromFund(fund *models.Fund) eastmoney.FundHolderStructureResult {
	if fund == nil || !fund.HasHolderStructure {
		return eastmoney.FundHolderStructureResult{}
	}
	return eastmoney.FundHolderStructureResult{
		Latest: &eastmoney.FundHolderStructure{
			InstitutionalHoldingRatio: fund.InstitutionalHoldingRatio,
			InternalHoldingRatio:      fund.InternalHoldingRatio,
		},
	}
}
