package core

import "fmt"

type fundDailyUpsideRoom string

const (
	fundDailyUpsideRoomStrong       fundDailyUpsideRoom = "strong"
	fundDailyUpsideRoomConstructive fundDailyUpsideRoom = "constructive"
	fundDailyUpsideRoomStretched    fundDailyUpsideRoom = "stretched"
	fundDailyUpsideRoomLimited      fundDailyUpsideRoom = "limited"
	fundDailyUpsideRoomExhausted    fundDailyUpsideRoom = "exhausted"
)

type fundDailyUpsideAssessment struct {
	room               fundDailyUpsideRoom
	qualityScore       float64
	stretchScore       float64
	riskScore          float64
	allowAutomaticTrim bool
	trimRatio          float64
	reason             string
	riskNote           string
}

func assessFundDailyUpsideRoom(fund FundDailyAIFund, signals fundDailyLocalSignals) fundDailyUpsideAssessment {
	quality := fundDailyUpsideQualityScore(fund)
	stretch := fundDailyUpsideStretchScore(fund)
	risk := fundDailyUpsideRiskScore(fund, signals)
	net := quality - stretch*0.55 - risk*0.65

	room := fundDailyUpsideRoomStretched
	switch {
	case risk >= 48 && stretch >= 58:
		room = fundDailyUpsideRoomExhausted
	case risk >= 30 && stretch >= 40:
		room = fundDailyUpsideRoomLimited
	case net >= 30 && risk <= 18 && stretch <= 22:
		room = fundDailyUpsideRoomStrong
	case net >= 14 && risk <= 28:
		room = fundDailyUpsideRoomConstructive
	case net < -12 || stretch >= 52:
		room = fundDailyUpsideRoomLimited
	}

	ratio := fundDailyUpsideTrimRatio(room, risk, stretch)
	allowTrim := ratio > 0
	return fundDailyUpsideAssessment{
		room:               room,
		qualityScore:       quality,
		stretchScore:       stretch,
		riskScore:          risk,
		allowAutomaticTrim: allowTrim,
		trimRatio:          ratio,
		reason:             fundDailyUpsideReason(room, quality, stretch, risk),
		riskNote:           fundDailyUpsideRiskNote(room, ratio),
	}
}

func fundDailyUpsideQualityScore(fund FundDailyAIFund) float64 {
	score := 0.0
	switch {
	case fund.Score >= 95:
		score += 18
	case fund.Score >= 88:
		score += 12
	case fund.Score >= 80:
		score += 8
	case fund.Score > 0:
		score += 3
	}
	switch {
	case fund.StrategyScore >= 95:
		score += 14
	case fund.StrategyScore >= 88:
		score += 10
	case fund.StrategyScore >= 80:
		score += 6
	case fund.StrategyScore > 0:
		score += 2
	}
	switch {
	case fund.Sharp >= 1.5:
		score += 8
	case fund.Sharp >= 1.0:
		score += 5
	case fund.Sharp >= 0.6:
		score += 2
	}
	if fund.ExpectedAnnualReturn != nil {
		switch {
		case *fund.ExpectedAnnualReturn >= 25:
			score += 8
		case *fund.ExpectedAnnualReturn >= 15:
			score += 5
		case *fund.ExpectedAnnualReturn >= 8:
			score += 2
		}
	}
	rankScore := fundDailyUpsideRankScore(fund.RankRatios)
	score += rankScore
	return score
}

func fundDailyUpsideRankScore(ranks FundDailyAIRankRatios) float64 {
	values := []float64{ranks.Month1, ranks.Month3, ranks.Month6, ranks.ThisYear, ranks.Year1}
	sum := 0.0
	count := 0
	for _, value := range values {
		if value <= 0 {
			continue
		}
		sum += value
		count++
	}
	if count == 0 {
		return 0
	}
	avg := sum / float64(count)
	switch {
	case avg <= 15:
		return 8
	case avg <= 25:
		return 6
	case avg <= 40:
		return 3
	default:
		return 0
	}
}

