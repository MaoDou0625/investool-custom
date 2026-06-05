package sec

import "testing"

func TestParseInformationTableXML(t *testing.T) {
	xmlContent := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<informationTable xmlns="http://www.sec.gov/edgar/document/thirteenf/informationtable">
  <infoTable>
    <nameOfIssuer>APPLE INC</nameOfIssuer>
    <titleOfClass>COM</titleOfClass>
    <cusip>037833100</cusip>
    <value>  63785907 </value>
    <shrsOrPrnAmt>
      <sshPrnamt>300000000</sshPrnamt>
      <sshPrnamtType>SH</sshPrnamtType>
    </shrsOrPrnAmt>
    <investmentDiscretion>DFND</investmentDiscretion>
  </infoTable>
</informationTable>`)

	entries, err := ParseInformationTableXML(xmlContent)
	if err != nil {
		t.Fatalf("ParseInformationTableXML returned error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	entry := entries[0]
	if entry.NameOfIssuer != "APPLE INC" {
		t.Fatalf("unexpected issuer: %s", entry.NameOfIssuer)
	}
	if entry.CUSIP != "037833100" {
		t.Fatalf("unexpected CUSIP: %s", entry.CUSIP)
	}
	if entry.ValueThousands != 63785907 {
		t.Fatalf("unexpected value: %.0f", entry.ValueThousands)
	}
	if entry.SharesOrPrincipal != 300000000 {
		t.Fatalf("unexpected shares: %.0f", entry.SharesOrPrincipal)
	}
}

func TestNormalizeCIK(t *testing.T) {
	got := normalizeCIK("CIK1067983")
	if got != "0001067983" {
		t.Fatalf("unexpected CIK: %s", got)
	}
}
