package eastmoney

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/axiaoxin-com/goutils"
	"github.com/axiaoxin-com/logging"
	"go.uber.org/zap"
)

type respPCCompanySurvey struct {
	Basic []struct {
		Secucode         string `json:"SECUCODE"`
		SecurityNameAbbr string `json:"SECURITY_NAME_ABBR"`
		OrgName          string `json:"ORG_NAME"`
		Industry         string `json:"EM2016"`
		CSRCIndustry     string `json:"INDUSTRYCSRC1"`
		Profile          string `json:"ORG_PROFILE"`
		BusinessScope    string `json:"BUSINESS_SCOPE"`
	} `json:"jbzl"`
}

type respPCBusinessAnalysis struct {
	MainBusiness []struct {
		BusinessScope string `json:"BUSINESS_SCOPE"`
	} `json:"zyfw"`
	MainForms []pcBusinessMainForm `json:"zygcfx"`
}

type pcBusinessMainForm struct {
	ReportDate         string   `json:"REPORT_DATE"`
	MainopType         string   `json:"MAINOP_TYPE"`
	ItemName           string   `json:"ITEM_NAME"`
	MainBusinessIncome *float64 `json:"MAIN_BUSINESS_INCOME"`
	MBIRatio           *float64 `json:"MBI_RATIO"`
}

type respPCOperationsRequired struct {
	Boards []struct {
		BoardName string `json:"BOARD_NAME"`
	} `json:"ssbk"`
	CoreThemes []struct {
		Keyword          string `json:"KEYWORD"`
		MainpointContent string `json:"MAINPOINT_CONTENT"`
		IsPoint          string `json:"IS_POINT"`
	} `json:"hxtc"`
}

func (e EastMoney) queryCompanyProfileFromPC(ctx context.Context, secuCode string) (CompanyProfile, error) {
	profile, err := e.queryPCCompanySurvey(ctx, secuCode)
	if err != nil {
		return CompanyProfile{}, err
	}

	if business, err := e.queryPCBusinessAnalysis(ctx, secuCode); err == nil {
		if business.MainBusiness != "" {
			profile.MainBusiness = business.MainBusiness
		}
		profile.MainForms = business.MainForms
	} else {
		logging.Warnf(ctx, "EastMoney queryPCBusinessAnalysis fallback failed:%v", err)
	}

	if operations, err := e.queryPCOperationsRequired(ctx, secuCode); err == nil {
		if len(operations.Keywords) > 0 {
			profile.Keywords = operations.Keywords
		}
		if profile.Concept == "" {
			profile.Concept = strings.Join(operations.Concepts, ";")
		}
		if profile.MainBusiness == "" {
			profile.MainBusiness = operations.MainBusiness
		}
	} else {
		logging.Warnf(ctx, "EastMoney queryPCOperationsRequired fallback failed:%v", err)
	}

	return profile, nil
}

func mergeCompanyProfileFallback(base CompanyProfile, fallback CompanyProfile) CompanyProfile {
	if base.Secucode == "" {
		base.Secucode = fallback.Secucode
	}
	if base.Name == "" {
		base.Name = fallback.Name
	}
	if base.Industry == "" {
		base.Industry = fallback.Industry
	}
	if base.Concept == "" {
		base.Concept = fallback.Concept
	}
	if base.Profile == "" {
		base.Profile = fallback.Profile
	}
	if base.MainBusiness == "" {
		base.MainBusiness = fallback.MainBusiness
	}
	if len(base.Keywords) == 0 {
		base.Keywords = fallback.Keywords
	}
	if len(base.MainForms) == 0 {
		base.MainForms = fallback.MainForms
	}
	return base
}

func (e EastMoney) queryPCCompanySurvey(ctx context.Context, secuCode string) (CompanyProfile, error) {
	apiurl := "https://emweb.eastmoney.com/PC_HSF10/CompanySurvey/PageAjax"
	params := map[string]string{
		"code": formatPCSecuCode(secuCode),
	}
	logging.Debug(ctx, "EastMoney queryPCCompanySurvey "+apiurl+" begin", zap.Any("params", params))
	beginTime := time.Now()
	apiurl, err := goutils.NewHTTPGetURLWithQueryString(ctx, apiurl, params)
	if err != nil {
		return CompanyProfile{}, err
	}
	resp := respPCCompanySurvey{}
	err = goutils.HTTPGET(ctx, e.HTTPClient, apiurl, nil, &resp)
	latency := time.Now().Sub(beginTime).Milliseconds()
	logging.Debug(ctx, "EastMoney queryPCCompanySurvey "+apiurl+" end", zap.Int64("latency(ms)", latency))
	if err != nil {
		return CompanyProfile{}, err
	}
	if len(resp.Basic) == 0 {
		return CompanyProfile{}, errors.New("no pc company survey data")
	}

	basic := resp.Basic[0]
	name := basic.SecurityNameAbbr
	if name == "" {
		name = basic.OrgName
	}
	industry := basic.Industry
	if industry == "" {
		industry = basic.CSRCIndustry
	}
	return CompanyProfile{
		Secucode:     basic.Secucode,
		Name:         name,
		Industry:     industry,
		Profile:      strings.TrimSpace(basic.Profile),
		MainBusiness: strings.TrimSpace(basic.BusinessScope),
	}, nil
}