func fundDailyUpsideStretchScore(fund FundDailyAIFund) float64 {
	score := 0.0
	switch {
	case fund.RecentReturns.Month1 >= 30:
		score += 18
	case fund.RecentReturns.Month1 >= 20:
		score += 10
	case fund.RecentReturns.Month1 >= 12:
		score += 6
	}
	switch {
	case fund.RecentReturns.Month3 >= 70:
		score += 24
	case fund.RecentReturns.Month3 >= 45:
		score += 16
	case fund.RecentReturns.Month3 >= 25:
		score += 8
	}
	switch {
	case fund.RecentReturns.Month6 >= 90:
		score += 20
	case fund.RecentReturns.Month6 >= 60:
		score += 14
	case fund.RecentReturns.Month6 >= 45:
		score += 8
	}
	switch {
	case fund.ProfitRatio >= 35:
		score += 12
	case fund.ProfitRatio >= 25:
		score += 8
	case fund.ProfitRatio >= 15:
		score += 5
	}
	if fund.ProfitAmount >= fundDailyLocalMinProfitTakingAmount {
		score += 4
	}
	return score
}

func fundDailyUpsideRiskScore(fund FundDailyAIFund, signals fundDailyLocalSignals) float64 {
	score := 0.0
	switch {
	case fund.Stddev >= 40:
		score += 22
	case fund.Stddev >= 30:
		score += 14
	case fund.Stddev >= 25:
		score += 8
	}
	switch {
	case fund.Drawdown >= 40:
		score += 18
	case fund.Drawdown >= 30:
		score += 10
	case fund.Drawdown >= 25:
		score += 6
	}
	if signals.TechConcentrated && isFundDailyTechExposure(fund) {
		score += 10
	}
	if hasFundDailyLocalPortfolioTopOverlap(fund, signals) {
		score += 8
	}
	switch {
	case fund.CurrentWeight >= 28:
		score += 8
	case fund.CurrentWeight >= 18:
		score += 4
	}
	return score
}

func fundDailyUpsideTrimRatio(room fundDailyUpsideRoom, risk float64, stretch float64) float64 {
	switch room {
	case fundDailyUpsideRoomStrong, fundDailyUpsideRoomConstructive:
		return 0
	case fundDailyUpsideRoomStretched:
		return 0.10
	case fundDailyUpsideRoomLimited:
		if risk >= 42 && stretch >= 55 {
			return 0.20
		}
		return 0.15
	case fundDailyUpsideRoomExhausted:
		return 0.20
	default:
		return 0.10
	}
}

func fundDailyUpsideReason(room fundDailyUpsideRoom, quality float64, stretch float64, risk float64) string {
	return fmt.Sprintf("先评估上涨空间：%s，质量支撑 %.1f，涨幅透支 %.1f，风险压力 %.1f；按上涨空间和风险强度分层止盈。", fundDailyUpsideRoomLabel(room), quality, stretch, risk)
}

func fundDailyUpsideRiskNote(room fundDailyUpsideRoom, ratio float64) string {
	if ratio <= 0 {
		return fmt.Sprintf("上涨空间评估为%s，自动止盈暂缓。", fundDailyUpsideRoomLabel(room))
	}
	return fmt.Sprintf("上涨空间评估为%s，本次止盈比例约 %.0f%%。", fundDailyUpsideRoomLabel(room), ratio*100)
}

func fundDailyUpsideRoomLabel(room fundDailyUpsideRoom) string {
	switch room {
	case fundDailyUpsideRoomStrong:
		return "较充分"
	case fundDailyUpsideRoomConstructive:
		return "尚可"
	case fundDailyUpsideRoomStretched:
		return "偏紧"
	case fundDailyUpsideRoomLimited:
		return "受限"
	case fundDailyUpsideRoomExhausted:
		return "明显不足"
	default:
		return "待确认"
	}
}
