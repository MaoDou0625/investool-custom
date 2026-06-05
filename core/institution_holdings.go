package core

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"

	secdata "github.com/axiaoxin-com/investool/datacenter/sec"
	"github.com/axiaoxin-com/investool/models"
)

const DefaultInstitutionID = "berkshire"

type InstitutionProfile struct {
	ID          string
	Name        string
	CIK         string
	Description string
}

type Institution13FAnalysis struct {
	Report          models.Institution13FReport
	Profile         InstitutionProfile
	SectorExposures []InstitutionSectorExposure
	TopChanges      []Institution13FChange
	Overlaps        []InstitutionFundOverlap
	Warnings        []string
}

type InstitutionSectorExposure struct {
	Sector       string
	ValueUSD     float64
	Weight       float64
	HoldingCount int
}

type Institution13FChange struct {
	IssuerName     string
	Ticker         string
	Sector         string
	ValueUSD       float64
	CurrentShares  float64
	PreviousShares float64
	ShareChange    float64
	ShareChangePct float64
	Kind           string
}

type InstitutionFundOverlap struct {
	IssuerName        string
	Ticker            string
	CUSIP             string
	FundStockCode     string
	FundStockName     string
	FundSource        string
	FundWeight        float64
	InstitutionWeight float64
	MatchType         string
}

var institutionNameNormalizer = regexp.MustCompile(`[^0-9A-Z\p{Han}]+`)

func DefaultInstitutionProfiles() []InstitutionProfile {
	return []InstitutionProfile{
		{
			ID:          DefaultInstitutionID,
			Name:        "Berkshire Hathaway",
			CIK:         "0001067983",
			Description: "Warren Buffett / Berkshire Hathaway 13F",
		},
	}
}

func ResolveInstitutionProfile(id string, customCIK string) InstitutionProfile {
	id = strings.TrimSpace(strings.ToLower(id))
	customCIK = strings.TrimSpace(customCIK)
	if customCIK != "" {
		profileID := firstNonEmpty(id, strings.TrimSpace(customCIK))
		return InstitutionProfile{
			ID:          profileID,
			Name:        "Custom Institution",
			CIK:         customCIK,
			Description: "Custom SEC 13F CIK",
		}
	}
	if id == "" {
		id = DefaultInstitutionID
	}
	for _, profile := range DefaultInstitutionProfiles() {
		if profile.ID == id {
			return profile
		}
	}
	return DefaultInstitutionProfiles()[0]
}

func BuildInstitution13FReport(profile InstitutionProfile, current secdata.Report, previous *secdata.Report) models.Institution13FReport {
	previousShares := map[string]float64{}
	if previous != nil {
		for _, entry := range previous.Entries {
			key := institutionHoldingKey(entry.CUSIP, entry.NameOfIssuer)
			previousShares[key] += entry.SharesOrPrincipal
		}
	}

	totalValue := 0.0
	holdings := make([]models.Institution13FHolding, 0, len(current.Entries))
	for _, entry := range current.Entries {
		valueUSD := entry.ValueThousands
		if valueUSD < 0 {
			valueUSD = 0
		}
		totalValue += valueUSD
		holding := models.Institution13FHolding{
			IssuerName:   entry.NameOfIssuer,
			TitleOfClass: entry.TitleOfClass,
			CUSIP:        entry.CUSIP,
			Ticker:       InstitutionTicker(entry.CUSIP, entry.NameOfIssuer),
			Sector:       InstitutionSector(entry.CUSIP, entry.NameOfIssuer),
			ValueUSD:     valueUSD,
			Shares:       entry.SharesOrPrincipal,
		}
		key := institutionHoldingKey(entry.CUSIP, entry.NameOfIssuer)
		if prev := previousShares[key]; prev > 0 {
			holding.PreviousShares = prev
			holding.ShareChange = holding.Shares - prev
			holding.ShareChangePct = holding.ShareChange / prev * 100
		} else if previous != nil {
			holding.IsNew = true
		}
		holdings = append(holdings, holding)
	}
	if totalValue > 0 {
		for idx := range holdings {
			holdings[idx].Weight = holdings[idx].ValueUSD / totalValue * 100
		}
	}
	sort.SliceStable(holdings, func(i, j int) bool {
		return holdings[i].ValueUSD > holdings[j].ValueUSD
	})

	return models.Institution13FReport{
		InstitutionID:       profile.ID,
		InstitutionName:     firstNonEmpty(current.InstitutionName, profile.Name),
		CIK:                 current.CIK,
		Form:                current.Filing.Form,
		AccessionNumber:     current.Filing.AccessionNumber,
		FilingDate:          current.Filing.FilingDate,
		ReportDate:          current.Filing.ReportDate,
		SourceURL:           filingIndexURL(current.Filing.CIK, current.Filing.AccessionNumber),
		InformationTableURL: current.InformationTableURL,
		TotalValueUSD:       totalValue,
		Holdings:            holdings,
	}
}

