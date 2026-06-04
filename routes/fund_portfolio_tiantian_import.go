package routes

import (
	"fmt"

	"github.com/axiaoxin-com/investool/core"
	"github.com/axiaoxin-com/investool/models"
)

const fundPortfolioTianTianImportActionSave = "save"

func previewTianTianImportDrafts(
	drafts []core.FundPortfolioTianTianHoldingDraft,
	warnings []string,
) *fundPortfolioTianTianImportResult {
	result := &fundPortfolioTianTianImportResult{
		Title:       "天天基金识别结果",
		Status:      "识别预览",
		StatusClass: "is-neutral",
		Details: []string{
			fmt.Sprintf("识别到 %d 只基金，当前还没有写入本地持仓文件。", len(drafts)),
			"确认字段无误后，可以点击保存按钮写入本地持仓。",
		},
		NextSteps: []string{
			"如果金额、份额或成本字段为空，请确认导出的 Excel 是否已经完成份额确认。",
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
		Title:       "天天基金导入结果",
		Status:      "已处理",
		StatusClass: "is-neutral",
	}

	store := newFundPortfolioStore()
	portfolio, err := store.Load()
	if err != nil {
		result.Status = "导入失败"
		result.StatusClass = "is-error"
		result.Failed = true
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
			item.Note = "Tiantian import"
		}
		if err := store.Upsert(item); err != nil {
			result.SkippedCount++
			result.Failed = true
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
	if result.Failed && result.AddedCount+result.UpdatedCount == 0 {
		result.Status = "导入失败"
		result.StatusClass = "is-error"
	}
	result.NextSteps = []string{
		"回到“我的基金”页面后，可以在已持有列表中查看评分、仓位和图表分析。",
	}
	return result
}
