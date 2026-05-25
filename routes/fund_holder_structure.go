// 基金持有人结构

package routes

import (
	"sync"

	"github.com/axiaoxin-com/investool/datacenter"
	"github.com/axiaoxin-com/investool/datacenter/eastmoney"
	"github.com/axiaoxin-com/investool/models"
	"github.com/axiaoxin-com/logging"
	"github.com/gin-gonic/gin"
)

func queryFundHolderStructures(c *gin.Context, funds map[string]*models.Fund) map[string]eastmoney.FundHolderStructureResult {
	results := map[string]eastmoney.FundHolderStructureResult{}
	var wg sync.WaitGroup
	var mu sync.Mutex
	for code := range funds {
		wg.Add(1)
		go func(code string) {
			defer wg.Done()
			result, err := datacenter.EastMoney.QueryFundHolderStructure(c, code)
			if err != nil {
				logging.Errorf(c, "QueryFundHolderStructure code:%s err:%v", code, err)
				return
			}
			mu.Lock()
			results[code] = result
			mu.Unlock()
		}(code)
	}
	wg.Wait()
	return results
}