func AnalyzeInstitution13F(report models.Institution13FReport, previous *models.Institution13FReport, exposure FundPortfolioExposureReport, profile InstitutionProfile) Institution13FAnalysis {
	analysis := Institution13FAnalysis{
		Report:   report,
		Profile:  profile,
		Warnings: []string{},
	}
	if len(report.Holdings) == 0 {
		analysis.Warnings = append(analysis.Warnings, "13F holding table is empty or not available.")
		return analysis
	}
	analysis.SectorExposures = BuildInstitutionSectorExposures(report.Holdings, report.TotalValueUSD)
	analysis.TopChanges = BuildInstitutionTopChanges(report, previous)
	analysis.Overlaps = BuildInstitutionFundOverlaps(report.Holdings, exposure.StockExposures)
	if len(analysis.Overlaps) == 0 {
		analysis.Warnings = append(analysis.Warnings, "No direct overlap was found between this 13F portfolio and current fund heavy-stock exposure.")
	}
	return analysis
}

func BuildInstitutionSectorExposures(holdings []models.Institution13FHolding, totalValue float64) []InstitutionSectorExposure {
	if totalValue <= 0 {
		for _, holding := range holdings {
			totalValue += holding.ValueUSD
		}
	}
	acc := map[string]*InstitutionSectorExposure{}
	for _, holding := range holdings {
		if holding.ValueUSD <= 0 {
			continue
		}
		sector := firstNonEmpty(holding.Sector, "Unclassified")
		row := acc[sector]
		if row == nil {
			row = &InstitutionSectorExposure{Sector: sector}
			acc[sector] = row
		}
		row.ValueUSD += holding.ValueUSD
		row.HoldingCount++
	}
	rows := make([]InstitutionSectorExposure, 0, len(acc))
	for _, row := range acc {
		if totalValue > 0 {
			row.Weight = row.ValueUSD / totalValue * 100
		}
		rows = append(rows, *row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].Weight > rows[j].Weight
	})
	return rows
}

func BuildInstitutionTopChanges(report models.Institution13FReport, previous *models.Institution13FReport) []Institution13FChange {
	changes := []Institution13FChange{}
	currentKeys := map[string]struct{}{}
	for _, holding := range report.Holdings {
		key := institutionHoldingKey(holding.CUSIP, holding.IssuerName)
		currentKeys[key] = struct{}{}
		if holding.IsNew || math.Abs(holding.ShareChangePct) >= 1 {
			changes = append(changes, Institution13FChange{
				IssuerName:     holding.IssuerName,
				Ticker:         holding.Ticker,
				Sector:         holding.Sector,
				ValueUSD:       holding.ValueUSD,
				CurrentShares:  holding.Shares,
				PreviousShares: holding.PreviousShares,
				ShareChange:    holding.ShareChange,
				ShareChangePct: holding.ShareChangePct,
				Kind:           holdingChangeKind(holding),
			})
		}
	}
	if previous != nil {
		for _, holding := range previous.Holdings {
			key := institutionHoldingKey(holding.CUSIP, holding.IssuerName)
			if _, ok := currentKeys[key]; ok {
				continue
			}
			changes = append(changes, Institution13FChange{
				IssuerName:     holding.IssuerName,
				Ticker:         holding.Ticker,
				Sector:         holding.Sector,
				PreviousShares: holding.Shares,
				ShareChange:    -holding.Shares,
				ShareChangePct: -100,
				Kind:           "Exited",
			})
		}
	}
	sort.SliceStable(changes, func(i, j int) bool {
		ai := math.Abs(changes[i].ShareChangePct)
		aj := math.Abs(changes[j].ShareChangePct)
		if ai != aj {
			return ai > aj
		}
		return changes[i].ValueUSD > changes[j].ValueUSD
	})
	if len(changes) > 20 {
		changes = changes[:20]
	}
	return changes
}

