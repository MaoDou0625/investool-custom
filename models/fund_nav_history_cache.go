package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const maxFundNAVHistoryCachePoints = 366

var fundNAVHistoryCacheStoreMu sync.Mutex

type FundNAVHistoryCache struct {
	Items map[string]FundNAVHistoryCacheItem `json:"items"`
}

type FundNAVHistoryCacheItem struct {
	Code      string                     `json:"code"`
	Name      string                     `json:"name,omitempty"`
	UpdatedAt string                     `json:"updated_at"`
	Points    []FundNAVHistoryCachePoint `json:"points"`
}

type FundNAVHistoryCachePoint struct {
	Date    string  `json:"date"`
	UnitNAV float64 `json:"unit_nav"`
}

type FundNAVHistoryCacheStore struct {
	filename string
}

func NewFundNAVHistoryCacheStore(filename string) *FundNAVHistoryCacheStore {
	return &FundNAVHistoryCacheStore{filename: filename}
}

func (s *FundNAVHistoryCacheStore) Load() (FundNAVHistoryCache, error) {
	fundNAVHistoryCacheStoreMu.Lock()
	defer fundNAVHistoryCacheStoreMu.Unlock()
	return s.loadUnlocked()
}

func (s *FundNAVHistoryCacheStore) Save(cache FundNAVHistoryCache) error {
	fundNAVHistoryCacheStoreMu.Lock()
	defer fundNAVHistoryCacheStoreMu.Unlock()

	cache.normalize()
	return s.saveUnlocked(cache)
}

func (s *FundNAVHistoryCacheStore) loadUnlocked() (FundNAVHistoryCache, error) {
	cache := FundNAVHistoryCache{Items: map[string]FundNAVHistoryCacheItem{}}
	if s.filename == "" {
		return cache, fmt.Errorf("fund nav history cache filename is empty")
	}

	content, err := os.ReadFile(s.filename)
	if os.IsNotExist(err) {
		return cache, nil
	}
	if err != nil {
		return cache, err
	}

	content = bytes.TrimPrefix(content, []byte{0xEF, 0xBB, 0xBF})
	if len(bytes.TrimSpace(content)) == 0 {
		return cache, nil
	}
	if err := json.Unmarshal(content, &cache); err != nil {
		return cache, err
	}
	cache.normalize()
	return cache, nil
}

func (s *FundNAVHistoryCacheStore) saveUnlocked(cache FundNAVHistoryCache) error {
	if s.filename == "" {
		return fmt.Errorf("fund nav history cache filename is empty")
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
	if err := encoder.Encode(cache); err != nil {
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

func (c *FundNAVHistoryCache) normalize() {
	if c.Items == nil {
		c.Items = map[string]FundNAVHistoryCacheItem{}
	}

	normalized := map[string]FundNAVHistoryCacheItem{}
	for code, item := range c.Items {
		code = strings.TrimSpace(code)
		if code == "" {
			code = strings.TrimSpace(item.Code)
		}
		if code == "" {
			continue
		}
		item.Code = code
		item.Name = strings.TrimSpace(item.Name)
		item.sortAndLimit()
		normalized[code] = item
	}
	c.Items = normalized
}

func (i *FundNAVHistoryCacheItem) sortAndLimit() {
	points := make([]FundNAVHistoryCachePoint, 0, len(i.Points))
	for _, point := range i.Points {
		if strings.TrimSpace(point.Date) == "" || point.UnitNAV <= 0 {
			continue
		}
		points = append(points, FundNAVHistoryCachePoint{
			Date:    strings.TrimSpace(point.Date),
			UnitNAV: point.UnitNAV,
		})
	}
	sort.SliceStable(points, func(a, b int) bool {
		return points[a].Date < points[b].Date
	})
	if len(points) > maxFundNAVHistoryCachePoints {
		points = points[len(points)-maxFundNAVHistoryCachePoints:]
	}
	i.Points = points
}
