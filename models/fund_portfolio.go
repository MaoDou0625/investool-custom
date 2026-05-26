package models

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	FundPortfolioStatusOwned = "owned"
	FundPortfolioStatusWatch = "watch"
)

var (
	fundPortfolioCodePattern = regexp.MustCompile(`^\d{6}$`)
	fundPortfolioStoreMu     sync.Mutex
)

type FundPortfolio struct {
	Items []FundPortfolioItem `json:"items"`
}

type FundPortfolioItem struct {
	Code          string  `json:"code" form:"code"`
	Status        string  `json:"status" form:"status"`
	CostNav       float64 `json:"cost_nav" form:"cost_nav"`
	HoldingShares float64 `json:"holding_shares" form:"holding_shares"`
	// HoldingAmount is the current holding market value shown by the user's broker/app.
	HoldingAmount float64 `json:"holding_amount" form:"holding_amount"`
	TargetWeight  float64 `json:"target_weight" form:"target_weight"`
	Note          string  `json:"note" form:"note"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
}

func (i FundPortfolioItem) StatusName() string {
	switch i.Status {
	case FundPortfolioStatusOwned:
		return "已持有"
	case FundPortfolioStatusWatch:
		return "意向观察"
	default:
		return "未分类"
	}
}

func (i FundPortfolioItem) IsOwned() bool {
	return i.Status == FundPortfolioStatusOwned
}

func (i FundPortfolioItem) IsWatch() bool {
	return i.Status == FundPortfolioStatusWatch
}

func (i *FundPortfolioItem) Normalize(now time.Time) error {
	i.Code = strings.TrimSpace(i.Code)
	if !fundPortfolioCodePattern.MatchString(i.Code) {
		return fmt.Errorf("invalid fund code: %s", i.Code)
	}

	switch i.Status {
	case FundPortfolioStatusOwned, FundPortfolioStatusWatch:
	default:
		i.Status = FundPortfolioStatusWatch
	}

	i.Note = strings.TrimSpace(i.Note)
	if i.CostNav < 0 {
		i.CostNav = 0
	}
	if i.HoldingShares < 0 {
		i.HoldingShares = 0
	}
	if i.HoldingAmount < 0 {
		i.HoldingAmount = 0
	}
	if i.TargetWeight < 0 {
		i.TargetWeight = 0
	}

	ts := now.Format("2006-01-02 15:04:05")
	if i.CreatedAt == "" {
		i.CreatedAt = ts
	}
	i.UpdatedAt = ts
	return nil
}

func (p FundPortfolio) Codes() []string {
	codes := make([]string, 0, len(p.Items))
	for _, item := range p.Items {
		if item.Code != "" {
			codes = append(codes, item.Code)
		}
	}
	return codes
}

func (p FundPortfolio) OwnedItems() []FundPortfolioItem {
	return filterFundPortfolioItems(p.Items, FundPortfolioStatusOwned)
}

func (p FundPortfolio) WatchItems() []FundPortfolioItem {
	return filterFundPortfolioItems(p.Items, FundPortfolioStatusWatch)
}

type FundPortfolioStore struct {
	filename string
}

func NewFundPortfolioStore(filename string) *FundPortfolioStore {
	return &FundPortfolioStore{filename: filename}
}

func (s *FundPortfolioStore) Load() (FundPortfolio, error) {
	fundPortfolioStoreMu.Lock()
	defer fundPortfolioStoreMu.Unlock()
	return s.loadUnlocked()
}

func (s *FundPortfolioStore) Upsert(item FundPortfolioItem) error {
	fundPortfolioStoreMu.Lock()
	defer fundPortfolioStoreMu.Unlock()

	portfolio, err := s.loadUnlocked()
	if err != nil {
		return err
	}
	if err := item.Normalize(time.Now()); err != nil {
		return err
	}

	replaced := false
	for idx := range portfolio.Items {
		if portfolio.Items[idx].Code == item.Code {
			item.CreatedAt = portfolio.Items[idx].CreatedAt
			portfolio.Items[idx] = item
			replaced = true
			break
		}
	}
	if !replaced {
		portfolio.Items = append(portfolio.Items, item)
	}
	portfolio.sort()
	return s.saveUnlocked(portfolio)
}

func (s *FundPortfolioStore) Delete(code string) error {
	fundPortfolioStoreMu.Lock()
	defer fundPortfolioStoreMu.Unlock()

	portfolio, err := s.loadUnlocked()
	if err != nil {
		return err
	}

	code = strings.TrimSpace(code)
	items := make([]FundPortfolioItem, 0, len(portfolio.Items))
	for _, item := range portfolio.Items {
		if item.Code != code {
			items = append(items, item)
		}
	}
	portfolio.Items = items
	portfolio.sort()
	return s.saveUnlocked(portfolio)
}

func (s *FundPortfolioStore) loadUnlocked() (FundPortfolio, error) {
	portfolio := FundPortfolio{}
	if s.filename == "" {
		return portfolio, fmt.Errorf("fund portfolio filename is empty")
	}

	f, err := os.Open(s.filename)
	if os.IsNotExist(err) {
		return portfolio, nil
	}
	if err != nil {
		return portfolio, err
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&portfolio); err != nil {
		return portfolio, err
	}
	portfolio.sort()
	return portfolio, nil
}

func (s *FundPortfolioStore) saveUnlocked(portfolio FundPortfolio) error {
	if s.filename == "" {
		return fmt.Errorf("fund portfolio filename is empty")
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
	if err := encoder.Encode(portfolio); err != nil {
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

func (p *FundPortfolio) sort() {
	sort.SliceStable(p.Items, func(i, j int) bool {
		if p.Items[i].Status != p.Items[j].Status {
			return p.Items[i].Status < p.Items[j].Status
		}
		return p.Items[i].Code < p.Items[j].Code
	})
}

func filterFundPortfolioItems(items []FundPortfolioItem, status string) []FundPortfolioItem {
	result := []FundPortfolioItem{}
	for _, item := range items {
		if item.Status == status {
			result = append(result, item)
		}
	}
	return result
}
