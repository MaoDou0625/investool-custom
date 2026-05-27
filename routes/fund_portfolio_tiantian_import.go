package routes

import (
	"fmt"
	"net/http"

	"github.com/axiaoxin-com/investool/core"
	"github.com/axiaoxin-com/investool/models"
	"github.com/gin-gonic/gin"
)

const fundPortfolioTianTianImportActionSave = "save"

func FundPortfolioTianTianImportText(c *gin.Context) {
	importText := c.PostForm("holding_text")
	action := c.PostForm("action")
	data := newFundPortfolioTianTianViewData()
	data.ImportText = importText

	report, err := core.ParseFundPortfolioTianTianText(importText)
	if err != nil {
		data.Error = err.Error()
		c.HTML(http.StatusOK, "fund_portfolio_tiantian.html", data)
		return
	}

	data.ImportPreview = report.Drafts
	data.ImportWarnings = report.Warnings
	if action == fundPortfolioTianTianImportActionSave {
		data.ImportResult = saveTianTianImportDrafts(report.Drafts)
	} else {
		data.ImportResult = previewTianTianImportDrafts(report.Drafts, report.Warnings)
	}
	c.HTML(http.StatusOK, "fund_portfolio_tiantian.html", data)
}

func previewTianTianImportDrafts(
	drafts []core.FundPortfolioTianTianHoldingDraft,
	warnings []string,
) *fundPortfolioTianTianImportResult {
	result := &fundPortfolioTianTianImportResult{
		Title:       "天天基金文本识别结果",
		Status:      "识别预览",
		StatusClass: "is-neutral",
		Details: []string{
			fmt.Sprintf("识别到 %d 只基金，当前还没有写入本地持仓文件。", len(drafts)),
			"确认字段无误后，可以点击“识别并保存完整项”。",
		},
		NextSteps: []string{
			"如果金额、份额或成本字段为空，请复制持仓详情页更多文字后再识别。",
			"保存只会写入识别到金额、份额或成本的基金，只有代码的项目会被跳过。",
		},
	}
	for _, warning := range warnings {
		result.Details = append(result.Details, warning)
	}
	return result
}

func saveTianTianImportDrafts(drafts []core.FundPortfolioTianTianHoldingDraft) *fundPortfolioTianTianImportResult {
	result := &fundPortfolioTianTianImportResult{
		Title:       "天天基金文本导入结果",
		Status:      "已处理",
		StatusClass: "is-neutral",
	}

	store := newFundPortfolioStore()
	portfolio, err := store.Load()
	if err != nil {
		result.Status = "导入失败"
		result.Details = []string{err.Error()}
		return result
	}
	existingCodes := map[string]bool{}
	for _, item := range portfolio.Items {
		existingCodes[item.Code] = true
	}

	for _, draft := range drafts {
		if !draft.HasImportableData() {
			result.SkippedCount++
			result.Details = append(result.Details, draft.DisplayName()+"：缺少金额、份额或成本字段，已跳过。")
			continue
		}

		item := draft.Item
		item.Status = models.FundPortfolioStatusOwned
		if item.Note == "" {
			item.Note = "Tiantian text import"
		}
		if err := store.Upsert(item); err != nil {
			result.SkippedCount++
			result.Details = append(result.Details, draft.DisplayName()+"："+err.Error())
			continue
		}
		if existingCodes[item.Code] {
			result.UpdatedCount++
			result.Details = append(result.Details, draft.DisplayName()+"：已更新。")
		} else {
			result.AddedCount++
			result.Details = append(result.Details, draft.DisplayName()+"：已新增。")
			existingCodes[item.Code] = true
		}
	}

	if len(result.Details) == 0 {
		result.Details = append(result.Details, "没有可保存的基金。")
	}
	result.NextSteps = []string{
		"回到“我的基金”页面后，可以在已持有列表中查看评分、仓位和图表分析。",
	}
	return result
}
