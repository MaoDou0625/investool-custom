package core

import (
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/360EntSecGroup-Skylar/excelize/v2"
	"github.com/axiaoxin-com/investool/models"
)

type tiantianXLSXColumnMap struct {
	Code        int
	Name        int
	Type        int
	UnitNAV     int
	NAVDate     int
	Amount      int
	Profit      int
	ProfitRatio int
}

func ParseFundPortfolioTianTianXLSX(r io.Reader) (FundPortfolioTianTianImportReport, error) {
	report := FundPortfolioTianTianImportReport{}
	workbook, err := excelize.OpenReader(r)
	if err != nil {
		return report, fmt.Errorf("无法读取天天基金 Excel 文件: %w", err)
	}

	for _, sheet := range workbook.GetSheetList() {
		rows, err := workbook.GetRows(sheet)
		if err != nil {
			return report, err
		}
		drafts := parseTianTianXLSXRows(rows)
		report.Drafts = append(report.Drafts, drafts...)
	}

	if len(report.Drafts) == 0 {
		return report, fmt.Errorf("未识别到天天基金持仓行，请确认上传的是天天基金导出的基金持仓 xlsx")
	}
	for _, draft := range report.Drafts {
		if !draft.HasImportableData() {
			report.Warnings = append(report.Warnings, fmt.Sprintf("%s 未识别到金额、份额或成本字段", draft.DisplayName()))
		}
	}
	return report, nil
}

func parseTianTianXLSXRows(rows [][]string) []FundPortfolioTianTianHoldingDraft {
	drafts := []FundPortfolioTianTianHoldingDraft{}
	if len(rows) == 0 {
		return drafts
	}

	headerIdx, columns, ok := findTianTianXLSXHeader(rows)
	if !ok {
		return drafts
	}
	seen := map[string]bool{}
	for _, row := range rows[headerIdx+1:] {
		code := cellAt(row, columns.Code)
		if len(findTianTianFundCodes(code)) != 1 {
			continue
		}
		code = findTianTianFundCodes(code)[0]
		if seen[code] {
			continue
		}
		seen[code] = true
		drafts = append(drafts, parseTianTianXLSXRow(row, columns, code))
	}
	return drafts
}

func findTianTianXLSXHeader(rows [][]string) (int, tiantianXLSXColumnMap, bool) {
	for idx, row := range rows {
		columns := tiantianXLSXColumnMap{
			Code:        -1,
			Name:        -1,
			Type:        -1,
			UnitNAV:     -1,
			NAVDate:     -1,
			Amount:      -1,
			Profit:      -1,
			ProfitRatio: -1,
		}
		for colIdx, cell := range row {
			header := normalizeTianTianXLSXHeader(cell)
			switch {
			case strings.Contains(header, "产品代码") || strings.Contains(header, "基金代码"):
				columns.Code = colIdx
			case strings.Contains(header, "产品名称") || strings.Contains(header, "基金名称"):
				columns.Name = colIdx
			case strings.Contains(header, "产品类型") || strings.Contains(header, "基金类型"):
				columns.Type = colIdx
			case strings.Contains(header, "最新净值") || strings.Contains(header, "单位净值"):
				columns.UnitNAV = colIdx
			case strings.Contains(header, "净值日期"):
				columns.NAVDate = colIdx
			case strings.Contains(header, "金额") || strings.Contains(header, "市值"):
				columns.Amount = colIdx
			case strings.Contains(header, "持仓收益") && strings.Contains(header, "%"):
				columns.ProfitRatio = colIdx
			case strings.Contains(header, "持仓收益"):
				columns.Profit = colIdx
			}
		}
		if columns.Code >= 0 && columns.Name >= 0 && columns.Amount >= 0 {
			return idx, columns, true
		}
	}
	return -1, tiantianXLSXColumnMap{}, false
}

func parseTianTianXLSXRow(row []string, columns tiantianXLSXColumnMap, code string) FundPortfolioTianTianHoldingDraft {
	draft := FundPortfolioTianTianHoldingDraft{
		Item: models.FundPortfolioItem{
			Code:   code,
			Status: models.FundPortfolioStatusOwned,
		},
		Name: strings.TrimSpace(cellAt(row, columns.Name)),
	}

	unitNAV, hasUnitNAV := parseTianTianXLSXNumber(cellAt(row, columns.UnitNAV), 4)
	if hasUnitNAV {
		draft.UnitNAV = unitNAV
	}
	draft.NAVDate = strings.TrimSpace(cellAt(row, columns.NAVDate))

	currentAmount, hasCurrentAmount := parseTianTianXLSXNumber(cellAt(row, columns.Amount), 2)
	if hasCurrentAmount {
		draft.Item.CurrentAmount = currentAmount
	}
	profitAmount, hasProfitAmount := parseTianTianXLSXNumber(cellAt(row, columns.Profit), 2)
	profitRatio, hasProfitRatio := parseTianTianXLSXNumber(cellAt(row, columns.ProfitRatio), 4)

	if hasCurrentAmount && hasUnitNAV && unitNAV > 0 {
		draft.Item.HoldingShares = currentAmount / unitNAV
		draft.Warnings = append(draft.Warnings, "持有份额由金额 ÷ 最新净值估算")
	}
	if hasCurrentAmount && hasProfitAmount && draft.Item.HoldingShares > 0 {
		costAmount := currentAmount - profitAmount
		if costAmount > 0 {
			draft.Item.CostNav = costAmount / draft.Item.HoldingShares
			draft.Warnings = append(draft.Warnings, "成本净值由金额、持仓收益和估算份额反推")
		}
	}
	if draft.Item.CostNav <= 0 {
		draft.Warnings = append(draft.Warnings, "Excel 未提供可反推成本净值的持仓收益，收益率暂无法精确计算")
	}

	draft.Item.Note = buildTianTianXLSXNote(row, columns, draft, profitAmount, hasProfitAmount, profitRatio, hasProfitRatio)
	return draft
}

func parseTianTianXLSXNumber(value string, maxDecimals int) (float64, bool) {
	value = strings.TrimSpace(value)
	if value == "" || value == "--" {
		return 0, false
	}
	value = strings.TrimSuffix(value, "%")
	parsed, ok := parseScreenshotNumber(value, maxDecimals)
	if !ok || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return 0, false
	}
	return parsed, true
}

func normalizeTianTianXLSXHeader(value string) string {
	value = normalizeFundTianTianImportText(value)
	replacer := strings.NewReplacer(" ", "", "\t", "", "\r", "", "\n", "", "(", "", ")", "", "（", "", "）", "")
	return replacer.Replace(value)
}

func cellAt(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

func buildTianTianXLSXNote(
	row []string,
	columns tiantianXLSXColumnMap,
	draft FundPortfolioTianTianHoldingDraft,
	profitAmount float64,
	hasProfitAmount bool,
	profitRatio float64,
	hasProfitRatio bool,
) string {
	parts := []string{"Tiantian Excel import"}
	if fundType := strings.TrimSpace(cellAt(row, columns.Type)); fundType != "" {
		parts = append(parts, "type "+fundType)
	}
	if draft.UnitNAV > 0 {
		nav := fmt.Sprintf("NAV %.4f", draft.UnitNAV)
		if draft.NAVDate != "" {
			nav += " on " + draft.NAVDate
		}
		parts = append(parts, nav)
	}
	if hasProfitAmount {
		parts = append(parts, fmt.Sprintf("profit %.2f", profitAmount))
	}
	if hasProfitRatio {
		parts = append(parts, fmt.Sprintf("profit ratio %.4f%%", profitRatio))
	}
	return strings.Join(parts, "; ")
}
