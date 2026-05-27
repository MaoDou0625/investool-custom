package core

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/axiaoxin-com/investool/models"
)

type FundPortfolioTianTianImportReport struct {
	Drafts   []FundPortfolioTianTianHoldingDraft
	Warnings []string
}

type FundPortfolioTianTianHoldingDraft struct {
	Item     models.FundPortfolioItem
	Name     string
	UnitNAV  float64
	NAVDate  string
	Warnings []string
}

var fundTianTianCodePattern = regexp.MustCompile(`(^|[^\d])(\d{6})([^\d]|$)`)

func (d FundPortfolioTianTianHoldingDraft) DisplayName() string {
	if d.Name == "" {
		return d.Item.Code
	}
	return d.Item.Code + " " + d.Name
}

func (d FundPortfolioTianTianHoldingDraft) HasImportableData() bool {
	return d.Item.Code != "" && (d.Item.CurrentAmount > 0 || d.Item.HoldingShares > 0 || d.Item.CostNav > 0)
}

func normalizeFundTianTianImportText(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		switch {
		case r >= '０' && r <= '９':
			b.WriteRune('0' + (r - '０'))
		case r == '，' || r == '、':
			b.WriteRune(',')
		case r == '．' || r == '·' || r == '。':
			b.WriteRune('.')
		case r == '（' || r == '【':
			b.WriteRune('(')
		case r == '）' || r == '】':
			b.WriteRune(')')
		case r == '％':
			b.WriteRune('%')
		case r == '－' || r == '—' || r == '一':
			b.WriteRune('-')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func findTianTianFundCodes(text string) []string {
	matches := fundTianTianCodePattern.FindAllStringSubmatch(text, -1)
	codes := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) >= 3 {
			codes = append(codes, match[2])
		}
	}
	return codes
}

func appendTianTianDraftWarnings(report *FundPortfolioTianTianImportReport) {
	for _, draft := range report.Drafts {
		if !draft.HasImportableData() {
			report.Warnings = append(report.Warnings, fmt.Sprintf("%s 未识别到金额、份额或成本字段", draft.DisplayName()))
		}
	}
}
