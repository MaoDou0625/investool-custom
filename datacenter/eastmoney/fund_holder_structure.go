// 天天基金基金持有人结构

package eastmoney

import (
	"context"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/axiaoxin-com/goutils"
	"github.com/axiaoxin-com/logging"
	"github.com/corpix/uarand"
	"go.uber.org/zap"
)

// FundHolderStructure 基金持有人结构
type FundHolderStructure struct {
	// 公告日期
	AnnouncementDate string `json:"announcement_date"`
	// 机构持有比例（%）
	InstitutionalHoldingRatio float64 `json:"institutional_holding_ratio"`
	// 个人持有比例（%）
	PersonalHoldingRatio float64 `json:"personal_holding_ratio"`
	// 内部持有比例（%）
	InternalHoldingRatio float64 `json:"internal_holding_ratio"`
	// 总份额（亿份）
	TotalShare float64 `json:"total_share"`
}

// FundHolderStructures 基金持有人结构列表
type FundHolderStructures []FundHolderStructure

// FundHolderStructureResult 基金持有人结构结果
type FundHolderStructureResult struct {
	Summary string               `json:"summary"`
	Latest  *FundHolderStructure `json:"latest,omitempty"`
	Items   FundHolderStructures `json:"items"`
}

func newFundHolderStructureResult(summary string, items FundHolderStructures) FundHolderStructureResult {
	result := FundHolderStructureResult{
		Summary: summary,
		Items:   items,
	}
	if len(items) > 0 {
		result.Latest = &items[0]
	}
	return result
}

// QueryFundHolderStructure 查询基金持有人结构
func (e EastMoney) QueryFundHolderStructure(ctx context.Context, fundCode string) (FundHolderStructureResult, error) {
	apiurl := "http://fundf10.eastmoney.com/FundArchivesDatas.aspx"
	params := map[string]string{
		"type": "cyrjg",
		"code": fundCode,
	}
	logging.Debug(ctx, "EastMoney QueryFundHolderStructure "+apiurl+" begin", zap.Any("params", params))
	beginTime := time.Now()
	apiurl, err := goutils.NewHTTPGetURLWithQueryString(ctx, apiurl, params)
	if err != nil {
		return FundHolderStructureResult{}, err
	}
	header := map[string]string{
		"user-agent": uarand.GetRandom(),
	}
	resp, err := goutils.HTTPGETRaw(ctx, e.HTTPClient, apiurl, header)
	latency := time.Now().Sub(beginTime).Milliseconds()
	logging.Debug(
		ctx,
		"EastMoney QueryFundHolderStructure "+apiurl+" end",
		zap.Int64("latency(ms)", latency),
	)
	if err != nil {
		return FundHolderStructureResult{}, err
	}
	return parseFundHolderStructureResponse(string(resp))
}

func parseFundHolderStructureResponse(resp string) (FundHolderStructureResult, error) {
	summary := extractFundHolderStructureField(resp, "summary")
	content := extractFundHolderStructureField(resp, "content")
	if content == "" {
		if summary != "" {
			return newFundHolderStructureResult(summary, nil), nil
		}
		return FundHolderStructureResult{}, fmt.Errorf("invalid fund holder structure response")
	}

	rowRegexp := regexp.MustCompile(`(?is)<tr>\s*<td[^>]*>(.*?)</td>\s*<td[^>]*>(.*?)</td>\s*<td[^>]*>(.*?)</td>\s*<td[^>]*>(.*?)</td>\s*<td[^>]*>(.*?)</td>\s*</tr>`)
	rows := rowRegexp.FindAllStringSubmatch(html.UnescapeString(content), -1)
	items := FundHolderStructures{}
	for _, row := range rows {
		item := FundHolderStructure{
			AnnouncementDate:          cleanFundHolderStructureText(row[1]),
			InstitutionalHoldingRatio: parseFundHolderStructureFloat(row[2]),
			PersonalHoldingRatio:      parseFundHolderStructureFloat(row[3]),
			InternalHoldingRatio:      parseFundHolderStructureFloat(row[4]),
			TotalShare:                parseFundHolderStructureFloat(row[5]),
		}
		items = append(items, item)
	}
	return newFundHolderStructureResult(html.UnescapeString(summary), items), nil
}

func extractFundHolderStructureField(resp, field string) string {
	reg := regexp.MustCompile(`(?is)` + field + `:"(.*?)"[,}]`)
	matched := reg.FindStringSubmatch(resp)
	if len(matched) != 2 {
		return ""
	}
	return matched[1]
}

func cleanFundHolderStructureText(raw string) string {
	tagRegexp := regexp.MustCompile(`(?is)<[^>]+>`)
	text := tagRegexp.ReplaceAllString(raw, "")
	return strings.TrimSpace(html.UnescapeString(text))
}

func parseFundHolderStructureFloat(raw string) float64 {
	text := cleanFundHolderStructureText(raw)
	text = strings.TrimSuffix(text, "%")
	text = strings.TrimSuffix(text, "亿份")
	if text == "" || text == "--" {
		return 0
	}
	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0
	}
	return value
}
