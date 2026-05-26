package core

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/axiaoxin-com/investool/models"
)

type FundPortfolioExposureReport struct {
	TotalCurrentAmount float64
	CoveredAmount      float64
	CoveredWeight      float64
	ThemeExposures     []FundPortfolioThemeExposure
	ETFLookThroughs    []FundPortfolioETFLookThrough
	StockExposures     []FundPortfolioStockExposure
	Warnings           []string
}

type FundPortfolioThemeExposure struct {
	Theme       string
	Amount      float64
	Weight      float64
	SourceFunds string
}

type FundPortfolioETFLookThrough struct {
	FundCode         string
	FundName         string
	CurrentAmount    float64
	CurrentWeight    float64
	TargetETFCode    string
	TargetETFName    string
	TargetStockAsset string
	LookedThrough    bool
	Status           string
}

type FundPortfolioStockExposure struct {
	StockCode       string
	StockName       string
	Industry        string
	Theme           string
	Amount          float64
	Weight          float64
	SourceFund      string
	SourceFundName  string
	TargetETFCode   string
	TargetETFName   string
	HoldingRatio    float64
	LookedThrough   bool
	ExposureSummary string
}

type fundPortfolioExposureAccumulator struct {
	theme   string
	amount  float64
	sources map[string]float64
}

func FundPortfolioTargetETFCodes(items []models.FundPortfolioItem, funds map[string]*models.Fund) []string {
	seen := map[string]struct{}{}
	codes := []string{}
	for _, item := range items {
		if !item.IsOwned() {
			continue
		}
		fund := funds[item.Code]
		if fund == nil || !isFundCode(fund.TargetETFCode) {
			continue
		}
		if _, ok := seen[fund.TargetETFCode]; ok {
			continue
		}
		seen[fund.TargetETFCode] = struct{}{}
		codes = append(codes, fund.TargetETFCode)
	}
	sort.Strings(codes)
	return codes
}

func BuildFundPortfolioExposureReport(
	items []models.FundPortfolioItem,
	funds map[string]*models.Fund,
	targetFunds map[string]*models.Fund,
) FundPortfolioExposureReport {
	report := FundPortfolioExposureReport{
		TotalCurrentAmount: CalculatePortfolioCurrentTotal(items, funds),
		Warnings:           []string{},
	}
	if report.TotalCurrentAmount <= 0 {
		report.Warnings = append(report.Warnings, "没有可用于计算暴露的已持有基金金额")
		return report
	}

	themeAccumulators := map[string]*fundPortfolioExposureAccumulator{}
	for _, item := range items {
		if !item.IsOwned() {
			continue
		}

		fund := funds[item.Code]
		if fund == nil {
			report.Warnings = append(report.Warnings, fmt.Sprintf("%s 缺少基金详情，无法计算暴露", item.Code))
			continue
		}
		currentAmount, _, ok := estimateFundCurrentAmount(item, fund)
		if !ok {
			report.Warnings = append(report.Warnings, fmt.Sprintf("%s 缺少当前金额，无法计算暴露", item.Code))
			continue
		}

		exposureFund := fund
		sourceLabel := fundExposureSourceLabel(fund.Code, "")
		lookedThrough := false
		if isFundCode(fund.TargetETFCode) {
			targetFund := targetFunds[fund.TargetETFCode]
			lookThrough := FundPortfolioETFLookThrough{
				FundCode:         fund.Code,
				FundName:         fund.Name,
				CurrentAmount:    currentAmount,
				CurrentWeight:    currentAmount / report.TotalCurrentAmount * 100,
				TargetETFCode:    fund.TargetETFCode,
				TargetETFName:    firstNonEmpty(fund.TargetETFName, fund.TargetETFCode),
				TargetStockAsset: "--",
			}
			if targetFund != nil {
				exposureFund = targetFund
				lookedThrough = true
				sourceLabel = fundExposureSourceLabel(fund.Code, targetFund.Code)
				lookThrough.TargetETFName = firstNonEmpty(targetFund.Name, lookThrough.TargetETFName)
				lookThrough.TargetStockAsset = firstNonEmpty(targetFund.AssetsProportion.Stock, "--")
				lookThrough.LookedThrough = true
				lookThrough.Status = "已按目标ETF穿透"
			} else {
				lookThrough.Status = "未获取到目标ETF详情，使用基金名称估算主题"
				report.Warnings = append(report.Warnings, fmt.Sprintf("%s 的目标ETF %s 未获取到详情", fund.Code, fund.TargetETFCode))
			}
			report.ETFLookThroughs = append(report.ETFLookThroughs, lookThrough)
		}

		coveredAmount := addFundThemeExposure(themeAccumulators, exposureFund, fund, currentAmount, sourceLabel)
		if coveredAmount <= 0 {
			theme := inferFundTheme(fund, "", "")
			addThemeExposure(themeAccumulators, theme, currentAmount, sourceLabel)
			coveredAmount = currentAmount
			report.Warnings = append(report.Warnings, fmt.Sprintf("%s 缺少行业持仓数据，已按基金名称归类为%s", fund.Code, theme))
		}
		report.CoveredAmount += coveredAmount

		report.StockExposures = append(report.StockExposures, buildFundStockExposures(fund, exposureFund, currentAmount, report.TotalCurrentAmount, lookedThrough)...)
	}

	report.CoveredWeight = report.CoveredAmount / report.TotalCurrentAmount * 100
	report.ThemeExposures = buildThemeExposureRows(themeAccumulators, report.TotalCurrentAmount)
	sort.Slice(report.ETFLookThroughs, func(i, j int) bool {
		return report.ETFLookThroughs[i].CurrentWeight > report.ETFLookThroughs[j].CurrentWeight
	})
	sort.Slice(report.StockExposures, func(i, j int) bool {
		return report.StockExposures[i].Weight > report.StockExposures[j].Weight
	})
	if len(report.StockExposures) > 20 {
		report.StockExposures = report.StockExposures[:20]
	}
	return report
}

