package routes

import (
	"fmt"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/axiaoxin-com/investool/core"
	"github.com/gin-gonic/gin"
)

const maxFundPortfolioTianTianXLSXSize = 5 << 20

func FundPortfolioTianTianImportXLSX(c *gin.Context) {
	header, err := requireFundPortfolioTianTianXLSX(c)
	data := newFundPortfolioTianTianViewData()
	if err != nil {
		data.Error = err.Error()
		c.HTML(http.StatusOK, "fund_portfolio_tiantian.html", data)
		return
	}

	file, err := header.Open()
	if err != nil {
		data.Error = err.Error()
		c.HTML(http.StatusOK, "fund_portfolio_tiantian.html", data)
		return
	}
	defer file.Close()

	report, err := core.ParseFundPortfolioTianTianXLSX(file)
	if err != nil {
		data.Error = err.Error()
		c.HTML(http.StatusOK, "fund_portfolio_tiantian.html", data)
		return
	}

	data.ImportPreview = report.Drafts
	data.ImportWarnings = report.Warnings
	if c.PostForm("action") == fundPortfolioTianTianImportActionSave {
		data.ImportResult = saveTianTianImportDrafts(report.Drafts)
	} else {
		data.ImportResult = previewTianTianXLSXImportDrafts(header.Filename, report.Drafts, report.Warnings)
	}
	c.HTML(http.StatusOK, "fund_portfolio_tiantian.html", data)
}

func requireFundPortfolioTianTianXLSX(c *gin.Context) (*multipart.FileHeader, error) {
	header, err := c.FormFile("holding_xlsx")
	if err != nil {
		return nil, fmt.Errorf("请选择天天基金导出的 xlsx 文件")
	}
	if header.Size <= 0 {
		return nil, fmt.Errorf("xlsx 文件为空")
	}
	if header.Size > maxFundPortfolioTianTianXLSXSize {
		return nil, fmt.Errorf("xlsx 文件不能超过 5MB")
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".xlsx" {
		return nil, fmt.Errorf("仅支持 .xlsx 文件: %s", ext)
	}
	return header, nil
}

func previewTianTianXLSXImportDrafts(
	filename string,
	drafts []core.FundPortfolioTianTianHoldingDraft,
	warnings []string,
) *fundPortfolioTianTianImportResult {
	result := previewTianTianImportDrafts(drafts, warnings)
	result.Title = "天天基金 Excel 识别结果"
	result.Details = append([]string{
		fmt.Sprintf("文件 %s 识别到 %d 只基金，当前还没有写入本地持仓文件。", filename, len(drafts)),
		"金额会作为当前总值；只有在持仓收益已确认时，才会结合最新净值估算份额并反推成本净值。",
	}, result.Details[1:]...)
	return result
}
