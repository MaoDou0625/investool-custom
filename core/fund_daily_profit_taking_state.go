package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	fundDailyProfitTakingStateVersion      = "fund_daily_profit_taking_state.v1"
	fundDailyProfitTakingEpisodeCooldown   = 5 * 24 * time.Hour
	fundDailyProfitTakingBaselineGrowRatio = 1.15
)

var fundDailyProfitTakingStateStoreMu sync.Mutex

type FundDailyProfitTakingState struct {
	Version   string                                 `json:"version"`
	UpdatedAt string                                 `json:"updated_at,omitempty"`
	Items     map[string]FundDailyProfitTakingRecord `json:"items,omitempty"`
}

type FundDailyProfitTakingRecord struct {
	Code             string  `json:"code"`
	Name             string  `json:"name,omitempty"`
	UpsideRoom       string  `json:"upside_room"`
	BaselineDate     string  `json:"baseline_date"`
	BaselineAmount   float64 `json:"baseline_amount"`
	BaselineShares   float64 `json:"baseline_shares,omitempty"`
	TargetRatio      float64 `json:"target_ratio"`
	TargetAmount     float64 `json:"target_amount"`
	TargetShares     float64 `json:"target_shares,omitempty"`
	AdvisedAmount    float64 `json:"advised_amount"`
	AdvisedShares    float64 `json:"advised_shares,omitempty"`
	LastAdviceDate   string  `json:"last_advice_date,omitempty"`
	LastAdviceAmount float64 `json:"last_advice_amount,omitempty"`
	LastAdviceShares float64 `json:"last_advice_shares,omitempty"`
	UpdatedAt        string  `json:"updated_at,omitempty"`
}

type FundDailyProfitTakingStateStore struct {
	filename string
}

func NewFundDailyProfitTakingStateStore(filename string) *FundDailyProfitTakingStateStore {
	return &FundDailyProfitTakingStateStore{filename: filename}
}

func (s *FundDailyProfitTakingStateStore) Load() (FundDailyProfitTakingState, error) {
	fundDailyProfitTakingStateStoreMu.Lock()
	defer fundDailyProfitTakingStateStoreMu.Unlock()

	if s.filename == "" {
		return emptyFundDailyProfitTakingState(), errors.New("empty fund daily profit taking state filename")
	}
	content, err := os.ReadFile(s.filename)
	if errors.Is(err, os.ErrNotExist) {
		return emptyFundDailyProfitTakingState(), nil
	}
	if err != nil {
		return emptyFundDailyProfitTakingState(), err
	}
	content = bytes.TrimPrefix(content, []byte{0xEF, 0xBB, 0xBF})
	if len(bytes.TrimSpace(content)) == 0 {
		return emptyFundDailyProfitTakingState(), nil
	}
	state := FundDailyProfitTakingState{}
	if err := json.Unmarshal(content, &state); err != nil {
		return emptyFundDailyProfitTakingState(), err
	}
	return normalizeFundDailyProfitTakingState(state), nil
}

func (s *FundDailyProfitTakingStateStore) Save(state FundDailyProfitTakingState) error {
	fundDailyProfitTakingStateStoreMu.Lock()
	defer fundDailyProfitTakingStateStoreMu.Unlock()

	if s.filename == "" {
		return errors.New("empty fund daily profit taking state filename")
	}
	state = normalizeFundDailyProfitTakingState(state)
	content, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if dir := filepath.Dir(s.filename); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	tmpFilename := s.filename + ".tmp.json"
	if err := os.WriteFile(tmpFilename, content, 0644); err != nil {
		return err
	}
	return os.Rename(tmpFilename, s.filename)
}

func emptyFundDailyProfitTakingState() FundDailyProfitTakingState {
	return FundDailyProfitTakingState{
		Version: fundDailyProfitTakingStateVersion,
		Items:   map[string]FundDailyProfitTakingRecord{},
	}
}

func normalizeFundDailyProfitTakingState(state FundDailyProfitTakingState) FundDailyProfitTakingState {
	if state.Version == "" {
		state.Version = fundDailyProfitTakingStateVersion
	}
	if state.Items == nil {
		state.Items = map[string]FundDailyProfitTakingRecord{}
	}
	return state
}

func applyFundDailyProfitTakingState(
	option fundDailyLocalProfitTakingOption,
	state FundDailyProfitTakingState,
	generatedAt time.Time,
) (fundDailyLocalProfitTakingOption, FundDailyProfitTakingState, bool, string) {
	state = normalizeFundDailyProfitTakingState(state)
	if option.action == "sell" {
		return option, state, true, ""
	}
	targetRatio := fundDailyProfitTakingTargetRatio(option)
	if option.fund.Code == "" || option.amount <= 0 || targetRatio <= 0 {
		return option, state, false, ""
	}

	date := generatedAt.Format("2006-01-02")
	updatedAt := generatedAt.Format("2006-01-02 15:04:05")
	record := state.Items[option.fund.Code]
	record = fundDailyProfitTakingRecordForOption(option, record, targetRatio, date, updatedAt)

	if record.LastAdviceDate == date && record.LastAdviceAmount >= fundDailyLocalMinProfitTakingAmount {
		option.amount = minPositive(record.LastAdviceAmount, option.amount, fundDailyLocalMaxProfitTakingAmount, option.fund.CurrentAmount*0.30)
		option.amount = floorDailyAmount(option.amount)
		if option.amount < fundDailyLocalMinProfitTakingAmount {
			state.Items[option.fund.Code] = record
			return option, state, false, ""
		}
		note := fundDailyProfitTakingStateNote(record, option.amount)
		state.Items[option.fund.Code] = record
		return option, state, true, note
	}

	remaining := record.TargetAmount - record.AdvisedAmount
	if remaining <= 0 {
		state.Items[option.fund.Code] = record
		return option, state, false, ""
	}
	amount := minPositive(option.amount, remaining, fundDailyLocalMaxProfitTakingAmount, option.fund.CurrentAmount*0.30)
	amount = floorDailyAmount(amount)
	if amount < fundDailyLocalMinProfitTakingAmount {
		state.Items[option.fund.Code] = record
		return option, state, false, ""
	}

	shareAmount := fundDailyRedeemShareAmount(amount, option.fund)
	record.AdvisedAmount += amount
	record.AdvisedShares += shareAmount
	record.LastAdviceDate = date
	record.LastAdviceAmount = amount
	record.LastAdviceShares = shareAmount
	record.UpdatedAt = updatedAt
	state.UpdatedAt = updatedAt
	state.Items[option.fund.Code] = record

	option.amount = amount
	note := fundDailyProfitTakingStateNote(record, amount)
	return option, state, true, note
}

