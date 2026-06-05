package core

import (
	"testing"

	secdata "github.com/axiaoxin-com/investool/datacenter/sec"
	"github.com/axiaoxin-com/investool/models"
)

func TestBuildInstitution13FReportComputesWeightsAndChanges(t *testing.T) {
	profile := ResolveInstitutionProfile("berkshire", "")
	current := secdata.Report{
		CIK:             "0001067983",
		InstitutionName: "Berkshire Hathaway",
		Filing: secdata.Filing{
			CIK:             "0001067983",
			AccessionNumber: "0000000000-26-000001",
			ReportDate:      "2026-03-31",
		},
		Entries: []secdata.InformationTableEntry{
			{NameOfIssuer: "APPLE INC", CUSIP: "037833100", ValueThousands: 1000, SharesOrPrincipal: 80},
			{NameOfIssuer: "COCA COLA CO", CUSIP: "191216100", ValueThousands: 500, SharesOrPrincipal: 50},
		},
	}
	previous := secdata.Report{
		Entries: []secdata.InformationTableEntry{
			{NameOfIssuer: "APPLE INC", CUSIP: "037833100", ValueThousands: 900, SharesOrPrincipal: 100},
		},
	}

	report := BuildInstitution13FReport(profile, current, &previous)
	if report.TotalValueUSD != 1500 {
		t.Fatalf("unexpected total value: %.0f", report.TotalValueUSD)
	}
	if len(report.Holdings) != 2 {
		t.Fatalf("expected 2 holdings, got %d", len(report.Holdings))
	}
	if report.Holdings[0].Ticker != "AAPL" {
		t.Fatalf("expected AAPL ticker, got %s", report.Holdings[0].Ticker)
	}
	if report.Holdings[0].ShareChangePct != -20 {
		t.Fatalf("expected -20%% share change, got %.2f", report.Holdings[0].ShareChangePct)
	}
	if !report.Holdings[1].IsNew {
		t.Fatalf("expected second holding to be new")
	}
}

func TestBuildInstitutionFundOverlapsMatchesByName(t *testing.T) {
	overlaps := BuildInstitutionFundOverlaps(
		[]models.Institution13FHolding{
			{IssuerName: "APPLE INC", CUSIP: "037833100", Ticker: "AAPL", Weight: 40},
		},
		[]FundPortfolioStockExposure{
			{StockName: "Apple", StockCode: "not-aapl", Weight: 3.5, ExposureSummary: "fund 1"},
		},
	)
	if len(overlaps) != 1 {
		t.Fatalf("expected one overlap, got %d", len(overlaps))
	}
	if overlaps[0].MatchType != "name" {
		t.Fatalf("unexpected match type: %s", overlaps[0].MatchType)
	}
}
