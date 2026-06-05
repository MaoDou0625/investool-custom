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
	"time"
)

var institution13FCacheMu sync.Mutex

type Institution13FCache struct {
	UpdatedAt time.Time                         `json:"updated_at"`
	Reports   map[string][]Institution13FReport `json:"reports"`
}

type Institution13FReport struct {
	InstitutionID       string                  `json:"institution_id"`
	InstitutionName     string                  `json:"institution_name"`
	CIK                 string                  `json:"cik"`
	Form                string                  `json:"form"`
	AccessionNumber     string                  `json:"accession_number"`
	FilingDate          string                  `json:"filing_date"`
	ReportDate          string                  `json:"report_date"`
	SourceURL           string                  `json:"source_url"`
	InformationTableURL string                  `json:"information_table_url"`
	TotalValueUSD       float64                 `json:"total_value_usd"`
	Holdings            []Institution13FHolding `json:"holdings"`
}

type Institution13FHolding struct {
	IssuerName     string  `json:"issuer_name"`
	TitleOfClass   string  `json:"title_of_class"`
	CUSIP          string  `json:"cusip"`
	Ticker         string  `json:"ticker"`
	Sector         string  `json:"sector"`
	ValueUSD       float64 `json:"value_usd"`
	Shares         float64 `json:"shares"`
	Weight         float64 `json:"weight"`
	PreviousShares float64 `json:"previous_shares"`
	ShareChange    float64 `json:"share_change"`
	ShareChangePct float64 `json:"share_change_pct"`
	IsNew          bool    `json:"is_new"`
	IsExited       bool    `json:"is_exited"`
}

type Institution13FCacheStore struct {
	filename string
}

func NewInstitution13FCacheStore(filename string) *Institution13FCacheStore {
	return &Institution13FCacheStore{filename: filename}
}

func (s *Institution13FCacheStore) Load() (Institution13FCache, error) {
	institution13FCacheMu.Lock()
	defer institution13FCacheMu.Unlock()
	return s.loadUnlocked()
}

func (s *Institution13FCacheStore) Save(cache Institution13FCache) error {
	institution13FCacheMu.Lock()
	defer institution13FCacheMu.Unlock()
	cache.normalize()
	return s.saveUnlocked(cache)
}

func (s *Institution13FCacheStore) UpsertReport(report Institution13FReport, maxReports int) error {
	institution13FCacheMu.Lock()
	defer institution13FCacheMu.Unlock()

	cache, err := s.loadUnlocked()
	if err != nil {
		return err
	}
	if cache.Reports == nil {
		cache.Reports = map[string][]Institution13FReport{}
	}
	key := strings.TrimSpace(report.InstitutionID)
	if key == "" {
		key = strings.TrimSpace(report.CIK)
	}
	if key == "" {
		return fmt.Errorf("institution report missing id and CIK")
	}

	reports := cache.Reports[key]
	replaced := false
	for idx := range reports {
		if reports[idx].AccessionNumber == report.AccessionNumber {
			reports[idx] = report
			replaced = true
			break
		}
	}
	if !replaced {
		reports = append(reports, report)
	}
	sort.SliceStable(reports, func(i, j int) bool {
		return reports[i].ReportDate > reports[j].ReportDate
	})
	if maxReports <= 0 {
		maxReports = 8
	}
	if len(reports) > maxReports {
		reports = reports[:maxReports]
	}
	cache.Reports[key] = reports
	cache.UpdatedAt = time.Now()
	cache.normalize()
	return s.saveUnlocked(cache)
}

func (s *Institution13FCacheStore) Latest(institutionID string) (Institution13FReport, bool, error) {
	cache, err := s.Load()
	if err != nil {
		return Institution13FReport{}, false, err
	}
	reports := cache.Reports[strings.TrimSpace(institutionID)]
	if len(reports) == 0 {
		return Institution13FReport{}, false, nil
	}
	return reports[0], true, nil
}

func (s *Institution13FCacheStore) loadUnlocked() (Institution13FCache, error) {
	cache := Institution13FCache{Reports: map[string][]Institution13FReport{}}
	if strings.TrimSpace(s.filename) == "" {
		return cache, fmt.Errorf("institution 13F cache filename is empty")
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

func (s *Institution13FCacheStore) saveUnlocked(cache Institution13FCache) error {
	if strings.TrimSpace(s.filename) == "" {
		return fmt.Errorf("institution 13F cache filename is empty")
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

func (c *Institution13FCache) normalize() {
	if c.Reports == nil {
		c.Reports = map[string][]Institution13FReport{}
	}
	for key, reports := range c.Reports {
		filtered := make([]Institution13FReport, 0, len(reports))
		for _, report := range reports {
			if strings.TrimSpace(report.AccessionNumber) == "" {
				continue
			}
			normalizeInstitution13FReportValueScale(&report)
			sort.SliceStable(report.Holdings, func(i, j int) bool {
				return report.Holdings[i].ValueUSD > report.Holdings[j].ValueUSD
			})
			filtered = append(filtered, report)
		}
		sort.SliceStable(filtered, func(i, j int) bool {
			return filtered[i].ReportDate > filtered[j].ReportDate
		})
		c.Reports[key] = filtered
	}
}

func normalizeInstitution13FReportValueScale(report *Institution13FReport) {
	const likelyInflatedValueThreshold = 10_000_000_000_000.0
	if report == nil || report.TotalValueUSD <= likelyInflatedValueThreshold {
		return
	}
	report.TotalValueUSD = report.TotalValueUSD / 1000
	for idx := range report.Holdings {
		report.Holdings[idx].ValueUSD = report.Holdings[idx].ValueUSD / 1000
	}
	recalculateInstitution13FWeights(report)
}

func recalculateInstitution13FWeights(report *Institution13FReport) {
	if report == nil {
		return
	}
	total := report.TotalValueUSD
	if total <= 0 {
		for _, holding := range report.Holdings {
			total += holding.ValueUSD
		}
		report.TotalValueUSD = total
	}
	if total <= 0 {
		return
	}
	for idx := range report.Holdings {
		report.Holdings[idx].Weight = report.Holdings[idx].ValueUSD / total * 100
	}
}

func (r Institution13FReport) TotalValueText() string {
	return fmt.Sprintf("%.2f 亿美元", r.TotalValueUSD/100000000)
}

func (h Institution13FHolding) ValueText() string {
	return fmt.Sprintf("%.2f 亿美元", h.ValueUSD/100000000)
}

func (h Institution13FHolding) WeightText() string {
	return fmt.Sprintf("%.2f%%", h.Weight)
}

func (h Institution13FHolding) ShareChangeText() string {
	if h.IsNew {
		return "新进"
	}
	if h.IsExited {
		return "清仓"
	}
	if h.PreviousShares <= 0 {
		return "--"
	}
	return fmt.Sprintf("%+.2f%%", h.ShareChangePct)
}