type pcBusinessProfile struct {
	MainBusiness string
	MainForms    []MainForm
}

func (e EastMoney) queryPCBusinessAnalysis(ctx context.Context, secuCode string) (pcBusinessProfile, error) {
	apiurl := "https://emweb.eastmoney.com/PC_HSF10/BusinessAnalysis/PageAjax"
	params := map[string]string{
		"code": formatPCSecuCode(secuCode),
	}
	logging.Debug(ctx, "EastMoney queryPCBusinessAnalysis "+apiurl+" begin", zap.Any("params", params))
	beginTime := time.Now()
	apiurl, err := goutils.NewHTTPGetURLWithQueryString(ctx, apiurl, params)
	if err != nil {
		return pcBusinessProfile{}, err
	}
	resp := respPCBusinessAnalysis{}
	err = goutils.HTTPGET(ctx, e.HTTPClient, apiurl, nil, &resp)
	latency := time.Now().Sub(beginTime).Milliseconds()
	logging.Debug(ctx, "EastMoney queryPCBusinessAnalysis "+apiurl+" end", zap.Int64("latency(ms)", latency))
	if err != nil {
		return pcBusinessProfile{}, err
	}

	business := pcBusinessProfile{}
	if len(resp.MainBusiness) > 0 {
		business.MainBusiness = strings.TrimSpace(resp.MainBusiness[0].BusinessScope)
	}
	business.MainForms = buildLatestPCMainForms(resp.MainForms)
	return business, nil
}

type pcOperationsProfile struct {
	Concepts     []string
	Keywords     []string
	MainBusiness string
}

func (e EastMoney) queryPCOperationsRequired(ctx context.Context, secuCode string) (pcOperationsProfile, error) {
	apiurl := "https://emweb.eastmoney.com/PC_HSF10/OperationsRequired/PageAjax"
	params := map[string]string{
		"code": formatPCSecuCode(secuCode),
	}
	logging.Debug(ctx, "EastMoney queryPCOperationsRequired "+apiurl+" begin", zap.Any("params", params))
	beginTime := time.Now()
	apiurl, err := goutils.NewHTTPGetURLWithQueryString(ctx, apiurl, params)
	if err != nil {
		return pcOperationsProfile{}, err
	}
	resp := respPCOperationsRequired{}
	err = goutils.HTTPGET(ctx, e.HTTPClient, apiurl, nil, &resp)
	latency := time.Now().Sub(beginTime).Milliseconds()
	logging.Debug(ctx, "EastMoney queryPCOperationsRequired "+apiurl+" end", zap.Int64("latency(ms)", latency))
	if err != nil {
		return pcOperationsProfile{}, err
	}

	result := pcOperationsProfile{}
	for _, board := range resp.Boards {
		if board.BoardName != "" {
			result.Concepts = append(result.Concepts, board.BoardName)
		}
	}
	for _, theme := range resp.CoreThemes {
		switch {
		case theme.Keyword == "主营业务" && theme.MainpointContent != "":
			result.MainBusiness = strings.TrimSpace(theme.MainpointContent)
		case theme.IsPoint == "1" && theme.Keyword != "":
			result.Keywords = append(result.Keywords, theme.Keyword)
		}
	}
	return result, nil
}

func buildLatestPCMainForms(items []pcBusinessMainForm) []MainForm {
	latestDate := ""
	for _, item := range items {
		if item.ReportDate > latestDate {
			latestDate = item.ReportDate
		}
	}
	if latestDate == "" {
		return nil
	}

	mainForms := []MainForm{}
	for _, item := range items {
		if item.ReportDate != latestDate {
			continue
		}
		mainForms = append(mainForms, MainForm{
			Type:            mapPCMainFormType(item.MainopType),
			MainForm:        item.ItemName,
			MainIncome:      formatPCMainIncome(item.MainBusinessIncome),
			MainIncomeRatio: formatPCMainIncomeRatio(item.MBIRatio),
		})
	}
	sort.SliceStable(mainForms, func(i, j int) bool {
		if mainForms[i].Type == mainForms[j].Type {
			return mainForms[i].MainForm < mainForms[j].MainForm
		}
		return mainForms[i].Type < mainForms[j].Type
	})
	return mainForms
}

func mapPCMainFormType(t string) string {
	switch t {
	case "2":
		return "3"
	case "3":
		return "2"
	default:
		return t
	}
}

func formatPCMainIncome(v *float64) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%.2f亿", *v/100000000)
}

func formatPCMainIncomeRatio(v *float64) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%.2f%%", *v*100)
}
