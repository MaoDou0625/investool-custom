package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

const maxFundPortfolioHistorySnapshots = 240

var fundPortfolioHistoryStoreMu sync.Mutex

type FundPortfolioHistory struct {
	Snapshots []FundPortfolioSnapshot `json:"snapshots"`
}

type FundPortfolioSnapshot struct {
	Date               string                      `json:"date"`
	Timestamp          string                      `json:"timestamp"`
	TotalCurrentAmount float64                     `json:"total_current_amount"`
	CostAmount         float64                     `json:"cost_amount"`
	ProfitAmount       float64                     `json:"profit_amount"`
	ProfitRatio        float64                     `json:"profit_ratio"`
	Items              []FundPortfolioSnapshotItem `json:"items"`
}

type FundPortfolioSnapshotItem struct {
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	UnitNav       float64 `json:"unit_nav"`
	CurrentAmount float64 `json:"current_amount"`
	CurrentWeight float64 `json:"current_weight"`
	ProfitAmount  float64 `json:"profit_amount"`
	ProfitRatio   float64 `json:"profit_ratio"`
}

type FundPortfolioHistoryStore struct {
	filename string
}

func NewFundPortfolioHistoryStore(filename string) *FundPortfolioHistoryStore {
	return &FundPortfolioHistoryStore{filename: filename}
}

func (s *FundPortfolioHistoryStore) Load() (FundPortfolioHistory, error) {
	fundPortfolioHistoryStoreMu.Lock()
	defer fundPortfolioHistoryStoreMu.Unlock()
	return s.loadUnlocked()
}

func (s *FundPortfolioHistoryStore) UpsertSnapshot(snapshot FundPortfolioSnapshot) error {
	fundPortfolioHistoryStoreMu.Lock()
	defer fundPortfolioHistoryStoreMu.Unlock()

	if snapshot.Date == "" {
		return fmt.Errorf("fund portfolio snapshot date is empty")
	}
	history, err := s.loadUnlocked()
	if err != nil {
		return err
	}

	replaced := false
	for idx := range history.Snapshots {
		if history.Snapshots[idx].Date == snapshot.Date {
			history.Snapshots[idx] = snapshot
			replaced = true
			break
		}
	}
	if !replaced {
		history.Snapshots = append(history.Snapshots, snapshot)
	}
	history.sortAndLimit()
	return s.saveUnlocked(history)
}

func (s *FundPortfolioHistoryStore) loadUnlocked() (FundPortfolioHistory, error) {
	history := FundPortfolioHistory{}
	if s.filename == "" {
		return history, fmt.Errorf("fund portfolio history filename is empty")
	}

	content, err := os.ReadFile(s.filename)
	if os.IsNotExist(err) {
		return history, nil
	}
	if err != nil {
		return history, err
	}

	content = bytes.TrimPrefix(content, []byte{0xEF, 0xBB, 0xBF})
	if len(bytes.TrimSpace(content)) == 0 {
		return history, nil
	}
	if err := json.Unmarshal(content, &history); err != nil {
		return history, err
	}
	history.sortAndLimit()
	return history, nil
}

func (s *FundPortfolioHistoryStore) saveUnlocked(history FundPortfolioHistory) error {
	if s.filename == "" {
		return fmt.Errorf("fund portfolio history filename is empty")
	}

	dir := filepath.Dir(s.filename)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	tmpFilename := s.filename + ".tmp.json"
	f, err := os.Create(tmpFilename)
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(history); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpFilename)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpFilename)
		return err
	}
	return os.Rename(tmpFilename, s.filename)
}

func (h *FundPortfolioHistory) sortAndLimit() {
	sort.SliceStable(h.Snapshots, func(i, j int) bool {
		return h.Snapshots[i].Date < h.Snapshots[j].Date
	})
	if len(h.Snapshots) > maxFundPortfolioHistorySnapshots {
		h.Snapshots = h.Snapshots[len(h.Snapshots)-maxFundPortfolioHistorySnapshots:]
	}
}
