package core

import (
	"sort"
	"strings"
	"time"
)

type FundDailyWorkdayCalendar struct {
	NonWorkdayDates []string
	WorkdayDates    []string
}

type FundDailyWorkdayGuard struct {
	Enabled          bool   `json:"enabled"`
	Date             string `json:"date"`
	Weekday          string `json:"weekday"`
	IsWorkday        bool   `json:"is_workday"`
	AllowBuy         bool   `json:"allow_buy"`
	CalendarOverride string `json:"calendar_override,omitempty"`
	Reason           string `json:"reason,omitempty"`
}

func DefaultFundDailyWorkdayCalendar() FundDailyWorkdayCalendar {
	return FundDailyWorkdayCalendar{
		NonWorkdayDates: []string{
			"2026-01-01", "2026-01-02", "2026-01-03",
			"2026-02-15", "2026-02-16", "2026-02-17", "2026-02-18", "2026-02-19", "2026-02-20", "2026-02-21", "2026-02-22", "2026-02-23",
			"2026-04-04", "2026-04-05", "2026-04-06",
			"2026-05-01", "2026-05-02", "2026-05-03", "2026-05-04", "2026-05-05",
			"2026-06-19", "2026-06-20", "2026-06-21",
			"2026-09-25", "2026-09-26", "2026-09-27",
			"2026-10-01", "2026-10-02", "2026-10-03", "2026-10-04", "2026-10-05", "2026-10-06", "2026-10-07",
		},
		WorkdayDates: []string{
			"2026-01-04",
			"2026-02-14", "2026-02-28",
			"2026-05-09",
			"2026-09-20",
			"2026-10-10",
		},
	}
}

func BuildFundDailyWorkdayGuard(now time.Time, enabled bool, calendar FundDailyWorkdayCalendar) FundDailyWorkdayGuard {
	local := now
	if local.IsZero() {
		local = time.Now()
	}
	local = local.In(time.Local)
	date := local.Format("2006-01-02")
	isWorkday, override := isFundDailyWorkday(date, local.Weekday(), calendar)
	guard := FundDailyWorkdayGuard{
		Enabled:          enabled,
		Date:             date,
		Weekday:          fundDailyWeekdayName(local.Weekday()),
		IsWorkday:        isWorkday,
		AllowBuy:         !enabled || isWorkday,
		CalendarOverride: override,
	}
	if enabled && !isWorkday {
		guard.Reason = "今天是非工作日，基金操作建议不安排新增买入，只保留持有和观察。"
	}
	return guard
}

func (g FundDailyWorkdayGuard) BlocksBuy() bool {
	return g.Enabled && !g.AllowBuy
}

func applyFundDailyWorkdayGuard(portfolioActions []FundDailyAction, candidateActions []FundDailyAction, guard FundDailyWorkdayGuard) ([]FundDailyAction, []FundDailyAction) {
	if !guard.BlocksBuy() {
		return portfolioActions, candidateActions
	}
	for idx := range portfolioActions {
		blockFundDailyBuyAction(&portfolioActions[idx], "非工作日暂不加仓", "hold", guard.Reason)
	}
	for idx := range candidateActions {
		blockFundDailyBuyAction(&candidateActions[idx], "非工作日观察", "watch", guard.Reason)
	}
	return portfolioActions, candidateActions
}

func blockFundDailyBuyAction(action *FundDailyAction, label string, level string, reason string) {
	if action.SuggestedAmount <= 0 && action.ActionLevel != "buy" {
		return
	}
	action.Action = label
	action.ActionLevel = level
	action.SuggestedAmount = 0
	action.SuggestedWeight = 0
	action.Reasons = prependUniqueDailyReason(action.Reasons, reason)
}

func isFundDailyWorkday(date string, weekday time.Weekday, calendar FundDailyWorkdayCalendar) (bool, string) {
	if containsFundDailyCalendarDate(calendar.WorkdayDates, date) {
		return true, "configured_workday"
	}
	if containsFundDailyCalendarDate(calendar.NonWorkdayDates, date) {
		return false, "configured_non_workday"
	}
	return isFundDailyWeekdayWorkday(weekday), ""
}

func containsFundDailyCalendarDate(dates []string, date string) bool {
	for _, item := range dates {
		if strings.TrimSpace(item) == date {
			return true
		}
	}
	return false
}

func normalizeFundDailyCalendarDates(dates []string) []string {
	if len(dates) == 0 {
		return nil
	}
	seen := map[string]bool{}
	normalized := make([]string, 0, len(dates))
	for _, item := range dates {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		normalized = append(normalized, item)
	}
	sort.Strings(normalized)
	return normalized
}

func isFundDailyWeekdayWorkday(weekday time.Weekday) bool {
	return weekday >= time.Monday && weekday <= time.Friday
}

func fundDailyWeekdayName(weekday time.Weekday) string {
	switch weekday {
	case time.Monday:
		return "星期一"
	case time.Tuesday:
		return "星期二"
	case time.Wednesday:
		return "星期三"
	case time.Thursday:
		return "星期四"
	case time.Friday:
		return "星期五"
	case time.Saturday:
		return "星期六"
	case time.Sunday:
		return "星期日"
	default:
		return weekday.String()
	}
}