func BuildInstitutionFundOverlaps(holdings []models.Institution13FHolding, exposures []FundPortfolioStockExposure) []InstitutionFundOverlap {
	holdingByTicker := map[string]models.Institution13FHolding{}
	holdingByName := map[string]models.Institution13FHolding{}
	for _, holding := range holdings {
		if holding.Ticker != "" {
			holdingByTicker[strings.ToUpper(holding.Ticker)] = holding
		}
		nameKey := normalizeInstitutionCompareName(holding.IssuerName)
		if nameKey != "" {
			holdingByName[nameKey] = holding
		}
	}

	overlaps := []InstitutionFundOverlap{}
	seen := map[string]struct{}{}
	for _, exposure := range exposures {
		if exposure.Weight <= 0 {
			continue
		}
		var holding models.Institution13FHolding
		matchType := ""
		if exposure.StockCode != "" {
			if match, ok := holdingByTicker[strings.ToUpper(exposure.StockCode)]; ok {
				holding = match
				matchType = "ticker"
			}
		}
		if matchType == "" {
			nameKey := normalizeInstitutionCompareName(exposure.StockName)
			if match, ok := holdingByName[nameKey]; ok {
				holding = match
				matchType = "name"
			}
		}
		if matchType == "" {
			continue
		}
		key := strings.Join([]string{holding.CUSIP, exposure.StockCode, exposure.SourceFund, exposure.TargetETFCode}, "|")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		overlaps = append(overlaps, InstitutionFundOverlap{
			IssuerName:        holding.IssuerName,
			Ticker:            holding.Ticker,
			CUSIP:             holding.CUSIP,
			FundStockCode:     exposure.StockCode,
			FundStockName:     exposure.StockName,
			FundSource:        exposure.ExposureSummary,
			FundWeight:        exposure.Weight,
			InstitutionWeight: holding.Weight,
			MatchType:         matchType,
		})
	}
	sort.SliceStable(overlaps, func(i, j int) bool {
		if overlaps[i].FundWeight != overlaps[j].FundWeight {
			return overlaps[i].FundWeight > overlaps[j].FundWeight
		}
		return overlaps[i].InstitutionWeight > overlaps[j].InstitutionWeight
	})
	return overlaps
}

func InstitutionTicker(cusip, issuerName string) string {
	key := strings.ToUpper(strings.TrimSpace(cusip))
	if ticker := institutionTickerByCUSIP[key]; ticker != "" {
		return ticker
	}
	normalized := strings.ToUpper(issuerName)
	for keyword, ticker := range institutionTickerByIssuerKeyword {
		if strings.Contains(normalized, keyword) {
			return ticker
		}
	}
	return ""
}

func InstitutionSector(cusip, issuerName string) string {
	key := strings.ToUpper(strings.TrimSpace(cusip))
	if sector := institutionSectorByCUSIP[key]; sector != "" {
		return sector
	}
	normalized := strings.ToUpper(issuerName)
	for keyword, sector := range institutionSectorByIssuerKeyword {
		if strings.Contains(normalized, keyword) {
			return sector
		}
	}
	return "Unclassified"
}

func (s InstitutionSectorExposure) ValueText() string {
	return fmt.Sprintf("%.2f 亿美元", s.ValueUSD/100000000)
}

func (s InstitutionSectorExposure) WeightText() string {
	return fmt.Sprintf("%.2f%%", s.Weight)
}

func (c Institution13FChange) ValueText() string {
	if c.ValueUSD <= 0 {
		return "--"
	}
	return fmt.Sprintf("%.2f 亿美元", c.ValueUSD/100000000)
}

func (c Institution13FChange) ShareChangeText() string {
	return fmt.Sprintf("%+.2f%%", c.ShareChangePct)
}

func (o InstitutionFundOverlap) FundWeightText() string {
	return fmt.Sprintf("%.2f%%", o.FundWeight)
}

func (o InstitutionFundOverlap) InstitutionWeightText() string {
	return fmt.Sprintf("%.2f%%", o.InstitutionWeight)
}

func holdingChangeKind(holding models.Institution13FHolding) string {
	switch {
	case holding.IsNew:
		return "New"
	case holding.ShareChangePct > 0:
		return "Increased"
	case holding.ShareChangePct < 0:
		return "Reduced"
	default:
		return "Unchanged"
	}
}

