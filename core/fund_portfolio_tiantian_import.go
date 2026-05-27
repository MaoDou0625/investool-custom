package core

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/axiaoxin-com/investool/models"
)

type FundPortfolioTianTianImportReport struct {
	Drafts   []FundPortfolioTianTianHoldingDraft
	Warnings []string
	RawText  string
}

type FundPortfolioTianTianHoldingDraft struct {
	Item     models.FundPortfolioItem
	Name     string
	UnitNAV  float64
	NAVDate  string
	Warnings []string
}

var fundTianTianCodePattern = regexp.MustCompile(`(^|[^\d])(\d{6})([^\d]|$)`)

func ParseFundPortfolioTianTianText(raw string) (FundPortfolioTianTianImportReport, error) {
	report := FundPortfolioTianTianImportReport{
		RawText: raw,
	}
	normalized := normalizeFundTianTianImportText(raw)
	if strings.TrimSpace(normalized) == "" {
		return report, fmt.Errorf("请先粘贴天天基金持仓页文本")
	}

	lines := splitNonEmptyLines(normalized)
	seen := map[string]bool{}
	for idx, line := range lines {
		codes := findTianTianFundCodes(line)
		for _, code := range codes {
			if seen[code] {
				continue
			}
			seen[code] = true
			segment := buildTianTianFundSegment(lines, idx)
			report.Drafts = append(report.Drafts, parseTianTianFundSegment(code, segment))
		}
	}

	if len(report.Drafts) == 0 {
		return report, fmt.Errorf("未识别到 6 位基金代码，请复制持仓列表或持仓详情页文本")
	}
	for _, draft := range report.Drafts {
		if !draft.HasImportableData() {
			report.Warnings = append(report.Warnings, fmt.Sprintf("%s 未识别到金额、份额或成本字段", draft.DisplayName()))
		}
	}
	return report, nil
}

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