func (r FundPortfolioExposureReport) HasData() bool {
	return len(r.ThemeExposures) > 0 || len(r.ETFLookThroughs) > 0 || len(r.StockExposures) > 0
}

func (r FundPortfolioExposureReport) CoveredWeightText() string {
	if r.TotalCurrentAmount <= 0 {
		return "--"
	}
	return fmt.Sprintf("%.1f%%", r.CoveredWeight)
}

func (e FundPortfolioThemeExposure) AmountText() string {
	return fmt.Sprintf("%.2f", e.Amount)
}

func (e FundPortfolioThemeExposure) WeightText() string {
	return fmt.Sprintf("%.1f%%", e.Weight)
}

func (e FundPortfolioETFLookThrough) CurrentWeightText() string {
	return fmt.Sprintf("%.1f%%", e.CurrentWeight)
}

func (e FundPortfolioETFLookThrough) CurrentAmountText() string {
	return fmt.Sprintf("%.2f", e.CurrentAmount)
}

func (e FundPortfolioStockExposure) AmountText() string {
	return fmt.Sprintf("%.2f", e.Amount)
}

func (e FundPortfolioStockExposure) WeightText() string {
	return fmt.Sprintf("%.2f%%", e.Weight)
}

func addFundThemeExposure(
	accumulators map[string]*fundPortfolioExposureAccumulator,
	exposureFund *models.Fund,
	originalFund *models.Fund,
	currentAmount float64,
	sourceLabel string,
) float64 {
	coveredAmount := 0.0
	for _, industry := range exposureFund.IndustryProportions {
		if strings.TrimSpace(industry.Industry) == "" || industry.Industry == "合计" {
			continue
		}
		ratio := parsePercentValue(industry.Prop)
		if ratio <= 0 {
			continue
		}
		amount := currentAmount * ratio / 100
		theme := inferFundTheme(originalFund, industry.Industry, "")
		addThemeExposure(accumulators, theme, amount, sourceLabel)
		coveredAmount += amount
	}
	if coveredAmount > 0 {
		return coveredAmount
	}

	for _, stock := range exposureFund.Stocks {
		if stock.HoldRatio <= 0 {
			continue
		}
		amount := currentAmount * stock.HoldRatio / 100
		theme := inferFundTheme(originalFund, stock.Industry, stock.Name)
		addThemeExposure(accumulators, theme, amount, sourceLabel)
		coveredAmount += amount
	}
	return coveredAmount
}

func addThemeExposure(accumulators map[string]*fundPortfolioExposureAccumulator, theme string, amount float64, sourceLabel string) {
	if theme == "" || amount <= 0 {
		return
	}
	acc := accumulators[theme]
	if acc == nil {
		acc = &fundPortfolioExposureAccumulator{
			theme:   theme,
			sources: map[string]float64{},
		}
		accumulators[theme] = acc
	}
	acc.amount += amount
	acc.sources[sourceLabel] += amount
}