func fundDailyProfitTakingRecordForOption(option fundDailyLocalProfitTakingOption, record FundDailyProfitTakingRecord, targetRatio float64, date string, updatedAt string) FundDailyProfitTakingRecord {
	if shouldStartFundDailyProfitTakingEpisode(option, record, date) {
		record = FundDailyProfitTakingRecord{
			Code:           option.fund.Code,
			Name:           option.fund.Name,
			UpsideRoom:     string(option.upside.room),
			BaselineDate:   date,
			BaselineAmount: option.fund.CurrentAmount,
			BaselineShares: option.fund.HoldingShares,
			TargetRatio:    targetRatio,
			UpdatedAt:      updatedAt,
		}
	} else {
		record.Code = option.fund.Code
		record.Name = option.fund.Name
		if fundDailyProfitTakingRoomSeverity(option.upside.room) > fundDailyProfitTakingRoomSeverityString(record.UpsideRoom) {
			record.UpsideRoom = string(option.upside.room)
			record.TargetRatio = targetRatio
		}
		if record.TargetRatio <= 0 {
			record.TargetRatio = targetRatio
		}
		if targetRatio > record.TargetRatio {
			record.TargetRatio = targetRatio
		}
		record.UpdatedAt = updatedAt
	}

	record.TargetAmount = record.BaselineAmount * record.TargetRatio
	record.TargetShares = record.BaselineShares * record.TargetRatio
	return record
}

func fundDailyProfitTakingTargetRatio(option fundDailyLocalProfitTakingOption) float64 {
	if option.isExplicit && option.fund.CurrentAmount > 0 && option.amount > 0 {
		return option.amount / option.fund.CurrentAmount
	}
	return option.upside.trimRatio
}

func shouldStartFundDailyProfitTakingEpisode(option fundDailyLocalProfitTakingOption, record FundDailyProfitTakingRecord, date string) bool {
	if record.Code == "" || record.BaselineDate == "" || record.BaselineAmount <= 0 {
		return true
	}
	if option.fund.CurrentAmount > record.BaselineAmount*fundDailyProfitTakingBaselineGrowRatio {
		return true
	}
	if option.fund.HoldingShares > 0 && record.BaselineShares > 0 && option.fund.HoldingShares > record.BaselineShares*fundDailyProfitTakingBaselineGrowRatio {
		return true
	}
	episodeDate, err := time.Parse("2006-01-02", record.BaselineDate)
	if err != nil {
		return true
	}
	currentDate, err := time.Parse("2006-01-02", date)
	if err != nil {
		return false
	}
	return currentDate.Sub(episodeDate) > fundDailyProfitTakingEpisodeCooldown
}

func fundDailyProfitTakingStateNote(record FundDailyProfitTakingRecord, amount float64) string {
	if record.BaselineShares > 0 {
		return fmt.Sprintf("本轮止盈比例以 %s 的基准持仓 %.2f 元 / %.2f 份计算，累计目标 %.2f 元 / %.2f 份；本轮已建议 %.2f 元，本次 %.2f 元。", record.BaselineDate, record.BaselineAmount, record.BaselineShares, record.TargetAmount, record.TargetShares, record.AdvisedAmount, amount)
	}
	return fmt.Sprintf("本轮止盈比例以 %s 的基准持仓 %.2f 元计算，累计目标 %.2f 元；本轮已建议 %.2f 元，本次 %.2f 元。", record.BaselineDate, record.BaselineAmount, record.TargetAmount, record.AdvisedAmount, amount)
}

func fundDailyProfitTakingGeneratedAt(value string) time.Time {
	if parsed, err := time.ParseInLocation("2006-01-02 15:04:05", value, time.Local); err == nil {
		return parsed
	}
	if parsed, err := time.ParseInLocation("2006-01-02", value, time.Local); err == nil {
		return parsed
	}
	return time.Now()
}

func fundDailyProfitTakingRoomSeverity(room fundDailyUpsideRoom) int {
	switch room {
	case fundDailyUpsideRoomStretched:
		return 1
	case fundDailyUpsideRoomLimited:
		return 2
	case fundDailyUpsideRoomExhausted:
		return 3
	default:
		return 0
	}
}

func fundDailyProfitTakingRoomSeverityString(room string) int {
	return fundDailyProfitTakingRoomSeverity(fundDailyUpsideRoom(room))
}