func splitNonEmptyLines(text string) []string {
	rawLines := strings.FieldsFunc(text, func(r rune) bool {
		return r == '\r' || r == '\n'
	})
	lines := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
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

func buildTianTianFundSegment(lines []string, codeLineIdx int) string {
	start := codeLineIdx
	if codeLineIdx > 0 && shouldIncludePreviousTianTianLine(lines[codeLineIdx], lines[codeLineIdx-1]) {
		start = codeLineIdx - 1
	}

	end := codeLineIdx + 1
	for end < len(lines) && end <= codeLineIdx+8 {
		if len(findTianTianFundCodes(lines[end])) > 0 {
			break
		}
		end++
	}
	return strings.Join(lines[start:end], "\n")
}

func shouldIncludePreviousTianTianLine(codeLine string, previousLine string) bool {
	if len(findTianTianFundCodes(previousLine)) > 0 {
		return false
	}
	trimmed := strings.TrimSpace(codeLine)
	if trimmed == "" {
		return false
	}
	digits := 0
	for _, r := range trimmed {
		if unicode.IsDigit(r) {
			digits++
		}
	}
	return digits >= 6 && digits >= len([]rune(trimmed))/2
}

func parseTianTianFundSegment(code string, segment string) FundPortfolioTianTianHoldingDraft {
	draft := FundPortfolioTianTianHoldingDraft{
		Item: models.FundPortfolioItem{
			Code:   code,
			Status: models.FundPortfolioStatusOwned,
			Note:   "Tiantian text import",
		},
		Name: parseTianTianFundName(segment, code),
	}

	compact := normalizeFundScreenshotText(segment)
	if currentAmount, ok := parseTianTianNumberAfterAny(compact, []string{
		`持有[金奎]?额`,
		`持仓市值`,
		`当前市值`,
		`资产(?:\(元\))?`,
		`金额(?:\(元\))?`,
	}, 2); ok {
		draft.Item.CurrentAmount = currentAmount
	}
	if costNAV, ok := parseTianTianNumberAfterAny(compact, []string{
		`持仓成本(?:价|净值)?`,
		`成本(?:价|净值)`,
	}, 4); ok {
		draft.Item.CostNav = costNAV
	}
	if shares, ok := parseTianTianNumberAfterAny(compact, []string{
		`持有份额`,
		`持仓份额`,
		`份额`,
	}, 2); ok {
		draft.Item.HoldingShares = shares
	}
	if unitNAV, navDate, ok := parseTianTianUnitNAV(compact); ok {
		draft.UnitNAV = unitNAV
		draft.NAVDate = navDate
	}
	if draft.Item.CurrentAmount <= 0 && draft.UnitNAV > 0 && draft.Item.HoldingShares > 0 {
		draft.Item.CurrentAmount = draft.UnitNAV * draft.Item.HoldingShares
		draft.Warnings = append(draft.Warnings, "未识别到持有金额，已用净值乘份额估算")
	}
	if !draft.HasImportableData() {
		draft.Warnings = append(draft.Warnings, "未识别到金额、份额或成本字段，暂不建议直接保存")
	}
	return draft
}

func parseTianTianNumberAfterAny(text string, labelPatterns []string, maxDecimals int) (float64, bool) {
	for _, labelPattern := range labelPatterns {
		if value, ok := parseScreenshotNumberAfter(text, labelPattern, maxDecimals); ok {
			return value, true
		}
	}
	return 0, false
}

func parseTianTianUnitNAV(text string) (float64, string, bool) {
	if unitNAV, navDate, ok := parseScreenshotUnitNAV(text); ok {
		return unitNAV, navDate, true
	}
	re := regexp.MustCompile(`(?:基金|单位)净(?:值|位).{0,8}?(` + fundScreenshotNumberPattern + `)(?:\(([^)]*)\))?`)
	match := re.FindStringSubmatch(text)
	if len(match) < 2 {
		return 0, "", false
	}
	value, ok := parseScreenshotNumber(match[1], 4)
	if !ok {
		return 0, "", false
	}
	navDate := ""
	if len(match) >= 3 {
		navDate = strings.Trim(match[2], "- ")
	}
	return value, navDate, true
}

func parseTianTianFundName(segment string, code string) string {
	lines := splitNonEmptyLines(segment)
	candidates := []string{}
	for idx, line := range lines {
		if strings.Contains(line, code) {
			candidates = append(candidates, cleanTianTianFundNameCandidate(strings.ReplaceAll(line, code, "")))
			if idx > 0 {
				candidates = append(candidates, cleanTianTianFundNameCandidate(lines[idx-1]))
			}
			if idx+1 < len(lines) {
				candidates = append(candidates, cleanTianTianFundNameCandidate(lines[idx+1]))
			}
		}
	}
	for _, candidate := range candidates {
		if isLikelyTianTianFundName(candidate) {
			return candidate
		}
	}
	return ""
}

func cleanTianTianFundNameCandidate(value string) string {
	value = strings.TrimSpace(value)
	replacements := []string{"中高风险", "中风险", "高风险", "低风险", "详情", "基金", "代码"}
	for _, replacement := range replacements {
		value = strings.ReplaceAll(value, replacement, "")
	}
	for _, label := range []string{"持有金额", "持仓成本价", "持有份额", "基金净值", "昨日收益", "持有收益"} {
		if idx := strings.Index(value, label); idx >= 0 {
			value = value[:idx]
		}
	}
	return strings.Trim(value, " \t:-：,，|")
}

func isLikelyTianTianFundName(value string) bool {
	runes := []rune(value)
	if len(runes) < 2 || len(runes) > 40 {
		return false
	}
	hasCJK := false
	for _, r := range runes {
		if r >= '\u4e00' && r <= '\u9fff' {
			hasCJK = true
			break
		}
	}
	if !hasCJK {
		return false
	}
	for _, blocked := range []string{"金额", "份额", "收益", "净值", "成本", "日期"} {
		if strings.Contains(value, blocked) {
			return false
		}
	}
	return true
}
