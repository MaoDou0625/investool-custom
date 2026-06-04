package routes

import (
	"net/http"

	"github.com/axiaoxin-com/investool/core"
	"github.com/gin-gonic/gin"
)

type fundPortfolioTianTianImportJSONResponse struct {
	Saved        bool                                   `json:"saved"`
	Failed       bool                                   `json:"failed"`
	Filename     string                                 `json:"filename"`
	DraftCount   int                                    `json:"draft_count"`
	AddedCount   int                                    `json:"added_count"`
	UpdatedCount int                                    `json:"updated_count"`
	SkippedCount int                                    `json:"skipped_count"`
	Status       string                                 `json:"status"`
	Details      []string                               `json:"details"`
	Warnings     []string                               `json:"warnings"`
	Drafts       []fundPortfolioTianTianImportJSONDraft `json:"drafts"`
}

type fundPortfolioTianTianImportJSONDraft struct {
	Code          string   `json:"code"`
	Name          string   `json:"name"`
	CurrentAmount float64  `json:"current_amount"`
	CostNAV       float64  `json:"cost_nav"`
	HoldingShares float64  `json:"holding_shares"`
	UnitNAV       float64  `json:"unit_nav"`
	NAVDate       string   `json:"nav_date"`
	Warnings      []string `json:"warnings"`
}

func FundPortfolioTianTianImportXLSXJSON(c *gin.Context) {
	header, err := requireFundPortfolioTianTianXLSX(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	file, err := header.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	defer file.Close()

	report, err := core.ParseFundPortfolioTianTianXLSX(file)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	save := c.DefaultPostForm("action", fundPortfolioTianTianImportActionSave) == fundPortfolioTianTianImportActionSave
	var result *fundPortfolioTianTianImportResult
	if save {
		result = saveTianTianImportDrafts(report.Drafts)
	} else {
		result = previewTianTianXLSXImportDrafts(header.Filename, report.Drafts, report.Warnings)
	}

	response := buildTianTianImportJSONResponse(header.Filename, report, result, save)
	if response.Failed {
		c.JSON(http.StatusInternalServerError, response)
		return
	}
	c.JSON(http.StatusOK, response)
}

func buildTianTianImportJSONResponse(
	filename string,
	report core.FundPortfolioTianTianImportReport,
	result *fundPortfolioTianTianImportResult,
	saved bool,
) fundPortfolioTianTianImportJSONResponse {
	response := fundPortfolioTianTianImportJSONResponse{
		Filename:   filename,
		DraftCount: len(report.Drafts),
		Warnings:   report.Warnings,
		Drafts:     buildTianTianImportJSONDrafts(report.Drafts),
	}
	if result != nil {
		response.AddedCount = result.AddedCount
		response.UpdatedCount = result.UpdatedCount
		response.SkippedCount = result.SkippedCount
		response.Status = result.Status
		response.Details = result.Details
		response.Failed = result.Failed
	}
	response.Saved = saved && response.AddedCount+response.UpdatedCount > 0
	return response
}

func buildTianTianImportJSONDrafts(
	drafts []core.FundPortfolioTianTianHoldingDraft,
) []fundPortfolioTianTianImportJSONDraft {
	result := make([]fundPortfolioTianTianImportJSONDraft, 0, len(drafts))
	for _, draft := range drafts {
		result = append(result, fundPortfolioTianTianImportJSONDraft{
			Code:          draft.Item.Code,
			Name:          draft.Name,
			CurrentAmount: draft.Item.CurrentAmount,
			CostNAV:       draft.Item.CostNav,
			HoldingShares: draft.Item.HoldingShares,
			UnitNAV:       draft.UnitNAV,
			NAVDate:       draft.NAVDate,
			Warnings:      draft.Warnings,
		})
	}
	return result
}
