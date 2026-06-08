package sina

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/axiaoxin-com/goutils"
	"github.com/axiaoxin-com/logging"
	"github.com/corpix/uarand"
	"go.uber.org/zap"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

const MarketQuoteCategoryIndustry = "industry"

// QueryIndustryBoardMovers queries Sina industry-board movers.
func (s Sina) QueryIndustryBoardMovers(ctx context.Context, limit int, ascending bool) ([]MarketQuote, error) {
	if limit <= 0 {
		limit = 10
	}
	apiurl := "https://vip.stock.finance.sina.com.cn/q/view/newSinaHy.php"
	logging.Debug(ctx, "Sina QueryIndustryBoardMovers "+apiurl+" begin")
	beginTime := time.Now()
	header := map[string]string{
		"User-Agent": uarand.GetRandom(),
		"Referer":    "https://vip.stock.finance.sina.com.cn/mkt/frames/sl_bk.html",
	}
	resp, err := goutils.HTTPGETRaw(ctx, s.HTTPClient, apiurl, header)
	latency := time.Since(beginTime).Milliseconds()
	logging.Debug(ctx, "Sina QueryIndustryBoardMovers "+apiurl+" end", zap.Int64("latency(ms)", latency))
	if err != nil {
		return nil, err
	}
	text, err := decodeSinaIndustryBoardResponse(resp)
	if err != nil {
		return nil, err
	}
	quotes, err := parseSinaIndustryBoardResponse(text)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(quotes, func(i, j int) bool {
		if ascending {
			return quotes[i].ChangePercent < quotes[j].ChangePercent
		}
		return quotes[i].ChangePercent > quotes[j].ChangePercent
	})
	if len(quotes) > limit {
		quotes = quotes[:limit]
	}
	return quotes, nil
}

func decodeSinaIndustryBoardResponse(resp []byte) (string, error) {
	if utf8.Valid(resp) {
		return string(resp), nil
	}
	trans := transform.NewReader(bytes.NewReader(resp), simplifiedchinese.GBK.NewDecoder())
	decoded, err := ioutil.ReadAll(trans)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

func parseSinaIndustryBoardResponse(text string) ([]MarketQuote, error) {
	payload, err := sinaIndustryBoardPayload(text)
	if err != nil {
		return nil, err
	}
	raw := map[string]string{}
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		return nil, err
	}
	quotes := make([]MarketQuote, 0, len(raw))
	for key, value := range raw {
		quote, ok := parseSinaIndustryBoardValue(key, value)
		if ok {
			quotes = append(quotes, quote)
		}
	}
	if len(quotes) == 0 {
		return nil, fmt.Errorf("sina industry board empty")
	}
	return quotes, nil
}

func sinaIndustryBoardPayload(text string) (string, error) {
	eq := strings.Index(text, "=")
	if eq < 0 {
		return "", fmt.Errorf("sina industry assignment invalid")
	}
	payload := strings.TrimSpace(text[eq+1:])
	payload = strings.TrimSuffix(payload, ";")
	payload = strings.TrimSpace(payload)
	if !strings.HasPrefix(payload, "{") || !strings.HasSuffix(payload, "}") {
		return "", fmt.Errorf("sina industry payload invalid")
	}
	return payload, nil
}

func parseSinaIndustryBoardValue(key string, value string) (MarketQuote, bool) {
	fields := strings.Split(value, ",")
	if len(fields) < 8 {
		return MarketQuote{}, false
	}
	code := strings.TrimSpace(fields[0])
	if code == "" {
		code = strings.TrimSpace(key)
	}
	name := strings.TrimSpace(fields[1])
	if name == "" {
		return MarketQuote{}, false
	}
	return MarketQuote{
		Symbol:        code,
		Name:          name,
		Category:      MarketQuoteCategoryIndustry,
		Price:         parseSinaFloat(fields[3]),
		ChangeAmount:  parseSinaFloat(fields[4]),
		ChangePercent: parseSinaFloat(fields[5]),
		Turnover:      parseSinaFloat(fields[7]),
	}, true
}
