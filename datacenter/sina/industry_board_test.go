package sina

import "testing"

func TestParseSinaIndustryBoardResponse(t *testing.T) {
	text := `var S_Finance_bankuai_sinaindustry = {"new_blhy":"new_blhy,玻璃行业,19,19.36,-0.82,-4.10,1469239233,22482162699,sh600293,4.293,4.130,0.170,三峡新材","new_xny":"new_xny,新能源,5,16.75,0.58,3.39,85808625,1352381043,sh600629,1.37,15.840,0.22,华建集团"};`

	quotes, err := parseSinaIndustryBoardResponse(text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(quotes) != 2 {
		t.Fatalf("expected 2 quotes, got %d", len(quotes))
	}
	if !hasSinaIndustryQuote(quotes, "玻璃行业", -4.10) {
		t.Fatalf("expected glass industry quote, got %+v", quotes)
	}
	if !hasSinaIndustryQuote(quotes, "新能源", 3.39) {
		t.Fatalf("expected new energy quote, got %+v", quotes)
	}
}

func hasSinaIndustryQuote(quotes []MarketQuote, name string, changePercent float64) bool {
	for _, quote := range quotes {
		if quote.Name == name && quote.Category == MarketQuoteCategoryIndustry && quote.ChangePercent == changePercent {
			return true
		}
	}
	return false
}