func institutionHoldingKey(cusip, issuerName string) string {
	cusip = strings.ToUpper(strings.TrimSpace(cusip))
	if cusip != "" {
		return cusip
	}
	return normalizeInstitutionCompareName(issuerName)
}

func normalizeInstitutionCompareName(name string) string {
	upper := strings.ToUpper(strings.TrimSpace(name))
	for _, suffix := range []string{
		" INC", " INC.", " CORP", " CORP.", " CORPORATION", " CO", " CO.", " LTD", " LTD.", " PLC", " ADR", " ADS", " CLASS A", " CL A", " COM",
	} {
		upper = strings.ReplaceAll(upper, suffix, " ")
	}
	return strings.TrimSpace(institutionNameNormalizer.ReplaceAllString(upper, ""))
}

func filingIndexURL(cik, accessionNumber string) string {
	normalizedCIK := strings.TrimLeft(strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(cik)), "CIK"), "0")
	if normalizedCIK == "" {
		normalizedCIK = "0"
	}
	noDash := strings.ReplaceAll(accessionNumber, "-", "")
	return fmt.Sprintf("https://www.sec.gov/Archives/edgar/data/%s/%s/%s-index.htm", normalizedCIK, noDash, accessionNumber)
}

var institutionTickerByCUSIP = map[string]string{
	"037833100": "AAPL",
	"025816109": "AXP",
	"060505104": "BAC",
	"191216100": "KO",
	"166764100": "CVX",
	"674599105": "OXY",
	"615369105": "MCO",
	"500754106": "KHC",
	"23918K108": "DVA",
	"172967424": "C",
	"92826C839": "V",
	"57636Q104": "MA",
	"023135106": "AMZN",
	"14040H105": "COF",
}

var institutionTickerByIssuerKeyword = map[string]string{
	"APPLE":            "AAPL",
	"AMERICAN EXPRESS": "AXP",
	"BANK OF AMERICA":  "BAC",
	"COCA COLA":        "KO",
	"COCA-COLA":        "KO",
	"CHEVRON":          "CVX",
	"OCCIDENTAL":       "OXY",
	"MOODYS":           "MCO",
	"MOODY":            "MCO",
	"KRAFT HEINZ":      "KHC",
	"CHUBB":            "CB",
	"DAVITA":           "DVA",
	"CITIGROUP":        "C",
	"VISA":             "V",
	"MASTERCARD":       "MA",
	"AMAZON":           "AMZN",
	"CAPITAL ONE":      "COF",
}

var institutionSectorByCUSIP = map[string]string{
	"037833100": "Technology",
	"025816109": "Financials",
	"060505104": "Financials",
	"191216100": "Consumer Staples",
	"166764100": "Energy",
	"674599105": "Energy",
	"615369105": "Financials",
	"500754106": "Consumer Staples",
	"23918K108": "Health Care",
	"172967424": "Financials",
	"92826C839": "Financials",
	"57636Q104": "Financials",
	"023135106": "Consumer Discretionary",
	"14040H105": "Financials",
}

var institutionSectorByIssuerKeyword = map[string]string{
	"APPLE":            "Technology",
	"AMERICAN EXPRESS": "Financials",
	"BANK OF AMERICA":  "Financials",
	"COCA":             "Consumer Staples",
	"CHEVRON":          "Energy",
	"OCCIDENTAL":       "Energy",
	"MOODY":            "Financials",
	"KRAFT":            "Consumer Staples",
	"CHUBB":            "Financials",
	"DAVITA":           "Health Care",
	"CITIGROUP":        "Financials",
	"VISA":             "Financials",
	"MASTERCARD":       "Financials",
	"AMAZON":           "Consumer Discretionary",
	"CAPITAL ONE":      "Financials",
}

func FetchInstitution13FReport(ctx context.Context, client *secdata.Client, profile InstitutionProfile) (models.Institution13FReport, *models.Institution13FReport, error) {
	if client == nil {
		client = secdata.NewClient()
	}
	reports, err := client.FetchLatest13FReports(ctx, profile.CIK, 2)
	if err != nil {
		return models.Institution13FReport{}, nil, err
	}
	var previousSEC *secdata.Report
	if len(reports) > 1 {
		previousSEC = &reports[1]
	}
	current := BuildInstitution13FReport(profile, reports[0], previousSEC)
	if previousSEC == nil {
		return current, nil, nil
	}
	previous := BuildInstitution13FReport(profile, *previousSEC, nil)
	return current, &previous, nil
}
