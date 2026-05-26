package core

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/axiaoxin-com/investool/models"
)

type FundPortfolioScreenshotDraft struct {
	Item          models.FundPortfolioItem `json:"item"`
	TotalAmount   float64                  `json:"total_amount"`
	PendingAmount float64                  `json:"pending_amount"`
	UnitNAV       float64                  `json:"unit_nav"`
	NAVDate       string                   `json:"nav_date"`
	RawText       string                   `json:"raw_text"`
	Warnings      []string                 `json:"warnings"`
}

var (
	fundScreenshotCodePattern   = regexp.MustCompile(`(?:\d\s*){6}`)
	fundScreenshotNumberPattern = `[-+]?\d[\d,]*(?:\.\d+)?`
)

func RecognizeImageText(ctx context.Context, imagePath string) (string, error) {
	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("screenshot OCR currently requires Windows built-in OCR")
	}
	if strings.TrimSpace(imagePath) == "" {
		return "", fmt.Errorf("image path is empty")
	}
	if _, err := os.Stat(imagePath); err != nil {
		return "", err
	}

	scriptPath := filepath.Join("scripts", "windows_ocr.ps1")
	if _, err := os.Stat(scriptPath); err != nil {
		return "", fmt.Errorf("windows OCR script not found: %w", err)
	}

	powershell, err := exec.LookPath("powershell.exe")
	if err != nil {
		powershell, err = exec.LookPath("powershell")
		if err != nil {
			return "", fmt.Errorf("powershell is required for Windows OCR: %w", err)
		}
	}

	ocrCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(
		ocrCtx,
		powershell,
		"-NoProfile",
		"-ExecutionPolicy",
		"Bypass",
		"-File",
		scriptPath,
		"-ImagePath",
		imagePath,
	)
	output, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(bytes.TrimPrefix(output, []byte{0xEF, 0xBB, 0xBF})))
	if ocrCtx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("Windows OCR timed out")
	}
	if err != nil {
		return "", fmt.Errorf("Windows OCR failed: %w: %s", err, text)
	}
	if text == "" {
		return "", fmt.Errorf("Windows OCR returned empty text")
	}
	return text, nil
}

func ParseFundPortfolioScreenshotText(raw string) (FundPortfolioScreenshotDraft, error) {
	draft := FundPortfolioScreenshotDraft{
		Item: models.FundPortfolioItem{
			Status: models.FundPortfolioStatusOwned,
		},
		RawText: raw,
	}

	compact := normalizeFundScreenshotText(raw)
	code := parseFundCodeFromScreenshotText(compact)
	if code == "" {
		return draft, fmt.Errorf("unable to recognize fund code")
	}
	draft.Item.Code = code

	currentAmount, hasCurrentAmount := parseScreenshotNumberAfter(compact, `持有[金奎]?额`, 2)
	if hasCurrentAmount {
		draft.Item.CurrentAmount = currentAmount
	}

	if totalAmount, ok := parseScreenshotNumberAfter(compact, `金额(?:\(元\))?`, 2); ok {
		draft.TotalAmount = totalAmount
	}
	if pendingAmount, ok := parseScreenshotNumberAfter(compact, `待确认[金奎]?额`, 2); ok {
		draft.PendingAmount = pendingAmount
	}
	if !hasCurrentAmount && draft.TotalAmount > 0 {
		draft.Item.CurrentAmount = draft.TotalAmount - draft.PendingAmount
		draft.Warnings = append(draft.Warnings, "未识别到持有金额，已用总金额减待确认金额估算")
	}

	if costNAV, ok := parseScreenshotNumberAfter(compact, `持仓成本价`, 4); ok {
		draft.Item.CostNav = costNAV
	} else {
		draft.Warnings = append(draft.Warnings, "未识别到持仓成本价")
	}

	if shares, ok := parseScreenshotNumberAfter(compact, `持有份额`, 2); ok {
		draft.Item.HoldingShares = shares
	} else {
		draft.Warnings = append(draft.Warnings, "未识别到持有份额")
	}

	if unitNAV, navDate, ok := parseScreenshotUnitNAV(compact); ok {
		draft.UnitNAV = unitNAV
		draft.NAVDate = navDate
	}

	if draft.Item.CurrentAmount <= 0 && draft.UnitNAV > 0 && draft.Item.HoldingShares > 0 {
		draft.Item.CurrentAmount = draft.UnitNAV * draft.Item.HoldingShares
		draft.Warnings = append(draft.Warnings, "未识别到持有金额，已用基金净值乘持有份额估算")
	}
	if draft.Item.CurrentAmount <= 0 {
		draft.Warnings = append(draft.Warnings, "未识别到持有金额")
	}

	draft.Item.Note = buildFundScreenshotNote(draft)
	return draft, nil
}

func normalizeFundScreenshotText(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		if unicode.IsSpace(r) {
			continue
		}
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

func parseFundCodeFromScreenshotText(text string) string {
	match := fundScreenshotCodePattern.FindString(text)
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, match)
}

func parseScreenshotNumberAfter(text string, labelPattern string, maxDecimals int) (float64, bool) {
	re := regexp.MustCompile(labelPattern + `(?:\([^)]*\))?.{0,12}?(` + fundScreenshotNumberPattern + `)`)
	match := re.FindStringSubmatch(text)
	if len(match) < 2 {
		return 0, false
	}
	return parseScreenshotNumber(match[1], maxDecimals)
}

func parseScreenshotUnitNAV(text string) (float64, string, bool) {
	re := regexp.MustCompile(`(?:基金|基奎)净(?:值|位)(?:\(([^)]*)\))?.{0,8}?(` + fundScreenshotNumberPattern + `)`)
	match := re.FindStringSubmatch(text)
	if len(match) < 3 {
		return 0, "", false
	}
	value, ok := parseScreenshotNumber(match[2], 4)
	if !ok {
		return 0, "", false
	}
	return value, strings.Trim(match[1], "- "), true
}

func parseScreenshotNumber(value string, maxDecimals int) (float64, bool) {
	normalized := strings.ReplaceAll(value, ",", "")
	if maxDecimals > 0 {
		if dot := strings.Index(normalized, "."); dot >= 0 && len(normalized)-dot-1 > maxDecimals {
			normalized = normalized[:dot+1+maxDecimals]
		}
	}
	parsed, err := strconv.ParseFloat(normalized, 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func buildFundScreenshotNote(draft FundPortfolioScreenshotDraft) string {
	parts := []string{"OCR screenshot"}
	if draft.PendingAmount > 0 {
		parts = append(parts, fmt.Sprintf("pending %.2f", draft.PendingAmount))
	}
	if draft.UnitNAV > 0 {
		nav := fmt.Sprintf("NAV %.4f", draft.UnitNAV)
		if draft.NAVDate != "" {
			nav += " on " + draft.NAVDate
		}
		parts = append(parts, nav)
	}
	return strings.Join(parts, "; ")
}