func buildThemeExposureRows(accumulators map[string]*fundPortfolioExposureAccumulator, totalAmount float64) []FundPortfolioThemeExposure {
	rows := make([]FundPortfolioThemeExposure, 0, len(accumulators))
	for _, acc := range accumulators {
		rows = append(rows, FundPortfolioThemeExposure{
			Theme:       acc.theme,
			Amount:      acc.amount,
			Weight:      acc.amount / totalAmount * 100,
			SourceFunds: summarizeExposureSources(acc.sources, totalAmount),
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Weight > rows[j].Weight
	})
	return rows
}

func buildFundStockExposures(
	originalFund *models.Fund,
	exposureFund *models.Fund,
	currentAmount float64,
	totalAmount float64,
	lookedThrough bool,
) []FundPortfolioStockExposure {
	rows := []FundPortfolioStockExposure{}
	for _, stock := range exposureFund.Stocks {
		if stock.HoldRatio <= 0 {
			continue
		}
		amount := currentAmount * stock.HoldRatio / 100
		targetETFCode := ""
		targetETFName := ""
		if lookedThrough {
			targetETFCode = exposureFund.Code
			targetETFName = exposureFund.Name
		}
		row := FundPortfolioStockExposure{
			StockCode:      stock.Code,
			StockName:      stock.Name,
			Industry:       stock.Industry,
			Theme:          inferFundTheme(originalFund, stock.Industry, stock.Name),
			Amount:         amount,
			Weight:         amount / totalAmount * 100,
			SourceFund:     originalFund.Code,
			SourceFundName: originalFund.Name,
			TargetETFCode:  targetETFCode,
			TargetETFName:  targetETFName,
			HoldingRatio:   stock.HoldRatio,
			LookedThrough:  lookedThrough,
		}
		row.ExposureSummary = row.sourceText()
		rows = append(rows, row)
	}
	return rows
}

func (e FundPortfolioStockExposure) sourceText() string {
	if e.LookedThrough && e.TargetETFCode != "" {
		return fmt.Sprintf("%s -> %s 持仓 %.2f%%", e.SourceFund, e.TargetETFCode, e.HoldingRatio)
	}
	return fmt.Sprintf("%s 持仓 %.2f%%", e.SourceFund, e.HoldingRatio)
}

func summarizeExposureSources(sources map[string]float64, totalAmount float64) string {
	type sourceRow struct {
		label  string
		amount float64
	}
	rows := make([]sourceRow, 0, len(sources))
	for label, amount := range sources {
		rows = append(rows, sourceRow{label: label, amount: amount})
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].amount > rows[j].amount
	})
	if len(rows) > 3 {
		rows = rows[:3]
	}
	parts := make([]string, 0, len(rows))
	for _, row := range rows {
		parts = append(parts, fmt.Sprintf("%s %.1f%%", row.label, row.amount/totalAmount*100))
	}
	return strings.Join(parts, "；")
}

func fundExposureSourceLabel(code string, targetETFCode string) string {
	if targetETFCode != "" {
		return fmt.Sprintf("%s→%s", code, targetETFCode)
	}
	return code
}

func parsePercentValue(value string) float64 {
	normalized := strings.TrimSpace(strings.TrimSuffix(value, "%"))
	normalized = strings.ReplaceAll(normalized, ",", "")
	if normalized == "" || normalized == "--" {
		return 0
	}
	parsed, err := strconv.ParseFloat(normalized, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func inferFundTheme(fund *models.Fund, industry string, stockName string) string {
	text := strings.ToLower(strings.Join([]string{
		fund.Name,
		fund.IndexName,
		industry,
		stockName,
	}, " "))

	switch {
	case strings.Contains(text, "黄金") || strings.Contains(text, "金银珠宝") || strings.Contains(text, "贵金属"):
		return "黄金/贵金属"
	case strings.Contains(text, "海外科技") || strings.Contains(text, "纳斯达克"):
		return "海外科技"
	case strings.Contains(text, "人工智能") || strings.Contains(text, "ai") || strings.Contains(text, "云计算") || strings.Contains(text, "机器人"):
		return "AI/数字科技"
	case strings.Contains(text, "科创") || strings.Contains(text, "半导体") || strings.Contains(text, "芯片") || strings.Contains(text, "电子"):
		return "科创/半导体"
	case strings.Contains(text, "新能源") || strings.Contains(text, "能源革新") || strings.Contains(text, "电力设备"):
		return "新能源/电力设备"
	case strings.Contains(text, "绿色电力") || strings.Contains(text, "公用事业") || strings.Contains(text, "电力、热力"):
		return "绿色电力/公用事业"
	case strings.Contains(text, "石油") || strings.Contains(text, "石化") || strings.Contains(text, "化工"):
		return "石油化工"
	case strings.Contains(text, "量化") || strings.Contains(text, "多因子"):
		return "量化多因子"
	case strings.Contains(text, "有色金属"):
		return "有色金属"
	case strings.Contains(text, "通信") || strings.Contains(text, "计算机") || strings.Contains(text, "信息技术"):
		return "科技硬件/软件"
	case strings.Contains(text, "医药"):
		return "医药"
	case strings.Contains(text, "汽车") || strings.Contains(text, "机械") || strings.Contains(text, "制造"):
		return "制造/高端制造"
	case strings.Contains(text, "采矿"):
		return "资源/采矿"
	default:
		if strings.TrimSpace(industry) != "" {
			return industry
		}
		return "其他/未分类"
	}
}

func isFundCode(code string) bool {
	if len(code) != 6 {
		return false
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" && strings.TrimSpace(value) != "--" {
			return value
		}
	}
	return ""
}
