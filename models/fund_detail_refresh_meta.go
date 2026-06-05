package models

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"strings"
	"time"
)

const FundDetailRefreshMetaTimeLayout = "2006-01-02 15:04:05"

type FundDetailRefreshMeta struct {
	Items map[string]FundDetailRefreshMetaItem `json:"items"`
}

type FundDetailRefreshMetaItem struct {
	Code      string `json:"code"`
	UpdatedAt string `json:"updated_at"`
	Priority  string `json:"priority,omitempty"`
}

type FundDetailRefreshMetaStore struct {
	filename string
}

func NewFundDetailRefreshMetaStore(filename string) *FundDetailRefreshMetaStore {
	return &FundDetailRefreshMetaStore{filename: filename}
}

func NewFundDetailRefreshMeta() FundDetailRefreshMeta {
	return FundDetailRefreshMeta{Items: map[string]FundDetailRefreshMetaItem{}}
}

func (s *FundDetailRefreshMetaStore) Load() (FundDetailRefreshMeta, error) {
	content, err := ioutil.ReadFile(s.filename)
	if err != nil {
		if os.IsNotExist(err) {
			return NewFundDetailRefreshMeta(), nil
		}
		return NewFundDetailRefreshMeta(), err
	}

	meta := NewFundDetailRefreshMeta()
	if err := json.Unmarshal(content, &meta); err != nil {
		return NewFundDetailRefreshMeta(), err
	}
	meta.Normalize()
	return meta, nil
}

func (s *FundDetailRefreshMetaStore) Save(meta FundDetailRefreshMeta) error {
	meta.Normalize()
	content, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(s.filename, content, 0666)
}

func (m *FundDetailRefreshMeta) Normalize() {
	if m.Items == nil {
		m.Items = map[string]FundDetailRefreshMetaItem{}
	}
	for code, item := range m.Items {
		item.Code = strings.TrimSpace(item.Code)
		if item.Code == "" {
			item.Code = code
		}
		m.Items[code] = item
	}
}

func (m FundDetailRefreshMeta) LastUpdatedAt(code string) time.Time {
	item, ok := m.Items[code]
	if !ok || item.UpdatedAt == "" {
		return time.Time{}
	}
	updatedAt, err := time.ParseInLocation(FundDetailRefreshMetaTimeLayout, item.UpdatedAt, time.Local)
	if err != nil {
		return time.Time{}
	}
	return updatedAt
}

func (m FundDetailRefreshMeta) IsStale(code string, now time.Time, staleAfter time.Duration) bool {
	if staleAfter <= 0 {
		return true
	}
	updatedAt := m.LastUpdatedAt(code)
	if updatedAt.IsZero() {
		return true
	}
	return !updatedAt.After(now.Add(-staleAfter))
}

func (m *FundDetailRefreshMeta) Touch(code string, updatedAt time.Time, priority string) {
	m.Normalize()
	if code == "" {
		return
	}
	if updatedAt.IsZero() {
		updatedAt = time.Now()
	}
	m.Items[code] = FundDetailRefreshMetaItem{
		Code:      code,
		UpdatedAt: updatedAt.Format(FundDetailRefreshMetaTimeLayout),
		Priority:  priority,
	}
}
